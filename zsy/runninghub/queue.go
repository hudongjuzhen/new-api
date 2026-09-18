package runninghub

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
)

// Gateway-side queue for RunningHub tasks.
//
// RunningHub rejects the request that exceeds an account's concurrency instead
// of queueing it, so the gateway queues for it. When a submit arrives and every
// channel of the app's site is at its cap (ChannelSettings.MaxConcurrency) the
// task is accepted right away and persisted twice:
//
//   - a host tasks row in status QUEUED ("排队中") carrying the pre-charged
//     quota and the billing snapshot — the record the user sees and cancels,
//   - one RhQueuedTask row carrying the upstream request body built at accept
//     time, so the eventual submit is byte-identical to a direct one.
//
// A dispatcher admits queued tasks strictly by accept time (FIFO) as slots free
// up. A slot is occupied for the whole task lifetime, which is what the
// upstream actually counts; see concurrency.go.
//
// Queued rows are created with progress="100%" and channel_id=0 on purpose:
// the core poller only picks up tasks whose progress is not 100%, and a task
// with no channel yet has no upstream state to query. Dispatch flips the row to
// IN_PROGRESS with a real channel id, which is what makes it pollable again.

const (
	// queueDispatchInterval is how often the dispatcher looks for a free slot.
	queueDispatchInterval = 3 * time.Second
	// queueDispatchBatch bounds one dispatch pass.
	queueDispatchBatch = 20
	// queueDispatchAttempts bounds how many different channels one dispatch may
	// try before giving up, mirroring the submit path's retry allowance.
	queueDispatchAttempts = 3
)

// queueMaxAgeMinutes is how long a task may sit in the queue before it is failed
// and refunded. Without it a site whose channels are all disabled would hold the
// user's quota forever. A variable so tests can shrink the window.
var queueMaxAgeMinutes = 720

// RhQueuedTask is the pending upstream submit of one accepted task.
type RhQueuedTask struct {
	ID       uint   `gorm:"primarykey" json:"id"`
	TaskID   string `gorm:"type:varchar(191);not null;uniqueIndex" json:"taskId"`
	UserID   int    `gorm:"index" json:"userId"`
	Platform string `gorm:"type:varchar(30)" json:"platform"`
	// Site scopes the dispatch to a channel pool (cn → type 61, intl → type 62),
	// exactly like the submit path's site routing.
	Site string `gorm:"type:varchar(16);index" json:"site"`
	// SubmitPath is the upstream path to POST to; PathAbsolute marks a fully
	// qualified URL (model-API apps may carry one).
	SubmitPath   string `gorm:"type:varchar(512)" json:"submitPath"`
	PathAbsolute bool   `json:"pathAbsolute"`
	// Body is the upstream request body as built when the task was accepted.
	Body string `gorm:"type:text" json:"body"`
	// Group is the token group of the accepting request, used by the billing
	// log if the task is refunded without ever running.
	Group string `gorm:"type:varchar(50)" json:"group"`

	CreatedAt int64 `gorm:"index" json:"createdAt"`
}

// TableName keeps the plugin table name stable and namespaced.
func (RhQueuedTask) TableName() string { return "rh_queued_tasks" }

// submitPathFor mirrors the URL composition of TaskAdaptor.BuildRequestURL for
// the three app kinds, minus the base URL (which only becomes known once a
// channel is picked at dispatch time). Keep both in sync.
func submitPathFor(kind AppKind, upstreamID string) (path string, absolute bool) {
	switch kind {
	case AppKindWorkflow:
		return PathSubmitWorkflow + upstreamID, false
	case AppKindModel:
		if strings.HasPrefix(upstreamID, "http") {
			return upstreamID, true
		}
		if strings.HasPrefix(upstreamID, "/") {
			return upstreamID, false
		}
		return openAPIV2Prefix + "/" + upstreamID, false
	default:
		return PathSubmitAICApp + upstreamID, false
	}
}

// enqueueTask persists one queued dispatch entry and wakes the dispatcher.
func enqueueTask(entry *RhQueuedTask) error {
	if entry.CreatedAt == 0 {
		entry.CreatedAt = time.Now().Unix()
	}
	if err := db().Create(entry).Error; err != nil {
		return err
	}
	wakeQueueDispatcher()
	return nil
}

