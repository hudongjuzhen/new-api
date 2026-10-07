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

	caps, err := templateCapabilities(manifest)
	if err != nil {
		row.Problem = err.Error()
		return row
	}
	if len(caps) == 0 {
		row.Problem = fmt.Sprintf("这份模板没写 %q —— 它是「这份插件要配哪些能力」的唯一来源，"+
			"而能力名是运营的知识、代码推不出来。请写下它需要的能力（例如 [%q]）。",
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
func templateCapabilities(manifest map[string]any) ([]string, error) {
	raw, ok := manifest[templateCapabilitiesKey]
	if !ok || raw == nil {
		return nil, nil
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("%s 应当是一个字符串数组", templateCapabilitiesKey)
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
	return out, nil
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
func knownPluginKinds() []string { return []string{"catalog", "world"} }

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
