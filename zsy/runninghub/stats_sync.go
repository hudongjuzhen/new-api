package runninghub

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Admin stats (GET /dashboard/zsy/rh/stats)
//
// One aggregate response for the plugin dashboard: app/instance/keypool
// counters from plugin-owned tables plus task status counters scoped to the
// RunningHub platform in the host tasks table.
// ---------------------------------------------------------------------------

// AppsStats summarises the app template registry.
type AppsStats struct {
	Total     int64            `json:"total"`
	Published int64            `json:"published"`
	ByKind    map[string]int64 `json:"byKind"`
}

// InstancesStats summarises app↔channel bindings.
type InstancesStats struct {
	Total   int64 `json:"total"`
	Enabled int64 `json:"enabled"`
}

// KeypoolStats summarises keypool rows and in-flight submit audits.
type KeypoolStats struct {
	TotalKeys      int64 `json:"totalKeys"`
	EnabledKeys    int64 `json:"enabledKeys"`
	PendingSubmits int64 `json:"pendingSubmits"`
}

// TasksStats summarises host tasks scoped to the RunningHub platform.
type TasksStats struct {
	Total    int64 `json:"total"`
	InFlight int64 `json:"inFlight"`
	Success  int64 `json:"success"`
	Failure  int64 `json:"failure"`
}

// AdminStats is the response shape of GET /dashboard/zsy/rh/stats.
type AdminStats struct {
	Apps      AppsStats      `json:"apps"`
	Instances InstancesStats `json:"instances"`
	Keypool   KeypoolStats   `json:"keypool"`
	Tasks     TasksStats     `json:"tasks"`
}

// rhTaskPlatform is the host `tasks.platform` value for this plugin
// (strconv of the channel type, same convention the submit chain uses).
var rhTaskPlatform = constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeRunningHub))

// CollectStats gathers every counter in a fixed number of aggregate queries.
func CollectStats() (*AdminStats, error) {
	out := &AdminStats{
		Apps:  AppsStats{ByKind: map[string]int64{}},
		Tasks: TasksStats{},
	}

	// apps: total + published + per-kind counts
	if err := db().Model(&App{}).Count(&out.Apps.Total).Error; err != nil {
		return nil, fmt.Errorf("stats apps total: %w", err)
	}
	if err := db().Model(&App{}).Where("published = ?", true).Count(&out.Apps.Published).Error; err != nil {
		return nil, fmt.Errorf("stats apps published: %w", err)
	}
	type kindRow struct {
		Kind AppKind
		C    int64
	}
	var kinds []kindRow
	if err := db().Model(&App{}).Select("kind, count(*) as c").Group("kind").Scan(&kinds).Error; err != nil {
		return nil, fmt.Errorf("stats apps by kind: %w", err)
	}
	for _, k := range kinds {
		out.Apps.ByKind[string(k.Kind)] = k.C
	}

	// instances: total + enabled
	if err := db().Model(&AppInstance{}).Count(&out.Instances.Total).Error; err != nil {
		return nil, fmt.Errorf("stats instances total: %w", err)
	}
	if err := db().Model(&AppInstance{}).Where("enabled = ?", true).Count(&out.Instances.Enabled).Error; err != nil {
		return nil, fmt.Errorf("stats instances enabled: %w", err)
	}

	// keypool: keys + enabled keys + pending audits
	if err := db().Model(&AppInstanceKeyPool{}).Count(&out.Keypool.TotalKeys).Error; err != nil {
		return nil, fmt.Errorf("stats keypool total: %w", err)
	}
	if err := db().Model(&AppInstanceKeyPool{}).Where("enabled = ?", true).Count(&out.Keypool.EnabledKeys).Error; err != nil {
		return nil, fmt.Errorf("stats keypool enabled: %w", err)
	}
	if err := db().Model(&KeypoolPending{}).Where("state = ?", keypoolStatePending).Count(&out.Keypool.PendingSubmits).Error; err != nil {
		return nil, fmt.Errorf("stats keypool pending: %w", err)
	}

	// tasks scoped to this platform, grouped by status
	type statusRow struct {
		Status model.TaskStatus
		C      int64
	}
	var statuses []statusRow
	if err := db().Model(&model.Task{}).
		Select("status, count(*) as c").
		Where("platform = ?", string(rhTaskPlatform)).
		Group("status").
		Scan(&statuses).Error; err != nil {
		return nil, fmt.Errorf("stats tasks by status: %w", err)
	}
	for _, s := range statuses {
		out.Tasks.Total += s.C
		switch {
		case s.Status == model.TaskStatusSuccess:
			out.Tasks.Success = s.C
		case s.Status == model.TaskStatusFailure:
			out.Tasks.Failure = s.C
		}
	}
	out.Tasks.InFlight = out.Tasks.Total - out.Tasks.Success - out.Tasks.Failure
	return out, nil
}

// ---------------------------------------------------------------------------
// Sync-from-channel (POST /dashboard/zsy/rh/apps/sync-from-channel)
//
// RunningHub does not expose an upstream "list apps" API (verified during the
// §3.9 probe), so the sync reconciles against the channel's own model
// registry instead — the same registry Distribute uses for routing:
//
//  A. channel.models → plugin: every channel model that resolves to an app
//     (exact UpstreamID match, or the dev-plan `rh-aiapp-<id>` style prefix)
//     gets an AppInstance row if one does not exist yet.
//  B. plugin → channel.models: every app with a live instance on this
//     channel gets its UpstreamID appended to channel.models when missing,
//     then the channel is saved through the host Channel.Update() so the
//     abilities table is rebuilt.
//
// The operation is idempotent: running it twice yields zero changes.
// ---------------------------------------------------------------------------