// queuedTasksInOrder returns the queue in accept order (oldest first), which is
// also the order tasks are admitted in.
func queuedTasksInOrder(limit int) ([]RhQueuedTask, error) {
	var entries []RhQueuedTask
	query := db().Order("created_at asc, id asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&entries).Error
	return entries, err
}

func deleteQueuedTask(taskID string) {
	if err := db().Where("task_id = ?", taskID).Delete(&RhQueuedTask{}).Error; err != nil {
		common.SysError(fmt.Sprintf("runninghub queue: delete entry %s failed: %s", taskID, err.Error()))
	}
}

// queuedTaskByTaskID returns the queue entry of a task, or nil when it is not
// waiting in the queue. Find (not First) keeps a miss out of the GORM error log:
// cancelling a running task has no entry, which is the normal case.
func queuedTaskByTaskID(taskID string) (*RhQueuedTask, error) {
	var entries []RhQueuedTask
	if err := db().Where("task_id = ?", taskID).Limit(1).Find(&entries).Error; err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, nil
	}
	return &entries[0], nil
}

// queuedTaskCount reports how many tasks wait in the queue (admin stats).
func queuedTaskCount() (int64, error) {
	var total int64
	err := db().Model(&RhQueuedTask{}).Count(&total).Error
	return total, err
}

// queueWake nudges the dispatcher so a freshly queued task does not wait for
// the next tick. Cross-instance wakeups rely on the ticker.
var queueWake = make(chan struct{}, 1)

func wakeQueueDispatcher() {
	select {
	case queueWake <- struct{}{}:
	default:
	}
}

// startQueueDispatcher launches the background admission loop. Non-master nodes
// skip the pass (the role is resolved after plugin init, hence the per-tick
// check); with several masters each dispatch claims its task row with a status
// CAS, so a task reaches the upstream exactly once.
func startQueueDispatcher() {
	go func() {
		ticker := time.NewTicker(queueDispatchInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
			case <-queueWake:
			}
			if !common.IsMasterNode || model.DB == nil {
				continue
			}
			dispatchQueuedTasks(context.Background())
		}
	}()
}

// dispatchQueuedTasks expires stale entries and admits everything that fits
// into the sites' free slots, oldest task first.
func dispatchQueuedTasks(ctx context.Context) {
	expireStaleQueuedTasks(ctx)

	entries, err := queuedTasksInOrder(queueDispatchBatch)
	if err != nil {
		common.SysError("runninghub queue: load failed: " + err.Error())
		return
	}
	for i := range entries {
		if ctx.Err() != nil {
			return
		}
		dispatchQueuedTask(ctx, &entries[i])
	}
}

// expireStaleQueuedTasks fails and refunds tasks that waited longer than
// queueMaxAgeMinutes. RunningHub tasks are exempt from the host's timeout sweep,
// so the queue owns this bound.
func expireStaleQueuedTasks(ctx context.Context) {
	cutoff := time.Now().Unix() - int64(queueMaxAgeMinutes)*60
	var stale []RhQueuedTask
	if err := db().Where("created_at < ?", cutoff).Order("created_at asc").Limit(50).Find(&stale).Error; err != nil {
		common.SysError("runninghub queue: stale scan failed: " + err.Error())
		return
	}
	reason := fmt.Sprintf("排队超时（%d 分钟）", queueMaxAgeMinutes)
	for i := range stale {
		entry := &stale[i]
		task, _ := loadTask(entry)
		failQueuedTask(ctx, task, reason)
		deleteQueuedTask(entry.TaskID)
	}
}

