package mode

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// ★★ "后台能改一档的公开 / 私有"（`docs/28` §3b，用户 2026-…）
//
// 用户的原话：
//
//	"new-api 的后台，模式管理 应该是可以编辑的，可以设置 权限是公开还是私有"
//
// 这一组守的是**那一下写盘**，而它最容易坏在两处：
//
//	A. ★★ **改完之后那份文件被整篇重排**（读进 map 再写回去 = 键序按字母排、
//	   `<` 被转义）—— 那会把这个方案的立足点弄丢："服务端那一份 = 客户端那一份、
//	   一眼可验"（`catalog.go` 文件头）。所以判据是**逐字节**：只有那两行变。
//	B. **改坏的东西**（坏文件 / 不存在的 id / 乱写的可见性）被照单收下 ——
//	   那会让一档付费模式悄悄变成公开可开通。
//
// 外加两条"运营那一侧"的：`.bak` 留最早那一版、改完**立刻生效**。
// =========================================================================

/**
 * ★★ 本组用例的夹具 = **仓库里真的那一份** `modes/mv.json`。
 *
 * ⚠★ 为什么不用手写的 `aValidMode`：那个夹具把嵌套值写成了一行
 * （`"words": { "text": "歌词" }`），而写回时每一步都过 `json.Indent`
 * （把嵌套展开成 2 空格缩进）—— 于是"逐字节只差那两行"这条断言会因为
 * **夹具本身**而不成立，看起来却像"写回把整份文件重排了"（踩过一次）。
 * 真文件是 `JSON.stringify(…, null, 2)` 写出来的，本来就是规范缩进
 * （`catalog_test.go` 的 `TestShippedModes_…` 用的是同一条理由）。
 */
func seedRealMode(t *testing.T) (dir string, raw string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "modes", "mv.json"))
	require.NoError(t, err, "仓库里那份 modes/mv.json 读不到 —— 本组用例的夹具就是它")
	dir = t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mv.json"), data, 0o600))
	return dir, string(data)
}

/** 把一份文本里某一格那一行取出来（**当夹具的期望值用**，避免在用例里抄死字符串） */
func lineOf(t *testing.T, text, key string) string {
	t.Helper()
	for _, l := range strings.Split(text, "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), `"`+key+`"`) {
			return l
		}
	}
	t.Fatalf("这一份里没有 %q 那一行", key)
	return ""
}

// currentVisibilityOf reads the `x-visibility` value out of a raw mode file.
//
// ⚠★ 这一组用例的夹具是**仓库里那一份真的** `modes/mv.json`，而它同时是
// **开发机上那个服务在读的那一份**（`ZSY_MODE_DIR` 缺省 = `<工作目录>/modes`）——
// 运营会在后台改它的可见性（`docs/28` §4.11 ②，旁边那些 `.bak` 就是那一次改动的痕迹）。
//
// 所以"**它现在是 public 还是 private**"这件事**不许写死在用例里**：
// 写死的后果是运营改一次之后，几条"逐字节只差那两行 / 第一次改要留 .bak"
// 的用例会**变成什么都没改**（`got == original`、没有 `.bak`），而它们真正要守的
// 东西（写回不重排、`.bak` 留最早那一版）在那时**一条都没被验到**。
func currentVisibilityOf(t *testing.T, raw string) string {
	t.Helper()
	line := lineOf(t, raw, visibilityKey)
	value := strings.Trim(strings.TrimSpace(strings.SplitN(line, ":", 2)[1]), `",`)
	require.Contains(t, []string{VisibilityPublic, VisibilityPrivate}, value,
		"仓库里那份模式文件的 x-visibility 是 %q（不是这两个值之一）", value)
	return value
}

// flipVisibility answers **the other one** —— 用例要的是"改一次"，
// 不是"改成 private"（见 `currentVisibilityOf` 那段说明）。
func flipVisibility(v string) string {
	if v == VisibilityPublic {
		return VisibilityPrivate
	}
	return VisibilityPublic
}

/** 铺一份模式库并改其中一档的元信息 */
func writeMeta(t *testing.T, dir, id string, in ModeMetaInput) (string, error) {
	t.Helper()
	useModeDirFrom(t, dir)
	return WriteModeMeta(id, in)
}

