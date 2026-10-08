package world

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// ★★ 签发一份"按账号绑定"的插件文件（docs/22 §2.3 / docs/23 §12.15）
//
// 后台点一下 → 按「某个账号 + 某个插件」产出一份 JSON → 发给那个人导入。
// 导入时客户端会核对账号，于是：
//
//	把给 A 的文件发给了 B  →  B 导入时**当场**被告知"这是给 zsy（ID 7）的"
//	                          而不是装上一看菜单有、点进去说"你没有能力"
//
// ⚠★ 这一块是「一致性」，**不是「安全」**。文件谁都能改，改了也确实绕不过去 ——
// 但**改了也没用**：真正的闸门是这里每次 op 现算的判据。所以 `check` 是防误改的。
//
// # 签发不改变任何授权
//
// ★ 它**只生成一个文件**。能力是 `zsy_world_entitlements` 里那些行说了算 ——
// 一个没被授予能力的账号**照样拿得到一份文件**（后台可以签），
// 而那份文件对他**一点用都没有**：op 会回 `E_ENTITLEMENT`。
//
// 这一条很重要，所以再写一遍：**签发 ≠ 授权**。所以下面的 `capabilities` 只是
// "这份文件上写着它配哪些能力"（给用户看的），而不是"他因此获得了什么"。
// 真正的授予走 `entitlements/grant`，那条路与这个文件无关。
// =========================================================================

// PluginEntitlementBlock is the `entitlement` block written into the signed file.
//
// ⚠ 字段名是**跨语言契约**（客户端 `src/core/plugin-entitlement.js` 读的就是它们）。
// 改任何一个字段名都要同时改客户端，否则症状是"每一份文件都被判成没有绑定信息"
// （那会让这条判据**静默失效**，比报错更难发现）。
type PluginEntitlementBlock struct {
	PluginID     string   `json:"pluginId"`
	UserID       int      `json:"userId"`
	Username     string   `json:"username"`
	Site         string   `json:"site"`
	Capabilities []string `json:"capabilities"`
	IssuedAt     string   `json:"issuedAt"`
	Check        string   `json:"check"`
}

// pluginIssueParams is the body of POST /dashboard/zsy/world/plugins/issue.
type pluginIssueParams struct {
	// UserID is the account this file is for. Required — a file for "nobody"
	// would be a file whose binding check can never pass.
	UserID int `json:"user_id"`
	// PluginID names the template (the file name under the templates directory).
	PluginID string `json:"plugin_id"`
	// Capabilities optionally overrides the template's `x-capabilities`.
	//
	// ⚠ 默认取模板里那一份：那是运营写下的"这个插件配哪些能力"，
	// 而**每次签发都手填**迟早会填歪（填歪的后果是文件上的能力列表与
	// 实际授予的对不上 —— 而客户端只是拿它显示，不会因此拒绝，所以错了也不报错）。
	Capabilities []string `json:"capabilities"`
}

// pluginTemplateView is one row of GET /dashboard/zsy/world/plugins.
type pluginTemplateView struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Capabilities []string `json:"capabilities"`
	// Visibility 是 `public` / `private`（`docs/27` §3）。★ 它必须回报：
	// 后台那一屏要显示"这份插件会不会出现在所有人的插件页里" ——
	// 而那是运营唯一能看出"我是不是把它设成了公共"的地方。
	Visibility string   `json:"visibility"`
	Screens    []string `json:"screens"`
	Problem    string   `json:"problem"`
	Source     string   `json:"source"`
}

// listPluginTemplates (GET /dashboard/zsy/world/plugins)
//
// ★ 有问题的模板**也列出来**，并把那句话带上（`problem`）—— 绝不静默。
// 一个不出现在列表里的模板会让运营以为"后台坏了"，而真相是那个文件里有个笔误。
func listPluginTemplates(c *gin.Context) {
	rows := ReloadPluginTemplates()

	items := make([]pluginTemplateView, 0, len(rows))
	for _, row := range rows {
		items = append(items, pluginTemplateView{
			ID:           row.ID,
			Name:         row.Name,
			Capabilities: nonNilStrings(row.Capabilities),
			Visibility:   row.Visibility,
			Screens:      templateScreenTitles(row.Manifest),
			Problem:      row.Problem,
			Source:       row.Source,
		})
	}
	common.ApiSuccess(c, gin.H{
		"items": items,
		// 目录也回给界面：运营拿了新模板要放进去，而"放哪儿"是那一屏唯一
		// 无法从列表本身推出来的信息。
		"directory": PluginTemplateDir(),
		// 已知的两个白名单也回给界面（写模板时要用）。
		"knownKinds":   knownPluginKinds(),
		"knownSources": knownPluginSources(),
	})
}

