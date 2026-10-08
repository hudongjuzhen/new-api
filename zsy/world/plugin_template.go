package world

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// =========================================================================
// ★★ 插件模板：服务端按账号签发一份插件文件（docs/22 §2.3 / docs/23 §12.15）
//
// 后台点一下「生成插件文件」→ 按「某个账号 + 某个插件」产出一份 JSON →
// 发给那个人导入。导入时客户端会核对账号，于是"我给了谁"有一个**看得见的凭据**，
// 而"把给 A 的文件发给了 B"会在**导入那一刻**被说出来。
//
// ⚠★ 这一块是「一致性」，**不是「安全」**。文件谁都能改，改了也确实绕不过去 ——
// 但**改了也没用**：真正的闸门是这里每次 op 现算的判据（docs/23 §8.3 判据 ①）。
// 所以校验和是**防误改**的（算法公开、无密钥），不要给它加"签名"来买安全感。
//
// # ★ 为什么"模板"是**源**文件，而不是"每一份签发的文件"
//
// 每份签发出去的文件里都有**这个账号的用户名**，所以它们各不相同 ——
// 把生成物落盘就等于"一个人一个文件"，很快就会变成一堆没人敢删的垃圾。
//
// 所以磁盘上只放**源**：里面**没有** `entitlement`，也没有 `check`。
// 签发时把模板读进来，填上 `entitlement` 并**现算** `check`。
//
// | 好处 | 说明 |
// |---|---|
// | 运营只维护一份 | 改插件的屏 = 改那一个 JSON，下一次签发自动作数 |
// | 没有生成物垃圾 | 磁盘上永远只有源文件 |
// | ★ **没有第二处要同步改的东西** | 若把 `check` 也写进模板，运营每改一个字都要手算一遍哈希 —— 那必然会忘，而忘了的症状是**每一份文件都报"被改过"**，看起来像文件坏了 |
//
// # 放哪儿 / 长什么样
//
// 目录由 `ZSY_WORLD_PLUGIN_TEMPLATES` 指定，默认 `<工作目录>/plugin-templates`
// （与 `prompts/`、`schema/` 那些同级）。一个模板 = 一份 `.json`，**文件名就是插件 id**：
//
//	plugin-templates/world-ip.json  →  id 必须是 "world-ip"
//
// ⚠ 文件名与 `id` 必须一致，启动时**逐份校验**：不一致的模板会被**列出来并说明**
// （绝不静默 —— 一个不出现在列表里的模板会让运营以为"后台坏了"）。
//
// # 能力那一格也在模板里（`x-capabilities`）
//
// 模板要声明"它需要哪些能力"，因为世界上不止一种能力（`world-ip` 与
// `world-ip-ai`），而"这个插件该配哪个"是**运营的知识**，不是代码能推的。
// 它用 `x-` 前缀：那是 JSON Schema 给"扩展关键字"的惯例，写明"这一格是给宿主用的，
// 不属于插件格式本身"（客户端会把它当不认识的字段**原样保留**，不会报错）。
// =========================================================================

// PluginTemplate 是一份**源**模板（磁盘上那一份）。
type PluginTemplate struct {
	// ID 是插件的 id，必须与文件名一致。
	ID string `json:"id"`
	// Name 是界面上显示的名字。
	Name string `json:"name"`
	// Capabilities 是"这份插件要配哪些能力"（见上面 `x-capabilities` 那段）。
	Capabilities []string `json:"x-capabilities"`
	// Visibility 是"这份插件怎么发出去"：`public`（公共目录，一键可装）或
	// `private`（只能按账号签发）。见上面 `x-visibility` 那一整段。
	//
	// ⚠ 它**不进插件文件**（`hostOnlyKeys` 会在两个出口都把它拿掉）：
	// 客户端不需要、也不该拿这个值做任何判断 —— 能不能装是**服务端**决定的事。
	Visibility string `json:"x-visibility"`

	// Manifest 是模板**原样**那一份（含上面那三格，它们是插件格式允许的额外字段）。
	//
	// ⚠ 留着原样是刻意的：签发时要把整份摊回文件里，而"读一遍再写回去"不许丢字段
	// ——与客户端 `validatePlugin` 那条"不认识的字段原样保留"是同一条纪律。
	Manifest map[string]any `json:"-"`

	// Problem 非空表示这份模板有问题（装配失败）。列表里会**列出来并说明**。
	Problem string `json:"problem"`
	// Source is the file it came from (operator-facing, for "which file do I edit").
	Source string `json:"source"`
}

