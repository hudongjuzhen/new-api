package runninghub

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// AppKeypoolEntryView is one keypool row as seen by the admin UI. Keys are
// always masked — the admin manages keys through the channel page, not here.
type AppKeypoolEntryView struct {
	ID     uint   `json:"id"`
	Key    string `json:"key"`
	Enabled bool   `json:"enabled"`
	Remark string `json:"remark"`
	// Occupancy is the number of in-flight (pending) submits charged to this
	// key. It mirrors the 对账式 keypool design (dev plan §3.4).
	Occupancy int64 `json:"occupancy"`
}

// AppKeypoolResult is the read-side shape of an app's keypool: the pool
// entries plus the bound channel info so the admin UI can explain where the
// keys come from.
type AppKeypoolResult struct {
	AppID           uint                   `json:"appId"`
	BoundChannelID  int64                  `json:"channelId"`
	ChannelName     string                 `json:"channelName"`
	Total           int                    `json:"total"`
	Enabled         int                    `json:"enabled"`
	Keys            []AppKeypoolEntryView  `json:"keys"`
}

// KeypoolKeyStat is the per-key occupancy snapshot returned by keypool-refresh.
type KeypoolKeyStat struct {
	PoolID    uint   `json:"poolId"`
	KeyMasked string `json:"key"`
	Enabled   bool   `json:"enabled"`
	Occupancy int64  `json:"occupancy"`
}

// KeypoolRefreshResult summarises one refresh run: how many keys were added /
// disabled to match the channel key list, how many stale pending audits were
// released because their task reached a terminal state, and the resulting
// per-key occupancy snapshot.
type KeypoolRefreshResult struct {
	AppID           uint             `json:"appId"`
	KeysAdded       int              `json:"keysAdded"`
	KeysDisabled    int              `json:"keysDisabled"`
	KeysRestored    int              `json:"keysRestored"`
	PendingReleased int              `json:"pendingReleased"`
	Keys            []KeypoolKeyStat `json:"keys"`
}

// ---------------------------------------------------------------------------
// Validation / pure helpers
// ---------------------------------------------------------------------------

// keypoolStatePending is the only in-flight state the submit chain writes;
// anything else is terminal bookkeeping.
const keypoolStatePending = "pending"

// splitChannelKeys parses the channel's newline-separated key list. This is
// the same convention the host uses for multi-key channels (model/channel.go).
func splitChannelKeys(channelKey string) []string {
	raw := strings.Split(strings.Trim(channelKey, "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, k := range raw {
		if k = strings.TrimSpace(k); k != "" {
			out = append(out, k)
		}
	}
	return out
}

// maskKey hides the middle of an API key for admin display: first 4 + last 4
// characters remain visible, everything in between collapses to "****".
func maskKey(key string) string {
	switch {
	case key == "":
		return ""
	case len(key) <= 8:
		return "****"
	default:
		return key[:4] + "****" + key[len(key)-4:]
	}
}

// isTerminalTaskStatus reports whether a host task status means the submit
// window is closed (no longer occupying keypool concurrency).
func isTerminalTaskStatus(s model.TaskStatus) bool {
	return s == model.TaskStatusSuccess || s == model.TaskStatusFailure
}

// ---------------------------------------------------------------------------
// Store layer
// ---------------------------------------------------------------------------

func keypoolOccupancy(poolIDs []uint) (map[uint]int64, error) {
	out := make(map[uint]int64, len(poolIDs))
	if len(poolIDs) == 0 {
		return out, nil
	}
	type row struct {
		PoolID uint
		C      int64
	}
	var rows []row
	if err := db().Model(&KeypoolPending{}).
		Select("pool_id, count(*) as c").
		Where("pool_id IN ? AND state = ?", poolIDs, keypoolStatePending).
		Group("pool_id").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("count keypool occupancy: %w", err)
	}
	for _, r := range rows {
		out[r.PoolID] = r.C
	}
	return out, nil
}

func loadPoolEntries(appID uint) ([]AppKeyPool, error) {
	var pools []AppKeyPool
	if err := db().Where("app_id = ?", appID).Order("id asc").Find(&pools).Error; err != nil {
		return nil, fmt.Errorf("load keypool entries: %w", err)
	}
	return pools, nil
}

// AppKeypoolStatus loads an app's current keypool state plus the bound channel
// info. Returns ErrAppNotFound when the app is missing.
func AppKeypoolStatus(appID uint) (*AppKeypoolResult, error) {
	app := &App{}
	if err := db().First(app, "id = ?", appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("load runninghub app: %w", err)
	}
	result := &AppKeypoolResult{
		AppID:          app.ID,
		BoundChannelID: app.ChannelID,
		Keys:           []AppKeypoolEntryView{},
	}
	if app.ChannelID > 0 {
		if ch, err := model.GetChannelById(int(app.ChannelID), false); err == nil {
			result.ChannelName = ch.Name
		}
	}
	pools, err := loadPoolEntries(appID)
	if err != nil {
		return nil, err
	}
	poolIDs := make([]uint, 0, len(pools))
	for _, p := range pools {
		poolIDs = append(poolIDs, p.ID)
	}
	occupancy, err := keypoolOccupancy(poolIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		result.Keys = append(result.Keys, AppKeypoolEntryView{
			ID:        p.ID,
			Key:       maskKey(p.Key),
			Enabled:   p.Enabled,
			Remark:    p.Remark,
			Occupancy: occupancy[p.ID],
		})
		if p.Enabled {
			result.Enabled++
		}
	}
	result.Total = len(result.Keys)
	return result, nil
}

