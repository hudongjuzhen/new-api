package appauth

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// mountRoutes attaches the third-party application face of the plugin. It is
// called once from router.SetRouter through the extcore route registry.
//
// Middleware parity with the host's /api group (RouteTag, global API rate
// limit, critical per-IP rate limit, anonymous body size limit, no caching)
// plus the optional pre-shared app secret. middleware.TurnstileCheck is
// deliberately absent: a server-to-server caller cannot solve a browser
// challenge.
func mountRoutes(router *gin.Engine) {
	if !cfg.Enabled {
		common.SysLog("zsy-appauth: routes not mounted, " + envEnabled + " is false")
		return
	}

	group := router.Group("/api/zsy/auth")
	group.Use(middleware.RouteTag("api"))
	group.Use(middleware.GlobalAPIRateLimit())
	group.Use(middleware.CriticalRateLimit())
	group.Use(middleware.AnonymousRequestBodyLimit())
	group.Use(middleware.DisableCache())
	if cfg.AppSecret != "" {
		group.Use(requireAppSecret)
	}
	{
		group.POST("/register", register)
		group.POST("/login", login)
	}

	common.SysLog("zsy-appauth: mounted /api/zsy/auth/register and /api/zsy/auth/login, default key name=" +
		cfg.DefaultKeyName + " group=" + cfg.DefaultKeyGroup)
}
