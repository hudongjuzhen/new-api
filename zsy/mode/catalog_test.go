package mode

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// 模式库的守卫（docs/28 §4）
//
// 三层，缺一层这个功能就不成立：
//
//	1. **读盘与校验**：文件名 = id、format 对、label 有、medium 在白名单里 ——
//	   坏的那一份要**列出来并说明**（绝不静默），而不是消失；
//	2. ★★ **发出去的那一份必须干净**：取件回的文本里**不许有 `x-` 那两格**
//	   （客户端的模式格式里没有它们，而"不认识的顶层字段跟着导出"会把它们
//	   原样存进用户本地的模式文件、再原样写回磁盘）；
//	3. ★ **仓库 `modes/` 里那 9 份是好的** —— 这一条盯着**真正要发出去的东西**。
// =========================================================================

/** 一份最小的、合法的模式文件 */
func aValidMode(id, medium string) string {
	return `{
  "format": "aimv-work-mode",
  "formatVersion": 1,
  "x-visibility": "public",
  "x-summary": "一句话说明",
  "id": "` + id + `",
  "label": "测试模式 ` + id + `",
  "medium": "` + medium + `",
  "workScale": "single",
  "words": { "text": "歌词" }
}`
}

/** 把模式库指向一个临时目录（用例自己铺文件） */
func useModeDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600))
	}
	t.Setenv(envModeDir, dir)
	/* ⚠ 用完把缓存清掉：缓存是模块级的，下一个用例会读到这个临时目录 */
	t.Cleanup(func() { LoadModes() })
	return dir
}

/** 只挂那两个公开 handler 的最小路由（不拖进宿主中间件 —— 那些要 DB） */
func publicRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/list", listPublicModes)
	r.GET("/:id/file", getModeFile)
	return r
}

func getJSON(t *testing.T, r *gin.Engine, path string) (int, map[string]any) {
	t.Helper()
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	payload := map[string]any{}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), "回包不是 JSON：%s", rec.Body.String())
	return rec.Code, payload
}

// ---------------------------------------------------------------------------
// 一、读盘与校验
// ---------------------------------------------------------------------------

func TestLoadModes_ReportsProblemsPerFile(t *testing.T) {
	useModeDir(t, map[string]string{
		"mv.json": aValidMode("mv", "video"),
		/* 文件名说 a，里面的 id 说 b —— 这是最容易犯的那种错 */
		"mismatch.json": `{"format":"aimv-work-mode","formatVersion":1,"id":"other",` +
			`"label":"x","medium":"video"}`,
		/* 不是模式文件 */
		"notmode.json": `{"format":"something-else","formatVersion":1,"id":"notmode",` +
			`"label":"x","medium":"video"}`,
		/* 没有 label */
		"nolabel.json": `{"format":"aimv-work-mode","formatVersion":1,"id":"nolabel","medium":"video"}`,
		/* medium 不认识 */
		"badmedium.json": `{"format":"aimv-work-mode","formatVersion":1,"id":"badmedium",` +
			`"label":"x","medium":"vidio"}`,
		/* x-visibility 写错了 */
		"badvis.json": `{"format":"aimv-work-mode","formatVersion":1,"x-visibility":"publik",` +
			`"id":"badvis","label":"x","medium":"video"}`,
		/* 不是 JSON */
		"broken.json": `{ 这不是 JSON`,
	})

	rows := ReloadModes()
	byID := map[string]*ModeFile{}
	for _, r := range rows {
		byID[r.ID] = r
	}

	require.Contains(t, byID, "mv", "好模式没被读进来")
	require.Empty(t, byID["mv"].Problem)
	require.Equal(t, VisibilityPublic, byID["mv"].Visibility)
	require.Equal(t, "测试模式 mv", byID["mv"].Label)

	/*
	 * ★ 下面六条是**同一件事**：坏文件要**出现在列表里**并把原因说清楚。
	 * 不出现的话，运营看到的是"我放进去的文件不见了" —— 那看起来像后台坏了。
	 */
	for _, tc := range []struct {
		id      string
		want    string
		because string
	}{
		{"mismatch", "不一致", "文件名与 id 不一致要说清两个值"},
		{"notmode", "format", "不是模式文件要点名 format"},
		{"nolabel", "label", "少了 label 要点名它"},
		{"badmedium", "medium", "medium 写错要指出是哪一格"},
		{"badvis", visibilityKey, "可见性写错要点名那一格"},
		{"broken", "JSON", "不是 JSON 要这么说"},
	} {
		row := byID[tc.id]
		require.NotNil(t, row, "%s 应当被列出来（绝不静默）", tc.id)
		require.NotEmpty(t, row.Problem, "%s 应当有 problem", tc.id)
		require.Contains(t, row.Problem, tc.want, tc.because)
	}
}

