package mode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// =========================================================================
// ★★ 「AI 一键生成模式」的提示词（用户 2026-…）
//
// 用户的原话：
//
//	"右上角增加一个添加模式的功能，点击添加模式，可以输入模式名称，模式类型，模式简介，
//	 然后能够 AI一键生成，选择一个API密钥，然后调用 glm-5.3-flash 这个模型生成，
//	 具体的生成规则，你看看是否能够理解，能的话，可以直接生成，
//	 没有把握可参考 D:\MV项目\aimv-studio 这个项目中对应的方法。"
//
// # 参考项目里那一条（`aimv-studio/src/ai/modeAssistant.js` 的 `buildModeCreateInstruction`）
//
// 它的骨架是**四段**，这里逐段照抄（而内容按本仓库的模式文件写）：
//
//	① 你是干什么的 + 用户的需求        → 见下面【你是谁】【用户的需求】
//	② 这一份的身份（id / 名称 / basedOn）→ 见下面【这一份的身份】；★ id **由我们给**，
//	    不让模型造（它是文件名，那条形状不该交给模型去记）
//	③ 【可写的字段】+ 一份**形状示例**   → ★ 见下面 `modeShapeTemplate`：
//	    示例是**从仓库里真的那一份模式文件里抽出来的**（不是手写的），
//	    所以它不会与真实契约分叉
//	④ 【输出格式】+【硬规矩】           → 见下面
//
// ⚠★ 一处**刻意不照抄**的地方：参考项目的"可改字段清单"是它自己代码里生成的
// （它有 `wordFields()` 那种表）；本仓库的模式文件是**磁盘上的 JSON**，
// 字段从**真的那几份文件**里读出来 —— 那才是"同源"在这里的意思。
//
// ⚠ 另一处：参考项目有 `basedOn`（从哪一档派生）而**本仓库的模式格式里没有这一格**
// （它是那个客户端自己的溯源字段，`docs/28` §4.10 里说过它后来还撤掉了）——
// 所以这里**不许**让模型写 basedOn，写了就会变成一份客户端不认的顶层字段。
// =========================================================================

// modeGenerationModel is the default model for mode generation.
//
// ⚠ 它是**默认值**，而调用方可以覆盖（`ai` 那一格里的 `model`）：
// 站点上的模型名由运营在渠道里配，硬写死一个名字会让"我想换个便宜模型"变成一次改代码。
const modeGenerationModel = "glm-5.3-flash"

// modeGenerationMaxTokens bounds the model's answer.
//
// ⚠ 一份模式文件 7–33 KB，而中文一个字大约一个多 token —— 4096 会把答案**从中间截断**，
// 而截断的那一份是"JSON 解析失败"（看起来像模型不会写 JSON，其实是预算不够）。
const modeGenerationMaxTokens = 16000

/*
 * `videoKind` 的两个合法字面量（视频那一家的**小类**）。
 *
 * ⚠★ 它们是**客户端枚举的副本**，这里只用来"别把非法值照抄进给模型看的示例"
 * 与"生成出来的那一份校验一下"。客户端那边认的是
 * `core/modeCatalogKinds.js` 的 `VIDEO_KINDS`（`audio` / `frame` / `other`），
 * 而 `other` 是它清洗之后的中性兜底 —— **不是能写的值**，所以不在这两个里。
 */
const (
	videoKindAudio = "audio"
	videoKindFrame = "frame"
)

