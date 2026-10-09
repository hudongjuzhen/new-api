package mode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

// =========================================================================
// ★★ 模式库：磁盘上一个目录，一份模式一个文件（docs/28 §4）
//
//	<工作目录>/modes/mv.json        ← 与客户端 `modes/mv.json` **逐字同格式**
//	<工作目录>/modes/audiobook.json
//
// 目录由 `ZSY_MODE_DIR` 指定，默认 `<工作目录>/modes`（与 `prompts/`、
// `plugin-templates/` 那些同级）。
//
// # ⚠★ 为什么存量是**文件**而不是一张表
//
// 用户的原话是"每个模式的 json 就默认存放在服务端，本地没有的时候会提示开通，
// 点击开通之后该 json 就会下载到本地" —— 也就是说：
//
//	服务端那一份  ──（一次 GET）──▶  客户端 `<数据目录>/modes/<id>.json`
//
// **两份必须是同一个东西**。放在磁盘上时："服务端这份就是用户在本地会拿到的那一份"
// 是一眼可验的（拷出来 diff 即可，也能直接发给别人）。放进数据库的大字段之后，
// 它变成"要靠一段代码才能还原出来的东西"，而**差异只会在用户机器上显现**
// —— 那正是本项目最忌讳的一类坏法（改一处、另一处不跟着变，且不报错）。
//
// # 两格扩展：`x-visibility` / `x-summary`
//
// 模式文件本身**没有**"给谁看"这一格（那是分发策略，不是模式的一部分），
// 所以它用 `x-` 前缀挂在文件头上 —— 与 `plugin-templates/*.json` 的
// `x-capabilities` / `x-visibility` 同一条惯例（JSON Schema 给扩展关键字的写法）。
//
// ⚠★ 这两格**不会**跟着文件下到用户机器上：取件那一条（`controllers_public.go`）
// 会把它们拿掉。理由与插件那一条**逐字相同**：客户端的模式格式里没有这两格，
// 而"不认识的顶层字段跟着导出"意味着它们会被**原样存进用户本地的模式文件**
// 再原样写回磁盘 —— 一份带着宿主内部字段的模式文件会跟着用户到处跑。
// =========================================================================

const (
	envModeDir        = "ZSY_MODE_DIR"
	modeDirDefault    = "modes"
	visibilityKey     = "x-visibility"
	summaryKey        = "x-summary"
	modeFormat        = "aimv-work-mode"
	modeFormatVersion = 1
)

// Visibility classes.
//
// ⚠★ **默认是 `private`，不是 `public`** —— 方向与 `plugin-templates` 那一条
// **逐字相同**：`public` 意味着"任何账号点一下就装上了"，而一份忘写这一格的模式
// 悄悄变成人人可用**没有任何地方会报错**。反过来（忘写 = 谁都看不到）的症状
// 一眼就能看见（广场空着、用户来问），修起来也只是补一格。
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// hostOnlyKeys are the extension keys that belong to **this backend** and must
// never travel into the file the client stores.
//
// ⚠★ One list, one place. Two lists would drift, and the symptom of a drift is
// "the file I downloaded carries a key the arena does not know about" —
// which nobody would think to look for.
var hostOnlyKeys = []string{visibilityKey, summaryKey}

// knownMediums mirrors the client's `MODE_MEDIUM_IDS` / `PLUGIN_MEDIUMS`.
//
// ⚠ 它是**第二处定义**（客户端 `core/workModes.js` 那一份说了算），所以纪律是
// "**只允许更保守**"：宁可这里拒掉一份其实能用的模式，也不要发出一份客户端
// 认不出的东西 —— 与 `zsy/world` 的 `knownPluginKinds` 同一条。
var knownMediums = []string{"video", "audio", "text"}

