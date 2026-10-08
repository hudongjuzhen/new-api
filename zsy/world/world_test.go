package world

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/extcore"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// =========================================================================
// world_test.go — step 1 acceptance (docs/23 §9 step 1 and §8.3 ①)
//
// A. an account without `world-ip` gets 403 / E_ENTITLEMENT, and the body
//    carries no document, no element and no prompt;
// B. an account with `world-ip` reads a hand-written empty world back verbatim;
// C. revoking through the admin face takes effect on the very next op;
// D. plus the guards that keep those three honest: the route tree, the
//    capability table, expiry, staleness, and owner scoping.
// =========================================================================

// ---------------------------------------------------------------------------
// Harness
// ---------------------------------------------------------------------------

type testEnv struct {
	t      *testing.T
	db     *gorm.DB
	router *gin.Engine
	// keyOf maps a seeded username to the account key that authenticates it on
	// the op face.
	keyOf map[string]string
	// adminOf maps a seeded admin username to its dashboard access token, which is
	// what middleware.AdminAuth accepts.
	adminOf map[string]string
	// idOf maps a seeded username to its user id.
	idOf map[string]int
}

// emptyWorldDoc is the hand-written world of acceptance criterion B.
//
// ⚠★ Its shape was CORRECTED in step 2, after the authoritative validator was
// run against it. The first version — modelled on `schema/1.1/examples/minimal.mtw`
// with an element in the style of 工具1's `ElementRow` — was **not** schema-valid,
// and `validate.mjs` said exactly why:
//
//	/elements/0 must have required property 'metadata'
//	/elements/0 must NOT have additional properties {"additionalProperty":"first_chapter"}
//	/elements/0 must NOT have additional properties {"additionalProperty":"status"}
//	/element_types/0 must have required property 'enabled'
//	/element_types/0 must have required property 'field_template'
//	/element_types/0/source must be equal to one of the allowed values ["base","auto","manual"]
//
// An element carries `metadata` (keys come from its type's `field_template`) and
// `first_appear_chapter` / `appear_chapters` — NOT `first_chapter` / `status`.
// That pair belongs to 工具1's *view* struct in `api.rs`, not to the format. And an
// `element_type` needs `source` ∈ {base,auto,manual} plus `enabled` and
// `field_template`; `"source": "builtin"` is not a legal value.
//
// Keeping this fixture schema-valid matters beyond tidiness: step 2's acceptance
// criterion is a byte-for-byte comparison against a local `validate.mjs` run, and
// a fixture that trips the shape layer would make that comparison exercise the
// error path instead of the happy path. The element is kept so "the unauthorised
// response contains no element" is checked against real content.
const emptyWorldDoc = `{
  "format": "MTW",
  "version": "1.1",
  "meta": {
    "novel_title": "验收用手写世界",
    "chapter_count": 0,
    "parsed_at": "2026-09-19T10:00:00Z",
    "parser_name": "矩阵科技世界解析器",
    "parser_version": "0.1.0",
    "mtw_version": "1.1"
  },
  "element_types": [
    {
      "id": "type_character",
      "name": "角色",
      "name_en": "character",
      "source": "base",
      "enabled": true,
      "field_template": []
    }
  ],
  "elements": [
    {
      "id": "char_001",
      "type_id": "type_character",
      "name": "许云霄",
      "alias": ["云霄"],
      "metadata": {},
      "relations": [],
      "prompts": [],
      "sounds": []
    }
  ],
  "relations": [],
  "chapters": [],
  "events": [],
  "prompts": [],
  "sounds": [],
  "consistency_anchors": []
}`

// brokenWorldDoc is a document that is *shape*-valid but has a 规范 18.3 error:
// `char_001` points at a type that is not declared, and `rel_001` points at an
// element that does not exist. It exists so the validator-sidecar test can assert
// that a real verdict travels through Go unchanged — a document with no problems
// at all would only prove the happy path.
const brokenWorldDoc = `{
  "format": "MTW",
  "version": "1.1",
  "meta": {
    "novel_title": "有错的世界",
    "chapter_count": 0,
    "parsed_at": "2026-09-19T10:00:00Z",
    "parser_name": "矩阵科技世界解析器",
    "parser_version": "0.1.0",
    "mtw_version": "1.1"
  },
  "element_types": [],
  "elements": [
    { "id": "char_001", "type_id": "type_ghost", "name": "无类型", "metadata": {} }
  ],
  "relations": [
    { "id": "rel_001", "from": "char_001", "to": "char_999", "type": "认识" }
  ],
  "chapters": [],
  "events": [],
  "prompts": [],
  "sounds": [],
  "consistency_anchors": []
}`

// mustSameJSONBytes asserts that a document received from the server is the
// stored document, value for value.
//
// `got` is the decoded JSON value the response carried, so it is compared two
// ways: JSONEq for content, and a canonical-bytes comparison for "Go did not
// reshape it". Canonical means Go's own json.Marshal output (compact, keys
// sorted), applied to BOTH sides — so the assertion is about structure, not
// about the whitespace a fixture happens to use or the response's framing.
func mustSameJSONBytes(t *testing.T, stored []byte, got []byte, label string) {
	t.Helper()
	assert.JSONEq(t, string(stored), string(got), "%s differs in content", label)
	assert.Equal(t, canonicalJSON(t, stored), canonicalJSON(t, got),
		"%s was reshaped by the server (keys reordered or fields dropped)", label)
}

// canonicalJSON re-encodes JSON text in Go's canonical form.
func canonicalJSON(t *testing.T, raw []byte) string {
	t.Helper()
	var decoded any
	require.NoError(t, json.Unmarshal(raw, &decoded), "not valid JSON: %s", raw)
	out, err := json.Marshal(decoded)
	require.NoError(t, err)
	return string(out)
}

// newTestEnv boots an isolated stack: an in-memory SQLite database carrying the
// host tables this plugin touches (users, tokens) plus its own three, and a gin
// engine with the plugin's real route chain mounted through the extcore
// registry, i.e. the exact path router.SetRouter uses in production.
//
// Process-wide globals are snapshotted and restored. It deliberately avoids
// t.Parallel: model.DB is a process-wide global.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	origDB := model.DB
	origRedis, origMem := common.RedisEnabled, common.MemoryCacheEnabled
	origCfg := cfg

	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	t.Cleanup(func() {
		model.DB = origDB
		common.RedisEnabled, common.MemoryCacheEnabled = origRedis, origMem
		cfg = origCfg
	})

	dsn := fmt.Sprintf("file:zsy_world_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// One connection keeps the shared in-memory database alive for the whole
	// test even when GORM drops an idle connection.
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	t.Cleanup(func() { _ = sqlDB.Close() })

	// The reserved-word column quoting helpers (commonKeyCol …) are only
	// initialized by InitDB/InitLogDB. Pin LOG_SQL_DSN empty so InitLogDB takes
	// the "LOG_DB = DB" branch: that runs initCol() against this in-memory
	// database and, just as importantly, gives the admin audit path a LOG_DB to
	// write to — AdminAuth records every admin write, so a nil LOG_DB makes the
	// audit goroutine panic and the test unusable.
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())

	// The plugin's own three tables plus the host tables this package's tests
	// drive. The list is the plugin's migrate registration plus model.User /
	// model.Token / model.Log, so a table the plugin forgot to register fails here
	// rather than in production.
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.Log{}))
	require.NoError(t, db.AutoMigrate(extcore.ExtraMigrateModels()...))
	require.NoError(t, db.AutoMigrate(&WorldProject{}, &WorldSnapshot{}, &WorldEntitlement{}))

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
// user id.
//
// It writes through the database directly instead of model.User.Insert, whose
// post-insert sidebar step needs host option tables this package does not own.
// Everything the *plugin* reads — status, id, the token row — is produced by the
// host's own types.
func (e *testEnv) seedAccount(username string, role int) int {
	e.t.Helper()

	user := &model.User{
		Username:    username,
		Password:    "$2a$10$0123456789012345678901uZ0eGmvJ1kRw4Q1W4Wv8Q6dGmBdGFmC",
		DisplayName: username,
		Role:        role,
		Status:      common.UserStatusEnabled,
		// users.aff_code carries a unique index and Insert is what normally fills
		// it; a direct insert has to supply one.
		AffCode: common.GetRandomString(8),
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
// access token that middleware.AdminAuth() accepts, and returns it.
//
// ⚠ The two faces really do take different credentials: the op face
// (/api/zsy/world/**) authenticates the relay-style account key, while the admin
// face (/dashboard/zsy/world/**) authenticates a dashboard access token —
// exactly like the rest of new-api. The tests drive both with real credentials on
// purpose: grant/revoke is what acceptance criterion C turns on, so a hook that
// bypassed AdminAuth would be testing the wrong door.
func (e *testEnv) ensureAdminAccessToken() string {
	e.t.Helper()

	if token, ok := e.adminOf[adminUsername]; ok {
		return token
	}
	adminID, ok := e.idOf[adminUsername]
	require.True(e.t, ok, "call seedAccount(%q, common.RoleAdminUser) first", adminUsername)

	accessToken := common.GetRandomString(32)
	require.NoError(e.t,
		e.db.Model(&model.User{}).Where("id = ?", adminID).
			Update("access_token", accessToken).Error,
		"seed dashboard access token for %s", adminUsername)
	e.adminOf[adminUsername] = accessToken
	return accessToken
}

// callOp posts one op through the mounted route chain.
func (e *testEnv) callOp(username string, body string) (*httptest.ResponseRecorder, map[string]any) {
	e.t.Helper()
	return e.postJSON("/api/zsy/world/op", body, e.keyOf[username])
}

// recordedCallOp posts one op while recording the engine request it produced, and
// answers both.
//
// ★ It exists because "what we sent" and "what the engine did with it" have to be
// compared **within one run**. Producing the record in one test and replaying it
// in another looks equivalent and is not: the two tests race (this suite runs
// t.Parallel in places), so the reader can look before the writer has written —
// which is exactly what the first version of these diagnostics did.
func (e *testEnv) recordedCallOp(username string, body string) (*httptest.ResponseRecorder, map[string]any, []byte) {
	e.t.Helper()

	var mu sync.Mutex
	var captured []byte
	previous := engineRequestRecorder
	engineRequestRecorder = func(payload []byte) {
		mu.Lock()
		defer mu.Unlock()
		if captured == nil {
			captured = append([]byte(nil), payload...)
		}
	}
	e.t.Cleanup(func() { engineRequestRecorder = previous })

	rec, payload := e.callOp(username, body)

	mu.Lock()
	defer mu.Unlock()
	return rec, payload, captured
}

// postJSON posts a raw JSON body with an optional account key.
func (e *testEnv) postJSON(path string, body string, key string) (*httptest.ResponseRecorder, map[string]any) {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, decodeEnvelope(e.t, rec.Body.Bytes())
}

// getJSON issues a GET with an optional account key.
func (e *testEnv) getJSON(path string, key string) (*httptest.ResponseRecorder, map[string]any) {
	e.t.Helper()

	req := httptest.NewRequest(http.MethodGet, path, nil)
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)
	return rec, decodeEnvelope(e.t, rec.Body.Bytes())
}

func decodeEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()
	payload := map[string]any{}
	require.NoError(t, json.Unmarshal(body, &payload), "response is not JSON: %s", body)
	return payload
}

// codeOf reads the contract code of an envelope.
func codeOf(t *testing.T, payload map[string]any) string {
	t.Helper()
	code, ok := payload["code"].(string)
	require.True(t, ok, "envelope has no code: %v", payload)
	return code
}

// dataMap reads the `data` object of an envelope.
func dataMap(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "data missing in %v", payload)
	return data
}

// jsonInt reads an integer-valued JSON number.
func jsonInt(t *testing.T, m map[string]any, key string) int64 {
	t.Helper()
	v, ok := m[key].(float64)
	require.True(t, ok, "%s missing or not a number in %v", key, m)
	return int64(v)
}

// mustGrant grants a capability through the admin route, i.e. the same path an
// operator uses, so the tests exercise the real write face too.
func (e *testEnv) mustGrant(admin string, userID int, capability string, expiresAt int64) {
	e.t.Helper()
	require.Equal(e.t, adminUsername, admin, "the harness only wires an admin token for %q", adminUsername)
	body := fmt.Sprintf(`{"user_id":%d,"capability":%q,"source":"admin","expires_at":%d}`,
		userID, capability, expiresAt)
	rec, payload := e.postJSON("/dashboard/zsy/world/entitlements/grant", body, e.ensureAdminAccessToken())
	require.Equal(e.t, http.StatusOK, rec.Code, "grant failed: %s", rec.Body.String())
	require.Equal(e.t, true, payload["success"], "grant failed: %s", rec.Body.String())
}

// mustRevoke revokes a capability through the admin route.
func (e *testEnv) mustRevoke(admin string, userID int, capability string) {
	e.t.Helper()
	require.Equal(e.t, adminUsername, admin, "the harness only wires an admin token for %q", adminUsername)
	body := fmt.Sprintf(`{"user_id":%d,"capability":%q}`, userID, capability)
	rec, payload := e.postJSON("/dashboard/zsy/world/entitlements/revoke", body, e.ensureAdminAccessToken())
	require.Equal(e.t, http.StatusOK, rec.Code, "revoke failed: %s", rec.Body.String())
	require.Equal(e.t, true, payload["success"], "revoke failed: %s", rec.Body.String())
}

