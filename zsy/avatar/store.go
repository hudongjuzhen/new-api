package avatar

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// ErrAvatarNotFound is returned when a required Avatar row is missing, so
// callers can answer with a stable "not found" instead of a database error.
var ErrAvatarNotFound = errors.New("avatar: avatar not found")

// db exposes the host connection to the store layer only; controllers never
// touch GORM directly.
func db() *gorm.DB { return model.DB }

// VoiceSampleResolver answers "what sample audio does this voice_type carry?" for
// a batch of voice ids at once. It is a parameter rather than a direct call so the
// store can be exercised without the voice catalog loaded, and so a failing
// look-up degrades to "no sample URL" instead of failing the persona list. A nil
// resolver selects the real catalog lookup (see voice_link.go).
type VoiceSampleResolver func(voiceIDs []string) (map[string]VoiceSample, error)

// resolveVoiceSamples attaches the voice-catalog fields to a page of personas.
//
// A resolver failure is logged and swallowed: the persona rows themselves were
// read successfully, and a missing sample URL must not turn the whole plaza into
// an error page.
func resolveVoiceSamples(views []*AvatarView, resolve VoiceSampleResolver) {
	if resolve == nil || len(views) == 0 {
		return
	}
	ids := make([]string, 0, len(views))
	for _, view := range views {
		if view.VoiceID != "" {
			ids = append(ids, view.VoiceID)
		}
	}
	if len(ids) == 0 {
		return
	}

	samples, err := resolve(ids)
	if err != nil {
		common.SysError("[zsy-avatar] voice sample lookup failed: " + err.Error())
		return
	}
	for _, view := range views {
		sample, ok := samples[view.VoiceID]
		if !ok {
			continue
		}
		view.VoiceAvailable = true
		view.VoiceName = sample.Name
		view.VoiceSampleURL = sample.AudioURL
		view.VoiceSampleName = sample.AudioName
	}
}

// applyAvatarFilters turns a list query into the shared WHERE clause.
// AvatarSearch (one page) and AvatarExportRows (the whole selection) must answer
// the same filters, so the clause lives here rather than in either caller.
func applyAvatarFilters(base *gorm.DB, q AvatarListQuery) *gorm.DB {
	if q.Enabled != nil {
		base = base.Where("enabled = ?", *q.Enabled)
	}
	if q.Gender != "" {
		base = base.Where("gender = ?", normalizeVocabulary(q.Gender, genderAliases))
	}
	if q.AgeRange != "" {
		base = base.Where("age_range = ?", normalizeVocabulary(q.AgeRange, ageRangeAliases))
	}
	if q.Race != "" {
		base = base.Where("race = ?", normalizeVocabulary(q.Race, raceAliases))
	}
	if q.VoiceID != "" {
		base = base.Where("voice_id = ?", normalizeVoiceID(q.VoiceID))
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		base = base.Where(
			"(name LIKE ? OR description LIKE ? OR gender LIKE ? OR age_range LIKE ? OR race LIKE ? OR scenes LIKE ? OR voice_id LIKE ?)",
			kw, kw, kw, kw, kw, kw, kw,
		)
	}
	return base
}

// AvatarSearch filters, paginates and counts the catalog. Out-of-range pagination
// is normalized here rather than rejected, so both the public list and the admin
// list share one contract:
//
//	page < 1        → 1
//	page_size < 1   → 20
//	page_size > 100 → 100
//
// Ordering is sort_order ascending with id ascending as the tie-break, which keeps
// paging stable when several personas share the same sort value. A nil resolve
// selects the real voice-catalog lookup.
func AvatarSearch(q AvatarListQuery, resolve VoiceSampleResolver) (AvatarListResult, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}

	base := applyAvatarFilters(db().Model(&Avatar{}), q)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return AvatarListResult{}, fmt.Errorf("avatar list count: %w", err)
	}

	var rows []Avatar
	offset := (q.Page - 1) * q.PageSize
	if err := base.Order("sort_order asc, id asc").Limit(q.PageSize).Offset(offset).Find(&rows).Error; err != nil {
		return AvatarListResult{}, fmt.Errorf("avatar list query: %w", err)
	}

	items := make([]*AvatarView, 0, len(rows))
	for i := range rows {
		items = append(items, avatarToView(&rows[i]))
	}
	resolveVoiceSamples(items, voiceSampleResolverOrDefault(resolve))

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	return AvatarListResult{
		Items:      items,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// AvatarExportRows returns every persona matching the filters, in plaza order. An
// export is the whole selection rather than one page of it, so there is no
// pagination here; the CSV handler streams the rows straight out.
func AvatarExportRows(q AvatarListQuery) ([]Avatar, error) {
	rows := []Avatar{}
	query := applyAvatarFilters(db().Model(&Avatar{}), q)
	if err := query.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("avatar export query: %w", err)
	}
	return rows, nil
}

