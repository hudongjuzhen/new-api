package runninghub

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// Admin controllers — app-level keypool view / refresh
// =========================================================================

// getAppKeypool (GET /dashboard/zsy/rh/apps/:id/keypool) — returns the app's
// current keypool state (masked keys, enabled flags, per-key occupancy) plus
// the bound channel that sources the keys.
func getAppKeypool(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := AppKeypoolStatus(id)
	if err != nil {
		if errors.Is(err, ErrAppNotFound) {
			common.ApiErrorMsg(c, "应用不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// refreshAppKeypool (POST /dashboard/zsy/rh/apps/:id/keypool-refresh) — syncs
// the app's pool key list from its bound channel and reconciles in-flight
// submit audits against terminal task states.
func refreshAppKeypool(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := AppSyncKeypool(id)
	if err != nil {
		if errors.Is(err, ErrAppNotFound) {
			common.ApiErrorMsg(c, "应用不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_app.keypool_refresh", map[string]any{
		"id":              id,
		"keysAdded":       result.KeysAdded,
		"keysDisabled":    result.KeysDisabled,
		"keysRestored":    result.KeysRestored,
		"pendingReleased": result.PendingReleased,
	})
	common.ApiSuccess(c, result)
}