// pluginTemplateDirEnv names the templates directory.
const pluginTemplateDirEnv = "ZSY_WORLD_PLUGIN_TEMPLATES"

// pluginTemplateDirDefault is relative to the process working directory, like the
// other deployment-provided trees (`prompts/`, `schema/`).
const pluginTemplateDirDefault = "plugin-templates"

// templateCapabilitiesKey is the extension key carrying the capability list.
const templateCapabilitiesKey = "x-capabilities"

// =========================================================================
// ★★ 可见性：这份模板是「公共」还是「私有」（docs/27 §3）
//
// 用户 2026-… 的要求原话：
//
//	"这里也有两类，这两类的选择，在服务器端，一类是公共类型……能够直接在插件
//	 这里看到，能够直接点击一键安装，任何账号都能直接一键安装，另一类是私有模式
//	 ……需要后台下载证书提供。"
//
// | 取值 | 客户端怎么拿到它 | 谁决定 |
//	|---|---|---|
//	| `public`  | ★ 插件页里列出来，任何人点一下「一键安装」（`/api/zsy/plugins`） | 运营写在模板里 |
//	| `private` | 只有**后台按账号签发**的那一份文件（`/dashboard/zsy/world/plugins/issue`） | 同上 |
//
// ⚠★ **默认是 `private`，不是 `public`** —— 这一条是刻意的，而且方向不能反：
// 本格子是后加的，磁盘上那些**写于它出现之前**的模板（`world-ip.json` 就是）
// 一份 `x-visibility` 都没有。默认成 `public` 的话，它们会在升级那一刻
// 悄悄变成"任何账号一键可装" —— 而 `world-ip` 是**付费能力**的插件，
// 那正是它绝不该发生的事（见 `plugin_issue.go` 文件头："签发 ≠ 授权"，
// 但一封公开发出去的文件会让"这个能力存在"这件事不再受运营控制）。
//
// ⚠ 它与 `x-capabilities` 是**两件不同的事**，别合并：
//
//	`x-capabilities`  这份插件**配哪些能力**（给后台看的，写进签发文件的 entitlement）
//	`x-visibility`    ★ 这份插件**怎么发出去**（公共目录 / 按账号签发）
//
// 一个 `public` 插件同样可以带能力（装上不等于有权用）—— 那正是世界 IP 的设计。
// =========================================================================

// templateVisibilityKey is the extension key carrying the visibility class.
const templateVisibilityKey = "x-visibility"

// Visibility classes.
const (
	// VisibilityPublic — listed by GET /api/zsy/plugins, one-click installable by
	// any account.
	VisibilityPublic = "public"
	// VisibilityPrivate — only reachable as a file issued for one account.
	VisibilityPrivate = "private"
)

// knownVisibilities is the vocabulary, in the order the admin UI offers it.
func knownVisibilities() []string { return []string{VisibilityPublic, VisibilityPrivate} }

func isKnownVisibility(v string) bool {
	for _, k := range knownVisibilities() {
		if k == v {
			return true
		}
	}
	return false
}

// hostOnlyKeys are the extension keys that belong to **this backend** and must
// never travel into a plugin file the user imports.
//
// ⚠★ One list, two renderers (the issued file and the published catalog).
// Two lists would drift, and the symptom of a drift is "the file I downloaded
// carries a field the file I was sent does not" — which nobody would think to
// look for. The client stores unknown fields verbatim (docs/22 §2.1), so a
// leaked key would be persisted and written back forever.
var hostOnlyKeys = []string{templateCapabilitiesKey, templateVisibilityKey}

