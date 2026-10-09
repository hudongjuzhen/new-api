package mode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// 「AI 一键生成模式」那一条链（用户 2026-…）
//
// 这一组全是**纯函数**（提示词、回包解析、id 推导）—— 真正打网络的那一段
// （`callGenerationModel`）不在这里测：它要一个活的站内中继与一个配好的模型，
// 而那件事在集成环境里验（`docs` 里那条"探针"的纪律）。
//
// 它守的是三个**不报错**的坏法：
//
//	1. ★★ 模型把 JSON 包在 ```json 里、或前后带两句客套话 —— 那**不是失败**
//	2. ★★ 模型给的 JSON 后面还接了一段解释 —— 切错位置会得到"解析不了"
//	3. ★ 提示词里**不许**出现源码里那些长篇正文（它会撑爆预算，而且模型会照抄）
// =========================================================================

func TestExtractModeJSON_HandlesTheUsualWrappers(t *testing.T) {
	cases := []struct {
		name   string
		answer string
		wantID string
	}{
		{"光秃秃的一份", `{"id":"food","label":"美食"}`, "food"},
		{"```json 围栏", "```json\n{\"id\":\"food\",\"label\":\"美食\"}\n```", "food"},
		{"``` 围栏（没写语言）", "```\n{\"id\":\"food\"}\n```", "food"},
		{"前面带客套话", "好的，这是这一份模式：\n{\"id\":\"food\"}", "food"},
		{"后面接一段解释", "{\"id\":\"food\"}\n\n上面这份就是全部内容，你可以按需调整。", "food"},
		{"里面带花括号的字符串", `{"id":"food","words":{"text":"他说：\"{\" 这个符号"}}`, "food"},
		{"里面带转义引号", `{"id":"food","words":{"text":"反斜杠 \\ 与引号 \""}}`, "food"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractModeJSON(tc.answer)
			require.NoError(t, err)
			assert.Equal(t, tc.wantID, got["id"])
		})
	}
}

