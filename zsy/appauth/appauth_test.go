package appauth

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
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

type testEnv struct {
	t      *testing.T
	db     *gorm.DB
	router *gin.Engine
}

type apiEnvelope struct {
	Success bool            `json:"success"`
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// newTestEnv boots an isolated stack: an in-memory SQLite database carrying the
// host tables this plugin touches, and a gin engine with the plugin's real
// route chain mounted. Process-wide globals are snapshotted and restored.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	origDB, origLogDB := model.DB, model.LOG_DB
	origRedis, origMem := common.RedisEnabled, common.MemoryCacheEnabled
	origRegister, origPasswordRegister := common.RegisterEnabled, common.PasswordRegisterEnabled
	origPasswordLogin, origEmailVerification := common.PasswordLoginEnabled, common.EmailVerificationEnabled
	origInviteCodeRequired := common.InviteCodeRequired
	origOptionMap := common.OptionMap
	origCfg := cfg
	origUsableGroups := setting.UserUsableGroups2JSONString()
	origGroupRatio := ratio_setting.GroupRatio2JSONString()

	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordLoginEnabled = true
	common.EmailVerificationEnabled = false
	common.InviteCodeRequired = false
	// model.UpdateOption writes into common.OptionMap; the test binary never
	// runs InitOptionMap, so install a fresh non-nil map.
	common.OptionMap = make(map[string]string)

	t.Cleanup(func() {
		model.DB, model.LOG_DB = origDB, origLogDB
		common.RedisEnabled, common.MemoryCacheEnabled = origRedis, origMem
		common.RegisterEnabled, common.PasswordRegisterEnabled = origRegister, origPasswordRegister
		common.PasswordLoginEnabled, common.EmailVerificationEnabled = origPasswordLogin, origEmailVerification
		common.InviteCodeRequired = origInviteCodeRequired
		common.OptionMap = origOptionMap
		cfg = origCfg
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(origUsableGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(origGroupRatio))
	})

	dsn := fmt.Sprintf("file:zsy_appauth_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	// The reserved-word column quoting helpers (commonKeyCol …) are only
	// initialized by InitDB/InitLogDB. Pin LOG_SQL_DSN empty so InitLogDB takes
	// the "LOG_DB = DB" branch and runs initCol() against the in-memory database.
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Token{}, &model.UserSession{}, &model.TwoFA{}))
	t.Cleanup(func() { _ = sqlDB.Close() })

	gin.SetMode(gin.TestMode)
	router := gin.New()
	// Mount through the extcore registry, i.e. the exact path router.SetRouter
	// uses in production, so a broken registration fails the tests too.
	extcore.MountRoutes(router)

	return &testEnv{t: t, db: db, router: router}
}

func (e *testEnv) doJSON(method, path string, body any, headers map[string]string) (*httptest.ResponseRecorder, apiEnvelope) {
	e.t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		require.NoError(e.t, err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	e.router.ServeHTTP(rec, req)

	envelope := apiEnvelope{}
	if rec.Body.Len() > 0 && strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		require.NoError(e.t, json.Unmarshal(rec.Body.Bytes(), &envelope), "body=%s", rec.Body.String())
	}
	return rec, envelope
}

