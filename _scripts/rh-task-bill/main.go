// Diagnostic: dump everything the billing chain recorded for one RunningHub task.
//
// Usage: go run ./_scripts/rh-task-bill <task_id>
//
// Reads SQL_DSN from the process env or from .env (godotenv), then prints the
// tasks row, the plugin app row, the related logs rows and the pricing options
// as JSON so a mismatch between the configured price and the charged quota can
// be explained from data instead of guesses.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	_ = godotenv.Load(".env")
	dsn := os.Getenv("SQL_DSN")
	if dsn == "" {
		log.Fatal("SQL_DSN not set (checked env and .env)")
	}
	taskID := "task_U4n7F7SxBdO4TdrfbWADfYd7XEiF9CRq"
	if len(os.Args) > 1 {
		taskID = os.Args[1]
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

	out := map[string]any{}
	emit(out, "task", queryTask(ctx, db, taskID))
	emit(out, "affected_accounts", queryRowList(ctx, db,
		`SELECT u.id, u.username, u.quota AS user_quota, u.used_quota AS user_used_quota,
		        t.id AS token_id, t.name AS token_name, t.remain_quota, t.used_quota AS token_used_quota,
		        t.unlimited_quota, t.`+"`group`"+` AS token_group,
		        c.id AS channel_id, c.name AS channel_name, c.used_quota AS channel_used_quota
		 FROM tasks k
		 LEFT JOIN users   u ON u.id = k.user_id
		 LEFT JOIN tokens  t ON t.id = (SELECT token_id FROM tasks WHERE id = k.id)
		 LEFT JOIN channels c ON c.id = k.channel_id
		 WHERE k.task_id = ?`, taskID))
	emit(out, "logs_window", queryRowList(ctx, db,
		`SELECT id, type, user_id, token_id, model_name, quota, channel_id, created_at, content, other
		 FROM logs WHERE user_id = 1 AND created_at BETWEEN ? AND ?
		 ORDER BY id`, 1789705600, 1789706100))
	emit(out, "app_tables", queryRowList(ctx, db,
		`SELECT table_name FROM information_schema.tables
		 WHERE table_schema = DATABASE() AND (table_name LIKE '%app%' OR table_name LIKE '%rh%')`))
	emit(out, "logs_by_task_id", queryRowList(ctx, db,
		`SELECT id, type, user_id, username, token_name, model_name, quota, prompt_tokens,
		        completion_tokens, channel_id, token_id, `+"`group`"+`, created_at, content, other
		 FROM logs WHERE other LIKE ? ORDER BY id DESC LIMIT 20`,
		"%"+taskID+"%"))
	emit(out, "logs_recent_rh", queryRowList(ctx, db,
		`SELECT id, type, user_id, model_name, quota, channel_id, created_at, content, other
		 FROM logs WHERE model_name LIKE 'rh-%' OR content LIKE '%RunningHub%'
		 ORDER BY id DESC LIMIT 30`))
	emit(out, "rh_success_tasks", queryRowList(ctx, db,
		`SELECT id, task_id, user_id, channel_id, quota, status,
		        JSON_EXTRACT(private_data, '$.billing_context.model_price')     AS model_price,
		        JSON_EXTRACT(private_data, '$.billing_context.other_ratios')    AS other_ratios,
		        JSON_EXTRACT(private_data, '$.billing_context.per_call_billing') AS per_call,
		        JSON_EXTRACT(data, '$.usage.consumeCoins')                      AS consume_coins,
		        submit_time, finish_time
		 FROM tasks WHERE platform IN ('61','62','63') ORDER BY id DESC LIMIT 40`))
	emit(out, "all_apps", queryRowList(ctx, db,
		`SELECT id, name, upstream_id, site, published, per_call_billing, per_second_billing,
		        quota_per_second, fixed_quota_per_call, seconds_expr, model_base_rate_ratio
		 FROM apps ORDER BY id`))
	emit(out, "apps_row", queryRowList(ctx, db,
		`SELECT id, name, kind, upstream_id, site, published, per_call_billing, fixed_quota_per_call,
		        per_second_billing, quota_per_second, seconds_expr, model_base_rate_ratio, category_id
		 FROM apps WHERE upstream_id IN (
		     SELECT DISTINCT JSON_UNQUOTE(JSON_EXTRACT(properties, '$.origin_model_name')) FROM tasks WHERE task_id = ?
		 )`, taskID))
	emit(out, "options_pricing", queryOptionRows(ctx, db))
	emit(out, "channels_rh", queryRowList(ctx, db,
		`SELECT id, type, name, status, base_url, `+"`group`"+`, models
		 FROM channels WHERE type IN (61,62,63) ORDER BY id`))

	buf, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(buf))
}

// queryTask prints every column of the tasks row(s) matching the public task id
// (and, as a fallback, any task whose private_data mentions the upstream id).
func queryTask(ctx context.Context, db *sql.DB, taskID string) any {
	suffix := strings.TrimPrefix(taskID, "task_")
	cols := []string{"id", "task_id", "platform", "user_id", "channel_id", "quota", "action",
		"status", "fail_reason", "submit_time", "start_time", "finish_time", "progress",
		"properties", "private_data", "data"}
	rows, err := db.QueryContext(ctx,
		"SELECT "+strings.Join(cols, ", ")+" FROM tasks WHERE task_id = ? OR private_data LIKE ? LIMIT 5",
		taskID, "%"+suffix+"%")
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return scanRows(rows, cols)
}

func queryRowList(ctx context.Context, db *sql.DB, q string, args ...any) any {
	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	cols, _ := rows.Columns()
	return scanRows(rows, cols)
}

// queryOptionRows returns the pricing-related options so the base price of the
// model can be recomputed exactly as the host would.
func queryOptionRows(ctx context.Context, db *sql.DB) any {
	keys := []string{"ModelPrice", "ModelRatio", "GroupRatio", "GroupGroupRatio", "QuotaPerUnit",
		"Price", "USDExchangeRate", "CompletionRatio", "TaskPricePatches"}
	rows, err := db.QueryContext(ctx,
		"SELECT `key`, value FROM options WHERE `key` IN ("+placeholders(len(keys))+")", toAny(keys)...)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	defer rows.Close()
	result := map[string]string{}
	for rows.Next() {
		var k string
		var v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		// Model price tables are huge; keep only the interesting slice.
		val := v.String
		if len(val) > 4000 {
			val = trimToModel(val, "rh-") + " …(truncated)"
		}
		result[k] = val
	}
	return result
}

func trimToModel(jsonText, needle string) string {
	if i := strings.Index(jsonText, needle); i >= 0 {
		start := i - 200
		if start < 0 {
			start = 0
		}
		end := i + 2000
		if end > len(jsonText) {
			end = len(jsonText)
		}
		return "…" + jsonText[start:end]
	}
	return jsonText[:min(len(jsonText), 500)]
}

func placeholders(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

func toAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func scanRows(rows *sql.Rows, cols []string) any {
	defer rows.Close()
	var list []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return map[string]any{"scan_error": err.Error()}
		}
		row := map[string]any{}
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				row[c] = string(v)
			default:
				row[c] = v
			}
		}
		list = append(list, row)
	}
	return list
}

func emit(out map[string]any, name string, v any) {
	out[name] = v
	fmt.Fprintf(os.Stderr, "collected %s\n", name)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
