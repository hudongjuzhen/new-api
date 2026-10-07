// 世界 IP · 能力授予工具（运维用）
//
// =========================================================================
// 它为什么存在
//
// 「世界 IP」的授予 / 撤销**只在插件自己的后台接口上**（`docs/23` §7），
// 而那个接口目前**没有网页界面** —— 后台那一屏是分期里还没做的一项
// （`docs/23` §12.12）。所以给账号开权限只有两条路：
//
//	1. 手敲 curl（要自己拼 JSON、自己对 200 里 `success:false` 的坑）
//	2. ★ **这个工具**
//
// 它把"我该怎么做"压成一条命令，并且**把每一步都打出来**：用了哪个账号、
// 授予了哪个能力、现在生效的有哪几个。
//
// =========================================================================
// 怎么用
//
// 先去站点后台拿一个 **访问令牌**（Access Token）——注意它**不是** API 令牌：
//
//	后台 → 个人设置 → 「安全」卡片 → 访问令牌 → 重新生成（会复制到剪贴板）
//
// 然后：
//
//	go run ./_scripts/world-entitlement --token <访问令牌>
//
// 它会自己查出你是哪个账号（`GET /api/user/self`），不需要你手填 user id。
//
// 常用变体：
//
//	--base https://你的站点          # 默认 https://www.aipole.top
//	--user-id 7                      # 给**别的**账号开（默认给令牌主人自己）
//	--only world-ip                  # 只授一个能力（默认两个都授）
//	--revoke                         # 改成撤销（验收判据 C：撤销之后立刻不能）
//	--list                           # 只看现在有哪些能力，什么都不改
//	--expires 2026-12-31             # 到期时间（不给 = 永不过期）
//	--dry-run                        # 只打印将要发什么，不发
//
// 凭据也可以走环境变量（**推荐**：命令行会进 shell 历史）：
//
//	$env:ZSY_WORLD_ADMIN_TOKEN = "……"      # PowerShell
//	export ZSY_WORLD_ADMIN_TOKEN=……        # bash
//	go run ./_scripts/world-entitlement
//
// =========================================================================
// 三个必须知道的坑（都在这份代码里处理掉了）
//
//  1. ★ **这一组接口出错时也返回 HTTP 200**（`common.ApiErrorMsg` 就是这么写的），
//     失败写在响应体的 `success:false` 里。所以判据**不能**只看状态码 ——
//     这个工具按 `success` 判，并把 `message` 原样打出来。
//
//  2. ★ **两个面吃两种凭据**（`world_test.go:288` 的注释写着这件事）：
//     - `/api/zsy/world/**`（op 面）吃**账号密钥**（relay 那一种，`sk-…`）
//     - `/dashboard/zsy/world/**`（后台面）吃**访问令牌**（本工具用的这一种）
//     拿账号密钥来跑这个工具会 401，而那句话是英文的（宿主的中间件说的），
//     所以这里提前判一次并说清该用哪一个。
//
//  3. ⚠ **撤销"没生效的能力"是失败，不是成功**：服务端会回
//     "没有可撤销的行"。那不是故障（它说明本来就没有），本工具照样如实打印。
//
// =========================================================================
// 安全
//
// ⚠ 令牌**只从命令行参数或环境变量读**，绝不写进任何文件、也不进日志。
// 打印时一律打码（只留末尾 4 位）。授予人的身份在**服务端**由
// `middleware.AdminAuth()` 判定 —— 这个工具只是把请求发出去，
// 它**自己不做任何鉴权判断**（没有令牌它就什么也做不了）。
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// 默认站点。与 `docs/23` §12.4.6.6 的真机验收用的是同一个。
const defaultBase = "https://www.aipole.top"

// 两个能力的名字。★ 它们是**契约字面量**（`zsy/world/models.go` 的
// `CapabilityWorldIP` / `CapabilityWorldIPAI`），不是这里发明的 —— 与
// `docs/23` §10 第 1 项那两个码逐字一致。
const (
	capabilityWorldIP   = "world-ip"
	capabilityWorldIPAI = "world-ip-ai"
)