// mustSeedWorld plants one document version for an account and returns the
// project id and version.
func (e *testEnv) mustSeedWorld(owner string, doc string) (uint, int64) {
	e.t.Helper()
	userID := e.idOf[owner]
	project, err := WorldProjectCreate(userID, "手写世界", "验收用小说")
	require.NoError(e.t, err)
	snapshot, err := WorldSnapshotAppend(project.ID, []byte(doc), "seed")
	require.NoError(e.t, err)
	return project.ID, snapshot.Version
}

// mountRoutesForTest mounts the plugin's real route tree on a bare engine so a
// test can pin the published URLs and prove the registration does not panic. The
// production mountRoutes stays unexported, mirroring zsy/voice's test hook.
func mountRoutesForTest(router *gin.Engine) { mountRoutes(router) }

// ---------------------------------------------------------------------------
// The registered shape: routes, tables, plugin metadata
// ---------------------------------------------------------------------------

func TestMountRoutes_PublishesTheDocumentedURLs(t *testing.T) {
	router := gin.New()
	require.NotPanics(t, func() { mountRoutesForTest(router) })

	registered := make([]string, 0, 12)
	for _, route := range router.Routes() {
		registered = append(registered, route.Method+" "+route.Path)
	}
	assert.ElementsMatch(t, []string{
		"POST /api/zsy/world/op",
		"GET /api/zsy/world/entitlements",
		// ★ 公共插件目录（docs/27 §3）：私有模板一个字节都不在这一面上
		"GET /api/zsy/plugins",
		"GET /dashboard/zsy/world/projects",
		"GET /dashboard/zsy/world/projects/:id",
		"GET /dashboard/zsy/world/projects/:id/versions/:version",
		"GET /dashboard/zsy/world/entitlements",
		"POST /dashboard/zsy/world/entitlements/grant",
		"POST /dashboard/zsy/world/entitlements/revoke",
		// 插件文件：列模板 + 按账号签发一份（docs/22 §2.3 / docs/23 §12.15）
		"GET /dashboard/zsy/world/plugins",
		"POST /dashboard/zsy/world/plugins/issue",
	}, registered)
}

func TestExtension_RegistersPluginAndThreeTables(t *testing.T) {
	names := make([]string, 0, 8)
	for _, info := range extcore.Plugins() {
		names = append(names, info.Name)
	}
	assert.Contains(t, names, pluginName)

	// The three tables of docs/23 §6.3 must be in the host's AutoMigrate list;
	// a table missing here would exist only because a test created it.
	targets := extcore.ExtraMigrateModels()
	tableNames := make([]string, 0, len(targets))
	for _, target := range targets {
		if named, ok := target.(interface{ TableName() string }); ok {
			tableNames = append(tableNames, named.TableName())
		}
	}
	assert.Contains(t, tableNames, "zsy_world_projects")
	assert.Contains(t, tableNames, "zsy_world_snapshots")
	assert.Contains(t, tableNames, "zsy_world_entitlements")
}

func TestOpTable_MatchesTheContract(t *testing.T) {
	// docs/23 §4.2, row for row: which op needs which capability, and which are
	// login-only. This is the table the gate reads, so a silent change here would
	// change who can do what.
	want := map[string]string{
		"world.get":      "",
		"world.validate": "",
		"project.create": CapabilityWorldIP,
		"world.mutate":   CapabilityWorldIP,
		"assets.plan":    CapabilityWorldIPAI,
		"ingest.run":     CapabilityWorldIPAI,
	}

	// ★ Every documented op must be present with exactly the documented capability.
	// Checking this direction (rather than exact set equality) is what lets a new
	// read-only op be added deliberately — see below — without weakening the
	// guarantee that matters: nobody can quietly change who may call what.
	byOp := map[string]string{}
	for _, spec := range opTable {
		byOp[spec.Op] = spec.Capability
	}
	for op, capability := range want {
		got, present := byOp[op]
		require.True(t, present, "op %q from docs/23 §4.2 is missing from the table", op)
		assert.Equal(t, capability, got, "op %q capability", op)
	}

	// ★ The two additions beyond §4.2, and their shared condition.
	//
	// `world.entitlements` and `project.list` exist because the desktop client has a
	// single network path to this plugin (`world_op` in Rust), so a lone GET would
	// mean a second place in the client that knows the plugin's URL. They are allowed
	// only because each is BOTH login-only AND read-only:
	//
	//   - login-only: neither may gate anything, and
	//   - read-only:  neither may change a world.
	//
	// Asserting the capability here means a future edit that quietly gated them — or
	// gave them a capability — fails this test rather than silently changing who can
	// see their own worlds.
	for _, op := range []string{"world.entitlements", "project.list"} {
		spec := opByName(op)
		require.NotNil(t, spec, "%s must stay in the table", op)
		assert.Equal(t, "", spec.Capability,
			"%s must stay login-only: it is a read of the caller's own state, not a gate (§4.2)", op)
	}
	require.Len(t, opTable, len(want)+2,
		"the op surface changed size beyond the documented additions: %v", knownOpsText())
}

func TestErrorCodes_AreTheSevenLiterals(t *testing.T) {
	// The literals are the cross-language contract (docs/23 §4.1): a client
	// branches on them. Pinning them here means a rename cannot ship silently.
	assert.Equal(t, "E_ENTITLEMENT", CodeEntitlement)
	assert.Equal(t, "E_QUOTA", CodeQuota)
	assert.Equal(t, "E_AUTH", CodeAuth)
	assert.Equal(t, "E_INPUT", CodeInput)
	assert.Equal(t, "E_CONFLICT", CodeConflict)
	assert.Equal(t, "E_STALE", CodeStale)
	assert.Equal(t, "E_UPSTREAM", CodeUpstream)

	// Two of the statuses are pinned by the design document; the rest are this
	// plugin's convention and are listed so a change is a deliberate act.
	assert.Equal(t, http.StatusForbidden, httpStatusForCode(CodeEntitlement), "docs/23 §8.3 ①")
	assert.Equal(t, http.StatusUnauthorized, httpStatusForCode(CodeAuth))
	assert.Equal(t, http.StatusPaymentRequired, httpStatusForCode(CodeQuota))
	assert.Equal(t, http.StatusBadRequest, httpStatusForCode(CodeInput))
	assert.Equal(t, http.StatusConflict, httpStatusForCode(CodeConflict))
	assert.Equal(t, http.StatusConflict, httpStatusForCode(CodeStale))
	assert.Equal(t, http.StatusBadGateway, httpStatusForCode(CodeUpstream))
}

// ---------------------------------------------------------------------------
// E_AUTH — the entrance itself
// ---------------------------------------------------------------------------

func TestOp_WithoutCredentialsIsE_AUTH(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("nobody", common.RoleCommonUser)

	body := `{"op":"world.get","params":{"project_id":1}}`

	rec, payload := env.postJSON("/api/zsy/world/op", body, "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, CodeAuth, codeOf(t, payload))
	assert.Equal(t, false, payload["success"])
	assert.NotContains(t, rec.Body.String(), "许云霄")

	rec, payload = env.postJSON("/api/zsy/world/op", body, "this-key-does-not-exist")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, CodeAuth, codeOf(t, payload))

	rec, payload = env.getJSON("/api/zsy/world/entitlements", "")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, CodeAuth, codeOf(t, payload))
}

// ---------------------------------------------------------------------------
// Criterion A — no capability, no content
// ---------------------------------------------------------------------------