// issuePluginFile (POST /dashboard/zsy/world/plugins/issue)
//
// It answers the **file itself** (plus the suggested file name), not a link: the
// operator downloads it and sends it to the user, and nothing has to be stored
// server-side for that to work. That is deliberate — see plugin_template.go on
// why only the *source* template lives on disk.
func issuePluginFile(c *gin.Context) {
	var in pluginIssueParams
	if err := c.ShouldBindJSON(&in); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	if in.UserID <= 0 {
		common.ApiErrorMsg(c, "需要 user_id：这份文件是「给某个账号」签发的，没有账号就没有绑定信息。")
		return
	}

	pluginID := strings.TrimSpace(in.PluginID)
	if pluginID == "" {
		common.ApiErrorMsg(c, "需要 plugin_id：它对应模板目录里的哪一份模板（文件名去掉 .json）。")
		return
	}

	tpl, ok := PluginTemplateByID(pluginID)
	if !ok {
		known := templateIDsText()
		common.ApiErrorMsg(c, fmt.Sprintf("没有 %q 这份模板。%s", pluginID, known))
		return
	}
	if tpl.Problem != "" {
		/*
		 * ★ 把模板自己那句话原样透出去：它指名了哪个文件、哪一格写错了。
		 * 换一句"模板不可用"会让运营去猜。
		 */
		common.ApiErrorMsg(c, fmt.Sprintf("模板 %s 有问题，先修好它再签发：%s", pluginID, tpl.Problem))
		return
	}

	user, err := model.GetUserById(in.UserID, false)
	if err != nil || user == nil {
		common.ApiErrorMsg(c, fmt.Sprintf("找不到账号 %d", in.UserID))
		return
	}

	caps, err := resolveIssueCapabilities(in.Capabilities, tpl.Capabilities)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	/*
	 * ★ 站点取**这次请求**的 Host（与 `ingest.run` 的 api_base 同一条思路）：
	 * 它按构造就是"调用方访问的那个地址"，于是签出来的文件上写的站点与用户
	 * 实际登录的站点一致。写死一个域名会让换域名之后的文件全部报"站点不匹配"。
	 */
	site := requestSiteOrigin(c)
	if site == "" {
		common.ApiErrorMsg(c, "无法从这次请求推断站点地址（Host 头为空或不合法）—— 文件上那一格要用它")
		return
	}

	check := PluginCheck(pluginID, user.Id, user.Username, site)
	issuedAt := time.Now().UTC()
	block := PluginEntitlementBlock{
		PluginID:     pluginID,
		UserID:       user.Id,
		Username:     user.Username,
		Site:         site,
		Capabilities: caps,
		IssuedAt:     issuedAt.Format(time.RFC3339),
		Check:        check,
	}

	file, err := renderPluginFile(tpl.Manifest, block)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	/*
	 * ★ 版本取自**模板**（`manifest.version`），不是这里编一个。
	 *
	 * | 做法 | 后果 |
	 * |---|---|
	 * | 模板里的 version | 运营改模板时**顺手改它**，于是"v0.2.0 那份"与"v0.1.0 那份"在文件名与界面上分得开 |
	 * | 这里按签发次数自增一个号 | 那个号**不代表任何东西**：它既不是插件的版本，也与模板对不对得上无关 |
	 *
	 * ⚠ 文件里带版本这件事有它自己的用处：客户端那一屏会把 `version` 显示出来，
	 * 于是用户报问题时能说出"我装的是 v0.1.0" —— 而在那之前，
	 * "他手上是哪一版"是**查不出来**的（只能让他把文件发回来）。
	 */
	pluginVersion := stringField(tpl.Manifest, "version", "")
	name := pluginFileName(pluginID, pluginVersion, user.Username, user.Id, issuedAt)
	common.SysLog(fmt.Sprintf(
		"[zsy-world] issued plugin file plugin=%q to user=%d (%s) capabilities=%v",
		pluginID, user.Id, user.Username, caps))

	common.ApiSuccess(c, gin.H{
		"fileName": name,
		// `file` 是**文本**而不是嵌套对象：运营/用户拿到的是一个文件，
		// 而"再序列化一次"会让它变成另一种排版（缩进、键序），
		// 与"下载下来就是它"这件事脱钩。校验和只认内容里的那几格，不受排版影响，
		// 但**人**会拿它去比对，所以这里给的就是那份文本本身。
		"file":     file,
		"pluginId": pluginID,
		/* 模板里那个版本（可能为空 —— 旧模板没写）。界面用它显示"这份是哪一版"。 */
		"pluginVersion": pluginVersion,
		"userId":        user.Id,
		"username":      user.Username,
		"site":          site,
		"check":         check,
		// 文件上写着的能力。★ 它必须回报：界面要拿它与 `granted` 相比，
		// 才能说出那句"文件装得上、可是用不了，还差 X" —— 而那是这一屏最要紧的一句话。
		"capabilities": caps,
		// ⚠ 这一句要跟文件一起回去：运营最可能问的下一个问题是
		// "他手上那份还能用吗"，而答案取决于这两个能力有没有真的授予。
		"granted": grantedCapabilities(user.Id, caps),
	})
}