// ModeFile is one mode: the file on disk plus what this layer reads out of it.
type ModeFile struct {
	// ID is the mode id, and it must equal the file name (without `.json`).
	//
	// ⚠ 这条判据与客户端 `work_modes.rs` 的 `mode_path` **同源**：文件名就是 id。
	// 不一致时的表现是"服务端发的文件名说 mv、里面写着别的"，而客户端落盘时
	// 会以**文件名**为准 —— 于是用户拿到的是 mv，而内容不是。绝不静默。
	ID string `json:"id"`
	// Label is the display name (the mode file's own `label`).
	Label string `json:"label"`
	// Medium is "video" / "audio" / "text" —— 客户端按它分组。
	Medium string `json:"medium"`
	// WorkScale is "single" / "serial"（客户端那一格的原值，只给运营看一眼）。
	WorkScale string `json:"workScale"`
	// Visibility is "public" / "private" (see above).
	Visibility string `json:"visibility"`
	// Summary is the one-liner shown on the plaza card (`x-summary`).
	Summary string `json:"summary"`
	// Problem non-empty means this file is not shippable. It is **listed anyway**
	// on the admin face ("绝不静默" — 一个不出现的文件会让运营以为后台坏了).
	Problem string `json:"problem"`
	// Source is the file it came from (operator-facing).
	Source string `json:"source"`

	// Manifest is the parsed file **as written** (nothing stripped).
	Manifest map[string]any `json:"-"`
	// Raw is the file text.
	Raw string `json:"-"`
}

// IsPublic answers whether this mode is listed to every account.
//
// ⚠ It is a named predicate rather than an inline comparison: this single line is
// what stands between a paid mode and "any account can open it", so it gets a
// name a test can point at.
func (m *ModeFile) IsPublic() bool {
	return m != nil && m.Visibility == VisibilityPublic && m.Problem == ""
}

// Shippable answers whether the file may be handed to a client at all.
func (m *ModeFile) Shippable() bool { return m != nil && m.Problem == "" }

/* ==========================================================================
 * 目录与读盘
 * ======================================================================== */

var (
	modeMu    sync.RWMutex
	modeCache map[string]*ModeFile
	modeDir   string
)

// ModeDir answers the directory mode files are read from.
func ModeDir() string {
	modeMu.RLock()
	defer modeMu.RUnlock()
	if modeDir != "" {
		return modeDir
	}
	return resolveModeDir()
}

// resolveModeDir reads the env var, falling back to the default.
func resolveModeDir() string {
	if p := strings.TrimSpace(common.GetEnvOrDefaultString(envModeDir, "")); p != "" {
		return p
	}
	return modeDirDefault
}

// LoadModes reads every mode file and caches the result.
//
// It never returns an error: a bad file is reported **per file** (in the admin
// list), because refusing to load the whole set would hide the good ones behind
// one bad file — and the operator needs to see which file is wrong.
func LoadModes() []*ModeFile {
	dir := resolveModeDir()

	rows := make([]*ModeFile, 0, 12)
	entries, err := os.ReadDir(dir)
	if err != nil {
		/*
		 * 目录不存在**不是错误**：一个还没配过模式库的部署照样应当能打开那一屏
		 * （它会显示"目录里还没有模式"+ 目录路径，让运营知道该往哪儿放）。
		 */
		modeMu.Lock()
		modeCache = map[string]*ModeFile{}
		modeDir = dir
		modeMu.Unlock()
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

	next := make(map[string]*ModeFile, len(names))
	for _, name := range names {
		path := filepath.Join(dir, name)
		wantID := strings.TrimSuffix(name, filepath.Ext(name))
		rows = append(rows, loadOneMode(path, wantID, next))
	}

	modeMu.Lock()
	modeCache = next
	modeDir = dir
	modeMu.Unlock()
	return rows
}

// ReloadModes forces a re-read (the admin page's 重新读取 button).
func ReloadModes() []*ModeFile { return LoadModes() }

// loadOneMode reads and validates one file, registering it in `into` on success.
func loadOneMode(path, wantID string, into map[string]*ModeFile) *ModeFile {
	row := &ModeFile{ID: wantID, Label: wantID, Source: path, Visibility: VisibilityPrivate}

	raw, err := os.ReadFile(path)
	if err != nil {
		row.Problem = fmt.Sprintf("读不到这个文件：%v", err)
		return row
	}
	row.Raw = string(raw)

	var manifest map[string]any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		row.Problem = fmt.Sprintf("不是合法的 JSON：%v", err)
		return row
	}
	row.Manifest = manifest

	row.Label = stringField(manifest, "label", wantID)
	row.Medium = strings.TrimSpace(stringField(manifest, "medium", ""))
	row.WorkScale = strings.TrimSpace(stringField(manifest, "workScale", ""))
	row.Summary = strings.TrimSpace(stringField(manifest, summaryKey, ""))

	vis, err := readVisibility(manifest)
	if err != nil {
		row.Problem = err.Error()
		return row
	}
	row.Visibility = vis

	if problem := modeShapeProblem(manifest, wantID); problem != "" {
		row.Problem = problem
		return row
	}

	if _, dup := into[wantID]; dup {
		row.Problem = fmt.Sprintf("有两份模式都叫 %q", wantID)
		return row
	}
	into[wantID] = row
	return row
}

