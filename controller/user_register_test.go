package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupRegisterTestDB boots an isolated database carrying the tables the
// register path touches: the account itself, its sessions (a password-bearing
// insert may revoke them), the system log and the optional default token.
func setupRegisterTestDB(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousInviteCodeRequired := common.InviteCodeRequired
	previousQuotaForNewUser := common.QuotaForNewUser

	common.RedisEnabled = false
	common.QuotaForNewUser = 0
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.UserSession{}, &model.Log{}, &model.Token{}))

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.InviteCodeRequired = previousInviteCodeRequired
		common.QuotaForNewUser = previousQuotaForNewUser
		common.SetDatabaseTypes(previousMainType, previousLogType)
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
}

// performRegister drives the real register handler and reports whether the site
// accepted the request, together with the server message for diagnostics.
func performRegister(t *testing.T, body string) (bool, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")

	Register(c)

	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload), "body=%s", recorder.Body.String())
	return payload.Success, payload.Message
}

func seedInviter(t *testing.T, username, inviteCode string) model.User {
	t.Helper()
	inviter := model.User{
		Username: username,
		Password: "unused-password-hash",
		Status:   common.UserStatusEnabled,
		AffCode:  inviteCode,
	}
	require.NoError(t, model.DB.Create(&inviter).Error)
	return inviter
}

func registeredUser(t *testing.T, username string) *model.User {
	t.Helper()
	var user model.User
	err := model.DB.Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	require.NoError(t, err)
	return &user
}

func TestRegisterRequiresInviteCodeWhenEnabled(t *testing.T) {
	setupRegisterTestDB(t)
	common.InviteCodeRequired = true

	tests := []struct {
		name string
		code string
	}{
		{name: "no invite code", code: ""},
		{name: "blank invite code", code: "   "},
		{name: "unknown invite code", code: "not-a-real-code"},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			username := fmt.Sprintf("no_invite_user_%d", index)
			body := fmt.Sprintf(
				`{"username":%q,"password":"pass12345","aff_code":%q}`,
				username, tt.code,
			)
			success, message := performRegister(t, body)
			assert.False(t, success, "registration must be refused (message=%s)", message)
			assert.Nil(t, registeredUser(t, username),
				"a refused registration must not create an account")
		})
	}
}

func TestRegisterBindsInviterFromInviteCode(t *testing.T) {
	setupRegisterTestDB(t)
	common.InviteCodeRequired = true
	inviter := seedInviter(t, "invite-code-owner", "inv9")

	body := `{"username":"invited_user","password":"pass12345","aff_code":"inv9"}`
	success, message := performRegister(t, body)
	require.True(t, success, "a valid invite code must be accepted (message=%s)", message)

	created := registeredUser(t, "invited_user")
	require.NotNil(t, created)
	assert.Equal(t, inviter.Id, created.InviterId, "the invite relationship must be recorded")
}

func TestRegisterWithoutInviteCodeStillWorksWhenNotRequired(t *testing.T) {
	setupRegisterTestDB(t)
	common.InviteCodeRequired = false

	body := `{"username":"open_user","password":"pass12345"}`
	success, message := performRegister(t, body)
	require.True(t, success, "a site that does not ask for an invite code must still register (message=%s)", message)

	created := registeredUser(t, "open_user")
	require.NotNil(t, created)
	assert.Zero(t, created.InviterId)
}
