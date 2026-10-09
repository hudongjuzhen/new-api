package mode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// 模式**正文**的读与写（用户 2026-…）
//
// 用户的原话：
//
//	"在编辑弹窗里……里面有这个模式的具体内容编辑，根据 json 中的内容做编辑"
//
// 这一组守的是三件事，每一件错了都**不报错**、只是把坏东西写进磁盘：
//
//	1. ★★ **先校验、后落盘**：填错的表单不许碰磁盘（那份文件是客户端会下到本机的）
//	2. ★★ 写完之后**仍然是一份合法模式**（format / id / label / medium + 可见性）
//	3. ★ 键序按**磁盘上那份**走（否则"这次改了什么"就 diff 不出来了）
// =========================================================================

func TestReadModeContent_ReturnsBodyAndFillsMissingHostKeys(t *testing.T) {
	/*
	 * ⚠★ 这一条盯的是一个**很容易漏**的坏法：文件里**没写** `x-visibility`
	 * （老模式文件都没写），而界面读回来的那一格是空的 ——
	 * 于是运营打开它、只改了一个词、顺手保存，**可见性那一格就被清掉了**。
	 */
	useModeDir(t, map[string]string{
		"mv.json": `{
  "format": "aimv-work-mode",
  "formatVersion": 1,
  "id": "mv",
  "label": "MV",
  "medium": "video",
  "words": { "text": "歌词" }
}`,
	})

	content, err := ReadModeContent("mv")
	require.NoError(t, err)
	assert.Equal(t, "mv", content.ID)
	assert.Equal(t, "歌词", content.Content["words"].(map[string]any)["text"])

	/* ★ 没写的那两格**补上服务端认的那个值**（默认 private，与服务端同源） */
	assert.Equal(t, VisibilityPrivate, content.Content[visibilityKey])
	assert.Equal(t, "", content.Content[summaryKey])
}

func TestReadModeContent_RefusesMissingAndBrokenFiles(t *testing.T) {
	useModeDir(t, map[string]string{
		"bad.json": `{"format":"aimw-work-mode","formatVersion":1,"id":"bad","label":"坏","medium":"video"}`,
	})

	_, err := ReadModeContent("nope")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "模式库里没有")

	/* ⚠ 坏文件**不给读**（也说清为什么）：给运营一个表单去改它只会把问题搅得更乱 */
	_, err = ReadModeContent("bad")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "先修好它再编辑")
}

func TestWriteModeContent_RejectsBadShapeAndLeavesTheFileAlone(t *testing.T) {
	dir := useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})
	before, err := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, err)

	cases := []struct {
		name  string
		patch map[string]any
		want  string
	}{
		{"format 写错", map[string]any{"format": "aimw-work-mode"}, "不是模式文件"},
		{"id 与文件名不一致", map[string]any{"id": "other"}, "文件名与 id 不一致"},
		{"label 空着", map[string]any{"label": "   "}, "没有写 label"},
		{"medium 认不出", map[string]any{"medium": "movie"}, "本应用不认识"},
		{"可见性认不出", map[string]any{visibilityKey: "publik"}, "只能写 public / private"},
		{"可见性不是字符串", map[string]any{visibilityKey: 3}, "只能是 public / private"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			content, readErr := ReadModeContent("mv")
			require.NoError(t, readErr)
			for k, v := range tc.patch {
				content.Content[k] = v
			}

			_, writeErr := WriteModeContent("mv", content.Content)
			require.Error(t, writeErr)
			assert.Contains(t, writeErr.Error(), tc.want)

			/*
			 * ★★ 判据是"**磁盘一个字节都没动**"（不只是"返回了一个错误"）：
			 * 先写后校验的表现是那份好好的模式被一份填错的表单覆盖掉。
			 */
			after, readBack := os.ReadFile(filepath.Join(dir, "mv.json"))
			require.NoError(t, readBack)
			assert.Equal(t, string(before), string(after))
		})
	}
}

func TestWriteModeContent_SavesAndKeepsItValid(t *testing.T) {
	dir := useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})

	content, err := ReadModeContent("mv")
	require.NoError(t, err)
	content.Content["words"].(map[string]any)["text"] = "台词"
	content.Content[visibilityKey] = VisibilityPrivate

	rendered, err := WriteModeContent("mv", content.Content)
	require.NoError(t, err)

	/* ★ 写完之后仍然是一份**合法、能发出去**的模式（跑一遍读盘那套校验） */
	row, found := ModeByID("mv")
	require.True(t, found)
	assert.Empty(t, row.Problem, "写完之后读回来被判成坏文件了：%s", row.Problem)
	assert.NoError(t, err)
	assert.Equal(t, VisibilityPrivate, row.Visibility)
	assert.Equal(t, "台词", row.Manifest["words"].(map[string]any)["text"])

	/* ⚠ `.bak` 留的是**改之前**那一版（运营唯一的回头路） */
	bak, bakErr := os.ReadFile(filepath.Join(dir, "mv.json.bak"))
	require.NoError(t, bakErr)
	assert.Contains(t, string(bak), "歌词")

	/* ⚠ 写的是**磁盘上那份**（`rendered` 与文件内容逐字相同） */
	onDisk, diskErr := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, diskErr)
	assert.Equal(t, rendered, string(onDisk))
}

