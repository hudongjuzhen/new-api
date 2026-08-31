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

// HTTP-level tests for the admin app keypool endpoints. The plugin's
// standalone test package has no host DB singleton, so every case here
// short-circuits before the first DB access:
//   - non-numeric :id param → rejected by parseAppIDParam
//
// DB-backed paths are covered by the integration suite once the host DB
// fixture is wired in.

func newKeypoolTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/dashboard/zsy/rh/apps")
	g.GET("/:id/keypool", runninghub.TestHookGetAppKeypool)
	g.POST("/:id/keypool-refresh", runninghub.TestHookRefreshAppKeypool)
	return r
}

func doJSONRaw(t *testing.T, r http.Handler, method, url string, body string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	req := httptest.NewRequest(method, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, w.Body.Bytes()
}

func TestAppKeypoolGet_BadIDParam_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	r := newKeypoolTestRouter()
	w, raw := doJSON(t, r, http.MethodGet, "/dashboard/zsy/rh/apps/abc/keypool", nil)
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "id")
}

func TestAppKeypoolRefresh_BadIDParam_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	r := newKeypoolTestRouter()
	w, raw := doJSON(t, r, http.MethodPost, "/dashboard/zsy/rh/apps/abc/keypool-refresh", nil)
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "id")
}
