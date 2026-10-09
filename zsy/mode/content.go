package mode

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// =========================================================================
// ★★ 读 / 写一份模式的**完整正文**（用户 2026-…）
//
// 用户的原话：
//
//	"在编辑弹窗里……里面有这个模式的具体内容编辑，根据 json 中的内容做编辑"
//
// # ⚠★ 它与 `WriteModeMeta` 的分工
//
// | 那一个动作 | 改什么 | 走哪条路 |
// |---|---|---|
// | 设公开 / 私有 + 一句话说明 | 只有那两格 `x-` | `WriteModeMeta`（**逐键搬运**，改完只差两行） |
// | 编辑模式内容 | `words` / `planDialog` / `requires` / `libraries`…（几乎整份） | ★ 这里 |
//
// 两条路的**风险完全不同**，所以纪律也不同：
//
//	`WriteModeMeta`  一次只动两格 → 必须**逐字节可验**（磁盘那份与改之前只差那两行）
//	这一条          几乎整份都要写 → "逐字节"这件事**做不到**（用户就是要改它），
//	                所以换成**校验 + 原子替换 + 留 `.bak`** 三件套：
//	                ★ 校验没过时磁盘**一个字节都不动**（与客户端"先校验、后落盘"同一条纪律）
//
// # ⚠★ 键序仍然按**磁盘上那份**来
//
// 一份模式的顶层键序是**人写的**（`format` 在最前、正文在后）。让 Go 的
// `map[string]any` 去序列化会**按字母序重排**（`appRows` 跑到最前）——
// 于是运营只改了 `words` 里一个词，磁盘上那份却整篇重排，"这次改了什么"就 diff 不出来了。
// 所以这里与 `rewriteMetaKeys` 同源：**读进有序的键表，写时按那个顺序走**，
// 新出现的键追加在末尾。
//
// # 两格 `x-` 的归属
//
// 读回来的 `content` **含**那两格（运营在界面上要看见自己设的是公开还是私有），
// 写回去时它们**照样落在这份文件里**（它们本来就是这个文件的一部分）。
// ⚠ 它们不会被发到客户端（`RenderModeFile` 在那里拿掉）—— 那是另一条路的事。
// =========================================================================

// maxModeContentBytes bounds one mode file's text (mirrors the client's 8 MB import cap).
//
// ⚠ 没有这道闸的话，一次"保存"可以把一份几百 MB 的 JSON 写进模式目录 ——
// 而那之后**每一次列目录**（包括公开面那一次）都要读它。
const maxModeContentBytes = 8 << 20

// ModeContent is one mode's full body plus the two host-only keys.
type ModeContent struct {
	ID string `json:"modeId"`
	// Content is the file as a decoded JSON object (host-only keys included).
	Content map[string]any `json:"content"`
}

// ReadModeContent reads one mode's full body.
//
// ⚠ 它与 `ModeByID` 同一条纪律：**每次现读磁盘**，不查缓存 ——
// 缓存是模块级的，两个函数对同一份文件给出相反结论就是"同一个问题有两个答案"。
func ReadModeContent(id string) (*ModeContent, error) {
	row, found := ModeByID(id)
	if !found {
		return nil, fmt.Errorf("mode: 模式库里没有 %q 这一档（文件名就是模式 id，例如 mv.json）", id)
	}
	if row.Problem != "" {
		return nil, fmt.Errorf("mode: %s 这一份有问题，先修好它再编辑：%s", id, row.Problem)
	}
	if row.Manifest == nil {
		return nil, fmt.Errorf("mode: %s 这一份读不出正文（不是合法的 JSON 对象）", id)
	}

	out := make(map[string]any, len(row.Manifest))
	for k, v := range row.Manifest {
		out[k] = v
	}
	/*
	 * ★ 那两格 `x-` **补进去**：`row.Manifest` 是文件里原样的内容，而文件**可能没写**
	 * 这两格（老文件）。不补的话界面上那两格是空的，而保存时又"看起来什么都没设" ——
	 * 于是运营打开一档私有模式、只改了一个词、顺手保存，**可见性那一格就被清掉了**。
	 */
	if _, ok := out[visibilityKey]; !ok {
		out[visibilityKey] = row.Visibility
	}
	if _, ok := out[summaryKey]; !ok {
		out[summaryKey] = row.Summary
	}
	return &ModeContent{ID: row.ID, Content: out}, nil
}

