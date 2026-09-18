package appauth

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// sessionLoginMethod labels the sessions this plugin creates, so operators can
// tell them apart from dashboard logins in the session list.
const sessionLoginMethod = "zsy_appauth"

// registerRequest is the third-party registration payload. Only username and
// password are mandatory; the email pair is required when the host enables
// EmailVerificationEnabled, and aff_code is required when the host enables
// InviteCodeRequired (registration is invite-only).
type registerRequest struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	Email            string `json:"email"`
	VerificationCode string `json:"verification_code"`
	AffCode          string `json:"aff_code"`
	// IssueSession defaults to true. Callers that only need the API key can set
	// it to false and avoid consuming the user's login-session quota.
	IssueSession *bool `json:"issue_session"`
}

// loginRequest is the third-party login payload.
type loginRequest struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	IssueSession *bool  `json:"issue_session"`
}

func wantsSession(flag *bool) bool {
	return flag == nil || *flag
}

// register creates a user through the same rules the host register endpoint
// enforces, then provisions the account's API key and returns it in plaintext.
func register(c *gin.Context) {
	if !common.RegisterEnabled {
		respondError(c, http.StatusForbidden, codeRegisterDisabled, i18n.MsgUserRegisterDisabled)
		return
	}
	if !common.PasswordRegisterEnabled {
		respondError(c, http.StatusForbidden, codePasswordRegisterDisabled, i18n.MsgUserPasswordRegisterDisabled)
		return
	}

	var req registerRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		respondError(c, http.StatusBadRequest, codeInvalidParams, i18n.MsgInvalidParams)
		return
	}

	candidate := model.User{
		Username:         strings.TrimSpace(req.Username),
		Password:         req.Password,
		Email:            model.NormalizeEmail(req.Email),
		VerificationCode: strings.TrimSpace(req.VerificationCode),
		AffCode:          strings.TrimSpace(req.AffCode),
	}
	if candidate.Username == "" {
		respondError(c, http.StatusBadRequest, codeInvalidParams, i18n.MsgInvalidParams)
		return
	}
	if err := common.Validate.Struct(&candidate); err != nil {
		respondError(c, http.StatusBadRequest, codeInvalidParams, i18n.MsgUserInputInvalid,
			map[string]any{"Error": err.Error()})
		return
	}

	if common.EmailVerificationEnabled {
		if candidate.Email == "" || candidate.VerificationCode == "" {
			respondError(c, http.StatusBadRequest, codeEmailVerificationRequired, i18n.MsgUserEmailVerificationRequired)
			return
		}
		if !common.VerifyCodeWithKey(candidate.Email, candidate.VerificationCode, common.EmailVerificationPurpose) {
			respondError(c, http.StatusBadRequest, codeVerificationCodeError, i18n.MsgUserVerificationCodeError)
			return
		}
		if err := model.EnsureEmailAvailable(candidate.Email, 0); err != nil {
			if errors.Is(err, model.ErrEmailAlreadyTaken) {
				respondError(c, http.StatusConflict, codeEmailTaken, i18n.MsgUserEmailAlreadyTaken)
				return
			}
			common.SysError("zsy-appauth: ensure email available failed: " + err.Error())
			respondError(c, http.StatusInternalServerError, codeDatabaseError, i18n.MsgDatabaseError)
			return
		}
	}

	emailForExistCheck := ""
	if common.EmailVerificationEnabled {
		emailForExistCheck = candidate.Email
	}
	exist, err := model.CheckUserExistOrDeleted(candidate.Username, emailForExistCheck)
	if err != nil {
		common.SysError("zsy-appauth: check user exists failed: " + err.Error())
		respondError(c, http.StatusInternalServerError, codeDatabaseError, i18n.MsgDatabaseError)
		return
	}
	if exist {
		respondError(c, http.StatusConflict, codeUserExists, i18n.MsgUserExists)
		return
	}

	inviterID, inviteErr := model.CheckInviteCode(candidate.AffCode)
	if inviteErr != nil {
		if errors.Is(inviteErr, model.ErrInviteCodeRequired) {
			respondError(c, http.StatusBadRequest, codeInviteCodeRequired, i18n.MsgUserInviteCodeRequired)
			return
		}
		respondError(c, http.StatusBadRequest, codeInviteCodeInvalid, i18n.MsgUserInviteCodeInvalid)
		return
	}
	newUser := model.User{
		Username:    candidate.Username,
		Password:    candidate.Password,
		DisplayName: candidate.Username,
		InviterId:   inviterID,
		Role:        common.RoleCommonUser,
	}
	if common.EmailVerificationEnabled {
		newUser.Email = candidate.Email
	}
	if err := newUser.Insert(inviterID); err != nil {
		if errors.Is(err, model.ErrEmailAlreadyTaken) {
			respondError(c, http.StatusConflict, codeEmailTaken, i18n.MsgUserEmailAlreadyTaken)
			return
		}
		common.SysError("zsy-appauth: insert user failed: " + err.Error())
		respondError(c, http.StatusInternalServerError, codeRegisterFailed, i18n.MsgUserRegisterFailed)
		return
	}

	var created model.User
	if err := model.DB.Where("username = ?", newUser.Username).First(&created).Error; err != nil {
		common.SysError("zsy-appauth: reload created user failed: " + err.Error())
		respondError(c, http.StatusInternalServerError, codeRegisterFailed, i18n.MsgUserRegisterFailed)
		return
	}

	result, ok := provisionAccount(c, &created, req.IssueSession)
	if !ok {
		return
	}
	respondSuccess(c, result)
}