// templateVisibility reads `x-visibility` and normalises it.
//
// ⚠ Missing / empty means **private** — the reasoning is in the block above,
// and it is the one place that decision is allowed to live.
func templateVisibility(manifest map[string]any) (string, error) {
	raw, ok := manifest[templateVisibilityKey]
	if !ok || raw == nil {
		return VisibilityPrivate, nil
	}
	name := strings.TrimSpace(fmt.Sprint(raw))
	if name == "" {
		return VisibilityPrivate, nil
	}
	if !isKnownVisibility(name) {
		return "", fmt.Errorf("%s 里是不认识的值 %q（能写的是 %s）—— "+
			"它决定这份插件是「任何账号都能一键安装」还是「只能由后台按账号签发」，"+
			"写错了会让一份付费插件变成公开可装。",
			templateVisibilityKey, name, strings.Join(knownVisibilities(), " / "))
	}
	return name, nil
}

// IsPublic answers whether a template is published in the public catalog.
//
// ⚠ It is a named predicate rather than an inline `== VisibilityPublic` at the
// call site: the catalog filter is the **only** thing standing between a paid
// plugin and "any account can install it", so it gets a name that a test can
// point at.
func (t *PluginTemplate) IsPublic() bool {
	return t != nil && t.Visibility == VisibilityPublic && t.Problem == ""
}

// templateCache holds the loaded templates, keyed by plugin id.
//
// ⚠ It is a cache **of files on disk**, and it is deliberately invalidated by
// hand (`ReloadPluginTemplates`). Why not re-read on every request: the file set
// changes only when an operator adds or edits one, so re-reading per request would
// buy nothing and make "which template is live" depend on the moment — while the
// cost of a stale cache is visible and fixable (the admin page has a 重新读取 button,
// and the operator who just dropped a file in is exactly the one looking at it).
var (
	templateMu    sync.RWMutex
	templateCache map[string]*PluginTemplate
	templateDir   string
)

// PluginTemplateDir answers the directory template files are read from.
func PluginTemplateDir() string {
	templateMu.RLock()
	defer templateMu.RUnlock()
	if templateDir != "" {
		return templateDir
	}
	return resolveTemplateDir()
}

// resolveTemplateDir reads the env var, falling back to the default.
//
// ⚠ An env value is used **verbatim** (absolute or relative to the working
// directory) — same convention as `ZSY_WORLD_VALIDATOR_SVC`.
func resolveTemplateDir() string {
	if p := strings.TrimSpace(common.GetEnvOrDefaultString(pluginTemplateDirEnv, "")); p != "" {
		return p
	}
	return pluginTemplateDirDefault
}

// LoadPluginTemplates reads every template and caches the result.
//
// It never returns an error: a bad template is reported **per template** (in the
// list the admin page shows), because refusing to load the whole set would hide
// the good ones behind one bad file — and the operator needs to see which file is
// wrong.
func LoadPluginTemplates() []*PluginTemplate {
	dir := resolveTemplateDir()

	rows := make([]*PluginTemplate, 0, 8)
	entries, err := os.ReadDir(dir)
	if err != nil {
		/*
		 * 目录不存在**不是错误**：一个还没配过模板的部署照样应当能打开那一屏
		 * （它会显示"目录里还没有模板"+ 目录路径，让运营知道该往哪儿放）。
		 */
		templateMu.Lock()
		templateCache = map[string]*PluginTemplate{}
		templateDir = dir
		templateMu.Unlock()
		return rows
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".json") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)

	next := make(map[string]*PluginTemplate, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		wantID := strings.TrimSuffix(name, filepath.Ext(name))
		rows = append(rows, loadOneTemplate(path, wantID, next))
	}

	templateMu.Lock()
	templateCache = next
	templateDir = dir
	templateMu.Unlock()
	return rows
}

// ReloadPluginTemplates forces a re-read (the admin page's 重新读取 button).
func ReloadPluginTemplates() []*PluginTemplate { return LoadPluginTemplates() }

