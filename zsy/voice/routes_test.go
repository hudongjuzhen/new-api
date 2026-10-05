package voice_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/zsy/voice"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Route tree — the URLs documented in docs/zsy-voiceplaza-api.md. Registering
// the real tree also proves the mount does not panic on gin's route tree (a
// conflict there would take the whole gateway down at boot).
// ---------------------------------------------------------------------------

func TestMountRoutes_PublishesTheDocumentedURLs(t *testing.T) {
	router := gin.New()
	require.NotPanics(t, func() { voice.TestHookMountRoutes(router) })

	registered := make([]string, 0, 10)
	for _, route := range router.Routes() {
		registered = append(registered, route.Method+" "+route.Path)
	}
	sort.Strings(registered)

	assert.Equal(t, []string{
		"DELETE /dashboard/zsy/voice/:id",
		"GET /api/zsy/voice/:id",
		"GET /api/zsy/voice/list",
		"GET /dashboard/zsy/voice/:id",
		"GET /dashboard/zsy/voice/export",
		"GET /dashboard/zsy/voice/list",
		"POST /dashboard/zsy/voice",
		"POST /dashboard/zsy/voice/import",
		"POST /dashboard/zsy/voice/upload",
		"PUT /dashboard/zsy/voice/:id",
	}, registered)
}

func TestMountRoutes_PublicListAnswersThroughTheRealChain(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "小美", "zh_female_xiaomei", 0, true)
	insertVoice(t, "下架音色", "zh_female_offline", 1, false)

	router := gin.New()
	voice.TestHookMountRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/zsy/voice/list?page=1&page_size=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	payload := decodeEnvelope(t, w.Body.Bytes())
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)
	assert.EqualValues(t, 1, jsonInt(t, data, "total"), "the mounted public face only serves on-shelf voices")
	assert.EqualValues(t, 1, jsonInt(t, data, "pageSize"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
}
