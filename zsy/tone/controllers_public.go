package tone

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
	codeToneNotFound  = "TONE_NOT_FOUND"
	codeDatabaseError = "DATABASE_ERROR"
)

// respondPublicError answers a third-party caller with a real HTTP status plus
// a stable code, keeping the {success, message} shape the dashboard uses.
func respondPublicError(c *gin.Context, status int, code string, message string) {
	c.JSON(status, gin.H{"success": false, "code": code, "message": message})
}

// listPublicTones (GET /api/zsy/tone/list) — the paginated tone catalog.
//
// Only on-shelf (enabled) tones are returned: the `enabled` query parameter is
// ignored on this face, it belongs to the admin list.
func listPublicTones(c *gin.Context) {
	q := parseToneListQuery(c)
	enabled := true
	q.Enabled = &enabled

	result, err := ToneSearch(q)
	if err != nil {
		common.SysError("[zsy-tone] public list failed: " + err.Error())
		respondPublicError(c, http.StatusInternalServerError, codeDatabaseError, "读取文风列表失败")
		return
	}
	common.ApiSuccess(c, result)
}

// getPublicTone (GET /api/zsy/tone/:id) — one on-shelf tone. An off-shelf tone
// answers 404 exactly like a missing one, so the catalog never leaks
// unpublished drafts.
func getPublicTone(c *gin.Context) {
	id, err := parseToneIDParam(c)
	if err != nil {
		respondPublicError(c, http.StatusBadRequest, codeInvalidParams, err.Error())
		return
	}
	view, err := ToneGetByID(id)
	if err != nil {
		if errors.Is(err, ErrToneNotFound) {
			respondPublicError(c, http.StatusNotFound, codeToneNotFound, "文风不存在")
			return
		}
		common.SysError("[zsy-tone] public detail failed: " + err.Error())
		respondPublicError(c, http.StatusInternalServerError, codeDatabaseError, "读取文风失败")
		return
	}
	if !view.Enabled {
		respondPublicError(c, http.StatusNotFound, codeToneNotFound, "文风不存在或已下架")
		return
	}
	common.ApiSuccess(c, view)
}

// getToneStandard (GET /api/zsy/tone/standard) — the published 文风标准: the
// controlled vocabularies with their labels and descriptions, the field caps,
// the compatibility promise, and the worked-example convention.
//
// # Why it is public and why it is cacheable
//
// The whole point of publishing the vocabulary is that clients read it instead
// of retyping it, and a client reads it once at startup — so this is the one
// endpoint under this group that is *not* no-store. Its payload only changes
// when a new version ships (see ToneStandardVersion), which is exactly the
// condition an HTTP cache is designed for.
//
// The group still applies middleware.DisableCache() to everything else: a
// mutable catalog must never be cached. Overriding the header here rather than
// splitting the group keeps the public URLs in one readable block in routes.go.
func getToneStandard(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=3600")
	common.ApiSuccess(c, ToneStandardOf())
}
