package tone

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
// Admin controllers — tone CRUD behind /dashboard/zsy/tone
//
// They answer with the dashboard envelope ({success, message, data}) so the
// admin page reuses the host's axios interceptor and toast handling unchanged.
// =========================================================================

// listTonesAdmin (GET /dashboard/zsy/tone/list) — paginated list of every tone,
// on-shelf or not, with keyword / category / tone / language / enabled filters.
func listTonesAdmin(c *gin.Context) {
	result, err := ToneSearch(parseToneListQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// getTone (GET /dashboard/zsy/tone/:id) — one tone, including off-shelf ones.
func getTone(c *gin.Context) {
	id, err := parseToneIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	view, err := ToneGetByID(id)
	if err != nil {
		if errors.Is(err, ErrToneNotFound) {
			common.ApiErrorMsg(c, "文风不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

// createTone (POST /dashboard/zsy/tone) — create one tone.
func createTone(c *gin.Context) {
	var dto ToneCreateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	view, err := ToneInsert(&dto)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-tone] created tone id=%d name=%q category=%q tone=%q",
		view.ID, view.Name, view.Category, view.Tone))
	common.ApiSuccess(c, view)
}

// updateTone (PUT /dashboard/zsy/tone/:id) — partial update.
func updateTone(c *gin.Context) {
	id, err := parseToneIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var dto ToneUpdateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	view, err := ToneUpdate(id, &dto)
	if err != nil {
		if errors.Is(err, ErrToneNotFound) {
			common.ApiErrorMsg(c, "文风不存在")
			return
		}
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-tone] updated tone id=%d name=%q enabled=%v",
		view.ID, view.Name, view.Enabled))
	common.ApiSuccess(c, view)
}

// deleteTone (DELETE /dashboard/zsy/tone/:id) — hard delete.
func deleteTone(c *gin.Context) {
	id, err := parseToneIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	name, err := ToneDelete(id)
	if err != nil {
		if errors.Is(err, ErrToneNotFound) {
			common.ApiErrorMsg(c, "文风不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-tone] deleted tone id=%d name=%q", id, name))
	common.ApiSuccess(c, gin.H{"id": id, "name": name})
}

// =========================================================================
// Admin CSV import / export
// =========================================================================

// exportTones (GET /dashboard/zsy/tone/export) — download the tones matching the
// same filters as the list, as a UTF-8 CSV (BOM included so Excel renders
// Chinese correctly). Without filters this is also the import template: an empty
// catalog exports the header row alone.
//
// ★ It doubles as the authoring surface for the 文风标准: the exported column
// order is the standard's field order, and a curated export is what an operator
// hands to whoever maintains the next revision of the vocabulary.
func exportTones(c *gin.Context) {
	rows, err := ToneExportRows(parseToneListQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := "zsy-tones-" + time.Now().Format("20060102-150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	// The headers are already on the wire, so a write failure can only be
	// logged: the caller sees a truncated file rather than a broken envelope.
	if err := WriteTonesCSV(c.Writer, rows); err != nil {
		common.SysError("[zsy-tone] export failed: " + err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-tone] exported %d tones to %s", len(rows), filename))
}

// importTones (POST /dashboard/zsy/tone/import) — multipart CSV upload.
//
// Body: field `file` with the CSV, optional query `mode` = create | upsert
// (default upsert). The answer reports what happened per row instead of failing
// the whole file, so one bad line does not cost the operator the import.
func importTones(c *gin.Context) {
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

	rows, warnings, err := ParseTonesCSV(file)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	mode := strings.TrimSpace(c.DefaultQuery("mode", ImportModeUpsert))
	result, err := ImportTones(rows, mode, warnings)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-tone] import mode=%s total=%d created=%d updated=%d failed=%d",
		mode, result.Total, result.Created, result.Updated, result.Failed))
	common.ApiSuccess(c, result)
}
