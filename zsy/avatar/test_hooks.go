package avatar

import "github.com/gin-gonic/gin"

// Test hooks. The handlers themselves stay unexported so the plugin's public
// surface is only its routes; these thin wrappers let the package's own tests
// mount production handlers on a bare gin engine, without the auth middlewares
// (which need a real session) and without the host's rate limiters.

// TestHookListPublicAvatars exposes listPublicAvatars.
func TestHookListPublicAvatars(c *gin.Context) { listPublicAvatars(c) }

// TestHookGetPublicAvatar exposes getPublicAvatar.
func TestHookGetPublicAvatar(c *gin.Context) { getPublicAvatar(c) }

// TestHookListAvatarsAdmin exposes listAvatarsAdmin.
func TestHookListAvatarsAdmin(c *gin.Context) { listAvatarsAdmin(c) }

// TestHookCreateAvatar exposes createAvatar.
func TestHookCreateAvatar(c *gin.Context) { createAvatar(c) }

// TestHookGetAvatar exposes getAvatar.
func TestHookGetAvatar(c *gin.Context) { getAvatar(c) }

// TestHookUpdateAvatar exposes updateAvatar.
func TestHookUpdateAvatar(c *gin.Context) { updateAvatar(c) }

// TestHookDeleteAvatar exposes deleteAvatar.
func TestHookDeleteAvatar(c *gin.Context) { deleteAvatar(c) }

// TestHookExportAvatars exposes exportAvatars.
func TestHookExportAvatars(c *gin.Context) { exportAvatars(c) }

// TestHookImportAvatars exposes importAvatars.
func TestHookImportAvatars(c *gin.Context) { importAvatars(c) }

// TestHookMountRoutes mounts the plugin's real route tree (public + admin groups
// with their middlewares) on a bare engine, so a test can pin the published URLs
// and prove the registration itself does not panic.
func TestHookMountRoutes(router *gin.Engine) { mountRoutes(router) }

// TestHookCatalogVoiceSamples exposes the voice-catalog lookup so a test can
// assert the SQL projection against a real zsy_voices table.
func TestHookCatalogVoiceSamples(voiceIDs []string) (map[string]VoiceSample, error) {
	return catalogVoiceSamples(voiceIDs)
}
