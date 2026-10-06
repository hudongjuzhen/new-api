package tone

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// parseToneListQuery translates a list query string into the typed query shared
// by the public and admin list endpoints (and by the CSV export, which follows
// the active filters).
//
// Pagination accepts the documented `page` / `page_size` pair plus the host's
// own `p` / `size` aliases, so a caller used to the core list endpoints does not
// have to translate. An unparsable or missing value falls back to the default
// instead of failing the request: a bad page number must not break a catalog
// listing.
//
// Text filters are trimmed here only; ToneSearch/applyToneFilters additionally
// normalizes the controlled vocabularies, so a direct store call behaves like an
// HTTP call.
func parseToneListQuery(c *gin.Context) ToneListQuery {
	q := ToneListQuery{
		Keyword:  strings.TrimSpace(c.Query("keyword")),
		Category: strings.TrimSpace(c.Query("category")),
		Tone:     strings.TrimSpace(c.Query("tone")),
		Language: strings.TrimSpace(c.Query("language")),
		Page:     queryInt(c, []string{"page", "p"}, 1),
		PageSize: queryInt(c, []string{"page_size", "size"}, defaultPageSize),
	}
	if raw := strings.TrimSpace(c.Query("enabled")); raw != "" {
		enabled := raw == "1" || strings.EqualFold(raw, "true")
		q.Enabled = &enabled
	}
	return q
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

// parseToneIDParam reads the positive :id path parameter used by both faces.
func parseToneIDParam(c *gin.Context) (uint, error) {
	raw := strings.TrimSpace(c.Param("id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("非法的文风 ID: %q", raw)
	}
	return uint(id), nil
}