// buildModeCreateInstruction assembles the whole prompt.
//
// @param request 用户写的那段需求（"我要做美食探店类的短视频"）
// @param label   他起的名字（界面上的输入框）
// @param medium  video / audio / text
// @param summary 一句话说明（写进 `x-summary`）
// @param id      已经定好的 id（由 `suggestModeID` 推出来）
func buildModeCreateInstruction(request, label, medium, summary, id string) string {
	lines := []string{
		"你是这个软件（一个 AI 视频/音频/文稿创作工具）的「创作模式」设计助手。",
		"创作模式就是一份模板：它决定界面上用哪些词、两个弹窗叫什么、" +
			"以及规划脚本时发给模型的整份指令。用户描述他要做的那类作品，你把它写成一份完整的模式文件。",
		"",
		"【用户的需求】",
		nonEmpty(request, "（用户没有写需求，请按名称与类型自己判断这一类作品该怎么做）"),
		"",
		"【这一份的身份（已经定好，原样用，不要改动）】",
		fmt.Sprintf("id：%s", id),
		fmt.Sprintf("名称 label：%s", nonEmpty(label, id)),
		fmt.Sprintf("类型 medium：%s（它只能是 video / audio / text 之一）", medium),
		fmt.Sprintf("一句话说明 x-summary：%s", nonEmpty(summary, "（用户没写，请替他写一句）")),
		"",
		"【可写的字段】",
		modeFieldCatalog(),
		"",
		"【形状示例】",
		"下面这份示例是**从真实模式文件里抽出来的形状**。键名、层级、类型都要照它；" +
			"值要换成你自己的内容（示例里那些占位说明不是内容）。",
		"```json",
		modeShapeTemplate(),
		"```",
		"",
		"【输出格式】",
		"只输出一个 JSON 对象（一整份模式文件），不要任何解释、不要 Markdown 代码块之外的文字。",
		"顶层键就用示例里那些，顺序也照它。",
		fmt.Sprintf("format 必须是 %q、formatVersion 必须是 %d —— 写错这一份就导不进来。", modeFormat, modeFormatVersion),
		fmt.Sprintf("id 必须原样是 %q（它是文件名，我们会按它落盘）。", id),
		"",
		"【硬规矩】",
		"- 只输出 JSON：第一个字符是 `{`，最后一个字符是 `}`。",
		"- 不许写 id / label / medium / x-* 之外我们没提过的顶层字段；" +
			"特别地**不要写 basedOn**（本格式里没有这一格）。",
		fmt.Sprintf("- words 那一节**每一格都要写**（共 %d 格）——写着这一类作品该用的词，"+
			"不要照抄示例里的\"歌词\"\"镜头\"（那是视频的写法，音频/文稿类要换掉）。", len(modeWordKeys())),
		"- libraries 至少写一张表（这类作品最需要攒的东西，例如「角色库」「产品库」「素材库」）。",
		"- requires 那四个布尔都要写，而且**按这份模式实际需不需要**来定。",
		"- ★★ **planDialog.instruction 必须是对象、不是字符串**：示例里它是对象，那些键" +
			"（skeleton / ruleCutPoints / framing / songFacts / …）**一个都不许少、一个都不许并成一句话**；" +
			"`framing` 再下一层也是对象。⚠ 写成字符串的后果是**客户端整节回落成内置 mv 的提示词** ——" +
			"用户看到的是「请你作为顶级MV导演…」，而不是他这一档该有的那一套（这一条真发生过）。",
		"- ★★ **planDialog.templates 必须是字符串数组**（5–7 条）：每一条是一个能直接用的完整句子，" +
			"第 0 条只有开场白（交代「你是谁、要做什么」），其余几条各给一种做法；" +
			"**不是** [{\"label\":…,\"prompt\":…}] 那样的对象数组（客户端会把对象读成 \"[object Object]\"）。",
		"- ★★ **planDialog.audio 必须是对象**（15 格），不是一段字符串；同样的理由（写错就整节回落 mv）。",
		"- ★ 不许把占位说明当内容写进文件：像「（这一段指令写什么）」「（这一格写什么）」" +
			"这种句子是给你看的形状提示；正式那一份每一格都要是这一类作品**真的能用**的整句话。",
		"- 拿不准的地方按这一类作品最普通、最好用的做法写 —— 这是一份**模板**，" +
			"用户拿到之后还能自己改。",
	}
	return strings.Join(lines, "\n")
}