func TestExtractModeJSON_FailsWithSomethingActionable(t *testing.T) {
	/* ① 完全没有 JSON */
	_, err := extractModeJSON("我觉得这个需求需要先想清楚，你想做什么类型的片子呢？")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "没有给出 JSON")

	/* ② 被截断（没有闭合） */
	_, err = extractModeJSON(`{"id":"food","words":{"text":"美食"`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "没有闭合")

	/* ③ 闭合了但不是合法 JSON（这里多了一个逗号） */
	_, err = extractModeJSON(`{"id":"food",}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "解析不了")
}

func TestSuggestModeID_DerivesFromLabelAndAvoidsCollisions(t *testing.T) {
	useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})

	id, err := suggestModeID("Food Shop Story")
	require.NoError(t, err)
	assert.Equal(t, "food-shop-story", id)

	/* ⚠ 中文名推不出有意义的 slug（回落到一个能用的 id，而不是报错） */
	id, err = suggestModeID("美食探店")
	require.NoError(t, err)
	assert.True(t, isValidModeID(id), "回落出来的 id 也得是合法形状：%q", id)
	assert.True(t, strings.HasPrefix(id, "mode-"), "回落应当是 mode-<时间戳>：%q", id)

	/* ⚠ 撞名要躲开（不许悄悄覆盖已有的那一档） */
	id, err = suggestModeID("mv")
	require.NoError(t, err)
	assert.Equal(t, "mv-2", id)
}

func TestSuggestModeID_TrimsToTheLegalShape(t *testing.T) {
	useModeDir(t, map[string]string{})

	id, err := suggestModeID("  夜里 的 城市 —— 短片!!!  ")
	require.NoError(t, err)
	assert.True(t, isValidModeID(id), "推出来的 id 必须合法：%q", id)

	id, err = suggestModeID(strings.Repeat("a", 60))
	require.NoError(t, err)
	assert.LessOrEqual(t, len(id), 32, "id 最长 32：%q", id)
}

func TestBuildModeCreateInstruction_PinsIdentityAndShrinksTheTemplate(t *testing.T) {
	useModeDir(t, map[string]string{"sample.json": aValidModeWithWords("sample", "text")})

	prompt := buildModeCreateInstruction("我要做美食探店类的短视频", "美食探店", "video", "一分钟讲一家店", "food-shop")

	/* ① 用户那几个输入都要在（模型得知道"给谁写、写什么"） */
	assert.Contains(t, prompt, "我要做美食探店类的短视频")
	assert.Contains(t, prompt, "美食探店")
	assert.Contains(t, prompt, "video")
	assert.Contains(t, prompt, "一分钟讲一家店")

	/* ② ★ id 是**我们给的**（它是文件名，那条形状不交给模型记） */
	assert.Contains(t, prompt, `id：food-shop`)
	assert.Contains(t, prompt, `原样是 "food-shop"`)

	/* ④ ★ 形状示例里有真的契约（format / formatVersion / 那几格） */
	assert.Contains(t, prompt, modeFormat)
	assert.Contains(t, prompt, `"formatVersion":1`)

	/* ⑤ ★★ 不许让模型写 `basedOn`（本格式里没有这一格） */
	assert.Contains(t, prompt, "不要写 basedOn")
}

func TestModeShapeTemplate_KeepsTheContractAndDropsTheProse(t *testing.T) {
	/*
	 * ★★ 形状示例必须是**从真的模式文件里抽出来的**（所以形状不会与契约分叉），
	 * 同时把那些"一整段的话"换成占位说明 —— 否则几千字的正文会把提示词撑到几万
	 * token，而模型会**照抄**它们（示例里是视频那一档的写法，音频/文稿类要换掉）。
	 */
	useModeDir(t, map[string]string{"sample.json": aValidModeWithWords("sample", "text")})

	template := modeShapeTemplate()

	/* ⚠★ 正文那些字**一个都不该在**（它们在夹具文件里，而夹具那一份是"视频那一档的写法"） */
	for _, prose := range []string{"歌词", "导入音频", "（六个分节的骨架）", "这是模板里的第一条"} {
		assert.NotContains(t, template, prose, "示例里不该带上 %q 这一句正文", prose)
	}

	/* ★ 键一个不动（它们是**契约**：客户端按这些键取词） */
	for _, key := range modeWordKeys() {
		assert.Contains(t, template, `"`+key+`"`, "words 的键 %q 应当在示例里", key)
	}
	/* ★ 值换成了占位说明 */
	assert.Contains(t, template, "（这一格写什么）")
	/* ★ 嵌套那一层也换了（`planSelfCheck` 那种） */
	assert.Contains(t, template, `"planSelfCheck":{"musicName":"（这一格写什么）"`)

	/* ⚠ 不是合法 JSON 的话模型读不出形状（这份示例是嵌在提示词里的） */
	assert.True(t, strings.HasPrefix(template, "{"), "示例应当是 JSON 对象")
	var parsed map[string]any
	require.NoError(t, common.UnmarshalJsonStr(template, &parsed))
	assert.Equal(t, modeFormat, parsed["format"])
}

func TestBuildModeCreateInstruction_ListsEveryWordKeyFromTheShippedModes(t *testing.T) {
	/*
	 * ★★ 字段清单必须是**从磁盘上真的那几份模式里生成**的：手写一份就会与真实契约
	 * 分叉，而分叉的表现是"模型写了一个客户端不认识的键"（存进文件、谁都不报错）。
	 */
	useModeDir(t, map[string]string{
		"a.json": aValidModeWithWords("a", "text"),
		"b.json": aValidModeWithWords("b", "text"),
	})

	prompt := buildModeCreateInstruction("随便", "测试", "text", "", "test-mode")
	for _, key := range modeWordKeys() {
		assert.Contains(t, prompt, key, "words 的键 %q 没在提示词里", key)
	}
	assert.Contains(t, prompt, "每一格都要写")
}

// aValidModeWithWords builds a mode file with a realistic `words` section.
func aValidModeWithWords(id, medium string) string {
	return `{
  "format": "aimv-work-mode",
  "formatVersion": 1,
  "id": "` + id + `",
  "label": "示例",
  "medium": "` + medium + `",
  "workScale": "single",  "words": {
    "audio": "导入音频",
    "text": "歌词",
    "planDialog": "AI 全稿规划",
    "planSelfCheck": { "musicName": "音频", "refUse": "只有角色那几条会被编成参考" }
  },
  "requires": { "audio": true, "lyrics": false, "refs": false, "segments": true },
  "planDialog": {
    "label": "AI 全稿规划",
    "instruction": { "skeleton": "（六个分节的骨架）", "techniquesAiList": ["一", "二"] },
    "templates": ["这是模板里的第一条"],
    "audio": "音频提示词"
  },
  "libraries": [
    { "id": "characters", "label": "角色库", "dir": "characters", "itemFile": "item.json",
      "fields": [{ "key": "name", "label": "人物名称", "type": "text" }] }
  ],
  "extraButtons": []
}`
}

func TestGeneratedModeIsAcceptedAndStored(t *testing.T) {
	/*
	 * ★★ 这一条走的是**模型回包那一段之后**的整条落地链（不碰网络）：
	 *
	 *	模型给的 JSON → 覆盖身份那几格 → `modeShapeProblem` → 写盘 → 读回来
	 *
	 * 它替下的是"打不通模型就没法验落地"这个洞：真正要证明的是
	 * **一份模型形状的答案能不能变成一份服务端认、客户端也读得出的模式文件**。
	 * 剩下那一小段（真的发一次 HTTP）要活的站内中继与管理员的密钥，由集成环境验。
	 */
	dir := useModeDir(t, map[string]string{})

	/* ① 模型"回"了一份 JSON（这里是 `extractModeJSON` 之后的产物形状） */
	generated := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(aValidModeWithWords("x", "video"), &generated))

	/* ② 覆盖身份那几格（`GenerateMode` 里那一步：以运营填的为准） */
	generated["format"] = modeFormat
	generated["formatVersion"] = modeFormatVersion
	generated["id"] = "food-shop"
	generated["label"] = "美食探店"
	generated["medium"] = "video"
	generated[summaryKey] = "一分钟讲一家店"
	generated[visibilityKey] = VisibilityPrivate

	/*
	 * ③ 校验（与读盘同源的那道闸）。
	 *
	 * ⚠★ 这一行**抓到过一个真错**：上面那几格是**在 Go 里拼出来的**（`modeFormatVersion`
	 * 进 `map[string]any` 之后是 `int`），而 `numberField` 原来只认 `float64`
	 * —— 于是"从 JSON 读回来的那份能保存、生成出来的这份被判成'没有写 formatVersion'"。
	 * 那条路正是"AI 一键生成"，它会**一次都成功不了**，而那句话说的是假话。
	 */
	require.Empty(t, modeShapeProblem(generated, "food-shop"))

	/* ④ 落盘 */
	_, err := writeNewMode("food-shop", generated)
	require.NoError(t, err)

	/* ⑤ 读回来：服务端认它，而且每一格都在 */
	row, found := ModeByID("food-shop")
	require.True(t, found)
	assert.Empty(t, row.Problem)
	assert.Equal(t, "美食探店", row.Label)
	assert.Equal(t, "video", row.Medium)
	assert.Equal(t, VisibilityPrivate, row.Visibility)
	assert.Equal(t, "一分钟讲一家店", row.Summary)

	/* ⑥ 文件确实在那儿，而且是**服务端会发出去的那一份** */
	_, err = os.Stat(filepath.Join(dir, "food-shop.json"))
	assert.NoError(t, err)
	rendered, err := RenderModeFile(row)
	require.NoError(t, err)
	assert.Contains(t, rendered, `"food-shop"`)
	/* ⚠ `x-` 那两格**不许**跟着发到客户端（`RenderModeFile` 会拿掉它们） */
	assert.NotContains(t, rendered, visibilityKey)
	assert.NotContains(t, rendered, summaryKey)
}

func TestWriteNewMode_RefusesToOverwriteAnExistingMode(t *testing.T) {
	/*
	 * ⚠★ "新建"与"编辑"是两件事：覆盖一档已有的模式必须走编辑弹窗
	 * （那里的判据是"运营看着它改的"），而这一条路上悄悄覆盖会让别人那一档
	 * **无声地变样**。
	 */
	dir := useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})
	before, err := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, err)

	content := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(aValidMode("mv", "video"), &content))
	_, err = writeNewMode("mv", content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已经在了")

	after, err := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after), "拒绝覆盖时磁盘不该被动过")
}
