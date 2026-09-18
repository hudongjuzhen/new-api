package runninghub_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// unmarshalInto decodes a response body into the caller's struct.
func unmarshalInto(t *testing.T, raw []byte, out any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(raw, out))
}

// ---------------------------------------------------------------------------
// Adaptor answers: the submit response a generic relay caller receives, and the
// OpenAI video object GET /v1/videos/{task_id} returns.
// ---------------------------------------------------------------------------

// newAdaptorContext builds the gin context + recorder one adaptor call needs.
func newAdaptorContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", nil)
	return c, w
}

// newRelayInfo builds the minimal relay info the adaptor reads: the pre-generated
// public id plus a non-nil channel meta (the adaptor's debug path reads the
// channel fields).
func newRelayInfo(publicTaskID string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		TaskRelayInfo: &relaycommon.TaskRelayInfo{PublicTaskID: publicTaskID},
		ChannelMeta:   &relaycommon.ChannelMeta{},
	}
}

func rhSubmitResponse(t *testing.T, upstreamTaskID string) *http.Response {
	t.Helper()
	body := `{"taskId":"` + upstreamTaskID + `","status":"RUNNING","clientId":"c-1"}`
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

// TestDoResponse_WritesGatewayAnswerForRawRelayCall pins the fix that makes the
// generic relay endpoints usable at all: without a controller-owned response the
// caller must still learn the public task id it polls with.
func TestDoResponse_WritesGatewayAnswerForRawRelayCall(t *testing.T) {
	c, w := newAdaptorContext(t)

	upstreamID, raw, taskErr := (&runninghub.TaskAdaptor{}).DoResponse(c, rhSubmitResponse(t, "rh-up-1"), newRelayInfo("task_public_1"))

	require.Nil(t, taskErr)
	assert.Equal(t, "rh-up-1", upstreamID)
	assert.Contains(t, string(raw), "rh-up-1", "the raw upstream body is handed back for the task row")

	written := &runninghub.SubmitResponse{}
	unmarshalInto(t, w.Body.Bytes(), written)
	assert.Equal(t, "task_public_1", written.TaskID)
	assert.Equal(t, "task_public_1", written.TaskIDCompat)
	assert.Equal(t, "rh-up-1", written.UpstreamTaskID)
	assert.Equal(t, runninghub.StatusRunning, written.Status)
}

// TestDoResponse_StaysSilentWhenRunHandlerAnswers keeps the plugin's own run
// endpoint from appending a second JSON body to the dashboard envelope.
func TestDoResponse_StaysSilentWhenRunHandlerAnswers(t *testing.T) {
	c, w := newAdaptorContext(t)
	runninghub.TestHookMarkControllerResponds(c)

	_, _, taskErr := (&runninghub.TaskAdaptor{}).DoResponse(c, rhSubmitResponse(t, "rh-up-2"), newRelayInfo("task_public_2"))

	require.Nil(t, taskErr)
	assert.Empty(t, w.Body.String(), "the adaptor must not write a body the handler owns")
}

// TestDoResponse_KeepsUpstreamErrorsAsTaskErrors makes sure the new response
// write did not swallow the error path.
func TestDoResponse_KeepsUpstreamErrorsAsTaskErrors(t *testing.T) {
	c, w := newAdaptorContext(t)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(`{"code":803,"msg":"nodeId not found","errorCode":"803","errorMessage":"node mismatch"}`)),
	}

	_, _, taskErr := (&runninghub.TaskAdaptor{}).DoResponse(c, resp, newRelayInfo("task_public_3"))

	require.NotNil(t, taskErr)
	assert.Equal(t, "rh_node_mismatch", taskErr.Code)
	assert.Empty(t, w.Body.String(), "a failed submit is answered by the host, not the adaptor")
}

