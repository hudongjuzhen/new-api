package runninghub

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The generation-record status is driven entirely by the query response the
// poller parses here. RH answers a purged/never-created task with an error body
// that carries no status at all, so these tests pin that an upstream error
// closes the record as FAILURE — and that a request-level error (our own broken
// call) does not.

// stuckTaskQueryResponse is the exact body RH returns for tasks 16/17 in the
// generation-records panel: status empty, errorCode 1004, no results.
const stuckTaskQueryResponse = `{
  "taskId": "2100773838112505858",
  "status": "",
  "errorCode": "1004",
  "errorMessage": "Task not found, please check the task ID | 任务不存在或已过期，请检查任务ID",
  "results": null,
  "failedReason": {},
  "usage": null
}`

func TestParseTaskResult_UpstreamErrorWithoutStatusIsFailure(t *testing.T) {
	adaptor := &TaskAdaptor{}

	info, err := adaptor.ParseTaskResult([]byte(stuckTaskQueryResponse))
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, model.TaskStatusFailure, info.Status)
	assert.Equal(t, "100%", info.Progress)
	assert.Equal(t, "2100773838112505858", info.TaskID)
	// failedReason is {} here, so the message must come from errorMessage —
	// never the literal "{}".
	assert.Contains(t, info.Reason, "Task not found, please check the task ID")
	assert.Contains(t, info.Reason, "1004")
	assert.NotEqual(t, "{}", info.Reason)
}

func TestParseTaskResult_RequestLevelErrorKeepsTaskInFlight(t *testing.T) {
	adaptor := &TaskAdaptor{}

	cases := []struct {
		name string
		body string
	}{
		{
			name: "invalid url (1001)",
			body: `{"taskId":"t1","status":"","errorCode":"1001","errorMessage":"Invalid URL"}`,
		},
		{
			name: "unparseable body (1007)",
			body: `{"taskId":"t1","status":"","errorCode":"1007","errorMessage":"taskId must not be null"}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := adaptor.ParseTaskResult([]byte(tc.body))
			require.NoError(t, err)
			require.NotNil(t, info)
			assert.Equal(t, model.TaskStatusQueued, info.Status)
		})
	}
}

func TestParseTaskResult_StatusWinsOverErrorFields(t *testing.T) {
	adaptor := &TaskAdaptor{}

	// A live task that carries an error-ish field must not be closed early:
	// only the status decides QUEUED/RUNNING/SUCCESS/FAILED.
	cases := []struct {
		name       string
		body       string
		wantStatus string
	}{
		{
			name:       "queued with stale error text",
			body:       `{"taskId":"t1","status":"QUEUED","errorCode":"","errorMessage":"retrying"}`,
			wantStatus: model.TaskStatusQueued,
		},
		{
			name:       "running",
			body:       `{"taskId":"t1","status":"RUNNING"}`,
			wantStatus: model.TaskStatusInProgress,
		},
		{
			name:       "success with results",
			body:       `{"taskId":"t1","status":"SUCCESS","results":[{"url":"https://x/y.png"}]}`,
			wantStatus: model.TaskStatusSuccess,
		},
		{
			name:       "failed with a string failedReason",
			body:       `{"taskId":"t1","status":"FAILED","failedReason":"content rejected"}`,
			wantStatus: model.TaskStatusFailure,
		},
		{
			name:       "canceled",
			body:       `{"taskId":"t1","status":"CANCELED"}`,
			wantStatus: model.TaskStatusFailure,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := adaptor.ParseTaskResult([]byte(tc.body))
			require.NoError(t, err)
			require.NotNil(t, info)
			assert.Equal(t, tc.wantStatus, info.Status)
			if tc.wantStatus == model.TaskStatusSuccess {
				assert.Equal(t, "https://x/y.png", info.Url)
				assert.Equal(t, "100%", info.Progress)
			}
		})
	}
}

func TestPickFailureReason_PrefersRealReasons(t *testing.T) {
	assert.Equal(t,
		"content rejected",
		pickFailureReason(&QueryResp{FailedReason: []byte(`"content rejected"`)}),
	)
	assert.Equal(t,
		"[1004] task gone",
		pickFailureReason(&QueryResp{FailedReason: []byte(`{}`), ErrorCode: "1004", ErrorMessage: "task gone"}),
	)
	assert.Equal(t,
		"runninghub task failed",
		pickFailureReason(&QueryResp{FailedReason: []byte(`{}`)}),
	)
}

func TestParseTaskResult_UsageBecomesCompletionTokens(t *testing.T) {
	adaptor := &TaskAdaptor{}
	body := `{"taskId":"t1","status":"SUCCESS","usage":{"consumeCoins":"51"},"results":[{"url":"https://x/y.png"}]}`

	info, err := adaptor.ParseTaskResult([]byte(body))
	require.NoError(t, err)
	assert.Equal(t, 51, info.CompletionTokens)
}
