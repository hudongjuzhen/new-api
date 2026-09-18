package runninghub

import (
	"context"

	"github.com/QuantumNous/new-api/constant"
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

// TestHookSiteToChannelType exercises the site→channel-type mapping table used
// by submitAppRun / selectChannelBySiteType. Exported so the routing decision
// (site=cn ↔ type 61, site=intl ↔ type 62) is pinned in a unit test.
func TestHookSiteToChannelType(t interface {
	Errorf(format string, args ...any)
	Helper()
}) {
	if got := siteToChannelType("cn"); got != constant.ChannelTypeRunningHub {
		t.Errorf("siteToChannelType(cn)=%d, want %d", got, constant.ChannelTypeRunningHub)
	}
	if got := siteToChannelType("intl"); got != constant.ChannelTypeRunningHubIntl {
		t.Errorf("siteToChannelType(intl)=%d, want %d", got, constant.ChannelTypeRunningHubIntl)
	}
}

// TestHookSiteToChannelTypeValue returns the channel type for a site value,
// mirroring the first decision of selectChannelBySiteType.
func TestHookSiteToChannelTypeValue(site string) int {
	return siteToChannelType(site)
}

// TestHookGetPublicAppDetail exposes the user-side detail handler for tests.
func TestHookGetPublicAppDetail(c *gin.Context) { getPublicAppDetail(c) }

// TestHookSubmitAppRun exposes the user-side submit handler for tests.
func TestHookSubmitAppRun(c *gin.Context) { submitAppRun(c) }

// TestHookCancelAppTask exposes the user-side cancel handler for tests.
func TestHookCancelAppTask(c *gin.Context) { cancelAppTask(c) }

// TestHookGetAppTaskResult exposes the user-side task query handler for tests.
func TestHookGetAppTaskResult(c *gin.Context) { getAppTaskResult(c) }

// TestHookGetTaskResultContent exposes the result-file preview proxy for tests.
func TestHookGetTaskResultContent(c *gin.Context) { getTaskResultContent(c) }

// TestHookUploadAppMedia exposes the user-side upload proxy handler for tests.
func TestHookUploadAppMedia(c *gin.Context) { uploadAppMedia(c) }

// TestHookMountUserRoutes mounts the plugin's user-facing route group with the
// real authentication middleware chain, so the API-key (third-party) path can
// be exercised end to end — including credential classification, token scoping
// and the response envelope.
func TestHookMountUserRoutes(router *gin.Engine) {
	userRoutes(router.Group("/api/zsy/rh"))
}

// TestHookMarkControllerResponds mirrors the marker submitAppRun sets, so tests
// can pin the adaptor's "the run handler already answered" behaviour.
func TestHookMarkControllerResponds(c *gin.Context) { c.Set(contextKeyControllerResponds, true) }

// TestHookValidateRunPayload runs the in-memory validation path used by
// submitAppRun without hitting the relay pipeline. Exported for table-driven
// unit tests covering the typed schema validator.
func TestHookValidateRunPayload(schema []rhparser.SchemaParam, values map[string]any) error {
	_, err := validateAndBuildNodeInfoList(schema, values)
	return err
}

// --- Concurrency gate hooks ------------------------------------------------

// TestHookReserveSlotOnce grants at most one concurrency slot on the enabled
// channels of a site type. It returns the granted channel id (0 when every
// channel of the site is saturated) plus the release function the caller now
// owns. Repeated calls model concurrent submits of the same process.
func TestHookReserveSlotOnce(channelType int, group string) (int, func(), error) {
	candidates, err := siteCandidates(channelType, group)
	if err != nil {
		return 0, nil, err
	}
	channel, slot, err := reserveSlot(candidates, 0)
	if err != nil {
		return 0, nil, err
	}
	if channel == nil {
		return 0, func() {}, nil
	}
	return channel.Id, slot.release, nil
}

// TestHookQueuedTaskCount returns how many tasks currently wait in the queue.
func TestHookQueuedTaskCount() (int64, error) { return queuedTaskCount() }

// TestHookDispatchQueue runs one dispatcher pass synchronously.
func TestHookDispatchQueue() { dispatchQueuedTasks(context.Background()) }

// TestHookSetQueueMaxAgeMinutes shrinks the queue-expiry window so the expiry
// test does not have to wait 12 hours. It returns the previous value.
func TestHookSetQueueMaxAgeMinutes(minutes int) int {
	previous := queueMaxAgeMinutes
	queueMaxAgeMinutes = minutes
	return previous
}

// TestHookSubmitPathFor exposes the submit-path composition used both by the
// adaptor and by the queued dispatcher.
func TestHookSubmitPathFor(kind AppKind, upstreamID string) (string, bool) {
	return submitPathFor(kind, upstreamID)
}
