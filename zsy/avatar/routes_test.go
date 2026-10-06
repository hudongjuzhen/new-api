package avatar_test

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/QuantumNous/new-api/zsy/avatar"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Route tree — the URLs documented in docs/zsy-avatarplaza-api.md. Registering the
// real tree also proves the mount does not panic on gin's route tree (a conflict
// there would take the whole gateway down at boot).
// ---------------------------------------------------------------------------

func TestMountRoutes_PublishesTheDocumentedURLs(t *testing.T) {
	router := gin.New()
	require.NotPanics(t, func() { avatar.TestHookMountRoutes(router) })

	registered := make([]string, 0, 10)
	for _, route := range router.Routes() {
		registered = append(registered, route.Method+" "+route.Path)
	}
	sort.Strings(registered)

	assert.Equal(t, []string{
		"DELETE /dashboard/zsy/avatar/:id",
		"GET /api/zsy/avatar/:id",
		"GET /api/zsy/avatar/list",
		"GET /dashboard/zsy/avatar/:id",
		"GET /dashboard/zsy/avatar/export",
		"GET /dashboard/zsy/avatar/list",
		"POST /dashboard/zsy/avatar",
		"POST /dashboard/zsy/avatar/import",
		"PUT /dashboard/zsy/avatar/:id",
	}, registered)
}

func TestMountRoutes_PublicListAnswersThroughTheRealChain(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t)
	insertAvatar(t, "下架形象", 2, false)

	router := gin.New()
	avatar.TestHookMountRoutes(router)

	req := httptest.NewRequest(http.MethodGet, "/api/zsy/avatar/list?page=1&page_size=1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	payload := decodeEnvelope(t, w.Body.Bytes())
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)
	assert.EqualValues(t, 1, jsonInt(t, data, "total"), "the mounted public face only serves on-shelf personas")
	assert.EqualValues(t, 1, jsonInt(t, data, "pageSize"))
	assert.Contains(t, w.Header().Get("Cache-Control"), "no-store")
}