// resolveIssueCapabilities decides the capability list written into the file.
//
// The template's list wins by default; an explicit list replaces it. A capability
// name that the plugin does not know is refused here (rather than written into a
// file nobody validates) because `capabilities` is display-only on the client —
// a typo would travel silently.
func resolveIssueCapabilities(requested, fromTemplate []string) ([]string, error) {
	pick := requested
	if len(pick) == 0 {
		pick = fromTemplate
	}
	out := make([]string, 0, len(pick))
	seen := map[string]struct{}{}
	for _, raw := range pick {
		name := strings.TrimSpace(raw)
		if name == "" {
			continue
		}
		if !isKnownCapability(name) {
			return nil, fmt.Errorf("不认识的能力 %q（已知的是 %s）", name, knownCapabilitiesText())
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	/*
	 * ★★ 空列表是**合法**的（2026-…，`docs/27` §3）—— 这一条原来写的是
	 * 「这份文件要配至少一个能力」，而它现在会挡住一整类正当的插件。
	 *
	 * 「私有 + 一个能力都不要」是一种**明确的状态**：那份文件只是一份
	 * **按账号发的凭据**（"音频模式"那个界面插件就是这样 —— 它只画用户自己
	 * 磁盘上的音频工程，一个需要能力的 op 都不调；运营只是不想让它公开可装）。
	 * 拦下它 = 这类插件**只能**做成公共的，而那正是用户点名不要的形状。
	 *
	 * ⚠ 但"请求里给了名字、清洗之后一个不剩"仍然是错：那说明它写歪了
	 * （给的全是空白），而错误的那份文件照样会被客户端的账号核对放行 ——
	 * 静默地把一份"本来要配能力"的文件签成"不要能力"。
	 */
	if len(out) == 0 && len(requested) > 0 {
		return nil, fmt.Errorf("capabilities 里给的名字清洗之后一个都不剩（是不是全填成了空白？）")
	}
	return out, nil
}

// grantedCapabilities answers which of `want` the account holds **right now**.
//
// ★ It is recomputed per call and never cached (docs/23 §8.4), so the operator
// sees the same verdict the next op would.
func grantedCapabilities(userID int, want []string) []string {
	live, err := WorldCapabilitiesActive(userID)
	if err != nil {
		common.SysError(fmt.Sprintf("zsy-world: read capabilities of user %d failed: %v", userID, err))
		return []string{}
	}
	have := map[string]struct{}{}
	for _, c := range live {
		have[c] = struct{}{}
	}
	out := make([]string, 0, len(want))
	for _, c := range want {
		if _, ok := have[c]; ok {
			out = append(out, c)
		}
	}
	return out
}

// renderPluginFile writes the template out with the entitlement block added.
//
// ★ It keeps every field the template had (the client promises the same the other
// way round: "不认识的字段原样保留"). A field silently dropped here would show up
// as "the file I sent works, the one I just generated does not".
func renderPluginFile(manifest map[string]any, block PluginEntitlementBlock) (string, error) {
	out := make(map[string]any, len(manifest)+1)
	for k, v := range manifest {
		out[k] = v
	}
	/*
	 * ⚠ 拿掉宿主专用的那几格再写出去（清单在 `hostOnlyKeys` 里，**只有一份**）：
	 *
	 *	`x-capabilities`  是**给这个后台看的**（"这份插件要配哪些能力"），
	 *	                  它不属于插件格式；留着它会让每份签发的文件都带一格
	 *	                  客户端不认识的扩展，而"不认识的字段原样保留"意味着它会
	 *	                  被原样存进用户本机的插件表、再原样写回磁盘。
	 *	`x-visibility`    ★ 同理，而且更该拿掉：它是**服务端的分发策略**
	 *	                  （公共 / 私有），对一个已经拿到文件的用户没有任何意义。
	 *	                  留着它还会给"用户手改这一格"留一个诱人的错念 ——
	 *	                  那一格**从来不是**判据（判据在服务端那一份模板上）。
	 *	`entitlement`     若模板里手写了一份（不该有），必须被下面这份**覆盖**。
	 */
	for _, k := range hostOnlyKeys {
		delete(out, k)
	}
	out[PluginEntitlementField] = block

	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("world: encode plugin file: %w", err)
	}
	return string(raw) + "\n", nil
}

// pluginFileName suggests a download name.
//
// ★★ 名字里带三样东西，因为运营的硬盘上会同时躺着好几份看着一样的插件文件：
//
//	插件 id    —— 是哪一份插件
//	版本       —— **哪一版**（改过模板之后，旧文件与新文件必须能分辨）
//	账号与日期 —— 给谁的、什么时候签的
//
// 例：`world-ip-v0.1.0-zsy-user7-20261007.aimv-plugin.json`
//
// ⚠ 这个文件名**只是给人看的**：真正的判据全在文件**内容**里（`id` / `version` /
// `entitlement`）。所以这里怎么拼都不影响正确性 —— 但它影响"发错文件"的概率，
// 而发错文件正是这一整块要防的事。
func pluginFileName(pluginID, version, username string, userID int, issuedAt time.Time) string {
	parts := []string{pluginID}
	if safe := sanitizeFilePart(version); safe != "" {
		parts = append(parts, "v"+safe)
	}
	if safe := sanitizeFilePart(username); safe != "" {
		parts = append(parts, safe)
	}
	parts = append(parts, fmt.Sprintf("user%d", userID))
	parts = append(parts, issuedAt.Format("20060102"))
	return strings.Join(parts, "-") + ".aimv-plugin.json"
}

// sanitizeFilePart keeps a user name usable inside a file name.
//
// ⚠ It replaces path separators and control characters rather than rejecting the
// name: a username with a slash in it must still produce a **downloadable** file,
// and refusing would leave the operator with no way to issue one.
func sanitizeFilePart(name string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' ||
			r == '"' || r == '<' || r == '>' || r == '|':
			b.WriteRune('_')
		case r < 0x20:
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	out := strings.TrimSpace(b.String())
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

// requestSiteOrigin answers the site address **from this request**, same idea as
// the ingest gateway base: the hostname the caller reached is by construction one
// that routes here, so a generated file carries the address the user actually uses.
//
// ⚠ Unlike the gateway base it does **not** append `/v1`: this value goes into the
// file as "which site is this file for" and is compared against the account's own
// base address (`AccountView.base`), which has no path.
func requestSiteOrigin(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	host := strings.TrimSpace(c.Request.Host)
	if host == "" || strings.ContainsAny(host, " \t/\\") {
		return ""
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if scheme == "http" &&
		strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}
	return scheme + "://" + host
}

// templateScreenTitles lists a template's screen titles (for the admin list).
func templateScreenTitles(manifest map[string]any) []string {
	screens, ok := manifest["screens"].([]any)
	if !ok {
		return []string{}
	}
	out := make([]string, 0, len(screens))
	for _, raw := range screens {
		s, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, stringField(s, "title", stringField(s, "id", "")))
	}
	return out
}

// templateIDsText names the templates that are available, for an error message.
func templateIDsText() string {
	rows := ReloadPluginTemplates()
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Problem == "" {
			ids = append(ids, row.ID)
		}
	}
	if len(ids) == 0 {
		return fmt.Sprintf("模板目录 %s 里还没有一份能用的模板。", PluginTemplateDir())
	}
	return "现有的是：" + strings.Join(ids, " / ")
}
