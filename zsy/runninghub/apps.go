package runninghub

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// DTOs
// ---------------------------------------------------------------------------

// AppListQuery is the validated query shape for the list endpoint.
type AppListQuery struct {
	Keyword   string `json:"keyword"`
	Kind      string `json:"kind"`
	Published *bool  `json:"published"`
	AdminOnly *bool  `json:"adminOnly"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
	SortBy    string `json:"sortBy"`    // id | name | createdAt
	SortOrder string `json:"sortOrder"` // asc | desc
}

// AppListResult carries pagination data + rows back to the admin UI. It is
// deliberately shaped to match the host's channel list envelope so the
// frontend does not need a second paginator component.
type AppListResult struct {
	Items      []*AppView       `json:"items"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
	KindCounts map[string]int64 `json:"kindCounts"`
}

// AppView is the read-side shape of an App, flattening ParamSchema into a
// slice for the admin UI.
type AppView struct {
	ID                 uint                   `json:"id"`
	CreatedAt          int64                  `json:"createdAt"`
	UpdatedAt          int64                  `json:"updatedAt"`
	Name               string                 `json:"name"`
	Slug               string                 `json:"slug"`
	Kind               AppKind                `json:"kind"`
	UpstreamID         string                 `json:"upstreamId"`
	Description        string                 `json:"description"`
	CoverURL           string                 `json:"coverUrl"`
	Published          bool                   `json:"published"`
	AdminOnly          bool                   `json:"adminOnly"`
	ParamSchema        []rhparser.SchemaParam `json:"paramSchema"`
	PerCallBilling     bool                   `json:"perCallBilling"`
	FixedQuotaPerCall  int64                  `json:"fixedQuotaPerCall"`
	ModelBaseRateRatio float64                `json:"modelBaseRateRatio"`
	ChannelID          int64                  `json:"channelId"`
}

// AppCreateDTO / AppUpdateDTO are the write shapes accepted by the admin
// endpoints. We decouple the DB model from the API shape to keep the
// ParamSchema (slice) round-tripping explicit and to prevent callers from
// setting ID/CreatedAt directly.
type AppCreateDTO struct {
	Name               string                 `json:"name"`
	Slug               string                 `json:"slug"`
	Kind               AppKind                `json:"kind"`
	UpstreamID         string                 `json:"upstreamId"`
	Description        string                 `json:"description"`
	CoverURL           string                 `json:"coverUrl"`
	Published          bool                   `json:"published"`
	AdminOnly          bool                   `json:"adminOnly"`
	ParamSchema        []rhparser.SchemaParam `json:"paramSchema"`
	PerCallBilling     bool                   `json:"perCallBilling"`
	FixedQuotaPerCall  int64                  `json:"fixedQuotaPerCall"`
	ModelBaseRateRatio float64                `json:"modelBaseRateRatio"`
	ChannelID          int64                  `json:"channelId"`
}

type AppUpdateDTO = AppCreateDTO // field set identical; alias keeps symmetry

// ParseCurlResponse is the admin-facing shape of the curl-parser output.
type ParseCurlResponse struct {
	Kind         string                     `json:"kind"`
	UpstreamID   string                     `json:"upstreamId"`
	BaseURL      string                     `json:"baseUrl,omitempty"`
	NodeInfoList []rhparser.NodeInfo        `json:"nodeInfoList"`
	Schema       []rhparser.SchemaParam     `json:"schema"`
	Errors       []rhparser.ErrSchemaReport `json:"schemaErrors,omitempty"`
}

// ---------------------------------------------------------------------------
// Query / param validation helpers
// ---------------------------------------------------------------------------

func (q *AppListQuery) normalize() {
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
	case "name":
		q.SortBy = "name"
	case "createdat", "created_at":
		q.SortBy = "created_at"
	case "id":
		q.SortBy = "id"
	default:
		q.SortBy = "id"
	}
}

func orderClauseFor(q AppListQuery) string {
	col := q.SortBy
	// column whitelist
	switch col {
	case "name":
	case "created_at":
	case "id":
	default:
		col = "id"
	}
	dir := "DESC"
	if q.SortOrder == "asc" {
		dir = "ASC"
	}
	// None of these are reserved words; no cross-DB escaping required.
	return fmt.Sprintf("%s %s", col, dir)
}

// ---------------------------------------------------------------------------
// Store layer (thin wrappers around model.DB; kept in runninghub package so
// the runninghub controller/service do not leak GORM specifics up the stack).
// ---------------------------------------------------------------------------