func TestWriteModeContent_KeepsFileKeyOrder(t *testing.T) {
	/*
	 * ⚠★ 键序按**磁盘上那份**走。让 Go 的 `map[string]any` 去序列化会按字母序重排
	 * （`appRows` 跑到最前）—— 于是运营只改了一个词，磁盘上那份却整篇重排，
	 * "这次改了什么"就再也 diff 不出来了。
	 */
	useModeDir(t, map[string]string{
		"mv.json": `{
  "format": "aimv-work-mode",
  "formatVersion": 1,
  "id": "mv",
  "label": "MV",
  "medium": "video",
  "words": { "text": "歌词" }
}`,
	})

	content, err := ReadModeContent("mv")
	require.NoError(t, err)
	content.Content["appRows"] = map[string]any{"sd": "1"}
	content.Content["words"].(map[string]any)["text"] = "台词"

	rendered, err := WriteModeContent("mv", content.Content)
	require.NoError(t, err)

	/* `format` 仍然是第一行（不是 `appRows`） */
	lines := strings.Split(rendered, "\n")
	require.Greater(t, len(lines), 3)
	assert.Contains(t, lines[1], `"format"`, "顶层第一个键应当是 format，而实际是：%s", lines[1])
	/* ⚠ 新加的键追加在**老键之后**（`appRows` 排在 `words` 后面），不是插到中间去 */
	body := strings.TrimRight(rendered, "\n")
	assert.Contains(t, body, "\"appRows\"")
	assert.Less(
		t,
		strings.Index(body, "\"words\""),
		strings.Index(body, "\"appRows\""),
		"新加的键应当排在老键之后，实际渲染：\n%s", rendered,
	)
	/* ⚠ 嵌套那一格也得缩进对（不然是一份"合法但没法读"的 JSON） */
	assert.Contains(t, rendered, "\n    \"text\": \"台词\"")
}

func TestWriteModeContent_NoChangeWritesNothing(t *testing.T) {
	/*
	 * ⚠ 没变就**不写**：白写一次会让"改过没有"这件事看不出来，
	 * 而 `.bak` 会被无谓地覆盖掉（运营唯一的回头路就没了）。
	 */
	dir := useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})
	before, err := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, err)

	content, err := ReadModeContent("mv")
	require.NoError(t, err)
	rendered, err := WriteModeContent("mv", content.Content)
	require.NoError(t, err)

	/*
	 * ★★ 判据是"**逐字节没动**"：这一份是人手写的（`"words": { "text": "歌词" }` 在一行里），
	 * 渲染出来却是三行 —— 比文本的话这次"什么都没改的保存"会把整篇重排，
	 * 而运营明明什么都没改。所以判据落在**内容**上，而磁盘上一个字节都不动。
	 */
	after, err := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, err)
	assert.Equal(t, string(before), string(after))
	assert.Equal(t, string(before), rendered)
	/* ★ 而且没有留下一个 `.bak`（什么都没改，不该有备份） */
	_, statErr := os.Stat(filepath.Join(dir, "mv.json.bak"))
	assert.True(t, os.IsNotExist(statErr), "什么都没改却留下了 .bak")
}

func TestWriteModeContent_RefusesHugeFiles(t *testing.T) {
	/* ⚠ 一次"保存"不许把几百 MB 的 JSON 写进模式目录（那之后每次列目录都要读它） */
	useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})

	content, err := ReadModeContent("mv")
	require.NoError(t, err)
	content.Content["padding"] = strings.Repeat("x", maxModeContentBytes+16)

	_, err = WriteModeContent("mv", content.Content)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "太大了")
}

func TestRoundTrip_ShippedModeSurvivesOneSave(t *testing.T) {
	/*
	 * ★★ 拿仓库里**真的要发出去**的那一份跑一遍"读 → 原样写回"：
	 * 逐字节相同。它钉的是"这个功能不会顺手把线上那几份模式改样"。
	 */
	row, found := ModeByID("short-drama")
	if !found || row.Problem != "" {
		t.Skip("仓库里没有可用的 short-drama 这一份，跳过")
	}
	dir := useModeDir(t, map[string]string{"short-drama.json": row.Raw})

	content, err := ReadModeContent("short-drama")
	require.NoError(t, err)
	rendered, err := WriteModeContent("short-drama", content.Content)
	require.NoError(t, err)

	onDisk, err := os.ReadFile(filepath.Join(dir, "short-drama.json"))
	require.NoError(t, err)
	assert.Equal(t, string(onDisk), rendered)
}
