package mode

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// mountRoutes attaches both faces of the plugin. It is registered with extcore
// and invoked once from router.SetRouter after the core routes are installed, so
// installing or removing the plugin never edits a core router file.
//
//	/api/zsy/mode/list                      ★ 有哪些可以开通（认账号，但不要求登录）
//	/api/zsy/mode/:id/file                  ★「开通」下的就是它（同上）
//	/dashboard/zsy/mode/list                (admin auth) 全部模式 + 每档几个人有权限
//	/dashboard/zsy/mode/:id/meta            (admin auth) ★ 改一档的公开 / 私有 + 说明
//	/dashboard/zsy/mode/entitlements        (admin auth) 谁被授权了哪几档
//	/dashboard/zsy/mode/entitlements/grant  (admin auth) ★ 后台给权限
//	/dashboard/zsy/mode/entitlements/revoke (admin auth) ★ 收回来
//
// Middleware parity with the host's /api group (RouteTag, global API rate limit,
// no caching) is copied from zsy/voice and zsy/world on purpose: these routes sit
// outside the core router groups, so the plugin must reuse the host's named
// middlewares rather than depend on the internals of SetApiRouter.
//
// ⚠★ The public face requires **no** login, but it does **identify** you when you
// present a key: `public` modes are answered to everyone, and a `private` mode
// only to an account the admin granted it to (docs/28 §3b). A key that is present
// and invalid is an error rather than a silent downgrade to anonymous — see
// auth.go for why that direction is the only safe one.
func mountRoutes(router *gin.Engine) {
	public := router.Group("/api/zsy/mode")
	public.Use(middleware.RouteTag("api"))
	public.Use(middleware.GlobalAPIRateLimit())
	/*
	 * ⚠ 模式文件**不能**被中间层缓存：运营改一份模式、或改一个 `x-visibility`、
	 * 或在后台给某个账号授权，要**立刻**生效（"开通"是一个动作，而用户按下它
	 * 之前应当看到的是**此刻**他能开通的东西 —— 列出来的是按账号算的）。
	 * 客户端那一侧同样没有缓存（第 4 段那一条照 `mode_catalog.rs` 写）。
	 */
	public.Use(middleware.DisableCache())
	// ★★ 认账号（可匿名）—— 它是 private 那一半能"给了权限之后才显示"的全部机制。
	public.Use(resolveModeAccount)
	{
		public.GET("/list", listPublicModes)
		public.GET("/:id/file", getModeFile)
	}

	admin := router.Group("/dashboard/zsy/mode")
	admin.Use(middleware.AdminAuth())
	// The admin API is mutable data: a cached list would keep showing rows another
	// operator already replaced, so every response is no-store.
	admin.Use(middleware.DisableCache())
	{
		admin.GET("/list", listAdminModes)
		admin.POST("/:id/meta", updateModeMeta)
		admin.GET("/entitlements", listAdminEntitlements)
		admin.POST("/entitlements/grant", grantModeEntitlement)
		admin.POST("/entitlements/revoke", revokeModeEntitlement)
	}

	common.SysLog("zsy-mode: mounted /api/zsy/mode/{list,:id/file} and " +
		"/dashboard/zsy/mode/{list,:id/meta,entitlements,entitlements/grant,entitlements/revoke}")
}
