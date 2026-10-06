package avatar

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// Public controllers — the read-only face of the plaza
//
// They answer with a real HTTP status plus a stable machine-readable code;
// third-party callers branch on the code, never on the localized message.
// =========================================================================

// Stable machine-readable codes for the public face.
const (
	codeInvalidParams  = "INVALID_PARAMS"
	codeAvatarNotFound = "AVATAR_NOT_FOUND"
	codeDatabaseError  = "DATABASE_ERROR"
)

// respondPublicError answers a third-party caller with a real HTTP status plus a
// stable code, keeping the {success, message} shape the dashboard uses.
func respondPublicError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": message})
}

// listPublicAvatars (GET /api/zsy/avatar/list) — the paginated persona catalog,
// with every persona's linked voice sample already resolved.
//
// Only on-shelf (enabled) personas are returned: the `enabled` query parameter is
// ignored on this face, it belongs to the admin list.
func listPublicAvatars(c *gin.Context) {
	q := parseAvatarListQuery(c)
	enabled := true
	q.Enabled = &enabled

	result, err := AvatarSearch(q, nil)
	if err != nil {
		common.SysError("[zsy-avatar] public list failed: " + err.Error())
		respondPublicError(c, http.StatusInternalServerError, codeDatabaseError, "读取形象列表失败")
		return
	}
	common.ApiSuccess(c, result)
}

// getPublicAvatar (GET /api/zsy/avatar/:id) — one on-shelf persona. An off-shelf
// persona answers 404 exactly like a missing one, so the catalog never leaks
// unpublished drafts.
func getPublicAvatar(c *gin.Context) {
	id, err := parseAvatarIDParam(c)
	if err != nil {
		respondPublicError(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	view, err := AvatarGetByID(id, nil)
	if err != nil {
		if errors.Is(err, ErrAvatarNotFound) {
			respondPublicError(c, http.StatusNotFound, codeAvatarNotFound, "形象不存在")
			return
		}
		common.SysError("[zsy-avatar] public detail failed: " + err.Error())
		respondPublicError(c, http.StatusInternalServerError, codeDatabaseError, "读取形象失败")
		return
	}
	if !view.Enabled {
		respondPublicError(c, http.StatusNotFound, codeAvatarNotFound, "形象不存在或已下架")
		return
	}
	common.ApiSuccess(c, view)
}

// =========================================================================
// Admin controllers — persona CRUD behind /dashboard/zsy/avatar
//
// They answer with the dashboard envelope ({success, message, data}) so the admin
// page reuses the host's axios interceptor and toast handling unchanged.
// =========================================================================

// listAvatarsAdmin (GET /dashboard/zsy/avatar/list) — paginated list of every
// persona, on-shelf or not, with keyword / gender / age_range / race / voice_id /
// enabled filters.
func listAvatarsAdmin(c *gin.Context) {
	result, err := AvatarSearch(parseAvatarListQuery(c), nil)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// getAvatar (GET /dashboard/zsy/avatar/:id) — one persona, including off-shelf
// ones.
func getAvatar(c *gin.Context) {
	id, err := parseAvatarIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	view, err := AvatarGetByID(id, nil)
	if err != nil {
		if errors.Is(err, ErrAvatarNotFound) {
			common.ApiErrorMsg(c, "形象不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

// createAvatar (POST /dashboard/zsy/avatar) — create one persona.
func createAvatar(c *gin.Context) {
	var dto AvatarCreateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	view, err := AvatarInsert(&dto)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-avatar] created avatar id=%d name=%q voiceId=%q",
		view.ID, view.Name, view.VoiceID))
	common.ApiSuccess(c, view)
}

// updateAvatar (PUT /dashboard/zsy/avatar/:id) — partial update.
func updateAvatar(c *gin.Context) {
	id, err := parseAvatarIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var dto AvatarUpdateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	view, err := AvatarUpdate(id, &dto)
	if err != nil {
		if errors.Is(err, ErrAvatarNotFound) {
			common.ApiErrorMsg(c, "形象不存在")
			return
		}
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-avatar] updated avatar id=%d name=%q enabled=%v",
		view.ID, view.Name, view.Enabled))
	common.ApiSuccess(c, view)
}

// deleteAvatar (DELETE /dashboard/zsy/avatar/:id) — hard delete.
func deleteAvatar(c *gin.Context) {
	id, err := parseAvatarIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	name, err := AvatarDelete(id)
	if err != nil {
		if errors.Is(err, ErrAvatarNotFound) {
			common.ApiErrorMsg(c, "形象不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-avatar] deleted avatar id=%d name=%q", id, name))
	common.ApiSuccess(c, gin.H{"id": id, "name": name})
}

// =========================================================================
// Admin CSV import / export
// =========================================================================

// exportAvatars (GET /dashboard/zsy/avatar/export) — download the personas
// matching the same filters as the list, as a UTF-8 CSV (BOM included so Excel
// renders Chinese correctly). Without filters this is also the import template:
// an empty catalog exports the header row alone.
func exportAvatars(c *gin.Context) {
	rows, err := AvatarExportRows(parseAvatarListQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := "zsy-avatars-" + time.Now().Format("20060102-150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	// The headers are already on the wire, so a write failure can only be logged:
	// the caller sees a truncated file rather than a broken envelope.
	if err := WriteAvatarsCSV(c.Writer, rows); err != nil {
		common.SysError("[zsy-avatar] export failed: " + err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-avatar] exported %d avatars to %s", len(rows), filename))
}

// importAvatars (POST /dashboard/zsy/avatar/import) — multipart CSV upload.
//
// Body: field `file` with the CSV, optional query `mode` = create | upsert
// (default upsert). The answer reports what happened per row instead of failing
// the whole file, so one bad line does not cost the operator the import.
func importAvatars(c *gin.Context) {
	const maxBodyBytes = maxImportBytes + (64 << 10)
	tooLarge := fmt.Sprintf("导入失败: CSV 文件不能超过 %dMB", maxImportBytes>>20)
	if c.Request.ContentLength > maxBodyBytes {
		common.ApiErrorMsg(c, tooLarge)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBodyBytes)

	header, err := c.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			common.ApiErrorMsg(c, tooLarge)
			return
		}
		common.ApiErrorMsg(c, "导入失败: 请通过 multipart 字段 file 选择 CSV 文件")
		return
	}
	file, err := header.Open()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	defer file.Close()

	rows, warnings, err := ParseAvatarsCSV(file)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	mode := defaultImportMode(c.DefaultQuery("mode", ImportModeUpsert))
	result, err := ImportAvatars(rows, mode, warnings)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-avatar] import mode=%s total=%d created=%d updated=%d failed=%d",
		mode, result.Total, result.Created, result.Updated, result.Failed))
	common.ApiSuccess(c, result)
}

// defaultImportMode trims the requested mode so a stray space in a query string
// still selects the documented value, and an absent one falls back to upsert.
func defaultImportMode(raw string) string {
	if trimmed := strings.TrimSpace(raw); trimmed != "" {
		return trimmed
	}
	return ImportModeUpsert
}
