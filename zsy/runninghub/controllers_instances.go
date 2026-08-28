package runninghub

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// Admin controllers — Instance CRUD + keypool refresh
// =========================================================================

// parseInstanceListQuery translates gin query strings into InstanceListQuery,
// mirroring the app list conventions (p / page_size / snake_case filters).
func parseInstanceListQuery(c *gin.Context) InstanceListQuery {
	p, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	q := InstanceListQuery{
		Page:      p,
		PageSize:  pageSize,
		SortBy:    strings.TrimSpace(c.DefaultQuery("sort_by", "id")),
		SortOrder: strings.TrimSpace(c.DefaultQuery("sort_order", "desc")),
	}
	if s := c.Query("app_id"); s != "" {
		if v, err := strconv.ParseUint(s, 10, 64); err == nil {
			appID := uint(v)
			q.AppID = &appID
		}
	}
	if s := c.Query("channel_id"); s != "" {
		if v, err := strconv.ParseInt(s, 10, 64); err == nil {
			q.ChannelID = &v
		}
	}
	if s := c.Query("enabled"); s != "" {
		v := s == "1" || strings.EqualFold(s, "true")
		q.Enabled = &v
	}
	return q
}

// listInstances (GET /dashboard/zsy/rh/instances).
func listInstances(c *gin.Context) {
	result, err := InstanceSearch(parseInstanceListQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// createInstance (POST /dashboard/zsy/rh/instances).
func createInstance(c *gin.Context) {
	var dto InstanceCreateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiError(c, fmt.Errorf("请求体错误: %w", err))
		return
	}
	view, err := InstanceInsert(&dto)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_instance.create", map[string]any{
		"id":        view.ID,
		"appId":     view.AppID,
		"channelId": view.ChannelID,
	})
	common.ApiSuccess(c, view)
}

// updateInstance (PUT /dashboard/zsy/rh/instances/:id).
func updateInstance(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var dto InstanceUpdateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiError(c, fmt.Errorf("请求体错误: %w", err))
		return
	}
	view, err := InstanceUpdate(id, &dto)
	if err != nil {
		if errors.Is(err, ErrInstanceNotFound) {
			common.ApiErrorMsg(c, "实例不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_instance.update", map[string]any{
		"id":        view.ID,
		"appId":     view.AppID,
		"channelId": view.ChannelID,
	})
	common.ApiSuccess(c, view)
}

// deleteInstance (DELETE /dashboard/zsy/rh/instances/:id).
func deleteInstance(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	deletedID, err := InstanceDelete(id)
	if err != nil {
		if errors.Is(err, ErrInstanceNotFound) {
			common.ApiErrorMsg(c, "实例不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_instance.delete", map[string]any{"id": deletedID})
	common.ApiSuccess(c, map[string]any{"deleted": true, "id": deletedID})
}

// refreshKeypool (POST /dashboard/zsy/rh/instances/:id/keypool-refresh) —
// syncs the pool key list from the bound channel and reconciles in-flight
// submit audits against terminal task states.
func refreshKeypool(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	result, err := InstanceSyncKeypool(id)
	if err != nil {
		if errors.Is(err, ErrInstanceNotFound) {
			common.ApiErrorMsg(c, "实例不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_instance.keypool_refresh", map[string]any{
		"id":              id,
		"keysAdded":       result.KeysAdded,
		"keysDisabled":    result.KeysDisabled,
		"keysRestored":    result.KeysRestored,
		"pendingReleased": result.PendingReleased,
	})
	common.ApiSuccess(c, result)
}
