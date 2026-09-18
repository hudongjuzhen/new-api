package runninghub

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// mountRoutes attaches the plugin's gin route groups. It is registered with
// extcore and invoked once from router.SetRouter after the core routes are
// installed.
//
// Route layout:
//
//	/api/zsy/rh/apps        (user-side + third-party)  list/run/poll/cancel
//	/dashboard/zsy/rh/apps  (admin)                    app CRUD, curl import
//
// The user-side group authenticates with callerAuthRequired, which accepts a
// dashboard session, a dashboard personal access token or a relay API key, so
// an external service can drive apps with the same API key it uses for /v1/*
// (see auth_helpers.go for the scope differences between the two).
//
// NOTE: These routes intentionally sit *outside* the core router groups so the
// plugin does not take a hard dependency on the internals of
// SetApiRouter/SetDashboardRouter. Admin and user auth are guarded with
// existing middlewares looked up by name via controller helpers.
func mountRoutes(router *gin.Engine) {
	// Admission loop for tasks accepted while every channel of their site was at
	// its concurrency cap (queue.go). It starts here rather than in init() so it
	// only runs in the real server process (router.SetRouter), never in the
	// plugin's own tests; the loop is idle until something is queued.
	startQueueDispatcher()

	userRoutes(router.Group("/api/zsy/rh"))

	admin := router.Group("/dashboard/zsy/rh")
	admin.Use(requireAdminAuth)
	{
		apps := admin.Group("/apps")
		{
			apps.GET("", listApps)
			apps.GET("/:id", getApp)
			apps.POST("", createApp)
			apps.PUT("/:id", updateApp)
			apps.DELETE("/:id", deleteApp)
			apps.POST("/parse-curl", parseCurlEndpoint)
			apps.POST("/fetch-template", fetchAppTemplate)
			apps.POST("/sync-from-channel", syncAppsFromChannel)
		}
		admin.GET("/app-categories", listCategories)
		admin.POST("/app-categories", createCategory)
		admin.PUT("/app-categories/:id", updateCategory)
		admin.DELETE("/app-categories/:id", deleteCategory)
		admin.GET("/stats", stats)
	}
}

// userRoutes registers the user-facing app-center API on group. Kept as its own
// function so the test hook mounts exactly the chain production uses.
func userRoutes(group *gin.RouterGroup) {
	apps := group.Group("/apps")
	{
		// Browsing is open: the app center lists what anonymous visitors may
		// see, and the dynamic form is rendered from the same payload.
		apps.GET("", listPublicApps)
		apps.GET("/:id", getPublicAppDetail)

		apps.POST("/:id/run", requireCallerAuth, submitAppRun)
		apps.GET("/task/:task_id", requireCallerAuth, getAppTaskResult)
		// Inline text preview of one result file. Task-scoped: the URL must be
		// one of the caller's own task results (the upstream storage sends no
		// CORS headers, so the browser cannot read it directly).
		apps.GET("/task/:task_id/content", requireCallerAuth, getTaskResultContent)
		// Cancel a queued or running run: local fail+refund while the task is
		// still queued, upstream cancel (then fail+refund) once it runs.
		apps.POST("/task/:task_id/cancel", requireCallerAuth, cancelAppTask)
		// Dashboard-only: the host task table carries no index by API key, so a
		// key-scoped list cannot be filtered without a schema change (see
		// listMyRhTasks).
		apps.GET("/tasks", requireCallerAuth, listMyRhTasks)
	}
	// Media upload proxy: forwards user files to the RunningHub site the app's
	// `site` field declares (SSRF-safe: the target
	// /openapi/v2/media/upload/binary is derived from the resolved channel's
	// configured base URL, never from anything the caller sends).
	// Consumers that reach back with the returned fileName (e.g. image/video
	// node inputs) do so by calling the public file endpoint of the *matching*
	// site (cn vs ai), which the harness does not filter — making fileName
	// round-trips work as-is.
	group.POST("/upload", requireCallerAuth, uploadAppMedia)
	group.GET("/upload-channel", requireCallerAuth, getUploadChannelStatus)
}

// auth helpers -------------------------------------------------------------

func requireCallerAuth(c *gin.Context) { callerAuthRequired(c) }
func requireAdminAuth(c *gin.Context)  { adminAuthRequired(c) }

// ctx wraps to avoid import cycle in tests.
func contextFromGin(c *gin.Context) context.Context { return c.Request.Context() }

// notYetImplemented is the placeholder answer for plugin features that are
// routed but not wired yet; it keeps the route tree compiling while telling the
// caller exactly what is missing.
func notYetImplemented(c *gin.Context, feature string) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "NOT_IMPLEMENTED",
		"feature": feature,
		"plugin":  pluginName,
	})
}

// avoid-import lint: keep common package ref so future JSON writes are ready.
var _ = common.Marshal
var _ = fmt.Sprintf
var _ = strings.TrimSpace