// ErrAppNotFound is returned when a required App record is missing.
var ErrAppNotFound = errors.New("runninghub: app not found")

func db() *gorm.DB { return model.DB }

// AppSearch runs keyword/kind/published filters, pagination, and an overall
// count for the admin UI list.
func AppSearch(q AppListQuery) (AppListResult, error) {
	q.normalize()
	base := db().Model(&App{})
	if q.Kind != "" {
		base = base.Where("kind = ?", q.Kind)
	}
	if q.Published != nil {
		base = base.Where("published = ?", *q.Published)
	}
	if q.AdminOnly != nil {
		base = base.Where("admin_only = ?", *q.AdminOnly)
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		base = base.Where(
			"(name LIKE ? OR slug LIKE ? OR upstream_id LIKE ? OR description LIKE ?)",
			kw, kw, kw, kw,
		)
	}

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return AppListResult{}, fmt.Errorf("runninghub apps count: %w", err)
	}

	kindCounts, err := appKindCounts(q)
	if err != nil {
		return AppListResult{}, err
	}

	var rows []App
	offset := (q.Page - 1) * q.PageSize
	err = base.Order(orderClauseFor(q)).Limit(q.PageSize).Offset(offset).Find(&rows).Error
	if err != nil {
		return AppListResult{}, fmt.Errorf("runninghub apps list: %w", err)
	}
	views := make([]*AppView, 0, len(rows))
	for i := range rows {
		av, err := appToView(&rows[i])
		if err != nil {
			return AppListResult{}, fmt.Errorf("runninghub app[%d] view: %w", i, err)
		}
		views = append(views, av)
	}

	totalPages := int(total) / q.PageSize
	if int(total)%q.PageSize > 0 || totalPages == 0 {
		totalPages++
	}
	return AppListResult{
		Items:      views,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
		KindCounts: kindCounts,
	}, nil
}

func appKindCounts(q AppListQuery) (map[string]int64, error) {
	base := db().Model(&App{})
	// Keyword filter is intentionally excluded; counts represent the side-nav
	// totals after structural filters (kind/published/adminOnly) but before
	// the free text search to match the host channel controller's semantics.
	if q.Published != nil {
		base = base.Where("published = ?", *q.Published)
	}
	if q.AdminOnly != nil {
		base = base.Where("admin_only = ?", *q.AdminOnly)
	}
	type row struct {
		Kind AppKind
		C    int64
	}
	var list []row
	if err := base.Select("kind, count(*) as c").Group("kind").Scan(&list).Error; err != nil {
		return nil, fmt.Errorf("runninghub apps kind counts: %w", err)
	}
	out := make(map[string]int64, len(list))
	for _, r := range list {
		out[string(r.Kind)] = r.C
	}
	return out, nil
}

// AppGetByID loads a single app by primary id. Returns ErrAppNotFound if
// missing, so callers can translate to a standard 404-ish API error.
func AppGetByID(id uint) (*AppView, error) {
	app := &App{}
	if err := db().First(app, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	return appToView(app)
}

// AppInsert validates and creates one app. Uniqueness of name is enforced at
// the DB layer via the unique index; collisions are translated to a stable
// error message.
func AppInsert(dto *AppCreateDTO) (*AppView, error) {
	app, err := applyDto(dto, nil)
	if err != nil {
		return nil, err
	}
	if err := validateApp(app); err != nil {
		return nil, err
	}
	if err := validatePinChannel(app.ChannelID); err != nil {
		return nil, err
	}
	if err := db().Create(app).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("应用名称 %q 已存在", app.Name)
		}
		return nil, fmt.Errorf("create runninghub app: %w", err)
	}
	syncAppBillingPrice(nil, app)
	return appToView(app)
}

// AppUpdate applies dto to the existing app at id. Returns ErrAppNotFound
// when the app does not exist.
func AppUpdate(id uint, dto *AppUpdateDTO) (*AppView, error) {
	app := &App{}
	if err := db().First(app, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAppNotFound
		}
		return nil, err
	}
	old := *app
	updated, err := applyDto(dto, app)
	if err != nil {
		return nil, err
	}
	if err := validateApp(updated); err != nil {
		return nil, err
	}
	if err := validatePinChannel(updated.ChannelID); err != nil {
		return nil, err
	}
	if err := db().Save(updated).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("应用名称 %q 已存在", updated.Name)
		}
		return nil, fmt.Errorf("update runninghub app: %w", err)
	}
	syncAppBillingPrice(&old, updated)
	return appToView(updated)
}