// TestNoCapability_CapabilityGatedOpIsRefusedAndLeaksNothing is enforcement
// criterion A (docs/23 §8.3 ①): a key without `world-ip` must be answered 403
// with E_ENTITLEMENT, and the body must not carry a document, an element or a
// prompt.
//
// ★ The op under test is deliberately a capability-gated one. docs/23 §4.2 makes
// `world.get` and `world.validate` **login-only** on purpose — they answer "我买过
// 了，我要用我的世界" — so calling world.get here would be testing the opposite of
// what the criterion says. `world.mutate` is the lightest op that requires
// `world-ip`, and its refusal is what criterion A is about.
//
// The response is checked against content the server actually holds: two worlds
// are planted before the call — one owned by the caller, one by a third party —
// so whichever a broken gate leaked, the assertion catches real data.
func TestNoCapability_CapabilityGatedOpIsRefusedAndLeaksNothing(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("poor", common.RoleCommonUser)
	env.seedAccount("owner", common.RoleCommonUser)

	// A world owned by someone else, and one owned by the caller.
	_, _ = env.mustSeedWorld("owner", emptyWorldDoc)
	ownProjectID, ownVersion := env.mustSeedWorld("poor", emptyWorldDoc)
	require.Empty(t, env.capabilitiesOf("poor"), "criterion A needs an account with no capability")

	body := fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"mutation":"rename"}}`,
		ownProjectID, ownVersion)
	rec, payload := env.callOp("poor", body)

	// ── the contract ────────────────────────────────────────────────────────
	assert.Equal(t, http.StatusForbidden, rec.Code, "docs/23 §8.3 ①")
	assert.Equal(t, false, payload["success"])
	assert.Equal(t, CodeEntitlement, codeOf(t, payload))
	assert.NotEmpty(t, payload["message"])

	// ── the leak check ──────────────────────────────────────────────────────
	// Nothing that is world content may appear: no document, no element, no
	// prompt, no type table.
	forbidden := []string{
		"element_types", "consistency_anchors",
		`"elements"`, "char_001", "type_character", "许云霄", "novel_title",
		"chapter_count", "手写世界",
	}
	raw := rec.Body.String()
	for _, needle := range forbidden {
		assert.NotContains(t, raw, needle,
			"an unauthorised response leaked %q: %s", needle, raw)
	}
	// The failure envelope carries no `data` field at all, which is the structural
	// reason a leak cannot happen later either.
	assert.NotContains(t, payload, "data", "failure envelopes must not carry data")

	// And the world is untouched: a refused mutation must not have half-run.
	after, err := WorldSnapshotDocRaw(ownProjectID, ownVersion)
	require.NoError(t, err)
	assert.Equal(t, canonicalJSON(t, []byte(emptyWorldDoc)), canonicalJSON(t, after),
		"the refused op must not have modified the document")
	project, err := WorldProjectGet(ownProjectID)
	require.NoError(t, err)
	assert.EqualValues(t, ownVersion, project.CurrentVersion, "no new version was written")
}

// TestNoCapability_GateRunsBeforeInputMeansNoInfoLeak: for a capability-gated op
// the entitlement check precedes parameter decoding (op_dispatch.go), so an
// unauthorized caller cannot learn project ids, version numbers, or even which
// fields the op wants by reading E_INPUT messages.
//
// A login-only op is the control: there the gate is login, so the same nonsense
// parameters do reach the decoder and answer E_INPUT. The contrast is the point —
// it shows the gate ordering is per-op, not a blanket "everything is refused".
func TestNoCapability_GateRunsBeforeInputMeansNoInfoLeak(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("poor", common.RoleCommonUser)
	require.Empty(t, env.capabilitiesOf("poor"))

	// Gated ops: nonsense parameters never get read.
	for _, body := range []string{
		`{"op":"world.mutate","params":{}}`,
		`{"op":"world.mutate","params":{"project_id":"不是数字"}}`,
		`{"op":"project.create","params":{"name":""}}`,
	} {
		rec, payload := env.callOp("poor", body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "body=%s", body)
		assert.Equal(t, CodeEntitlement, codeOf(t, payload), "body=%s", body)
		assert.NotContains(t, payload, "data", "body=%s", body)
	}

	// Login-only ops: readable, so the decoder is what answers.
	for _, body := range []string{
		`{"op":"world.get","params":{}}`,
		`{"op":"world.get","params":{"project_id":"不是数字"}}`,
	} {
		rec, payload := env.callOp("poor", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", body)
		assert.Equal(t, CodeInput, codeOf(t, payload), "body=%s", body)
	}
}

// TestNoCapability_IngestNeedsTheAICapabilityNotTheplainOne: docs/23 §6.3 keeps
// two capabilities because their cost differs by an order of magnitude. Holding
// `world-ip` must not open the two ops that burn model money.
func TestNoCapability_IngestNeedsTheAICapabilityNotThePlainOne(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("plain", common.RoleCommonUser)

	env.mustGrant("admin", env.idOf["plain"], CapabilityWorldIP, 0)

	for _, body := range []string{
		`{"op":"ingest.run","params":{"project_id":1}}`,
		`{"op":"assets.plan","params":{"project_id":1}}`,
	} {
		rec, payload := env.callOp("plain", body)
		assert.Equal(t, http.StatusForbidden, rec.Code, "body=%s", body)
		assert.Equal(t, CodeEntitlement, codeOf(t, payload), "body=%s", body)
	}

	// ★ With the AI capability the op travels past the ENTITLEMENT gate — and then
	// stops at the next gate down the line: this request has no base_version, so it
	// is E_INPUT. (It used to be E_UPSTREAM back when ingest.run was a placeholder;
	// now that it is implemented, the honest answer is "your request is incomplete",
	// which is exactly the distinction docs/23 §4.1 draws between the two codes.)
	//
	// The point of the assertion is unchanged and is what this test is for: the
	// capability gate, not the implementation status, decides who gets in.
	env.mustGrant("admin", env.idOf["plain"], CapabilityWorldIPAI, 0)
	rec, payload := env.callOp("plain", `{"op":"ingest.run","params":{"project_id":1}}`)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, CodeInput, codeOf(t, payload))
	assert.Contains(t, payload["message"], "base_version")
}

// realEngineDocPath locates a real .mtw produced by 工具1's own pipeline, or "".
func realEngineDocPath(t *testing.T) string {
	t.Helper()

	if p := strings.TrimSpace(os.Getenv(engineProducedDocEnv)); p != "" {
		if abs, err := filepath.Abs(p); err == nil && fileExists(abs) {
			return abs
		}
	}

	roots := make([]string, 0, 2)
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Dir(thisFile))
	}
	for _, root := range roots {
		for _, base := range engineRepoRoots(root) {
			candidate := filepath.Join(base, "apps", "out", "demo-world.mtw")
			if fileExists(candidate) {
				return candidate
			}
		}
	}
	return ""
}

// mustRealWorldDoc loads a real engine-produced document and returns it as text.
//
// ★★ Why the mutation tests need THIS and not a hand-written fixture — the single
// most important lesson of step 3:
//
// `world.mutate` works on the document's `meta.id_registry`, because that is where
// the engine's identity model lives. A hand-written document without a registry
// loads as an EMPTY registry, so every mutation answers "注册表里没有 char_001" — the
// engine is right and the fixture is unusable. Measured, not guessed: the first
// version of these tests failed exactly that way.
//
// The plugin itself is fine with such a document (it never inspects one), which is
// why `world.get`/`world.validate` tests can still use hand-written fixtures: those
// ops do not need the registry. Only mutations do.
func mustRealWorldDoc(t *testing.T) string {
	t.Helper()

	path := realEngineDocPath(t)
	if path == "" {
		t.Skipf("找不到引擎产出的 .mtw：把 %s 指向一份，或运行 "+
			"`cargo run -p world-parser --bin world-parser-demo` 生成 apps/out/demo-world.mtw。",
			engineProducedDocEnv)
	}
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Greater(t, len(raw), 1000, "a real document should not be a stub: %s", path)
	return string(raw)
}

// findEngineProducedDoc is the older name kept for the step-2 acceptance test.
func findEngineProducedDoc(t *testing.T) string { return realEngineDocPath(t) }

// engineBinaryName is the Rust sidecar's file name. On Windows the built artifact
// carries .exe, so both spellings are probed.
const engineBinaryName = "world-parser-svc"

// engineTestTargetDir is the artifact directory of the TEST build.
//
// ★★ Why this exists, and why it is not tidiness:
//
// The billing tests need the engine built with `--features test-provider` (the
// `mock_script` channel). If that build shares `target/debug` with ordinary builds,
// then ANY later `cargo build` or `cargo test --workspace` silently REPLACES it with
// a production-shaped binary — and the billing tests then fail with "未配置 API key",
// i.e. they look like a credentials problem while actually being a stale-artifact
// problem. That happened twice during this work.
//
// So the test build gets its own target directory, which cargo never lets a
// different feature set overwrite. The build command is:
//
//	cd <engine repo>
//	$env:CARGO_TARGET_DIR = "target\svc-test"      # PowerShell
//	cargo build -p world-parser --bin world-parser-svc --features test-provider
//
// ⚠ CARGO_TARGET_DIR="target\svc-test" yields `target\svc-test\debug\<bin>` — the
// profile level is still there. Getting this wrong is silent: the lookup simply
// falls through to `target/debug` and finds the production binary. The strict
// helper below is what makes that loud instead of a false pass.
const engineTestTargetDir = "target/svc-test/debug"

// engineTargetDirs lists the artifact directories a sidecar binary may live in,
// in priority order: the dedicated test build first, then the ordinary builds.
func engineTargetDirs() []string {
	return []string{engineTestTargetDir, "target/debug", "target/release"}
}

// findEngineBinary locates the compiled `world-parser-svc`.
//
// Resolution order: an explicit override, then the engine checkout's
// `target/{debug,release}`. It answers "" when the binary has not been built —
// the caller skips with the build command, rather than failing: an un-built
// sidecar is a missing prerequisite, not a broken plugin.
func findEngineBinary(t *testing.T) string {
	t.Helper()

	if p := strings.TrimSpace(os.Getenv(engineBinaryEnv)); p != "" {
		if abs, err := filepath.Abs(p); err == nil && fileExists(abs) {
			return abs
		}
	}

	roots := make([]string, 0, 2)
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Dir(thisFile))
	}

	// ⚠ Loop order matters: target dirs are the OUTER loop, so the dedicated test
	// build wins over an ordinary debug build *anywhere* in the candidate list.
	// With the loops the other way round, the first root that happened to have a
	// plain `target/debug` binary won — which is precisely the stale, unmockable
	// build this ordering exists to avoid. (Found because the strict helper below
	// reported the wrong path instead of failing silently.)
	names := []string{engineBinaryName, engineBinaryName + ".exe"}
	bases := make([]string, 0, 8)
	for _, root := range roots {
		bases = append(bases, engineRepoRoots(root)...)
	}
	for _, profile := range engineTargetDirs() {
		for _, base := range bases {
			for _, name := range names {
				candidate := filepath.Join(base, filepath.FromSlash(profile), name)
				if fileExists(candidate) {
					return candidate
				}
			}
		}
	}
	return ""
}

// requireEngineSidecar wires the Rust engine up for a test, or skips with the
// exact command that would produce it.
func requireEngineSidecar(t *testing.T) {
	t.Helper()

	requireValidatorSidecar(t) // the gate needs the authority too

	binary := findEngineBinary(t)
	if binary == "" {
		t.Skipf(
			"世界引擎不可达：找不到 %s。先用 test-provider 构建它（见 engineTestTargetDir 的注释）：\n"+
				"  cd <引擎仓库>\n"+
				"  $env:CARGO_TARGET_DIR = \"target\\svc-test\"\n"+
				"  cargo build -p world-parser --bin world-parser-svc --features test-provider\n"+
				"（或用 %s 指定路径）",
			engineBinaryName, engineBinaryEnv)
	}

	original := cfg.EnginePath
	cfg.EnginePath = binary
	t.Cleanup(func() { cfg.EnginePath = original })
}

// requireMockableEngine additionally asserts the located engine honours
// `mock_script`, i.e. it really is the `test-provider` build.
//
// ★ It FAILS rather than skips: a skip here would let "the test binary got
// clobbered by a plain cargo build" present as a green suite with no billing
// coverage at all — the exact silent failure the dedicated target dir exists to
// prevent. A caller that genuinely cannot build with the feature should see a red
// test and a message naming the command.
func requireMockableEngine(t *testing.T) {
	t.Helper()
	requireEngineSidecar(t)

	req := fmt.Sprintf(
		`{"op":"ingest","source_text":"1. 甲\n正文甲。\n","max_chapters":1,"media":false,"mock_script":%s}`,
		mustJSONString(t, ingestMockScript))
	cmd := exec.Command(cfg.EnginePath)
	cmd.Stdin = strings.NewReader(req + "\n")
	out, err := cmd.Output()
	require.NoError(t, err, "running %s", cfg.EnginePath)

	var resp sidecarResponse
	require.NoError(t, json.Unmarshal(out, &resp), "engine answer: %s", out)
	if resp.Status == "ok" {
		return
	}
	// A production-shaped binary ignores `mock_script` and tries the real gateway,
	// which fails on missing credentials. Say so precisely.
	detail := ""
	if resp.Error != nil {
		detail = resp.Error.Message
	}
	t.Fatalf(
		"引擎 %s 不认 mock_script（它看起来是**生产构建**），计费测试无法在不花钱的前提下运行。\n"+
			"引擎的回答：%s\n"+
			"请按 engineTestTargetDir 的注释用 `--features test-provider` 重新构建。",
		cfg.EnginePath, detail)
}

// engineBinaryEnv points at the built Rust sidecar explicitly.
const engineBinaryEnv = "ZSY_WORLD_ENGINE_BIN"

// ---------------------------------------------------------------------------
// Criterion B — a capable account reads its world back verbatim
// ---------------------------------------------------------------------------

func TestCapableAccount_WorldGetReturnsTheHandWrittenWorld(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)
	require.EqualValues(t, 1, version, "a fresh project's first snapshot is version 1")

	body := fmt.Sprintf(`{"op":"world.get","params":{"project_id":%d}}`, projectID)
	rec, payload := env.callOp("writer", body)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, true, payload["success"])
	assert.Equal(t, CodeOK, codeOf(t, payload))

	data := dataMap(t, payload)
	assert.EqualValues(t, projectID, jsonInt(t, data, "project_id"))
	assert.EqualValues(t, 1, jsonInt(t, data, "version"))
	assert.Equal(t, true, data["changed"])
	assert.Equal(t, "手写世界", data["project_name"])

	doc, ok := data["doc"].(map[string]any)
	require.True(t, ok, "doc missing or not an object: %v", data)
	assert.Equal(t, "MTW", doc["format"])
	assert.Equal(t, "1.1", doc["version"])
	// The element survived: the world is readable, not merely non-empty.
	elements, ok := doc["elements"].([]any)
	require.True(t, ok, "elements missing: %v", doc)
	require.Len(t, elements, 1)
	first, ok := elements[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "char_001", first["id"])
	assert.Equal(t, "许云霄", first["name"])

	// ★ The strongest form of "原样取回": the bytes Go emits for the doc are the
	// bytes that were stored. A round-trip through map[string]any would reorder
	// keys and quietly become a second, lossy definition of the format.
	stored, err := WorldSnapshotDocRaw(projectID, version)
	require.NoError(t, err)
	encoded, err := json.Marshal(data["doc"])
	require.NoError(t, err)
	mustSameJSONBytes(t, stored, encoded, "world.get doc")
}

func TestCapableAccount_SinceVersionIsTheSilentRefetchPath(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)

	// Already current → no document on the wire (docs/23 §5.2 "一致 → 什么都不做").
	body := fmt.Sprintf(`{"op":"world.get","params":{"project_id":%d,"since_version":%d}}`, projectID, version)
	rec, payload := env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := dataMap(t, payload)
	assert.Equal(t, false, data["changed"])
	assert.NotContains(t, data, "doc")

	// Behind → the document comes back.
	body = fmt.Sprintf(`{"op":"world.get","params":{"project_id":%d,"since_version":%d}}`, projectID, version-1)
	rec, payload = env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data = dataMap(t, payload)
	assert.Equal(t, true, data["changed"])
	assert.Contains(t, data, "doc")

	// Ahead → E_STALE: a client must not be allowed to hold a version the server
	// never issued, or it could later overwrite good data with it.
	body = fmt.Sprintf(`{"op":"world.get","params":{"project_id":%d,"since_version":%d}}`, projectID, version+1)
	rec, payload = env.callOp("writer", body)
	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Equal(t, CodeStale, codeOf(t, payload))
	assert.NotContains(t, payload, "data")
}

func TestWorldGet_CannotReadSomeoneElsesWorld(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	env.seedAccount("stranger", common.RoleCommonUser)
	env.mustGrant("admin", env.idOf["stranger"], CapabilityWorldIP, 0)

	projectID, _ := env.mustSeedWorld("writer", emptyWorldDoc)

	body := fmt.Sprintf(`{"op":"world.get","params":{"project_id":%d}}`, projectID)
	rec, payload := env.callOp("stranger", body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, CodeInput, codeOf(t, payload))
	assert.NotContains(t, rec.Body.String(), "许云霄")
	assert.NotContains(t, rec.Body.String(), "char_001")
}

func TestWorldGet_UnknownProjectAndBadInputAreE_INPUT(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	cases := []struct {
		name string
		body string
	}{
		{"missing project_id", `{"op":"world.get","params":{}}`},
		{"unknown project", `{"op":"world.get","params":{"project_id":99999}}`},
		{"negative since_version", `{"op":"world.get","params":{"project_id":1,"since_version":-1}}`},
		{"unknown op", `{"op":"world.destroy","params":{}}`},
		{"absent op", `{"params":{}}`},
		{"not json", `{not json`},
		{"wrong field type", `{"op":"world.get","params":{"project_id":"abc"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec, payload := env.callOp("writer", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
			assert.Equal(t, CodeInput, codeOf(t, payload))
			assert.NotEmpty(t, payload["message"])
			assert.NotContains(t, payload, "data")
		})
	}
}

// ---------------------------------------------------------------------------
// world.validate — the authoritative verdict, transported
// ---------------------------------------------------------------------------
//
// ⚠ These tests only run when the validator sidecar is actually reachable, and
// they SKIP (loudly, naming the knob) when it is not. That is deliberate: a test
// that silently passed with no validator would be worse than no test, because it
// would make "the sidecar is wired" unverifiable. See requireValidatorSidecar.