const tokenEnv = "ZSY_WORLD_ADMIN_TOKEN"

func main() {
	var (
		base     = flag.String("base", defaultBase, "站点地址（不带尾斜杠）")
		token    = flag.String("token", "", "访问令牌（Access Token）；也可用环境变量 "+tokenEnv)
		userID   = flag.Int("user-id", 0, "给哪个账号开（0 = 令牌主人自己）")
		only     = flag.String("only", "", "只处理一个能力："+capabilityWorldIP+" / "+capabilityWorldIPAI)
		revoke   = flag.Bool("revoke", false, "改成撤销（默认是授予）")
		listOnly = flag.Bool("list", false, "只看现在有哪些能力，什么都不改")
		expires  = flag.String("expires", "", "到期日期 YYYY-MM-DD（不给 = 永不过期）")
		dryRun   = flag.Bool("dry-run", false, "只打印将要发什么，不发请求")
		timeout  = flag.Duration("timeout", 20*time.Second, "单次请求超时")
	)
	flag.Usage = usage
	flag.Parse()

	site := strings.TrimRight(strings.TrimSpace(*base), "/")
	key := strings.TrimSpace(*token)
	if key == "" {
		key = strings.TrimSpace(os.Getenv(tokenEnv))
	}
	if key == "" {
		die("没有凭据。\n\n"+
			"  这个工具要的是**访问令牌**（Access Token），不是 API 令牌：\n"+
			"    后台 → 个人设置 → 「安全」卡片 → 访问令牌 → 重新生成（会复制到剪贴板）\n\n"+
			"  两种给法（推荐环境变量：命令行会进 shell 历史）：\n"+
			"    go run ./_scripts/world-entitlement --token <访问令牌>\n"+
			"    $env:%s = \"……\"   # PowerShell\n"+
			"    export %s=……       # bash", tokenEnv, tokenEnv)
	}

	caps := []string{capabilityWorldIP, capabilityWorldIPAI}
	if want := strings.TrimSpace(*only); want != "" {
		if want != capabilityWorldIP && want != capabilityWorldIPAI {
			die("--only 只认 %q 与 %q（给的是 %q）", capabilityWorldIP, capabilityWorldIPAI, want)
		}
		caps = []string{want}
	}

	expiresAt, err := parseExpiry(*expires)
	if err != nil {
		die("%v", err)
	}

	client := &http.Client{Timeout: *timeout}
	fmt.Printf("站点：%s\n访问令牌：%s\n\n", site, mask(key))

	// ── 1. 我是谁 ────────────────────────────────────────────────────────
	me, err := fetchSelf(client, site, key)
	if err != nil {
		die("读不到自己的账号信息：%v\n\n"+
			"  ⚠ 如果你拿的是**账号密钥**（`sk-…`，用在 /api/ 那一面的那种），换它没用 ——\n"+
			"     后台这一面要的是**访问令牌**（个人设置 →「安全」→ 访问令牌）。", err)
	}
	target := *userID
	if target <= 0 {
		target = me.ID
	}
	who := fmt.Sprintf("账号 %d（%s，角色 %d）", me.ID, me.Username, me.Role)
	if target != me.ID {
		who = fmt.Sprintf("%s → **给另一个账号 %d** 操作", who, target)
	}
	fmt.Printf("我是：%s\n\n", who)

	// ── 2. 现在有什么 ────────────────────────────────────────────────────
	before, err := listEntitlements(client, site, key, target)
	if err != nil {
		die("读不到能力列表：%v", err)
	}
	fmt.Printf("现在生效的能力：%s\n", renderList(before.Active))
	for _, row := range before.Items {
		state := "生效中"
		switch {
		case row.RevokedAt != nil:
			state = fmt.Sprintf("已撤销（%s）", stamp(row.RevokedAt))
		case row.ExpiresAt != nil && *row.ExpiresAt <= time.Now().Unix():
			state = fmt.Sprintf("已过期（%s）", stamp(row.ExpiresAt))
		}
		fmt.Printf("  · %-12s %s  source=%s  添加于 %s\n",
			row.Capability, state, row.Source, stamp(&row.CreatedAt))
	}
	fmt.Println()

	if *listOnly {
		return
	}

	// ── 3. 改 ────────────────────────────────────────────────────────────
	action := "grant"
	if *revoke {
		action = "revoke"
	}
	failures := 0
	for _, cap := range caps {
		body := map[string]any{"user_id": target, "capability": cap}
		if action == "grant" && expiresAt != nil {
			body["expires_at"] = *expiresAt
		}
		if *dryRun {
			pretty, _ := json.Marshal(body)
			fmt.Printf("[dry-run] POST %s/dashboard/zsy/world/entitlements/%s  %s\n", site, action, pretty)
			continue
		}
		msg, ok, err := postEntitlement(client, site, key, action, body)
		switch {
		case err != nil:
			fmt.Printf("✗ %s %s：请求失败：%v\n", action, cap, err)
			failures++
		case !ok:
			/*
			 * ⚠ 服务端**故意**这样答（HTTP 200 + success:false）。最常见的两条是
			 * "没有可撤销的行"（本来就没有）与"未知能力名"（拼错了）。
			 */
			fmt.Printf("✗ %s %s：%s\n", action, cap, msg)
			failures++
		default:
			fmt.Printf("✓ %s %s\n", action, cap)
		}
	}
	if *dryRun {
		return
	}
	fmt.Println()

	// ── 4. 现在有什么（改完再看一遍，这才是判据） ─────────────────────────
	after, err := listEntitlements(client, site, key, target)
	if err != nil {
		die("改完了，但读不回列表（请手动确认）：%v", err)
	}
	fmt.Printf("现在生效的能力：%s\n", renderList(after.Active))

	/*
	 * ★ 最后这一句是**这次操作真正要回答的问题**："他能不能用了"。
	 * ⚠ 判权**每次现算**（`docs/23` §8.4），所以这里读到的就是下一个 op
	 * 会看到的 —— **不需要重启服务、不需要用户重新登录**。
	 */
	switch {
	case len(after.Active) == 0:
		fmt.Println("⇒ 这个账号现在**没有**任何世界能力。")
	case contains(after.Active, capabilityWorldIP) && contains(after.Active, capabilityWorldIPAI):
		fmt.Println("⇒ 完整可用：能读 / 建 / 改世界，也能让模型解析小说。客户端那一屏上应该已经没有红色提示了。")
	case contains(after.Active, capabilityWorldIP):
		fmt.Println("⇒ 只能读 / 建 / 改世界；**解析小说（ingest.run）还是会被拒**（那个要 world-ip-ai）。")
	default:
		fmt.Println("⇒ 只有 " + capabilityWorldIPAI + "：改世界的写操作会被拒（那个要 world-ip）。")
	}
	if failures > 0 {
		os.Exit(1)
	}
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

type selfUser struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Role     int    `json:"role"`
}

