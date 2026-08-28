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

// InstanceListQuery is the validated query shape for the admin instance list.
type InstanceListQuery struct {
	AppID     *uint  `json:"appId"`
	ChannelID *int64 `json:"channelId"`
	Enabled   *bool  `json:"enabled"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	SortBy    string `json:"sortBy"`    // id | createdAt
	SortOrder string `json:"sortOrder"` // asc | desc
}

// InstanceListResult matches the plugin's other list envelopes (items/total/
// page/pageSize/totalPages) so the admin UI reuses one paginator.
type InstanceListResult struct {
	Items      []*InstanceView `json:"items"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"pageSize"`
	TotalPages int             `json:"totalPages"`
}

// KeyPoolEntryView is one keypool row as seen by the admin UI. Keys are always
// masked — the admin manages keys through the channel page, not here.
type KeyPoolEntryView struct {
	ID        uint   `json:"id"`
	KeyMasked string `json:"key"`
	Enabled   bool   `json:"enabled"`
	Remark    string `json:"remark"`
	// Occupancy is the number of in-flight (pending) submits charged to this
	// key. It mirrors the 对账式 keypool design (dev plan §3.4).
	Occupancy int64 `json:"occupancy"`
}

// InstanceView is the read-side shape of an AppInstance.
type InstanceView struct {
	ID             uint               `json:"id"`
	CreatedAt      int64              `json:"createdAt"`
	UpdatedAt      int64              `json:"updatedAt"`
	AppID          uint               `json:"appId"`
	AppName        string             `json:"appName"`
	ChannelID      int64              `json:"channelId"`
	ChannelName    string             `json:"channelName"`
	InstanceType   string             `json:"instanceType"`
	Enabled        bool               `json:"enabled"`
	Weight         int                `json:"weight"`
	BaseURL        string             `json:"baseUrl"`
	AccessPassword string             `json:"accessPassword"`
	Remark         string             `json:"remark"`
	Pool           []KeyPoolEntryView `json:"pool"`
	PoolTotal      int                `json:"poolTotal"`
	PoolEnabled    int                `json:"poolEnabled"`
}

// InstanceCreateDTO is the write shape accepted by POST /dashboard/zsy/rh/instances.
type InstanceCreateDTO struct {
	AppID          uint   `json:"appId"`
	ChannelID      int64  `json:"channelId"`
	InstanceType   string `json:"instanceType"`
	Enabled        *bool  `json:"enabled"`
	Weight         int    `json:"weight"`
	BaseURL        string `json:"baseUrl"`
	AccessPassword string `json:"accessPassword"`
	Remark         string `json:"remark"`
}

// InstanceUpdateDTO carries only the fields the admin may change; nil means
// "leave unchanged".
type InstanceUpdateDTO struct {
	AppID          *uint   `json:"appId"`
	ChannelID      *int64  `json:"channelId"`
	InstanceType   *string `json:"instanceType"`
	Enabled        *bool   `json:"enabled"`
	Weight         *int    `json:"weight"`
	BaseURL        *string `json:"baseUrl"`
	AccessPassword *string `json:"accessPassword"`
	Remark         *string `json:"remark"`
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
	InstanceID      uint             `json:"instanceId"`
	KeysAdded       int              `json:"keysAdded"`
	KeysDisabled    int              `json:"keysDisabled"`
	KeysRestored    int              `json:"keysRestored"`
	PendingReleased int              `json:"pendingReleased"`
	Keys            []KeypoolKeyStat `json:"keys"`
}

// ---------------------------------------------------------------------------
// Validation helpers
// ---------------------------------------------------------------------------

var ErrInstanceNotFound = errors.New("runninghub: instance not found")

// keypoolStatePending is the only in-flight state the submit chain writes;
// anything else is terminal bookkeeping.
const keypoolStatePending = "pending"

func normalizeInstanceType(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return InstanceDefault, nil
	case InstanceDefault, InstancePlus:
		return s, nil
	default:
		return "", fmt.Errorf("非法 instanceType: %q（应为 default / plus）", s)
	}
}

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

// validateInstanceRefs checks the two cross-table references every instance
// must satisfy: the app exists and the channel exists and is a RunningHub
// channel. It returns the resolved App and Channel rows so callers can embed
// names in views without re-querying.
func validateInstanceRefs(appID uint, channelID int64) (*App, *model.Channel, error) {
	if appID == 0 {
		return nil, nil, fmt.Errorf("appId 必须为正整数")
	}
	if channelID <= 0 {
		return nil, nil, fmt.Errorf("channelId 必须为正整数")
	}
	app := &App{}
	if err := db().First(app, "id = ?", appID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, fmt.Errorf("应用不存在 (id=%d)", appID)
		}
		return nil, nil, fmt.Errorf("load runninghub app: %w", err)
	}
	ch, err := model.GetChannelById(int(channelID), false)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, fmt.Errorf("渠道不存在 (id=%d)", channelID)
		}
		return nil, nil, fmt.Errorf("load channel: %w", err)
	}
	if ch.Type != pluginChannelType {
		return nil, nil, fmt.Errorf("渠道 %d 不是 RunningHub 渠道 (type=%d)", channelID, ch.Type)
	}
	return app, ch, nil
}

// ---------------------------------------------------------------------------
// Store layer
// ---------------------------------------------------------------------------

func instanceChannelNames(channelIDs []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(channelIDs))
	if len(channelIDs) == 0 {
		return out, nil
	}
	type row struct {
		Id   int
		Name string
	}
	var rows []row
	if err := db().Model(&model.Channel{}).Select("id, name").Where("id IN ?", channelIDs).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load channel names: %w", err)
	}
	for _, r := range rows {
		out[int64(r.Id)] = r.Name
	}
	return out, nil
}

func instanceAppNames(appIDs []uint) (map[uint]string, error) {
	out := make(map[uint]string, len(appIDs))
	if len(appIDs) == 0 {
		return out, nil
	}
	type row struct {
		ID   uint
		Name string
	}
	var rows []row
	if err := db().Model(&App{}).Select("id, name").Where("id IN ?", appIDs).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load app names: %w", err)
	}
	for _, r := range rows {
		out[r.ID] = r.Name
	}
	return out, nil
}

// keypoolOccupancy counts in-flight pending audits per pool id. Missing keys
// in the result mean zero occupancy.
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

func loadPoolEntries(instanceID uint) ([]AppInstanceKeyPool, error) {
	var pools []AppInstanceKeyPool
	if err := db().Where("instance_id = ?", instanceID).Order("id asc").Find(&pools).Error; err != nil {
		return nil, fmt.Errorf("load keypool entries: %w", err)
	}
	return pools, nil
}

func instanceToView(inst *AppInstance, appName, channelName string, withPool bool) (*InstanceView, error) {
	view := &InstanceView{
		ID:             inst.ID,
		CreatedAt:      inst.CreatedAt.Unix(),
		UpdatedAt:      inst.UpdatedAt.Unix(),
		AppID:          inst.AppID,
		AppName:        appName,
		ChannelID:      inst.ChannelID,
		ChannelName:    channelName,
		InstanceType:   inst.InstanceType,
		Enabled:        inst.Enabled,
		Weight:         inst.Weight,
		BaseURL:        inst.BaseURL,
		AccessPassword: inst.AccessPassword,
		Remark:         inst.Remark,
		Pool:           []KeyPoolEntryView{},
	}
	if !withPool {
		return view, nil
	}
	pools, err := loadPoolEntries(inst.ID)
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
		view.Pool = append(view.Pool, KeyPoolEntryView{
			ID:        p.ID,
			KeyMasked: maskKey(p.Key),
			Enabled:   p.Enabled,
			Remark:    p.Remark,
			Occupancy: occupancy[p.ID],
		})
		if p.Enabled {
			view.PoolEnabled++
		}
	}
	view.PoolTotal = len(view.Pool)
	return view, nil
}

func (q *InstanceListQuery) normalize() {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	if q.PageSize > 200 {
		q.PageSize = 200
	}
	switch strings.ToLower(q.SortOrder) {
	case "asc", "desc":
		q.SortOrder = strings.ToLower(q.SortOrder)
	default:
		q.SortOrder = "desc"
	}
	switch strings.ToLower(q.SortBy) {
	case "createdat", "created_at":
		q.SortBy = "created_at"
	default:
		q.SortBy = "id"
	}
}

// InstanceSearch lists instances with filters and pagination. List rows skip
// the per-key pool detail (PoolTotal/PoolEnabled still populated) so one page
// stays a bounded number of queries.
func InstanceSearch(q InstanceListQuery) (InstanceListResult, error) {
	q.normalize()
	base := db().Model(&AppInstance{})
	if q.AppID != nil {
		base = base.Where("app_id = ?", *q.AppID)
	}
	if q.ChannelID != nil {
		base = base.Where("channel_id = ?", *q.ChannelID)
	}
	if q.Enabled != nil {
		base = base.Where("enabled = ?", *q.Enabled)
	}
	var total int64
	if err := base.Count(&total).Error; err != nil {
		return InstanceListResult{}, fmt.Errorf("runninghub instances count: %w", err)
	}
	dir := "DESC"
	if q.SortOrder == "asc" {
		dir = "ASC"
	}
	var rows []AppInstance
	offset := (q.Page - 1) * q.PageSize
	if err := base.Order(fmt.Sprintf("%s %s", q.SortBy, dir)).Limit(q.PageSize).Offset(offset).Find(&rows).Error; err != nil {
		return InstanceListResult{}, fmt.Errorf("runninghub instances list: %w", err)
	}

	appIDs := make([]uint, 0, len(rows))
	chIDs := make([]int64, 0, len(rows))
	for i := range rows {
		appIDs = append(appIDs, rows[i].AppID)
		chIDs = append(chIDs, rows[i].ChannelID)
	}
	appNames, err := instanceAppNames(appIDs)
	if err != nil {
		return InstanceListResult{}, err
	}
	chNames, err := instanceChannelNames(chIDs)
	if err != nil {
		return InstanceListResult{}, err
	}

	items := make([]*InstanceView, 0, len(rows))
	for i := range rows {
		view, err := instanceToView(&rows[i], appNames[rows[i].AppID], chNames[rows[i].ChannelID], false)
		if err != nil {
			return InstanceListResult{}, err
		}
		// list rows carry pool counters only (bounded queries per page)
		pools, err := loadPoolEntries(rows[i].ID)
		if err != nil {
			return InstanceListResult{}, err
		}
		view.PoolTotal = len(pools)
		for _, p := range pools {
			if p.Enabled {
				view.PoolEnabled++
			}
		}
		items = append(items, view)
	}
	totalPages := int(total) / q.PageSize
	if int(total)%q.PageSize > 0 || totalPages == 0 {
		totalPages++
	}
	return InstanceListResult{
		Items:      items,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// InstanceGetByID loads one instance with full pool detail.
func InstanceGetByID(id uint) (*InstanceView, error) {
	inst := &AppInstance{}
	if err := db().First(inst, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, fmt.Errorf("load runninghub instance: %w", err)
	}
	appNames, err := instanceAppNames([]uint{inst.AppID})
	if err != nil {
		return nil, err
	}
	chNames, err := instanceChannelNames([]int64{inst.ChannelID})
	if err != nil {
		return nil, err
	}
	return instanceToView(inst, appNames[inst.AppID], chNames[inst.ChannelID], true)
}

// InstanceInsert validates references and creates one instance.
func InstanceInsert(dto *InstanceCreateDTO) (*InstanceView, error) {
	if dto == nil {
		return nil, fmt.Errorf("empty instance payload")
	}
	app, ch, err := validateInstanceRefs(dto.AppID, dto.ChannelID)
	if err != nil {
		return nil, err
	}
	instanceType, err := normalizeInstanceType(dto.InstanceType)
	if err != nil {
		return nil, err
	}
	if dto.Weight < 0 {
		return nil, fmt.Errorf("weight 不能为负数")
	}
	weight := dto.Weight
	if weight == 0 {
		weight = 1
	}
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	inst := &AppInstance{
		AppID:          dto.AppID,
		ChannelID:      dto.ChannelID,
		InstanceType:   instanceType,
		Enabled:        enabled,
		Weight:         weight,
		BaseURL:        strings.TrimSpace(dto.BaseURL),
		AccessPassword: strings.TrimSpace(dto.AccessPassword),
		Remark:         strings.TrimSpace(dto.Remark),
	}
	if err := db().Create(inst).Error; err != nil {
		return nil, fmt.Errorf("create runninghub instance: %w", err)
	}
	return instanceToView(inst, app.Name, ch.Name, false)
}

// InstanceUpdate applies the non-nil fields of dto onto the instance.
func InstanceUpdate(id uint, dto *InstanceUpdateDTO) (*InstanceView, error) {
	inst := &AppInstance{}
	if err := db().First(inst, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, fmt.Errorf("load runninghub instance: %w", err)
	}
	appID, chID := inst.AppID, inst.ChannelID
	if dto.AppID != nil {
		appID = *dto.AppID
	}
	if dto.ChannelID != nil {
		chID = *dto.ChannelID
	}
	app, ch, err := validateInstanceRefs(appID, chID)
	if err != nil {
		return nil, err
	}
	if dto.InstanceType != nil {
		t, err := normalizeInstanceType(*dto.InstanceType)
		if err != nil {
			return nil, err
		}
		inst.InstanceType = t
	}
	if dto.Enabled != nil {
		inst.Enabled = *dto.Enabled
	}
	if dto.Weight != nil {
		if *dto.Weight < 0 {
			return nil, fmt.Errorf("weight 不能为负数")
		}
		inst.Weight = *dto.Weight
	}
	if dto.BaseURL != nil {
		inst.BaseURL = strings.TrimSpace(*dto.BaseURL)
	}
	if dto.AccessPassword != nil {
		inst.AccessPassword = strings.TrimSpace(*dto.AccessPassword)
	}
	if dto.Remark != nil {
		inst.Remark = strings.TrimSpace(*dto.Remark)
	}
	inst.AppID = appID
	inst.ChannelID = chID
	if err := db().Save(inst).Error; err != nil {
		return nil, fmt.Errorf("update runninghub instance: %w", err)
	}
	return instanceToView(inst, app.Name, ch.Name, false)
}

// InstanceDelete removes an instance. Pool entries and pending audits are
// hard-deleted: pool rows are derivable from the channel key list (refresh
// rebuilds them), and their unique key index would otherwise block re-adding
// the same key after a soft delete. The instance row itself is soft-deleted.
func InstanceDelete(id uint) (uint, error) {
	inst := &AppInstance{}
	if err := db().First(inst, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrInstanceNotFound
		}
		return 0, fmt.Errorf("load runninghub instance: %w", err)
	}
	pools, err := loadPoolEntries(id)
	if err != nil {
		return 0, err
	}
	poolIDs := make([]uint, 0, len(pools))
	for _, p := range pools {
		poolIDs = append(poolIDs, p.ID)
	}
	err = db().Transaction(func(tx *gorm.DB) error {
		if len(poolIDs) > 0 {
			if err := tx.Where("instance_id = ?", id).Delete(&AppInstanceKeyPool{}).Error; err != nil {
				return err
			}
			if err := tx.Where("pool_id IN ?", poolIDs).Delete(&KeypoolPending{}).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&AppInstance{}, "id = ?", id).Error
	})
	if err != nil {
		return 0, fmt.Errorf("delete runninghub instance: %w", err)
	}
	return inst.ID, nil
}

// InstanceSyncKeypool refreshes the instance's keypool from the bound
// channel's key list and reconciles in-flight audits:
//
//  1. Upsert pool entries from channel.Key (newline separated): missing keys
//     are added, keys removed from the channel are disabled (history kept),
//     previously soft-deleted rows for re-added keys are restored.
//  2. Release stale pending audits whose task already reached a terminal
//     state (SUCCESS/FAILURE) — the 对账回落 of dev plan §3.4.
//  3. Return the per-key occupancy snapshot.
func InstanceSyncKeypool(instanceID uint) (*KeypoolRefreshResult, error) {
	inst := &AppInstance{}
	if err := db().First(inst, "id = ?", instanceID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, fmt.Errorf("load runninghub instance: %w", err)
	}
	ch, err := model.GetChannelById(int(inst.ChannelID), true)
	if err != nil {
		return nil, fmt.Errorf("load channel %d: %w", inst.ChannelID, err)
	}
	wantKeys := splitChannelKeys(ch.Key)

	result := &KeypoolRefreshResult{InstanceID: instanceID, Keys: []KeypoolKeyStat{}}

	// --- 1. Sync pool rows against the channel key list -------------------
	var existing []AppInstanceKeyPool
	if err := db().Unscoped().Where("instance_id = ?", instanceID).Find(&existing).Error; err != nil {
		return nil, fmt.Errorf("load keypool entries: %w", err)
	}
	live := make(map[string]AppInstanceKeyPool, len(existing))
	deleted := make(map[string]AppInstanceKeyPool, len(existing))
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
			if err := db().Unscoped().Model(&AppInstanceKeyPool{}).
				Where("id = ?", stale.ID).
				Update("deleted_at", nil).Error; err != nil {
				return nil, fmt.Errorf("restore keypool entry: %w", err)
			}
			result.KeysRestored++
			continue
		}
		entry := &AppInstanceKeyPool{InstanceID: instanceID, Key: key, Enabled: true}
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
		if err := db().Model(&AppInstanceKeyPool{}).
			Where("id = ?", p.ID).
			Update("enabled", false).Error; err != nil {
			return nil, fmt.Errorf("disable keypool entry: %w", err)
		}
		result.KeysDisabled++
	}

	// --- 2. Reconcile pending audits --------------------------------------
	pools, err := loadPoolEntries(instanceID)
	if err != nil {
		return nil, err
	}
	poolIDs := make([]uint, 0, len(pools))
	poolByKey := make(map[uint]AppInstanceKeyPool, len(pools))
	for _, p := range pools {
		poolIDs = append(poolIDs, p.ID)
		poolByKey[p.ID] = p
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
