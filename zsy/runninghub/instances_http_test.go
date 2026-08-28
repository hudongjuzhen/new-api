package runninghub_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTTP-level tests for the admin instance endpoints. The plugin's standalone
// test package has no host DB singleton, so every case here short-circuits
// before the first DB access:
//   - malformed JSON body    → rejected by ShouldBindJSON
//   - zero/negative refs     → rejected by validateInstanceRefs before db()
//   - non-numeric :id param  → rejected by parseAppIDParam
//
// DB-backed paths are covered by the integration suite once the host DB
// fixture is wired in.

func newInstancesTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/dashboard/zsy/rh/instances")
	g.GET("", runninghub.TestHookListInstances)
	g.POST("", runninghub.TestHookCreateInstance)
	g.PUT("/:id", runninghub.TestHookUpdateInstance)
	g.DELETE("/:id", runninghub.TestHookDeleteInstance)
	g.POST("/:id/keypool-refresh", runninghub.TestHookRefreshKeypool)
	return r
}

// doJSONRaw issues a request with a raw (possibly malformed) body.
func doJSONRaw(t *testing.T, r http.Handler, method, url string, body string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, w.Body.Bytes()
}

func TestInstanceCreate_BadJSON_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	r := newInstancesTestRouter()
	w, raw := doJSONRaw(t, r, http.MethodPost, "/dashboard/zsy/rh/instances", "{NOT JSON")
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "请求体错误")
}

func TestInstanceCreate_ZeroAppID_RejectedBeforeDB(t *testing.T) {
	t.Parallel()
	r := newInstancesTestRouter()
	w, raw := doJSON(t, r, http.MethodPost, "/dashboard/zsy/rh/instances", map[string]any{
		"appId":     0,
		"channelId": 5,
	})
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "appId")
}

func TestInstanceCreate_ZeroChannelID_RejectedBeforeDB(t *testing.T) {
	t.Parallel()
	r := newInstancesTestRouter()
	w, raw := doJSON(t, r, http.MethodPost, "/dashboard/zsy/rh/instances", map[string]any{
		"appId":     1,
		"channelId": 0,
	})
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "channelId")
}

func TestInstanceUpdate_BadIDParam_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	r := newInstancesTestRouter()
	w, raw := doJSON(t, r, http.MethodPut, "/dashboard/zsy/rh/instances/abc", map[string]any{
		"weight": 2,
	})
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "id")
}

func TestInstanceDelete_BadIDParam_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	r := newInstancesTestRouter()
	w, raw := doJSON(t, r, http.MethodDelete, "/dashboard/zsy/rh/instances/abc", nil)
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "id")
}

func TestInstanceKeypoolRefresh_BadIDParam_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	r := newInstancesTestRouter()
	w, raw := doJSON(t, r, http.MethodPost, "/dashboard/zsy/rh/instances/abc/keypool-refresh", nil)
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "id")
}