func fetchSelf(c *http.Client, site, key string) (*selfUser, error) {
	req, err := http.NewRequest(http.MethodGet, site+"/api/user/self", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	var out struct {
		Success bool     `json:"success"`
		Message string   `json:"message"`
		Data    selfUser `json:"data"`
	}
	if err := doJSON(c, req, &out); err != nil {
		return nil, err
	}
	if !out.Success || out.Data.ID <= 0 {
		if out.Message != "" {
			return nil, fmt.Errorf("%s", out.Message)
		}
		return nil, fmt.Errorf("站点没有返回账号信息（令牌无效？）")
	}
	return &out.Data, nil
}

// entitlementRow mirrors one row of the plugin's own list face. Only the fields
// this tool prints are decoded — it is a *reader*, not a second definition of
// the table.
type entitlementRow struct {
	Capability string `json:"capability"`
	Source     string `json:"source"`
	CreatedAt  int64  `json:"createdAt"`
	RevokedAt  *int64 `json:"revokedAt"`
	ExpiresAt  *int64 `json:"expiresAt"`
}

type entitlementList struct {
	Items  []entitlementRow `json:"items"`
	Active []string         `json:"active"`
}

func listEntitlements(c *http.Client, site, key string, userID int) (*entitlementList, error) {
	url := fmt.Sprintf("%s/dashboard/zsy/world/entitlements?user_id=%d", site, userID)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")

	var out struct {
		Success bool            `json:"success"`
		Message string          `json:"message"`
		Data    entitlementList `json:"data"`
	}
	if err := doJSON(c, req, &out); err != nil {
		return nil, err
	}
	if !out.Success {
		if out.Message != "" {
			return nil, fmt.Errorf("%s", out.Message)
		}
		return nil, fmt.Errorf("站点拒绝了这次读取")
	}
	return &out.Data, nil
}

// postEntitlement grants or revokes one capability.
//
// ★ It answers `ok` from the **body**, not the status code: this whole face
// returns HTTP 200 even when it refuses (`common.ApiErrorMsg`), so a status
// check would report every failure as a success.
func postEntitlement(c *http.Client, site, key, action string, body map[string]any) (string, bool, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return "", false, err
	}
	url := fmt.Sprintf("%s/dashboard/zsy/world/entitlements/%s", site, action)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return "", false, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	var out struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	if err := doJSON(c, req, &out); err != nil {
		return "", false, err
	}
	return out.Message, out.Success, nil
}

