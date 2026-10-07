package world

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// Admin face — /dashboard/zsy/world/** behind middleware.AdminAuth
//
// docs/23 §7 requires four things of this face; step 1 ships the one the
// entitlement contract cannot live without:
//
//	能力授予 / 撤销 / 延期   ← shipped here (enforcement criterion C)
//	世界列表 / 详情 / 版本历史 ← read-only, shipped here (operators must be able
//	                            to answer "这个用户的世界怎么了")
//	兑换码                   ← not shipped: new-api already has a redemption-code
//	                            system, and docs/23 §7 says to reuse it rather
//	                            than build a second one
//	用量统计                 ← not shipped: there is nothing to meter until
//	                            ingest.run bills (step 4)
//
// This face answers with the host's dashboard envelope (common.ApiSuccess /
// common.ApiErrorMsg) so the admin page reuses the host's axios interceptor and
// toast handling unchanged — the same choice zsy/voice made.
// =========================================================================

// listAdminProjects (GET /dashboard/zsy/world/projects?user_id=…&page=…)
//
// Documents are never listed here: a listing that carried documents would ship
// megabytes per page. Detail is a separate, explicit call.
func listAdminProjects(c *gin.Context) {
	userID, err := parseOptionalIntQuery(c, "user_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	page, pageSize := parsePagination(c)

	var rows []WorldProject
	var total int64
	if userID == 0 {
		base := db().Model(&WorldProject{})
		if err := base.Count(&total).Error; err != nil {
			common.ApiError(c, err)
			return
		}
		if err := base.Order("id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rows).Error; err != nil {
			common.ApiError(c, err)
			return
		}
	} else {
		var listErr error
		rows, total, listErr = WorldProjectListByUser(userID, (page-1)*pageSize, pageSize)
		if listErr != nil {
			common.ApiError(c, listErr)
			return
		}
	}

	common.ApiSuccess(c, gin.H{
		"items":    rows,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

// getAdminProject (GET /dashboard/zsy/world/projects/:id) — one project plus its
// version history, so an operator can see how it got to its current state
// without downloading every version.
func getAdminProject(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	project, err := WorldProjectGet(id)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			common.ApiErrorMsg(c, fmt.Sprintf("世界项目 %d 不存在", id))
			return
		}
		common.ApiError(c, err)
		return
	}
	history, err := WorldSnapshotHistory(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"project": project,
		"history": history,
	})
}

// getAdminProjectVersion (GET /dashboard/zsy/world/projects/:id/versions/:version)
// — one stored document, verbatim. This is the operator's evidence when a user
// reports "我的世界坏了".
func getAdminProjectVersion(c *gin.Context) {
	id, err := parseUintParam(c, "id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	version, err := parseVersionParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	snapshot, err := WorldSnapshotGet(id, version)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) {
			common.ApiErrorMsg(c, fmt.Sprintf("世界项目 %d 没有第 %d 版文档", id, version))
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"projectId": snapshot.ProjectID,
		"version":   snapshot.Version,
		"reason":    snapshot.Reason,
		"createdAt": snapshot.CreatedAt,
		"doc":       snapshot.Doc,
	})
}

// listAdminEntitlements (GET /dashboard/zsy/world/entitlements?user_id=…)
//
// It lists every row of a user — live, expired and revoked — because the
// operator's question is usually "他到底有没有 / 什么时候没的", and a face that
// only showed live rows could not answer it.
func listAdminEntitlements(c *gin.Context) {
	userID, err := parseOptionalIntQuery(c, "user_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if userID <= 0 {
		common.ApiErrorMsg(c, "请提供 user_id：能力是挂在账号上的，不接受全表列举")
		return
	}
	rows, err := WorldEntitlementListByUser(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// The live verdict is recomputed rather than inferred from the rows above, so
	// the admin screen shows exactly what an op would see right now.
	live, err := WorldCapabilitiesActive(userID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"userId": userID,
		"items":  rows,
		"active": live,
	})
}

// grantEntitlement (POST /dashboard/zsy/world/entitlements/grant)
//
// It takes effect on the very next op because nothing caches the verdict
// (docs/23 §8.4: "每次 ops 现算，不是登录时算一次存本地").
func grantEntitlement(c *gin.Context) {
	var in adminGrantParams
	if err := c.ShouldBindJSON(&in); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	var expiresAt *int64
	if in.ExpiresAt > 0 {
		value := in.ExpiresAt
		expiresAt = &value
	} else if in.ExpiresAt < 0 {
		common.ApiErrorMsg(c, "expires_at 必须是正的 unix 秒；不填表示永不过期")
		return
	}

	row, err := WorldEntitlementGrant(WorldEntitlementGrantInput{
		UserID:     in.UserID,
		Capability: strings.TrimSpace(in.Capability),
		Source:     strings.TrimSpace(in.Source),
		ExpiresAt:  expiresAt,
	})
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-world] granted capability=%q to user=%d source=%q expiresAt=%v by admin",
		row.Capability, row.UserID, row.Source, row.ExpiresAt))
	common.ApiSuccess(c, row)
}

// revokeEntitlement (POST /dashboard/zsy/world/entitlements/revoke)
//
// "卖了要能收回" (docs/23 §8.4). A zero rows-affected count is reported as a
// failure: revoking something that was never live is an operator mistake worth
// surfacing, not a silent success.
func revokeEntitlement(c *gin.Context) {
	var in adminRevokeParams
	if err := c.ShouldBindJSON(&in); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	capability := strings.TrimSpace(in.Capability)
	if in.UserID <= 0 || capability == "" {
		common.ApiErrorMsg(c, "需要 user_id 与 capability")
		return
	}

	affected, err := WorldEntitlementRevoke(in.UserID, capability)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if affected == 0 {
		common.ApiErrorMsg(c, fmt.Sprintf("账号 %d 当前没有生效中的 %q 能力：没有可撤销的行", in.UserID, capability))
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-world] revoked capability=%q from user=%d rows=%d by admin",
		capability, in.UserID, affected))
	common.ApiSuccess(c, gin.H{"userId": in.UserID, "capability": capability, "revoked": affected})
}

// ---------------------------------------------------------------------------
// Admin parameter helpers
// ---------------------------------------------------------------------------

// parsePagination bounds page / page_size the way zsy/voice does: out-of-range
// values are normalized, not rejected, so the two admin lists in this codebase
// behave the same.
func parsePagination(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page", "1")))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(strings.TrimSpace(c.DefaultQuery("page_size", "")))
	if pageSize < 1 {
		pageSize = cfg.DefaultPageSize
	}
	if pageSize > cfg.MaxPageSize {
		pageSize = cfg.MaxPageSize
	}
	return page, pageSize
}

// parseUintParam reads a positive integer path parameter.
func parseUintParam(c *gin.Context, name string) (uint, error) {
	raw := strings.TrimSpace(c.Param(name))
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || value == 0 {
		return 0, fmt.Errorf("%s 必须是正整数，实际为 %q", name, raw)
	}
	return uint(value), nil
}

// parseOptionalIntQuery reads an optional integer query parameter; absent or
// empty answers 0.
func parseOptionalIntQuery(c *gin.Context, name string) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数，实际为 %q", name, raw)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s 不能为负数", name)
	}
	return value, nil
}

// parseVersionParam reads the :version path parameter.
func parseVersionParam(c *gin.Context) (int64, error) {
	raw := strings.TrimSpace(c.Param("version"))
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("version 必须是正整数，实际为 %q", raw)
	}
	return value, nil
}
