package runninghub_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaykitdto "github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Per-channel concurrency gate + queue.
//
// The invariants protected here:
//   - capacity is per channel and equals ChannelSettings.MaxConcurrency
//     (0 = unlimited, never saturated)
//   - a slot is occupied while a dispatched task is non-terminal, and freed as
//     soon as the task reaches SUCCESS/FAILURE; a task that is only queued does
//     not occupy one
//   - a submit that finds every channel of the site saturated is accepted as a
//     QUEUED record (pre-charged), and the dispatcher admits queued tasks in
//     accept order once a slot frees
//   - queued and running tasks can both be cancelled, with the pre-charge
//     refunded in both cases
// ---------------------------------------------------------------------------

const (
	itestChannelBID = 9102
	// itestPerCallQuota mirrors the per-call price createSiteApp configures.
	itestPerCallQuota = 1_000
)

// setChannelMaxConcurrency stores max_concurrency in the channel's settings
// JSON exactly the way the admin API does.
func (e *rhITestEnv) setChannelMaxConcurrency(channelID, maxConcurrency int) {
	e.t.Helper()
	settings, err := common.Marshal(relaykitdto.ChannelSettings{MaxConcurrency: maxConcurrency})
	require.NoError(e.t, err)
	require.NoError(e.t, e.db.Model(&model.Channel{}).
		Where("id = ?", channelID).
		Update("setting", string(settings)).Error)
}

// createChannel adds a second enabled RunningHub channel to the site pool.
func (e *rhITestEnv) createChannel(id int, name string, weight uint) {
	e.t.Helper()
	prio := int64(0)
	baseURL := e.rh.server.URL
	require.NoError(e.t, e.db.Create(&model.Channel{
		Id: id, Type: constant.ChannelTypeRunningHub, Key: fmt.Sprintf("rh-key-%d", id),
		Status: common.ChannelStatusEnabled, Name: name, Group: "default",
		BaseURL: &baseURL, Priority: &prio, Weight: &weight,
	}).Error)
}

// insertTask records a task of the given status on a channel, which is what the
// gate counts as an occupied slot.
func (e *rhITestEnv) insertTask(taskID string, channelID int, status model.TaskStatus) *model.Task {
	e.t.Helper()
	task := &model.Task{
		TaskID:     taskID,
		UserId:     itestUserID,
		Platform:   constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeRunningHub)),
		ChannelId:  channelID,
		Status:     status,
		Action:     "rh_ai_app",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{OriginModelName: taskID},
	}
	require.NoError(e.t, e.db.Create(task).Error)
	return task
}

// createSiteApp publishes an app bound to a site, which is what engages the
// site-scoped channel selector (and therefore the gate).
func (e *rhITestEnv) createSiteApp(name, upstreamID, site string) uint {
	e.t.Helper()
	view, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:       name,
		Kind:       runninghub.AppKindAICApp,
		UpstreamID: upstreamID,
		Site:       site,
		Published:  true,
		ParamSchema: []rhparser.SchemaParam{
			{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true},
		},
		PerCallBilling:     true,
		FixedQuotaPerCall:  itestPerCallQuota,
		ModelBaseRateRatio: 1.0,
	})
	require.NoError(e.t, err)
	return view.ID
}

// submitAppBody issues the submit request without any testify assertion, so it
// is safe to call from a goroutine (require must stay on the test goroutine).
func (e *rhITestEnv) submitAppBody(appID uint) (int, []byte) {
	body, err := json.Marshal(map[string]any{
		"values": map[string]any{"122.prompt": "a tiny castle"},
	})
	if err != nil {
		return 0, nil
	}
	req := httptest.NewRequest(
		http.MethodPost,
		fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		bytes.NewReader(body),
	)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	return w.Code, w.Body.Bytes()
}

// cancelTask calls the user-facing cancel endpoint.
func (e *rhITestEnv) cancelTask(taskID string) (int, *apiEnvelope) {
	e.t.Helper()
	w, raw := doJSON(e.t, e.router, http.MethodPost,
		fmt.Sprintf("/api/zsy/rh/apps/task/%s/cancel", taskID), nil)
	return w.Code, parseAPIEnvelope(e.t, raw)
}

