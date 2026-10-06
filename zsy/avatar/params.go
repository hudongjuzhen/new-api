package avatar

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseAvatarListQuery translates a list query string into the typed query shared
// by the public and admin list endpoints (and by the CSV export, which follows
// the active filters).
//
// Pagination accepts the documented `page` / `page_size` pair plus the host's own
// `p` / `size` aliases, so a caller used to the core list endpoints does not have
// to translate. An unparsable or missing value falls back to the default instead
// of failing the request: a bad page number must not break a catalog listing.
//
// Text filters are trimmed here only; AvatarSearch/applyAvatarFilters additionally
// normalizes the controlled vocabularies, so a direct store call behaves like an
// HTTP call.
func parseAvatarListQuery(c *gin.Context) AvatarListQuery {
	q := AvatarListQuery{
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Gender:   strings.TrimSpace(c.Query("gender")),
		AgeRange: strings.TrimSpace(c.Query("age_range")),
		Race:     strings.TrimSpace(c.Query("race")),
		VoiceID:  queryVoiceID(c),
		Page:     queryInt(c, []string{"page", "p"}, 1),
		PageSize: queryInt(c, []string{"page_size", "size"}, defaultPageSize),
	}
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		enabled := raw == "1" || strings.EqualFold(raw, "true")
		q.Enabled = &enabled
	}
	return q
}

// queryVoiceID reads the voice filter. `voice_id` is the documented snake_case
// spelling; `voiceId` is accepted too so a caller can copy the field name
// straight out of a persona payload.
func queryVoiceID(c *gin.Context) string {
	if raw := strings.TrimSpace(c.Query("voice_id")); raw != "" {
		return raw
	}
	return strings.TrimSpace(c.Query("voiceId"))
}

// queryInt returns the first parsable value among names, or fallback.
func queryInt(c *gin.Context, names []string, fallback int) int {
	for _, name := range names {
		raw := strings.TrimSpace(c.Query(name))
		if raw == "" {
			continue
		}
		if n, err := strconv.Atoi(raw); err == nil {
			return n
		}
	}
	return fallback
}

// parseAvatarIDParam reads the positive :id path parameter used by both faces.
func parseAvatarIDParam(c *gin.Context) (uint, error) {
	raw := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("非法的形象 ID: %q", raw)
	}
	return uint(id), nil
}
