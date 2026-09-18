// One-off correction for RunningHub tasks that were undercharged by the
// "1 RH coin = 1 new-api quota" settlement bug.
//
// The bug: the adaptor's AdjustBillingOnComplete returned RH's
// usage.consumeCoins as raw quota, so the completion poll replaced a correct
// pre-charge with a near-zero charge. Fixing the code stops new occurrences but
// leaves the historical rows short. This script restores the intended charge for
// one task: the task row, the wallet, the token quota, the usage counters, and an
// auditable log trail.
//
// Usage:
//
//	go run ./_scripts/rh-bill-backfill -scan                  # read-only audit
//	go run ./_scripts/rh-bill-backfill -task task_XXX         # dry run
//	go run ./_scripts/rh-bill-backfill -task task_XXX -apply  # write
//
// SQL_DSN comes from the process env or .env.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	_ "github.com/go-sql-driver/mysql"
)

const (
	logTypeConsume = 2
	logTypeManage  = 3
	// quotaPerUnitFallback matches common.QuotaPerUnit's default: the host reads
	// the live value from the options table.
	quotaPerUnitFallback = 500000.0
)

type billingContext struct {
	ModelPrice      float64            `json:"model_price"`
	GroupRatio      float64            `json:"group_ratio"`
	ModelRatio      float64            `json:"model_ratio"`
	OtherRatios     map[string]float64 `json:"other_ratios"`
	OriginModelName string             `json:"origin_model_name"`
	PerCallBilling  bool               `json:"per_call_billing"`
}

type taskRow struct {
	ID          int64
	TaskID      string
	UserID      int
	ChannelID   int
	Quota       int64
	Status      string
	SubmitTime  int64
	PrivateData []byte
}

func main() {
	var (
		taskID = flag.String("task", "", "public task id to correct")
		apply  = flag.Bool("apply", false, "write the correction (default: dry run)")
		scan   = flag.Bool("scan", false, "list every RunningHub task whose charge is below its configured price")
	)
	flag.Parse()

	_ = godotenv.Load(".env")
	dsn := os.Getenv("SQL_DSN")
	if dsn == "" {
		log.Fatal("SQL_DSN not set (checked env and .env)")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(2 * time.Minute)
	db.SetMaxOpenConns(2)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("ping: %v", err)
	}

	quotaPerUnit := optionFloat(ctx, db, "QuotaPerUnit", quotaPerUnitFallback)
	fmt.Printf("QuotaPerUnit = %v\n", quotaPerUnit)

	if *scan {
		scanUndercharged(ctx, db, quotaPerUnit)
		return
	}
	if *taskID == "" {
		log.Fatal("pass -task <public task id>, or -scan")
	}

	task, err := loadTask(ctx, db, *taskID)
	if err != nil {
		log.Fatalf("load task: %v", err)
	}
	var privateData struct {
		BillingContext *billingContext `json:"billing_context"`
	}
	if err := json.Unmarshal(task.PrivateData, &privateData); err != nil {
		log.Fatalf("parse private_data: %v", err)
	}
	bc := privateData.BillingContext
	if bc == nil {
		log.Fatalf("task %s has no billing_context; refusing to guess the intended charge", task.TaskID)
	}

	expected, how := expectedQuota(bc, quotaPerUnit)
	delta := expected - task.Quota
	fmt.Printf("task        : %s (db id %d, user %d, channel %d, status %s)\n", task.TaskID, task.ID, task.UserID, task.ChannelID, task.Status)
	fmt.Printf("billing     : %s\n", how)
	fmt.Printf("quota       : current %d → expected %d (delta %d = $%.6f)\n",
		task.Quota, expected, delta, float64(delta)/quotaPerUnit)
	if delta == 0 {
		fmt.Println("nothing to do")
		return
	}
	if delta < 0 {
		log.Fatalf("task was overcharged, not undercharged (delta %d); refusing", delta)
	}

	done, err := alreadyCorrected(ctx, db, task.TaskID)
	if err != nil {
		log.Fatalf("idempotency check: %v", err)
	}
	if done {
		log.Fatalf("task %s already carries a manual correction log; refusing to apply twice", task.TaskID)
	}

	user, err := loadAccount(ctx, db, task.UserID)
	if err != nil {
		log.Fatalf("load user: %v", err)
	}
	fmt.Printf("user %d %s: quota %d, used_quota %d\n", user.ID, user.Username, user.Quota, user.UsedQuota)
	if user.Quota < delta {
		log.Fatalf("user balance %d < required %d; top up first instead of forcing a negative balance", user.Quota, delta)
	}

	token, err := loadToken(ctx, db, task.TaskID)
	if err != nil {
		log.Fatalf("load token: %v", err)
	}
	if token != nil {
		fmt.Printf("token %d %s: remain %d, used %d, unlimited=%v\n",
			token.ID, token.Name, token.RemainQuota, token.UsedQuota, token.UnlimitedQuota)
	}
	channelUsed, err := channelUsedQuota(ctx, db, task.ChannelID)
	if err != nil {
		log.Fatalf("load channel: %v", err)
	}
	fmt.Printf("channel %d: used_quota %d → %d\n", task.ChannelID, channelUsed, channelUsed+delta)
	fmt.Printf("user %d: quota %d → %d, used_quota %d → %d\n",
		user.ID, user.Quota, user.Quota-delta, user.UsedQuota, user.UsedQuota+delta)

	if !*apply {
		fmt.Println("\nDRY RUN — nothing written. Re-run with -apply to commit.")
		return
	}

	reason := fmt.Sprintf("人工补记按秒计费差额：任务 %s 原结算误将 RH 币按 1:1 记为 quota", task.TaskID)
	if err := applyCorrection(ctx, db, task, user, token, delta, expected, bc, quotaPerUnit, reason); err != nil {
		log.Fatalf("apply: %v", err)
	}

	after, err := loadTask(ctx, db, task.TaskID)
	if err != nil {
		log.Fatalf("reload task: %v", err)
	}
	afterUser, err := loadAccount(ctx, db, task.UserID)
	if err != nil {
		log.Fatalf("reload user: %v", err)
	}
	fmt.Printf("\nAPPLIED: task quota %d, user quota %d, user used_quota %d\n",
		after.Quota, afterUser.Quota, afterUser.UsedQuota)
}

