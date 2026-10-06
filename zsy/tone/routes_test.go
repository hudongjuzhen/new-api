package tone_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/zsy/tone"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Route tree — the URLs documented in docs/zsy-toneplaza-api.md. Registering the
// real tree also proves the mount does not panic on gin's route tree (a conflict
// there would take the whole gateway down at boot).
//
// ⚠ The list has no upload route, and that absence is the assertion: a tone has
// no media file, so a future copy-paste from zsy/voice that brings an upload
// endpoint along would fail here rather than quietly create an endpoint nothing
// calls.
// ---------------------------------------------------------------------------

func TestMountRoutes_PublishesTheDocumentedURLs(t *testing.T) {
	router := gin.New()
	require.NotPanics(t, func() { tone.TestHookMountRoutes(router) })

	registered := make([]string, 0, 10)
	for _, route := range router.Routes() {
		registered = append(registered, route.Method+" "+route.Path)
	}
	sort.Strings(registered)

	assert.Equal(t, []string{
		"DELETE /dashboard/zsy/tone/:id",
		"GET /api/zsy/tone/:id",
		"GET /api/zsy/tone/list",
		"GET /api/zsy/tone/standard",
		"GET /dashboard/zsy/tone/:id",
		"GET /dashboard/zsy/tone/export",
		"GET /dashboard/zsy/tone/list",
		"POST /dashboard/zsy/tone",
		"POST /dashboard/zsy/tone/import",
		"PUT /dashboard/zsy/tone/:id",
	}, registered)
}

func TestMountRoutes_PublicListAnswersThroughTheRealChain(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "克制的长文", "以克制的笔调写作", 0, true)
	insertTone(t, "下架文风", "这一条不上架", 1, false)

	router := gin.New()
	tone.TestHookMountRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/zsy/tone/list?page=1&page_size=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	payload := decodeEnvelope(t, w.Body.Bytes())
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)
	assert.EqualValues(t, 1, jsonInt(t, data, "total"), "the mounted public face only serves on-shelf tones")
	assert.EqualValues(t, 1, jsonInt(t, data, "pageSize"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
}

// TestMountRoutes_StandardIsTheOneCacheablePublicResponse pins the deliberate
// exception: the catalog is no-store, the standard is cacheable. Both are
// asserted together because the interesting failure is the pair drifting apart —
// a cacheable catalog would serve unpublished drafts, and a no-store standard
// would make every client hit the gateway on startup for a constant payload.
func TestMountRoutes_StandardIsTheOneCacheablePublicResponse(t *testing.T) {
	newToneTestDB(t)

	router := gin.New()
	tone.TestHookMountRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/zsy/tone/standard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	cacheControl := w.Header().Get("Cache-Control")
	assert.Contains(t, cacheControl, "public")
	assert.Contains(t, cacheControl, "max-age=3600")
	assert.NotContains(t, cacheControl, "no-store")
}

// TestMountRoutes_StandardDoesNotShadowTheIDRoute proves `/standard` is a real
// literal route and not a tone id: requesting it with no rows in the table still
// answers the standard instead of a "tone not found".
func TestMountRoutes_StandardDoesNotShadowTheIDRoute(t *testing.T) {
	newToneTestDB(t)

	router := gin.New()
	tone.TestHookMountRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/zsy/tone/standard", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, tone.ToneStandardVersion, dataMap(t, decodeEnvelope(t, w.Body.Bytes()))["version"])
}
