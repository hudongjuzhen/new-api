package runninghub

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// authRequired simply returns the authentication wrapper. We keep this file
// small: it only wires route groups. Controller handlers live in
// controller_rh_*.go files.

// mountRoutes attaches the plugin's gin route groups. It is registered with
// extcore and invoked once from router.SetRouter after the core routes are
// installed.
//
// Route layout:
//
//	/api/zsy/rh/app          (user-side)  Run an app via keypool, fetch results
//	/dashboard/zsy/rh/app    (admin)     App CRUD, instance management, curl import
//
// NOTE: These routes intentionally sit *outside* the core router groups so the
// plugin does not take hard dependency on the internals of
// SetApiRouter/SetDashboardRouter. Admin and user auth are guarded with
// existing middlewares looked up by name via controller helpers.
func mountRoutes(router *gin.Engine) {
	api := router.Group("/api/zsy/rh")
	{
		apps := api.Group("/apps")
		{
			apps.GET("", listPublicApps)
			apps.GET("/:id", getPublicAppDetail)
			apps.POST("/:id/run", requireUserAuth, submitAppRun)
			apps.GET("/task/:task_id", requireUserAuth, getAppTaskResult)
		}
	}

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
			apps.POST("/sync-from-channel", syncAppsFromChannel)
		}
		instances := admin.Group("/instances")
		{
			instances.GET("", listInstances)
			instances.POST("", createInstance)
			instances.PUT("/:id", updateInstance)
			instances.DELETE("/:id", deleteInstance)
			instances.POST("/:id/keypool-refresh", refreshKeypool)
		}
		admin.GET("/stats", stats)
	}
}

// auth helpers -------------------------------------------------------------
//
// We do not import the auth middleware directly to avoid a hard coupling with
// new-api's internal authz layering; instead we re-use the public
// service.UserAuthRequired middleware pattern via the same gin handler chain.
// These helpers are thin wrappers; they live in auth_helpers.go with the real
// implementation.

func requireUserAuth(c *gin.Context)  { userAuthRequired(c) }
func requireAdminAuth(c *gin.Context) { adminAuthRequired(c) }

// ctx wraps to avoid import cycle in tests.
func contextFromGin(c *gin.Context) context.Context { return c.Request.Context() }

// ---------------------------------------------------------------------------
// Controller stubs (return 501 NOT_IMPLEMENTED until the corresponding
// feature is wired in follow-up steps). This keeps the route tree compiling
// and the build green; callers will see a clear error message.
// ---------------------------------------------------------------------------

func notYetImplemented(c *gin.Context, feature string) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"error":   "NOT_IMPLEMENTED",
		"feature": feature,
		"plugin":  pluginName,
	})
}

// User-side handlers are implemented in controllers_user.go
// (listPublicApps, getPublicAppDetail, submitAppRun, getAppTaskResult).
// Instance CRUD + keypool-refresh admin handlers are implemented in
// controllers_instances.go; stats and the channel sync store layer live in
// stats_sync.go. They are NOT re-stubbed here to avoid redeclaring the same
// symbols.

// avoid-import lint: keep common package ref so future JSON writes are ready.
var _ = common.Marshal
var _ = fmt.Sprintf
var _ = strings.TrimSpace