// modeFieldCatalog lists the writable fields, generated from the real mode files.
//
// ⚠★ 它必须是**生成**的，不能手写一份：手写的那一份与磁盘上真的模式文件一旦分叉，
// 模型就会去写一个**不存在**的键（而它在服务端会被原样存进文件 —— 客户端不认识它，
// 用户只会看到"我这一份多了个没用的东西"）。
func modeFieldCatalog() string {
	words := modeWordKeys()
	top := modeTopLevelKeys()
	lines := []string{
		"顶层：",
		"- format / formatVersion（固定值，见下面）",
		"- id / label / medium / workScale（workScale 只能是 single 或 serial）",
		"- words：界面上那些词（下面每一格都要写）",
		"- planDialog：规划弹窗（label / title / instruction / templates / audio）—— " +
			"⚠★★ 这几格的**类型是硬契约**，见下面【硬规矩】；写错类型的后果不是报错，是客户端整节回落成内置 mv 那一套：",
		"    · label：这一个弹窗叫什么（例如「AI 整曲规划」）；title 是它上面那一行标题",
		"    · instruction：**对象**，23 格（skeleton / framing / songFacts / …）—— 视频类作品必写；" +
			"⚠ 音频与文稿类作品**不要写这一格**（那一整节讲的是镜头脚本）",
		"    · templates：**字符串数组**（5–7 条），第 0 条 = 开场白，会被放在整段指令的**第一行**",
		"    · audio：**对象**，15 格（audioScriptLead / exampleSpeak / sfxRules / segmentRule / gen / …）",
		"- shotDialog：镜头或版本弹窗（label）",
		"- requires：四个布尔 —— audio / lyrics / refs / segments（这项作品必填哪些）",
		"- libraries：资料库表（id / label / dir / itemFile / fields）",
		"- extraButtons：额外按钮（可以空数组）",
		"- appRows / lipSync / core / coreLabels（可以照示例给一份合理的）",
		"- ★ videoKind：**只有 medium=video 才写**，值只能是 audio（音频生视频）或 frame（首尾帧视频）。" +
			"它是视频那一家的小类，决定这份模式在「模式广场」里摆进哪一堆；不认识的值会落成「视频」那一堆",
		"",
		fmt.Sprintf("words 那一节（共 %d 格）：%s", len(words), strings.Join(words, " / ")),
		"",
		fmt.Sprintf("（顶层实际用到过的键，供参考：%s）", strings.Join(top, " / ")),
	}
	return strings.Join(lines, "\n")
}

// modeWordKeys lists the `words` keys used by the shipped mode files.
//
// ⚠ 从**磁盘上真的那几份**里读（`LoadModes`），而不是写死一份表：
// 那几份模式就是"客户端认哪些词"的唯一权威。
func modeWordKeys() []string {
	return keysOfNested("words")
}

