package voice

import (
	"errors"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// Stable machine-readable codes for the public face. Third-party callers branch
// on these, never on the localized message.
const (
	codeInvalidParams = "INVALID_PARAMS"
	codeVoiceNotFound = "VOICE_NOT_FOUND"
	codeDatabaseError = "DATABASE_ERROR"
)

// respondPublicError answers a third-party caller with a real HTTP status plus
// a stable code, keeping the {success, message} shape the dashboard uses.
func respondPublicError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": message})
}

// listPublicVoices (GET /api/zsy/voice/list) — the paginated voice catalog.
//
// Only on-shelf (enabled) voices are returned: the `enabled` query parameter is
// ignored on this face, it belongs to the admin list.
func listPublicVoices(c *gin.Context) {
	q := parseVoiceListQuery(c)
	enabled := true
	q.Enabled = &enabled

	result, err := VoiceSearch(q)
	if err != nil {
		common.SysError("[zsy-voice] public list failed: " + err.Error())
		respondPublicError(c, http.StatusInternalServerError, codeDatabaseError, "读取音色列表失败")
		return
	}
	common.ApiSuccess(c, result)
}

// getPublicVoice (GET /api/zsy/voice/:id) — one on-shelf voice. An off-shelf
// voice answers 404 exactly like a missing one, so the catalog never leaks
// unpublished drafts.
func getPublicVoice(c *gin.Context) {
	id, err := parseVoiceIDParam(c)
	if err != nil {
		respondPublicError(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	view, err := VoiceGetByID(id)
	if err != nil {
		if errors.Is(err, ErrVoiceNotFound) {
			respondPublicError(c, http.StatusNotFound, codeVoiceNotFound, "音色不存在")
			return
		}
		common.SysError("[zsy-voice] public detail failed: " + err.Error())
		respondPublicError(c, http.StatusInternalServerError, codeDatabaseError, "读取音色失败")
		return
	}
	if !view.Enabled {
		respondPublicError(c, http.StatusNotFound, codeVoiceNotFound, "音色不存在或已下架")
		return
	}
	common.ApiSuccess(c, view)
}
