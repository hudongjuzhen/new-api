package mode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/extcore"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// =========================================================================
// 私有模式的守卫（`docs/28` §3b）
//
// 用户的定性（逐字）：
//
//	"公共免费的一键开通，私有不显示。当后台给权限之后才显示，并且能够一键开通"
//
// 三句话拆成四条守卫，而**每一条都对应一种具体的坏法**：
//
//	A. 匿名看不到私有那几档           —— 坏了 = 私有模式变成了公开
//	B. 有权限的账号看得到、也取得到     —— 坏了 = "给了权限还是用不上"
//	C. ★★ **没权限时取件也取不到**     —— 坏了 = 知道 id 的人直接打 URL 就拿到了
//	D. ★★ **密钥无效要报错，不许降级成匿名** —— 坏了 = 买过的人以为自己的东西丢了
//
// 外加几条"运营那一侧"的：撤销立刻生效、过期不算、给不存在的模式授权要拒绝。
//
// ⚠★ 这一组走的是**真路由链**（`extcore.MountRoutes`）+ **真表**
// （`extcore.ExtraMigrateModels()`，即生产上 AutoMigrate 扫的那一份）：
// 私有模式这一半的全部内容就是"哪一条路认谁"，绕开路由去测函数等于没测。
// 这也顺手钉住了一件很容易忘的事 —— **表注册了没有**（忘了注册的表现是
// 生产上第一次授权时"没有这个表"，而单测里直接 AutoMigrate 的话永远不红）。
//
// ⚠ 请求体一律**下划线**（`user_id` / `mode_id` / `expires_at`）：
// 那是宿主 dashboard 那一套的形状，见 `controllers_admin.go` 的 `adminGrantParams`。
// =========================================================================

// ---------------------------------------------------------------------------
// 夹具
// ---------------------------------------------------------------------------

type testEnv struct {
	t       *testing.T
	db      *gorm.DB
	router  *gin.Engine
	keyOf   map[string]string
	adminOf map[string]string
	idOf    map[string]int
}

// newTestEnv builds an in-memory database with the host tables this plugin drives
// plus its own one, and a gin engine with the plugin's real route chain mounted
// through the extcore registry — i.e. the exact path router.SetRouter uses.
//
// ⚠ 它与 `zsy/world` 的同名夹具逐字同形（含 `LOG_SQL_DSN` 那一格：
// `middleware.AdminAuth()` 会给每一次后台写记一条审计日志，而 LOG_DB 为 nil
// 时那条 goroutine 会 panic —— 于是"后台那一面根本测不了"）。
//
// Process-wide globals are snapshotted and restored. It deliberately avoids
// t.Parallel: model.DB is a process-wide global.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	origDB := model.DB
	origRedis, origMem := common.RedisEnabled, common.MemoryCacheEnabled

	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	t.Cleanup(func() {
		model.DB = origDB
		common.RedisEnabled, common.MemoryCacheEnabled = origRedis, origMem
	})

	dsn := fmt.Sprintf("file:zsy_mode_%s?mode=memory&cache=shared",
		strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// One connection keeps the shared in-memory database alive for the whole
	// test even when GORM drops an idle connection.
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	t.Cleanup(func() { _ = sqlDB.Close() })

	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())

	/*
	 * ★★ 这一行是**生产上真正跑的那一份注册表**（`extcore.ExtraMigrateModels()`），
	 * 不是这里手写的一张清单 —— 于是"忘了 `RegisterMigrateModels`"会**在这里红**，
	 * 而不是在生产上第一次授权时变成"没有 zsy_mode_entitlements 这个表"。
	 */
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))
	require.NoError(t, db.AutoMigrate(extcore.ExtraMigrateModels()...))

	gin.SetMode(gin.TestMode)
	router := gin.New()
	extcore.MountRoutes(router)

	return &testEnv{
		t:       t,
		db:      db,
		router:  router,
		keyOf:   map[string]string{},
		adminOf: map[string]string{},
		idOf:    map[string]int{},
	}
}