// ChannelSyncItem reports what happened to one channel model / app pair.
type ChannelSyncItem struct {
	// Model is the channel model name (or the appended UpstreamID).
	Model string `json:"model"`
	// Action ∈ bound_instance | synced_to_channel | ok | orphan_model.
	Action string `json:"action"`
	AppID  uint   `json:"appId,omitempty"`
}

// ChannelSyncResult summarises one sync run.
type ChannelSyncResult struct {
	ChannelID            int64             `json:"channelId"`
	InstancesCreated     int64             `json:"instancesCreated"`
	ModelsSynced         int64             `json:"modelsSynced"`
	ChannelModelsUpdated bool              `json:"channelModelsUpdated"`
	Items                []ChannelSyncItem `json:"items"`
}

// sync actions
const (
	syncActionBoundInstance = "bound_instance"
	syncActionSyncedChannel = "synced_to_channel"
	syncActionOK            = "ok"
	syncActionOrphanModel   = "orphan_model"
)

// splitChannelModels parses the channel's comma-separated models list
// (same convention as model.Channel.GetModelList) with dedupe.
func splitChannelModels(models string) []string {
	raw := strings.Split(strings.Trim(models, ","), ",")
	out := make([]string, 0, len(raw))
	seen := make(map[string]bool, len(raw))
	for _, m := range raw {
		if m = strings.TrimSpace(m); m == "" || seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, m)
	}
	return out
}

// modelToUpstreamID resolves a channel model name to the app's UpstreamID.
// The current plugin convention uses the bare UpstreamID as the model name;
// the dev-plan's `rh-aiapp-<id>` / `rh-workflow-<id>` style prefixes are
// accepted as a fallback so hand-copied channel entries still bind.
func modelToUpstreamID(m string) string {
	for _, p := range []string{"rh-aiapp-", "rh-workflow-", "rh-model-"} {
		if strings.HasPrefix(m, p) {
			return strings.TrimPrefix(m, p)
		}
	}
	return m
}

// SyncChannelApps runs the bidirectional channel↔apps reconciliation.
func SyncChannelApps(channelID int64) (*ChannelSyncResult, error) {
	if channelID <= 0 {
		return nil, fmt.Errorf("channelId 必须为正整数")
	}
	ch, err := model.GetChannelById(int(channelID), true)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("渠道不存在 (id=%d)", channelID)
		}
		return nil, fmt.Errorf("load channel %d: %w", channelID, err)
	}
	if ch.Type != pluginChannelType {
		return nil, fmt.Errorf("渠道 %d 不是 RunningHub 渠道 (type=%d)", channelID, ch.Type)
	}

	result := &ChannelSyncResult{ChannelID: channelID, Items: []ChannelSyncItem{}}
	models := splitChannelModels(ch.Models)
	inModels := make(map[string]bool, len(models))
	for _, m := range models {
		inModels[m] = true
	}

	// app registry: UpstreamID -> app (lowest id wins on duplicates)
	var apps []App
	if err := db().Select("id, kind, upstream_id").Order("id asc").Find(&apps).Error; err != nil {
		return nil, fmt.Errorf("load runninghub apps: %w", err)
	}
	appByUpstream := make(map[string]*App, len(apps))
	appByID := make(map[uint]*App, len(apps))
	for i := range apps {
		app := &apps[i]
		if _, exists := appByUpstream[app.UpstreamID]; !exists {
			appByUpstream[app.UpstreamID] = app
		}
		appByID[app.ID] = app
	}

	// live instances already bound to this channel
	var instances []AppInstance
	if err := db().Where("channel_id = ?", channelID).Find(&instances).Error; err != nil {
		return nil, fmt.Errorf("load runninghub instances: %w", err)
	}
	boundApps := make(map[uint]bool, len(instances))
	for _, inst := range instances {
		boundApps[inst.AppID] = true
	}

	// A. channel.models -> instances
	for _, m := range models {
		app, ok := appByUpstream[modelToUpstreamID(m)]
		if !ok {
			result.Items = append(result.Items, ChannelSyncItem{Model: m, Action: syncActionOrphanModel})
			continue
		}
		if boundApps[app.ID] {
			result.Items = append(result.Items, ChannelSyncItem{Model: m, Action: syncActionOK, AppID: app.ID})
			continue
		}
		inst := &AppInstance{
			AppID:        app.ID,
			ChannelID:    channelID,
			InstanceType: InstanceDefault,
			Enabled:      true,
			Weight:       1,
		}
		if err := db().Create(inst).Error; err != nil {
			return nil, fmt.Errorf("create runninghub instance: %w", err)
		}
		boundApps[app.ID] = true
		result.InstancesCreated++
		result.Items = append(result.Items, ChannelSyncItem{Model: m, Action: syncActionBoundInstance, AppID: app.ID})
	}

	// B. instances -> channel.models
	modelsChanged := false
	for _, inst := range instances {
		app, ok := appByID[inst.AppID]
		if !ok || !inst.Enabled || inModels[app.UpstreamID] {
			continue
		}
		inModels[app.UpstreamID] = true
		models = append(models, app.UpstreamID)
		modelsChanged = true
		result.ModelsSynced++
		result.Items = append(result.Items, ChannelSyncItem{Model: app.UpstreamID, Action: syncActionSyncedChannel, AppID: app.ID})
	}
	if modelsChanged {
		ch.Models = strings.Join(models, ",")
		if err := ch.Update(); err != nil {
			return nil, fmt.Errorf("update channel models: %w", err)
		}
		result.ChannelModelsUpdated = true
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// stats (GET /dashboard/zsy/rh/stats).
func stats(c *gin.Context) {
	s, err := CollectStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, s)
}