// reserveSlot is a shorthand for one gate reservation attempt.
func reserveSlot(t *testing.T) (int, func()) {
	t.Helper()
	id, release, err := runninghub.TestHookReserveSlotOnce(constant.ChannelTypeRunningHub, "default")
	require.NoError(t, err)
	return id, release
}

func TestConcurrencyGate_FillsChannelCapacityThenSaturates(t *testing.T) {
	env := newRHITestEnv(t, "9101-gate-app")
	// A (the env's channel) allows one task, B allows two.
	env.setChannelMaxConcurrency(itestChannelID, 1)
	env.createChannel(itestChannelBID, "rh-itest-b", 100)
	env.setChannelMaxConcurrency(itestChannelBID, 2)

	grants := make(map[int]int)
	releases := make(map[int][]func())
	for i := 0; i < 3; i++ {
		id, release := reserveSlot(t)
		require.NotZero(t, id, "grant %d must land on a channel with free capacity", i+1)
		grants[id]++
		releases[id] = append(releases[id], release)
		t.Cleanup(release)
	}
	// Three slots exist in total (1 on A + 2 on B), spread by weight, not by
	// order — so assert the capacity split rather than a fixed sequence.
	assert.Equal(t, map[int]int{itestChannelID: 1, itestChannelBID: 2}, grants)

	// The fourth concurrent submit finds every channel saturated.
	id, release := reserveSlot(t)
	assert.Zero(t, id, "a saturated site must grant no slot")
	release()

	// Releasing one slot (the task reached a terminal state upstream) restores
	// exactly that channel's capacity: A is the only channel that can take it
	// now, because B's two slots are still held.
	releases[itestChannelID][0]()
	id, release = reserveSlot(t)
	assert.Equal(t, itestChannelID, id, "the freed slot must be reusable")
	release()
}

func TestConcurrencyGate_UnlimitedChannelNeverSaturates(t *testing.T) {
	env := newRHITestEnv(t, "9102-gate-unlimited")

	// Precondition: the channel has no max_concurrency setting at all, which is
	// the documented "unlimited" default.
	var channel model.Channel
	require.NoError(t, env.db.First(&channel, "id = ?", itestChannelID).Error)
	require.Zero(t, channel.GetSetting().MaxConcurrency)

	// Five concurrent reservations all land on the channel…
	for i := 0; i < 5; i++ {
		id, release := reserveSlot(t)
		require.Equal(t, itestChannelID, id)
		t.Cleanup(release)
	}

	// …and a sixth one still succeeds: an unset cap never engages the queue.
	id, release := reserveSlot(t)
	assert.Equal(t, itestChannelID, id)
	release()
}

func TestConcurrencyGate_CountsOnlyDispatchedNonTerminalTasks(t *testing.T) {
	env := newRHITestEnv(t, "9103-gate-inflight")
	env.setChannelMaxConcurrency(itestChannelID, 1)

	running := env.insertTask("rh-gate-running", itestChannelID, model.TaskStatusInProgress)
	env.insertTask("rh-gate-done", itestChannelID, model.TaskStatusSuccess)
	env.insertTask("rh-gate-failed", itestChannelID, model.TaskStatusFailure)
	// A task that is still waiting in the queue has no channel and has not
	// reached the upstream: it must not consume a slot (otherwise the queue
	// would deadlock behind itself).
	queued := env.insertTask("rh-gate-queued", 0, model.TaskStatusQueued)
	queued.Progress = "100%"
	require.NoError(t, env.db.Model(&model.Task{}).Where("id = ?", queued.ID).
		Update("progress", "100%").Error)

	// One in-flight task fills the single slot; terminal and queued tasks are
	// both free.
	id, release := reserveSlot(t)
	assert.Zero(t, id, "an in-flight task must occupy the channel's only slot")
	release()

	// Polling marks the task terminal → the slot frees.
	require.NoError(t, env.db.Model(&model.Task{}).
		Where("id = ?", running.ID).
		Update("status", model.TaskStatusSuccess).Error)
	id, release = reserveSlot(t)
	assert.Equal(t, itestChannelID, id, "a terminal task must release its slot")
	release()
}

