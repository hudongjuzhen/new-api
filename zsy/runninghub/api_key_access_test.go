package runninghub_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Third-party (relay API key) access to the RunningHub app API.
//
// These tests mount the plugin's real user-side route group — production
// middleware included — against the in-memory stack from integration_test, so
// the credential classification, the token scoping and the response envelopes
// are all exercised the way a real caller hits them.
// ---------------------------------------------------------------------------

const (
	apiKeyTokenID  = 9201
	apiKey2TokenID = 9202
	apiKeyPlain    = "rhtest"  // stored key; callers send "sk-rhtest"
	apiKey2Plain   = "rhtest2" // stored key; callers send "sk-rhtest2"
	apiKeyHeader   = "Bearer sk-rhtest"
	apiKey2Header  = "Bearer sk-rhtest2"
)

// apiErrorEnvelope is the third-party error shape: real HTTP status plus a
// stable machine-readable code.
type apiErrorEnvelope struct {
	Success bool   `json:"success"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// newAPIKeyEnv boots the integration env (DB + fake upstream) and adds two API
// keys plus a personal access token, then mounts the production route group.
func newAPIKeyEnv(t *testing.T, upstreamID string) (*rhITestEnv, *gin.Engine) {
	t.Helper()
	env := newRHITestEnv(t, upstreamID)

	for _, token := range []struct {
		id   int
		key  string
		name string
	}{
		{apiKeyTokenID, apiKeyPlain, "rh-apikey-1"},
		{apiKey2TokenID, apiKey2Plain, "rh-apikey-2"},
	} {
		require.NoError(t, env.db.Create(&model.Token{
			Id: token.id, UserId: itestUserID, Key: token.key, Name: token.name,
			Status: common.TokenStatusEnabled, RemainQuota: itestInitQuota,
		}).Error)
	}

	router := gin.New()
	runninghub.TestHookMountUserRoutes(router)
	return env, router
}

// doAuthed sends one request with an optional Authorization credential.
func doAuthed(t *testing.T, r http.Handler, method, path string, body any, credential string) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if credential != "" {
		req.Header.Set("Authorization", credential)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w, w.Body.Bytes()
}

// submitWithKey runs one app through the API-key path and returns the public
// task id.
func submitWithKey(t *testing.T, router http.Handler, appID uint, credential string) string {
	t.Helper()
	w, raw := doAuthed(t, router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{"values": map[string]any{"122.prompt": "a tiny castle"}}, credential)
	require.Equal(t, http.StatusOK, w.Code, "submit failed: %s", raw)
	env := parseAPIEnvelope(t, raw)
	require.True(t, env.Success, "submit failed: %s", env.Message)
	data, ok := env.Data.(map[string]any)
	require.True(t, ok, "submit envelope has no data object")
	taskID, _ := data["taskId"].(string)
	require.NotEmpty(t, taskID, "submit must return the public task id")
	return taskID
}

// enableRunningTask answers submit with a running task and the cancel endpoint
// with success, so the cancel path can be exercised end to end.
func enableRunningTask(env *rhITestEnv, upstreamTaskID string) {
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: upstreamTaskID, Status: runninghub.StatusRunning}
	}
	env.rh.cancelResp = func(string) (int, any) {
		return http.StatusOK, runninghub.FlatResp{Code: 0, Msg: "success"}
	}
}

// TestAPIKey_RunQueryAndCancelOwnTask is the third-party happy path: one relay
// API key runs an app, polls the run it paid for and cancels it.
func TestAPIKey_RunQueryAndCancelOwnTask(t *testing.T) {
	const upstreamID = "9201-api-key-app"
	env, router := newAPIKeyEnv(t, upstreamID)
	enableRunningTask(env, "rh-apikey-run-1")
	appID := env.createApp("apikey-app", upstreamID, true, 100_000, 1.0)

	taskID := submitWithKey(t, router, appID, apiKeyHeader)

	task := env.taskByTaskID(taskID)
	assert.Equal(t, apiKeyTokenID, task.PrivateData.TokenId, "the run must bill the calling key")
	assert.Equal(t, itestUserID, task.UserId)

	w, raw := doAuthed(t, router, http.MethodGet, "/api/zsy/rh/apps/task/"+taskID, nil, apiKeyHeader)
	require.Equal(t, http.StatusOK, w.Code, "query failed: %s", raw)
	queried := parseAPIEnvelope(t, raw)
	require.True(t, queried.Success, "query failed: %s", queried.Message)
	queriedTask, ok := queried.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, taskID, queriedTask["task_id"])

	w, raw = doAuthed(t, router, http.MethodPost, "/api/zsy/rh/apps/task/"+taskID+"/cancel", nil, apiKeyHeader)
	require.Equal(t, http.StatusOK, w.Code, "cancel failed: %s", raw)
	cancelled := parseAPIEnvelope(t, raw)
	require.True(t, cancelled.Success, "cancel failed: %s", cancelled.Message)
	assert.Equal(t, string(model.TaskStatusFailure), string(env.taskByTaskID(taskID).Status))

	// The upstream really received the cancel for this run.
	env.rh.mu.Lock()
	cancelCalls := len(env.rh.cancels)
	env.rh.mu.Unlock()
	assert.Equal(t, 1, cancelCalls)
}

// TestAPIKey_CannotReachAnotherKeysTask pins the key scope: a second key of the
// same account may neither read nor cancel the first key's run.
func TestAPIKey_CannotReachAnotherKeysTask(t *testing.T) {
	const upstreamID = "9202-api-key-scope"
	env, router := newAPIKeyEnv(t, upstreamID)
	enableRunningTask(env, "rh-apikey-run-2")
	appID := env.createApp("apikey-scope", upstreamID, true, 100_000, 1.0)

	taskID := submitWithKey(t, router, appID, apiKeyHeader)

	for _, tc := range []struct {
		name   string
		method string
		path   string
	}{
		{"query", http.MethodGet, "/api/zsy/rh/apps/task/" + taskID},
		{"cancel", http.MethodPost, "/api/zsy/rh/apps/task/" + taskID + "/cancel"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w, raw := doAuthed(t, router, tc.method, tc.path, nil, apiKey2Header)
			require.Equal(t, http.StatusForbidden, w.Code, "foreign key must be rejected: %s", raw)
			body := &apiErrorEnvelope{}
			require.NoError(t, json.Unmarshal(raw, body))
			assert.False(t, body.Success)
			assert.Equal(t, runninghub.ErrorCodeTaskNotOwnedByKey, body.Code)
		})
	}

	// The foreign key changed nothing: the run is still live.
	assert.Equal(t, string(model.TaskStatusInProgress), string(env.taskByTaskID(taskID).Status))
}

// TestAPIKey_CannotListAccountHistory documents the deliberate limit: the task
// table carries no index by key, so a key-scoped page cannot be produced and the
// account-wide page must not be handed out either.
func TestAPIKey_CannotListAccountHistory(t *testing.T) {
	const upstreamID = "9203-api-key-list"
	env, router := newAPIKeyEnv(t, upstreamID)
	enableRunningTask(env, "rh-apikey-run-3")
	appID := env.createApp("apikey-list", upstreamID, true, 100_000, 1.0)
	submitWithKey(t, router, appID, apiKeyHeader)

	w, raw := doAuthed(t, router, http.MethodGet, "/api/zsy/rh/apps/tasks", nil, apiKeyHeader)
	require.Equal(t, http.StatusForbidden, w.Code, "listing must be refused for keys: %s", raw)
	body := &apiErrorEnvelope{}
	require.NoError(t, json.Unmarshal(raw, body))
	assert.Equal(t, runninghub.ErrorCodeKeyScopedListUnsupported, body.Code)
}

// TestAPIKey_CannotBillAnotherKey rejects a submit that tries to name a
// different key: the calling key is the only key that may pay.
func TestAPIKey_CannotBillAnotherKey(t *testing.T) {
	const upstreamID = "9204-api-key-bill"
	env, router := newAPIKeyEnv(t, upstreamID)
	enableRunningTask(env, "rh-apikey-run-4")
	appID := env.createApp("apikey-bill", upstreamID, true, 100_000, 1.0)

	w, raw := doAuthed(t, router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{
			"values":  map[string]any{"122.prompt": "a tiny castle"},
			"tokenId": apiKey2TokenID,
		}, apiKeyHeader)
	require.Equal(t, http.StatusBadRequest, w.Code, "unexpected response: %s", raw)
	body := &apiErrorEnvelope{}
	require.NoError(t, json.Unmarshal(raw, body))
	assert.Equal(t, runninghub.ErrorCodeTokenSelectionNotAllowed, body.Code)

	// Nothing was submitted upstream and nothing was charged.
	env.rh.mu.Lock()
	submitCalls := len(env.rh.submits)
	env.rh.mu.Unlock()
	assert.Zero(t, submitCalls)
	assert.Equal(t, itestInitQuota, env.userQuota())
}

// TestAPIKey_RespectsTokenModelLimit pins the permission invariant: the plugin
// selects its own channel instead of going through middleware.Distribute, so the
// key's model allow-list has to be enforced here.
func TestAPIKey_RespectsTokenModelLimit(t *testing.T) {
	const upstreamID = "9205-api-key-limit"
	env, router := newAPIKeyEnv(t, upstreamID)
	enableRunningTask(env, "rh-apikey-run-5")
	appID := env.createApp("apikey-limit", upstreamID, true, 100_000, 1.0)

	require.NoError(t, env.db.Model(&model.Token{}).Where("id = ?", apiKeyTokenID).Updates(map[string]any{
		"model_limits_enabled": true,
		"model_limits":         "some-other-model",
	}).Error)

	w, raw := doAuthed(t, router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{"values": map[string]any{"122.prompt": "a tiny castle"}}, apiKeyHeader)
	require.Equal(t, http.StatusForbidden, w.Code, "model limit must be enforced: %s", raw)
	body := &apiErrorEnvelope{}
	require.NoError(t, json.Unmarshal(raw, body))
	assert.Equal(t, runninghub.ErrorCodeTokenModelForbidden, body.Code)
	assert.Equal(t, itestInitQuota, env.userQuota(), "no charge for a forbidden model")

	// An allow-list that covers the app lets the run through.
	require.NoError(t, env.db.Model(&model.Token{}).Where("id = ?", apiKeyTokenID).
		Update("model_limits", upstreamID).Error)
	submitWithKey(t, router, appID, apiKeyHeader)
}

// TestPersonalAccessToken_KeepsAccountScope proves the composite auth did not
// narrow the dashboard: a personal access token still lists the account's
// history, still reads runs that no key paid for, and still runs apps when it
// names the key that pays (the app center always sends one, see AppRunPayload).
func TestPersonalAccessToken_KeepsAccountScope(t *testing.T) {
	const upstreamID = "9206-pat-scope"
	env, router := newAPIKeyEnv(t, upstreamID)
	enableRunningTask(env, "rh-pat-run-1")
	appID := env.createApp("pat-scope", upstreamID, true, 100_000, 1.0)

	pat := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" // 32 chars, the host's PAT width
	require.NoError(t, env.db.Model(&model.User{}).Where("id = ?", itestUserID).Updates(map[string]any{
		"access_token": pat,
		"role":         common.RoleCommonUser,
	}).Error)

	w, raw := doAuthed(t, router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{
			"values":  map[string]any{"122.prompt": "a tiny castle"},
			"tokenId": apiKeyTokenID,
		}, "Bearer "+pat)
	require.Equal(t, http.StatusOK, w.Code, "PAT submit failed: %s", raw)
	submitted := parseAPIEnvelope(t, raw)
	require.True(t, submitted.Success, "PAT submit failed: %s", submitted.Message)
	taskID, _ := submitted.Data.(map[string]any)["taskId"].(string)
	require.NotEmpty(t, taskID)
	// The dashboard names the key that pays, so the run belongs to that key —
	// and the dashboard may still read it.
	assert.Equal(t, apiKeyTokenID, env.taskByTaskID(taskID).PrivateData.TokenId)

	w, raw = doAuthed(t, router, http.MethodGet, "/api/zsy/rh/apps/tasks", nil, "Bearer "+pat)
	require.Equal(t, http.StatusOK, w.Code, "PAT listing failed: %s", raw)
	require.True(t, parseAPIEnvelope(t, raw).Success)

	w, raw = doAuthed(t, router, http.MethodGet, "/api/zsy/rh/apps/task/"+taskID, nil, "Bearer "+pat)
	require.Equal(t, http.StatusOK, w.Code, "PAT query failed: %s", raw)
	require.True(t, parseAPIEnvelope(t, raw).Success)
}

// TestPublicAppList_NeedsNoCredential keeps the browsing endpoints open: the app
// center renders them for anonymous visitors.
func TestPublicAppList_NeedsNoCredential(t *testing.T) {
	const upstreamID = "9207-public-list"
	_, router := newAPIKeyEnv(t, upstreamID)

	w, raw := doAuthed(t, router, http.MethodGet, "/api/zsy/rh/apps", nil, "")
	require.Equal(t, http.StatusOK, w.Code, "public listing failed: %s", raw)
	require.True(t, parseAPIEnvelope(t, raw).Success)
}

// TestAPIKey_GarbageCredentialIsRejected keeps the fallback honest: an unknown
// credential is not silently treated as a dashboard caller.
func TestAPIKey_GarbageCredentialIsRejected(t *testing.T) {
	const upstreamID = "9208-bad-credential"
	env, router := newAPIKeyEnv(t, upstreamID)
	enableRunningTask(env, "rh-apikey-run-6")
	appID := env.createApp("bad-credential", upstreamID, true, 100_000, 1.0)

	w, _ := doAuthed(t, router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{"values": map[string]any{"122.prompt": "a tiny castle"}}, "Bearer sk-does-not-exist")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, itestInitQuota, env.userQuota())
}