// login verifies the credentials, makes sure the account owns its API key and
// returns both. Accounts that enabled TOTP are rejected here: this API does not
// weaken two-factor authentication, the caller has to finish through the host's
// POST /api/user/login + POST /api/user/login/2fa flow.
func login(c *gin.Context) {
	if !common.PasswordLoginEnabled {
		respondError(c, http.StatusForbidden, codePasswordLoginDisabled, i18n.MsgUserPasswordLoginDisabled)
		return
	}

	var req loginRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		respondError(c, http.StatusBadRequest, codeInvalidParams, i18n.MsgInvalidParams)
		return
	}
	username := strings.TrimSpace(req.Username)
	if username == "" || req.Password == "" {
		respondError(c, http.StatusBadRequest, codeInvalidParams, i18n.MsgInvalidParams)
		return
	}

	user := model.User{Username: username, Password: req.Password}
	if err := user.ValidateAndFill(); err != nil {
		if errors.Is(err, model.ErrDatabase) {
			common.SysError("zsy-appauth: login database error: " + err.Error())
			respondError(c, http.StatusInternalServerError, codeDatabaseError, i18n.MsgDatabaseError)
			return
		}
		respondError(c, http.StatusUnauthorized, codeLoginFailed, i18n.MsgUserUsernameOrPasswordError)
		return
	}

	twoFAEnabled, err := model.IsTwoFAEnabled(user.Id)
	if err != nil {
		common.SysError(fmt.Sprintf("zsy-appauth: load 2FA state for user %d failed: %v", user.Id, err))
		respondError(c, http.StatusInternalServerError, codeDatabaseError, i18n.MsgDatabaseError)
		return
	}
	if twoFAEnabled {
		respondErrorText(c, http.StatusForbidden, codeMFARequired,
			"该账号已开启两步验证：本接口不处理 TOTP，请改用 POST /api/user/login 获取 flow_token，再调用 POST /api/user/login/2fa")
		return
	}

	result, ok := provisionAccount(c, &user, req.IssueSession)
	if !ok {
		return
	}
	respondSuccess(c, result)
}

// provisionAccount builds the shared register/login payload: the user view, the
// account key (created when missing) and, unless the caller opted out, a
// dashboard login session. On failure it writes the error response itself and
// reports ok=false.
func provisionAccount(c *gin.Context, user *model.User, issueSession *bool) (authResult, bool) {
	token, keyCreated, err := ensureDefaultKey(user.Id)
	if err != nil {
		common.SysError(fmt.Sprintf("zsy-appauth: provision key for user %d failed: %v", user.Id, err))
		respondErrorText(c, http.StatusInternalServerError, codeKeyCreateFailed,
			"默认密钥创建失败，请稍后重试或调用 /api/zsy/auth/login（登录会补建缺失的默认密钥）")
		return authResult{}, false
	}

	result := authResult{
		User:       newUserView(user),
		Key:        newKeyView(token, user.Group),
		KeyCreated: keyCreated,
		Warning:    keyGroupWarning(user.Group, token.Group),
	}

	if !wantsSession(issueSession) {
		return result, true
	}
	bundle, err := service.CreateLoginSession(user.Id, sessionLoginMethod, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		status, code := service.AuthSessionErrorCode(err)
		common.SysError("zsy-appauth: create login session failed: " + err.Error())
		respondErrorText(c, status, code, "登录会话创建失败："+err.Error())
		return authResult{}, false
	}
	model.UpdateUserLastLoginAt(user.Id)
	service.WriteRefreshCookie(c, bundle.RefreshToken)
	result.Session = &sessionView{
		AccessToken:     bundle.AccessToken,
		TokenType:       bundle.TokenType,
		AccessExpiresAt: bundle.AccessExpiresAt,
		SID:             bundle.Session.SID,
	}
	return result, true
}
