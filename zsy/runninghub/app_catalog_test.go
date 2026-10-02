package runninghub_test

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// GET /api/zsy/rh/app-catalog — the one-shot published catalog.
//
// The tests below pin the contract a third-party integrator depends on: one
// request returns every app a visitor may see, grouped by category in display
// order, with the introduction and the full parameter schema included.
// ---------------------------------------------------------------------------

const catalogRoute = "/api/zsy/rh/app-catalog"

// catalogCategories reads the `categories` array of a catalog response.
func catalogCategories(t *testing.T, data map[string]any) []any {
	t.Helper()
	cats, ok := data["categories"].([]any)
	require.True(t, ok, "categories missing; data=%v", data)
	return cats
}

// categoryByName finds one category bucket in a catalog response.
func categoryByName(t *testing.T, data map[string]any, name string) map[string]any {
	t.Helper()
	for _, raw := range catalogCategories(t, data) {
		cat, ok := raw.(map[string]any)
		require.True(t, ok, "category entry is not an object: %v", raw)
		if cat["name"] == name {
			return cat
		}
	}
	t.Fatalf("category %q not found in %v", name, data["categories"])
	return nil
}

// catalogAppsOf returns the app objects of one category bucket.
func catalogAppsOf(t *testing.T, data map[string]any, categoryName string) []any {
	t.Helper()
	apps, ok := categoryByName(t, data, categoryName)["apps"].([]any)
	require.True(t, ok, "apps missing on category %q", categoryName)
	return apps
}

// appByName indexes one category's apps by name.
func appByName(t *testing.T, apps []any) map[string]map[string]any {
	t.Helper()
	out := make(map[string]map[string]any, len(apps))
	for _, item := range apps {
		app, ok := item.(map[string]any)
		require.True(t, ok, "app entry is not an object: %v", item)
		out[app["name"].(string)] = app
	}
	return out
}

func TestAppCatalog_ReturnsEveryPublishedAppWithSchema(t *testing.T) {
	env := newRHITestEnv(t, "8501-catalog")
	env.router.GET(catalogRoute, runninghub.TestHookListAppCatalog)

	env.createApp("提示词绘图", "8501-prompt-app", true, 1000, 1.0)
	env.createAppWithSchema("视频超分", "8501-upscale-wf", []rhparser.SchemaParam{
		{NodeID: "212", FieldName: "start", Label: "开始秒", Type: "seconds", Required: true},
		{NodeID: "229", FieldName: "end", Label: "结束秒", Type: "seconds", Required: true},
	})

	// A model-API app and two apps that must stay invisible to the public
	// catalog: an unpublished draft and an admin-only app.
	modelView, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name: "扁平模型应用", Kind: runninghub.AppKindModel, UpstreamID: "8501-flat-model",
		Description: "扁平请求体的模型应用", Published: true, ModelBaseRateRatio: 1.0,
		ParamSchema: []rhparser.SchemaParam{{NodeID: "", FieldName: "prompt", Label: "提示词", Type: "text", Required: true}},
	})
	require.NoError(t, err)
	_, err = runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name: "未发布草稿", Kind: runninghub.AppKindAICApp, UpstreamID: "8501-draft",
		Published: false, ModelBaseRateRatio: 1.0,
	})
	require.NoError(t, err)
	_, err = runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name: "管理员专用", Kind: runninghub.AppKindAICApp, UpstreamID: "8501-admin-only",
		Published: true, AdminOnly: true, ModelBaseRateRatio: 1.0,
	})
	require.NoError(t, err)

	w, raw := doJSON(t, env.router, http.MethodGet, catalogRoute, nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", string(raw))
	envelope := parseAPIEnvelope(t, raw)
	require.True(t, envelope.Success, "body: %s", string(raw))
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "data missing; body=%s", string(raw))

	// Three published, non-admin-only apps are in scope; the draft and the
	// admin-only one are not.
	assert.Equal(t, float64(3), data["totalApps"], "body: %s", string(raw))
	assert.NotZero(t, data["generatedAt"])

	apps := catalogAppsOf(t, data, "未分类")
	require.Len(t, apps, 3)
	assert.Equal(t, float64(3), categoryByName(t, data, "未分类")["appCount"])

	byName := appByName(t, apps)
	require.Contains(t, byName, "提示词绘图")
	require.Contains(t, byName, "视频超分")
	require.Contains(t, byName, "扁平模型应用")
	assert.NotContains(t, byName, "未发布草稿")
	assert.NotContains(t, byName, "管理员专用")

	// Basic information an app card needs: introduction, kind, billing mode and
	// upstream id — not just the id and the name.
	promptCard := byName["提示词绘图"]
	assert.Equal(t, "8501-prompt-app", promptCard["upstreamId"])
	assert.Equal(t, "ai_app", promptCard["kind"])
	assert.Equal(t, true, promptCard["perCallBilling"])
	assert.Equal(t, float64(1000), promptCard["fixedQuotaPerCall"])

	// Parameters travel with the list, so a client can render the form without a
	// second request per app.
	promptSchema, ok := promptCard["paramSchema"].([]any)
	require.True(t, ok, "paramSchema missing on the list entry")
	require.Len(t, promptSchema, 1)
	first := promptSchema[0].(map[string]any)
	assert.Equal(t, "122", first["nodeId"])
	assert.Equal(t, "prompt", first["fieldName"])
	assert.Equal(t, "提示词", first["label"])
	assert.Equal(t, "text", first["type"])
	assert.Equal(t, true, first["required"])

	workflowSchema, ok := byName["视频超分"]["paramSchema"].([]any)
	require.True(t, ok)
	assert.Len(t, workflowSchema, 2, "every parameter of an app must be listed")
	assert.Equal(t, float64(modelView.ID), byName["扁平模型应用"]["id"])
}