// TestWorldValidate_TransportsTheAuthoritativeVerdict is the step-2 acceptance
// test (docs/23 §9 step 2: "一次 world.validate 走通，结果与本地跑 validate.mjs
// 逐字一致").
//
// It runs the real thing twice — once through the op (Go → Node shim → validator),
// once by calling validate.mjs directly — and asserts the two agree item for item.
// A Go-side re-derivation of any rule would show up here as a difference, which is
// the only reason this test is worth having.
func TestWorldValidate_TransportsTheAuthoritativeVerdict(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	// Login-only (docs/23 §4.2): no `world-ip` needed, and none is granted here.
	require.Empty(t, env.capabilitiesOf("writer"))

	for _, tc := range []struct {
		name     string
		doc      string
		wantOK   bool
		wantErrs int
		// wantWarn is asserted exactly, not as a lower bound, so a validator
		// version bump that changes its warning set shows up here rather than
		// silently passing.
		wantWarn int
	}{
		// W7 (no meta.id_registry) + W5 (char_001 has neither relations nor
		// appear_chapters).
		{"schema-valid document", emptyWorldDoc, true, 0, 2},
		// E02 (unknown type_id) + E06 (relation points at a missing element);
		// warnings: W7 only — this document's element DOES have a relation, so W5
		// does not fire. (Measured, not assumed: a guessed value failed here first.)
		{"shape-valid but has 18.3 errors", brokenWorldDoc, false, 2, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// ⚠ Wire the sidecar FIRST: requireValidatorSidecar is what points
			// cfg.ValidatorSvcPath at the located shim, and anything the op does
			// before that call sees the unresolved default. (Learned by getting it
			// backwards: the op answered E_UPSTREAM and the comparison below never
			// ran.)
			requireValidatorSidecar(t)

			body := `{"op":"world.validate","params":{"doc":` + tc.doc + `}}`
			rec, payload := env.callOp("writer", body)
			require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
			assert.Equal(t, true, payload["success"])
			assert.Equal(t, CodeOK, codeOf(t, payload))

			data := dataMap(t, payload)
			assert.Equal(t, tc.wantOK, data["ok"], "verdict must be the validator's")
			assert.Equal(t, true, data["shape_ok"])
			assert.Equal(t, false, data["strict"], "§4.2 semantics: warnings do not fail")
			assert.Equal(t, "schema/1.1/validate.mjs", data["engine"])
			// `file` is a server-internal temp path and must not leak (§8.3 ④).
			assert.NotContains(t, data, "file")

			errorsOf := issuesOf(t, data, "errors")
			warningsOf := issuesOf(t, data, "warnings")
			require.Len(t, errorsOf, tc.wantErrs, "errors=%v", errorsOf)
			require.Len(t, warningsOf, tc.wantWarn, "warnings=%v", warningsOf)
			assert.Empty(t, data["shape_errors"])

			// ── 逐字一致：compare against a local validate.mjs run ──────────────
			want := runValidatorDirectly(t, tc.doc)
			assert.Equal(t, want.ShapeOK, data["shape_ok"])
			assert.Equal(t, want.Errors, errorsOf, "errors must be byte-equal to the validator's")
			assert.Equal(t, want.Warnings, warningsOf, "warnings must be byte-equal to the validator's")
		})
	}
}

// TestWorldValidate_StoredVersionGoesThroughTheSameValidator: the stored path
// must produce the same verdict as the inline path for the same bytes. If it did
// not, "validate what I just saved" and "validate what I am about to save" would
// disagree — the worst possible failure for a consistency feature.
func TestWorldValidate_StoredVersionGoesThroughTheSameValidator(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireValidatorSidecar(t) // wire the sidecar before any op runs
	projectID, version := env.mustSeedWorld("writer", brokenWorldDoc)

	// Inline.
	inlineBody := `{"op":"world.validate","params":{"doc":` + brokenWorldDoc + `}}`
	rec, payload := env.callOp("writer", inlineBody)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	inline := dataMap(t, payload)

	for _, tc := range []struct {
		name string
		body string
	}{
		{"explicit version", fmt.Sprintf(
			`{"op":"world.validate","params":{"project_id":%d,"version":%d}}`, projectID, version)},
		{"current version", fmt.Sprintf(
			`{"op":"world.validate","params":{"project_id":%d}}`, projectID)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, payload := env.callOp("writer", tc.body)
			require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
			data := dataMap(t, payload)

			assert.Equal(t, inline["ok"], data["ok"])
			assert.Equal(t, inline["errors"], data["errors"])
			assert.Equal(t, inline["warnings"], data["warnings"])
			// The stored path echoes which version it answered about, so a client
			// cannot mis-attribute a verdict to the wrong snapshot.
			assert.EqualValues(t, projectID, jsonInt(t, data, "project_id"))
			assert.EqualValues(t, version, jsonInt(t, data, "version"))
		})
	}
}

func TestWorldValidate_BadInputIsE_INPUTBeforeTheEngineIsConsulted(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireValidatorSidecar(t) // wire the sidecar before any op runs
	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)

	t.Run("bad input", func(t *testing.T) {
		for _, body := range []string{
			`{"op":"world.validate","params":{}}`,
			`{"op":"world.validate","params":{"doc":"不是对象"}}`,
			`{"op":"world.validate","params":{"doc":[]}}`,
			`{"op":"world.validate","params":{"project_id":99999,"version":1}}`,
			fmt.Sprintf(`{"op":"world.validate","params":{"project_id":%d,"version":99}}`, projectID),
		} {
			rec, payload := env.callOp("writer", body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", body)
			assert.Equal(t, CodeInput, codeOf(t, payload), "body=%s", body)
		}
	})

	t.Run("someone else's version is not even read", func(t *testing.T) {
		env.seedAccount("stranger", common.RoleCommonUser)
		body := fmt.Sprintf(`{"op":"world.validate","params":{"project_id":%d,"version":%d}}`, projectID, version)
		rec, payload := env.callOp("stranger", body)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, CodeInput, codeOf(t, payload))
		assert.NotContains(t, rec.Body.String(), "许云霄")
	})

	t.Run("no capabilities needed and none held", func(t *testing.T) {
		require.Empty(t, env.capabilitiesOf("writer"))
		body := fmt.Sprintf(`{"op":"world.validate","params":{"project_id":%d}}`, projectID)
		rec, _ := env.callOp("writer", body)
		assert.Equal(t, http.StatusOK, rec.Code, "login-only op must not be gated by a capability")
	})
}

// TestWorldValidate_UnconfiguredValidatorIsE_UPSTREAM_NotACrash: a deployment
// that has not pointed the plugin at the engine must get an actionable
// E_UPSTREAM that names the knob — not a 500, and not a silently empty verdict.
//
// This test needs no validator at all: it deliberately breaks the path.
func TestWorldValidate_UnconfiguredValidatorIsE_UPSTREAM_NotACrash(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("writer", common.RoleCommonUser)

	original := cfg.ValidatorSvcPath
	cfg.ValidatorSvcPath = filepath.Join(t.TempDir(), "definitely-not-here.mjs")
	t.Cleanup(func() { cfg.ValidatorSvcPath = original })

	body := `{"op":"world.validate","params":{"doc":` + emptyWorldDoc + `}}`
	rec, payload := env.callOp("writer", body)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Equal(t, CodeUpstream, codeOf(t, payload))
	assert.Equal(t, false, payload["success"])
	assert.NotContains(t, payload, "data", "a failed op must not carry a verdict")
	message, _ := payload["message"].(string)
	assert.Contains(t, message, "权威校验器", "the message must name the component")
	assert.Contains(t, message, envValidatorSvc, "the message must name the knob to set")
	assert.Contains(t, message, "definitely-not-here.mjs", "the message must name the path tried")
}

// TestWorldValidate_TimeoutsAreE_UPSTREAM: a wedged child must not pin the
// request. Simulated with a node one-liner that sleeps, so the timeout path is
// exercised for real rather than assumed.
func TestWorldValidate_TimeoutsAreE_UPSTREAM(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("writer", common.RoleCommonUser)

	originalPath, originalTimeout := cfg.ValidatorSvcPath, cfg.SidecarTimeout
	sleeper := filepath.Join(t.TempDir(), "sleeper.mjs")
	require.NoError(t, os.WriteFile(sleeper, []byte("setTimeout(() => {}, 60000);\n"), 0o644))
	cfg.ValidatorSvcPath = sleeper
	cfg.SidecarTimeout = 500 * time.Millisecond
	t.Cleanup(func() {
		cfg.ValidatorSvcPath = originalPath
		cfg.SidecarTimeout = originalTimeout
	})

	body := `{"op":"world.validate","params":{"doc":` + emptyWorldDoc + `}}`
	rec, payload := env.callOp("writer", body)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Equal(t, CodeUpstream, codeOf(t, payload))
	assert.Contains(t, payload["message"], "超时")
}

// TestWorldValidate_CrashIsRedacted: the other direction of the redaction policy.
// An engine that crashes with garbage on stderr must produce a *summary* to the
// client — no paths, no stack excerpt — while the detail goes to the server log.
func TestWorldValidate_CrashIsRedacted(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("writer", common.RoleCommonUser)

	original := cfg.ValidatorSvcPath
	// A script that exists (so the preflight check passes) and then dies loudly.
	crasher := filepath.Join(t.TempDir(), "crasher.mjs")
	require.NoError(t, os.WriteFile(crasher, []byte(
		`process.stderr.write("INTERNAL_SECRET_PATH=/srv/secret/engine\n"); process.exit(9);`+"\n"), 0o644))
	cfg.ValidatorSvcPath = crasher
	t.Cleanup(func() { cfg.ValidatorSvcPath = original })

	body := `{"op":"world.validate","params":{"doc":` + emptyWorldDoc + `}}`
	rec, payload := env.callOp("writer", body)

	assert.Equal(t, http.StatusBadGateway, rec.Code)
	assert.Equal(t, CodeUpstream, codeOf(t, payload))
	message, _ := payload["message"].(string)
	assert.NotContains(t, message, "INTERNAL_SECRET_PATH",
		"a crash excerpt must not be forwarded to the client")
	assert.NotContains(t, message, "crasher.mjs",
		"the server-side path must not be forwarded for an unexpected crash")
}

// ---------------------------------------------------------------------------
// ingest.run — the only op that costs money
// ---------------------------------------------------------------------------
//
// ★ These tests never call a real model. They run the engine with its own
// MockProvider (`--features test-provider`), which produces REAL token counts —
// and that is what makes the billing assertions meaningful: the charge is computed
// from measured token counts through the host's own conversion, so the test
// verifies the *formula*, not merely "some number was subtracted".
//
// ⚠ What they therefore do NOT cover (recorded in docs/23 §12.4.4): a real gateway
// round trip, real model behaviour, and whether the operator has priced the model.
// Those need the end-to-end pass with the client.

// ingestMockScript drives the engine's MockProvider. The extraction stages parse
// each response as JSON, so `{}` is a valid "nothing found" answer for every stage.
var ingestMockScript = []string{"{}", "{}", "{}", "{}", "{}", "{}", "{}", "{}"}

// smallNovel is a two-chapter novel — enough for the engine to split, extract and
// summarise, small enough to be instant.
const smallNovel = `序

这是一个很短的开头，用来占位。

1. 第一章

林轩走进青云山门，抬头看见石阶千级。他握紧了手里的玉佩。

2. 第二章

玄尘子从后山竹林里走出来，手里提着青锋剑。剑身泛着青光。
`

// seedQuota gives a user a quota balance.
//
// ★ 它**不再**去配「模型倍率」表：世界解析的模型费由中继按它自己的价目表扣，
// 插件不参与算价（理由见 billing.go 文件头）。余额仍然是必需的——跑之前那道准入
// 门槛（`E_QUOTA`）看的就是它。
func seedQuota(t *testing.T, userID int, quota int) {
	t.Helper()

	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", userID).
		Update("quota", quota).Error)
}

func mustJSONString(t *testing.T, v any) string {
	t.Helper()
	out, err := json.Marshal(v)
	require.NoError(t, err)
	return string(out)
}

func quotaOf(t *testing.T, userID int) int {
	t.Helper()
	var user model.User
	require.NoError(t, model.DB.Where("id = ?", userID).First(&user).Error)
	return user.Quota
}

