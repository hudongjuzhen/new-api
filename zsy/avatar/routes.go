package avatar

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// mountRoutes attaches both faces of the plugin. It is registered with extcore
// and invoked once from router.SetRouter after the core routes are installed, so
// installing or removing the plugin never edits a core router file.
//
//	/api/zsy/avatar/list      (public, no auth)  paginated on-shelf catalog
//	/api/zsy/avatar/:id       (public, no auth)  one on-shelf persona
//	/dashboard/zsy/avatar/…   (admin auth)       CRUD + CSV import/export
//
// The avatar group is its own URL space rather than a child of
// /api/zsy/voice/…: a persona is a separate catalog with its own filters, and
// nesting it under the voice group would collide with the voice `:id` route.
//
// Both groups sit outside the core router groups on purpose: the plugin must not
// depend on the internals of SetApiRouter / SetDashboardRouter, it only reuses
// the host's named middlewares.
func mountRoutes(router *gin.Engine) {
	public := router.Group("/api/zsy/avatar")
	public.Use(middleware.RouteTag("api"))
	public.Use(middleware.GlobalAPIRateLimit())
	public.Use(middleware.DisableCache())
	{
		public.GET("/list", listPublicAvatars)
		public.GET("/:id", getPublicAvatar)
	}

	admin := router.Group("/dashboard/zsy/avatar")
	admin.Use(middleware.AdminAuth())
	// The admin API is mutable data: a cached list would keep showing rows an
	// import or another admin already replaced, so every response is no-store.
	admin.Use(middleware.DisableCache())
	{
		admin.GET("/list", listAvatarsAdmin)
		admin.POST("", createAvatar)
		admin.GET("/export", exportAvatars)
		admin.POST("/import", importAvatars)
		admin.GET("/:id", getAvatar)
		admin.PUT("/:id", updateAvatar)
		admin.DELETE("/:id", deleteAvatar)
	}

	common.SysLog("zsy-avatar: mounted /api/zsy/avatar/list and /dashboard/zsy/avatar/{list,export,import,:id}")
}