// seedAccount creates an account with one unlimited account key and returns the
// user id (see the `zsy/world` twin for why it writes through the database).
func (e *testEnv) seedAccount(username string, role int) int {
	e.t.Helper()

	user := &model.User{
		Username:    username,
		Password:    "$2a$10$0123456789012345678901uZ0eGmvJ1kRw4Q1W4Wv8Q6dGmBdGFmC",
		DisplayName: username,
		Role:        role,
		Status:      common.UserStatusEnabled,
		AffCode:     common.GetRandomString(8),
	}
	require.NoError(e.t, e.db.Create(user).Error, "seed user %s", username)

	key, err := common.GenerateKey()
	require.NoError(e.t, err)
	token := &model.Token{
		UserId:         user.Id,
		Name:           "默认密钥",
		Key:            key,
		Status:         common.TokenStatusEnabled,
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}
	require.NoError(e.t, e.db.Create(token).Error, "seed token for %s", username)

	e.keyOf[username] = key
	e.idOf[username] = user.Id
	return user.Id
}

// adminUsername is the account the harness wires up for the admin face.
const adminUsername = "admin"

// ensureAdminAccessToken lazily gives the harness's admin account the dashboard
// access token `middleware.AdminAuth()` accepts, and returns it.
//
// ⚠ 两个面真的用的是两种凭据：公开面认 relay 那把账号密钥，后台面认后台访问令牌
// —— 与 new-api 其余部分一致。用例**故意**两边都用真凭据："后台给权限"那一下
// 正是这一段的验收点，用一个绕过 AdminAuth 的钩子测的是另一扇门。
func (e *testEnv) ensureAdminAccessToken() string {
	e.t.Helper()

	if token, ok := e.adminOf[adminUsername]; ok {
		return token
	}
	adminID, ok := e.idOf[adminUsername]
	require.True(e.t, ok, "先调 seedAccount(%q, common.RoleAdminUser)", adminUsername)

	accessToken := common.GetRandomString(32)
	require.NoError(e.t,
		e.db.Model(&model.User{}).Where("id = ?", adminID).
			Update("access_token", accessToken).Error)
	e.adminOf[adminUsername] = accessToken
	return accessToken
}

// getJSON issues a GET with an optional account key.
func (e *testEnv) getJSON(path, key string) (*httptest.ResponseRecorder, map[string]any) {
	e.t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, decodeEnvelope(e.t, rec.Body.Bytes())
}

// callAdmin issues an admin-face request with the dashboard access token.
func (e *testEnv) callAdmin(method, path, body string) (*httptest.ResponseRecorder, map[string]any) {
	e.t.Helper()
	token := e.ensureAdminAccessToken()

	var reader *bytes.Reader
	if body == "" {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader([]byte(body))
	}
	req := httptest.NewRequest(method, path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", token)
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, decodeEnvelope(e.t, rec.Body.Bytes())
}

func decodeEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()
	payload := map[string]any{}
	require.NoError(t, json.Unmarshal(body, &payload), "回包不是 JSON：%s", body)
	return payload
}

// modeIDsOf reads `data.items[].id` out of a catalog response.
func modeIDsOf(t *testing.T, payload map[string]any) []string {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "回包里没有 data：%v", payload)
	items, ok := data["items"].([]any)
	require.True(t, ok, "items 不是一个数组：%v", payload)
	out := make([]string, 0, len(items))
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		out = append(out, fmt.Sprint(row["id"]))
	}
	return out
}

// messageOf reads the operator-facing message of an envelope.
func messageOf(t *testing.T, payload map[string]any) string {
	t.Helper()
	return fmt.Sprint(payload["message"])
}

// privateLibrary is the fixture every test in this file starts from: one public
// mode and two private ones.
func privateLibrary(t *testing.T) {
	t.Helper()
	useModeDir(t, map[string]string{
		"mv.json": aValidMode("mv", "video"),
		"audiobook.json": `{"format":"aimv-work-mode","formatVersion":1,"x-visibility":"private",` +
			`"id":"audiobook","label":"有声书","medium":"audio"}`,
		"podcast.json": `{"format":"aimv-work-mode","formatVersion":1,"x-visibility":"private",` +
			`"id":"podcast","label":"播客","medium":"audio"}`,
	})
}

// ---------------------------------------------------------------------------
// A / B：看不看得见
// ---------------------------------------------------------------------------