// aValidModeWithNewline 是给"**读盘校验**"那一类用例用的最小夹具
// （`catalog_test.go` 的 `aValidMode` 加一个换行 —— 真文件末尾都有）。
func aValidModeWithNewline(id, medium string) string {
	return aValidMode(id, medium) + "\n"
}

/** 把模式库指向一个**已经存在**的目录（用例自己先铺好文件） */
func useModeDirFrom(t *testing.T, dir string) {
	t.Helper()
	t.Setenv(envModeDir, dir)
	t.Cleanup(func() { LoadModes() })
}

func TestWriteModeMeta_ChangesOnlyThoseTwoLines(t *testing.T) {
	/*
	 * ★★ 判据对着**仓库里真的那一份**（`modes/mv.json`）跑，而不是手写夹具。
	 *
	 * ⚠★ 理由是踩出来的：手写夹具里的嵌套值写成了一行（`"words": { "text": "歌词" }`），
	 * 而写回时每一步都过 `json.Indent`（把嵌套展开成 2 空格缩进）——
	 * 于是"逐字节只差那两行"这条断言会因为**夹具本身**而不成立，
	 * 看起来却像"写回把整份文件重排了"。
	 * 真文件是 `JSON.stringify(…, null, 2)` 写出来的，本来就是规范缩进
	 * （`catalog_test.go` 的 `TestShippedModes_…` 用的是同一条理由）。
	 */
	src := filepath.Join("..", "..", "modes", "mv.json")
	original, err := os.ReadFile(src)
	require.NoError(t, err, "仓库里那份 mv.json 读不到 —— 这条用例的夹具就是它")
	/*
	 * ★★ 目标可见性 = **与夹具现在不同的那一个**（不是写死的 `private`）——
	 * 理由见 `currentVisibilityOf` 那段：仓库里那一份的可见性是运营随时会改的活数据，
	 * 写死一个值会让这条"逐字节只差那两行"的用例在"本来就是那个值"时**变成什么都没改**。
	 */
	target := flipVisibility(currentVisibilityOf(t, string(original)))

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mv.json"), original, 0o600))
	got, err := writeMeta(t, dir, "mv", ModeMetaInput{
		Visibility: target,
		Summary:    strPtr("一句话说明（改过）"),
	})
	require.NoError(t, err)
	require.NotEqual(t, string(original), got, "什么都没改（可见性与原来一样、说明也一样？）")

	/* ★★ 逐字节：只有那两行变了 */
	beforeLines := strings.Split(string(original), "\n")
	afterLines := strings.Split(got, "\n")
	require.Equal(t, len(beforeLines), len(afterLines),
		"行数变了 —— 说明不是'原地改那两行'（换行 / 缩进被重排了）")

	diffs := []int{}
	for i := range beforeLines {
		if beforeLines[i] != afterLines[i] {
			diffs = append(diffs, i)
		}
	}
	require.Len(t, diffs, 2, "改的**不止两行**（键序 / 缩进 / 转义被改过）：%v", diffs)
	require.Contains(t, beforeLines[diffs[0]], visibilityKey)
	require.Contains(t, beforeLines[diffs[1]], summaryKey)

	/* ⚠ 而且磁盘上那份就是回给界面的那一份 */
	onDisk, err := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, err)
	require.Equal(t, got, string(onDisk))
}

