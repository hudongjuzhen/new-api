package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// resurrect-rh-tasks re-arms old RunningHub tasks that were wrongly marked
// FAILURE ("任务超时（1440分钟）") by the legacy timeout sweep. It:
//
//  1. Reads data.taskId (the upstream RH task id) back into private_data so
//     the poller can query it (GetUpstreamTaskID() prefers private_data,
//     falling back to task_id).
//  2. Clears fail_reason and flips status back to IN_PROGRESS so the poller's
//     per-platform timeout exemption (61/62/63) keeps them in-flight and the
//     upstream SUCCESS/FAILED/CANCELED result settles them normally.
//
// It only touches RunningHub-family tasks that are FAILURE with the legacy
// timeout reason — never non-timeout failures, never other platforms.
//
// Usage: go run ./_scripts/clean_rh_urls/dump.go
func main() {
	dsn := os.Getenv("SQL_DSN")
	if dsn == "" {
		log.Fatal("SQL_DSN not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(2 * time.Minute)
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	rows, err := db.QueryContext(ctx,
		`SELECT id, data, fail_reason FROM tasks
		 WHERE platform IN ('61','62','63')
		   AND status = 'FAILURE'
		   AND fail_reason LIKE '任务超时（1440分钟）%'
		   AND data IS NOT NULL AND data <> ''`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	type taskRow struct {
		id        int64
		upstream  string
		fail      string
	}
	var all []taskRow
	for rows.Next() {
		var id int64
		var data []byte
		var fail sql.NullString
		if err := rows.Scan(&id, &data, &fail); err != nil {
			log.Printf("scan id=%d: %v", id, err)
			continue
		}
		var payload struct {
			TaskID string `json:"taskId"`
		}
		if err := json.Unmarshal(data, &payload); err != nil || payload.TaskID == "" {
			log.Printf("id=%d has no upstream taskId, skipping (data=%s)", id, string(data))
			continue
		}
		all = append(all, taskRow{id: id, upstream: payload.TaskID, fail: fail.String})
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("found %d legacy timeout RH tasks to resurrect\n", len(all))
	done := 0
	for _, r := range all {
		// Fetch current private_data so we don't clobber other fields.
		var cur []byte
		if err := db.QueryRowContext(ctx, `SELECT private_data FROM tasks WHERE id = ?`, r.id).Scan(&cur); err != nil {
			log.Printf("id=%d read private_data: %v", r.id, err)
			continue
		}
		var pd map[string]any
		if len(cur) > 0 {
			if err := json.Unmarshal(cur, &pd); err != nil {
				log.Printf("id=%d parse private_data: %v", r.id, err)
				continue
			}
		}
		if pd == nil {
			pd = map[string]any{}
		}
		pd["upstream_task_id"] = r.upstream
		pd["result_url"] = "" // don't leave a stale result url on a resurrected task
		newPD, _ := json.Marshal(pd)

		// Also refresh submit_time to now: the timeout sweep compares
		// submit_time against its cutoff, and these resurrected tasks still
		// carry an old submit_time. Without this they are immediate timeout
		// candidates again on the very next sweep (even though the platform
		// exemption would skip them, it keeps them dirty). Setting it to now
		// makes them behave like freshly-submitted tasks.
		now := time.Now().Unix()
		res, err := db.ExecContext(ctx,
			`UPDATE tasks
			 SET status = 'IN_PROGRESS', progress = '50%',
			     fail_reason = '', private_data = ?, finish_time = 0,
			     submit_time = ?
			 WHERE id = ? AND status = 'FAILURE'`,
			string(newPD), now, r.id)
		if err != nil {
			log.Printf("id=%d update: %v", r.id, err)
			continue
		}
		if n, _ := res.RowsAffected(); n != 1 {
			log.Printf("id=%d updated %d rows (expect 1)", r.id, n)
			continue
		}
		log.Printf("id=%d resurrected upstream=%s", r.id, r.upstream)
		done++
	}
	fmt.Printf("done, resurrected %d tasks\n", done)
}