// registerUser drives the real register endpoint and returns the parsed result.
func (e *testEnv) registerUser(username string, extra map[string]any) authResult {
	e.t.Helper()
	body := map[string]any{"username": username, "password": "pass12345"}
	for k, v := range extra {
		body[k] = v
	}
	rec, envelope := e.doJSON(http.MethodPost, "/api/zsy/auth/register", body, nil)
	require.Equal(e.t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.True(e.t, envelope.Success, "register failed: %s", rec.Body.String())

	result := authResult{}
	require.NoError(e.t, json.Unmarshal(envelope.Data, &result))
	return result
}

func (e *testEnv) seedUser(username, password string) *model.User {
	e.t.Helper()
	user := &model.User{
		Username:    username,
		Password:    password,
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
	}
	require.NoError(e.t, user.Insert(0))
	var stored model.User
	require.NoError(e.t, model.DB.Where("username = ?", username).First(&stored).Error)
	return &stored
}

func (e *testEnv) tokensOf(userID int) []model.Token {
	e.t.Helper()
	var tokens []model.Token
	require.NoError(e.t, model.DB.Where("user_id = ?", userID).Order("id asc").Find(&tokens).Error)
	return tokens
}

// ---------------------------------------------------------------------------
// Key provisioning
// ---------------------------------------------------------------------------

func TestEnsureDefaultKey_ProvisionsConfiguredNameAndGroup(t *testing.T) {
	env := newTestEnv(t)
	user := env.seedUser("key_owner", "pass12345")

	token, created, err := ensureDefaultKey(user.Id)
	require.NoError(t, err)
	require.True(t, created)

	assert.Equal(t, cfg.DefaultKeyName, token.Name)
	assert.Equal(t, cfg.DefaultKeyGroup, token.Group)
	assert.Equal(t, common.TokenStatusEnabled, token.Status)
	assert.True(t, token.UnlimitedQuota)
	assert.Equal(t, int64(-1), token.ExpiredTime, "the account key must never expire")
	assert.False(t, token.ModelLimitsEnabled)
	assert.Empty(t, token.ModelLimits)
	assert.Len(t, token.GetFullKey(), 48, "key length follows common.GenerateKey")

	// The key must be resolvable through the host's own lookup used by relay auth.
	stored, err := model.GetTokenByKey(token.GetFullKey(), false)
	require.NoError(t, err)
	assert.Equal(t, user.Id, stored.UserId)

	storedTokens := env.tokensOf(user.Id)
	require.Len(t, storedTokens, 1, "provisioning must not create duplicate keys")
}

func TestEnsureDefaultKey_IsIdempotent(t *testing.T) {
	env := newTestEnv(t)
	user := env.seedUser("key_reuse", "pass12345")

	first, created, err := ensureDefaultKey(user.Id)
	require.NoError(t, err)
	require.True(t, created)

	second, createdAgain, err := ensureDefaultKey(user.Id)
	require.NoError(t, err)
	assert.False(t, createdAgain)
	assert.Equal(t, first.Id, second.Id)
	assert.Len(t, env.tokensOf(user.Id), 1)
}

func TestEnsureDefaultKey_ResolvesRenamedKeyByGroup(t *testing.T) {
	env := newTestEnv(t)
	user := env.seedUser("key_renamed", "pass12345")

	token, _, err := ensureDefaultKey(user.Id)
	require.NoError(t, err)

	token.Name = "管理员改过的名字"
	require.NoError(t, token.Update())

	found, created, err := ensureDefaultKey(user.Id)
	require.NoError(t, err)
	assert.False(t, created, "a renamed key must be reused, not duplicated")
	assert.Equal(t, token.Id, found.Id)
	assert.Len(t, env.tokensOf(user.Id), 1)
}

func TestKeyGroupWarning_Table(t *testing.T) {
	newTestEnv(t) // installs the global snapshot/restore fixtures

	tests := []struct {
		name        string
		usable      string
		groupRatio  string
		userGroup   string
		keyGroup    string
		wantWarning bool
	}{
		{
			name:        "group granted to the user group and priced",
			usable:      `{"default":"默认分组","官方渠道":"官方渠道"}`,
			groupRatio:  `{"default":1,"官方渠道":1}`,
			userGroup:   "default",
			keyGroup:    "官方渠道",
			wantWarning: false,
		},
		{
			name:        "group missing from the user usable groups",
			usable:      `{"default":"默认分组"}`,
			groupRatio:  `{"default":1,"官方渠道":1}`,
			userGroup:   "default",
			keyGroup:    "官方渠道",
			wantWarning: true,
		},
		{
			name:        "group missing from the group ratio table",
			usable:      `{"default":"默认分组","官方渠道":"官方渠道"}`,
			groupRatio:  `{"default":1}`,
			userGroup:   "default",
			keyGroup:    "官方渠道",
			wantWarning: true,
		},
		{
			name:        "empty key group follows the user group",
			usable:      `{"default":"默认分组"}`,
			groupRatio:  `{"default":1}`,
			userGroup:   "default",
			keyGroup:    "",
			wantWarning: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(tt.usable))
			require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(tt.groupRatio))

			warning := keyGroupWarning(tt.userGroup, tt.keyGroup)
			if tt.wantWarning {
				assert.NotEmpty(t, warning)
				return
			}
			assert.Empty(t, warning)
		})
	}
}

// ---------------------------------------------------------------------------
// POST /api/zsy/auth/register
// ---------------------------------------------------------------------------

func TestRegister_CreatesUserAndReturnsUsableKey(t *testing.T) {
	env := newTestEnv(t)

	result := env.registerUser("fresh_user", nil)

	require.NotNil(t, result.User.ID)
	assert.Equal(t, "fresh_user", result.User.Username)
	assert.Equal(t, "fresh_user", result.User.DisplayName)
	assert.Equal(t, common.RoleCommonUser, env.reloadRole(t, "fresh_user"))
	assert.True(t, result.KeyCreated, "registration must provision the account key")

	assert.Equal(t, DefaultKeyName, result.Key.Name)
	assert.Equal(t, DefaultKeyGroup, result.Key.Group)
	assert.True(t, result.Key.UnlimitedQuota)
	assert.Equal(t, int64(-1), result.Key.ExpiredTime)
	assert.Len(t, result.Key.APIKey, 48)

	stored, err := model.GetTokenByKey(result.Key.APIKey, false)
	require.NoError(t, err)
	assert.Equal(t, result.User.ID, stored.UserId)
	assert.Equal(t, DefaultKeyName, stored.Name)
	assert.Equal(t, DefaultKeyGroup, stored.Group)
	assert.Equal(t, common.TokenStatusEnabled, stored.Status)

	// The login session is issued by default so a caller can act immediately.
	require.NotNil(t, result.Session)
	assert.NotEmpty(t, result.Session.AccessToken)
	assert.Equal(t, "Bearer", result.Session.TokenType)
	assert.NotEmpty(t, result.Session.SID)
}