func TestPrivateModes_AnonymousSeesOnlyPublicOnes(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)

	_, payload := env.getJSON("/api/zsy/mode/list", "")
	require.Equal(t, true, payload["success"])
	/*
	 * ⚠★ 判据是"只有 mv"：私有那两档**根本不在列表里**（不是画成灰的）。
	 * 用户的原话是"私有不显示"，而"显示但点不动"是另一件事。
	 */
	require.Equal(t, []string{"mv"}, modeIDsOf(t, payload))
}

func TestPrivateModes_AppearOnlyAfterTheAdminGrants(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)
	key := env.keyOf["buyer"]

	/* 还没有权限：与匿名看到的**一模一样** */
	_, before := env.getJSON("/api/zsy/mode/list", key)
	require.Equal(t, []string{"mv"}, modeIDsOf(t, before), "还没授权就看得到私有模式")

	/* ★ 后台给权限 */
	_, granted := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))
	require.Equal(t, true, granted["success"], "授权失败了：%v", granted)

	/* ★★ 下一次请求立刻生效（判据现算，没有任何缓存） */
	_, after := env.getJSON("/api/zsy/mode/list", key)
	/*
	 * ⚠ 用 `ElementsMatch` 而不是逐个比：这一面的次序是 `LoadModes` 的
	 * **文件名字典序**（`audiobook` 排在 `mv` 前面），而那是"目录里读出来什么次序"
	 * 这件事的实现细节 —— 客户端按 `medium` 重新分堆，不依赖它。
	 * 钉死次序会让这一条在运营往目录里加一个文件时无端变红。
	 */
	require.ElementsMatch(t, []string{"mv", "audiobook"}, modeIDsOf(t, after),
		"授权之后私有模式仍然没出现")
}

func TestPrivateModes_InvalidKeyIsAnErrorNotASilentDowngrade(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)

	/*
	 * ⚠★ 这一条是这一层**全部的价值**所在（`auth.go` 文件头）。
	 *
	 * 降级成匿名的表现是：一个买过 `audiobook` 的账号在密钥失效之后打开广场，
	 * 看到的**与从没买过的人一模一样** —— 少了两档，而屏幕上**一个字都没说**。
	 * 用户会以为自己的东西丢了。
	 */
	rec, payload := env.getJSON("/api/zsy/mode/list", "sk-这不是一把有效的密钥")
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotEqual(t, true, payload["success"], "无效密钥被当成了匿名（静默少了两档）")
	require.Contains(t, messageOf(t, payload), "重新登录",
		"那句话要能照做（告诉用户下一步做什么）")
}

func TestPrivateModes_DisabledAccountLosesThem(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)
	key := env.keyOf["buyer"]

	_, granted := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))
	require.Equal(t, true, granted["success"])

	/* 停用这个账号 —— 被停用的账号不该还能看见他买过的私有模式清单 */
	require.NoError(t, env.db.Model(&model.User{}).Where("id = ?", userID).
		Update("status", common.UserStatusDisabled).Error)

	_, payload := env.getJSON("/api/zsy/mode/list", key)
	require.NotEqual(t, true, payload["success"], "被停用的账号仍然拿得到按账号算的目录")
}

// ---------------------------------------------------------------------------
// C：★★ 取件那一条也要自己判
// ---------------------------------------------------------------------------

func TestPrivateModes_FileIsRefusedWithoutAGrant(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)

	/*
	 * ⚠★ 这条 URL 是**手打**得出来的（id 就在文件名里），所以
	 * "列表里没列它"一个字都挡不住。"列得出来、却取不到"（或反过来）
	 * 是这一段最容易出的自相矛盾，两处因此走同一条 WHERE。
	 */
	_, payload := env.getJSON("/api/zsy/mode/audiobook/file", "")
	require.NotEqual(t, true, payload["success"], "没权限却取到了私有模式的文件")
	require.Contains(t, messageOf(t, payload), "私有模式")
	require.Contains(t, messageOf(t, payload), "后台",
		"那句话要能照做（告诉用户去哪儿开通权限），也要带上这一档的名字")

	/* ★ 而公共那一档**匿名也取得到**（"公共免费的一键开通"） */
	_, public := env.getJSON("/api/zsy/mode/mv/file", "")
	require.Equal(t, true, public["success"], "公共模式匿名取不到：%v", public)
}