// dispatchQueuedTask tries to admit one queued task. It returns silently while
// the task's site has no free slot — the next pass retries. Because entries are
// scanned oldest first, an earlier task always takes a slot ahead of a later
// one that could use the same pool.
func dispatchQueuedTask(ctx context.Context, entry *RhQueuedTask) {
	channelType := siteToChannelType(entry.Site)
	if channelType == 0 {
		task, _ := loadTask(entry)
		failQueuedTask(ctx, task, "无法解析站点: "+entry.Site)
		deleteQueuedTask(entry.TaskID)
		return
	}
	candidates, err := siteCandidates(channelType, "")
	if err != nil {
		common.SysError("runninghub queue: candidate load failed: " + err.Error())
		return
	}

	tried := make(map[int]bool, queueDispatchAttempts)
	for attempt := 0; attempt < queueDispatchAttempts; attempt++ {
		remaining := make([]rhCandidate, 0, len(candidates))
		for _, cand := range candidates {
			if !tried[cand.channel.Id] {
				remaining = append(remaining, cand)
			}
		}
		if len(remaining) == 0 {
			return
		}
		channel, slot, reserveErr := reserveSlot(remaining, 0)
		if reserveErr != nil {
			common.SysError("runninghub queue: reserve failed: " + reserveErr.Error())
			return
		}
		if channel == nil {
			// Every channel of the site is still at its cap: stay queued.
			return
		}
		tried[channel.Id] = true

		task, claimed := claimQueuedTask(entry)
		if !claimed {
			// The task is no longer ours to dispatch: either another dispatcher
			// won the claim, or a user cancel moved it on.
			settleUnclaimedQueueTask(ctx, task)
			slot.release()
			deleteQueuedTask(entry.TaskID)
			return
		}

		submitErr := submitQueuedTask(entry, task, channel)
		// The slot is released only now: the row becomes visible to the
		// non-terminal task count as soon as it carries the channel id.
		slot.release()
		if submitErr == nil {
			deleteQueuedTask(entry.TaskID)
			return
		}
		if attempt == queueDispatchAttempts-1 {
			failQueuedTask(ctx, task, "排队任务提交失败: "+submitErr.Error())
			deleteQueuedTask(entry.TaskID)
			return
		}
		// A channel-level failure is worth one more channel, exactly like the
		// submit path's retry: put the task back in QUEUED and try the next one.
		requeueClaimedTask(task, submitErr)
	}
}

// loadTask reads the task row a queue entry belongs to.
func loadTask(entry *RhQueuedTask) (*model.Task, bool) {
	task, exist, err := model.GetByTaskId(entry.UserID, entry.TaskID)
	if err != nil {
		common.SysError("runninghub queue: task lookup failed: " + err.Error())
		return nil, false
	}
	return task, exist
}

// claimQueuedTask moves the task row QUEUED → IN_PROGRESS with a status CAS, so
// two dispatchers (or a cancel racing the dispatcher) cannot both act on it.
// The returned task is the claimed row, or the current row when the claim lost.
func claimQueuedTask(entry *RhQueuedTask) (*model.Task, bool) {
	task, exist := loadTask(entry)
	if !exist {
		return nil, false
	}
	previous := task.Status
	task.Status = model.TaskStatusInProgress
	task.StartTime = time.Now().Unix()
	task.Progress = "10%"
	won, err := task.UpdateWithStatus(previous)
	if err != nil {
		common.SysError("runninghub queue: claim failed: " + err.Error())
		return task, false
	}
	return task, won && previous == model.TaskStatusQueued
}

// requeueClaimedTask returns a claimed task to QUEUED so a retry (or a later
// dispatch pass) can pick it up again.
func requeueClaimedTask(task *model.Task, cause error) {
	if task == nil {
		return
	}
	task.Status = model.TaskStatusQueued
	task.Progress = "100%"
	if _, err := task.UpdateWithStatus(model.TaskStatusInProgress); err != nil {
		common.SysError("runninghub queue: requeue failed: " + err.Error())
	} else {
		common.SysLog(fmt.Sprintf("runninghub queue: task %s requeued after channel failure: %s", task.TaskID, cause.Error()))
	}
}