func doJSON(c *http.Client, req *http.Request, into any) error {
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	/*
	 * ⚠ 401 / 403 时响应体通常**不是**这几条接口的信封（是宿主中间件写的
	 * `{"success":false,"message":…}`，有时还是英文）。所以解码失败时要把
	 * 状态码与原文一起报出来 —— 否则这里会变成一句"JSON 解析失败"，
	 * 而真正的原因（拿错凭据 / 账号不是管理员）就看不见了。
	 */
	if err := json.Unmarshal(raw, into); err != nil {
		snippet := strings.TrimSpace(string(raw))
		if len(snippet) > 300 {
			snippet = snippet[:300] + "…"
		}
		return fmt.Errorf("HTTP %d，响应不是预期的 JSON：%s", resp.StatusCode, snippet)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

func parseExpiry(text string) (*int64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, nil
	}
	t, err := time.ParseInLocation("2006-01-02", text, time.Local)
	if err != nil {
		return nil, fmt.Errorf("--expires 要写成 YYYY-MM-DD（给的是 %q）", text)
	}
	// 当天 23:59:59 到期，而不是 00:00:00 —— 写"到 12-31"的人想的是那一天能用完。
	end := t.Add(24*time.Hour - time.Second).Unix()
	return &end, nil
}

func renderList(items []string) string {
	if len(items) == 0 {
		/* ⚠ 这句要说清"这不是故障" —— 它正是客户端那一屏会显示的东西 */
		return "（无）← 所以客户端那一屏会说「还没有这个能力」，那是**对的**"
	}
	return strings.Join(items, "、")
}

func contains(items []string, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}

func stamp(unix *int64) string {
	if unix == nil {
		return "—"
	}
	return time.Unix(*unix, 0).Format("2006-01-02 15:04")
}

// mask keeps the last 4 characters so an operator can tell two tokens apart
// without the log becoming a place a credential can be copied from.
func mask(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return "****" + token[len(token)-4:]
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "\n✗ "+format+"\n\n", args...)
	os.Exit(1)
}

func usage() {
	fmt.Fprintf(os.Stderr, `世界 IP · 能力授予工具

  go run ./_scripts/world-entitlement [选项]

凭据（要**访问令牌**，不是 API 令牌；见下面的说明）：
  --token <令牌>              也可用环境变量 %s
站点：
  --base <地址>               默认 %s
给谁 / 给什么：
  --user-id <id>              默认 = 令牌主人自己
  --only <能力>               %s 或 %s（默认两个都授）
  --expires YYYY-MM-DD        到期日（不给 = 永不过期）
做什么：
  --revoke                    改成撤销（验收判据 C：撤销之后立刻不能）
  --list                      只看现在有什么，什么都不改
  --dry-run                   只打印将要发什么

拿访问令牌：后台 → 个人设置 → 「安全」卡片 → 访问令牌 → 重新生成。
`, tokenEnv, defaultBase, capabilityWorldIP, capabilityWorldIPAI)
}