func (e *testEnv) reloadRole(t *testing.T, username string) int {
	t.Helper()
	var stored model.User
	require.NoError(t, model.DB.Where("username = ?", username).First(&stored).Error)
	return stored.Role
}

func TestRegister_RejectsDuplicateUsername(t *testing.T) {
	env := newTestEnv(t)
	env.registerUser("dup_user", nil)

	rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/register",
		map[string]any{"username": "dup_user", "password": "pass12345"}, nil)

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.False(t, envelope.Success)
	assert.Equal(t, codeUserExists, envelope.Code)
}

func TestRegister_RejectsInvalidPassword(t *testing.T) {
	env := newTestEnv(t)

	rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/register",
		map[string]any{"username": "short_pw", "password": "123"}, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, codeInvalidParams, envelope.Code)
	assert.Empty(t, env.tokensOf(0), "no user may be provisioned for a rejected payload")
}

func TestRegister_HonoursHostRegisterSwitches(t *testing.T) {
	tests := []struct {
		name         string
		register     bool
		passwordReg  bool
		expectedCode string
	}{
		{name: "registration disabled", register: false, passwordReg: true, expectedCode: codeRegisterDisabled},
		{name: "password registration disabled", register: true, passwordReg: false, expectedCode: codePasswordRegisterDisabled},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			common.RegisterEnabled = tt.register
			common.PasswordRegisterEnabled = tt.passwordReg

			rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/register",
				map[string]any{"username": "blocked_user", "password": "pass12345"}, nil)

			assert.Equal(t, http.StatusForbidden, rec.Code)
			assert.Equal(t, tt.expectedCode, envelope.Code)

			var count int64
			require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "blocked_user").Count(&count).Error)
			assert.Zero(t, count, "a disabled switch must not create the account")
		})
	}
}

// Invite-only sites must refuse third-party registration without a resolvable
// invite code, and must record the inviter when one is supplied.
func TestRegister_RequiresInviteCodeWhenHostRequiresIt(t *testing.T) {
	tests := []struct {
		name         string
		affCode      string
		expectedCode string
	}{
		{name: "no invite code", affCode: "", expectedCode: codeInviteCodeRequired},
		{name: "unknown invite code", affCode: "not-a-real-code", expectedCode: codeInviteCodeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			common.InviteCodeRequired = true

			rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/register",
				map[string]any{"username": "invite_gated_user", "password": "pass12345", "aff_code": tt.affCode}, nil)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.False(t, envelope.Success)
			assert.Equal(t, tt.expectedCode, envelope.Code)

			var count int64
			require.NoError(t, model.DB.Model(&model.User{}).Where("username = ?", "invite_gated_user").Count(&count).Error)
			assert.Zero(t, count, "a refused registration must not create the account")
		})
	}
}

func TestRegister_BindsInviterFromInviteCode(t *testing.T) {
	env := newTestEnv(t)
	common.InviteCodeRequired = true
	inviter := env.seedUser("app_inviter", "pass12345")

	result := env.registerUser("app_invited_user", map[string]any{"aff_code": inviter.AffCode})

	var stored model.User
	require.NoError(t, model.DB.Where("id = ?", result.User.ID).First(&stored).Error)
	assert.Equal(t, inviter.Id, stored.InviterId, "the invite relationship must be recorded")
}

func TestRegister_NoSessionWhenOptedOut(t *testing.T) {
	env := newTestEnv(t)

	result := env.registerUser("no_session_user", map[string]any{"issue_session": false})

	assert.Nil(t, result.Session)
	assert.NotEmpty(t, result.Key.APIKey)

	var sessions int64
	require.NoError(t, model.DB.Model(&model.UserSession{}).Where("user_id = ?", result.User.ID).Count(&sessions).Error)
	assert.Zero(t, sessions, "issue_session=false must not consume login-session quota")
}

// ---------------------------------------------------------------------------
// POST /api/zsy/auth/login
// ---------------------------------------------------------------------------