// AvatarGetByID loads one persona by primary key, with its voice-catalog fields
// resolved.
func AvatarGetByID(id uint, resolve VoiceSampleResolver) (*AvatarView, error) {
	a := &Avatar{}
	if err := db().First(a, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAvatarNotFound
		}
		return nil, err
	}
	view := avatarToView(a)
	resolveVoiceSamples([]*AvatarView{view}, voiceSampleResolverOrDefault(resolve))
	return view, nil
}

// AvatarInsert validates and creates one persona. The unique index on name is the
// authority; a collision is translated into a message an operator can act on.
func AvatarInsert(dto *AvatarCreateDTO) (*AvatarView, error) {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	sortOrder := 0
	if dto.SortOrder != nil {
		sortOrder = *dto.SortOrder
	}

	a := &Avatar{
		Name:          strings.TrimSpace(dto.Name),
		Description:   strings.TrimSpace(dto.Description),
		ImageURL:      strings.TrimSpace(dto.ImageURL),
		FullBodyURL:   strings.TrimSpace(dto.FullBodyURL),
		FourViewURL:   strings.TrimSpace(dto.FourViewURL),
		ExpressionURL: strings.TrimSpace(dto.ExpressionURL),
		Gender:        normalizeVocabulary(dto.Gender, genderAliases),
		AgeRange:      normalizeVocabulary(dto.AgeRange, ageRangeAliases),
		Race:          normalizeVocabulary(dto.Race, raceAliases),
		Scenes:        scenesToCSV(dto.Scenes),
		VoiceID:       normalizeVoiceID(dto.VoiceID),
		Enabled:       enabled,
		SortOrder:     sortOrder,
	}
	if err := validateAvatar(a); err != nil {
		return nil, err
	}
	if err := db().Create(a).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("形象名称 %q 已存在", a.Name)
		}
		return nil, fmt.Errorf("create avatar: %w", err)
	}
	return avatarToView(a), nil
}

// AvatarUpdate applies the provided fields of dto to the persona at id. Omitted
// (nil) fields keep their stored value; explicit empty strings clear them.
func AvatarUpdate(id uint, dto *AvatarUpdateDTO) (*AvatarView, error) {
	a := &Avatar{}
	if err := db().First(a, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAvatarNotFound
		}
		return nil, err
	}

	if dto.Name != nil {
		a.Name = strings.TrimSpace(*dto.Name)
	}
	if dto.Description != nil {
		a.Description = strings.TrimSpace(*dto.Description)
	}
	if dto.ImageURL != nil {
		a.ImageURL = strings.TrimSpace(*dto.ImageURL)
	}
	if dto.FullBodyURL != nil {
		a.FullBodyURL = strings.TrimSpace(*dto.FullBodyURL)
	}
	if dto.FourViewURL != nil {
		a.FourViewURL = strings.TrimSpace(*dto.FourViewURL)
	}
	if dto.ExpressionURL != nil {
		a.ExpressionURL = strings.TrimSpace(*dto.ExpressionURL)
	}
	if dto.Gender != nil {
		a.Gender = normalizeVocabulary(*dto.Gender, genderAliases)
	}
	if dto.AgeRange != nil {
		a.AgeRange = normalizeVocabulary(*dto.AgeRange, ageRangeAliases)
	}
	if dto.Race != nil {
		a.Race = normalizeVocabulary(*dto.Race, raceAliases)
	}
	if dto.Scenes != nil {
		a.Scenes = scenesToCSV(*dto.Scenes)
	}
	if dto.VoiceID != nil {
		a.VoiceID = normalizeVoiceID(*dto.VoiceID)
	}
	if dto.Enabled != nil {
		a.Enabled = *dto.Enabled
	}
	if dto.SortOrder != nil {
		a.SortOrder = *dto.SortOrder
	}

	if err := validateAvatar(a); err != nil {
		return nil, err
	}
	if err := db().Save(a).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("形象名称 %q 已存在", a.Name)
		}
		return nil, fmt.Errorf("update avatar: %w", err)
	}
	return avatarToView(a), nil
}

// AvatarDelete removes one persona by id and returns its name (for the admin
// confirmation message). The uploaded picture is intentionally kept: other rows
// may still reference the same URL, and an orphaned file is harmless.
func AvatarDelete(id uint) (string, error) {
	a := &Avatar{}
	if err := db().Select("id", "name").First(a, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrAvatarNotFound
		}
		return "", fmt.Errorf("load avatar: %w", err)
	}
	if err := db().Delete(&Avatar{}, "id = ?", id).Error; err != nil {
		return a.Name, fmt.Errorf("delete avatar: %w", err)
	}
	return a.Name, nil
}

