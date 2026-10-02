package runninghub

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Published app catalog
// ---------------------------------------------------------------------------
//
// listPublicApps (GET /api/zsy/rh/apps) is paginated and page-shaped: a client
// that wants to render every app — a mobile app, a third-party integrator, a
// docs generator — would have to walk pages and then re-join each app onto its
// category by hand. The catalog endpoint (GET /api/zsy/rh/app-catalog) answers
// that need in ONE request: every published, non-admin-only app with its
// introduction, cover, category, site, billing mode and full parameter schema,
// already grouped by category in the portal's display order.
//
// It reuses the AppView read shape, so a client that can consume the list
// endpoint needs no second parser for this one.

// AppCatalogQuery selects which apps the catalog contains. Every field is
// optional; the zero value means "the whole published catalog". Kind/Site are
// canonical values (see AppKind and normalizeSite) — filtering happens in SQL,
// so an unknown kind yields an empty catalog rather than a SQL error.
type AppCatalogQuery struct {
	Kind       string `json:"kind"`
	Site       string `json:"site"`
	CategoryID uint   `json:"categoryId"`
}

// AppCatalogResponse is the whole payload of one catalog call: the taxonomy
// plus the apps attached to it. Categories are never filtered by Kind/Site so a
// client can still render the full category rail; each category carries its own
// matching appCount (0 for a category this filter does not reach).
type AppCatalogResponse struct {
	// GeneratedAt is a Unix timestamp, like every other time field in the
	// plugin API.
	GeneratedAt int64 `json:"generatedAt"`
	// TotalApps counts the apps in this response (after filters).
	TotalApps int `json:"totalApps"`
	// TotalCategories counts the entries in Categories, including empty ones.
	TotalCategories int `json:"totalCategories"`
	// Categories is ordered by SortOrder then id. The "未分类" (uncategorized)
	// entry is always last when at least one matching app has no category.
	Categories []*AppCatalogCategory `json:"categories"`
}

// AppCatalogCategory is one group of the response. Apps holds the full read
// shape — name, description, cover, parameter schema, billing — so the caller
// never has to fetch a second time to render a form.
type AppCatalogCategory struct {
	ID        uint       `json:"id"`
	Name      string     `json:"name"`
	SortOrder int        `json:"sortOrder"`
	AppCount  int        `json:"appCount"`
	Apps      []*AppView `json:"apps"`
}

// AppCatalogGet assembles the catalog for one query. Apps are ordered by name
// inside their category so the response is stable across calls; rows whose
// category row is missing (soft-deleted, or never set) land in the
// uncategorized group.
func AppCatalogGet(q AppCatalogQuery) (*AppCatalogResponse, error) {
	if db() == nil {
		return nil, fmt.Errorf("database is not initialised")
	}
	rows, err := catalogAppRows(q)
	if err != nil {
		return nil, err
	}
	categories, err := AppCategoryList()
	if err != nil {
		return nil, err
	}

	// Resolve every row before grouping so a malformed param_schema blob fails
	// the whole request instead of silently dropping one app. The category name
	// map is built once (see appToViewWithCategories) to keep this off the N+1
	// path.
	names := categoryNames(categories)
	views := make([]*AppView, 0, len(rows))
	for i := range rows {
		view, viewErr := appToViewWithCategories(&rows[i], names)
		if viewErr != nil {
			return nil, fmt.Errorf("runninghub app catalog[%d] view: %w", i, viewErr)
		}
		views = append(views, view)
	}
	return buildAppCatalog(views, categories, time.Now().Unix()), nil
}

// catalogAppRows loads every app the query matches, name-ordered so each
// category's apps come back in a stable order.
func catalogAppRows(q AppCatalogQuery) ([]App, error) {
	base := db().Model(&App{}).
		Where("published = ? AND admin_only = ?", true, false)
	if kind := strings.TrimSpace(q.Kind); kind != "" {
		base = base.Where("kind = ?", kind)
	}
	if site := normalizeSite(q.Site); site != "" {
		base = base.Where("site = ?", site)
	}
	if q.CategoryID > 0 {
		base = base.Where("category_id = ?", q.CategoryID)
	}

	var rows []App
	if err := base.Order("name asc, id asc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("runninghub app catalog: %w", err)
	}
	return rows, nil
}

// categoryNames flattens the category view list into the id → name map
// appToViewWithCategories joins on.
func categoryNames(categories []*AppCategoryView) map[uint]string {
	out := make(map[uint]string, len(categories))
	for _, cat := range categories {
		out[cat.ID] = cat.Name
	}
	return out
}

// buildAppCatalog groups resolved views by category in display order. It is a
// pure function of its inputs (no database access) so the grouping contract —
// category order, uncategorized bucket, per-category counts — is unit-testable
// on its own.
func buildAppCatalog(views []*AppView, categories []*AppCategoryView, generatedAt int64) *AppCatalogResponse {
	groups := make(map[uint][]*AppView)
	var uncategorized []*AppView
	present := make(map[uint]bool, len(views))
	for _, view := range views {
		present[view.CategoryID] = true
		if view.CategoryID == 0 {
			uncategorized = append(uncategorized, view)
			continue
		}
		groups[view.CategoryID] = append(groups[view.CategoryID], view)
	}

	resp := &AppCatalogResponse{
		GeneratedAt: generatedAt,
		TotalApps:   len(views),
		Categories:  make([]*AppCatalogCategory, 0, len(categories)+1),
	}
	// Every live category is listed, even when the current filter leaves it
	// empty: the response doubles as the category rail.
	for _, cat := range categories {
		resp.Categories = append(resp.Categories, &AppCatalogCategory{
			ID:        cat.ID,
			Name:      cat.Name,
			SortOrder: cat.SortOrder,
			AppCount:  len(groups[cat.ID]),
			Apps:      nonNilAppViews(groups[cat.ID]),
		})
	}
	// An app pointing at a category that no longer exists must not vanish: it
	// gets its own bucket keyed by the stale id.
	for id, apps := range groups {
		if categoryListed(categories, id) {
			continue
		}
		resp.Categories = append(resp.Categories, &AppCatalogCategory{
			ID:       id,
			AppCount: len(apps),
			Apps:     apps,
		})
	}
	if len(uncategorized) > 0 {
		resp.Categories = append(resp.Categories, &AppCatalogCategory{
			ID:       0,
			Name:     "未分类",
			AppCount: len(uncategorized),
			Apps:     uncategorized,
		})
	}
	sortCatalogCategories(resp.Categories)
	resp.TotalCategories = len(resp.Categories)
	return resp
}

// sortCatalogCategories puts the real categories in display order (sort_order,
// then id) and keeps the uncategorized bucket last. A stale-category bucket
// sorts as order 0 so it groups with the first real category instead of
// jumping to the front of the rail.
func sortCatalogCategories(categories []*AppCatalogCategory) {
	sort.SliceStable(categories, func(i, j int) bool {
		if categories[i].ID == 0 {
			return false
		}
		if categories[j].ID == 0 {
			return true
		}
		if categories[i].SortOrder != categories[j].SortOrder {
			return categories[i].SortOrder < categories[j].SortOrder
		}
		return categories[i].ID < categories[j].ID
	})
}

func categoryListed(categories []*AppCategoryView, id uint) bool {
	for _, cat := range categories {
		if cat.ID == id {
			return true
		}
	}
	return false
}

// nonNilAppViews keeps `apps` a JSON array (`[]`) instead of `null` for an
// empty category, so typed clients can iterate without a null check.
func nonNilAppViews(apps []*AppView) []*AppView {
	if apps == nil {
		return []*AppView{}
	}
	return apps
}