// TestIngestRun_ChargesOnlyTheGatewayNotThePlugin is the billing test.
//
// # 它守的是"一本账"（docs/23 §12.13 的落地）
//
// 模型调用打回本机中继、用的是发起者的令牌，中继已经把这一跑的钱扣在那个用户的
// 额度上了。所以这里的断言是**反方向**的：
//
//   - 钱包**一分钱都不许动**——插件不许自己再扣一次；
//   - 响应里**不许出现金额**：`billing.quota` 已删除。那个数既不是积分
//     （差一个 `QuotaPerUnit`），又是第二笔账——2026-10-08 实测中继 111,577
//     vs 插件 757,166，差 6.8 倍；
//   - token 与按阶段的用量**必须照旧报**（"钱花在哪了"仍然答得出来）。
//
// ⚠ 用 mock 引擎跑，所以这一条里**没有任何真实的模型调用**——这正是重点：
// 即使一笔都没被计费，插件也**不许**自己造一笔出来。反过来，若哪天有人把
// 扣费逻辑加回来，这条测试会立刻红。
func TestIngestRun_ChargesOnlyTheGatewayNotThePlugin(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireMockableEngine(t)
	writerID := env.idOf["writer"]
	env.mustGrant("admin", writerID, CapabilityWorldIPAI, 0)

	const startQuota = 10_000_000
	seedQuota(t, writerID, startQuota)

	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)

	body := fmt.Sprintf(
		`{"op":"ingest.run","params":{"project_id":%d,"base_version":%d,"source_text":%s,"media":false,"model":%q,"mock_script":%s}}`,
		projectID, version, mustJSONString(t, smallNovel), ingestDefaultModel,
		mustJSONString(t, ingestMockScript))
	rec, payload := env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, true, payload["success"], "body=%s", rec.Body.String())

	data := dataMap(t, payload)
	billing, ok := data["billing"].(map[string]any)
	require.True(t, ok, "billing missing: %v", data)

	// The engine really did make calls: a zero-token run would make this test vacuous.
	promptTokens := int(jsonInt(t, billing, "prompt_tokens"))
	completionTokens := int(jsonInt(t, billing, "completion_tokens"))
	require.Positive(t, promptTokens, "the engine must report real token usage: %v", billing)
	require.Positive(t, completionTokens, "the engine must report real token usage: %v", billing)

	// ★ 一：不许出现金额。
	_, hasQuota := billing["quota"]
	require.False(t, hasQuota,
		"billing 不许再带金额：那个数既不是积分、又是第二笔账（实测差 6.8 倍）：%v", billing)

	// ★ 二：钱包一分不动。
	require.Equal(t, startQuota, quotaOf(t, writerID),
		"模型费由中继那一笔承担；插件自己再扣一次就是同一次解析收两次钱")

	// ★ 三：用量照旧，按阶段的拆解也要到客户端（docs/23 §6.5）。
	stages, ok := billing["usage_by_stage"].([]any)
	require.True(t, ok, "usage_by_stage missing: %v", billing)
	assert.NotEmpty(t, stages, "the per-stage breakdown must reach the caller")
	assert.EqualValues(t, promptTokens+completionTokens > 0, true)
}

// TestIngestRun_RefusesWhenTheAccountCannotAffordIt: `E_QUOTA`, and nothing is spent.
//
// ★ `E_QUOTA` is not `E_ENTITLEMENT` (docs/23 §6.5). The account here HAS the
// capability — it simply has no credit. The two must stay distinguishable because
// the client draws different things for them ("去购买" vs "积分不足").
func TestIngestRun_RefusesWhenTheAccountCannotAffordIt(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t) // 这一条在引擎之前就返回了，不需要 mock
	writerID := env.idOf["writer"]
	env.mustGrant("admin", writerID, CapabilityWorldIPAI, 0)

	// Capability: owned. Credit: none.
	seedQuota(t, writerID, 0)
	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)

	body := fmt.Sprintf(
		`{"op":"ingest.run","params":{"project_id":%d,"base_version":%d,"source_text":%s,"max_chapters":2,"media":false,"model":%q}}`,
		projectID, version, mustJSONString(t, smallNovel), ingestDefaultModel)
	rec, payload := env.callOp("writer", body)

	assert.Equal(t, http.StatusPaymentRequired, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, CodeQuota, codeOf(t, payload))
	assert.NotContains(t, payload, "data", "a refused run must not hand back a document")
	message, _ := payload["message"].(string)
	// ★ 2026-10-08：金额一律走宿主自己的格式化器（`logger.LogQuota`），不再把裸
	// `quota` 写成"积分"——那个写法差一个 `QuotaPerUnit`（默认 500,000），
	// 用户手上那个数（2.23154 积分）与消息里那个数（757166）就是这么对不上的。
	assert.Contains(t, message, "余额不足")
	assert.NotContains(t, message, "积分", "裸 quota 不许再被写成\"积分\"")
	assert.Contains(t, message, "还差", "the client needs the shortfall to draw it")
	assert.Contains(t, message, "＄", "金额必须由宿主的格式化器给出（带单位）")

	// ★ Nothing was parsed and nothing was spent: the check runs BEFORE the engine.
	project, err := WorldProjectGet(projectID)
	require.NoError(t, err)
	assert.EqualValues(t, version, project.CurrentVersion, "a refused run must not add a version")
	assert.Equal(t, 0, quotaOf(t, writerID))
}

// TestIngestRun_UnpricedModelIsNoLongerThePluginsBusiness
//
// 这条测试以前守的是"运营没给这个型号配「模型倍率」就拒绝这次解析"。2026-10-08 之后
// 插件**不再自己算价**：模型调用打回本机中继、由中继按它自己的价目表逐次扣费
// （billing.go 文件头记着两笔账的实测差额）。于是"这个型号有没有倍率"不再是插件的
// 问题——中继会照它自己的规则处理，插件管不着也不该管。
//
// ★ 所以断言是**反方向**的：倍率表里没有的型号名照样跑完，而且那个名字必须原样回报
// （`billing.model`）——否则客户端无法解释"我指定了 A，为什么账单上写着 B"。
func TestIngestRun_UnpricedModelIsNoLongerThePluginsBusiness(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireMockableEngine(t)
	writerID := env.idOf["writer"]
	env.mustGrant("admin", writerID, CapabilityWorldIPAI, 0)
	seedQuota(t, writerID, 10_000_000)

	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)

	// A model name that is deliberately absent from the ratio table.
	const unpricedModel = "definitely-not-a-priced-model-xyz"
	body := fmt.Sprintf(
		`{"op":"ingest.run","params":{"project_id":%d,"base_version":%d,"source_text":%s,"max_chapters":1,"media":false,"model":%q,"mock_script":%s}}`,
		projectID, version, mustJSONString(t, smallNovel), unpricedModel,
		mustJSONString(t, ingestMockScript))
	rec, payload := env.callOp("writer", body)

	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	billing, ok := dataMap(t, payload)["billing"].(map[string]any)
	require.True(t, ok, "billing missing: %v", payload)
	assert.Equal(t, unpricedModel, billing["model"],
		"型号名必须原样回报：客户端要能对上自己指定了什么")
}

// TestIngestRun_NeedsTheAICapability: the plain capability must NOT open the paid op.
func TestIngestRun_NeedsTheAICapability(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("plain", common.RoleCommonUser)
	requireEngineSidecar(t)
	plainID := env.idOf["plain"]
	env.mustGrant("admin", plainID, CapabilityWorldIP, 0) // not world-ip-ai
	seedQuota(t, plainID, 10_000_000)

	projectID, version := env.mustSeedWorld("plain", emptyWorldDoc)
	body := fmt.Sprintf(
		`{"op":"ingest.run","params":{"project_id":%d,"base_version":%d,"source_text":%s}}`,
		projectID, version, mustJSONString(t, smallNovel))
	rec, payload := env.callOp("plain", body)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, CodeEntitlement, codeOf(t, payload))
	// The balance is untouched: the gate closes before anything is spent.
	require.Equal(t, 10_000_000, quotaOf(t, plainID))
}

// TestIngestRun_BaseVersionConflictDoesNotSpend: the concurrency check runs before
// the engine, so a stale client cannot trigger a paid run against a registry that
// has moved on (which would also drift every id).
func TestIngestRun_BaseVersionConflictDoesNotSpend(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t) // 这一条在引擎之前就返回了，不需要 mock
	writerID := env.idOf["writer"]
	env.mustGrant("admin", writerID, CapabilityWorldIPAI, 0)
	seedQuota(t, writerID, 10_000_000)

	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)
	// Make the server's version move on, so the client's base is stale.
	_, err := WorldSnapshotAppend(projectID, []byte(emptyWorldDoc), "test")
	require.NoError(t, err)

	body := fmt.Sprintf(
		`{"op":"ingest.run","params":{"project_id":%d,"base_version":%d,"source_text":%s,"mock_script":%s}}`,
		projectID, version, mustJSONString(t, smallNovel), mustJSONString(t, ingestMockScript))
	rec, payload := env.callOp("writer", body)

	assert.Equal(t, http.StatusConflict, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, CodeConflict, codeOf(t, payload))
	require.Equal(t, 10_000_000, quotaOf(t, writerID), "a refused run must not have spent anything")
}

// ---------------------------------------------------------------------------
// Validator-sidecar fixtures: finding it, and running it directly
// ---------------------------------------------------------------------------

// engineRepoEnvOverride points at the engine checkout explicitly, for a machine
// where it is not a sibling of the new-api checkout.
const engineRepoEnvOverride = "ZSY_WORLD_ENGINE_REPO"

// rawValidatorPath is set by requireValidatorSidecar when it locates the
// authoritative `validate.mjs`, so a test can then run it directly.
//
// It is package state rather than a per-test value because these tests run
// sequentially (none calls t.Parallel — model.DB is a process-wide global), and
// keeping it in one place beats threading a path through every helper.
var rawValidatorPath string

// validatorToolchain locates the validator shim and the raw validate.mjs.
//
// Resolution order:
//
//  1. explicit env overrides (ZSY_WORLD_VALIDATOR_SVC / ZSY_WORLD_VALIDATOR_RAW)
//  2. the engine repo as a *sibling* of wherever the new-api checkout lives —
//     both a sibling of the working directory and a sibling of this source file,
//     which are the two layouts a developer actually has
//  3. nothing → the caller skips
//
// ⚠ It deliberately does NOT walk up looking for `schema/1.1`: that path belongs
// to the *engine* repo, and new-api has an unrelated directory of that name, so a
// walk-up would eventually match the wrong tree. This was hit while wiring step 2
// — new-api/schema/1.1 exists and is not the validator.
func validatorToolchain(t *testing.T) (shim string, raw string) {
	t.Helper()

	if p := strings.TrimSpace(os.Getenv(envValidatorSvc)); p != "" {
		if abs, err := filepath.Abs(p); err == nil && fileExists(abs) {
			shim = abs
		}
	}
	if p := strings.TrimSpace(os.Getenv("ZSY_WORLD_VALIDATOR_RAW")); p != "" {
		if abs, err := filepath.Abs(p); err == nil && fileExists(abs) {
			raw = abs
		}
	}
	if shim != "" && raw != "" {
		return shim, raw
	}

	// Candidate roots, each checked for a sibling named after the engine repo.
	roots := make([]string, 0, 2)
	if wd, err := os.Getwd(); err == nil {
		roots = append(roots, wd)
	}
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		roots = append(roots, filepath.Dir(thisFile))
	}

	for _, root := range roots {
		for _, base := range engineRepoRoots(root) {
			schemaDir := filepath.Join(base, "schema", "1.1")
			if shim == "" && fileExists(filepath.Join(schemaDir, "world-validator-svc.mjs")) {
				shim = filepath.Join(schemaDir, "world-validator-svc.mjs")
			}
			if raw == "" && fileExists(filepath.Join(schemaDir, "validate.mjs")) {
				raw = filepath.Join(schemaDir, "validate.mjs")
			}
		}
		if shim != "" && raw != "" {
			break
		}
	}
	return shim, raw
}

// engineRepoRoots lists candidate paths of the engine checkout, given a directory
// to look around. An explicit override wins, then siblings under the usual
// workspace parent.
func engineRepoRoots(from string) []string {
	const repoName = "Matrix Tech World Parsing Format"

	out := make([]string, 0, 4)
	if p := strings.TrimSpace(os.Getenv(engineRepoEnvOverride)); p != "" {
		out = append(out, filepath.Clean(p))
	}
	// The engine repo sits beside the new-api checkout's parent, e.g.
	// D:\work\new-api → D:\work\Matrix Tech World Parsing Format.
	dir := from
	for i := 0; i < 4; i++ {
		out = append(out, filepath.Join(dir, repoName))
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return out
}

// requireValidatorSidecar wires the sidecar up for a test, or skips it with a
// message that says exactly how to make it run.
//
// ⚠ It skips rather than fails so the package stays testable in an environment
// without the engine repo. The price of that choice is paid here, once: the skip
// message is explicit, and TestWorldValidate_UnconfiguredValidatorIsE_UPSTREAM
// runs unconditionally — so "the sidecar is wired" can never be quietly untested
// while looking green.
func requireValidatorSidecar(t *testing.T) (rawValidator string) {
	t.Helper()

	shim, raw := validatorToolchain(t)
	if shim == "" || raw == "" {
		t.Skipf(
			"权威校验器不可达：找不到 %s / validate.mjs。"+
				"把 %s 指向 world-validator-svc.mjs（位于引擎仓库 schema/1.1/）后重跑。",
			ValidatorSvcPathDefault, envValidatorSvc)
	}
	if _, err := exec.LookPath(cfg.NodeBinary); err != nil {
		t.Skipf("找不到 node（%s）：请安装 node 或设置 %s。err=%v",
			cfg.NodeBinary, envNodeBinary, err)
	}

	original := cfg.ValidatorSvcPath
	cfg.ValidatorSvcPath = shim
	rawValidatorPath = raw
	t.Cleanup(func() {
		cfg.ValidatorSvcPath = original
		rawValidatorPath = ""
	})
	return raw
}

// runValidatorDirectly runs validate.mjs itself (no shim, no Go) and returns the
// verdict, so a test can compare the op's answer against the authority's.
//
// ★ It goes through the same public surface as the shim does — the `--json` CLI,
// one temp file — because that is what "与本地跑 validate.mjs 逐字一致" means. It
// does NOT import validate.mjs: an import would need its Ajv wiring re-created
// here, i.e. a second implementation, which is the thing under test.
func runValidatorDirectly(t *testing.T, doc string) validatorReportEntry {
	t.Helper()

	requireValidatorSidecar(t)
	raw := rawValidatorPath
	require.NotEmpty(t, raw, "requireValidatorSidecar must resolve the raw validator path")

	tmp := filepath.Join(t.TempDir(), "doc.mtw")
	// ⚠ No BOM: `validate.mjs` does JSON.parse on the raw text, and a leading
	// U+FEFF makes it report a parse failure as a *shape* error. (Found the hard
	// way while probing the validator's contract.)
	require.NoError(t, os.WriteFile(tmp, []byte(doc), 0o644))

	cmd := exec.Command(cfg.NodeBinary, raw, "--json", tmp)
	out, err := cmd.Output()
	// validate.mjs exits 1 when the document has errors, which is a legitimate
	// verdict, not a failure of this call.
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("running validate.mjs directly failed: %v", err)
	}
	if exitErr != nil && exitErr.ExitCode() != 1 {
		t.Fatalf("validate.mjs exited %d; stderr=%s", exitErr.ExitCode(), exitErr.Stderr)
	}

	var parsed validatorReport
	require.NoError(t, json.Unmarshal(out, &parsed), "validator output: %s", out)
	require.Len(t, parsed.Report, 1, "validator output: %s", out)
	return parsed.Report[0]
}