// loadOneTemplate reads and validates one file, registering it in `into` on success.
func loadOneTemplate(path, wantID string, into map[string]*PluginTemplate) *PluginTemplate {
	row := &PluginTemplate{ID: wantID, Name: wantID, Source: path}

	raw, err := os.ReadFile(path)
	if err != nil {
		row.Problem = fmt.Sprintf("读不到这个文件：%v", err)
		return row
	}

	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		row.Problem = fmt.Sprintf("不是合法的 JSON：%v", err)
		return row
	}

	row.Manifest = manifest
	row.Name = stringField(manifest, "name", wantID)

	// ★ 文件名与 id 必须一致 —— 否则"该改哪个文件"就有两个答案。
	id := strings.TrimSpace(stringField(manifest, "id", ""))
	if id == "" {
		row.Problem = fmt.Sprintf("这份模板没有写 id。文件名是 %s.json，所以它的 id 应当是 %q。", wantID, wantID)
		return row
	}
	if id != wantID {
		row.Problem = fmt.Sprintf("文件名与 id 不一致：文件名说 %q，而里面的 id 是 %q。"+
			"请把文件名改成 %s.json（或改掉 id）—— 文件名就是「该改哪个文件」的唯一答案。", wantID, id, id)
		return row
	}

	// 形状：至少要有一屏，且 kind/source 都在白名单里。
	//
	// ⚠ 这里**不重复**那些判据：真正的校验器在**客户端**（`validatePlugin`），
	// 而这一层的职责是"别把一份明显不成形的东西签出去"。所以只判三件客户端也会判、
	// 而且判错了代价最大的：有 id、有 screens、每屏有 kind 与 source。
	if problem := templateShapeProblem(manifest); problem != "" {
		row.Problem = problem
		return row
	}

	caps, capsWritten, err := templateCapabilities(manifest)
	if err != nil {
		row.Problem = err.Error()
		return row
	}
	/*
	 * ⚠★ 判据是"这一格**写没写**"，不是"里面有没有东西"（2026-…，`docs/27` §2）。
	 *
	 * 写这一格这件事本身有信息量：它是"这份插件配哪些能力"的唯一来源，而能力名
	 * 是运营的知识、代码推不出来。**没写** = 作者漏了（拦下来）。
	 * **写了、是空的** = ★ 作者明确说了"这份插件不需要任何能力" —— 那是一种
	 * 合法的状态（"视频模式"那一类工程界面插件就是这样：它只画用户自己磁盘上的
	 * 工程，一个需要能力的 op 都不调）。
	 *
	 * ⚠ 反过来说：**不许**把"没写"也当成"不需要" —— 那样一份本来要配
	 * `world-ip` 的模板漏写一格之后会**静默地**变成"什么能力都不要"，
	 * 而它签出去的文件上那一格就是空的（客户端只拿它显示，不会因此拒绝），
	 * 于是没有任何地方会报错。
	 */
	if !capsWritten {
		row.Problem = fmt.Sprintf("这份模板没写 %q —— 它是「这份插件要配哪些能力」的唯一来源，"+
			"而能力名是运营的知识、代码推不出来。请写下它需要的能力（例如 [%q]）；"+
			"如果它确实一个能力都不要，就写一个空数组 []（那是有意的声明，与「漏写」是两件事）。",
			templateCapabilitiesKey, CapabilityWorldIP)
		return row
	}
	for _, c := range caps {
		if !isKnownCapability(c) {
			row.Problem = fmt.Sprintf("%s 里有不认识的能力 %q（已知的是 %s）",
				templateCapabilitiesKey, c, knownCapabilitiesText())
			return row
		}
	}
	row.Capabilities = caps

	/*
	 * ★ 可见性（`x-visibility`）。它**排在能力之后**：能力那一格缺席是**错误**
	 * （见上面那段 —— 能力名是运营的知识，代码推不出来），而可见性缺席是
	 * **有默认值的**（`private`）。先报那个真错，别让一句"没写可见性"
	 * 把更值钱的那句盖掉。
	 */
	vis, err := templateVisibility(manifest)
	if err != nil {
		row.Problem = err.Error()
		return row
	}
	row.Visibility = vis

	if _, dup := into[wantID]; dup {
		row.Problem = fmt.Sprintf("有两份模板都叫 %q", wantID)
		return row
	}
	into[wantID] = row
	return row
}

