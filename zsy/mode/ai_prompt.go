package mode

import (
	"fmt"
	"sort"
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
		"- planDialog：规划弹窗（label / title / instruction / templates / audio）",
		"- shotDialog：镜头或版本弹窗（label）",
		"- requires：四个布尔 —— audio / lyrics / refs / segments（这项作品必填哪些）",
		"- libraries：资料库表（id / label / dir / itemFile / fields）",
		"- extraButtons：额外按钮（可以空数组）",
		"- appRows / lipSync / core / coreLabels（可以照示例给一份合理的）",
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
func placeholderPlanDialog(value any) any {
	dialog, ok := value.(map[string]any)
	if !ok {
		return value
	}
	out := make(map[string]any, len(dialog))
	for key, raw := range dialog {
		switch key {
		case "instruction":
			out[key] = placeholderInstruction(raw)
		case "templates":
			out[key] = []any{"（这个模式的默认模板：一句话交代「你是谁、要做什么」）"}
		case "audio":
			out[key] = "（这一类作品要生成的音频提示词，没有就留空串）"
		default:
			out[key] = raw
		}
	}
	return out
}

// placeholderInstruction keeps the instruction's **keys** and drops its prose.
func placeholderInstruction(value any) any {
	instruction, ok := value.(map[string]any)
	if !ok {
		return "（规划指令：按下面这些分节写，每一段都要写）"
	}
	out := make(map[string]any, len(instruction))
	for key, raw := range instruction {
		if nested, isMap := raw.(map[string]any); isMap {
			inner := make(map[string]any, len(nested))
			for innerKey := range nested {
				inner[innerKey] = "（这一格写什么）"
			}
			out[key] = inner
			continue
		}
		if list, isList := raw.([]any); isList {
			out[key] = []any{fmt.Sprintf("（这一节写什么，原来有 %d 条）", len(list))}
			continue
		}
		out[key] = "（这一段指令写什么）"
	}
	return out
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

// nonEmpty answers the fallback when a value is blank.
func nonEmpty(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}