// issuesOf reads an issues array out of a response's data object, in the exact
// {code,msg} shape the validator emits.
func issuesOf(t *testing.T, data map[string]any, key string) []worldIssue {
	t.Helper()
	raw, ok := data[key].([]any)
	require.True(t, ok, "%s missing or not an array in %v", key, data)
	out := make([]worldIssue, 0, len(raw))
	for _, item := range raw {
		entry, ok := item.(map[string]any)
		require.True(t, ok, "%s entry is not an object: %v", key, item)
		code, _ := entry["code"].(string)
		msg, _ := entry["msg"].(string)
		require.NotEmpty(t, code, "%s entry has no code: %v", key, item)
		require.NotEmpty(t, msg, "%s entry has no msg: %v", key, item)
		out = append(out, worldIssue{Code: code, Msg: msg})
	}
	return out
}

// fileExists is a tiny helper kept local so the harness does not pull in an
// os.Stat wrapper from elsewhere.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// TestProjectList_AnswersOnlyTheCallersOwnWorlds: the user face must not be a
// cross-tenant read.
//
// ★ This is the op that fixed a real bad behaviour: before it, the desktop client
// had no way to ask "do I have a world?", so it created one every time — the user's
// world list grew by one empty world per cold start (docs/23 §12.4.6).
func TestProjectList_AnswersOnlyTheCallersOwnWorlds(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("mine", common.RoleCommonUser)
	env.seedAccount("theirs", common.RoleCommonUser)

	mineID, _ := env.mustSeedWorld("mine", emptyWorldDoc)
	_, _ = env.mustSeedWorld("theirs", emptyWorldDoc)

	// Login-only: no capability granted in this test, none needed.
	require.Empty(t, env.capabilitiesOf("mine"))

	rec, payload := env.callOp("mine", `{"op":"project.list","params":{}}`)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, CodeOK, codeOf(t, payload))

	data := dataMap(t, payload)
	items, ok := data["items"].([]any)
	require.True(t, ok, "items missing: %v", data)
	require.Len(t, items, 1, "only the caller's own world may appear: %v", items)

	first, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, mineID, jsonInt(t, first, "id"))
	// The listing must stay cheap: no document travels in it (docs/23 §4.2 —
	// the same reason history listings carry no doc).
	assert.NotContains(t, first, "doc")
	assert.EqualValues(t, 1, jsonInt(t, data, "total"))

	// The other account sees only its own as well — the point is that neither
	// answer contains the other's id.
	_, otherPayload := env.callOp("theirs", `{"op":"project.list","params":{}}`)
	otherItems, ok := dataMap(t, otherPayload)["items"].([]any)
	require.True(t, ok)
	require.Len(t, otherItems, 1)
	otherFirst, ok := otherItems[0].(map[string]any)
	require.True(t, ok)
	assert.NotEqual(t, mineID, uint(jsonInt(t, otherFirst, "id")), "a cross-tenant read would look like this")
}

// TestProjectList_EmptyWhenTheAccountHasNoWorld: the empty answer is what lets the
// client stop creating a world on every cold start.
func TestProjectList_EmptyWhenTheAccountHasNoWorld(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("fresh", common.RoleCommonUser)

	rec, payload := env.callOp("fresh", `{"op":"project.list","params":{}}`)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	data := dataMap(t, payload)
	items, ok := data["items"].([]any)
	require.True(t, ok, "items must be an array (never null) so a client can iterate: %v", data)
	assert.Empty(t, items)
	assert.EqualValues(t, 0, jsonInt(t, data, "total"))
}

// ---------------------------------------------------------------------------
// project.create / world.mutate — step 3, with the validity gate
// ---------------------------------------------------------------------------

// TestProjectCreate_MakesAnEngineBuiltEmptyWorld: the empty world must come from
// the engine, not from a Go literal.
//
// ★ The assertion that matters is the last one: the authority accepts it. That is
// what a Go-side literal could not guarantee — my own hand-written fixture failed
// the schema in three places (docs/23 §12.7 deviation 10), and the engine's own
// `new_document` is the only thing that stays in step with the format.
func TestProjectCreate_MakesAnEngineBuiltEmptyWorld(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	writer := env.idOf["writer"]
	env.mustGrant("admin", writer, CapabilityWorldIP, 0)

	body := `{"op":"project.create","params":{"name":"我的世界","novel_title":"验收用小说"}}`
	rec, payload := env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, true, payload["success"], "body=%s", rec.Body.String())
	assert.Equal(t, CodeOK, codeOf(t, payload))

	data := dataMap(t, payload)
	project, ok := data["project"].(map[string]any)
	require.True(t, ok, "project missing: %v", data)
	assert.Equal(t, "我的世界", project["name"])
	assert.Equal(t, "验收用小说", project["novel_title"])
	require.EqualValues(t, 1, jsonInt(t, data, "version"), "the first snapshot is version 1")

	doc, ok := data["doc"].(map[string]any)
	require.True(t, ok, "doc missing or not an object: %v", data)
	assert.Equal(t, "MTW", doc["format"])
	// ★ The empty world carries its top-level sections as EMPTY ARRAYS, not as
	// absent keys. Not incidental: the schema requires all eleven, so a "minimal"
	// document built by omitting the empty ones is invalid — which is precisely the
	// mistake a hand-written literal makes.
	for _, section := range []string{
		"element_types", "elements", "relations", "chapters",
		"events", "prompts", "sounds", "consistency_anchors",
	} {
		value, present := doc[section]
		require.True(t, present, "the engine's empty world must declare %q", section)
		list, isList := value.([]any)
		require.True(t, isList, "%q must be an array, got %T", section, value)
		assert.Empty(t, list, "%q must be empty in a new world", section)
	}

	// ★ the authority's verdict on the engine's output
	want := runValidatorDirectly(t, mustMarshal(t, doc))
	assert.True(t, want.ShapeOK, "engine-built empty world must be schema-valid: %v", want.ShapeErrors)
	assert.Empty(t, want.Errors, "engine-built empty world must have no 18.3 errors")

	// It is readable back through world.get, unchanged.
	projectID := uint(jsonInt(t, project, "id"))
	rec, payload = env.callOp("writer", fmt.Sprintf(
		`{"op":"world.get","params":{"project_id":%d}}`, projectID))
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	assert.Contains(t, dataMap(t, payload), "doc")
}

// TestProjectCreate_EmptyTitleIsRefusedByTheAuthority is the design claim of
// op_project.go made testable: **Go does not check the title**, the authority does.
//
// The schema requires `novel_title` to have `minLength: 1`, so an empty title
// yields a document the schema rejects. A design that wrote that check in Go would
// be a second definition of a legal document; this one asks and reports.
func TestProjectCreate_EmptyTitleIsRefusedByTheAuthority(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	body := `{"op":"project.create","params":{"name":"没标题的世界"}}`
	rec, payload := env.callOp("writer", body)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, CodeInput, codeOf(t, payload))
	assert.NotContains(t, payload, "data", "a refused create must not hand back a document")

	// And nothing was written: the account owns no project.
	rows, total, err := WorldProjectListByUser(env.idOf["writer"], 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 0, total, "a refused create must not leave a row: %v", rows)
}

// TestProjectCreate_NeedsTheCapability: project.create is capability-gated.
func TestProjectCreate_NeedsTheCapability(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("poor", common.RoleCommonUser)

	rec, payload := env.callOp("poor", `{"op":"project.create","params":{"name":"x"}}`)
	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, CodeEntitlement, codeOf(t, payload))
	assert.NotContains(t, payload, "data")
}

// TestWorldMutate_RenameCommitsANewVersion is the happy path: the engine changes
// the document, the authority accepts it, and a NEW version is stored.
func TestWorldMutate_RenameCommitsANewVersion(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	writer := env.idOf["writer"]
	env.mustGrant("admin", writer, CapabilityWorldIP, 0)

	projectID, version := env.mustSeedWorld("writer", mustRealWorldDoc(t))

	body := fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_001","new_name":"改名后的云霄"}}`,
		projectID, version)
	rec, payload := env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, true, payload["success"], "body=%s", rec.Body.String())

	data := dataMap(t, payload)
	assert.Equal(t, "rename", data["kind"])
	assert.EqualValues(t, version+1, jsonInt(t, data, "version"), "a mutation writes a NEW version")

	doc, ok := data["doc"].(map[string]any)
	require.True(t, ok, "doc missing: %v", data)
	elements, ok := doc["elements"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, elements)

	// The engine's demo world has 12 elements; the one that must have changed is
	// char_001. Searching by id rather than by index is deliberate — an assertion
	// on elements[0] would be asserting the engine's element ORDER, which this op
	// makes no promise about.
	var renamed map[string]any
	for _, raw := range elements {
		entry, isObject := raw.(map[string]any)
		require.True(t, isObject)
		if entry["id"] == "char_001" {
			renamed = entry
		}
	}
	require.NotNil(t, renamed, "char_001 must still exist — a rename does not change the id")
	assert.Equal(t, "改名后的云霄", renamed["name"], "the rename reached the document")

	// The engine's own outcome travels verbatim.
	result, ok := data["result"].(map[string]any)
	require.True(t, ok, "result missing: %v", data)
	assert.Equal(t, "char_001", result["id"], "a rename does not change the id")
	assert.Equal(t, "改名后的云霄", result["new_name"])
	assert.Equal(t, "林轩", result["old_name"])
	// ★ The old name is REGISTERED AS AN ALIAS — that is what makes incremental
	// parsing able to recognise the old spelling later (规范 7.5.2). Asserting the
	// opposite was my first version's mistake: renaming is not "forget the old name".
	assert.Equal(t, true, result["alias_added"], "the previous name must be kept as an alias")

	// The new version is readable, and the old one is still there (§6.3: history
	// is kept, a mutation never deletes it).
	rec, payload = env.callOp("writer", fmt.Sprintf(
		`{"op":"world.get","params":{"project_id":%d,"since_version":%d}}`, projectID, version))
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, true, dataMap(t, payload)["changed"])

	history, err := WorldSnapshotHistory(projectID)
	require.NoError(t, err)
	require.Len(t, history, 2, "both versions are kept")
	assert.Equal(t, "rename", history[0].Reason)
	assert.Equal(t, "seed", history[1].Reason)
}