// expectedQuota recomputes the charge the submit path would have produced.
// Per-second apps charge model_price × QuotaPerUnit × seconds (the seconds
// factor arrives as the "seconds" other-ratio); every other mode charges the flat
// model_price × QuotaPerUnit.
func expectedQuota(bc *billingContext, quotaPerUnit float64) (int64, string) {
	if seconds, ok := bc.OtherRatios["seconds"]; ok && seconds > 0 {
		return int64(bc.ModelPrice*quotaPerUnit*seconds + 0.5),
			fmt.Sprintf("per-second: model_price %v × QuotaPerUnit %v × seconds %v (per_call=%v)",
				bc.ModelPrice, quotaPerUnit, seconds, bc.PerCallBilling)
	}
	return int64(bc.ModelPrice*quotaPerUnit + 0.5),
		fmt.Sprintf("flat: model_price %v × QuotaPerUnit %v (per_call=%v)", bc.ModelPrice, quotaPerUnit, bc.PerCallBilling)
}

// scanUndercharged lists successful RunningHub tasks whose recorded quota is
// below the price configured for them at submit time — the fingerprint of the
// coin-settlement bug. Failed/cancelled runs are excluded on purpose: they are
// refunded in full, so quota = 0 is correct for them.
func scanUndercharged(ctx context.Context, db *sql.DB, quotaPerUnit float64) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, task_id, user_id, channel_id, quota, status, submit_time, private_data
		 FROM tasks WHERE platform IN ('61','62','63') AND status = 'SUCCESS' ORDER BY id`)
	if err != nil {
		log.Fatalf("scan: %v", err)
	}
	defer rows.Close()

	flagged := 0
	scanned := 0
	for rows.Next() {
		var t taskRow
		if err := rows.Scan(&t.ID, &t.TaskID, &t.UserID, &t.ChannelID, &t.Quota, &t.Status, &t.SubmitTime, &t.PrivateData); err != nil {
			log.Fatalf("scan row: %v", err)
		}
		scanned++
		var privateData struct {
			BillingContext *billingContext `json:"billing_context"`
		}
		if err := json.Unmarshal(t.PrivateData, &privateData); err != nil || privateData.BillingContext == nil {
			continue
		}
		expected, _ := expectedQuota(privateData.BillingContext, quotaPerUnit)
		if expected > t.Quota {
			flagged++
			fmt.Printf("id=%-4d %s user=%d quota=%-8d expected=%-8d short=%-8d ($%.6f)\n",
				t.ID, t.TaskID, t.UserID, t.Quota, expected, expected-t.Quota,
				float64(expected-t.Quota)/quotaPerUnit)
		}
	}
	fmt.Printf("\nscanned %d successful RunningHub task(s); %d charged below their configured price\n", scanned, flagged)
}

func loadTask(ctx context.Context, db *sql.DB, taskID string) (*taskRow, error) {
	var t taskRow
	err := db.QueryRowContext(ctx,
		`SELECT id, task_id, user_id, channel_id, quota, status, submit_time, private_data
		 FROM tasks WHERE task_id = ?`, taskID).
		Scan(&t.ID, &t.TaskID, &t.UserID, &t.ChannelID, &t.Quota, &t.Status, &t.SubmitTime, &t.PrivateData)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// alreadyCorrected reports whether a manual correction log for this task exists,
// so re-running the script can never double-charge.
func alreadyCorrected(ctx context.Context, db *sql.DB, taskID string) (bool, error) {
	var n int64
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM logs WHERE other LIKE ? AND other LIKE '%"manual_correction":true%'`,
		"%"+taskID+"%").Scan(&n)
	return n > 0, err
}

