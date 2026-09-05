package main

import (
	"context"
	"database/sql"
	"log"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// verify prints current state of the 6 RunningHub tasks.
func main() {
	dsn := os.Getenv("SQL_DSN")
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(2 * time.Minute)
	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	rows, err := db.QueryContext(ctx,
		`SELECT id, task_id, platform, status, progress, fail_reason, submit_time, finish_time, private_data
		 FROM tasks WHERE platform IN ('61','62','63') ORDER BY id`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var taskID, platform, status, progress, failReason, privateData string
		var submitTime, finishTime int64
		if err := rows.Scan(&id, &taskID, &platform, &status, &progress, &failReason, &submitTime, &finishTime, &privateData); err != nil {
			log.Printf("scan: %v", err)
			continue
		}
		log.Printf("id=%d task=%s platform=%s status=%s progress=%s fail=%q submit=%d finish=%d private=%s", id, taskID, platform, status, progress, failReason, submitTime, finishTime, privateData)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}