// TestWorldMutate_BaseVersionConflictIsE_CONFLICT: docs/23 §4.4 — without this the
// loss is silent.
func TestWorldMutate_BaseVersionConflictIsE_CONFLICT(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	projectID, version := env.mustSeedWorld("writer", mustRealWorldDoc(t))
	require.EqualValues(t, 1, version, "the seeded project starts at v1")

	// ★ A real conflict needs two versions to exist, so a first mutation is allowed
	// to land — otherwise the only "stale" number available would be 0, which
	// `validateWorldMutate` rejects as a MISSING base_version (E_INPUT), not as a
	// conflict. Getting this wrong once is why the two cases are spelled out
	// separately below.
	first := fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_001","new_name":"第一次改名"}}`,
		projectID, version)
	rec, payload := env.callOp("writer", first)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.EqualValues(t, 2, jsonInt(t, dataMap(t, payload), "version"))

	// ── case 1: a stale base_version is E_CONFLICT (docs/23 §4.4) ───────────
	rec, payload = env.callOp("writer", first) // still based on v1, server is at v2
	assert.Equal(t, http.StatusConflict, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, CodeConflict, codeOf(t, payload))
	assert.Contains(t, payload["message"], "版本不一致")
	// The message must carry both numbers: the client has to know what to refetch.
	assert.Contains(t, payload["message"], "1")
	assert.Contains(t, payload["message"], "2")

	// ── case 2: no base_version at all is E_INPUT (a client bug, not a race) ─
	rec, payload = env.callOp("writer", fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"kind":"rename","element_id":"char_001","new_name":"无版本号"}}`, projectID))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, CodeInput, codeOf(t, payload))
	assert.Contains(t, payload["message"], "base_version")

	// ── nothing was written by either attempt ──────────────────────────────
	after, err := WorldProjectGet(projectID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, after.CurrentVersion, "only the first, well-formed mutation committed")

	history, err := WorldSnapshotHistory(projectID)
	require.NoError(t, err)
	assert.Len(t, history, 2, "the refused attempts must not have left snapshots: %v", history)
}

// TestWorldMutate_KindValidation: the per-kind required fields, before any engine
// call. These are the request's own shape rules, not world rules.
func TestWorldMutate_KindValidation(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)
	projectID, version := env.mustSeedWorld("writer", mustRealWorldDoc(t))

	for _, tc := range []struct{ name, body string }{
		{"unknown kind", fmt.Sprintf(`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"explode"}}`, projectID, version)},
		{"rename without element_id", fmt.Sprintf(`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","new_name":"x"}}`, projectID, version)},
		{"rename without new_name", fmt.Sprintf(`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_001"}}`, projectID, version)},
		{"retype without to_type", fmt.Sprintf(`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"retype","element_id":"char_001"}}`, projectID, version)},
		{"merge with one id", fmt.Sprintf(`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"merge","id_a":"char_001"}}`, projectID, version)},
		{"merge with the same id twice", fmt.Sprintf(`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"merge","id_a":"char_001","id_b":"char_001"}}`, projectID, version)},
		{"split without new_name", fmt.Sprintf(`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"split","id":"char_001"}}`, projectID, version)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec, payload := env.callOp("writer", tc.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
			assert.Equal(t, CodeInput, codeOf(t, payload))
		})
	}

	// The engine's own refusal (an id that does not exist) also surfaces as
	// E_INPUT — the request cannot be honoured, and retrying will not help.
	rec, payload := env.callOp("writer", fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_999","new_name":"x"}}`,
		projectID, version))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, CodeInput, codeOf(t, payload))
	assert.Contains(t, payload["message"], "char_999", "the engine's own sentence must reach the client")
}

// TestWorldMutate_GateBlocksAnInvalidDocument is the decision made in docs/23 §10
// item 10 (option A), pinned as a test.
//
// ★ The scenario is real, not synthetic: re-typing an element changes its type in
// the registry but NOT its `metadata`, so the new type's `field_template` no longer
// matches and its required fields are missing. The authority reports E14/E15;
// the engine's own structural check says the document is clean.
//
// What must happen: the op refuses, **nothing is written**, and the client is told
// what the authority objected to — in the authority's words.
func TestWorldMutate_GateBlocksAnInvalidDocument(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	// ★ The real document, re-typed into one of ITS OWN other types. That is the
	// whole reason this scenario is faithful: the engine's demo world already
	// declares `type_character` and `type_scene` with different `field_template`s,
	// and `char_001` carries character-only `metadata`. Re-typing it to `type_scene`
	// therefore produces exactly the E14/E15 pair that motivated the gate — no
	// hand-built fixture required, and nothing synthetic about it.
	doc := mustRealWorldDoc(t)
	projectID, version := env.mustSeedWorld("writer", doc)

	body := fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"retype","element_id":"char_001","to_type":"type_scene","reason":"闸门测试"}}`,
		projectID, version)
	rec, payload := env.callOp("writer", body)

	// ── refused ────────────────────────────────────────────────────────────
	assert.Equal(t, http.StatusBadRequest, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, CodeInput, codeOf(t, payload))
	assert.NotContains(t, payload, "data", "a gated mutation must not hand back a document")

	message, _ := payload["message"].(string)
	assert.Contains(t, message, "会让世界不合法", "the client must be told the change was refused")
	assert.Contains(t, message, "E14", "the authority's own codes must be quoted")

	// ── and NOTHING was written ────────────────────────────────────────────
	project, err := WorldProjectGet(projectID)
	require.NoError(t, err)
	assert.EqualValues(t, version, project.CurrentVersion, "the current version must not advance")

	history, err := WorldSnapshotHistory(projectID)
	require.NoError(t, err)
	assert.Len(t, history, 1, "no new snapshot: %v", history)

	stored, err := WorldSnapshotDocRaw(projectID, version)
	require.NoError(t, err)
	assert.Equal(t, canonicalJSON(t, []byte(doc)), canonicalJSON(t, stored),
		"the stored document must be byte-for-byte the one that was there before")
}

// TestWorldMutate_GateLetsWarningsThrough: the gate blocks the ERROR layer only.
// A mutation whose result has warnings but no errors must still commit — otherwise
// every edit would be blocked by quality advice, which is not what warnings mean
// (docs/23 §4.2: "12 项错误 + 9 项警告").
func TestWorldMutate_GateLetsWarningsThrough(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	// A rename on the real document: the authority's verdict carries warnings (or
	// none) but never errors, and the point is that only the error layer blocks
	// (docs/23 §4.2: "12 项错误 + 9 项警告").
	projectID, version := env.mustSeedWorld("writer", mustRealWorldDoc(t))

	body := fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_001","new_name":"仍可提交"}}`,
		projectID, version)
	rec, payload := env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "warnings must not block: %s", rec.Body.String())

	data := dataMap(t, payload)
	warnings, ok := data["warnings"].([]any)
	require.True(t, ok, "warnings missing: %v", data)
	// ⚠ Asserted as "the field is present as a list" rather than "non-empty": the
	// real document is clean, so the honest expectation is zero warnings — and the
	// thing under test is that the op COMMITTED, not how many warnings it had.
	assert.NotNil(t, warnings)
	assert.EqualValues(t, version+1, jsonInt(t, data, "version"), "the change committed")
}

// mustMarshal re-encodes a decoded JSON value for a direct comparison.
func mustMarshal(t *testing.T, value any) string {
	t.Helper()
	out, err := json.Marshal(value)
	require.NoError(t, err)
	return string(out)
}

// twoTypeDoc is a small world with two element types, so a re-type has a target.
//
// It is deliberately schema-valid as written, so a test failure can only come from
// the mutation, not from a malformed fixture.
func twoTypeDoc() string {
	return `{
  "format": "MTW",
  "version": "1.1",
  "meta": {
    "novel_title": "两种类型的世界",
    "chapter_count": 0,
    "parsed_at": "2026-09-19T10:00:00Z",
    "parser_name": "矩阵科技世界解析器",
    "parser_version": "0.1.0",
    "mtw_version": "1.1"
  },
  "element_types": [
    { "id": "type_character", "name": "角色", "name_en": "character", "source": "base", "enabled": true, "field_template": [] },
    { "id": "type_scene", "name": "场景", "name_en": "scene", "source": "base", "enabled": true,
      "field_template": [
        { "key": "place", "type": "string", "required": true },
        { "key": "scene_kind", "type": "string", "required": true }
      ] }
  ],
  "elements": [
    { "id": "char_001", "type_id": "type_character", "name": "许云霄", "metadata": {}, "relations": [], "prompts": [], "sounds": [] }
  ],
  "relations": [],
  "chapters": [],
  "events": [],
  "prompts": [],
  "sounds": [],
  "consistency_anchors": []
}`
}

// ---------------------------------------------------------------------------
// Criterion C — grant / revoke take effect on the next op
// ---------------------------------------------------------------------------

// TestRevoke_TakesEffectOnTheVeryNextOp is enforcement criterion C
// (docs/23 §9 step 7: "撤销之后立刻不能").
//
// ★ The op that must flip is a capability-gated one. docs/23 §4.2 makes
// `world.get` login-only precisely so a paying user keeps access to their own
// world; a test that asserted world.get stops working after a revoke would be
// asserting the opposite of the contract. So the test drives both:
//
//	world.mutate (needs world-ip)  → granted: past the gate; revoked: E_ENTITLEMENT
//	world.get    (login-only)      → still readable after the revoke, because
//	                                 the revoke takes away a capability to act,
//	                                 not the user's own data
//
// The second half is the more interesting assertion: it is what stops someone
// from "fixing" the gate by denying everything.
func TestRevoke_TakesEffectOnTheVeryNextOp(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	writer := env.idOf["writer"]

	projectID, version := env.mustSeedWorld("writer", mustRealWorldDoc(t))
	gated := func() string {
		return fmt.Sprintf(
			`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_001","new_name":"撤销测试改名"}}`,
			projectID, version)
	}
	loginOnly := fmt.Sprintf(`{"op":"world.get","params":{"project_id":%d}}`, projectID)

	// ── before the grant: refused, and the refusal is the gate's ────────────
	rec, payload := env.callOp("writer", gated())
	require.Equal(t, http.StatusForbidden, rec.Code)
	require.Equal(t, CodeEntitlement, codeOf(t, payload))

	// ── grant through the admin face: the next op really does the work ──────
	env.mustGrant("admin", writer, CapabilityWorldIP, 0)
	rec, payload = env.callOp("writer", gated())
	require.Equal(t, http.StatusOK, rec.Code,
		"grant must take effect immediately, and the engine is wired now: %s", rec.Body.String())
	require.Equal(t, CodeOK, codeOf(t, payload))
	// ★ `base_version` on the second call must be the version the grant-produced
	// mutation wrote, which is checked implicitly: a stale base_version would have
	// been E_CONFLICT, not 200.

	// ── revoke: the very next op is refused again ──────────────────────────
	env.mustRevoke("admin", writer, CapabilityWorldIP)
	current, err := WorldProjectGet(projectID)
	require.NoError(t, err)
	revoked := fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_002","new_name":"不该生效"}}`,
		projectID, current.CurrentVersion)
	rec, payload = env.callOp("writer", revoked)
	assert.Equal(t, http.StatusForbidden, rec.Code, "revocation must be immediate")
	assert.Equal(t, CodeEntitlement, codeOf(t, payload))
	assert.NotContains(t, payload, "data")
	assert.NotContains(t, rec.Body.String(), "林轩")

	// ★ The gate runs BEFORE the engine, so a refused mutation left no trace at
	// all — not even a new snapshot.
	after, err := WorldProjectGet(projectID)
	require.NoError(t, err)
	assert.EqualValues(t, current.CurrentVersion, after.CurrentVersion,
		"a refused (unauthorised) mutation must not have advanced the version")

	// ── and the user's own world is still theirs ──────────────────────────
	rec, payload = env.callOp("writer", loginOnly)
	assert.Equal(t, http.StatusOK, rec.Code,
		"docs/23 §4.2: world.get is login-only, so revoking a capability must not lock a user out of their own world")
	assert.Equal(t, CodeOK, codeOf(t, payload))
	assert.Contains(t, dataMap(t, payload), "doc")

	// ── regrant: reversible ────────────────────────────────────────────────
	env.mustGrant("admin", writer, CapabilityWorldIP, 0)
	rec, payload = env.callOp("writer", revoked)
	assert.Equal(t, http.StatusOK, rec.Code, "regrant must take effect immediately: %s", rec.Body.String())
	assert.Equal(t, CodeOK, codeOf(t, payload))

	rows, err := WorldEntitlementListByUser(writer)
	require.NoError(t, err)
	require.Len(t, rows, 2, "revoking stamps revoked_at instead of deleting the row")
	// Newest first (id desc): the regranted row is live, the first grant is the
	// one that was revoked.
	assert.Nil(t, rows[0].RevokedAt, "the regrant is live")
	assert.NotNil(t, rows[1].RevokedAt, "the revoked grant kept its revocation record")
}

func TestRevoke_NothingLiveIsReportedNotSilentlySucceeded(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	body := fmt.Sprintf(`{"user_id":%d,"capability":%q}`, env.idOf["writer"], CapabilityWorldIP)
	rec, payload := env.postJSON("/dashboard/zsy/world/entitlements/revoke", body, env.ensureAdminAccessToken())
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, false, payload["success"], "revoking a capability nobody holds is an operator mistake")
	assert.Contains(t, payload["message"], CapabilityWorldIP)
}

// TestExpiredEntitlement_IsRefusedAndFutureOneIsNot pins the expiry half of the
// verdict: an expired capability is not a capability. Like the revoke test it
// drives a capability-gated op, because a login-only op would succeed either way.
func TestExpiredEntitlement_IsRefusedAndFutureOneIsNot(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireEngineSidecar(t)
	writer := env.idOf["writer"]

	projectID, version := env.mustSeedWorld("writer", mustRealWorldDoc(t))
	body := fmt.Sprintf(
		`{"op":"world.mutate","params":{"project_id":%d,"base_version":%d,"kind":"rename","element_id":"char_001","new_name":"过期测试改名"}}`,
		projectID, version)

	// Expired an hour ago.
	env.mustGrant("admin", writer, CapabilityWorldIP, common.GetTimestamp()-3600)
	rec, payload := env.callOp("writer", body)
	assert.Equal(t, http.StatusForbidden, rec.Code, "an expired capability is not a capability")
	assert.Equal(t, CodeEntitlement, codeOf(t, payload))
	assert.NotContains(t, payload, "data")

	// The expired attempt wrote nothing.
	after, err := WorldProjectGet(projectID)
	require.NoError(t, err)
	assert.EqualValues(t, version, after.CurrentVersion)

	// A live grant alongside the expired one wins, because every read is an
	// existence check over live rows.
	env.mustGrant("admin", writer, CapabilityWorldIP, common.GetTimestamp()+3600)
	rec, payload = env.callOp("writer", body)
	assert.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	assert.Equal(t, CodeOK, codeOf(t, payload))
	assert.EqualValues(t, version+1, jsonInt(t, dataMap(t, payload), "version"))
}

