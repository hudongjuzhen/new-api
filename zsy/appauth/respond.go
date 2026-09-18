package appauth

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// Stable machine-readable error codes. Callers must branch on these, never on
// the localized message text.
const (
	codeInvalidParams             = "INVALID_PARAMS"
	codeRegisterDisabled          = "REGISTER_DISABLED"
	codePasswordRegisterDisabled  = "PASSWORD_REGISTER_DISABLED"
	codePasswordLoginDisabled     = "PASSWORD_LOGIN_DISABLED"
	codeEmailVerificationRequired = "EMAIL_VERIFICATION_REQUIRED"
	codeVerificationCodeError     = "VERIFICATION_CODE_ERROR"
	codeInviteCodeRequired        = "INVITE_CODE_REQUIRED"
	codeInviteCodeInvalid         = "INVITE_CODE_INVALID"
	codeEmailTaken                = "EMAIL_TAKEN"
	codeUserExists                = "USER_EXISTS"
	codeLoginFailed               = "LOGIN_FAILED"
	codeMFARequired               = "MFA_REQUIRED"
	codeRegisterFailed            = "REGISTER_FAILED"
	codeKeyCreateFailed           = "KEY_CREATE_FAILED"
	codeSessionFailed             = "SESSION_FAILED"
	codeDatabaseError             = "DATABASE_ERROR"
	codeAppSecretInvalid          = "APP_SECRET_INVALID"
)

func respondSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": data})
}

// respondError reports a business failure with a real HTTP status code plus a
// stable code, and a message localized through the host i18n catalogue.
func respondError(c *gin.Context, status int, code string, messageKey string, args ...map[string]any) {
	c.JSON(status, gin.H{
		"success": false,
		"code":    code,
		"message": common.TranslateMessage(c, messageKey, args...),
	})
}

// respondErrorText is respondError for plugin-specific messages that have no
// entry in the host i18n catalogue.
func respondErrorText(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": message})
}

// requireAppSecret enforces the optional pre-shared secret. Installing it only
// when ZSY_AUTH_APP_SECRET is configured keeps the plugin zero-config, while
// letting a deployment lock the endpoints down to known callers.
func requireAppSecret(c *gin.Context) {
	provided := strings.TrimSpace(c.GetHeader(appSecretHeader))
	if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.AppSecret)) != 1 {
		respondErrorText(c, http.StatusUnauthorized, codeAppSecretInvalid,
			"缺少或无效的 "+appSecretHeader+" 请求头")
		c.Abort()
		return
	}
	c.Next()
}

// userView is the secret-free user projection returned to third-party callers.
type userView struct {
	ID          int    `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Group       string `json:"group"`
	Quota       int    `json:"quota"`
	UsedQuota   int    `json:"used_quota"`
	Status      int    `json:"status"`
}

// keyView describes the API key backing the account. APIKey holds the key
// exactly as stored — no "sk-" prefix, matching the host's
// POST /api/token/:id/key — and callers send it as
// "Authorization: Bearer sk-<api_key>".
type keyView struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Group          string `json:"group"`
	APIKey         string `json:"api_key"`
	Status         int    `json:"status"`
	UnlimitedQuota bool   `json:"unlimited_quota"`
	ExpiredTime    int64  `json:"expired_time"`
	// GroupUsable reports whether the key's group is currently reachable for
	// this user's group. A false value means relay requests with this key are
	// rejected with 403 until the operator enables the group (or grants it to
	// the user's group).
	GroupUsable bool `json:"group_usable"`
}

// sessionView mirrors the host login bundle for callers that want to act as the
// user through the dashboard API as well.
type sessionView struct {
	AccessToken     string `json:"access_token"`
	TokenType       string `json:"token_type"`
	AccessExpiresAt int64  `json:"access_expires_at"`
	SID             string `json:"sid"`
}

// authResult is the shared success payload of register and login.
type authResult struct {
	User       userView     `json:"user"`
	Key        keyView      `json:"key"`
	KeyCreated bool         `json:"key_created"`
	Session    *sessionView `json:"session,omitempty"`
	// Warning explains a condition the caller should surface to an operator,
	// e.g. a default key group that is not reachable for this user.
	Warning string `json:"warning,omitempty"`
}