func TestWriteModeMeta_InsertsTheKeysAfterFormatVersion(t *testing.T) {
	dir := t.TempDir()
	/* 一份**没有**那两格的模式（= 老文件，默认私有） */
	bare := `{
  "format": "aimv-work-mode",
  "formatVersion": 1,
  "id": "bare",
  "label": "老模式",
  "medium": "video"
}
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "bare.json"), []byte(bare), 0o600))

	got, err := writeMeta(t, dir, "bare", ModeMetaInput{
		Visibility: VisibilityPublic,
		Summary:    strPtr("新加的说明"),
	})
	require.NoError(t, err)
	/*
	 * ⚠ 位置是 `formatVersion` 之后："这个文件是什么"那两行 → "它怎么发"。
	 * 断言按**行的先后**判（不是"包含"）：插到文件尾也算"包含了"，而那是错的。
	 */
	lines := strings.Split(got, "\n")
	at := func(needle string) int {
		for i, l := range lines {
			if strings.Contains(l, needle) {
				return i
			}
		}
		return -1
	}
	require.Less(t, at(`"formatVersion"`), at(`"x-visibility"`), "可见性插到了 formatVersion 之前")
	require.Less(t, at(`"x-visibility"`), at(`"x-summary"`), "说明没有跟在可见性后面")
	require.Less(t, at(`"x-summary"`), at(`"id"`), "两格被插到了正文那几格中间")

	/* ⚠ 而且一份老文件改完仍然是**可用的**模式（读得回来、那一格生效了） */
	row, found := ModeByID("bare")
	require.True(t, found)
	require.Equal(t, VisibilityPublic, row.Visibility)
	require.Equal(t, "新加的说明", row.Summary)
	require.Empty(t, row.Problem)
}

func TestWriteModeMeta_LeavesTheOtherKeyAlone(t *testing.T) {
	dir, original := seedRealMode(t)
	summaryLine := lineOf(t, original, summaryKey)
	target := flipVisibility(currentVisibilityOf(t, original))

	/* ★ 只改可见性（`summary` **不传** = nil）→ 说明书那格**一个字都不许动** */
	got, err := writeMeta(t, dir, "mv", ModeMetaInput{Visibility: target})
	require.NoError(t, err)
	require.Contains(t, got, summaryLine, "只改可见性却把说明动了")
	require.Contains(t, got, `"x-visibility": "`+target+`"`, "可见性没改成 %s", target)

	/* ★ 反过来：只改说明 → 可见性那格不动 */
	got2, err := writeMeta(t, dir, "mv", ModeMetaInput{Summary: strPtr("换一句")})
	require.NoError(t, err)
	require.Contains(t, got2, `"x-visibility": "`+target+`"`, "只改说明却把可见性动了")
	require.Contains(t, got2, `"x-summary": "换一句"`)

	/* ★ 传一个**空串** = 明确要清掉那一格（与"不传"是两件事） */
	got3, err := writeMeta(t, dir, "mv", ModeMetaInput{Summary: strPtr("")})
	require.NoError(t, err)
	require.Contains(t, got3, `"x-summary": ""`)
}

func TestWriteModeMeta_RefusesBadVisibility(t *testing.T) {
	dir, original := seedRealMode(t)

	for _, bad := range []string{"publik", "Public", "公开", "true"} {
		_, err := writeMeta(t, dir, "mv", ModeMetaInput{Visibility: bad})
		require.Error(t, err, "%q 竟然被收下了", bad)
		require.Contains(t, err.Error(), "可见性只能写", "%q 那句话没说清能写什么", bad)
	}
	/* ⚠ 而且**磁盘一个字节都没动**（拒绝了就要看得出来才算数） */
	onDisk, err := os.ReadFile(filepath.Join(dir, "mv.json"))
	require.NoError(t, err)
	require.Equal(t, original, string(onDisk))
}

func TestWriteModeMeta_RefusesUnknownAndBrokenModes(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "broken.json"),
		[]byte(`{ 这不是 JSON`), 0o600))

	_, err := writeMeta(t, dir, "nosuch", ModeMetaInput{Visibility: VisibilityPublic})
	require.Error(t, err)
	require.Contains(t, err.Error(), "nosuch")

	/*
	 * ⚠★ 坏文件**不许改**：改它只会把问题搅得更乱 —— 后台那一屏会把坏文件
	 * 列出来并说明原因（`problem`），运营该做的是去修那个文件。
	 */
	_, err = writeMeta(t, dir, "broken", ModeMetaInput{Visibility: VisibilityPublic})
	require.Error(t, err)
	require.Contains(t, err.Error(), "有问题")
}

func TestWriteModeMeta_KeepsTheEarliestBackup(t *testing.T) {
	dir, original := seedRealMode(t)

	/*
	 * ⚠ 第一次改必须是**真的改**：目标值从夹具自己推（见 `currentVisibilityOf`）——
	 * 写死 `private` 时，若仓库里那份本来就是 private，则"没变化就不写"
	 * 会让这一步**一个 `.bak` 都不留**，而报错说的是"第一次改没有留 .bak"
	 * （指向的是 `.bak` 那一支，与真正的原因隔着两层）。
	 */
	first := flipVisibility(currentVisibilityOf(t, original))
	_, err := writeMeta(t, dir, "mv", ModeMetaInput{Visibility: first})
	require.NoError(t, err)
	firstBackup, err := os.ReadFile(filepath.Join(dir, "mv.json.bak"))
	require.NoError(t, err, "第一次改没有留 .bak")
	require.Equal(t, original, string(firstBackup))

	/* 再改一次（改回原来那个值）：`.bak` 仍然是**最早的**那一版（不是上一次那一版） */
	_, err = writeMeta(t, dir, "mv", ModeMetaInput{Visibility: flipVisibility(first)})
	require.NoError(t, err)
	second, err := os.ReadFile(filepath.Join(dir, "mv.json.bak"))
	require.NoError(t, err)
	require.Equal(t, original, string(second), ".bak 被第二次改覆盖了（那不是'最早那一版'）")
}

func TestWriteModeMeta_NoChangeWritesNothing(t *testing.T) {
	dir, original := seedRealMode(t)
	visibilityLine := lineOf(t, original, visibilityKey)
	/* 把原文里那一格的值取出来（"public" / "private"），再**原样**设一次 */
	value := strings.Trim(strings.TrimSpace(strings.SplitN(visibilityLine, ":", 2)[1]), `",`)

	path := filepath.Join(dir, "mv.json")
	before, err := os.Stat(path)
	require.NoError(t, err)

	got, err := writeMeta(t, dir, "mv", ModeMetaInput{Visibility: value})
	require.NoError(t, err)
	require.Equal(t, original, got, "设成与原来一样的值，文件却变了")

	_, statErr := os.Stat(path + ".bak")
	require.True(t, os.IsNotExist(statErr), "没有真改动却留了一个 .bak")

	after, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, before.ModTime(), after.ModTime(), "没有真改动却把文件重写了一遍")
}