func TestVisibility_DefaultsToPrivate(t *testing.T) {
	useModeDir(t, map[string]string{
		/* 没写 x-visibility */
		"nofield.json": `{"format":"aimv-work-mode","formatVersion":1,"id":"nofield",` +
			`"label":"x","medium":"video"}`,
	})
	rows := ReloadModes()
	require.Len(t, rows, 1)
	/*
	 * ⚠★ 默认必须是 private：`public` 意味着"任何账号点一下就开通了"，
	 * 而一份忘写这一格的模式悄悄变成人人可用**没有任何地方会报错**。
	 * 反过来（忘写 = 谁都看不到）的症状一眼就能看见。
	 */
	require.Equal(t, VisibilityPrivate, rows[0].Visibility)
	require.False(t, rows[0].IsPublic())
	require.Empty(t, rows[0].Problem, "没写这一格是合法的（默认 private），不是错误")
}

func TestModeByID_RefusesPathTraversal(t *testing.T) {
	useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})

	ok, found := ModeByID("mv")
	require.True(t, found)
	require.Equal(t, "mv", ok.ID)

	/*
	 * ⚠★ id 来自 URL，而路径不许由用户数据拼出来（与 media.rs 那条边界纪律同源）。
	 * 这几个都必须在**碰磁盘之前**就被形状校验挡下。
	 */
	for _, bad := range []string{"../mv", "..%2Fmv", "a/b", "MV", "-x", "x y", "", ".hidden"} {
		_, found := ModeByID(bad)
		require.False(t, found, "%q 不该被当成一个模式 id", bad)
	}
}

// ---------------------------------------------------------------------------
// 二、发出去的那一份必须干净
// ---------------------------------------------------------------------------

func TestRenderModeFile_StripsHostKeysButKeepsEverythingElse(t *testing.T) {
	useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})
	row, found := ModeByID("mv")
	require.True(t, found)

	text, err := RenderModeFile(row)
	require.NoError(t, err)

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &out))

	/* ★★ 那两格 `x-` 一个都不许跟到用户机器上 */
	require.NotContains(t, out, visibilityKey, "x-visibility 跟到了用户机器上")
	require.NotContains(t, out, summaryKey, "x-summary 跟到了用户机器上")

	/* ⚠ 其余字段**一个都不许丢**（含运营手写进来的任何不认识的格子） */
	for _, k := range []string{"format", "formatVersion", "id", "label", "medium", "workScale", "words"} {
		require.Contains(t, out, k, "%s 丢了", k)
	}
	require.Equal(t, "mv", out["id"])
	require.Equal(t, float64(1), out["formatVersion"])

	/*
	 * ★ 键序也**不许被重排**（第一版用 `json.MarshalIndent` 走了 map，
	 * 于是 Go 按字母序重排 —— `modes/mv.json` 的第一行本该是 `format`，
	 * 发出去却成了 `appRows`）。这里只钉住"文件头那几格仍然在最前面"。
	 *
	 * ⚠ 逐字节那一条**不在这个夹具上做**：`aValidMode` 是手写的，`words` 那一格
	 * 写成了单行 —— 而真正发出去的那几份都是 `JSON.stringify(…, null, 2)` 写出来的，
	 * 逐字节那条断言放在 `TestShippedModes_…` 里对着**真文件**跑。
	 */
	order := []string{`"format"`, `"formatVersion"`, `"id"`, `"label"`, `"medium"`}
	for i := 1; i < len(order); i++ {
		require.Less(t, strings.Index(text, order[i-1]), strings.Index(text, order[i]),
			"顶层键的次序被打乱了（%s 应当排在 %s 前面）", order[i-1], order[i])
	}
}

// ---------------------------------------------------------------------------
// 三、两个面（公开列 / 取件）
// ---------------------------------------------------------------------------