// templateShapeProblem answers "" when the manifest is shaped like a plugin.
func templateShapeProblem(manifest map[string]any) string {
	if got := strings.TrimSpace(stringField(manifest, "format", "")); got != "aimv-plugin" {
		return fmt.Sprintf("format 应当是 \"aimv-plugin\"（现在是 %q）", got)
	}
	screens, ok := manifest["screens"].([]any)
	if !ok || len(screens) == 0 {
		return "screens 至少要有一屏 —— 一个不带屏的插件装上之后什么也不会出现。"
	}
	for i, raw := range screens {
		s, ok := raw.(map[string]any)
		if !ok {
			return fmt.Sprintf("第 %d 屏不是一个对象。", i+1)
		}
		if strings.TrimSpace(stringField(s, "id", "")) == "" {
			return fmt.Sprintf("第 %d 屏没有写 id。", i+1)
		}
		kind := strings.TrimSpace(stringField(s, "kind", ""))
		if !isKnownPluginKind(kind) {
			return fmt.Sprintf("第 %d 屏的 kind 是 %q，本应用不认识（已知的是 %s）。",
				i+1, kind, strings.Join(knownPluginKinds(), " / "))
		}
		/*
		 * ★★ `kind: "engineering"` 那一支（`docs/27` §2）：它**不读任何目录**，
		 * 改要一格 `medium`（"我是哪一家的工程界面"）。
		 *
		 * ⚠ 这一支**必须在 source 那一关之前**：让它也去过 `source` 只会有
		 * 一个下场 —— 每一份正确的工程界面模板都被判"没有写 source"，
		 * 而运营手里那份文件完全是对的（他也说不清该填哪个目录，因为这一屏
		 * 压根不取目录）。与客户端 `plugin-manifest.js` 里那段逐字同形。
		 */
		if kind == pluginKindEngineering {
			medium := strings.TrimSpace(stringField(s, "medium", ""))
			if !isKnownPluginMedium(medium) {
				return fmt.Sprintf("第 %d 屏的 medium 是 %q，本应用不认识（能写的是 %s）—— "+
					"kind 是 engineering 的屏要说明自己是哪一家的工程界面。",
					i+1, medium, strings.Join(knownPluginMediums(), " / "))
			}
			continue
		}
		source := strings.TrimSpace(stringField(s, "source", ""))
		if source == "" {
			return fmt.Sprintf("第 %d 屏没有写 source。", i+1)
		}
		/*
		 * ★ 这一条是"别签出一份客户端一定装不上的模板"里最值钱的一条：
		 * 数据源是宿主侧的白名单，写错一个字母**导入就会被拒**，而运营手里
		 * 只有一份"看起来完全正常"的 JSON。
		 */
		if !isKnownPluginSource(source) {
			return fmt.Sprintf("第 %d 屏的 source 是 %q，本应用不认识（已知的是 %s）。"+
				"数据源是宿主实现好的接口，写错了客户端会拒绝导入。",
				i+1, source, strings.Join(knownPluginSources(), " / "))
		}
	}
	return ""
}