func TestAppCatalog_GroupsByCategoryInDisplayOrder(t *testing.T) {
	env := newRHITestEnv(t, "8502-catalog-order")
	env.router.GET(catalogRoute, runninghub.TestHookListAppCatalog)

	video, err := runninghub.AppCategoryInsert("视频", 20)
	require.NoError(t, err)
	image, err := runninghub.AppCategoryInsert("图像", 10)
	require.NoError(t, err)

	imageApp := env.createApp("文生图", "8502-image-app", true, 1000, 1.0)
	videoApp := env.createApp("图生视频", "8502-video-app", true, 2000, 1.0)
	require.NoError(t, env.db.Model(&runninghub.App{}).Where("id = ?", imageApp).Update("category_id", image.ID).Error)
	require.NoError(t, env.db.Model(&runninghub.App{}).Where("id = ?", videoApp).Update("category_id", video.ID).Error)

	w, raw := doJSON(t, env.router, http.MethodGet, catalogRoute, nil)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", string(raw))
	data := parseAPIEnvelope(t, raw).Data.(map[string]any)

	cats := catalogCategories(t, data)
	// sort_order decides the rail: 图像 (10) before 视频 (20). No uncategorized
	// bucket exists here because every app has a category.
	require.Len(t, cats, 2, "body: %s", string(raw))
	assert.Equal(t, "图像", cats[0].(map[string]any)["name"])
	assert.Equal(t, "视频", cats[1].(map[string]any)["name"])

	imageCat := categoryByName(t, data, "图像")
	assert.Equal(t, float64(image.ID), imageCat["id"])
	assert.Equal(t, float64(10), imageCat["sortOrder"])
	apps := imageCat["apps"].([]any)
	require.Len(t, apps, 1)
	assert.Equal(t, "文生图", apps[0].(map[string]any)["name"])
	assert.Equal(t, "图像", apps[0].(map[string]any)["categoryName"], "the category name is joined onto the app")
}

