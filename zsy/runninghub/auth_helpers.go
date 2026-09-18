package runninghub

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

// Caller authentication ------------------------------------------------------
//
// The user-facing route family (/api/zsy/rh/...) serves three kinds of callers
// through the same Authorization header:
//
//   - a dashboard session / access JWT      (the web UI)          → dashboard
//   - a dashboard personal access token     (users.access_token)  → dashboard
//   - a relay API key (sk-…)                (third-party services) → API key
//
// middleware.UserAuth understands the first two and middleware.TokenAuth the
// third, so the credential is classified first and the matching middleware is
// delegated to. The classification is also recorded on the context: handlers
// must be able to tell "called with an API key" (quota, task visibility and
// model limits scoped to that one key) apart from "called from the dashboard"
// (scoped to the whole account). It is set before delegating because the
// delegate runs the rest of the chain itself.

const callerKindContextKey = "rh_caller_kind"

const (
	callerKindDashboard = "dashboard"
	callerKindToken     = "token"
)

// userAuthRequired delegates to the dashboard auth middleware (session JWT or
// personal access token).
func userAuthRequired(c *gin.Context) { middleware.UserAuth()(c) }

// adminAuthRequired delegates to the dashboard admin-auth middleware.
func adminAuthRequired(c *gin.Context) { middleware.AdminAuth()(c) }

// callerAuthRequired accepts a dashboard credential or a relay API key. Both
// delegates run the remaining handler chain themselves, so this middleware must
// not call c.Next() afterwards.
func callerAuthRequired(c *gin.Context) {
	credential := bearerCredential(c)
	if credential != "" && isDashboardCredential(credential) {
		c.Set(callerKindContextKey, callerKindDashboard)
		middleware.UserAuth()(c)
		return
	}
	c.Set(callerKindContextKey, callerKindToken)
	middleware.TokenAuth()(c)
}

// callerUsesAPIKey reports whether this request authenticated with a relay API
// key rather than a dashboard credential.
func callerUsesAPIKey(c *gin.Context) bool {
	return c.GetString(callerKindContextKey) == callerKindToken
}

// bearerCredential returns the raw Authorization credential without the
// optional "Bearer " prefix, or "" when the header is absent.
func bearerCredential(c *gin.Context) string {
	header := strings.TrimSpace(c.GetHeader("Authorization"))
	if len(header) > 7 && strings.EqualFold(header[:7], "bearer ") {
		return strings.TrimSpace(header[7:])
	}
	return header
}

// isDashboardCredential reports whether raw is a credential middleware.UserAuth
// accepts: a dashboard access JWT (recognised structurally, without a DB hit) or
// a stored personal access token.
func isDashboardCredential(raw string) bool {
	if _, internal, _ := service.ParseDashboardAccessToken(raw); internal {
		return true
	}
	user, err := model.ValidateAccessToken(raw)
	return err == nil && user != nil
}

// apiError writes an error a third-party client can act on: a real HTTP status
// plus a stable machine-readable code. The dashboard's {success, message} shape
// is preserved so the web UI keeps rendering the message unchanged.
func apiError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{
		"success": false,
		"code":    code,
		"message": message,
	})
}

// tokenScopeDenied returns a non-empty explanation when the authenticated caller
// must not act on task.
//
// The dashboard may act on every task of the signed-in account. An API key is
// narrower: it only reaches the runs it paid for, so a key can never read or
// cancel a run another key (or an unkeyed dashboard run) started.
func tokenScopeDenied(c *gin.Context, task *model.Task) string {
	if task == nil {
		return "任务不存在或无权访问"
	}
	if !callerUsesAPIKey(c) {
		return ""
	}
	tokenID := c.GetInt(string(constant.ContextKeyTokenId))
	if tokenID > 0 && task.PrivateData.TokenId == tokenID {
		return ""
	}
	return "该任务不是当前 API Key 提交的任务，API Key 只能访问自己提交的记录"
}

// tokenAllowsApp reports whether the key paying for this request may use the
// app, honouring the host's per-token model allow-list.
//
// middleware.Distribute enforces that list on the /v1/* relay path; the plugin
// resolves its own channel from the site pool, so the check has to be repeated
// here — otherwise a key restricted to a model list could run every RunningHub
// app. A dashboard caller that did not select a key has no token context and is
// governed by the account's own group permissions instead.
func tokenAllowsApp(c *gin.Context, upstreamID string) bool {
	if c.GetInt(string(constant.ContextKeyTokenId)) <= 0 {
		return true
	}
	enabled, _ := common.GetContextKey(c, constant.ContextKeyTokenModelLimitEnabled)
	if allowed, ok := enabled.(bool); !ok || !allowed {
		return true
	}
	limits, _ := common.GetContextKey(c, constant.ContextKeyTokenModelLimit)
	allowedModels, ok := limits.(map[string]bool)
	if !ok || len(allowedModels) == 0 {
		return false
	}
	_, allowed := allowedModels[ratio_setting.FormatMatchingModelName(upstreamID)]
	return allowed
}
