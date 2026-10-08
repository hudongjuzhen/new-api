// Command _probe_quota 是一个**只读**探针，用来回答一个具体的账目问题：
//
//	「世界解析那一次，用户的钱包到底被扣了多少 quota？」
//
// 它只做 SELECT，不改任何数据。之所以直接查库而不走 HTTP：站点侧的
// `secret` 在应用里是加密存的（settings.json 的 secrets{data,nonce,algo}），
// 拿不到明文 token；而 `.env` 里的 SQL_DSN 是本机自带的凭据。
//
// 用法（在 D:\work\new-api 下）：
//
//	go run .\_probe_quota\main.go
//
// 目录名以 `_` 开头，所以 `go build ./...` / `go test ./...` 不会看见它。
package main

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

// readDSN 从 .env 里取 SQL_DSN。**刻意不打印它**——里面有密码。
func readDSN(path string) string {
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Println("读不到 .env：", err)
		os.Exit(1)
	}
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "SQL_DSN=") {
			continue
		}
		v := strings.TrimPrefix(line, "SQL_DSN=")
		return strings.Trim(v, "'\"")
	}
	fmt.Println(".env 里没有 SQL_DSN")
	os.Exit(1)
	return ""
}

func main() {
	db, err := sql.Open("mysql", readDSN(`D:\work\new-api\.env`))
	if err != nil {
		fmt.Println("连不上：", err)
		os.Exit(1)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		fmt.Println("ping 失败：", err)
		os.Exit(1)
	}
	fmt.Println("== 已连上站点库（只读）==")

	// 0) logs 表有哪些列（后面按实际列名拼查询，避免猜错列）
	cols := map[string]bool{}
	if rows, err := db.Query("SHOW COLUMNS FROM logs"); err == nil {
		for rows.Next() {
			var field, typ string
			var null, key, extra sql.NullString
			var def sql.NullString
			if err := rows.Scan(&field, &typ, &null, &key, &def, &extra); err == nil {
				cols[field] = true
			}
		}
		rows.Close()
	}
	for _, want := range []string{"prompt_tokens", "completion_tokens", "quota", "content", "model_name"} {
		fmt.Printf("  列 %-18s %v\n", want, cols[want])
	}

	fmt.Println("\n== 1) 日志里带「世界解析」的行（插件自己记的那一笔）==")
	{
		rows, err := db.Query(`SELECT id, quota, model_name, created_at, LEFT(content, 200)
			FROM logs WHERE content LIKE '%世界解析%' ORDER BY id DESC LIMIT 10`)
		if err != nil {
			fmt.Println("  查询失败：", err)
		} else {
			n := 0
			for rows.Next() {
				var id int64
				var quota sql.NullInt64
				var model, content sql.NullString
				var created int64
				if err := rows.Scan(&id, &quota, &model, &created, &content); err == nil {
					n++
					fmt.Printf("  #%d quota=%d model=%s content=%s\n", id, quota.Int64, model.String, content.String)
				}
			}
			rows.Close()
			if n == 0 {
				fmt.Println("  （没有）")
			}
		}
	}

	fmt.Println("\n== 2) 那次解析窗口内(11:07~11:49)的中继扣费合计 ==")
	{
		tokenCols := ""
		if cols["prompt_tokens"] && cols["completion_tokens"] {
			tokenCols = ", SUM(prompt_tokens), SUM(completion_tokens)"
		}
		q := `SELECT COUNT(*), SUM(quota)` + tokenCols + ` FROM logs
			WHERE type = 2 AND created_at BETWEEN UNIX_TIMESTAMP('2026-10-08 11:07:00')
			                                  AND UNIX_TIMESTAMP('2026-10-08 11:49:00')`
		rows, err := db.Query(q)
		if err != nil {
			fmt.Println("  查询失败：", err)
		} else {
			for rows.Next() {
				var n int64
				var quota, p, c sql.NullInt64
				if tokenCols == "" {
					if err := rows.Scan(&n, &quota); err == nil {
						fmt.Printf("  条数=%d  quota 合计=%d\n", n, quota.Int64)
					}
				} else {
					if err := rows.Scan(&n, &quota, &p, &c); err == nil {
						fmt.Printf("  条数=%d  quota 合计=%d  prompt=%d  completion=%d\n",
							n, quota.Int64, p.Int64, c.Int64)
					}
				}
			}
			rows.Close()
		}
	}

	fmt.Println("\n== 3) 账号余额（积分 = quota/QuotaPerUnit*10）==")
	{
		rows, err := db.Query(`SELECT id, username, quota, used_quota, request_count FROM users WHERE id = 1`)
		if err != nil {
			fmt.Println("  查询失败：", err)
		} else {
			for rows.Next() {
				var id, quota, used, reqs sql.NullInt64
				var name sql.NullString
				if err := rows.Scan(&id, &name, &quota, &used, &reqs); err == nil {
					fmt.Printf("  user=%s(%d) 余额 quota=%d (%.6f 元 / %.4f 积分)  已用 quota=%d (%.4f 积分)  请求数=%d\n",
						name.String, id.Int64, quota.Int64,
						float64(quota.Int64)/500000, float64(quota.Int64)/500000*10,
						used.Int64, float64(used.Int64)/500000*10, reqs.Int64)
				}
			}
			rows.Close()
		}
	}
}