func TestAppCatalog_FiltersByKindSiteAndCategory(t *testing.T) {
	env := newRHITestEnv(t, "8503-catalog-filter")
	env.router.GET(catalogRoute, runninghub.TestHookListAppCatalog)

	cat, err := runninghub.AppCategoryInsert("工作流", 1)
	require.NoError(t, err)
	emptyCat, err := runninghub.AppCategoryInsert("空分类", 2)
	require.NoError(t, err)

	cnApp := env.createApp("国内应用", "8503-cn-app", true, 1000, 1.0)
	require.NoError(t, env.db.Model(&runninghub.App{}).Where("id = ?", cnApp).
		Updates(map[string]any{"site": "cn", "category_id": cat.ID}).Error)
	intlApp := env.createApp("国际应用", "8503-intl-app", true, 1000, 1.0)
	require.NoError(t, env.db.Model(&runninghub.App{}).Where("id = ?", intlApp).
		Updates(map[string]any{"site": "intl"}).Error)

	t.Run("site filter matches only the requested site", func(t *testing.T) {
		_, raw := doJSON(t, env.router, http.MethodGet, catalogRoute+"?site=cn", nil)
		data := parseAPIEnvelope(t, raw).Data.(map[string]any)
		assert.Equal(t, float64(1), data["totalApps"], "body: %s", string(raw))
		apps := catalogAppsOf(t, data, "工作流")
		require.Len(t, apps, 1)
		assert.Equal(t, "国内应用", apps[0].(map[string]any)["name"])
	})

	t.Run("kind filter is applied to the app query", func(t *testing.T) {
		_, raw := doJSON(t, env.router, http.MethodGet, catalogRoute+"?kind=model", nil)
		data := parseAPIEnvelope(t, raw).Data.(map[string]any)
		assert.Equal(t, float64(0), data["totalApps"], "no model-kind app exists; body: %s", string(raw))
	})

	t.Run("empty categories stay in the rail with a zero count", func(t *testing.T) {
		_, raw := doJSON(t, env.router, http.MethodGet, catalogRoute+"?site=intl", nil)
		data := parseAPIEnvelope(t, raw).Data.(map[string]any)
		empty := categoryByName(t, data, "空分类")
		assert.Equal(t, float64(emptyCat.ID), empty["id"])
		assert.Equal(t, float64(0), empty["appCount"])
		apps, ok := empty["apps"].([]any)
		require.True(t, ok, "apps must be an array, not null")
		assert.Empty(t, apps)
	})

	t.Run("categoryId narrows the catalog to one bucket", func(t *testing.T) {
		_, raw := doJSON(t, env.router, http.MethodGet,
			catalogRoute+"?categoryId="+strconv.FormatUint(uint64(cat.ID), 10), nil)
		data := parseAPIEnvelope(t, raw).Data.(map[string]any)
		assert.Equal(t, float64(1), data["totalApps"], "body: %s", string(raw))
		assert.Equal(t, float64(1), categoryByName(t, data, "工作流")["appCount"])
		assert.Equal(t, float64(0), categoryByName(t, data, "空分类")["appCount"])
	})
}

func TestAppCatalog_RejectsUnknownFilters(t *testing.T) {
	env := newRHITestEnv(t, "8504-catalog-errors")
	env.router.GET(catalogRoute, runninghub.TestHookListAppCatalog)

	cases := []struct {
		name     string
		query    string
		wantCode string
	}{
		{"unknown kind", "?kind=nope", runninghub.ErrorCodeCatalogInvalidKind},
		{"unknown site", "?site=mars", runninghub.ErrorCodeCatalogInvalidSite},
		{"non-numeric categoryId", "?categoryId=abc", runninghub.ErrorCodeCatalogInvalidCategory},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w, raw := doJSON(t, env.router, http.MethodGet, catalogRoute+c.query, nil)
			assert.Equal(t, http.StatusBadRequest, w.Code, "body: %s", string(raw))
			var body map[string]any
			require.NoError(t, json.Unmarshal(raw, &body))
			assert.Equal(t, false, body["success"])
			assert.Equal(t, c.wantCode, body["code"])
		})
	}
}
