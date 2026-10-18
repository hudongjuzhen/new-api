package runninghub_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// relayTaskDTO converts a stored task row to the payload every task API answers
// with (the site's /api/task and the client's /api/task/self both go through
// this function).
func relayTaskDTO(task *model.Task) *dto.TaskDto {
	return relay.TaskModel2Dto(task)
}

// ---------------------------------------------------------------------------
// Caller-supplied remark (备注).
//
// A run may carry a short note in the caller's own words — "生成 30 秒视频",
// "生成 4 张图片". It is display text and nothing else: it must NOT reach
// RunningHub, and it must NOT touch pricing. What it has to do is land on the
// task row, because that row is what the user actually reads afterwards:
//
//	任务日志（/usage-logs/task → 详情）  ← TaskDto.remark
//	账号的使用记录（/api/task/self）    ← TaskDto.remark / properties.remark
// ---------------------------------------------------------------------------

// newRemarkEnv boots the standard integration stack for a remark test: a
// published per-call app plus a fake upstream that answers every submit with a
// successful run.
func newRemarkEnv(t *testing.T, upstreamID, upstreamTaskID string) (*rhITestEnv, uint) {
	t.Helper()
	env := newRHITestEnv(t, upstreamID)
	enableRunningTask(env, upstreamTaskID)
	return env, env.createApp("remark app", upstreamID, true, itestPerCallQuota, 1.0)
}

// submitWithRemark runs the user-side submit endpoint with an explicit payload
// and returns the public task id plus the decoded task row.
func (e *rhITestEnv) submitWithRemark(t *testing.T, appID uint, payload map[string]any) string {
	t.Helper()
	code, envResp, data := e.submitAppWithPayload(appID, payload)
	require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	taskID, _ := data["taskId"].(string)
	require.NotEmpty(t, taskID)
	return taskID
}

// TestAppRemark_ReachesTheTaskRowAndNeverTheUpstream pins the whole contract of
// the field: the note is stored on the task, and it does not leak into the
// upstream submit body.
func TestAppRemark_ReachesTheTaskRowAndNeverTheUpstream(t *testing.T) {
	const upstreamID = "9501-remark-app"
	env, appID := newRemarkEnv(t, upstreamID, "rh-remark-run-1")

	taskID := env.submitWithRemark(t, appID, map[string]any{
		"values": map[string]any{"122.prompt": "一只小城堡"},
		"remark": "生成 30 秒视频",
	})

	task := env.taskByTaskID(taskID)
	assert.Equal(t, "生成 30 秒视频", task.Properties.Remark,
		"the note rides on the task row — that row is what 任务日志 and /api/task/self show")

	require.NotEmpty(t, env.rh.submits, "the run must have reached the upstream")
	last := env.rh.submits[len(env.rh.submits)-1]
	assert.NotContains(t, last.Body, "生成 30 秒视频",
		"the note is display text: RunningHub must never receive it")
	assert.NotContains(t, last.Body, "remark",
		"not even the field name belongs in the upstream body")
}

// TestAppRemark_IsExposedToBothReaders pins the two readers the note exists for:
// the task DTO the site's 任务日志 renders, and the account usage records the
// client reads from /api/task/self.
func TestAppRemark_IsExposedToBothReaders(t *testing.T) {
	const upstreamID = "9502-remark-dto"
	env, appID := newRemarkEnv(t, upstreamID, "rh-remark-run-dto")

	taskID := env.submitWithRemark(t, appID, map[string]any{
		"values": map[string]any{"122.prompt": "一只小城堡"},
		"remark": "生成 4 张图片",
	})
	task := env.taskByTaskID(taskID)
	require.Equal(t, "生成 4 张图片", task.Properties.Remark)

	dto := relayTaskDTO(task)
	assert.Equal(t, "生成 4 张图片", dto.Remark, "TaskDto.remark is what the task-log 详情 column reads")

	// The same value has to survive the JSON hop, both flat (the convenient
	// field) and inside properties (the durable one). One of the two silently
	// going missing is exactly the bug a client cannot distinguish from
	// "the caller did not send a note".
	raw, err := common.Marshal(dto)
	require.NoError(t, err)
	var decoded struct {
		Remark     string `json:"remark"`
		Properties struct {
			Remark string `json:"remark"`
		} `json:"properties"`
	}
	require.NoError(t, common.Unmarshal(raw, &decoded), "payload: %s", raw)
	assert.Equal(t, "生成 4 张图片", decoded.Remark)
	assert.Equal(t, "生成 4 张图片", decoded.Properties.Remark)
}

