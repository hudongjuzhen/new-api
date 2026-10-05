package voice

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

// mountRoutes attaches both faces of the plugin. It is registered with extcore
// and invoked once from router.SetRouter after the core routes are installed,
// so installing or removing the plugin never edits a core router file.
//
//	/api/zsy/voice/list      (public, no auth)  paginated on-shelf catalog
//	/api/zsy/voice/:id       (public, no auth)  one on-shelf voice
//	/dashboard/zsy/voice/…   (admin auth)       CRUD + sample-audio upload
//
// Both groups sit outside the core router groups on purpose: the plugin must
// not depend on the internals of SetApiRouter / SetDashboardRouter, it only
// reuses the host's named middlewares.
func mountRoutes(router *gin.Engine) {
	public := router.Group("/api/zsy/voice")
	public.Use(middleware.RouteTag("api"))
	public.Use(middleware.GlobalAPIRateLimit())
	public.Use(middleware.DisableCache())
	{
		public.GET("/list", listPublicVoices)
		public.GET("/:id", getPublicVoice)
	}

	admin := router.Group("/dashboard/zsy/voice")
	admin.Use(middleware.AdminAuth())
	{
		admin.GET("/list", listVoicesAdmin)
		admin.POST("", createVoice)
		admin.POST("/upload", uploadVoiceAudio)
		admin.GET("/export", exportVoices)
		admin.POST("/import", importVoices)
		admin.GET("/:id", getVoice)
		admin.PUT("/:id", updateVoice)
		admin.DELETE("/:id", deleteVoice)
	}

	common.SysLog("zsy-voice: mounted /api/zsy/voice/list and /dashboard/zsy/voice/{list,upload,export,import,:id}")
}