// readVisibility reads `x-visibility` and normalises it.
//
// ⚠ Missing / empty means **private** — the reasoning is in the const block above,
// and it is the one place that decision is allowed to live.
func readVisibility(manifest map[string]any) (string, error) {
	raw, ok := manifest[visibilityKey]
	if !ok || raw == nil {
		return VisibilityPrivate, nil
	}
	name := strings.TrimSpace(fmt.Sprint(raw))
	if name == "" {
		return VisibilityPrivate, nil
	}
	if name != VisibilityPublic && name != VisibilityPrivate {
		return "", fmt.Errorf("%s 里是不认识的值 %q（能写的是 %s / %s）—— "+
			"它决定这份模式是「任何账号都能一键开通」还是「后台给了权限才显示」，"+
			"写错了会让一份付费模式变成公开可开通。",
			visibilityKey, name, VisibilityPublic, VisibilityPrivate)
	}
	return name, nil
}

// modeShapeProblem answers "" when the file is shaped like a mode.
//
// ⚠ 这一层**不重复**客户端那些判据（词表、指令、库表、按钮白名单）——
// 真正的校验器在客户端（`core/workModes.js` 的 `parseWorkMode`）。这里只判
// "别把一份明显不成形的东西发出去"，而那几件恰好也是客户端一定会判、
// 判错了代价最大的。
func modeShapeProblem(manifest map[string]any, wantID string) string {
	if got := strings.TrimSpace(stringField(manifest, "format", "")); got != modeFormat {
		return fmt.Sprintf("format 应当是 %q（现在是 %q）—— 这一份不是模式文件。", modeFormat, got)
	}
	ver := numberField(manifest, "formatVersion")
	if ver <= 0 {
		return "这份模式没有写 formatVersion（或者它不是数字）。"
	}
	if ver > modeFormatVersion {
		return fmt.Sprintf("这份模式是更新版本写的（formatVersion %v，本版认到 %d）—— "+
			"发出去客户端会拒收，请先升级服务端或改用旧版本写的模式。", ver, modeFormatVersion)
	}
	id := strings.TrimSpace(stringField(manifest, "id", ""))
	if id == "" {
		return fmt.Sprintf("这份模式没有写 id。文件名是 %s.json，所以它的 id 应当是 %q。", wantID, wantID)
	}
	if id != wantID {
		return fmt.Sprintf("文件名与 id 不一致：文件名说 %q，而里面的 id 是 %q。"+
			"请把文件名改成 %s.json（或改掉 id）—— 文件名就是「该改哪个文件」的唯一答案，也是客户端落盘时用的那个名字。",
			wantID, id, id)
	}
	if strings.TrimSpace(stringField(manifest, "label", "")) == "" {
		return "这份模式没有写 label —— 它是广场卡片上那一行字（也是下拉里那一项），空着用户认不出它是什么。"
	}
	medium := strings.TrimSpace(stringField(manifest, "medium", ""))
	if !isKnownMedium(medium) {
		return fmt.Sprintf("这份模式的 medium 是 %q，本应用不认识（能写的是 %s）—— "+
			"它决定这一档出现在哪一家（也决定它属于哪个发行版）。",
			medium, strings.Join(knownMediums, " / "))
	}
	return ""
}