func TestPublicFace_ListsPublicOnlyAndNeverBroken(t *testing.T) {
	useModeDir(t, map[string]string{
		"mv.json": aValidMode("mv", "video"),
		"audiobook.json": `{"format":"aimv-work-mode","formatVersion":1,"x-visibility":"private",` +
			`"id":"audiobook","label":"有声书","medium":"audio"}`,
		/* ★ 写着 public 但**有毛病** —— 它不该出现在目录里 */
		"broken-pub.json": `{"format":"aimv-work-mode","formatVersion":1,"x-visibility":"public",` +
			`"id":"broken-pub","label":"x","medium":"vidio"}`,
	})

	r := publicRouter()
	code, payload := getJSON(t, r, "/list")
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, true, payload["success"])

	items, ok := payload["data"].(map[string]any)["items"].([]any)
	require.True(t, ok, "items 不是一个数组：%v", payload)
	require.Len(t, items, 1, "只该有 mv 那一条（private 的不列、坏的不列）")
	first, _ := items[0].(map[string]any)
	require.Equal(t, "mv", first["id"])
	require.Equal(t, "video", first["medium"])
	/* ⚠ 公开面**不给 visibility**：那是服务端的分发策略，不该诱使客户端自己算一遍 */
	require.NotContains(t, first, "visibility")
	require.NotEmpty(t, payload["data"].(map[string]any)["directory"])
}

func TestPublicFace_ServesTheFileWithoutHostKeys(t *testing.T) {
	useModeDir(t, map[string]string{"mv.json": aValidMode("mv", "video")})

	r := publicRouter()
	code, payload := getJSON(t, r, "/mv/file")
	require.Equal(t, http.StatusOK, code)

	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "data 丢了：%v", payload)
	text, ok := data["file"].(string)
	require.True(t, ok, "file 应当是**文本**（客户端要写盘的正是这一串，与签发插件那一条同源）")
	require.NotContains(t, text, visibilityKey)
	require.NotContains(t, text, summaryKey)
	require.Equal(t, "mv.json", data["fileName"])
	require.Equal(t, "video", data["medium"])

	/* 私有/不存在的那一档取不到 */
	_, missing := getJSON(t, r, "/nope/file")
	require.NotEqual(t, true, missing["success"], "不存在的模式不该取得到")
}

// ---------------------------------------------------------------------------
// 四、路由表与「仓库里那 9 份」
// ---------------------------------------------------------------------------

func TestMountRoutes_PublishesTheDocumentedURLs(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NotPanics(t, func() { mountRoutes(router) })

	registered := make([]string, 0, 8)
	for _, route := range router.Routes() {
		registered = append(registered, route.Method+" "+route.Path)
	}
	/*
	 * ★ 这张表是**接线守卫**：少一条 = 那一件事在真机上 404，而单测全绿
	 * （每一条路由内部的处置都另有用例钉着，但"它有没有挂上去"只有这里看得见）。
	 *
	 * ⚠ 第 3b 段（私有模式）加了后面三条 —— 它们就是"后台给权限"那一下，
	 * 而 `docs/28` §6 里那一段的全部内容就是这三条路 + 一张表。
	 * ⚠ `/:id/meta` 是 2026-… 加的："后台能改一档的公开 / 私有"（用户点名要的）。
	 */
	require.ElementsMatch(t, []string{
		"GET /api/zsy/mode/list",
		"GET /api/zsy/mode/:id/file",
		"GET /dashboard/zsy/mode/list",
		"POST /dashboard/zsy/mode/:id/meta",
		"GET /dashboard/zsy/mode/entitlements",
		"POST /dashboard/zsy/mode/entitlements/grant",
		"POST /dashboard/zsy/mode/entitlements/revoke",
	}, registered)
}