// modeTopLevelKeys lists the top-level keys used by the shipped mode files.
func modeTopLevelKeys() []string {
	seen := map[string]bool{}
	for _, row := range LoadModes() {
		if row.Problem != "" {
			continue
		}
		for key := range row.Manifest {
			if isHostOnlyKey(key) {
				continue
			}
			seen[key] = true
		}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// keysOfNested answers the union of one nested object's keys across the shipped modes.
func keysOfNested(field string) []string {
	seen := map[string]bool{}
	for _, row := range LoadModes() {
		if row.Problem != "" {
			continue
		}
		nested, ok := row.Manifest[field].(map[string]any)
		if !ok {
			continue
		}
		for key := range nested {
			seen[key] = true
		}
	}
	out := make([]string, 0, len(seen))
	for key := range seen {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

// modeShapeTemplate renders the example JSON handed to the model.
//
// ★★ 它是**从仓库里真的那一份模式文件里抽出来的**，所以形状**不会与真实契约分叉**
// （参考项目那一份手写的示例里就躺过一个谁都不认的键，它自己的注释里记着这件事）。
//
// 抽法：取一份**文本类**的模式（它最短、没有镜头指令那一摊），然后
//
//	· `format` / `formatVersion` / `medium` / `workScale` / `requires` 原样留着；
//	· `words` 的**值换成占位说明**（"这一类作品该用的词"），键与层数一个不动；
//	· `planDialog` / `shotDialog` / `libraries` / `extraButtons` 同样处理；
//	· 正文里那些一整段的话（`instruction` / `templates` / `audio`）**换成一个说明串** ——
//	  否则几千字的指令会把提示词撑到几万 token，而那些字是**这一档特有**的，
//	  模型不该照抄。
func modeShapeTemplate() string {
	row := pickShapeSample()
	if row == nil {
		return "{}"
	}
	template := map[string]any{}
	for key, value := range row.Manifest {
		if isHostOnlyKey(key) {
			continue
		}
		switch key {
		case "id":
			template[key] = "（原样用上面给你的 id）"
		case "label":
			template[key] = "（原样用上面给你的名称）"
		case "words":
			template[key] = placeholderWords(value)
		case "planDialog":
			template[key] = placeholderPlanDialog(value)
		case "libraries":
			template[key] = placeholderLibraries(value)
		case "videoKind":
			/*
			 * ★★ 视频那一家的**小类**（`audio` / `frame`），2026-… 客户端加的。
			 *
			 * ⚠★ 这一格先**归一**再进示例：客户端清洗之后的兜底值是 `"other"`，
			 * 而它不是能写的字面量（真实踩过：一份生成出来的短剧模式顶着
			 * `videoKind: "other"`，于是它永远躺在「视频」那一堆里、
			 * 不参与「音频生视频」的分堆，而界面上看不出任何异常）。
			 * 照抄出去的表现就是"模型学会了写 other"。
			 *
			 * ⚠ 本仓库现挂的十份里还没有这一格（它是客户端新加的、示例文件里
			 * 因此也没有）—— 这一条守的是**下一次导入之后**：那时示例文件会带上
			 * 这一格，而它必须示范一个合法的值。
			 */
			template[key] = videoKindOf(value, "")
		default:
			template[key] = value
		}
	}
	/* ⚠ 顺序：把身份那几格放最前（模型先看到的是"这是什么"） */
	ordered := map[string]any{
		"format":        modeFormat,
		"formatVersion": modeFormatVersion,
	}
	for _, key := range []string{"id", "label", "medium", "workScale", "words", "requires", "planDialog", "shotDialog", "libraries", "extraButtons"} {
		if value, ok := template[key]; ok {
			ordered[key] = value
			delete(template, key)
		}
	}
	rest := make([]string, 0, len(template))
	for key := range template {
		rest = append(rest, key)
	}
	sort.Strings(rest)
	for _, key := range rest {
		ordered[key] = template[key]
	}

	/*
	 * ⚠ 用 `common.Marshal`（紧凑版）而不是缩进版：这一份是**给模型看的**，
	 * 紧凑的 JSON 少几千个空白 token，而形状一个字都没少
	 * （`common` 那一层只提供 `Marshal` 与 `IndentJson`，没有 `MarshalIndent`）。
	 */
	raw, err := common.Marshal(ordered)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// pickShapeSample answers which mode file the example comes from.
//
// ⚠ 偏好**最短的那一份**：示例是给模型看的形状，越短越省预算，
// 而形状本身与长度无关（`words` 28 格、`libraries` 的字段表，哪一份都一样）。
func pickShapeSample() *ModeFile {
	rows := LoadModes()
	var best *ModeFile
	bestScore := 0
	for _, row := range rows {
		if row.Problem != "" || row.Manifest == nil {
			continue
		}
		/*
		 * ⚠ 打分是"**越大越优先**"（这样下面那一下加成好写）：
		 * 文本类加 1 MB —— 示例是给模型看的形状，越短越省预算，
		 * 而文本类那几份没有镜头指令那一大摊（`instruction` 23 段），最短。
		 * 同一类里则取**较长**的那一份：它的字段更全（例如多带几格 `words` 的自查项）。
		 */
		score := len(row.Raw)
		if strings.TrimSpace(stringField(row.Manifest, "medium", "")) == "text" {
			score += 1 << 20
		}
		if best == nil || score > bestScore {
			best = row
			bestScore = score
		}
	}
	return best
}

// placeholderWords replaces every `words` value with a note, keeping the keys.
//
// ⚠★ **键一个不动、值全部换掉**：键是契约（客户端按这些键取词），
// 而值是"这一类作品该用什么词"——那正是要模型去写的。
func placeholderWords(value any) any {
	words, ok := value.(map[string]any)
	if !ok {
		return value
	}
	out := make(map[string]any, len(words))
	for key, raw := range words {
		if nested, isMap := raw.(map[string]any); isMap {
			inner := make(map[string]any, len(nested))
			for innerKey := range nested {
				inner[innerKey] = "（这一格写什么）"
			}
			out[key] = inner
			continue
		}
		out[key] = "（这一格写什么）"
	}
	return out
}

// placeholderPlanDialog shrinks the planning dialog: keys stay, long prose does not.
//
// ★★ 类型是**契约**，所以占位只换"值"，不换形状（2026-… 修，见下面 `placeholderCells` 的说明）。
func placeholderPlanDialog(value any) any {
	dialog, ok := value.(map[string]any)
	if !ok {
		return value
	}
	out := make(map[string]any, len(dialog))
	for key, raw := range dialog {
		switch key {
		case "instruction":
			out[key] = placeholderCells(raw)
		case "templates":
			/*
			 * ⚠★ 字符串数组（不是对象数组）：示例里写成对象，模型就会照着写成
			 * [{"label":…,"prompt":…}]，而客户端那一边 `String(对象)` 得到的是
			 * `"[object Object]"` —— 表现是**开场白丢失**，规划时用的是内置 mv
			 * 的那一句「请你作为顶级MV导演…」（真实踩过）。
			 */
			out[key] = []any{"（这个模式的默认模板：一句话交代「你是谁、要做什么」）"}
		case "audio":
			out[key] = placeholderCells(raw)
		default:
			out[key] = raw
		}
	}
	return out
}

// placeholderCells keeps a nested cell map's **keys and types**, and drops its prose.
//
// ★★ 为什么按"再下一层"走而不是整节塞一句话（2026-… 修）
//
// 这一份示例是模型唯一的形状凭据：它把 `instruction`（23 格）整节换成**一个字符串**时，
// 模型看到的形状就是"instruction 是一个字符串"，于是它照着写了一个字符串。
// 而客户端那一边 `instruction` 必须是对象 —— 读不到那 23 格就**整节回落成内置 mv 的提示词**，
// 于是用户看到的规划提示词永远写着「请你作为顶级MV导演…」（真实踩过的那一次）。
//
// 数组同样处理：值换成一个同形的占位（例如 `["（这一节写什么）"]`），
// **不**改成字符串 —— 类型一变，模型学到的形状就是错的。
func placeholderCells(value any) any {
	cells, ok := value.(map[string]any)
	if !ok {
		return placeholderNote
	}
	out := make(map[string]any, len(cells))
	for key, raw := range cells {
		switch nested := raw.(type) {
		case map[string]any:
			out[key] = placeholderCells(nested)
		case []any:
			out[key] = []any{noteOf("这一节原来是 " + strconv.Itoa(len(nested)) + " 条，照这个数目给这一类作品写几条")}
		default:
			out[key] = placeholderNote
		}
	}
	return out
}

// placeholderNote is the "write this cell" note handed to the model.
const placeholderNote = "（这一格写什么）"

// noteOf builds one of the bracketed shape notes.
func noteOf(text string) string { return "（" + text + "）" }

// videoKindOf normalises one `videoKind` value.
//
// ⚠ 认不出（含客户端清洗之后的兜底值 `"other"`）时回 `fallback` —— 绝不把
// 一个非法字面量照抄进给模型看的示例里。
func videoKindOf(value any, fallback string) string {
	switch strings.TrimSpace(fmt.Sprint(value)) {
	case videoKindAudio:
		return videoKindAudio
	case videoKindFrame:
		return videoKindFrame
	}
	return fallback
}

// placeholderLibraries keeps one library's shape and blanks the rest.
func placeholderLibraries(value any) any {
	list, ok := value.([]any)
	if !ok || len(list) == 0 {
		return value
	}
	first, isMap := list[0].(map[string]any)
	if !isMap {
		return value
	}
	out := map[string]any{}
	for key, raw := range first {
		if key == "fields" {
			out[key] = []any{map[string]any{
				"key":   "name",
				"label": "（这一格叫什么）",
				"type":  "text",
			}}
			continue
		}
		out[key] = raw
	}
	return []any{out}
}

// generatedModeProblem checks the parts of a **generated** mode that the model can get wrong.
//
// ★★ 与 `modeShapeProblem` 的分工（2026-… 加，因为真出过一次）
//
// | | `modeShapeProblem` | ★ 这一个 |
// |---|---|---|
// | 谁用 | **两条路都用**（读盘 + 生成） | 只有「AI 一键生成」那一条路 |
// | 判什么 | 身份那五格（format / version / id / label / medium） | **契约那几格的类型** |
//
// ⚠★ 为什么类型检查**不能**放进 `modeShapeProblem`：它同时是**读盘**那道闸，
// 而模式库是**运营手放上去的地方**（`ZSY_MODE_DIR`）。把类型写死在那里，
// 等于"一份只有 `words` 的老模式从此显示成坏的"—— 而客户端本来能读它
// （`normalizeWorkMode` 逐格回落），那种红是**假警报**，代价是运营不敢动文件。
//
// 而"生成"那一条路上，这些格子是**我们自己刚刚求模型写的**：写错了不该落盘
// （落盘之后用户拿到的是一份"看着正常、跑起来却整节回落成 mv"的模式 ——
// 表现是规划提示词里写着「请你作为顶级MV导演…」，而文件里明明是自己那一档的说明）。
//
// 判据都是从**客户端真的读法**倒推的：
//
//	· `planDialog.instruction` 必须是对象 —— 客户端读不到那 23 格就整节回落 mv
//	· `planDialog.templates` 必须是**字符串**数组、第 0 条非空 —— 它会被放在指令第一行
//	· `planDialog.audio`（给了的话）必须是对象 —— 同一类回落
//	· `words` 要盖住客户端认的那些键 —— 缺哪格落回 mv 的词
//	· `videoKind` 只能是 audio / frame —— 写错会掉进「视频」那一堆
func generatedModeProblem(manifest map[string]any, medium, label string) string {
	dialog, hasDialog := manifest["planDialog"].(map[string]any)
	if !hasDialog {
		if medium == "video" {
			return "生成的这一份没有 planDialog —— 视频类作品的规划弹窗（label / " +
				"title / instruction / templates / audio）是它最主要的一格，" +
				"少了它用户打开「规划」看到的是空白。请重试一次。"
		}
		return ""
	}

	/* ① 模板那一条是"整段指令的第一行"，错了就没有开场白 */
	templates, ok := dialog["templates"].([]any)
	if !ok {
		return fmt.Sprintf("planDialog.templates 应当是**字符串数组**（它是 %s）—— "+
			"它会被放在整段规划指令的第一行；写成对象数组的话客户端会读成 "+
			"\"[object Object]\"，开场白就丢了。请按示例的形状重写这一格。", jsonKindOf(dialog["templates"]))
	}
	if len(templates) == 0 {
		return "planDialog.templates 是空的 —— 它至少要有第 0 条（开场白）。"
	}
	for i, one := range templates {
		if jsonKindOf(one) != "字符串" {
			return fmt.Sprintf("planDialog.templates 第 %d 条不是字符串（它是%s）—— "+
				"这个数组里只能放字符串。", i, jsonKindOf(one))
		}
		if strings.TrimSpace(fmt.Sprint(one)) == "" {
			return fmt.Sprintf("planDialog.templates 第 %d 条是空串 —— "+
				"空的那一条在界面上就是一个空框（点「换一个」会换出空白）。", i)
		}
	}

	/*
	 * ② 分镜指令那 23 格：**视频类作品的命根子**。
	 *
	 * ⚠ 音频 / 文稿那两类**刻意不写**这一格（那一整节讲的是镜头脚本），
	 * 所以只在 medium=video 时要求它。
	 */
	if medium == "video" {
		instruction, isMap := dialog["instruction"].(map[string]any)
		if !isMap {
			return fmt.Sprintf("planDialog.instruction 应当是**对象**（它是%s）—— "+
				"那一节是 23 格（skeleton / framing / songFacts / …），"+
				"客户端读不到这些格就**整节回落成内置 mv 的提示词**，"+
				"用户看到的会是「请你作为顶级MV导演…」。请按示例的形状重写这一格。",
				jsonKindOf(dialog["instruction"]))
		}
		/*
		 * ⚠ 只查**那两格本身就是一层格子**的（`framing` / `songFacts`）：
		 * 其余 21 格是**字符串**（skeleton / ruleCutPoints / …）—— 一起查的话
		 * 每一份合法生成的模式都会被自己判死（这一版第一版就踩了这个坑，
		 * 报出来的是"planDialog.instruction.skeleton 应当是对象"）。
		 *
		 * 而那两格**没有也合法**：客户端对缺格是逐键回落，用户自己写的一格也不算错。
		 * 这里管的是**有、但被写成了一句话**那种（提示词里那句话被照抄回来的形状）。
		 */
		for _, key := range instructionObjectCells {
			if problem := objectCellProblem(instruction, "planDialog.instruction", key); problem != "" {
				return problem
			}
		}
	}

	/* ③ `audio` 那一节：给了就必须是对象 */
	if _, given := dialog["audio"]; given {
		if _, isMap := dialog["audio"].(map[string]any); !isMap {
			return fmt.Sprintf("planDialog.audio 应当是**对象**（它是%s）—— "+
				"它是 15 格（audioScriptLead / exampleSpeak / segmentRule / gen / …）。"+
				"写成一整段字符串的话客户端读不到那些格。", jsonKindOf(dialog["audio"]))
		}
	}

	/* ④ 词表：客户端认哪些键，就得写哪些键 */
	words, ok := manifest["words"].(map[string]any)
	if !ok {
		return "生成的这一份没有 words —— 界面上那些词全落回内置 mv 那一套" +
			"（按钮上写着「歌词」「镜头」）。请补上这一节。"
	}
	var missing []string
	for _, key := range modeWordKeys() {
		if _, written := words[key]; !written {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return fmt.Sprintf("words 少写了 %d 格：%s —— 客户端认的是这 %d 格，"+
			"缺的那些会逐格回落成内置 mv 的词（音频/文稿类尤其明显）。"+
			"请把这一节补全（每一格都要写，包括这一档确实用不到的那几格）。",
			len(missing), strings.Join(missing, " / "), len(modeWordKeys()))
	}

	/* ⑤ 视频那一家的小类 */
	if medium == "video" {
		kind := videoKindOf(manifest["videoKind"], "")
		if kind == "" {
			return fmt.Sprintf("videoKind 应当是 %s 或 %s（收到的是 %q）—— "+
				"它决定这一档在「模式广场」里摆进「音频生视频」还是「首尾帧视频」；"+
				"认不出的值会落进「视频」那一堆，而界面上看不出异常。",
				videoKindAudio, videoKindFrame, fmt.Sprint(manifest["videoKind"]))
		}
	}

	/* ⑥ 占位说明**不许**当成内容落盘（提示词里那些句子被照抄回来过） */
	if note := placeholderLeakOf(manifest); note != "" {
		return fmt.Sprintf("生成的这一份把**形状提示**当成了内容：%s —— "+
			"「（这一格写什么）」这种句子是给你看的占位说明，"+
			"每一格都要写这一类作品真的能用的整句话。", note)
	}

	/* ⑦ 与读盘同源的那一道闸（身份那五格） */
	if problem := modeShapeProblem(manifest, strings.TrimSpace(label)); problem != "" {
		return problem
	}
	return ""
}

// instructionObjectCells lists the instruction cells that are **themselves a layer
// of cells** —— the other 21 are plain strings.
//
// ⚠★ 这张表是**客户端契约的副本**（`core/workModes.js` 的 `normalizeInstruction`：
// `framing` / `songFacts` 是逐键回落的、`framing.sectionLabels` 还再下一层，
// 其余每一格都是一句话）。把它写在这里而不是"凡对象皆可"，
// 是为了让"字符串那 21 格"不被误判成错的 —— 那会把每一份合法生成的模式都判死。
var instructionObjectCells = []string{"framing", "songFacts"}

// objectCellProblem checks one nested cell that must itself be an object.
//
// ⚠ **没有那一格不算错**（客户端对缺格是逐键回落）；错的是"有、但被写成了一句话"。
func objectCellProblem(cells map[string]any, path, key string) string {
	value, written := cells[key]
	if !written {
		return ""
	}
	if _, isMap := value.(map[string]any); !isMap {
		return fmt.Sprintf("%s.%s 应当是**对象**（它是%s）—— 它自己是一层格子（各写一句话），"+
			"而客户端读不到它就逐键回落成内置 mv 的那几句。",
			path, key, jsonKindOf(value))
	}
	return ""
}

// placeholderLeakOf answers the first leftover shape note, or "".
//
// ⚠ 只认提示词里那两种写法（`（这一格写什么）` / `（这一节……）`）：
// 匹配得太宽会把用户自己写的括号内容也判成泄漏，那种红没人能解释。
func placeholderLeakOf(value any) string {
	switch node := value.(type) {
	case map[string]any:
		for _, raw := range node {
			if hit := placeholderLeakOf(raw); hit != "" {
				return hit
			}
		}
	case []any:
		for _, raw := range node {
			if hit := placeholderLeakOf(raw); hit != "" {
				return hit
			}
		}
	case string:
		trimmed := strings.TrimSpace(node)
		if strings.Contains(trimmed, "（这一格写什么）") || strings.HasPrefix(trimmed, "（这一节") {
			return trimmed
		}
	}
	return ""
}

// jsonKindOf names a JSON value's kind the way the error messages talk about it.
func jsonKindOf(value any) string {
	switch value.(type) {
	case nil:
		return "空的"
	case string:
		return "字符串"
	case bool:
		return "布尔"
	case float64, int, int64:
		return "数字"
	case []any:
		return "数组"
	case map[string]any:
		return "对象"
	}
	return "认不出的值"
}

// nonEmpty answers the fallback when a value is blank.
func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