func isKnownMedium(m string) bool {
	for _, k := range knownMediums {
		if k == m {
			return true
		}
	}
	return false
}

// ModeByID answers one mode by id, reading the file every time.
//
// ★★ **判据是磁盘上有没有这个文件**，不查缓存 —— 与 `zsy/world` 的
// `PluginTemplateByID` **逐字同一条教训**：缓存是模块级的，两个函数对同一份文件
// 给出相反结论（"没有这份模式"，而同一个函数的错误分支却能列出它）就是"同一个问题
// 有两个答案"。缓存因此降级成纯粹的**列表**用途，读漏了不影响取件。
//
// 代价是每次取件多一次 `os.ReadFile`（一份模式 7–33 KB）—— 而开通是人点一下的动作。
func ModeByID(id string) (*ModeFile, bool) {
	dir := resolveModeDir()
	safe := strings.TrimSpace(id)
	/*
	 * ⚠★ **路径不许由用户数据拼出来**（与 `media.rs` 的 `rename_file_in`
	 * 同一条边界纪律）：`id` 来自 URL，`../` 会读到目录外面去。
	 * 判据是模式 id 的形状（小写字母开头 + 字母数字下划线短横线，≤32）——
	 * 与客户端 `MODE_ID_RE` / Rust `MODES_ID_PATTERN` 同一个形状。
	 */
	if !isValidModeID(safe) {
		return nil, false
	}
	path := filepath.Join(dir, safe+".json")

	if _, err := os.Stat(path); err != nil {
		return nil, false
	}

	into := map[string]*ModeFile{}
	row := loadOneMode(path, safe, into)
	return row, true
}

// isValidModeID mirrors the client's `MODE_ID_RE` and Rust's `MODE_ID_PATTERN`.
func isValidModeID(id string) bool {
	if id == "" || len(id) > 32 {
		return false
	}
	for i, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		case (r == '_' || r == '-') && i > 0:
		default:
			return false
		}
	}
	return true
}