// WriteModeContent replaces one mode's body.
//
// 返回**写完之后**的文件原文（调用方可以据此回显，也可以拿去比对）。
//
// ⚠★ 次序不能换：**先校验、后落盘**。反过来（先写后校验）的代价是一份填错的表单
// 把磁盘上好好的那一份覆盖掉 —— 而那份文件是**客户端会下到本机的东西**，
// 覆盖了就要去翻 `.bak`（与客户端 `modeInstall.js` 那条"先校验后落盘"同源）。
func WriteModeContent(id string, content map[string]any) (string, error) {
	row, found := ModeByID(id)
	if !found {
		return "", fmt.Errorf("mode: 模式库里没有 %q 这一档（文件名就是模式 id，例如 mv.json）", id)
	}
	if row.Problem != "" {
		return "", fmt.Errorf("mode: %s 这一份有问题，先修好它再编辑：%s", id, row.Problem)
	}
	if content == nil {
		return "", fmt.Errorf("mode: 这次没有任何要写的内容（收到的是空的）。")
	}

	// ① 可见性：写进来的那一格必须认得（认不出时**拒绝**，不静默按私有存）
	if raw, ok := content[visibilityKey]; ok {
		text, isText := raw.(string)
		if !isText {
			return "", fmt.Errorf("mode: %s 只能是 %s / %s（收到的是一个 %T）",
				visibilityKey, VisibilityPublic, VisibilityPrivate, raw)
		}
		trimmed := strings.TrimSpace(text)
		if trimmed != VisibilityPublic && trimmed != VisibilityPrivate {
			return "", fmt.Errorf("mode: %s 只能写 %s / %s（收到的是 %q）—— "+
				"它决定这一档是「任何账号都能一键开通」还是「后台给了权限才显示」。",
				visibilityKey, VisibilityPublic, VisibilityPrivate, trimmed)
		}
	}

	/*
	 * ★★ 正文那一套校验**与读盘完全同源**（`modeShapeProblem`）：format / formatVersion /
	 * id / label / medium 五条。用第二份校验的坏法是"保存时放行了、读盘时判成坏文件"
	 * （或者反过来）—— 而那时磁盘上已经躺着一份发不出去的东西了。
	 */
	if problem := modeShapeProblem(content, id); problem != "" {
		return "", fmt.Errorf("mode: 这一份保存不了：%s", problem)
	}

	out, err := renderOrderedMode(content, row.Raw)
	if err != nil {
		return "", err
	}
	if len(out) > maxModeContentBytes {
		return "", fmt.Errorf("mode: 这一份太大了（%d 字节，上限 %d）—— "+
			"模式文件是给客户端解析的，几百 MB 的那种它读不动。",
			len(out), maxModeContentBytes)
	}
	/*
	 * ⚠★ **没变就不写**，而"变了没有"的判据是**语义**上的（比一遍两边的 JSON），
	 * 不是"渲染出来的文本一不一样"。
	 *
	 * 这一条是**实测**改的：第一版比的是文本，而一份人手写的模式文件里
	 * `"words": { "text": "歌词" }` 是**一行**的，渲染出来却是三行 ——
	 * 于是"打开弹窗、什么都不改、点保存"会把整篇文件重排一遍（几十 KB 的 diff），
	 * 而运营明明什么都没改。文本相同才算没变太严了；**内容相同**才是他要的意思。
	 */
	if sameManifest(row.Manifest, content) {
		return row.Raw, nil
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

// renderOrderedMode serialises one mode object, keeping the key order of `previous`.
//
// ⚠★ 这是与 `rewriteMetaKeys` **同一条纪律**的第二处实现（那边只换两格、这边换整份）：
// 键序按**磁盘上那份**走，新键追加在末尾。理由写在文件头（"这次改了什么"要 diff 得出来）。
//
// ⚠ 缩进用两个空格 —— 与仓库里那几份模式文件**逐字相同**（它们是运营用编辑器改的，
// 四空格那些文件在这里一存就会整篇变样）。
func renderOrderedMode(content map[string]any, previous string) (string, error) {
	order := topLevelKeyOrder(previous)
	seen := make(map[string]bool, len(order))

	// 先按老顺序写，然后是这次新加的那些键（按字母序，保证同样的输入得到同样的输出）
	rest := make([]string, 0, 4)
	for key := range content {
		if !seen[key] && !containsKey(order, key) {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)

	var b strings.Builder
	b.WriteString("{\n")
	first := true
	writePair := func(key string) error {
		raw, err := common.Marshal(content[key])
		if err != nil {
			return fmt.Errorf("mode: 写 %q 那一格失败：%w", key, err)
		}
		/* ⚠ 缩进由 `indentValue` 一处管（改文件那一条路用的是同一个函数） */
		text, err := indentValue(raw)
		if err != nil {
			return fmt.Errorf("mode: 重新缩进 %q 失败：%w", key, err)
		}
		if !first {
			b.WriteString(",\n")
		}
		first = false
		b.WriteString("  ")
		b.WriteString(strconv.Quote(key))
		b.WriteString(": ")
		b.WriteString(text)
		return nil
	}

	for _, key := range order {
		if _, ok := content[key]; !ok {
			continue
		}
		seen[key] = true
		if err := writePair(key); err != nil {
			return "", err
		}
	}
	for _, key := range rest {
		seen[key] = true
		if err := writePair(key); err != nil {
			return "", err
		}
	}
	b.WriteString("\n}\n")
	return b.String(), nil
}

// topLevelKeyOrder reads the top-level keys of a mode file **in file order**.
//
// ⚠ 读不出来时回**空表**（调用方会退化成"按字母序"）：一份读不出来的东西
// 在这一条路上本来就走不到（`ModeByID` 已经把它标成坏文件并拦住了），
// 所以这里不必再报一次错。
func topLevelKeyOrder(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	if _, err := dec.Token(); err != nil {
		return nil
	}
	keys := make([]string, 0, 16)
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return keys
		}
		key, ok := tok.(string)
		if !ok {
			return keys
		}
		keys = append(keys, key)
		/* ⚠ 值那一半必须**读掉**才能读到下一个键（`dec.Token` 是按值成对吐的） */
		var skip json.RawMessage
		if err := dec.Decode(&skip); err != nil {
			return keys
		}
	}
	return keys
}

// sameManifest answers whether two decoded mode objects describe the same thing.
//
// ⚠★ 为什么这条判据必须是**语义**的（而不是"渲染出来的文本一样"）：
// 人手写的模式文件里 `"words": { "text": "歌词" }` 是**一行**的，而渲染出来是三行 ——
// 比文本的话，"打开弹窗、什么都不改、点保存"会把整篇文件重排一遍（几十 KB 的 diff），
// 而运营明明什么都没改。**内容相同**才是他要的那个意思。
//
// ⚠ 它比的是"渲染成 JSON 之后一不一样"：`map[string]any` 的键序在 `Marshal` 里
// 是**排序过的**（同一个 Go 版本下稳定），所以两份内容相同、键序不同的对象
// 也会被判成相同 —— 那正是我们要的（键序是**文件**的排版，不是内容）。
func sameManifest(a, b map[string]any) bool {
	left, err := common.Marshal(a)
	if err != nil {
		return false
	}
	right, err := common.Marshal(b)
	if err != nil {
		return false
	}
	return string(left) == string(right)
}

// containsKey answers whether a key is in the ordered list.
func containsKey(keys []string, want string) bool {
	for _, key := range keys {
		if key == want {
			return true
		}
	}
	return false
}
