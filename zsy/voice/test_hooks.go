package voice

import "github.com/gin-gonic/gin"

// Test hooks. The handlers themselves stay unexported so the plugin's public
// surface is only its routes; these thin wrappers let the package's own tests
// mount production handlers on a bare gin engine, without the auth middlewares
// (which need a real session) and without the host's rate limiters.

// TestHookListPublicVoices exposes listPublicVoices.
func TestHookListPublicVoices(c *gin.Context) { listPublicVoices(c) }

// TestHookGetPublicVoice exposes getPublicVoice.
func TestHookGetPublicVoice(c *gin.Context) { getPublicVoice(c) }

// TestHookListVoicesAdmin exposes listVoicesAdmin.
func TestHookListVoicesAdmin(c *gin.Context) { listVoicesAdmin(c) }

// TestHookCreateVoice exposes createVoice.
func TestHookCreateVoice(c *gin.Context) { createVoice(c) }

// TestHookGetVoice exposes getVoice.
func TestHookGetVoice(c *gin.Context) { getVoice(c) }

// TestHookUpdateVoice exposes updateVoice.
func TestHookUpdateVoice(c *gin.Context) { updateVoice(c) }

// TestHookDeleteVoice exposes deleteVoice.
func TestHookDeleteVoice(c *gin.Context) { deleteVoice(c) }

// TestHookUploadVoiceAudio exposes uploadVoiceAudio.
func TestHookUploadVoiceAudio(c *gin.Context) { uploadVoiceAudio(c) }

// TestHookExportVoices exposes exportVoices.
func TestHookExportVoices(c *gin.Context) { exportVoices(c) }

// TestHookImportVoices exposes importVoices.
func TestHookImportVoices(c *gin.Context) { importVoices(c) }

// TestHookMountRoutes mounts the plugin's real route tree (public + admin
// groups with their middlewares) on a bare engine, so a test can pin the
// published URLs and prove the registration itself does not panic.
func TestHookMountRoutes(router *gin.Engine) { mountRoutes(router) }