func TestPrivateModes_FileIsServedOnceGranted(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)
	key := env.keyOf["buyer"]

	_, granted := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))
	require.Equal(t, true, granted["success"])

	_, payload := env.getJSON("/api/zsy/mode/audiobook/file", key)
	require.Equal(t, true, payload["success"], "授权之后仍然取不到：%v", payload)
	data, _ := payload["data"].(map[string]any)
	text, _ := data["file"].(string)
	require.Contains(t, text, `"id": "audiobook"`)
	/* ⚠ 取件那一份仍然**不许带那两格 `x-`**（与公共那一条同一条纪律） */
	require.NotContains(t, text, visibilityKey)
	require.NotContains(t, text, summaryKey)
}

// ---------------------------------------------------------------------------
// D：撤销 / 过期 / 给不存在的模式授权
// ---------------------------------------------------------------------------

func TestPrivateModes_RevokeTakesEffectOnTheNextRequest(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)
	key := env.keyOf["buyer"]

	_, _ = env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))
	_, withGrant := env.getJSON("/api/zsy/mode/list", key)
	require.Contains(t, modeIDsOf(t, withGrant), "audiobook")

	_, revoked := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/revoke",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))
	require.Equal(t, true, revoked["success"], "撤销失败：%v", revoked)

	/*
	 * ★★ "卖了要能收回"：撤销之后**下一次请求**就看不到 —— 判据每次现算，
	 * 没有任何地方缓存它（`entitlements.go` 文件头）。
	 */
	_, after := env.getJSON("/api/zsy/mode/list", key)
	require.Equal(t, []string{"mv"}, modeIDsOf(t, after), "撤销之后还看得到那一档")

	/* ⚠ 而文件那一条也要一起关掉（两处同一条判据） */
	_, file := env.getJSON("/api/zsy/mode/audiobook/file", key)
	require.NotEqual(t, true, file["success"], "撤销之后文件仍然取得到")
}

func TestPrivateModes_RevokingSomethingNotLiveIsReported(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)

	/*
	 * ⚠ 没生效的行却报成功 = 运营以为收回了，而用户手上还开着。
	 * 与 `zsy/world` 的 `revokeEntitlement` 同一条纪律（0 行是**运营的误操作**，
	 * 要报给他，不是静默成功）。
	 */
	_, payload := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/revoke",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))
	require.NotEqual(t, true, payload["success"], "没有可撤销的行却报了成功")
	require.Contains(t, messageOf(t, payload), "没有可撤销的行")
}

func TestPrivateModes_ExpiredGrantDoesNotCount(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)
	key := env.keyOf["buyer"]

	/* 一小时前就过期了 */
	_, granted := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook","expires_at":%d}`, userID, nowStamp()-3600))
	require.Equal(t, true, granted["success"])

	/*
	 * ⚠★ 到期了还能用是**收费**那一类模式最要命的坏法，而它的表现是
	 * "什么都没发生" —— 所以这一条单独钉。
	 */
	_, payload := env.getJSON("/api/zsy/mode/list", key)
	require.Equal(t, []string{"mv"}, modeIDsOf(t, payload), "过期了的授权仍然算数")

	/* ★ 而一个还没到期的算数（否则上一条可能只是"授权根本没写进去"） */
	_, _ = env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"podcast","expires_at":%d}`, userID, nowStamp()+3600))
	_, later := env.getJSON("/api/zsy/mode/list", key)
	require.ElementsMatch(t, []string{"mv", "podcast"}, modeIDsOf(t, later))
}

func TestPrivateModes_GrantingAnUnknownModeIsRefused(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)

	/*
	 * ⚠★ 给一个模式库里没有的 id 授权，产生的是一行**永远不起作用的记录**：
	 * 运营看到"已经授权了"，用户那边什么都没有 —— 两边都以为好了。
	 * 所以这里必须拒绝，并且说清**该先把文件放进去**（那句话要能照做）。
	 */
	_, payload := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"nosuchmode"}`, userID))
	require.NotEqual(t, true, payload["success"], "给不存在的模式授权居然成功了")
	require.Contains(t, messageOf(t, payload), "nosuchmode")
	require.Contains(t, messageOf(t, payload), "模式库")

	/* ⚠ 而且**没有留下任何一行**（"拒绝了"要看得到才算数） */
	var count int64
	require.NoError(t, env.db.Model(&ModeEntitlement{}).Count(&count).Error)
	require.EqualValues(t, 0, count)
}

