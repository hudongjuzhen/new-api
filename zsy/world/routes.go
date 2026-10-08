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
//	/api/zsy/plugins              (public, no auth)     ★ the public plugin catalog
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

	/*
	 * ★★ 公共插件目录（`docs/27` §3）—— `GET /api/zsy/plugins`。
	 *
	 * ⚠ 它**挂在 `/api/zsy/plugins` 而不是 `/api/zsy/world/plugins`**：这一面
	 * 回答的是"站上有哪些插件可以装"，而世界 IP 只是其中**一份**插件 ——
	 * 挂在 world 底下会让"为什么一个声线墙插件要去世界的地址拿"变成每个
	 * 新人都要问一遍的问题。
	 *
	 * ⚠★ 它**没有鉴权**，而且这与"任何账号都能一键安装"不是矛盾：
	 * 这一面按定义只回答 `x-visibility: public` 的那些模板，私有模板
	 * 一个字节都不出现（见 `plugin_public.go` 文件头）。真正的闸门 ——
	 * "这个账号有没有某个能力" —— 在每一次 op 里现算（docs/23 §8.3 ①）。
	 */
	pluginPublic := router.Group("/api/zsy/plugins")
	pluginPublic.Use(middleware.RouteTag("api"))
	pluginPublic.Use(middleware.GlobalAPIRateLimit())
	pluginPublic.Use(middleware.DisableCache())
	{
		pluginPublic.GET("", listPublicPlugins)
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
		"entitlements,entitlements/grant,entitlements/revoke,plugins,plugins/issue} " +
		"and /api/zsy/plugins (public catalog)")
}