func TestLogin_ReturnsAccountKeyAndProvisionsMissingOne(t *testing.T) {
	env := newTestEnv(t)
	user := env.seedUser("login_user", "pass12345")

	rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/login",
		map[string]any{"username": "login_user", "password": "pass12345"}, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.True(t, envelope.Success)

	result := authResult{}
	require.NoError(t, json.Unmarshal(envelope.Data, &result))

	assert.Equal(t, user.Id, result.User.ID)
	assert.True(t, result.KeyCreated, "login must provision the key an account created in the dashboard lacks")
	assert.Equal(t, DefaultKeyName, result.Key.Name)
	assert.Equal(t, DefaultKeyGroup, result.Key.Group)
	require.NotNil(t, result.Session)
	assert.NotEmpty(t, result.Session.AccessToken)

	// A second login reuses the same key instead of minting another one.
	_, second := env.doJSON(http.MethodPost, "/api/zsy/auth/login",
		map[string]any{"username": "login_user", "password": "pass12345"}, nil)
	secondResult := authResult{}
	require.NoError(t, json.Unmarshal(second.Data, &secondResult))
	assert.False(t, secondResult.KeyCreated)
	assert.Equal(t, result.Key.ID, secondResult.Key.ID)
	assert.Len(t, env.tokensOf(user.Id), 1)
}

func TestLogin_RejectsWrongPassword(t *testing.T) {
	env := newTestEnv(t)
	env.seedUser("bad_pw_user", "pass12345")

	rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/login",
		map[string]any{"username": "bad_pw_user", "password": "wrong-password"}, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, codeLoginFailed, envelope.Code)
	assert.Empty(t, env.tokensOf(0))
}

func TestLogin_RejectsAccountWithMFAEnabled(t *testing.T) {
	env := newTestEnv(t)
	user := env.seedUser("mfa_user", "pass12345")
	require.NoError(t, model.DB.Create(&model.TwoFA{UserId: user.Id, Secret: "JBSWY3DPEHPK3PXP", IsEnabled: true}).Error)

	rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/login",
		map[string]any{"username": "mfa_user", "password": "pass12345"}, nil)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, codeMFARequired, envelope.Code)
	assert.Len(t, env.tokensOf(user.Id), 0, "MFA accounts must not receive a key through this API")
}

func TestLogin_HonoursPasswordLoginSwitch(t *testing.T) {
	env := newTestEnv(t)
	env.seedUser("switch_user", "pass12345")
	common.PasswordLoginEnabled = false

	rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/login",
		map[string]any{"username": "switch_user", "password": "pass12345"}, nil)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Equal(t, codePasswordLoginDisabled, envelope.Code)
}

// ---------------------------------------------------------------------------
// Pre-shared app secret gate
// ---------------------------------------------------------------------------

func TestAppSecretGate(t *testing.T) {
	tests := []struct {
		name       string
		secret     string
		header     string
		wantStatus int
		wantCode   string
	}{
		{name: "wrong secret rejected", secret: "s3cret", header: "nope", wantStatus: http.StatusUnauthorized, wantCode: codeAppSecretInvalid},
		{name: "missing secret rejected", secret: "s3cret", header: "", wantStatus: http.StatusUnauthorized, wantCode: codeAppSecretInvalid},
		{name: "correct secret accepted", secret: "s3cret", header: "s3cret", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newTestEnv(t)
			cfg.AppSecret = tt.secret
			// Re-mount so the gate joins the chain with the new configuration.
			env.router = gin.New()
			mountRoutes(env.router)

			headers := map[string]string{}
			if tt.header != "" {
				headers[appSecretHeader] = tt.header
			}
			rec, envelope := env.doJSON(http.MethodPost, "/api/zsy/auth/register",
				map[string]any{"username": "gated_user", "password": "pass12345"}, headers)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantCode == "" {
				assert.True(t, envelope.Success, "body=%s", rec.Body.String())
				return
			}
			assert.Equal(t, tt.wantCode, envelope.Code)
		})
	}
}

func TestPluginIsRegisteredInExtcore(t *testing.T) {
	found := false
	for _, info := range extcore.Plugins() {
		if info.Name == pluginName {
			found = true
			assert.NotEmpty(t, info.Version)
			assert.NotEmpty(t, info.Desc)
		}
	}
	assert.True(t, found, "the plugin must appear in the extcore plugin registry")
}

func TestMountRoutes_SkippedWhenDisabled(t *testing.T) {
	env := newTestEnv(t)
	cfg.Enabled = false
	env.router = gin.New()
	mountRoutes(env.router)

	rec, _ := env.doJSON(http.MethodPost, "/api/zsy/auth/register",
		map[string]any{"username": "disabled_plugin", "password": "pass12345"}, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
