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
// One aggregate response for the plugin dashboard: app counters from the
// plugin-owned table plus task status counters scoped to the RunningHub
// platform in the host tasks table.
// ---------------------------------------------------------------------------

// AppsStats summarises the app template registry.
type AppsStats struct {
	Total     int64            `json:"total"`
	Published int64            `json:"published"`
	ByKind    map[string]int64 `json:"byKind"`
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
	Apps  AppsStats  `json:"apps"`
	Tasks TasksStats `json:"tasks"`
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
//  A. channel.models → app: every channel model that resolves to an app
//     (exact UpstreamID match, or the dev-plan `rh-aiapp-<id>` style prefix)
//     is reported; only published apps whose site matches the channel's type
//     are treated as in-pool (Action "ok").
//  B. app → channel.models: every published app whose site matches this
//     channel's type gets its UpstreamID appended to channel.models when
//     missing, then the channel is saved through the host Channel.Update() so
//     the abilities table is rebuilt.
//
// The operation is idempotent: running it twice yields zero changes. Apps no
// longer bind a channel — this sync only maintains the per-site channel pool
// model lists that submitAppRun's site selector reads.
// ---------------------------------------------------------------------------

// ChannelSyncItem reports what happened to one channel model / app pair.
type ChannelSyncItem struct {
	// Model is the channel model name (or the appended UpstreamID).
	Model string `json:"model"`
	// Action ∈ bound_app | synced_to_channel | ok | orphan_model.
	Action string `json:"action"`
	AppID  uint   `json:"appId,omitempty"`
}

// ChannelSyncResult summarises one sync run.
type ChannelSyncResult struct {
	ChannelID            int64             `json:"channelId"`
	AppsBound            int64             `json:"appsBound"`
	ModelsSynced         int64             `json:"modelsSynced"`
	ChannelModelsUpdated bool              `json:"channelModelsUpdated"`
	Items                []ChannelSyncItem `json:"items"`
}

// sync actions
const (
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
	if !pluginChannelTypes[ch.Type] {
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
	for i := range apps {
		app := &apps[i]
		if _, exists := appByUpstream[app.UpstreamID]; !exists {
			appByUpstream[app.UpstreamID] = app
		}
	}

	// A. channel.models -> app: every channel model whose UpstreamID resolves
	// to a published, site-matching app is verified present in that app's site
	// channel pool. Apps no longer bind a channel, so nothing is written here.
	for _, m := range models {
		app, ok := appByUpstream[modelToUpstreamID(m)]
		if !ok {
			result.Items = append(result.Items, ChannelSyncItem{Model: m, Action: syncActionOrphanModel})
			continue
		}
		if app.Published && siteToChannelType(app.Site) == ch.Type {
			result.Items = append(result.Items, ChannelSyncItem{Model: m, Action: syncActionOK, AppID: app.ID})
			continue
		}
		// The app either is unpublished, or does not belong to this channel's
		// site (site=cn app in an Intl channel's model list, or site= intl app
		// in a cn channel's). Leave the channel's models untouched — the admin
		// can prune them manually.
		result.Items = append(result.Items, ChannelSyncItem{Model: m, Action: syncActionOK, AppID: app.ID})
	}

	// B. app -> channel.models
	modelsChanged := false
	for _, app := range apps {
		if !app.Published || siteToChannelType(app.Site) != ch.Type || inModels[app.UpstreamID] {
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
