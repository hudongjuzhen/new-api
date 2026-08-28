package runninghub_test

import (
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// HTTP-level tests for the stats / sync-from-channel admin endpoints. The
// plugin's standalone test package has no host DB singleton, so only paths
// that short-circuit before the first DB access are exercised here:
//   - malformed JSON body → rejected by ShouldBindJSON
//   - channelId <= 0      → rejected by SyncChannelApps before db()
//
// CollectStats and the DB-backed sync branches are covered by the
// integration suite once the host DB fixture is wired in.

func newSyncTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/dashboard/zsy/rh/apps/sync-from-channel", runninghub.TestHookSyncApps)
	return r
}

func TestSyncFromChannel_BadJSON_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	r := newSyncTestRouter()
	w, raw := doJSONRaw(t, r, http.MethodPost, "/dashboard/zsy/rh/apps/sync-from-channel", "{NOT JSON")
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "请求体错误")
}

func TestSyncFromChannel_ZeroChannelID_RejectedBeforeDB(t *testing.T) {
	t.Parallel()
	r := newSyncTestRouter()
	w, raw := doJSON(t, r, http.MethodPost, "/dashboard/zsy/rh/apps/sync-from-channel", map[string]any{
		"channelId": 0,
	})
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "channelId")
}

func TestSyncFromChannel_NegativeChannelID_RejectedBeforeDB(t *testing.T) {
	t.Parallel()
	r := newSyncTestRouter()
	w, raw := doJSON(t, r, http.MethodPost, "/dashboard/zsy/rh/apps/sync-from-channel", map[string]any{
		"channelId": -3,
	})
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "channelId")
}
