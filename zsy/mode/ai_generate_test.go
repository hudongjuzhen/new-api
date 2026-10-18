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

// TestModeShapeTemplate_KeepsTheCellTypes 盯的是"示例里那几格的**类型**"。
//
// ★★ 这是 2026-10-14 修的那个真缺陷的守卫：示例原来把 `instruction`（23 格）
// 与 `audio`（15 格）各压成**一个字符串**，于是模型看到的形状就是
// "instruction 是一个字符串"，它照着写了 —— 而客户端那一边 `instruction`
// 必须是对象，读不到那 23 格就**整节回落成内置 mv 的提示词**。
// 用户那边的表现是：自己那一档的说明明明写着「你是资深短剧编剧…」，
// 跑起来发出去的却是「请你作为顶级MV导演…」。
func TestModeShapeTemplate_KeepsTheCellTypes(t *testing.T) {
	useModeDir(t, map[string]string{"sample.json": aValidModeWithWords("sample", "video")})

	template := modeShapeTemplate()
	var parsed map[string]any
	require.NoError(t, common.UnmarshalJsonStr(template, &parsed))

	dialog, ok := parsed["planDialog"].(map[string]any)
	require.True(t, ok, "示例里应当有 planDialog")

	/* ① instruction 还是**对象**，而且再下一层也是（模型靠这个看见那 23 格） */
	instruction, ok := dialog["instruction"].(map[string]any)
	require.True(t, ok, "planDialog.instruction 在示例里必须是对象，不能是一句话")
	assert.Contains(t, instruction, "skeleton")
	assert.Contains(t, instruction, "framing")
	framing, ok := instruction["framing"].(map[string]any)
	require.True(t, ok, "framing 在示例里也必须是对象（它自己是一层格子）")
	assert.Contains(t, framing, "specHeading")
	labels, ok := framing["sectionLabels"].(map[string]any)
	require.True(t, ok, "再下一层（sectionLabels）同样是对象")
	assert.Contains(t, labels, "summary")
	/* ⚠ 数组那一格仍然是数组（类型变了模型就学错形状） */
	_, isList := instruction["techniquesAiList"].([]any)
	assert.True(t, isList, "数组那一格在示例里必须还是数组")

	/* ② audio 也必须是对象（同一条理由） */
	audio, ok := dialog["audio"].(map[string]any)
	require.True(t, ok, "planDialog.audio 在示例里必须是对象，不能是一句话")
	assert.Contains(t, audio, "audioScriptLead")
	_, genIsMap := audio["gen"].(map[string]any)
	assert.True(t, genIsMap, "audio.gen 也留着形状")

	/* ③ templates 是**字符串数组**：写成对象数组的话客户端会读成 "[object Object]" */
	templates, ok := dialog["templates"].([]any)
	require.True(t, ok, "planDialog.templates 必须是数组")
	require.NotEmpty(t, templates)
	_, firstIsString := templates[0].(string)
	assert.True(t, firstIsString, "templates 的每一条都必须是字符串（开场白就在第 0 条）")
}

// TestBuildModeCreateInstruction_NailsTheContractCells 盯的是提示词**有没有把话说清**。
//
// ⚠ 光让示例"形状对"是不够的：模型还得知道**这几格的类型是硬契约**、
// 以及写错的后果是什么。这一条把那段话钉住 —— 它被删掉时没有任何别的用例会红。
func TestBuildModeCreateInstruction_NailsTheContractCells(t *testing.T) {
	useModeDir(t, map[string]string{"sample.json": aValidModeWithWords("sample", "video")})

	prompt := buildModeCreateInstruction("我要做短剧", "短剧", "video", "一集一集地做", "short-drama")

	assert.Contains(t, prompt, "planDialog.instruction 必须是对象", "没告诉模型 instruction 的类型")
	assert.Contains(t, prompt, "planDialog.templates 必须是字符串数组", "没告诉模型 templates 的类型")
	assert.Contains(t, prompt, "planDialog.audio 必须是对象", "没告诉模型 audio 的类型")
	assert.Contains(t, prompt, "videoKind", "没告诉模型视频那一家的 videoKind")
	assert.Contains(t, prompt, "顶级MV导演", "没把「写错的后果」说清（这一句正是用户看到的那一句）")
}