// TestConvertToOpenAIVideo covers the shapes /v1/videos/{task_id} can return.
func TestConvertToOpenAIVideo(t *testing.T) {
	seconds := 8.0
	tests := []struct {
		name                 string
		task                 *model.Task
		wantStatus           string
		wantProgress         int
		wantURL              string
		wantResults          []string
		wantError            string
		wantSeconds          string
		wantNoErrorOnSuccess bool
	}{
		{
			name: "success exposes every result url",
			task: &model.Task{
				TaskID:   "task_ok",
				Status:   model.TaskStatusSuccess,
				Progress: "100%",
				Data: []byte(`{"taskId":"rh-up-ok","status":"SUCCESS","results":[` +
					`{"url":"https://rh/out-0.png","nodeId":"9"},{"url":"https://rh/out-1.png","nodeId":"10"}]}`),
				PrivateData: model.TaskPrivateData{
					BillingContext: &model.TaskBillingContext{OtherRatios: map[string]float64{"seconds": seconds}},
				},
			},
			wantStatus:           dto.VideoStatusCompleted,
			wantProgress:         100,
			wantURL:              "https://rh/out-0.png",
			wantResults:          []string{"https://rh/out-0.png", "https://rh/out-1.png"},
			wantSeconds:          "8",
			wantNoErrorOnSuccess: true,
		},
		{
			name: "running run reports progress and no results yet",
			task: &model.Task{
				TaskID:   "task_running",
				Status:   model.TaskStatusInProgress,
				Progress: "50%",
				Data:     []byte(`{"taskId":"rh-up-run","status":"RUNNING"}`),
			},
			wantStatus:   dto.VideoStatusInProgress,
			wantProgress: 50,
			wantNoErrorOnSuccess: true,
		},
		{
			name: "failure surfaces the upstream reason",
			task: &model.Task{
				TaskID:     "task_failed",
				Status:     model.TaskStatusFailure,
				Progress:   "100%",
				FailReason: "本地兜底原因",
				Data:       []byte(`{"taskId":"rh-up-fail","status":"FAILED","errorCode":"1501","errorMessage":"内容审核未通过"}`),
			},
			wantStatus:   dto.VideoStatusFailed,
			wantProgress: 100,
			wantError:    "[1501] 内容审核未通过",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := (&runninghub.TaskAdaptor{}).ConvertToOpenAIVideo(tc.task)
			require.NoError(t, err)

			video := &dto.OpenAIVideo{}
			unmarshalInto(t, raw, video)
			assert.Equal(t, tc.task.TaskID, video.ID)
			assert.Equal(t, tc.wantStatus, video.Status)
			assert.Equal(t, tc.wantProgress, video.Progress)

			if tc.wantNoErrorOnSuccess {
				assert.Nil(t, video.Error)
			}
			if tc.wantError != "" {
				require.NotNil(t, video.Error)
				assert.Equal(t, tc.wantError, video.Error.Message)
			}
			if tc.wantURL != "" {
				assert.Equal(t, tc.wantURL, video.Metadata["url"])
			}
			if tc.wantResults != nil {
				results, ok := video.Metadata["results"].([]any)
				require.True(t, ok, "results metadata must be a list")
				got := make([]string, 0, len(results))
				for _, entry := range results {
					got = append(got, entry.(string))
				}
				assert.Equal(t, tc.wantResults, got)
			}
			if tc.wantSeconds != "" {
				assert.Equal(t, tc.wantSeconds, video.Seconds)
			}
		})
	}
}

// TestGetModelList_PublishesAppUpstreamIDs pins the channel model list to the
// published apps, so the channel form's model fetch cannot inject placeholder
// names that no channel can route.
func TestGetModelList_PublishesAppUpstreamIDs(t *testing.T) {
	const upstreamID = "9301-model-list"
	env := newRHITestEnv(t, upstreamID)

	env.createApp("model-list-published", upstreamID, true, 1000, 1.0)
	hidden, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:               "model-list-draft",
		Kind:               runninghub.AppKindWorkflow,
		UpstreamID:         "9302-draft",
		Published:          false,
		ModelBaseRateRatio: 1.0,
	})
	require.NoError(t, err)
	require.NotZero(t, hidden.ID)

	models := (&runninghub.TaskAdaptor{}).GetModelList()
	assert.Equal(t, []string{upstreamID}, models)
}
