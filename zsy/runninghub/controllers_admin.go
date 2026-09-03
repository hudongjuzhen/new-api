package runninghub

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/gin-gonic/gin"
)

// ---------------------------------------------------------------------------
// Query helpers — translate gin query strings into the typed AppListQuery
// DTO.  Mirror the host's SearchChannels conventions: `p` for page,
// `page_size` for pageSize, snake_case for filters.
// ---------------------------------------------------------------------------

func parseListQuery(c *gin.Context) AppListQuery {
	p, _ := strconv.Atoi(c.DefaultQuery("p", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	kind := strings.TrimSpace(c.Query("kind"))
	keyword := strings.TrimSpace(c.Query("keyword"))
	sortBy := strings.TrimSpace(c.DefaultQuery("sort_by", "id"))
	sortOrder := strings.TrimSpace(c.DefaultQuery("sort_order", "desc"))

	q := AppListQuery{
		Keyword:   keyword,
		Kind:      kind,
		Page:      p,
		PageSize:  pageSize,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}
	if s := c.Query("published"); s != "" {
		v := s == "1" || strings.EqualFold(s, "true")
		q.Published = &v
	}
	if s := c.Query("admin_only"); s != "" {
		v := s == "1" || strings.EqualFold(s, "true")
		q.AdminOnly = &v
	}
	return q
}

func parseAppIDParam(c *gin.Context, name string) (uint, error) {
	s := strings.TrimSpace(c.Param(name))
	if s == "" {
		return 0, fmt.Errorf("缺少 %s 参数", name)
	}
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("非法 %s 参数: %w", name, err)
	}
	return uint(id), nil
}

// =========================================================================
// Admin controllers — App CRUD
// =========================================================================

// listApps (GET /dashboard/zsy/rh/apps) — pageable app list.
func listApps(c *gin.Context) {
	q := parseListQuery(c)
	result, err := AppSearch(q)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// getApp (GET /dashboard/zsy/rh/apps/:id) — single app detail.
func getApp(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	view, err := AppGetByID(id)
	if err != nil {
		if errors.Is(err, ErrAppNotFound) {
			common.ApiErrorMsg(c, "应用不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

// createApp (POST /dashboard/zsy/rh/apps) — create one app.
//
// Accepts a JSON body matching AppCreateDTO. When the payload contains no
// explicit schema but does carry a `curl` string, the endpoint refuses to
// silently proceed — the caller must first hit /parse-curl and then submit
// the decided schema. This guards the admin UI against creating apps with
// mismatched (nodeId, fieldName) pairs.
func createApp(c *gin.Context) {
	var payload struct {
		AppCreateDTO
		Curl string `json:"curl,omitempty"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, fmt.Errorf("请求体错误: %w", err))
		return
	}
	if strings.TrimSpace(payload.Curl) != "" {
		common.ApiErrorMsg(c, "请先通过 /parse-curl 端点解析 curl，再将输出填入 paramSchema 提交")
		return
	}
	view, err := AppInsert(&payload.AppCreateDTO)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_app.create", map[string]any{
		"id":         view.ID,
		"name":       view.Name,
		"kind":       view.Kind,
		"upstreamId": view.UpstreamID,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    view,
	})
}

// updateApp (PUT /dashboard/zsy/rh/apps/:id) — update app by id.
func updateApp(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var dto AppUpdateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiError(c, fmt.Errorf("请求体错误: %w", err))
		return
	}
	view, err := AppUpdate(id, &dto)
	if err != nil {
		if errors.Is(err, ErrAppNotFound) {
			common.ApiErrorMsg(c, "应用不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_app.update", map[string]any{
		"id":         view.ID,
		"name":       view.Name,
		"kind":       view.Kind,
		"upstreamId": view.UpstreamID,
	})
	common.ApiSuccess(c, view)
}

// deleteApp (DELETE /dashboard/zsy/rh/apps/:id) — soft-delete app.
func deleteApp(c *gin.Context) {
	id, err := parseAppIDParam(c, "id")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	name, err := AppDelete(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_app.delete", map[string]any{
		"id":   id,
		"name": name,
	})
	common.ApiSuccess(c, map[string]any{"deleted": true, "name": name})
}

// =========================================================================
// Curl parser endpoint
// =========================================================================

type parseCurlRequest struct {
	Curl   string                 `json:"curl"`
	Schema []rhparser.SchemaParam `json:"schema,omitempty"` // optional admin edits merged in
}

// parseCurlEndpoint calls rhparser.ParseCurl and drafts a schema.
//
// When Schema is provided on the request (i.e. caller is refining a prior
// parse), the parser re-validates the nodes list and returns the combined
// report alongside a freshly built ParseCurlResponse. Error responses
// deliberately use 200 OK + success:false so the admin form can render the
// message inline.
func parseCurlEndpoint(c *gin.Context) {
	var req parseCurlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, fmt.Errorf("请求体错误: %w", err))
		return
	}
	curl := strings.TrimSpace(req.Curl)
	if curl == "" {
		common.ApiErrorMsg(c, "curl 内容不能为空")
		return
	}
	parsed, err := rhparser.ParseCurl(curl)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// Merge explicit schema: if the admin sent schema in (edits after a
	// previous parse), prefer it for the "schema" output but still run
	// BuildSchemaFromNodes on the raw parsed node list to compute the
	// warnings/errors report.
	var summary rhparser.SchemaSummary
	if len(req.Schema) > 0 {
		summary.Params = append(summary.Params, req.Schema...)
		reported := rhparser.BuildSchemaFromNodes(parsed.NodeInfoList)
		summary.Errors = reported.Errors
	} else {
		summary = rhparser.BuildSchemaFromNodes(parsed.NodeInfoList)
	}
	out := ParseCurlResponse{
		Kind:         parsed.Kind,
		UpstreamID:   parsed.UpstreamID,
		BaseURL:      parsed.BaseURL,
		AppName:      rhparser.CurlAppName(&parsed),
		NodeInfoList: parsed.NodeInfoList,
		Schema:       summary.Params,
		Errors:       summary.Errors,
	}
	recordManageAudit(c, "rh_app.parse_curl", map[string]any{
		"kind":       out.Kind,
		"upstreamId": out.UpstreamID,
		"nodes":      len(out.NodeInfoList),
		"errors":     len(out.Errors),
	})
	common.ApiSuccess(c, out)
}

// =========================================================================
// Sync-from-channel
// =========================================================================

// syncAppsFromChannel (POST /dashboard/zsy/rh/apps/sync-from-channel) —
// bidirectional reconciliation between the RunningHub channel's model
// registry and the plugin's apps/instances. The store layer (SyncChannelApps)
// documents the exact semantics; RH has no upstream "list apps" API, so the
// channel's own models list is the source of truth.
func syncAppsFromChannel(c *gin.Context) {
	var payload struct {
		ChannelID int64 `json:"channelId"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		common.ApiError(c, fmt.Errorf("请求体错误: %w", err))
		return
	}
	result, err := SyncChannelApps(payload.ChannelID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rh_app.sync_from_channel", map[string]any{
		"channelId":            payload.ChannelID,
		"appsBound":            result.AppsBound,
		"modelsSynced":         result.ModelsSynced,
		"channelModelsUpdated": result.ChannelModelsUpdated,
	})
	common.ApiSuccess(c, result)
}

// =========================================================================
// Admin audit helper
// =========================================================================

// recordManageAudit records a management audit log entry. Since the
// runninghub plugin cannot import the unexported host helper by name, we
// replicate its signature here and delegate by looking up the controller
// package's exported shim. If unavailable we fall back to a no-op.
//
// NOTE: The host-level controller package exposes the function as
// `recordManageAudit(c, action, params)` (unexported). The plugin cannot
// call it directly. Instead we emit a lightweight entry through the public
// service layer (if available) and otherwise treat it as best-effort.
func recordManageAudit(c *gin.Context, action string, params map[string]any) {
	if c == nil {
		return
	}
	// Best-effort: forward to the controller-level shim via a public wrapper.
	// If the wrapper is not linked in (tests), ignore.
	if auditLogFunc != nil {
		auditLogFunc(c, action, params)
	}
}

// auditLogFunc can be set by an init in the host controller package. The
// plugin currently runs a no-op; when a public wrapper is added later this
// variable remains the single injection point.
var auditLogFunc func(c *gin.Context, action string, params map[string]any)