func TestConcurrencyQueue_SaturatedSubmitIsRecordedAsQueued(t *testing.T) {
	const upstreamID = "9104-queue-app"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	appID := env.createSiteApp("itest-queue", upstreamID, "cn")

	var submitMu sync.Mutex
	submitCount := 0
	env.rh.submitResp = func(string) (int, any) {
		submitMu.Lock()
		submitCount++
		seq := submitCount
		submitMu.Unlock()
		return http.StatusOK, runninghub.SubmitResp{
			TaskID: fmt.Sprintf("rh-queue-task-%d", seq),
			Status: runninghub.StatusQueued,
		}
	}
	upstreamSubmits := func() int {
		submitMu.Lock()
		defer submitMu.Unlock()
		return submitCount
	}

	// First run takes the only slot.
	code, envResp, data := env.submitApp(appID)
	require.Equal(t, http.StatusOK, code, "first submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "first submit failed: %s", envResp.Message)
	firstTaskID, _ := data["taskId"].(string)
	require.NotEmpty(t, firstTaskID)
	require.Equal(t, 1, upstreamSubmits())
	require.Equal(t, itestInitQuota-itestPerCallQuota, env.userQuota())

	// Second run must be accepted immediately as a QUEUED record: no waiting, no
	// upstream call, and the pre-charge is still taken.
	code, envResp, data = env.submitApp(appID)
	require.Equal(t, http.StatusOK, code, "queued submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "queued submit failed: %s", envResp.Message)
	assert.Equal(t, true, data["queued"])
	queuedTaskID, _ := data["taskId"].(string)
	require.NotEmpty(t, queuedTaskID)
	assert.NotEqual(t, firstTaskID, queuedTaskID)

	queued := env.taskByTaskID(queuedTaskID)
	assert.Equal(t, model.TaskStatusQueued, string(queued.Status))
	assert.Equal(t, itestPerCallQuota, queued.Quota, "a queued task is pre-charged like a direct one")
	assert.Equal(t, 1, upstreamSubmits(), "a queued task must not reach the upstream yet")
	assert.Equal(t, 2*itestPerCallQuota, itestInitQuota-env.userQuota())
	// The core poller only picks up tasks whose progress is not 100%; a queued
	// task must stay out of it (it has no upstream id to query).
	assert.Equal(t, "100%", queued.Progress)

	count, err := runninghub.TestHookQueuedTaskCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestConcurrencyQueue_DispatchesInAcceptOrder(t *testing.T) {
	const upstreamID = "9105-queue-order"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	appID := env.createSiteApp("itest-queue-order", upstreamID, "cn")

	var submitMu sync.Mutex
	submitted := make([]string, 0, 3)
	env.rh.submitResp = func(appID string) (int, any) {
		submitMu.Lock()
		defer submitMu.Unlock()
		taskID := fmt.Sprintf("rh-order-task-%d", len(submitted)+1)
		submitted = append(submitted, taskID)
		return http.StatusOK, runninghub.SubmitResp{TaskID: taskID, Status: runninghub.StatusQueued}
	}
	submittedIDs := func() []string {
		submitMu.Lock()
		defer submitMu.Unlock()
		return append([]string(nil), submitted...)
	}
	env.rh.queryResp = func(string) (int, any) {
		return http.StatusOK, runninghub.QueryResp{
			TaskID: "rh-order-task-1",
			Status: runninghub.StatusSuccess,
			Usage:  &runninghub.TaskUsage{ConsumeCoins: "0"},
			Results: []runninghub.TaskResult{
				{URL: "https://img.rh-itest.local/order-1.png"},
			},
		}
	}

	_, envResp, data := env.submitApp(appID)
	require.True(t, envResp.Success, "first submit failed: %s", envResp.Message)
	firstTaskID, _ := data["taskId"].(string)

	// Two more runs queue up behind the first, in this order.
	var queuedIDs []string
	for i := 0; i < 2; i++ {
		_, envResp, data = env.submitApp(appID)
		require.True(t, envResp.Success, "queued submit failed: %s", envResp.Message)
		require.Equal(t, true, data["queued"])
		id, _ := data["taskId"].(string)
		queuedIDs = append(queuedIDs, id)
	}
	require.Len(t, submittedIDs(), 1, "queued runs must not reach the upstream while the slot is held")

	// The first run finishes → one slot frees → the dispatcher admits the oldest
	// queued task only.
	env.pollOnce()
	require.Equal(t, model.TaskStatusSuccess, string(env.taskByTaskID(firstTaskID).Status))
	runninghub.TestHookDispatchQueue()
	require.Len(t, submittedIDs(), 2, "the freed slot must admit exactly one queued task")
	assert.Equal(t, model.TaskStatusInProgress, string(env.taskByTaskID(queuedIDs[0]).Status))
	assert.Equal(t, model.TaskStatusQueued, string(env.taskByTaskID(queuedIDs[1]).Status),
		"the later task must wait for its own slot")
	dispatched := env.taskByTaskID(queuedIDs[0])
	assert.Equal(t, itestChannelID, dispatched.ChannelId)
	assert.NotEmpty(t, dispatched.PrivateData.UpstreamTaskID)
	assert.Equal(t, "rh-order-task-2", dispatched.PrivateData.UpstreamTaskID)

	count, err := runninghub.TestHookQueuedTaskCount()
	require.NoError(t, err)
	assert.Equal(t, int64(1), count, "only the later task may remain queued")
}

func TestConcurrencyQueue_CancelQueuedTaskRefundsAndNeverSubmits(t *testing.T) {
	const upstreamID = "9106-queue-cancel"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	appID := env.createSiteApp("itest-queue-cancel", upstreamID, "cn")

	var submitMu sync.Mutex
	submitCount := 0
	env.rh.submitResp = func(string) (int, any) {
		submitMu.Lock()
		defer submitMu.Unlock()
		submitCount++
		return http.StatusOK, runninghub.SubmitResp{
			TaskID: fmt.Sprintf("rh-cancel-task-%d", submitCount),
			Status: runninghub.StatusQueued,
		}
	}

	_, envResp, _ := env.submitApp(appID)
	require.True(t, envResp.Success, "first submit failed: %s", envResp.Message)
	_, envResp, data := env.submitApp(appID)
	require.True(t, envResp.Success, "queued submit failed: %s", envResp.Message)
	queuedTaskID, _ := data["taskId"].(string)
	require.Equal(t, 2*itestPerCallQuota, itestInitQuota-env.userQuota())

	code, cancelResp := env.cancelTask(queuedTaskID)
	require.Equal(t, http.StatusOK, code, "cancel failed: %s", cancelResp.Message)
	require.True(t, cancelResp.Success, "cancel failed: %s", cancelResp.Message)

	cancelled := env.taskByTaskID(queuedTaskID)
	assert.Equal(t, model.TaskStatusFailure, string(cancelled.Status))
	assert.Equal(t, "用户取消", cancelled.FailReason)
	assert.Zero(t, cancelled.Quota, "the refund clears the recorded quota")
	// Only the first run stays charged.
	assert.Equal(t, itestInitQuota-itestPerCallQuota, env.userQuota())
	assert.Equal(t, itestInitQuota-itestPerCallQuota, env.tokenRemain())

	count, err := runninghub.TestHookQueuedTaskCount()
	require.NoError(t, err)
	assert.Zero(t, count, "the cancelled task must leave the queue")

	// A later dispatch pass must not resurrect it.
	runninghub.TestHookDispatchQueue()
	submitMu.Lock()
	defer submitMu.Unlock()
	assert.Equal(t, 1, submitCount, "a cancelled queued task must never reach the upstream")
}

func TestConcurrencyQueue_ExpiryRefundsAndFails(t *testing.T) {
	const upstreamID = "9107-queue-expiry"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	appID := env.createSiteApp("itest-queue-expiry", upstreamID, "cn")
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: "rh-expiry-task-1", Status: runninghub.StatusQueued}
	}

	_, envResp, _ := env.submitApp(appID)
	require.True(t, envResp.Success, "first submit failed: %s", envResp.Message)
	_, envResp, data := env.submitApp(appID)
	require.True(t, envResp.Success, "queued submit failed: %s", envResp.Message)
	queuedTaskID, _ := data["taskId"].(string)

	// Age the entry and shrink the expiry window: one dispatch pass must fail and
	// refund it instead of leaving the pre-charge parked forever.
	require.NoError(t, env.db.Model(&runninghub.RhQueuedTask{}).
		Where("task_id = ?", queuedTaskID).
		Update("created_at", time.Now().Unix()-3600).Error)
	previous := runninghub.TestHookSetQueueMaxAgeMinutes(1)
	t.Cleanup(func() { runninghub.TestHookSetQueueMaxAgeMinutes(previous) })

	runninghub.TestHookDispatchQueue()

	expired := env.taskByTaskID(queuedTaskID)
	assert.Equal(t, model.TaskStatusFailure, string(expired.Status))
	assert.Contains(t, expired.FailReason, "排队超时")
	assert.Equal(t, itestInitQuota-itestPerCallQuota, env.userQuota(), "an expired queued task must be refunded")
	count, err := runninghub.TestHookQueuedTaskCount()
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestConcurrencyQueue_CancelRunningTaskStopsUpstream(t *testing.T) {
	const upstreamID = "9108-run-cancel"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	appID := env.createSiteApp("itest-run-cancel", upstreamID, "cn")
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: "rh-run-cancel-1", Status: runninghub.StatusRunning}
	}
	// The upstream refuses to cancel: the local record must stay untouched and
	// no refund may be issued.
	env.rh.cancelResp = func(taskID string) (int, any) {
		return http.StatusOK, runninghub.FlatResp{Code: 817, Msg: "APIKEY_TASK_CANCEL_NOT_ALLOWED"}
	}

	_, envResp, data := env.submitApp(appID)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	taskID, _ := data["taskId"].(string)
	require.Equal(t, itestInitQuota-itestPerCallQuota, env.userQuota())

	code, cancelResp := env.cancelTask(taskID)
	require.Equal(t, http.StatusOK, code)
	assert.False(t, cancelResp.Success)
	assert.Contains(t, cancelResp.Message, "上游不允许取消")
	running := env.taskByTaskID(taskID)
	assert.Equal(t, model.TaskStatusInProgress, string(running.Status))
	assert.Equal(t, itestPerCallQuota, running.Quota, "a refused cancel must not refund")
	assert.Equal(t, itestInitQuota-itestPerCallQuota, env.userQuota())

	// Upstream agrees this time.
	env.rh.cancelResp = func(taskID string) (int, any) {
		require.Equal(t, "rh-run-cancel-1", taskID)
		return http.StatusOK, runninghub.FlatResp{Code: 0, Msg: "success"}
	}
	code, cancelResp = env.cancelTask(taskID)
	require.Equal(t, http.StatusOK, code, "cancel failed: %s", cancelResp.Message)
	require.True(t, cancelResp.Success, "cancel failed: %s", cancelResp.Message)

	cancelled := env.taskByTaskID(taskID)
	assert.Equal(t, model.TaskStatusFailure, string(cancelled.Status))
	assert.Equal(t, "用户取消", cancelled.FailReason)
	assert.Equal(t, itestInitQuota, env.userQuota(), "a confirmed cancel refunds the pre-charge")
	assert.Equal(t, itestInitQuota, env.tokenRemain())
}

func TestConcurrencyCap_SiteLessAppReportsBusyInsteadOfQueueing(t *testing.T) {
	const upstreamID = "9109-legacy-app"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	// The env's ability row lets the host's model→channel selection find the
	// channel for a site-less app.
	env.insertTask("rh-legacy-running", itestChannelID, model.TaskStatusInProgress)

	view, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:       "itest-legacy",
		Kind:       runninghub.AppKindAICApp,
		UpstreamID: upstreamID,
		Published:  true,
		ParamSchema: []rhparser.SchemaParam{
			{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true},
		},
		PerCallBilling:     true,
		FixedQuotaPerCall:  itestPerCallQuota,
		ModelBaseRateRatio: 1.0,
	})
	require.NoError(t, err)

	// A site-less app has no candidate pool to queue against, so the cap is
	// enforced by rejecting the request instead of waiting.
	code, envResp, _ := env.submitApp(view.ID)
	require.Equal(t, http.StatusTooManyRequests, code)
	assert.Contains(t, envResp.Message, "并发")
	assert.Equal(t, itestInitQuota, env.userQuota())
}