// templateCapabilities reads `x-capabilities` and normalises it.
//
// ★ 三个返回值：清洗后的名字、**这一格写没写**、错误。
//
// ⚠★ 中间那一个是 2026-…（`docs/27` §2）新加的，而它买的正是"空数组"这一种
// 写法：调用方要能区分「**漏写**了这一格」（拦下来）与「**写了 `[]`**」
// （作者明确声明"这份插件不需要任何能力"）。只看 `len(caps)==0` 的话，
// 两种情形长得一模一样，而它们的处置完全相反 —— 见 `loadOneTemplate`。
func templateCapabilities(manifest map[string]any) ([]string, bool, error) {
	raw, ok := manifest[templateCapabilitiesKey]
	if !ok || raw == nil {
		return nil, false, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, true, fmt.Errorf("%s 应当是一个字符串数组", templateCapabilitiesKey)
	}
	out := make([]string, 0, len(list))
	seen := map[string]struct{}{}
	for _, item := range list {
		name := strings.TrimSpace(fmt.Sprint(item))
		if name == "" {
			continue
		}
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	/*
	 * ⚠ 空的那一份要返回**非 nil** 的空切片（`make` 已经保证了）：调用方若用
	 * `caps == nil` 去判"漏写"，返回 nil 会让"写了 []"与"漏写"又混成一种。
	 * 现在这个区分由第二个返回值明确给出，这一条只是不去破坏它。
	 */
	return out, true, nil
}

// PluginTemplateByID answers one template by id, reading the file every time.
//
// ★★ **判据是磁盘上有没有这个文件**，不查缓存。这不是防御性编程，是一个实测过的
// 教训 —— 第一版从缓存里取，而它在**测试**里串了味：
//
//	`PluginTemplateByID("world-ip")` 说"没有这份模板"，
//	而同一个函数的错误分支 `templateIDsText()` 却能列出 `world-ip`。
//	两个函数对同一份文件给出了**相反的结论**。
//
// 机制是两条路读**不同的东西**：`PluginTemplateByID` 取的是缓存快照，
// 而 `templateIDsText()` 走 `ReloadPluginTemplates()`（现读）。缓存是模块级的，
// 每个用例给不同的临时目录，于是一个用例里"缓存里那份"与"现在这个目录里的那份"
// 可以不是同一份 —— 这正是"**同一个问题有两个答案**"的经典形状。
//
// 所以现在只有一个答案：文件名就是 id，那个文件在不在，就是有没有这份模板。
// 缓存降级成纯粹的**列表**用途（后台那一屏的展示），它读漏了不影响签发。
//
// 代价是每次签发多一次 `os.ReadFile` —— 而签发是人点一下的动作，不是热路径。
func PluginTemplateByID(id string) (*PluginTemplate, bool) {
	dir := resolveTemplateDir()
	path := filepath.Join(dir, id+".json")

	if _, err := os.Stat(path); err != nil {
		return nil, false
	}

	into := map[string]*PluginTemplate{}
	row := loadOneTemplate(path, id, into)
	return row, true
}

// stringField reads one string field without panicking on odd shapes.
func stringField(obj map[string]any, key, fallback string) string {
	raw, ok := obj[key]
	if !ok || raw == nil {
		return fallback
	}
	if s, ok := raw.(string); ok {
		if strings.TrimSpace(s) == "" {
			return fallback
		}
		return s
	}
	return fallback
}

// ---------------------------------------------------------------------------
// 插件格式里那两个白名单
// ---------------------------------------------------------------------------
//
// ⚠★ 这两份**抄自客户端**（`src/core/plugin-manifest.js` 的 `KINDS` 与
// `SOURCE_FIELDS` 的键）。抄一份的理由只有一个：签出"客户端一定装不上"的模板
// 是最坏的一种坏法（运营点了生成、发给用户、用户导入被拒，而运营完全不知道为什么）。
//
// ⚠ 它们是**第二处定义**，所以有一条纪律：客户端加了新 kind / 新 source 时，
// 这里要跟着加。判据是——**多出来的那一份只允许"更保守"**（宁可这里拒掉一份
// 其实能装的模板，也不要签出一份装不上的）。真正说了算的仍然是客户端。

// knownPluginKinds mirrors the client's `KINDS`.
//
// ★ `engineering` 是 2026-…（`docs/27` §2）加的第三格：**某一介质的工程界面**
// （"视频模式 / 音频模式 / 文本模式"那三份）。它不读目录、不要 `card`，
// 只要一格 `medium`。
func knownPluginKinds() []string { return []string{"catalog", "world", pluginKindEngineering} }

// pluginKindEngineering is the kind whose screens carry a `medium`.
const pluginKindEngineering = "engineering"

// knownPluginMediums mirrors the client's `PLUGIN_MEDIUMS`.
//
// ⚠★ 与 `engineering` 配套的那一格，而且它**同样不许写错一个字母**：
// 客户端的校验器对 `medium` 是**拒收**（不像 `normalizeMedium` 那样回落成
// `video`）—— 理由写在那边的 `PLUGIN_MEDIUMS` 上（回落的表现是"装得上、
// 看着正常、只是永远列错一家"，而作者以为自己写的是另一家）。
func knownPluginMediums() []string { return []string{"video", "audio", "text"} }

func isKnownPluginMedium(m string) bool {
	for _, k := range knownPluginMediums() {
		if k == m {
			return true
		}
	}
	return false
}

func isKnownPluginKind(kind string) bool {
	for _, k := range knownPluginKinds() {
		if k == kind {
			return true
		}
	}
	return false
}

// knownPluginSources mirrors the keys of the client's `SOURCE_FIELDS`.
func knownPluginSources() []string {
	return []string{"voiceCatalog", "avatarCatalog", "appCatalog", "worldOps"}
}

func isKnownPluginSource(source string) bool {
	for _, s := range knownPluginSources() {
		if s == source {
			return true
		}
	}
	return false
}