// validateAvatar enforces the field contract shared by create and update.
func validateAvatar(a *Avatar) error {
	switch {
	case a.Name == "":
		return errors.New("形象名称不能为空")
	case len([]rune(a.Name)) > maxNameLen:
		return fmt.Errorf("形象名称过长 (上限 %d 字符)", maxNameLen)
	case len([]rune(a.Description)) > maxDescriptionLen:
		return fmt.Errorf("简介过长 (上限 %d 字符)", maxDescriptionLen)
	case len([]rune(a.VoiceID)) > maxVoiceIDLen:
		return fmt.Errorf("音色 ID 过长 (上限 %d 字符)", maxVoiceIDLen)
	case a.SortOrder < -maxSortOrderAbs || a.SortOrder > maxSortOrderAbs:
		return fmt.Errorf("排序值超出范围 (-%d ~ %d)", maxSortOrderAbs, maxSortOrderAbs)
	}
	if a.Gender != "" {
		if _, ok := allowedGenders[a.Gender]; !ok {
			return fmt.Errorf("非法性别 %q (可选 male / female / neutral，留空表示未标注)", a.Gender)
		}
	}
	if a.AgeRange != "" {
		if _, ok := allowedAgeRanges[a.AgeRange]; !ok {
			return fmt.Errorf("非法年龄段 %q (可选 child / teen / young / middle / senior，留空表示未标注)", a.AgeRange)
		}
	}
	if a.Race != "" {
		if _, ok := allowedRaces[a.Race]; !ok {
			return fmt.Errorf(
				"非法种族 %q (可选 asian / black / white / latino / middle_eastern / south_asian / mixed，留空表示未标注)",
				a.Race)
		}
	}
	if err := checkScenes(a.Scenes); err != nil {
		return err
	}
	// Every picture follows the same rule; the label makes a rejection name the
	// one the operator has to fix.
	for _, picture := range []struct {
		label string
		url   string
	}{
		{labelCoverImage, a.ImageURL},
		{labelFullBodyImage, a.FullBodyURL},
		{labelFourViewImage, a.FourViewURL},
		{labelExpressionImage, a.ExpressionURL},
	} {
		if err := checkImageURL(picture.url, picture.label); err != nil {
			return err
		}
	}
	return nil
}

// checkImageURL keeps one picture to something the browser can actually load: the
// gateway's own "/uploads/..." path or an absolute http(s) URL. Anything else
// (javascript:, data:, a bare host) is rejected instead of being stored and echoed
// to every catalog caller. label is the picture's name in the admin form.
func checkImageURL(raw string, label string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > maxImageURLLen {
		return fmt.Errorf("%s地址过长 (上限 %d 字符)", label, maxImageURLLen)
	}
	if strings.HasPrefix(raw, "/") && !strings.HasPrefix(raw, "//") {
		return nil
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return nil
	}
	return fmt.Errorf("%s地址必须是 /uploads/... 路径或 http(s) 地址", label)
}

// normalizeVoiceID trims a voice identifier. The value is matched against the
// 音色广场 catalog verbatim, so its case is preserved: a provider's voice_type is
// case-sensitive and lower-casing it would silently break the link.
func normalizeVoiceID(raw string) string {
	return strings.TrimSpace(raw)
}

// checkScenes validates the stored comma-separated scene list.
func checkScenes(stored string) error {
	if stored == "" {
		return nil
	}
	if len(stored) > maxScenesCSVLen {
		return fmt.Errorf("适合场景总长度过长 (上限 %d 字符)", maxScenesCSVLen)
	}
	scenes := strings.Split(stored, ",")
	if len(scenes) > maxScenes {
		return fmt.Errorf("适合场景最多 %d 个", maxScenes)
	}
	for _, scene := range scenes {
		if len([]rune(scene)) > maxSceneLen {
			return fmt.Errorf("单个场景过长 (上限 %d 字符): %q", maxSceneLen, scene)
		}
	}
	return nil
}

// isUniqueViolation reports whether err is a unique-constraint failure on the
// hinted column. It is a best-effort cross-dialect text match (SQLite, MySQL and
// PostgreSQL each phrase it differently); a false negative only falls back to the
// generic database error.
func isUniqueViolation(err error, columnHint string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return (strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")) &&
		strings.Contains(msg, strings.ToLower(columnHint))
}