// AppSyncKeypool refreshes an app's keypool from its bound channel's key list
// and reconciles in-flight audits:
//
//  1. Requires the app to be pinned to a channel (App.ChannelID > 0).
//  2. Upsert pool entries from channel.Key (newline separated): missing keys
//     are added, keys removed from the channel are disabled (history kept),
//     previously soft-deleted rows for re-added keys are restored.
//  3. Release stale pending audits whose task already reached a terminal
//     state (SUCCESS/FAILURE) — the 对账回落 of dev plan §3.4.
//  4. Return the per-key occupancy snapshot.
func AppSyncKeypool(appID uint) (*KeypoolRefreshResult, error) {
	app := &App{}
	if err := db().First(app, "id = ?", appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, fmt.Errorf("load runninghub app: %w", err)
	}
	if app.ChannelID <= 0 {
		return nil, errors.New("当前应用尚未绑定 RunningHub 渠道，无法同步 keypool")
	}
	// The keypool needs the channel's raw key list to reconcile pool entries,
	// so the full row (including `key`) must be loaded — never Omit it here.
	ch, err := model.GetChannelById(int(app.ChannelID), true)
	if err != nil {
		return nil, fmt.Errorf("load channel %d: %w", app.ChannelID, err)
	}
	wantKeys := splitChannelKeys(ch.Key)

	result := &KeypoolRefreshResult{AppID: appID, Keys: []KeypoolKeyStat{}}

	// --- 1. Sync pool rows against the channel key list -------------------
	var existing []AppKeyPool
	if err := db().Unscoped().Where("app_id = ?", appID).Find(&existing).Error; err != nil {
		return nil, fmt.Errorf("load keypool entries: %w", err)
	}
	live := make(map[string]AppKeyPool, len(existing))
	deleted := make(map[string]AppKeyPool, len(existing))
	for _, p := range existing {
		if p.DeletedAt.Valid {
			deleted[p.Key] = p
		} else {
			live[p.Key] = p
		}
	}
	want := make(map[string]bool, len(wantKeys))
	for _, key := range wantKeys {
		want[key] = true
		if _, ok := live[key]; ok {
			continue
		}
		if stale, ok := deleted[key]; ok {
			// restore the soft-deleted row so the unique index stays satisfied
			if err := db().Unscoped().Model(&AppKeyPool{}).
				Where("id = ?", stale.ID).
				Update("deleted_at", nil).Error; err != nil {
				return nil, fmt.Errorf("restore keypool entry: %w", err)
			}
			result.KeysRestored++
			continue
		}
		entry := &AppKeyPool{AppID: appID, Key: key, Enabled: true}
		if err := db().Create(entry).Error; err != nil {
			return nil, fmt.Errorf("create keypool entry: %w", err)
		}
		result.KeysAdded++
	}
	// keys that disappeared from the channel get disabled, not deleted
	for key, p := range live {
		if want[key] || !p.Enabled {
			continue
		}
		if err := db().Model(&AppKeyPool{}).
			Where("id = ?", p.ID).
			Update("enabled", false).Error; err != nil {
			return nil, fmt.Errorf("disable keypool entry: %w", err)
		}
		result.KeysDisabled++
	}

	// --- 2. Reconcile pending audits --------------------------------------
	pools, err := loadPoolEntries(appID)
	if err != nil {
		return nil, err
	}
	poolIDs := make([]uint, 0, len(pools))
	for _, p := range pools {
		poolIDs = append(poolIDs, p.ID)
	}
	var pending []KeypoolPending
	if len(poolIDs) > 0 {
		if err := db().Where("pool_id IN ? AND state = ?", poolIDs, keypoolStatePending).Find(&pending).Error; err != nil {
			return nil, fmt.Errorf("load pending audits: %w", err)
		}
	}
	if len(pending) > 0 {
		taskIDs := make([]string, 0, len(pending))
		for _, p := range pending {
			taskIDs = append(taskIDs, p.NewApiTaskID)
		}
		type taskRow struct {
			TaskID string
			Status model.TaskStatus
		}
		var tasks []taskRow
		if err := db().Model(&model.Task{}).
			Select("task_id, status").
			Where("task_id IN ?", taskIDs).
			Scan(&tasks).Error; err != nil {
			return nil, fmt.Errorf("load tasks for reconcile: %w", err)
		}
		statusByID := make(map[string]model.TaskStatus, len(tasks))
		for _, t := range tasks {
			statusByID[t.TaskID] = t.Status
		}
		for _, p := range pending {
			if !isTerminalTaskStatus(statusByID[p.NewApiTaskID]) {
				continue
			}
			if err := db().Delete(&KeypoolPending{}, p.ID).Error; err != nil {
				return nil, fmt.Errorf("release pending audit: %w", err)
			}
			result.PendingReleased++
		}
	}

	// --- 3. Occupancy snapshot --------------------------------------------
	occupancy, err := keypoolOccupancy(poolIDs)
	if err != nil {
		return nil, err
	}
	for _, p := range pools {
		result.Keys = append(result.Keys, KeypoolKeyStat{
			PoolID:    p.ID,
			KeyMasked: maskKey(p.Key),
			Enabled:   p.Enabled,
			Occupancy: occupancy[p.ID],
		})
	}
	return result, nil
}