func TestPrivateModes_GrantNeedsAUserId(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)

	_, payload := env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		`{"mode_id":"audiobook"}`)
	require.NotEqual(t, true, payload["success"], "没有 user_id 也授权成功了")
}

// ---------------------------------------------------------------------------
// 后台那一屏要读得到的几格
// ---------------------------------------------------------------------------

func TestAdminModeList_ReportsVisibilityAndGrantCount(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)
	_, _ = env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))

	_, payload := env.callAdmin(http.MethodGet, "/dashboard/zsy/mode/list", "")
	require.Equal(t, true, payload["success"], "后台列表失败：%v", payload)
	data, _ := payload["data"].(map[string]any)
	items, _ := data["items"].([]any)
	require.Len(t, items, 3, "后台要把全部三份都列出来（含 private 的）")

	byID := map[string]map[string]any{}
	for _, raw := range items {
		row, _ := raw.(map[string]any)
		byID[fmt.Sprint(row["id"])] = row
	}
	require.Equal(t, VisibilityPrivate, byID["audiobook"]["visibility"])
	require.Equal(t, VisibilityPublic, byID["mv"]["visibility"])
	/*
	 * ★ `granted` 是运营**唯一**看得见"我给谁开过"的那个数：一个 private
	 * 而 0 授权的模式多半意味着"他忘了给谁开"，而那个 0 必须摆在他眼前。
	 */
	require.EqualValues(t, 1, byID["audiobook"]["granted"])
	require.EqualValues(t, 0, byID["podcast"]["granted"], "没人有权限的那一档不是 0")
	require.EqualValues(t, 0, byID["mv"]["granted"])
}

func TestAdminEntitlements_AsksForOneAccountOrOneMode(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)
	_, _ = env.callAdmin(http.MethodPost, "/dashboard/zsy/mode/entitlements/grant",
		fmt.Sprintf(`{"user_id":%d,"mode_id":"audiobook"}`, userID))

	/* 按账号问：拿到"他手上有什么"（含 active 那一份现算的名单） */
	_, byUser := env.callAdmin(http.MethodGet,
		fmt.Sprintf("/dashboard/zsy/mode/entitlements?user_id=%d", userID), "")
	require.Equal(t, true, byUser["success"], "%v", byUser)
	userData, _ := byUser["data"].(map[string]any)
	require.Equal(t, []any{"audiobook"}, userData["active"], "active 那一份不对：%v", userData)

	/* 按模式问：拿到"这一档给了谁" */
	_, byMode := env.callAdmin(http.MethodGet,
		"/dashboard/zsy/mode/entitlements?mode_id=audiobook", "")
	require.Equal(t, true, byMode["success"], "%v", byMode)
	modeData, _ := byMode["data"].(map[string]any)
	modeItems, _ := modeData["items"].([]any)
	require.Len(t, modeItems, 1)

	/*
	 * ⚠★ 两个都不给时**必须报错**，不许退化成"把所有授权都拉出来"：
	 * 授权表随账号数线性长大，而"全表列举"既没有界面用得上、
	 * 也会在某一天变成一次慢查询（与 `zsy/world` 拒绝全表同一条）。
	 */
	_, all := env.callAdmin(http.MethodGet, "/dashboard/zsy/mode/entitlements", "")
	require.NotEqual(t, true, all["success"], "不给 user_id / mode_id 却列举了整张表")
	require.Contains(t, messageOf(t, all), "user_id")

	/* 写错的参数值也不许当成"没给"（`?user_id=abc`） */
	_, bad := env.callAdmin(http.MethodGet, "/dashboard/zsy/mode/entitlements?user_id=abc", "")
	require.NotEqual(t, true, bad["success"], "user_id=abc 被当成了没给")
}

func TestAdminEntitlements_RefusesBothFiltersAtOnce(t *testing.T) {
	privateLibrary(t)
	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	userID := env.seedAccount("buyer", common.RoleCommonUser)

	_, payload := env.callAdmin(http.MethodGet,
		fmt.Sprintf("/dashboard/zsy/mode/entitlements?user_id=%d&mode_id=audiobook", userID), "")
	require.NotEqual(t, true, payload["success"], "两个筛选条件同时给了却当成一个")
}