// settleUnclaimedQueueTask handles a queued task that left the queue without
// this dispatcher running it. Two cases reach here: a claim window that was
// abandoned (process restart between claim and submit), or a task the core
// poller failed for an unrelated reason. Neither ever reached the upstream, so
// the pre-charge must go back; RefundTaskQuota is idempotent (it clears the
// quota it refunds) and the status CAS keeps a live task untouched.
func settleUnclaimedQueueTask(ctx context.Context, task *model.Task) {
	if task == nil || task.PrivateData.UpstreamTaskID != "" {
		// A task with an upstream id really ran: its own lifecycle owns the money.
		return
	}
	switch task.Status {
	case model.TaskStatusQueued, model.TaskStatusInProgress:
		failQueuedTask(ctx, task, "排队任务未能派发")
	case model.TaskStatusFailure:
		if task.Quota > 0 {
			service.RefundTaskQuota(ctx, task, "排队任务未派发（任务已结束）")
		}
	}
}

// submitQueuedTask replays the stored body against the granted channel and
// records the upstream task id on the task row.
func submitQueuedTask(entry *RhQueuedTask, task *model.Task, channel *model.Channel) error {
	baseURL, key := channelBaseAndKey(channel)
	settings := channel.GetSetting()
	httpClient, err := service.GetHttpClientWithProxySettings(settings.Proxy, settings)
	if err != nil {
		return fmt.Errorf("http client: %w", err)
	}
	client := NewClientForType(channel.Type, baseURL, key, httpClient)

	path := entry.SubmitPath
	if entry.PathAbsolute {
		// A model-API target carries its own host; the channel base URL must not
		// be prefixed.
		client.BaseURL = ""
	}
	resp, err := client.SubmitRaw(path, []byte(entry.Body))
	if err != nil {
		return err
	}
	if resp.TaskID == "" {
		return fmt.Errorf("上游未返回 taskId (code=%s msg=%s)", resp.ErrorCode, resp.ErrorMessage)
	}

	task.ChannelId = channel.Id
	task.PrivateData.UpstreamTaskID = resp.TaskID
	// Pin the key that actually ran the task: polling must query it with the
	// same account, and multi-key channels rotate keys per request.
	task.PrivateData.Key = key
	task.Progress = "20%"
	if raw, mErr := common.Marshal(resp); mErr == nil {
		task.Data = raw
	}
	if err := task.Update(); err != nil {
		return fmt.Errorf("persist dispatch result: %w", err)
	}
	// The pre-charge was recorded while no channel was chosen, so the channel
	// that finally ran the task is credited here; its later settle or refund
	// balances against the same channel.
	model.UpdateChannelUsedQuota(channel.Id, task.Quota)

	common.SysLog(fmt.Sprintf(
		"runninghub queue: dispatched task %s on channel #%d (upstream=%s, waited=%ds)",
		task.TaskID, channel.Id, resp.TaskID, time.Now().Unix()-entry.CreatedAt,
	))
	return nil
}

// failQueuedTask marks a queued (or claimed) task as failed, refunds the
// pre-charge and records the reason. It reports whether this caller won the
// status CAS: false means a concurrent actor (a user cancel, or the dispatcher
// claiming the task) already moved the row on.
func failQueuedTask(ctx context.Context, task *model.Task, reason string) bool {
	if task == nil {
		return false
	}
	previous := task.Status
	if previous != model.TaskStatusQueued && previous != model.TaskStatusInProgress {
		if previous == model.TaskStatusFailure && task.Quota > 0 && task.PrivateData.UpstreamTaskID == "" {
			service.RefundTaskQuota(ctx, task, "排队任务未派发（任务已结束）")
		}
		return false
	}
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.FailReason = reason
	won, err := task.UpdateWithStatus(previous)
	if err != nil {
		common.SysError("runninghub queue: fail update failed: " + err.Error())
		return false
	}
	if !won {
		return false
	}
	service.RefundTaskQuota(ctx, task, reason)
	return true
}

// channelBaseAndKey resolves the base URL and a rotated key for a channel: the
// same pair the submit path hands to the relay layer, without a gin context.
func channelBaseAndKey(channel *model.Channel) (string, string) {
	baseURL := constant.ChannelBaseURLs[channel.Type]
	if custom := channel.GetBaseURL(); custom != "" {
		baseURL = custom
	}
	key := channel.Key
	if next, _, err := channel.GetNextEnabledKey(); err == nil && next != "" {
		key = next
	}
	return baseURL, key
}