func TestGrant_RejectsAnUnknownCapability(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	body := fmt.Sprintf(`{"user_id":%d,"capability":"world-ip-everything"}`, env.idOf["writer"])
	rec, payload := env.postJSON("/dashboard/zsy/world/entitlements/grant", body, env.ensureAdminAccessToken())
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, payload["message"], CapabilityWorldIPAI,
		"the message must list the vocabulary the plugin accepts")

	rows, err := WorldEntitlementListByUser(env.idOf["writer"])
	require.NoError(t, err)
	assert.Empty(t, rows, "a rejected grant must not leave an inert row behind")
}

// capabilitiesOf reads the current capabilities through the real store call.
func (e *testEnv) capabilitiesOf(username string) []string {
	e.t.Helper()
	caps, err := WorldCapabilitiesActive(e.idOf[username])
	require.NoError(e.t, err)
	return caps
}

// ---------------------------------------------------------------------------
// The UX face and the admin read face
// ---------------------------------------------------------------------------

func TestEntitlementsEndpoint_ReportsTheLiveVerdict(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	writer := env.idOf["writer"]

	rec, payload := env.getJSON("/api/zsy/world/entitlements", env.keyOf["writer"])
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := dataMap(t, payload)
	assert.EqualValues(t, writer, jsonInt(t, data, "user_id"))
	assert.Empty(t, data["capabilities"])
	assert.Contains(t, data["known"], CapabilityWorldIP)

	// Revoked but not deleted: the live list must shrink even though rows remain.
	env.mustGrant("admin", writer, CapabilityWorldIP, 0)
	_, payload = env.getJSON("/api/zsy/world/entitlements", env.keyOf["writer"])
	assert.Contains(t, dataMap(t, payload)["capabilities"], CapabilityWorldIP)

	env.mustRevoke("admin", writer, CapabilityWorldIP)
	_, payload = env.getJSON("/api/zsy/world/entitlements", env.keyOf["writer"])
	assert.Empty(t, dataMap(t, payload)["capabilities"])

	rows, err := WorldEntitlementListByUser(writer)
	require.NoError(t, err)
	assert.Len(t, rows, 1, "the revoked row is still readable through the admin face")
}

func TestAdminProjectDetail_ShowsHistoryWithoutShippingDocuments(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	projectID, _ := env.mustSeedWorld("writer", emptyWorldDoc)

	rec, payload := env.getJSON(fmt.Sprintf("/dashboard/zsy/world/projects/%d", projectID), env.ensureAdminAccessToken())
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	data := dataMap(t, payload)

	project, ok := data["project"].(map[string]any)
	require.True(t, ok, "project missing: %v", data)
	assert.EqualValues(t, projectID, jsonInt(t, project, "id"))
	assert.EqualValues(t, 1, jsonInt(t, project, "currentVersion"))

	history, ok := data["history"].([]any)
	require.True(t, ok, "history missing: %v", data)
	require.Len(t, history, 1)
	entry, ok := history[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, 1, jsonInt(t, entry, "version"))
	assert.Equal(t, "seed", entry["reason"])
	assert.NotContains(t, entry, "doc", "a history listing must stay cheap")
	assert.NotContains(t, rec.Body.String(), "许云霄", "the listing must not carry the document")

	// One version, read verbatim, is the operator's evidence.
	rec, payload = env.getJSON(
		fmt.Sprintf("/dashboard/zsy/world/projects/%d/versions/1", projectID), env.ensureAdminAccessToken())
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	versionData := dataMap(t, payload)
	stored, err := WorldSnapshotDocRaw(projectID, 1)
	require.NoError(t, err)
	mustSameJSONBytes(t, stored, []byte(versionData["doc"].(string)), "admin version doc")
}

// ---------------------------------------------------------------------------
// Store invariants
// ---------------------------------------------------------------------------

func TestStore_VersionsAdvanceAndHistoryKeepsEveryVersion(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("writer", common.RoleCommonUser)
	userID := env.idOf["writer"]

	project, err := WorldProjectCreate(userID, "递增", "")
	require.NoError(t, err)
	assert.EqualValues(t, 0, project.CurrentVersion, "a new project has no version yet")

	first, err := WorldSnapshotAppend(project.ID, []byte(emptyWorldDoc), "seed")
	require.NoError(t, err)
	assert.EqualValues(t, 1, first.Version)
	require.NoError(t, WorldProjectSetCurrentVersion(project.ID, first.Version))

	second, err := WorldSnapshotAppend(project.ID, []byte(`{"format":"MTW","version":"1.1"}`), "retype")
	require.NoError(t, err)
	assert.EqualValues(t, 2, second.Version)

	reloaded, err := WorldProjectGet(project.ID)
	require.NoError(t, err)
	assert.EqualValues(t, 2, reloaded.CurrentVersion, "the pointer follows the newest snapshot")

	history, err := WorldSnapshotHistory(project.ID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	assert.EqualValues(t, 2, history[0].Version, "newest first")
	assert.Equal(t, "retype", history[0].Reason)
	assert.EqualValues(t, 1, history[1].Version)

	// A document that is not JSON must be refused: the row would be unserializable
	// into the response envelope later, and Go owns that transport concern.
	_, err = WorldSnapshotAppend(project.ID, []byte("这不是 JSON"), "seed")
	require.Error(t, err)
}

func TestStore_OwnerScopingAndProjectList(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("a", common.RoleCommonUser)
	env.seedAccount("b", common.RoleCommonUser)

	projectA, err := WorldProjectCreate(env.idOf["a"], "A 的世界", "")
	require.NoError(t, err)
	_, err = WorldProjectCreate(env.idOf["b"], "B 的世界", "")
	require.NoError(t, err)

	_, err = WorldProjectGetOwned(projectA.ID, env.idOf["b"])
	assert.ErrorIs(t, err, ErrProjectNotFound, "a foreign project reads as absent")

	owned, err := WorldProjectGetOwned(projectA.ID, env.idOf["a"])
	require.NoError(t, err)
	assert.Equal(t, "A 的世界", owned.Name)

	rows, total, err := WorldProjectListByUser(env.idOf["a"], 0, 10)
	require.NoError(t, err)
	assert.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	assert.Equal(t, projectA.ID, rows[0].ID)

	// Reading a project that exists but has no document is an empty answer, not
	// an error — project.create lands with an empty world in step 3.
	_, err = WorldSnapshotCurrent(projectA.ID)
	assert.ErrorIs(t, err, ErrSnapshotNotFound)
}

func TestWorldEntitlementActive_IgnoresOtherUsersAndOtherCapabilities(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	env.seedAccount("other", common.RoleCommonUser)

	env.mustGrant("admin", env.idOf["writer"], CapabilityWorldIP, 0)

	active, err := WorldEntitlementActive(env.idOf["writer"], CapabilityWorldIP)
	require.NoError(t, err)
	assert.True(t, active)

	active, err = WorldEntitlementActive(env.idOf["other"], CapabilityWorldIP)
	require.NoError(t, err)
	assert.False(t, active, "a capability is per account")

	active, err = WorldEntitlementActive(env.idOf["writer"], CapabilityWorldIPAI)
	require.NoError(t, err)
	assert.False(t, active, "a capability is per capability")
}

// ---------------------------------------------------------------------------
// The acceptance test that uses a document the ENGINE produced
// ---------------------------------------------------------------------------
//
// ⚠ Why this exists next to the hand-written fixtures: step 2 proved hand-written
// MTW is easy to get wrong — the first fixture failed three schema layers. A
// verdict comparison against a fixture I authored only proves the two
// implementations agree about *that* fixture. So this test runs a real .mtw
// produced by 工具1's own pipeline (12 elements, relations, chapters, prompts,
// sounds, anchors, ~23 KB) through both paths and requires them to agree.
//
// The document is NOT vendored into this repo on purpose: a copy would drift from
// the engine, and a stale copy would make this test assert agreement about a
// document no engine version produces any more.

// engineProducedDocEnv points at a real .mtw to use instead of the default demo
// document.
const engineProducedDocEnv = "ZSY_WORLD_ENGINE_DOC"

// TestWorldValidate_RealEngineDocument runs docs/23 §9 step 2's acceptance
// criterion against a document this plugin did not author.
func TestWorldValidate_RealEngineDocument(t *testing.T) {
	docPath := findEngineProducedDoc(t)
	if docPath == "" {
		t.Skipf("找不到引擎产出的 .mtw：把 %s 指向一份，或用 "+
			"`cargo run -p world-parser --bin world-parser-demo` 生成 apps/out/demo-world.mtw。",
			engineProducedDocEnv)
	}

	raw, err := os.ReadFile(docPath)
	require.NoError(t, err)
	doc := string(raw)
	require.Greater(t, len(doc), 1000, "a real document should not be a stub: %s", docPath)

	env := newTestEnv(t)
	env.seedAccount("writer", common.RoleCommonUser)
	requireValidatorSidecar(t) // wire the sidecar before any op runs

	// ── through the op: Go → Node shim → validate.mjs ──────────────────────
	body := `{"op":"world.validate","params":{"doc":` + doc + `}}`
	rec, payload := env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.Equal(t, CodeOK, codeOf(t, payload))

	data := dataMap(t, payload)
	throughOp := issuesOf(t, data, "errors")
	warningsThroughOp := issuesOf(t, data, "warnings")

	// ── directly: validate.mjs itself ──────────────────────────────────────
	want := runValidatorDirectly(t, doc)

	// ★ The criterion, item for item.
	assert.Equal(t, want.ShapeOK, data["shape_ok"], "shape verdict")
	assert.Equal(t, want.Errors, throughOp, "errors must match validate.mjs exactly")
	assert.Equal(t, want.Warnings, warningsThroughOp, "warnings must match validate.mjs exactly")
	assert.Equal(t, len(want.Errors) == 0 && want.ShapeOK, data["ok"],
		"ok must follow from the validator's own layers")

	// The engine's own demo document is expected to be clean; assert it explicitly
	// rather than leaving "0 errors" implicit, so an engine regression shows up
	// here as well.
	assert.Empty(t, throughOp, "工具1's demo document should have no 18.3 errors")

	t.Logf("matched %s through both paths: shapeOk=%v errors=%d warnings=%d",
		filepath.Base(docPath), want.ShapeOK, len(want.Errors), len(want.Warnings))
}

// ---------------------------------------------------------------------------
// The source guard docs/23 §8.2 demands of the Go side
// ---------------------------------------------------------------------------

// TestNoWorldRuleInGo is the Go half of the source guard in docs/23 §8.3 ③.
//
// The client-side guard proves the client holds no rule. This one proves the
// same for the service orchestration layer, which is the other place a rule could
// sneak in — the failure mode docs/23 §6.4 warns about ("Go 里出现第二份『什么算
// 合法 MTW』就是错的"). It scans the package's own sources for the vocabulary a
// second implementation would have to contain: element ID prefixes, the type
// system, the consistency algorithm, prompt templates, the validator's rule
// numbering.
//
// Comments are exempt on purpose. The doc comments in this package *quote* the
// design document and the upstream vocabulary (a struct comment lists the reason
// values 工具1 uses, a comment says which upstream command an op mirrors), and a
// guard that punished documentation would only teach the next reader to delete
// the documentation. Running code is what must stay rule-free.
func TestNoWorldRuleInGo(t *testing.T) {
	files, err := filepath.Glob("*.go")
	require.NoError(t, err)
	require.NotEmpty(t, files)

	// Each entry is a rule that only the engine or the validator may own.
	forbidden := []struct {
		needle string
		why    string
	}{
		{"char_", "element ID prefix (规范 14.1)"},
		{"loc_", "element ID prefix (规范 14.1)"},
		{"type_character", "the built-in type system (规范 5.x)"},
		{"id_registry", "consistency / id stability (规范 5.2)"},
		{"consistency_anchor", "the consistency algorithm (规范 6.7)"},
		{"提示词模板", "prompt templates (PRD 6.5)"},
		{"18.3", "the validator's rule numbering — only validate.mjs owns it"},
		{"walk_element_refs", "reference traversal — the engine's contract layer"},
	}
	scanned := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue // the fixtures live in the tests on purpose
		}
		content, readErr := os.ReadFile(file)
		require.NoError(t, readErr)
		code := stripGoComments(string(content))
		scanned++
		for _, entry := range forbidden {
			assert.NotContains(t, code, entry.needle,
				"%s must not appear in %s: %s belongs to the engine/validator, not to Go",
				entry.needle, file, entry.why)
		}
	}
	require.NotZero(t, scanned, "the guard scanned nothing, which would make it pass vacuously")
}

// stripGoComments removes // line comments and /* */ block comments, so the guard
// reads code only.
func stripGoComments(src string) string {
	var out strings.Builder
	for i := 0; i < len(src); {
		switch {
		case strings.HasPrefix(src[i:], "//"):
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			i += 2
			for i < len(src) && !strings.HasPrefix(src[i:], "*/") {
				i++
			}
			i += 2
		default:
			out.WriteByte(src[i])
			i++
		}
	}
	return out.String()
}
