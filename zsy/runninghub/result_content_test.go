package runninghub_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Result-file preview proxy (controller_result_content.go).
//
// The renderer needs the text of a result file, which the browser cannot read
// from RunningHub's storage (no CORS headers). The invariants protected here:
//   - only URLs belonging to the caller's own task are fetched
//   - a task of another user is not reachable at all
//   - only text-ish content is served, and the response is capped in size
// ---------------------------------------------------------------------------

const resultContentRoute = "/api/zsy/rh/apps/task/:task_id/content"

// allowLocalFetchForTest opens the SSRF policy for the fake file server, which
// listens on 127.0.0.1:<random port> and would otherwise be blocked.
func allowLocalFetchForTest(t *testing.T, rawURL string) {
	t.Helper()
	fetchSetting := system_setting.GetFetchSetting()
	original := *fetchSetting
	t.Cleanup(func() { *fetchSetting = original })

	parsed, err := url.Parse(rawURL)
	require.NoError(t, err)
	fetchSetting.AllowPrivateIp = true
	fetchSetting.AllowedPorts = []string{parsed.Port()}
}

// newResultFileServer serves one file body with the given content type.
func newResultFileServer(t *testing.T, contentType, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

// insertResultTask records a finished task whose stored data carries the given
// result URLs, which is the shape the poller persists.
func (e *rhITestEnv) insertResultTask(taskID string, resultURLs ...string) *model.Task {
	e.t.Helper()
	entries := make([]map[string]any, 0, len(resultURLs))
	for _, resultURL := range resultURLs {
		entries = append(entries, map[string]any{"url": resultURL, "outputType": "text"})
	}
	payload, err := json.Marshal(map[string]any{"status": "SUCCESS", "results": entries})
	require.NoError(e.t, err)
	task := &model.Task{
		TaskID:     taskID,
		UserId:     itestUserID,
		Platform:   constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeRunningHub)),
		ChannelId:  itestChannelID,
		Status:     model.TaskStatusSuccess,
		Action:     "rh_ai_app",
		Progress:   "100%",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{OriginModelName: taskID},
		Data:       payload,
	}
	require.NoError(e.t, e.db.Create(task).Error)
	return task
}

func TestResultContent_ReturnsTextOfOwnResult(t *testing.T) {
	service.InitHttpClient()
	env := newRHITestEnv(t, "9200-content-app")
	files := newResultFileServer(t, "text/plain; charset=utf-8", "hello rh\nsecond line")
	allowLocalFetchForTest(t, files.URL)
	resultURL := files.URL + "/results/out.txt"
	env.insertResultTask("rh-content-task", resultURL)
	env.router.GET(resultContentRoute, runninghub.TestHookGetTaskResultContent)

	w, raw := doJSON(t, env.router, http.MethodGet,
		"/api/zsy/rh/apps/task/rh-content-task/content?url="+url.QueryEscape(resultURL), nil)
	require.Equal(t, http.StatusOK, w.Code)

	envelope := parseAPIEnvelope(t, raw)
	require.True(t, envelope.Success, "preview failed: %s", envelope.Message)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok, "unexpected preview payload: %s", raw)
	assert.Equal(t, "hello rh\nsecond line", data["content"])
	assert.Equal(t, false, data["truncated"])
}

func TestResultContent_RejectsURLThatIsNotAOwnResult(t *testing.T) {
	service.InitHttpClient()
	env := newRHITestEnv(t, "9201-content-guard")
	files := newResultFileServer(t, "text/plain", "secret")
	allowLocalFetchForTest(t, files.URL)
	env.insertResultTask("rh-content-owned", files.URL+"/results/mine.txt")
	env.router.GET(resultContentRoute, runninghub.TestHookGetTaskResultContent)

	var upstreamCalls int
	attacker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("should never be fetched"))
	}))
	t.Cleanup(attacker.Close)

	w, raw := doJSON(t, env.router, http.MethodGet,
		"/api/zsy/rh/apps/task/rh-content-owned/content?url="+url.QueryEscape(attacker.URL+"/etc/passwd"), nil)
	require.Equal(t, http.StatusOK, w.Code)

	envelope := parseAPIEnvelope(t, raw)
	assert.False(t, envelope.Success, "a foreign URL must be rejected")
	assert.Contains(t, envelope.Message, "仅支持预览")
	assert.Zero(t, upstreamCalls, "the proxy must not fetch a URL the task does not own")
}

func TestResultContent_RejectsTaskOfAnotherUser(t *testing.T) {
	service.InitHttpClient()
	env := newRHITestEnv(t, "9202-content-owner")
	files := newResultFileServer(t, "text/plain", "owned by someone else")
	allowLocalFetchForTest(t, files.URL)
	resultURL := files.URL + "/results/private.txt"
	env.insertResultTask("rh-content-foreign", resultURL)

	// Same handler, a different authenticated user.
	otherUser := gin.New()
	otherUser.Use(func(c *gin.Context) {
		c.Set(string(constant.ContextKeyUserId), itestUserID+1)
		c.Next()
	})
	otherUser.GET(resultContentRoute, runninghub.TestHookGetTaskResultContent)

	w, raw := doJSON(t, otherUser, http.MethodGet,
		"/api/zsy/rh/apps/task/rh-content-foreign/content?url="+url.QueryEscape(resultURL), nil)
	require.Equal(t, http.StatusOK, w.Code)

	envelope := parseAPIEnvelope(t, raw)
	assert.False(t, envelope.Success)
	assert.Contains(t, envelope.Message, "任务不存在或无权访问")
}

func TestResultContent_RejectsNonTextFile(t *testing.T) {
	service.InitHttpClient()
	env := newRHITestEnv(t, "9203-content-binary")
	files := newResultFileServer(t, "image/png", "\x89PNG\r\n\x1a\n binary")
	allowLocalFetchForTest(t, files.URL)
	resultURL := files.URL + "/results/out.png"
	env.insertResultTask("rh-content-binary", resultURL)
	env.router.GET(resultContentRoute, runninghub.TestHookGetTaskResultContent)

	w, raw := doJSON(t, env.router, http.MethodGet,
		"/api/zsy/rh/apps/task/rh-content-binary/content?url="+url.QueryEscape(resultURL), nil)
	require.Equal(t, http.StatusOK, w.Code)

	envelope := parseAPIEnvelope(t, raw)
	assert.False(t, envelope.Success, "a binary result has no inline preview")
	assert.Contains(t, envelope.Message, "不支持在线预览")
}

func TestResultContent_TruncatesOversizedText(t *testing.T) {
	service.InitHttpClient()
	env := newRHITestEnv(t, "9204-content-truncate")
	// One byte over the 1 MiB cap.
	const cap_ = 1 << 20
	body := strings.Repeat("a", cap_+1)
	files := newResultFileServer(t, "text/plain", body)
	allowLocalFetchForTest(t, files.URL)
	resultURL := files.URL + "/results/long.txt"
	env.insertResultTask("rh-content-long", resultURL)
	env.router.GET(resultContentRoute, runninghub.TestHookGetTaskResultContent)

	w, raw := doJSON(t, env.router, http.MethodGet,
		"/api/zsy/rh/apps/task/rh-content-long/content?url="+url.QueryEscape(resultURL), nil)
	require.Equal(t, http.StatusOK, w.Code)

	envelope := parseAPIEnvelope(t, raw)
	require.True(t, envelope.Success, "preview failed: %s", envelope.Message)
	data, ok := envelope.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, data["truncated"], "an oversized file must be reported as truncated")
	content, _ := data["content"].(string)
	assert.Len(t, content, cap_, "only the capped prefix is returned")
}