type account struct {
	ID        int
	Username  string
	Quota     int64
	UsedQuota int64
}

func loadAccount(ctx context.Context, db *sql.DB, userID int) (*account, error) {
	a := &account{ID: userID}
	err := db.QueryRowContext(ctx,
		"SELECT `username`, `quota`, `used_quota` FROM users WHERE id = ?", userID).
		Scan(&a.Username, &a.Quota, &a.UsedQuota)
	return a, err
}

type tokenRow struct {
	ID             int
	Name           string
	RemainQuota    int64
	UsedQuota      int64
	UnlimitedQuota bool
}

// loadToken reads the token the task was billed against (recorded in
// private_data by the submit controller). A missing token is not fatal.
func loadToken(ctx context.Context, db *sql.DB, taskID string) (*tokenRow, error) {
	var tokenID sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT JSON_EXTRACT(private_data, '$.token_id') FROM tasks WHERE task_id = ?`, taskID).Scan(&tokenID)
	if err != nil || !tokenID.Valid || tokenID.Int64 <= 0 {
		return nil, nil
	}
	t := &tokenRow{ID: int(tokenID.Int64)}
	err = db.QueryRowContext(ctx,
		"SELECT `name`, `remain_quota`, `used_quota`, `unlimited_quota` FROM tokens WHERE id = ?", t.ID).
		Scan(&t.Name, &t.RemainQuota, &t.UsedQuota, &t.UnlimitedQuota)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return t, err
}

func channelUsedQuota(ctx context.Context, db *sql.DB, channelID int) (int64, error) {
	var used int64
	err := db.QueryRowContext(ctx, "SELECT `used_quota` FROM channels WHERE id = ?", channelID).Scan(&used)
	return used, err
}

func optionFloat(ctx context.Context, db *sql.DB, key string, fallback float64) float64 {
	var raw string
	if err := db.QueryRowContext(ctx, "SELECT `value` FROM options WHERE `key` = ?", key).Scan(&raw); err != nil {
		return fallback
	}
	var v float64
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return fallback
	}
	return v
}

// applyCorrection moves the whole delta in one transaction: task row, wallet,
// token quota, usage counters, plus the consume log the poller would have written
// and an admin audit row.
func applyCorrection(
	ctx context.Context, db *sql.DB, task *taskRow, user *account, token *tokenRow,
	delta, expected int64, bc *billingContext, quotaPerUnit float64, reason string,
) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// CAS on the quota we read: if the task changed meanwhile, nothing is written.
	res, err := tx.ExecContext(ctx,
		"UPDATE tasks SET quota = ? WHERE id = ? AND quota = ?", expected, task.ID, task.Quota)
	if err != nil {
		return fmt.Errorf("update task: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("task update affected %d rows (expected 1): concurrent change?", n)
	}

	res, err = tx.ExecContext(ctx,
		"UPDATE users SET `quota` = `quota` - ?, `used_quota` = `used_quota` + ? WHERE id = ? AND `quota` >= ?",
		delta, delta, task.UserID, delta)
	if err != nil {
		return fmt.Errorf("update user: %w", err)
	}
	if n, _ := res.RowsAffected(); n != 1 {
		return fmt.Errorf("user update affected %d rows (expected 1): insufficient balance?", n)
	}

	// Mirror service.taskAdjustTokenQuota: task billing decrements the token even
	// when it is unlimited (its remain_quota is allowed to run negative; the
	// enforced check happens at pre-consume for limited tokens only).
	if token != nil {
		q := "UPDATE tokens SET `remain_quota` = `remain_quota` - ?, `used_quota` = `used_quota` + ? WHERE id = ?"
		args := []any{delta, delta, token.ID}
		if !token.UnlimitedQuota {
			q += " AND `remain_quota` >= ?"
			args = append(args, delta)
		}
		res, err = tx.ExecContext(ctx, q, args...)
		if err != nil {
			return fmt.Errorf("update token: %w", err)
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return fmt.Errorf("token update affected %d rows (expected 1): insufficient token quota?", n)
		}
	}

	if _, err = tx.ExecContext(ctx,
		"UPDATE channels SET `used_quota` = `used_quota` + ? WHERE id = ?", delta, task.ChannelID); err != nil {
		return fmt.Errorf("update channel: %w", err)
	}

	now := time.Now().Unix()
	consumeOther, _ := json.Marshal(map[string]any{
		"task_id":                task.TaskID,
		"pre_consumed_quota":     expected,
		"actual_quota":           expected,
		"model_price":            bc.ModelPrice,
		"group_ratio":            bc.GroupRatio,
		"other_ratios":           bc.OtherRatios,
		"manual_correction":      true,
		"corrected_settle_log":   true,
		"quota_per_unit":         quotaPerUnit,
		"note":                   "原结算将 RH usage.consumeCoins 1:1 当作 quota，导致差距被退还",
	})
	tokenName := ""
	tokenID := 0
	if token != nil {
		tokenName, tokenID = token.Name, token.ID
	}
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO logs (`user_id`,`created_at`,`type`,`content`,`username`,`token_name`,`model_name`,"+
			"`quota`,`prompt_tokens`,`completion_tokens`,`use_time`,`is_stream`,`channel_id`,`token_id`,"+
			"`group`,`ip`,`request_id`,`upstream_request_id`,`other`) "+
			"VALUES (?,?,?,?,?,?,?,?,0,0,0,0,?,?,?,?,?,?,?)",
		task.UserID, now, logTypeConsume, reason, user.Username, tokenName, bc.OriginModelName,
		delta, task.ChannelID, tokenID, "", "", "", "", string(consumeOther)); err != nil {
		return fmt.Errorf("insert consume log: %w", err)
	}

	auditOther, _ := json.Marshal(map[string]any{
		"admin_info": map[string]any{
			"admin_id":       1,
			"admin_role":     100,
			"admin_username": "backfill-script",
			"auth_method":    "script",
		},
		"op": map[string]any{
			"action": "rh.billing.backfill",
			"params": map[string]any{
				"task_id":      task.TaskID,
				"user_id":      task.UserID,
				"from_quota":   task.Quota,
				"to_quota":     expected,
				"delta":        delta,
				"channel_id":   task.ChannelID,
				"token_id":     tokenID,
				"reason":       reason,
				"quota_source": "billing_context",
			},
		},
	})
	if _, err = tx.ExecContext(ctx,
		"INSERT INTO logs (`user_id`,`created_at`,`type`,`content`,`username`,`token_name`,`model_name`,"+
			"`quota`,`prompt_tokens`,`completion_tokens`,`use_time`,`is_stream`,`channel_id`,`token_id`,"+
			"`group`,`ip`,`request_id`,`upstream_request_id`,`other`) "+
			"VALUES (?,?,?,?,?,?,'',0,0,0,0,0,?,?,?,?,?,?,?)",
		task.UserID, now, logTypeManage, "RH 计费人工补记", user.Username, tokenName,
		task.ChannelID, tokenID, "", "", "", "", string(auditOther)); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return tx.Commit()
}