func TestShippedModes_AreAllGood(t *testing.T) {
	/*
	 * ★★ 这一条盯着**真正要发出去的那 10 份**（`new-api/modes/`）。
	 *
	 * 它是服务端这一摊最重要的一条守卫：前面那些用例都跑在临时目录里，
	 * 只证明"这个函数是对的"；而这一条证明"**我们手上这 10 份是好的**"
	 * —— 一份坏的在目录里时的表现是"广场上少了一档，而用户只会以为
	 * 这一版就是没有它"。
	 *
	 * ⚠ 第 10 档（`short-drama` 短剧）是 2026-… 加的：用户在客户端侧点名
	 * "根据视频工程模式增加一个短剧模式，也在 new-api 这里进行管理，
	 * **默认是公开版**" —— 所以它一进来就在下面那张公开名单里。
	 */
	t.Setenv(envModeDir, filepath.Join("..", "..", "modes"))
	t.Cleanup(func() { LoadModes() })

	rows := ReloadModes()
	require.Len(t, rows, 10, "仓库 modes/ 里应当有 10 份（视频 4 / 音频 2 / 文本 4）")

	want := map[string]string{
		"mv": "video", "science": "video", "ad-idea": "video", "short-drama": "video",
		"audiobook": "audio", "podcast": "audio",
		"article": "text", "novel": "text", "script-text": "text", "science-text": "text",
	}
	byID := map[string]*ModeFile{}
	for _, r := range rows {
		byID[r.ID] = r
		require.Empty(t, r.Problem, "仓库里这一份有问题：%s", r.Problem)
		require.Equal(t, want[r.ID], r.Medium, "%s 的 medium 不对", r.ID)
		require.NotEmpty(t, r.Label, "%s 没有 label", r.ID)
	}

	/*
	 * ★★ 可见性：**公开的那几档列在 `publicIDs` 里，其余一律必须是 private**。
	 *
	 * # ⚠★ 这一张名单是"发布默认值的快照"，不是"永远成立的产品决定"
	 *
	 * 可见性是**运营在后台**（`/mode-plaza` 那一屏，`POST …/:id/meta`）按商业节奏
	 * 改的 —— 而仓库里这一份 `modes/` **就是**开发机上那个服务读的那一份
	 * （`ZSY_MODE_DIR` 缺省 = `<工作目录>/modes`），所以运营在后台点一下，
	 * 这几个文件当场就变了（旁边那些 `.bak` 就是那些改动的痕迹）。
	 *
	 * 于是这条守卫的**方向**要说清：它挡的是"一份文件忘了写 `x-visibility`"
	 * （缺省 private，悄悄变成谁都看不到，而没有任何地方会报错）与"写了个乱值"。
	 * 真要把某一档开给所有人（或收回来）时，改的是**后台**，然后**回来把
	 * `publicIDs` 对齐** —— 那一步红一下，比"改了后台而没人知道发出去的是什么"好。
	 *
	 * ⚠ 名单是**少数派**（只有它列出来的那些是公开的），所以"多列一个"与
	 * "少列一个"两个方向都拦得住：两个方向都会让某一份的断言失败。
	 */
	publicIDs := []string{
		/* ★ 用户 2026-… 点名：新加的这一档默认公开版 */
		"short-drama",
	}
	isPublic := map[string]bool{}
	for _, id := range publicIDs {
		isPublic[id] = true
	}
	for id := range want {
		row := byID[id]
		require.NotNil(t, row, "少了 %s", id)
		if isPublic[id] {
			require.Equal(t, VisibilityPublic, row.Visibility, "%s 应当在公开名单里却写着 %s", id, row.Visibility)
			continue
		}
		require.Equal(t, VisibilityPrivate, row.Visibility,
			"%s 不在公开名单里却写着 %s（要在后台公开它的话，把 `publicIDs` 一起改）", id, row.Visibility)
	}
	/* ⚠ 公开名单里不许有仓库里不存在的 id（写错了名字时这一条会红） */
	for _, id := range publicIDs {
		require.NotNil(t, byID[id], "公开名单里的 %s 在仓库 modes/ 里不存在", id)
	}
}

// dropHostKeyLines removes the two host-only lines from a raw mode file.
//
// ⚠ 它只在**真文件**上有意义：仓库里那 9 份是 `JSON.stringify(…, null, 2)` 写出来的，
// 每一格恰好一行；而 `x-` 那两格的值都是短字符串，也各占一行。
func dropHostKeyLines(raw string) string {
	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, `"`+visibilityKey+`"`) ||
			strings.HasPrefix(trimmed, `"`+summaryKey+`"`) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n") + "\n"
}

func TestShippedModes_ServedTextIsTheFileMinusHostKeys(t *testing.T) {
	/*
	 * ★★ 这一条盯的是这个方案的**立足点**：选"存量放文件、不放数据库"的全部理由，
	 * 就是"服务端那一份 = 客户端本地那一份，一眼可验"（`catalog.go` 文件头）。
	 *
	 * 所以判据是**逐字节**的：取件回来的文本应当与磁盘那份原文**只差那两行**。
	 * ⚠ 第一版做不到（`json.MarshalIndent` 走 map → 键序被按字母重排），
	 * 而那种偏差正是一个"没人能解释的差异"：用户开通之后会发现本地这份的
	 * 第一行不是 `format`。
	 */
	t.Setenv(envModeDir, filepath.Join("..", "..", "modes"))
	t.Cleanup(func() { LoadModes() })

	rows := ReloadModes()
	require.NotEmpty(t, rows)
	for _, row := range rows {
		served, err := RenderModeFile(row)
		require.NoError(t, err, "%s 渲染失败", row.ID)
		require.Equal(t, dropHostKeyLines(row.Raw), served,
			"%s：取件回来的文本与磁盘那份不只差那两行（键序或缩进被动过）", row.ID)
	}
}
