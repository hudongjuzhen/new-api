package voice

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
// Admin controllers — voice CRUD behind /dashboard/zsy/voice
//
// They answer with the dashboard envelope ({success, message, data}) so the
// admin page reuses the host's axios interceptor and toast handling unchanged.
// =========================================================================

// listVoicesAdmin (GET /dashboard/zsy/voice/list) — paginated list of every
// voice, on-shelf or not, with keyword / voice_type / enabled filters.
func listVoicesAdmin(c *gin.Context) {
	result, err := VoiceSearch(parseVoiceListQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, result)
}

// getVoice (GET /dashboard/zsy/voice/:id) — one voice, including off-shelf ones.
func getVoice(c *gin.Context) {
	id, err := parseVoiceIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	view, err := VoiceGetByID(id)
	if err != nil {
		if errors.Is(err, ErrVoiceNotFound) {
			common.ApiErrorMsg(c, "音色不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, view)
}

// createVoice (POST /dashboard/zsy/voice) — create one voice.
func createVoice(c *gin.Context) {
	var dto VoiceCreateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	view, err := VoiceInsert(&dto)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-voice] created voice id=%d name=%q voiceType=%q", view.ID, view.Name, view.VoiceType))
	common.ApiSuccess(c, view)
}

// updateVoice (PUT /dashboard/zsy/voice/:id) — partial update.
func updateVoice(c *gin.Context) {
	id, err := parseVoiceIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	var dto VoiceUpdateDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	view, err := VoiceUpdate(id, &dto)
	if err != nil {
		if errors.Is(err, ErrVoiceNotFound) {
			common.ApiErrorMsg(c, "音色不存在")
			return
		}
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-voice] updated voice id=%d name=%q enabled=%v", view.ID, view.Name, view.Enabled))
	common.ApiSuccess(c, view)
}

// deleteVoice (DELETE /dashboard/zsy/voice/:id) — hard delete.
func deleteVoice(c *gin.Context) {
	id, err := parseVoiceIDParam(c)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	name, err := VoiceDelete(id)
	if err != nil {
		if errors.Is(err, ErrVoiceNotFound) {
			common.ApiErrorMsg(c, "音色不存在")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-voice] deleted voice id=%d name=%q", id, name))
	common.ApiSuccess(c, gin.H{"id": id, "name": name})
}

// =========================================================================
// Admin CSV import / export
// =========================================================================

// exportVoices (GET /dashboard/zsy/voice/export) — download the voices matching
// the same filters as the list, as a UTF-8 CSV (BOM included so Excel renders
// Chinese correctly). Without filters this is also the import template: an empty
// catalog exports the header row alone.
func exportVoices(c *gin.Context) {
	rows, err := VoiceExportRows(parseVoiceListQuery(c))
	if err != nil {
		common.ApiError(c, err)
		return
	}

	filename := "zsy-voices-" + time.Now().Format("20060102-150405") + ".csv"
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Header("Cache-Control", "no-store")
	c.Status(http.StatusOK)

	// The headers are already on the wire, so a write failure can only be
	// logged: the caller sees a truncated file rather than a broken envelope.
	if err := WriteVoicesCSV(c.Writer, rows); err != nil {
		common.SysError("[zsy-voice] export failed: " + err.Error())
		return
	}
	common.SysLog(fmt.Sprintf("[zsy-voice] exported %d voices to %s", len(rows), filename))
}

// importVoices (POST /dashboard/zsy/voice/import) — multipart CSV upload.
//
// Body: field `file` with the CSV, optional query `mode` = create | upsert
// (default upsert). The answer reports what happened per row instead of failing
// the whole file, so one bad line does not cost the operator the import.
func importVoices(c *gin.Context) {
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

	rows, warnings, err := ParseVoicesCSV(file)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	mode := strings.TrimSpace(c.DefaultQuery("mode", ImportModeUpsert))
	result, err := ImportVoices(rows, mode, warnings)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-voice] import mode=%s total=%d created=%d updated=%d failed=%d",
		mode, result.Total, result.Created, result.Updated, result.Failed))
	common.ApiSuccess(c, result)
}
