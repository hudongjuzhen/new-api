package tone

import "github.com/gin-gonic/gin"

// Test hooks. The handlers themselves stay unexported so the plugin's public
// surface is only its routes; these thin wrappers let the package's own tests
// mount production handlers on a bare gin engine, without the auth middlewares
// (which need a real session) and without the host's rate limiters.

// TestHookListPublicTones exposes listPublicTones.
func TestHookListPublicTones(c *gin.Context) { listPublicTones(c) }

// TestHookGetPublicTone exposes getPublicTone.
func TestHookGetPublicTone(c *gin.Context) { getPublicTone(c) }

// TestHookGetToneStandard exposes getToneStandard.
func TestHookGetToneStandard(c *gin.Context) { getToneStandard(c) }

// TestHookListTonesAdmin exposes listTonesAdmin.
func TestHookListTonesAdmin(c *gin.Context) { listTonesAdmin(c) }

// TestHookCreateTone exposes createTone.
func TestHookCreateTone(c *gin.Context) { createTone(c) }

// TestHookGetTone exposes getTone.
func TestHookGetTone(c *gin.Context) { getTone(c) }

// TestHookUpdateTone exposes updateTone.
func TestHookUpdateTone(c *gin.Context) { updateTone(c) }

// TestHookDeleteTone exposes deleteTone.
func TestHookDeleteTone(c *gin.Context) { deleteTone(c) }

// TestHookExportTones exposes exportTones.
func TestHookExportTones(c *gin.Context) { exportTones(c) }

// TestHookImportTones exposes importTones.
func TestHookImportTones(c *gin.Context) { importTones(c) }

// TestHookMountRoutes mounts the plugin's real route tree (public + admin
// groups with their middlewares) on a bare engine, so a test can pin the
// published URLs and prove the registration itself does not panic.
func TestHookMountRoutes(router *gin.Engine) { mountRoutes(router) }
