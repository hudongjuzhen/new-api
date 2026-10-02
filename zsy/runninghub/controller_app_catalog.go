package runninghub

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// listAppCatalog (GET /api/zsy/rh/app-catalog) answers the whole published app
// catalog — apps plus their categories — in one response, with no pagination.
//
// It is the third-party/integration entry point of the app center: the
// dashboard portal walks GET /api/zsy/rh/apps page by page because it renders a
// paginated grid, while a client that needs the full list (to cache it, to
// build a form for every app, or to generate its own docs) gets everything at
// once here. The app objects are the same AppView shape the list endpoint
// returns, so the two are interchangeable per item.
//
// Browsing stays public, exactly like GET /api/zsy/rh/apps: the catalog only
// ever contains published, non-admin-only apps.
func listAppCatalog(c *gin.Context) {
	query := AppCatalogQuery{
		Kind: strings.TrimSpace(c.Query("kind")),
		Site: strings.TrimSpace(c.Query("site")),
	}
	// `categoryId` (camelCase, like the app payloads) with `category_id` as the
	// snake_case alias the other plugin list endpoints accept.
	rawCategory := strings.TrimSpace(c.Query("categoryId"))
	if rawCategory == "" {
		rawCategory = strings.TrimSpace(c.Query("category_id"))
	}
	if rawCategory != "" {
		id, err := strconv.ParseUint(rawCategory, 10, 32)
		if err != nil {
			apiError(c, http.StatusBadRequest, ErrorCodeCatalogInvalidCategory, "非法 categoryId: 必须为非负整数")
			return
		}
		query.CategoryID = uint(id)
	}
	// Kind is a closed enum: an unknown value would otherwise return an empty
	// catalog that looks like "no apps configured", which is the wrong answer
	// for a typo.
	switch query.Kind {
	case "", string(AppKindAICApp), string(AppKindWorkflow), string(AppKindModel):
	default:
		apiError(c, http.StatusBadRequest, ErrorCodeCatalogInvalidKind,
			"非法 kind: 可选 ai_app / workflow / model")
		return
	}
	// Site accepts the same aliases as the app's site field (cn/intl plus the
	// RunningHub labels); normalizeSite collapses an unknown value to "", which
	// would silently widen the filter, so reject it explicitly instead.
	if query.Site != "" {
		site := normalizeSite(query.Site)
		if site == "" {
			apiError(c, http.StatusBadRequest, ErrorCodeCatalogInvalidSite, "非法 site: 可选 cn / intl")
			return
		}
		query.Site = site
	}

	catalog, err := AppCatalogGet(query)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, catalog)
}