// RenderModeFile writes a mode file out **without** the host-only keys.
//
// ★ 它与磁盘上那一份的差别只有「少了两格 `x-`」——**正文一个字都不动**，
// 含运营可能手写进来的任何不认识的字段（客户端承诺"不认识的顶层字段跟着导出"，
// 在这里丢掉一格的表现是"我写的模式明明有那一格，装上去就没了"，而且不报错）。
//
// # ⚠★ 为什么要自己走一遍有序编码，而不是 `json.MarshalIndent`
//
// 第一版写的就是 `json.MarshalIndent(map[string]any, "", "  ")`，而它**会按
// 字母序重排顶层键**（Go 的 map 是无序的，`encoding/json` 对 map 一律排序）。
// 探针当场露了出来：`modes/mv.json` 的第一行本来是 `"format"`，发出去却成了
// `"appRows"`。
//
// ⚠ 那不是"排版小事"，而是把这个方案的**立足点**弄丢了：
//
//	服务端那一份  ──（一次 GET）──▶  客户端 `<数据目录>/modes/<id>.json`
//
// 选"存量放文件、不放数据库"的**全部理由**就是"两份必须是同一个东西、一眼可验"
// （见本文件头）。键序一乱，两份就不能直接 diff 了；而用户第一次开通之后看到的
// 会是"我本地这份怎么长得跟服务端不一样"——一个没人能解释的差异。
//
// 所以这里按**原文件的键序**走一遍：`json.Decoder` 逐键取原始片段（保序），
// 每一段用 `json.Indent` 补回缩进，跳过 `hostOnlyKeys`。
// 结果与"把磁盘那份原文里那两行删掉"**逐字节相同**（`catalog_test.go` 有一条
// 断言钉的就是这件事）。
func RenderModeFile(row *ModeFile) (string, error) {
	if row == nil || strings.TrimSpace(row.Raw) == "" {
		return "", fmt.Errorf("mode: 这一份没有原文，发不出去")
	}

	dec := json.NewDecoder(strings.NewReader(row.Raw))
	if _, err := dec.Token(); err != nil { // 顶层那个 '{'
		return "", fmt.Errorf("mode: 读不出这一份的顶层对象: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("{\n")
	first := true
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("mode: 读键失败: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return "", fmt.Errorf("mode: 顶层出现了非字符串的键（%v）", keyTok)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return "", fmt.Errorf("mode: 读 %q 的值失败: %w", key, err)
		}
		if isHostOnlyKey(key) {
			continue
		}
		/* ⚠ 缩进由 `indentValue` 一处管（改文件那一条路用的是同一个函数） */
		text, err := indentValue(value)
		if err != nil {
			return "", fmt.Errorf("mode: 重新缩进 %q 失败: %w", key, err)
		}
		if !first {
			buf.WriteString(",\n")
		}
		first = false
		buf.WriteString("  ")
		buf.WriteString(strconv.Quote(key))
		buf.WriteString(": ")
		buf.WriteString(text)
	}
	if _, err := dec.Token(); err != nil { // 收尾那个 '}'
		return "", fmt.Errorf("mode: 这一份的顶层对象没收好: %w", err)
	}
	buf.WriteString("\n}\n")
	return buf.String(), nil
}

// isHostOnlyKey reports whether a top-level key belongs to this backend.
func isHostOnlyKey(key string) bool {
	for _, k := range hostOnlyKeys {
		if k == key {
			return true
		}
	}
	return false
}

/* ==========================================================================
 * ★★ 改一份模式的**分发策略**（`docs/28` §3b：后台要能设公开 / 私有）
 *
 * 用户 2026-… 的要求：
 *
 *	"new-api 的后台，模式管理 应该是**可以编辑**的，可以设置**权限是公开还是私有**"
 *
 * # 为什么是"改那两行"而不是"重新序列化一份"
 *
 * 与 `RenderModeFile` **逐字同一条理由**（选"存量放文件"的全部价值就是
 * "服务端那一份 = 客户端那一份、一眼可验"）：把整份 JSON 读进 `map[string]any`
 * 再写回去会**按字母序重排顶层键**、还会把 `<` 转义成 `\u003c` ——
 * 于是运营只是把一档从私有改成公开，磁盘上那份文件却**整篇被重排**，
 * 而"哪一档是哪一版"再也 diff 不出来。
 *
 * ⚠★ 所以这里走**逐键原样搬运 + 只换那两格的值**：改完之后，那份文件与改之前
 * 的差别**只有那两行**（`catalog_test.go` 有一条逐字节断言钉着）。
 * 那两格**没有**的时候插在 `formatVersion` 之后（文件头那两行之后）——
 * 那是它们**语义上**该在的地方（"这个文件是什么" → "它怎么发"）。
 * ======================================================================== */

// ModeMetaInput is what the admin may change about one mode file.
//
// ⚠ 只开放**分发策略**那两格：正文（`words` / `planDialog` / `libraries` / 指令…）
// **一个字都不从后台改** —— 它是给客户端解析的、几千行的结构化数据，
// 在一个 textarea 里改它等于让运营手写 JSON（而那正是"模式文件"存在的意义：
// 用编辑器或直接改文件）。要改正文就改文件，这一面**不假装**能改。
type ModeMetaInput struct {
	// Visibility 是 `public` / `private`（空 = 不改这一格）。
	Visibility string
	// Summary 是广场卡片上那句说明（`nil` = 不改；指向空串 = 清掉这一格）。
	Summary *string
}

// WriteModeMeta rewrites the two `x-` keys of one mode file, preserving everything else.
//
// 返回**改完之后**的原文（调用方可以据此更新它手上的那份）。
//
// ⚠ 它**先写 `.bak`**（只留最早那一版，与客户端 `work_modes.rs` 的处置同源）：
// 运营手滑把一档从公开改成私有时，那个 `.bak` 是他唯一能翻回去的地方。
// ⚠ 而且它**不碰那些坏文件**：一份读不出来的模式，改它只会把问题搅得更乱
// （后台那一屏会把坏文件列出来并说明原因，运营该做的是去修那个文件）。
func WriteModeMeta(id string, in ModeMetaInput) (string, error) {
	row, found := ModeByID(id)
	if !found {
		return "", fmt.Errorf("mode: 模式库里没有 %q 这一档（文件名就是模式 id，例如 mv.json）", id)
	}
	if row.Problem != "" {
		return "", fmt.Errorf("mode: %s 这一份有问题，先修好它再改：%s", id, row.Problem)
	}

	visibility := strings.TrimSpace(in.Visibility)
	if visibility != "" && visibility != VisibilityPublic && visibility != VisibilityPrivate {
		return "", fmt.Errorf("mode: 可见性只能写 %s / %s（收到的是 %q）—— "+
			"它决定这一档是「任何账号都能一键开通」还是「后台给了权限才显示」。",
			VisibilityPublic, VisibilityPrivate, visibility)
	}
	summary := ""
	hasSummary := in.Summary != nil
	if hasSummary {
		summary = strings.TrimSpace(*in.Summary)
	}

	out, err := rewriteMetaKeys(row.Raw, visibility, summary, hasSummary)
	if err != nil {
		return "", err
	}
	if out == row.Raw {
		/* ⚠ 没变就**不写**：白写一次会让"改过没有"这件事看不出来（`.bak` 也被无谓地覆盖） */
		return out, nil
	}

	path := row.Source
	/*
	 * ⚠ `.bak` 只在那份**还不存在**时才建（第一次改才留）—— 与"留最早那一版"
	 * 那条纪律一致：第二次改不该把最初的版本冲掉。
	 */
	bak := path + ".bak"
	if _, statErr := os.Stat(bak); os.IsNotExist(statErr) {
		if writeErr := os.WriteFile(bak, []byte(row.Raw), 0o600); writeErr != nil {
			return "", fmt.Errorf("mode: 备份 %s 失败（没有它就动手改文件太危险）：%w", bak, writeErr)
		}
	}
	/*
	 * ★ 先写同目录的临时文件再 `rename`（原子替换）：中途失败时磁盘上要么是旧那份、
	 * 要么是新那份，**不会**留下半份 JSON —— 而半份 JSON 在客户端那边是"这一档读不出来"。
	 */
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(out), 0o600); err != nil {
		return "", fmt.Errorf("mode: 写临时文件失败：%w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("mode: 替换 %s 失败：%w", path, err)
	}
	return out, nil
}

// rewriteMetaKeys reproduces the file **key by key**, replacing (or inserting)
// only `x-visibility` / `x-summary`.
//
// ⚠ 插入位置是 `formatVersion` 之后：那两格是"宿主的事"，紧跟着"这个文件是什么"
// 那两行读起来最顺（也与人手写这几份文件时的习惯一致）。
func rewriteMetaKeys(raw, visibility, summary string, hasSummary bool) (string, error) {
	dec := json.NewDecoder(strings.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		return "", fmt.Errorf("mode: 读不出这一份的顶层对象: %w", err)
	}

	type pair struct {
		key   string
		value string // 已经缩进好的一段文本
	}
	pairs := make([]pair, 0, 16)
	seenVisibility := false
	seenSummary := false

	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return "", fmt.Errorf("mode: 读键失败: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return "", fmt.Errorf("mode: 顶层出现了非字符串的键（%v）", keyTok)
		}
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return "", fmt.Errorf("mode: 读 %q 的值失败: %w", key, err)
		}
		text, err := indentValue(value)
		if err != nil {
			return "", fmt.Errorf("mode: 重新缩进 %q 失败: %w", key, err)
		}

		switch key {
		case visibilityKey:
			/*
			 * ⚠★ 已经写过了就**跳过原来那一格**：插入点在 `formatVersion` 之后，
			 * 而文件里那两格通常就在它后面 —— 不看这一眼会把同一格写**两遍**，
			 * 而 JSON 里重复的键**不报错**（`encoding/json` 取最后一个），
			 * 于是文件"看起来能用"、只是多了一行谁也说不清的东西。
			 * （踩过一次：逐字节断言当场抓到 `x-visibility` 出现了两次。）
			 */
			if seenVisibility {
				break
			}
			seenVisibility = true
			/* ⚠ 这一格**不传就不动**（空串 = 这次只改说明） */
			if visibility == "" {
				pairs = append(pairs, pair{key, text})
				break
			}
			pairs = append(pairs, pair{key, strconv.Quote(visibility)})
		case summaryKey:
			if seenSummary {
				break
			}
			seenSummary = true
			if !hasSummary {
				pairs = append(pairs, pair{key, text})
				break
			}
			pairs = append(pairs, pair{key, strconv.Quote(summary)})
		default:
			pairs = append(pairs, pair{key, text})
		}

		/* ★ 插入点：紧跟 `formatVersion` 之后（那两格**没有**时才插） */
		if key == "formatVersion" {
			if visibility != "" && !seenVisibility {
				pairs = append(pairs, pair{visibilityKey, strconv.Quote(visibility)})
				seenVisibility = true
			}
			if hasSummary && !seenSummary {
				pairs = append(pairs, pair{summaryKey, strconv.Quote(summary)})
				seenSummary = true
			}
		}
	}
	if _, err := dec.Token(); err != nil {
		return "", fmt.Errorf("mode: 这一份的顶层对象没收好: %w", err)
	}

	/* 没有 `formatVersion` 的文件不该走到这里（`modeShapeProblem` 会拦），兜底放最前面 */
	if visibility != "" && !seenVisibility {
		pairs = append([]pair{{visibilityKey, strconv.Quote(visibility)}}, pairs...)
	}
	if hasSummary && !seenSummary {
		pairs = append([]pair{{summaryKey, strconv.Quote(summary)}}, pairs...)
	}

	var buf bytes.Buffer
	buf.WriteString("{\n")
	for i, p := range pairs {
		if i > 0 {
			buf.WriteString(",\n")
		}
		buf.WriteString("  ")
		buf.WriteString(strconv.Quote(p.key))
		buf.WriteString(": ")
		buf.WriteString(p.value)
	}
	buf.WriteString("\n}\n")
	return buf.String(), nil
}

// indentValue re-indents one raw JSON value to the file's two-space style.
func indentValue(value json.RawMessage) (string, error) {
	var indented bytes.Buffer
	if err := json.Indent(&indented, value, "  ", "  "); err != nil {
		return "", err
	}
	return indented.String(), nil
}

/* ==========================================================================
 * 小工具（与 `zsy/world` 的同名函数逐字同形 —— 那一层抄 `plugin_template.go`）
 * ======================================================================== */

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

// numberField reads one numeric field (JSON numbers arrive as float64).
func numberField(obj map[string]any, key string) float64 {
	raw, ok := obj[key]
	if !ok || raw == nil {
		return 0
	}
	if n, ok := raw.(float64); ok {
		return n
	}
	return 0
}
