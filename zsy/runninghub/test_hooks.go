package runninghub

import (
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/gin-gonic/gin"
)

// TestHookParseCurl is exported for the plugin's own test suite only. It
// forwards to parseCurlEndpoint, which is the canonical admin handler.
func TestHookParseCurl(c *gin.Context) { parseCurlEndpoint(c) }

// TestHookCreateApp exposes the createApp admin handler for tests.
func TestHookCreateApp(c *gin.Context) { createApp(c) }

// TestHookGetApp exposes the getApp admin handler for tests.
func TestHookGetApp(c *gin.Context) { getApp(c) }

// TestHookUpdateApp exposes updateApp for tests.
func TestHookUpdateApp(c *gin.Context) { updateApp(c) }

// TestHookDeleteApp exposes deleteApp for tests.
func TestHookDeleteApp(c *gin.Context) { deleteApp(c) }

// TestHookListApps exposes listApps for tests.
func TestHookListApps(c *gin.Context) { listApps(c) }

// TestHookSyncApps exposes syncAppsFromChannel for tests.
func TestHookSyncApps(c *gin.Context) { syncAppsFromChannel(c) }

// TestHookValidateAppCreate runs the same DTO->App translation + validator
// used by AppInsert / AppUpdate but returns the validation error without
// touching the database. Exported for unit tests that exercise the validator
// table drive.
func TestHookValidateAppCreate(dto *AppCreateDTO) error {
	app, err := applyDto(dto, nil)
	if err != nil {
		return err
	}
	return validateApp(app)
}

// --- User-side hooks -------------------------------------------------------

// TestHookListPublicApps exposes the user-side list handler for tests.
func TestHookListPublicApps(c *gin.Context) { listPublicApps(c) }

// TestHookGetPublicAppDetail exposes the user-side detail handler for tests.
func TestHookGetPublicAppDetail(c *gin.Context) { getPublicAppDetail(c) }

// TestHookSubmitAppRun exposes the user-side submit handler for tests.
func TestHookSubmitAppRun(c *gin.Context) { submitAppRun(c) }

// TestHookGetAppTaskResult exposes the user-side task query handler for tests.
func TestHookGetAppTaskResult(c *gin.Context) { getAppTaskResult(c) }

// TestHookUploadAppMedia exposes the user-side upload proxy handler for tests.
func TestHookUploadAppMedia(c *gin.Context) { uploadAppMedia(c) }

// TestHookValidateRunPayload runs the in-memory validation path used by
// submitAppRun without hitting the relay pipeline. Exported for table-driven
// unit tests covering the typed schema validator.
func TestHookValidateRunPayload(schema []rhparser.SchemaParam, values map[string]any) error {
	_, err := validateAndBuildNodeInfoList(schema, values)
	return err
}

// --- App keypool admin hooks ----------------------------------------------

// TestHookGetAppKeypool exposes the getAppKeypool handler for tests.
func TestHookGetAppKeypool(c *gin.Context) { getAppKeypool(c) }

// TestHookRefreshAppKeypool exposes the refreshAppKeypool handler for tests.
func TestHookRefreshAppKeypool(c *gin.Context) { refreshAppKeypool(c) }
