package tone

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// mountRoutes attaches both faces of the plugin. It is registered with extcore
// and invoked once from router.SetRouter after the core routes are installed,
// so installing or removing the plugin never edits a core router file.
//
//	/api/zsy/tone/list       (public, no auth)  paginated on-shelf catalog
//	/api/zsy/tone/standard   (public, no auth)  the versioned 文风标准
//	/api/zsy/tone/:id        (public, no auth)  one on-shelf tone
//	/dashboard/zsy/tone/…    (admin auth)       CRUD + CSV import/export
//
// Both groups sit outside the core router groups on purpose: the plugin must
// not depend on the internals of SetApiRouter / SetDashboardRouter, it only
// reuses the host's named middlewares.
//
// ⚠ The two literal segments are registered **before** `/:id` inside each group.
// gin's tree would resolve them correctly either way, but reading the list in
// order is what keeps the next person from adding a literal below it and
// wondering whether it shadows the parameter.
func mountRoutes(router *gin.Engine) {
	public := router.Group("/api/zsy/tone")
	public.Use(middleware.RouteTag("api"))
	public.Use(middleware.GlobalAPIRateLimit())
	public.Use(middleware.DisableCache())
	{
		public.GET("/list", listPublicTones)
		// The standard is the one cacheable response in this group; the handler
		// overrides the Cache-Control header DisableCache just set (see there).
		public.GET("/standard", getToneStandard)
		public.GET("/:id", getPublicTone)
	}

	admin := router.Group("/dashboard/zsy/tone")
	admin.Use(middleware.AdminAuth())
	// The admin API is mutable data: a cached list would keep showing rows an
	// import or another admin already replaced, so every response is no-store.
	admin.Use(middleware.DisableCache())
	{
		admin.GET("/list", listTonesAdmin)
		admin.POST("", createTone)
		admin.GET("/export", exportTones)
		admin.POST("/import", importTones)
		admin.GET("/:id", getTone)
		admin.PUT("/:id", updateTone)
		admin.DELETE("/:id", deleteTone)
	}

	common.SysLog("zsy-tone: mounted /api/zsy/tone/{list,standard,:id} and /dashboard/zsy/tone/{list,export,import,:id}")
}