// TestAppRemark_LegacyNoteAliasIsAccepted keeps the `note` spelling working:
// the field was called note before it was called remark, and a caller that
// still sends the old name must not silently lose its statement line.
func TestAppRemark_LegacyNoteAliasIsAccepted(t *testing.T) {
	const upstreamID = "9503-remark-note-alias"
	env, appID := newRemarkEnv(t, upstreamID, "rh-remark-run-alias")

	taskID := env.submitWithRemark(t, appID, map[string]any{
		"values": map[string]any{"122.prompt": "一只小城堡"},
		"note":   "生成 4 张图片",
	})
	assert.Equal(t, "生成 4 张图片", env.taskByTaskID(taskID).Properties.Remark)
}

// TestAppRemark_IsCollapsedToOneLine: the note is rendered in a single-line cell
// and inside a JSON log line, so line breaks and runs of spaces are normalised
// instead of being stored verbatim.
func TestAppRemark_IsCollapsedToOneLine(t *testing.T) {
	const upstreamID = "9504-remark-normalize"
	env, appID := newRemarkEnv(t, upstreamID, "rh-remark-run-normalize")

	taskID := env.submitWithRemark(t, appID, map[string]any{
		"values": map[string]any{"122.prompt": "一只小城堡"},
		"remark": "  生成   30  秒视频\n\n（4K）\t ",
	})
	assert.Equal(t, "生成 30 秒视频 （4K）", env.taskByTaskID(taskID).Properties.Remark)
}

// TestAppRemark_IsBounded: an unbounded note would let one billing row become a
// document, and the record is meant to be readable at a glance.
func TestAppRemark_IsBounded(t *testing.T) {
	const upstreamID = "9505-remark-bounded"
	env, appID := newRemarkEnv(t, upstreamID, "rh-remark-run-bounded")

	taskID := env.submitWithRemark(t, appID, map[string]any{
		"values": map[string]any{"122.prompt": "一只小城堡"},
		"remark": strings.Repeat("画", 500),
	})

	stored := env.taskByTaskID(taskID).Properties.Remark
	// 120 runes + the ellipsis that says "the caller wrote more".
	assert.Equal(t, 121, utf8.RuneCountInString(stored), "stored: %q", stored)
	assert.True(t, strings.HasSuffix(stored, "…"), "the truncation must be visible, got %q", stored)
}

// TestAppRemark_BlankNoteIsTreatedAsAbsent: a note that carries no text must not
// become an empty badge on the record.
func TestAppRemark_BlankNoteIsTreatedAsAbsent(t *testing.T) {
	for name, payload := range map[string]map[string]any{
		"absent": {"values": map[string]any{"122.prompt": "一只小城堡"}},
		"blank": {
			"values": map[string]any{"122.prompt": "一只小城堡"},
			"remark": " \n\t ",
		},
		"empty alias": {
			"values": map[string]any{"122.prompt": "一只小城堡"},
			"note":   "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			upstreamID := "9506-remark-blank-" + strings.ReplaceAll(name, " ", "-")
			env, appID := newRemarkEnv(t, upstreamID, "rh-remark-run-blank")
			taskID := env.submitWithRemark(t, appID, payload)

			task := env.taskByTaskID(taskID)
			assert.Empty(t, task.Properties.Remark)
			assert.Empty(t, relayTaskDTO(task).Remark)
		})
	}
}

// TestAppRemark_SurvivesTheQueuedPath: a run accepted while every channel of the
// site is saturated is recorded before it is dispatched, so the note has to be
// written on that first insert — the queued record is the only row the user sees
// until the dispatcher picks it up.
func TestAppRemark_SurvivesTheQueuedPath(t *testing.T) {
	const upstreamID = "9507-remark-queue"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	appID := env.createSiteApp("remark queue app", upstreamID, "cn")
	enableRunningTask(env, "rh-remark-queue-run")

	// First run takes the only slot.
	firstID := env.submitWithRemark(t, appID, map[string]any{
		"values": map[string]any{"122.prompt": "一只小城堡"},
		"remark": "生成 30 秒视频",
	})
	assert.Equal(t, "生成 30 秒视频", env.taskByTaskID(firstID).Properties.Remark)

	// Second run finds the site saturated and is accepted as a QUEUED record.
	queuedID := env.submitWithRemark(t, appID, map[string]any{
		"values": map[string]any{"122.prompt": "一只小城堡"},
		"remark": "生成 4 张图片",
	})
	queued := env.taskByTaskID(queuedID)
	assert.Equal(t, "QUEUED", string(queued.Status), fmt.Sprintf("task %s", queuedID))
	assert.Equal(t, "生成 4 张图片", queued.Properties.Remark,
		"a queued run must be as self-explanatory as a submitted one")
}
