package world

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// mountRoutes attaches both faces of the plugin. It is registered with extcore
// and invoked once from router.SetRouter after the core routes are installed, so
// installing or removing the plugin never edits a core router file.
//
//	/api/zsy/world/op             (Bearer account key)  the only op entrance
//	/api/zsy/world/entitlements   (Bearer account key)  capabilities of this account
//	/dashboard/zsy/world/…        (admin auth)          projects, versions, entitlements
//
// Middleware parity with the host's /api group (RouteTag, global API rate limit,
// no caching) is copied from zsy/voice on purpose: these routes sit outside the
// core router groups, so the plugin must reuse the host's named middlewares
// rather than depend on the internals of SetApiRouter.
//
// ⚠ middleware.TokenAuth() is deliberately NOT used here, even though it is the
// host's account-key middleware. It answers in the OpenAI error shape, while
// every failure on this face must carry a docs/23 §4.1 code — and E_AUTH is one
// of the seven. requireWorldAuth validates through the same host call
// (model.ValidateUserToken) and then speaks this plugin's contract. See
// entitlements.go.
func mountRoutes(router *gin.Engine) {
	if !cfg.Enabled {
		common.SysLog("zsy-world: routes not mounted, " + envEnabled + " is false")
		return
	}

	user := router.Group("/api/zsy/world")
	user.Use(middleware.RouteTag("api"))
	user.Use(middleware.GlobalAPIRateLimit())
	user.Use(middleware.DisableCache())
	user.Use(requireWorldAuth)
	{
		user.POST("/op", opDispatch)
		user.GET("/entitlements", getEntitlements)
	}

	admin := router.Group("/dashboard/zsy/world")
	admin.Use(middleware.AdminAuth())
	// The admin API is mutable data: a cached list would keep showing rows another
	// operator already replaced, so every response is no-store.
	admin.Use(middleware.DisableCache())
	{
		admin.GET("/projects", listAdminProjects)
		admin.GET("/projects/:id", getAdminProject)
		admin.GET("/projects/:id/versions/:version", getAdminProjectVersion)
		admin.GET("/entitlements", listAdminEntitlements)
		admin.POST("/entitlements/grant", grantEntitlement)
		admin.POST("/entitlements/revoke", revokeEntitlement)
		// 插件文件：列模板 + 按账号签发一份（docs/22 §2.3 / docs/23 §12.15）
		admin.GET("/plugins", listPluginTemplates)
		admin.POST("/plugins/issue", issuePluginFile)
	}

	common.SysLog("zsy-world: mounted /api/zsy/world/{op,entitlements} and " +
		"/dashboard/zsy/world/{projects,projects/:id,projects/:id/versions/:version," +
		"entitlements,entitlements/grant,entitlements/revoke,plugins,plugins/issue}")
}