/* ---------------------------------------------------------------------------
 * 那一面 HTTP（真路由链 + 真表）
 * ------------------------------------------------------------------------- */

func TestAdminUpdateMeta_TakesEffectOnThePublicFace(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mv.json"), []byte(aValidModeWithNewline("mv", "video")), 0o600))
	/* ⚠ 一开始把它设成**私有**：于是"公开面列不列它"就是一条能测的判据 */
	_, err := writeMeta(t, dir, "mv", ModeMetaInput{Visibility: VisibilityPrivate})
	require.NoError(t, err)

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)

	/* 私有 → 匿名看不到 */
	_, before := env.getJSON("/api/zsy/mode/list", "")
	require.Equal(t, []string{}, modeIDsOf(t, before), "设成私有之后匿名还看得到")

	/* ★ 后台把它改成公开 */
	_, updated := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/mv/meta",
		`{"visibility":"public"}`)
	require.Equal(t, true, updated["success"], "改可见性失败了：%v", updated)
	data, _ := updated["data"].(map[string]any)
	require.Equal(t, VisibilityPublic, data["visibility"])
	require.Equal(t, 0, int(data["granted"].(float64)))

	/* ★★ 立刻生效（公开面每一次都重新读盘、重新判权限，没有任何缓存） */
	_, after := env.getJSON("/api/zsy/mode/list", "")
	require.Equal(t, []string{"mv"}, modeIDsOf(t, after), "改完公开之后仍然列不出来")
}

func TestAdminUpdateMeta_RefusesAnEmptyRequest(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mv.json"), []byte(aValidModeWithNewline("mv", "video")), 0o600))
	useModeDirFrom(t, dir)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)

	/*
	 * ⚠ 两格都不给 = 一次**什么都没做**的保存：报错比"回 200 而且什么都没变"好 ——
	 * 后者会让运营以为自己改成功了（而磁盘上那个文件一个字没动）。
	 */
	_, payload := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/mv/meta", `{}`)
	require.NotEqual(t, true, payload["success"], "空请求被当成了成功")
	require.Contains(t, messageOf(t, payload), "visibility")
}

func TestAdminUpdateMeta_RefusesBadVisibilityThroughHTTP(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "mv.json"), []byte(aValidModeWithNewline("mv", "video")), 0o600))
	useModeDirFrom(t, dir)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)

	_, payload := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/mv/meta",
		`{"visibility":"publik"}`)
	require.NotEqual(t, true, payload["success"], "乱写的可见性被收下了")
	require.Contains(t, messageOf(t, payload), "可见性只能写")
}

func strPtr(s string) *string { return &s }