// TestGeneratedModeProblem_CatchesWhatTheClientWouldSilentlyFallBackOn 是本轮那条真缺陷的守卫。
//
// ★★ 判据的来源是**客户端真的读法**（`core/workModes.js` 的回落链）：
// 类型写错**不会报错**，而是逐格/整节回落成内置 mv 那一套 —— 于是
// "模式装上了、看着正常，跑起来却是别人的提示词"。所以这一道闸必须在**落盘之前**。
func TestGeneratedModeProblem_CatchesWhatTheClientWouldSilentlyFallBackOn(t *testing.T) {
	cases := []struct {
		name  string
		spoil func(m map[string]any)
		want  string
	}{
		{
			name: "instruction 写成了一个字符串（真踩过的那一次）",
			spoil: func(m map[string]any) {
				m["planDialog"].(map[string]any)["instruction"] = "你是资深短剧编剧兼分镜导演…"
			},
			want: "instruction 应当是**对象**",
		},
		{
			name: "instruction 整节没了（视频类作品）",
			spoil: func(m map[string]any) {
				delete(m["planDialog"].(map[string]any), "instruction")
			},
			want: "instruction 应当是**对象**",
		},
		{
			name: "framing 被并成一句话",
			spoil: func(m map[string]any) {
				m["planDialog"].(map[string]any)["instruction"].(map[string]any)["framing"] = "（参考那一节）"
			},
			want: "framing 应当是**对象**",
		},
		{
			name: "audio 被并成一句话",
			spoil: func(m map[string]any) {
				m["planDialog"].(map[string]any)["audio"] = "规划每集的配音与音乐。"
			},
			want: "audio 应当是**对象**",
		},
		{
			name: "templates 里混进了对象（开场白会丢成 [object Object]）",
			spoil: func(m map[string]any) {
				m["planDialog"].(map[string]any)["templates"] = []any{
					map[string]any{"label": "逆袭爽剧", "prompt": "节奏快、反转密集"},
				}
			},
			want: "templates 第 0 条不是字符串",
		},
		{
			name: "templates 整格写成了一个字符串",
			spoil: func(m map[string]any) {
				m["planDialog"].(map[string]any)["templates"] = "都市逆袭 / 悬疑反转 / 情感甜宠"
			},
			want: "templates 应当是**字符串数组**",
		},
		{
			name: "templates 是空的",
			spoil: func(m map[string]any) {
				m["planDialog"].(map[string]any)["templates"] = []any{}
			},
			want: "至少要有第 0 条",
		},
		{
			name: "words 少了几格",
			spoil: func(m map[string]any) {
				delete(m["words"].(map[string]any), "planDialog")
			},
			want: "words 少写了",
		},
		{
			name:  "videoKind 是客户端那个兜底值 other",
			spoil: func(m map[string]any) { m["videoKind"] = "other" },
			want:  "videoKind 应当是",
		},
		{
			name:  "videoKind 干脆没写",
			spoil: func(m map[string]any) { delete(m, "videoKind") },
			want:  "videoKind 应当是",
		},
		{
			name: "形状提示被当成了内容（「（这一格写什么）」）",
			spoil: func(m map[string]any) {
				m["planDialog"].(map[string]any)["instruction"].(map[string]any)["skeleton"] = "（这一格写什么）"
			},
			want: "形状提示",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useModeDir(t, map[string]string{"sample.json": aValidModeWithWords("sample", "video")})
			generated := generatedFrom(t, "food-shop", "美食探店", "video")
			/* 这一档要当视频类审，所以 videoKind 得先有一份合法的 */
			generated["videoKind"] = videoKindAudio
			tc.spoil(generated)

			problem := generatedModeProblem(generated, "video", "food-shop")
			require.NotEmpty(t, problem, "这一份应当被拦下")
			assert.Contains(t, problem, tc.want)
		})
	}
}

// TestGeneratedModeProblem_AcceptsWhatTheModelShouldWrite 是上面那一条的反面。
//
// ⚠★ 没有这一条的话，把函数写成 `return "不合格"` 也全绿 —— 而"生成一次都成功不了"
// 比"偶尔放过一份坏的"更坏（用户点一下、等半天、拿到一句"请把需求写得更具体一点再试一次"）。
func TestGeneratedModeProblem_AcceptsWhatTheModelShouldWrite(t *testing.T) {
	useModeDir(t, map[string]string{"sample.json": aValidModeWithWords("sample", "video")})

	/* ① 视频那一档：instruction + audio + videoKind 都在 */
	video := generatedFrom(t, "food-shop", "美食探店", "video")
	video["videoKind"] = videoKindFrame
	assert.Empty(t, generatedModeProblem(video, "video", "food-shop"))

	/* ② 音频那一档：**刻意不写** instruction / audio / videoKind（那一节讲的是镜头） */
	audio := generatedFrom(t, "mypodcast", "我的播客", "audio")
	delete(audio["planDialog"].(map[string]any), "instruction")
	delete(audio["planDialog"].(map[string]any), "audio")
	assert.Empty(t, generatedModeProblem(audio, "audio", "mypodcast"))

	/* ③ 判据跟着**磁盘上那几份**走：words 的键集来自模式库，不是写死的一张表 */
	require.NotEmpty(t, modeWordKeys(), "先决条件：模式库里得有文件，不然 words 那一节无从判起")
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
//
// ⚠★ `planDialog` 那几格的**类型**是这一份夹具的重点（2026-…）：
// `instruction` / `audio` 是**对象**、`templates` 是**字符串数组** —— 与真文件同形。
// 从前这里把 `audio` 写成一个字符串，于是"示例里 instruction 是个字符串"
// 那类缺陷在这份夹具上一点都看不出来（真实踩过的那一次就是这样溜过去的）。
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
    "title": "AI 全稿规划",
    "instruction": {
      "skeleton": "（六个分节的骨架）",
      "ruleCutPoints": "（切点怎么选）",
      "framing": { "specHeading": "【参考】", "sectionLabels": { "summary": "概述" } },
      "techniquesAiList": ["一", "二"]
    },
    "templates": ["这是模板里的第一条"],
    "audio": { "audioScriptLead": "（音频脚本怎么写）", "gen": { "maxSecs": 100 } }
  },
  "libraries": [
    { "id": "characters", "label": "角色库", "dir": "characters", "itemFile": "item.json",
      "fields": [{ "key": "name", "label": "人物名称", "type": "text" }] }
  ],
  "extraButtons": []
}`
}

// generatedFrom builds the map `GenerateMode` validates: the model's JSON plus
// the identity cells we override ourselves.
//
// ⚠★ 上面拿它建"合法的那一份"、下面那一条建"要落地的那一份" —— 两处共用同一个形状，
// 所以"模型回包 → 覆盖身份 → 校验 → 落盘"这条链上**只有一处**定义什么叫合法。
func generatedFrom(t *testing.T, id, label, medium string) map[string]any {
	t.Helper()
	generated := map[string]any{}
	require.NoError(t, common.UnmarshalJsonStr(aValidModeWithWords(id, medium), &generated))
	generated["format"] = modeFormat
	generated["formatVersion"] = modeFormatVersion
	generated["id"] = id
	generated["label"] = label
	generated["medium"] = medium
	return generated
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