// AppDelete removes an app by id (soft delete because GORM DeletedAt is on
// the model). Missing ids are no-ops and return success, matching the host's
// DeleteChannel behavior so repeated delete clicks don't churn.
func AppDelete(id uint) (string, error) {
	app := &App{}
	if err := db().Select("name").First(app, "id = ?", id).Error; err == nil {
		name := app.Name
		if err := db().Delete(&App{}, "id = ?", id).Error; err != nil {
			return name, fmt.Errorf("delete runninghub app: %w", err)
		}
		return name, nil
	}
	if err := db().Delete(&App{}, "id = ?", id).Error; err != nil {
		return "", fmt.Errorf("delete runninghub app: %w", err)
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Billing price sync
// ---------------------------------------------------------------------------

// appPerCallModelPrice converts the app's per-call quota into the host model
// price unit ("$/call"): the host computes quota = price × QuotaPerUnit ×
// groupRatio during ModelPriceHelperPerCall.
func appPerCallModelPrice(fixedQuota int64) float64 {
	return float64(fixedQuota) / common.QuotaPerUnit
}

// syncAppBillingPrice keeps ratio_setting's per-call model price table aligned
// with the app's billing config. The submit path bills the app under its
// UpstreamID as the model name, and RelayTaskSubmit rebuilds PriceData from
// that table — so a per-call app without a synced price entry would fail with
// "model price not configured", and a stale entry on a switched app would
// silently re-enable per-call settlement.
//
//   - PerCallBilling=true  → upsert price = FixedQuotaPerCall / QuotaPerUnit
//   - switch to dynamic    → drop the entry only while it still equals the
//     previously synced value (an admin-edited price is left untouched)
//
// A deleted app leaves its price entry behind on purpose: the entry is
// harmless once no channel routes the model name, and the same UpstreamID
// reused by a later app overwrites it on save.
func syncAppBillingPrice(old *App, updated *App) {
	if updated == nil || strings.TrimSpace(updated.UpstreamID) == "" {
		return
	}
	prices := ratio_setting.GetModelPriceCopy()
	changed := false
	if updated.PerCallBilling {
		want := appPerCallModelPrice(updated.FixedQuotaPerCall)
		if cur, ok := prices[updated.UpstreamID]; !ok || cur != want {
			prices[updated.UpstreamID] = want
			changed = true
		}
	} else if old != nil && old.PerCallBilling && old.UpstreamID == updated.UpstreamID {
		if cur, ok := prices[old.UpstreamID]; ok && cur == appPerCallModelPrice(old.FixedQuotaPerCall) {
			delete(prices, old.UpstreamID)
			changed = true
		}
	}
	if !changed {
		return
	}
	data, err := common.Marshal(prices)
	if err != nil {
		common.SysError("runninghub: marshal model price map failed: " + err.Error())
		return
	}
	if err := ratio_setting.UpdateModelPriceByJSONString(string(data)); err != nil {
		common.SysError("runninghub: sync app model price failed: " + err.Error())
	}
}

// ---------------------------------------------------------------------------
// Model <-> View translation
// ---------------------------------------------------------------------------

func applyDto(dto *AppCreateDTO, onto *App) (*App, error) {
	if dto == nil {
		return nil, fmt.Errorf("empty app payload")
	}
	var target App
	if onto != nil {
		target = *onto
	}
	target.Name = strings.TrimSpace(dto.Name)
	target.Slug = strings.TrimSpace(dto.Slug)
	target.Kind = AppKind(strings.ToLower(strings.TrimSpace(string(dto.Kind))))
	target.UpstreamID = strings.TrimSpace(dto.UpstreamID)
	target.Description = dto.Description
	target.CoverURL = strings.TrimSpace(dto.CoverURL)
	target.Published = dto.Published
	target.AdminOnly = dto.AdminOnly
	target.PerCallBilling = dto.PerCallBilling
	target.FixedQuotaPerCall = dto.FixedQuotaPerCall
	target.ChannelID = dto.ChannelID
	if dto.ModelBaseRateRatio == 0 {
		// Treat explicit zero same as unset: back to 1.0 default so billing
		// multipliers are never a flat-rate-zero.
		target.ModelBaseRateRatio = 1.0
	} else {
		target.ModelBaseRateRatio = dto.ModelBaseRateRatio
	}
	if err := target.SetParamSchema(schemaParamsToField(dto.ParamSchema)); err != nil {
		return nil, fmt.Errorf("encode param_schema: %w", err)
	}
	return &target, nil
}

// schemaParamsToField converts parser-level schema params to the plugin's
// DB-typed FieldParam list. Keeping a distinct []FieldParam type (rather than
// leaking rhparser into models) prevents schema migrations from dragging
// parser internals around.
func schemaParamsToField(in []rhparser.SchemaParam) []FieldParam {
	if in == nil {
		return []FieldParam{}
	}
	out := make([]FieldParam, 0, len(in))
	for _, p := range in {
		fp := FieldParam{
			NodeID:      p.NodeID,
			FieldName:   p.FieldName,
			Label:       p.Label,
			Type:        ParameterFieldType(p.Type),
			Required:    p.Required,
			Default:     p.Default,
			Placeholder: p.Placeholder,
			Min:         p.Min,
			Max:         p.Max,
		}
		fp.Options = make([]ParameterOption, 0, len(p.Options))
		for _, o := range p.Options {
			fp.Options = append(fp.Options, ParameterOption{Label: o.Label, Value: o.Value})
		}
		out = append(out, fp)
	}
	return out
}

func appToView(a *App) (*AppView, error) {
	schema, err := a.ParamSchema()
	if err != nil {
		return nil, err
	}
	parserSchema := make([]rhparser.SchemaParam, 0, len(schema))
	for _, f := range schema {
		ps := rhparser.SchemaParam{
			NodeID:      f.NodeID,
			FieldName:   f.FieldName,
			Label:       f.Label,
			Type:        string(f.Type),
			Required:    f.Required,
			Default:     f.Default,
			Placeholder: f.Placeholder,
			Min:         f.Min,
			Max:         f.Max,
		}
		ps.Options = make([]rhparser.SchemaParamOption, 0, len(f.Options))
		for _, o := range f.Options {
			ps.Options = append(ps.Options, rhparser.SchemaParamOption{Label: o.Label, Value: o.Value})
		}
		parserSchema = append(parserSchema, ps)
	}
	return &AppView{
		ID:                 a.ID,
		CreatedAt:          a.CreatedAt.Unix(),
		UpdatedAt:          a.UpdatedAt.Unix(),
		Name:               a.Name,
		Slug:               a.Slug,
		Kind:               a.Kind,
		UpstreamID:         a.UpstreamID,
		Description:        a.Description,
		CoverURL:           a.CoverURL,
		Published:          a.Published,
		AdminOnly:          a.AdminOnly,
		ParamSchema:        parserSchema,
		PerCallBilling:     a.PerCallBilling,
		FixedQuotaPerCall:  a.FixedQuotaPerCall,
		ModelBaseRateRatio: a.ModelBaseRateRatio,
		ChannelID:          a.ChannelID,
	}, nil
}

// validatePinChannel ensures an optionally-pinned channel exists and is a
// RunningHub channel. Zero means "no pin" (submit falls back to model-based
// channel selection), so it is always accepted.
func validatePinChannel(channelID int64) error {
	if channelID <= 0 {
		return nil
	}
	ch, err := model.GetChannelById(int(channelID), false)
	if err != nil {
		return fmt.Errorf("绑定渠道无效 (id=%d): %w", channelID, err)
	}
	if !pluginChannelTypes[ch.Type] {
		return fmt.Errorf("渠道 %d (%s) 不是 RunningHub 渠道 (type=%d)", channelID, ch.Name, ch.Type)
	}
	return nil
}

// runAppDataMigration performs one-time, idempotent migrations when the
// plugin starts, best-effort so any failure is logged and skipped:
//
//  1. Channel backfill — copy a legacy `app_instances.channel_id` into the
//     app's `Apps.ChannelID` once, so apps created before the direct-channel
//     binding still work without a manual edit.
//  2. Keypool lift — migrate legacy per-instance keypool rows
//     (`app_instance_key_pools`) up to the per-app `app_key_pools` table,
//     keyed by the owning app, so existing keypool entries are not lost when
//     the instance concept is removed.
//
// The legacy tables are queried by explicit table name (GORM never manages
// the removed AppInstance model), so on a fresh install where those tables do
// not exist the queries simply error out and are skipped.
func runAppDataMigration() {
	runChannelBackfill()
	runKeypoolLift()
}

func runChannelBackfill() {
	var apps []App
	if err := db().Where("channel_id = 0").Limit(500).Find(&apps).Error; err != nil {
		common.SysError("runninghub channel backfill read apps: " + err.Error())
		return
	}
	if len(apps) == 0 {
		return
	}
	ids := make([]uint, 0, len(apps))
	for _, a := range apps {
		ids = append(ids, a.ID)
	}
	type legacyInst struct {
		AppID     uint
		ChannelID int64
	}
	var insts []legacyInst
	// app_instances is a legacy table; never auto-migrated after this change.
	if err := db().Table("app_instances").
		Select("app_id, channel_id").
		Where("app_id IN ?", ids).
		Scan(&insts).Error; err != nil {
		common.SysError("runninghub channel backfill read legacy instances: " + err.Error())
		return
	}
	chanOf := make(map[uint]int64, len(insts))
	for _, in := range insts {
		if in.ChannelID > 0 {
			chanOf[in.AppID] = in.ChannelID
		}
	}
	for _, a := range apps {
		cid, ok := chanOf[a.ID]
		if !ok || cid <= 0 {
			continue
		}
		if err := db().Model(&App{}).Where("id = ?", a.ID).Update("channel_id", cid).Error; err != nil {
			common.SysError(fmt.Sprintf("runninghub channel backfill app %d: %s", a.ID, err.Error()))
		}
	}
}

// runKeypoolLift copies legacy per-instance keypool rows into the per-app
// AppKeyPool table. Idempotent: an (app_id, key) pair already present is
// skipped, so re-running never duplicates rows.
func runKeypoolLift() {
	type legacyInst struct {
		ID    uint
		AppID uint
	}
	var insts []legacyInst
	if err := db().Table("app_instances").Select("id, app_id").Scan(&insts).Error; err != nil {
		common.SysError("runninghub keypool lift read legacy instances: " + err.Error())
		return
	}
	appOf := make(map[uint]uint, len(insts))
	for _, in := range insts {
		appOf[in.ID] = in.AppID
	}
	if len(appOf) == 0 {
		return
	}
	type legacyPool struct {
		InstanceID uint
		Key        string
		Enabled    bool
		Remark     string
	}
	var pools []legacyPool
	// Unscoped-style read: legacy audit rows may be soft-deleted but still
	// represent keys that must be lifted into the live per-app pool.
	if err := db().Table("app_instance_key_pools").
		Select("instance_id, key, enabled, remark").
		Scan(&pools).Error; err != nil {
		common.SysError("runninghub keypool lift read legacy pools: " + err.Error())
		return
	}
	lifted := 0
	for _, p := range pools {
		appID, ok := appOf[p.InstanceID]
		if !ok {
			continue
		}
		if err := db().Where("app_id = ? AND key = ?", appID, p.Key).
			First(&AppKeyPool{}).Error; err == nil {
			continue // already lifted
		}
		entry := &AppKeyPool{AppID: appID, Key: p.Key, Enabled: p.Enabled, Remark: p.Remark}
		if err := db().Create(entry).Error; err != nil {
			common.SysError(fmt.Sprintf("runninghub keypool lift app %d key: %s", appID, err.Error()))
			continue
		}
		lifted++
	}
	if lifted > 0 {
		common.SysError(fmt.Sprintf("runninghub keypool lift migrated %d legacy keys to per-app pool", lifted))
	}
}

// validateApp enforces the non-null / invariants. Anything that could cause
// the billing path to produce a non-positive multiplier is rejected here so
// the chain stays safe later on.
func validateApp(a *App) error {
	switch {
	case a.Name == "":
		return fmt.Errorf("应用名称不能为空")
	case len(a.Name) > 191:
		return fmt.Errorf("应用名称过长 (上限 191 字符)")
	}
	switch a.Kind {
	case AppKindAICApp, AppKindWorkflow, AppKindModel:
	default:
		return fmt.Errorf("非法应用 kind: %q（应为 ai_app / workflow / model）", a.Kind)
	}
	if strings.TrimSpace(a.UpstreamID) == "" {
		return fmt.Errorf("upstreamId 不能为空")
	}
	if len(a.UpstreamID) > 191 {
		return fmt.Errorf("upstreamId 过长 (上限 191 字符)")
	}
	if a.Slug != "" && len(a.Slug) > 191 {
		return fmt.Errorf("slug 过长 (上限 191 字符)")
	}
	// Billing invariants.
	switch {
	case a.FixedQuotaPerCall < 0:
		return fmt.Errorf("fixedQuotaPerCall 不能为负数")
	case a.ModelBaseRateRatio <= 0:
		return fmt.Errorf("modelBaseRateRatio 必须为正数 (当前 %v)", a.ModelBaseRateRatio)
	}
	return nil
}

// isUniqueViolation reports whether err is a GORM unique constraint failure
// against the given column. This is a best-effort cross-dialect text match;
// returning false simply falls back to the generic database error.
func isUniqueViolation(err error, columnHint string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return (strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")) &&
		strings.Contains(msg, strings.ToLower(columnHint))
}
