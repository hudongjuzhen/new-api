package voice

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// ErrVoiceNotFound is returned when a required Voice row is missing, so
// callers can answer with a stable "not found" instead of a database error.
var ErrVoiceNotFound = errors.New("voice: voice not found")

// db exposes the host connection to the store layer only; controllers never
// touch GORM directly.
func db() *gorm.DB { return model.DB }

// applyVoiceFilters turns a list query into the shared WHERE clause. VoiceSearch
// (one page) and VoiceExportRows (the whole selection) must answer the same
// filters, so the clause lives here rather than in either caller.
func applyVoiceFilters(base *gorm.DB, q VoiceListQuery) *gorm.DB {
	if q.Enabled != nil {
		base = base.Where("enabled = ?", *q.Enabled)
	}
	if q.VoiceType != "" {
		base = base.Where("voice_type = ?", q.VoiceType)
	}
	if q.Gender != "" {
		base = base.Where("gender = ?", normalizeVocabulary(q.Gender))
	}
	if q.AgeRange != "" {
		base = base.Where("age_range = ?", normalizeVocabulary(q.AgeRange))
	}
	if q.Language != "" {
		base = base.Where("language = ?", normalizeVocabulary(q.Language))
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		base = base.Where(
			"(name LIKE ? OR voice_type LIKE ? OR description LIKE ? OR gender LIKE ? OR age_range LIKE ? OR language LIKE ? OR scenes LIKE ?)",
			kw, kw, kw, kw, kw, kw, kw,
		)
	}
	return base
}

// VoiceSearch filters, paginates and counts the catalog. Out-of-range
// pagination is normalized here rather than rejected, so both the public list
// and the admin list share one contract:
//
//	page < 1        → 1
//	page_size < 1   → 20
//	page_size > 100 → 100
//
// Ordering is sort_order ascending with id ascending as the tie-break, which
// keeps paging stable when several voices share the same sort value.
func VoiceSearch(q VoiceListQuery) (VoiceListResult, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}

	base := applyVoiceFilters(db().Model(&Voice{}), q)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return VoiceListResult{}, fmt.Errorf("voice list count: %w", err)
	}

	var rows []Voice
	offset := (q.Page - 1) * q.PageSize
	if err := base.Order("sort_order asc, id asc").Limit(q.PageSize).Offset(offset).Find(&rows).Error; err != nil {
		return VoiceListResult{}, fmt.Errorf("voice list query: %w", err)
	}

	items := make([]*VoiceView, 0, len(rows))
	for i := range rows {
		items = append(items, voiceToView(&rows[i]))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	return VoiceListResult{
		Items:      items,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// VoiceExportRows returns every voice matching the filters, in plaza order. An
// export is the whole selection rather than one page of it, so there is no
// pagination here; the CSV handler streams the rows straight out.
func VoiceExportRows(q VoiceListQuery) ([]Voice, error) {
	rows := []Voice{}
	query := applyVoiceFilters(db().Model(&Voice{}), q)
	if err := query.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("voice export query: %w", err)
	}
	return rows, nil
}

// VoiceGetByID loads one voice by primary key.
func VoiceGetByID(id uint) (*VoiceView, error) {
	v := &Voice{}
	if err := db().First(v, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVoiceNotFound
		}
		return nil, err
	}
	return voiceToView(v), nil
}

// VoiceInsert validates and creates one voice. The unique index on name is the
// authority; a collision is translated into a message an operator can act on.
func VoiceInsert(dto *VoiceCreateDTO) (*VoiceView, error) {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	sortOrder := 0
	if dto.SortOrder != nil {
		sortOrder = *dto.SortOrder
	}

	v := &Voice{
		Name:        strings.TrimSpace(dto.Name),
		Description: strings.TrimSpace(dto.Description),
		VoiceType:   strings.TrimSpace(dto.VoiceType),
		Gender:      normalizeVocabulary(dto.Gender),
		AgeRange:    normalizeVocabulary(dto.AgeRange),
		Language:    normalizeVocabulary(dto.Language),
		Scenes:      scenesToCSV(dto.Scenes),
		AvatarURL:   strings.TrimSpace(dto.AvatarURL),
		AudioURL:    strings.TrimSpace(dto.AudioURL),
		AudioName:   strings.TrimSpace(dto.AudioName),
		AudioSize:   dto.AudioSize,
		Enabled:     enabled,
		SortOrder:   sortOrder,
	}
	if err := validateVoice(v); err != nil {
		return nil, err
	}
	if err := db().Create(v).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("音色名称 %q 已存在", v.Name)
		}
		return nil, fmt.Errorf("create voice: %w", err)
	}
	return voiceToView(v), nil
}

// VoiceUpdate applies the provided fields of dto to the voice at id. Omitted
// (nil) fields keep their stored value; explicit empty strings clear them.
func VoiceUpdate(id uint, dto *VoiceUpdateDTO) (*VoiceView, error) {
	v := &Voice{}
	if err := db().First(v, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrVoiceNotFound
		}
		return nil, err
	}

	if dto.Name != nil {
		v.Name = strings.TrimSpace(*dto.Name)
	}
	if dto.Description != nil {
		v.Description = strings.TrimSpace(*dto.Description)
	}
	if dto.VoiceType != nil {
		v.VoiceType = strings.TrimSpace(*dto.VoiceType)
	}
	if dto.Gender != nil {
		v.Gender = normalizeVocabulary(*dto.Gender)
	}
	if dto.AgeRange != nil {
		v.AgeRange = normalizeVocabulary(*dto.AgeRange)
	}
	if dto.Language != nil {
		v.Language = normalizeVocabulary(*dto.Language)
	}
	if dto.Scenes != nil {
		v.Scenes = scenesToCSV(*dto.Scenes)
	}
	if dto.AvatarURL != nil {
		v.AvatarURL = strings.TrimSpace(*dto.AvatarURL)
	}
	if dto.AudioURL != nil {
		v.AudioURL = strings.TrimSpace(*dto.AudioURL)
	}
	if dto.AudioName != nil {
		v.AudioName = strings.TrimSpace(*dto.AudioName)
	}
	if dto.AudioSize != nil {
		v.AudioSize = *dto.AudioSize
	}
	if dto.Enabled != nil {
		v.Enabled = *dto.Enabled
	}
	if dto.SortOrder != nil {
		v.SortOrder = *dto.SortOrder
	}

	if err := validateVoice(v); err != nil {
		return nil, err
	}
	if err := db().Save(v).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("音色名称 %q 已存在", v.Name)
		}
		return nil, fmt.Errorf("update voice: %w", err)
	}
	return voiceToView(v), nil
}

// VoiceDelete removes one voice by id and returns its name (for the admin
// confirmation message). The uploaded sample file is intentionally kept: other
// rows may still reference the same URL, and an orphaned file is harmless.
func VoiceDelete(id uint) (string, error) {
	v := &Voice{}
	if err := db().Select("id", "name").First(v, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrVoiceNotFound
		}
		return "", fmt.Errorf("load voice: %w", err)
	}
	if err := db().Delete(&Voice{}, "id = ?", id).Error; err != nil {
		return v.Name, fmt.Errorf("delete voice: %w", err)
	}
	return v.Name, nil
}

// validateVoice enforces the field contract shared by create and update.
func validateVoice(v *Voice) error {
	switch {
	case v.Name == "":
		return errors.New("音色名称不能为空")
	case len([]rune(v.Name)) > maxNameLen:
		return fmt.Errorf("音色名称过长 (上限 %d 字符)", maxNameLen)
	case v.VoiceType == "":
		return errors.New("voice_type 不能为空")
	case len([]rune(v.VoiceType)) > maxVoiceTypeLen:
		return fmt.Errorf("voice_type 过长 (上限 %d 字符)", maxVoiceTypeLen)
	case len([]rune(v.Description)) > maxDescriptionLen:
		return fmt.Errorf("简介过长 (上限 %d 字符)", maxDescriptionLen)
	case len([]rune(v.AudioName)) > maxAudioNameLen:
		return fmt.Errorf("音频文件名过长 (上限 %d 字符)", maxAudioNameLen)
	case v.AudioSize < 0:
		return errors.New("音频文件大小不能为负数")
	case v.SortOrder < -maxSortOrderAbs || v.SortOrder > maxSortOrderAbs:
		return fmt.Errorf("排序值超出范围 (-%d ~ %d)", maxSortOrderAbs, maxSortOrderAbs)
	}
	if v.Gender != "" {
		if _, ok := allowedGenders[v.Gender]; !ok {
			return fmt.Errorf("非法性别 %q (可选 male / female / neutral，留空表示未标注)", v.Gender)
		}
	}
	if v.AgeRange != "" {
		if _, ok := allowedAgeRanges[v.AgeRange]; !ok {
			return fmt.Errorf("非法年龄段 %q (可选 child / teen / young / middle / senior，留空表示未标注)", v.AgeRange)
		}
	}
	if err := checkLanguage(v.Language); err != nil {
		return err
	}
	if err := checkScenes(v.Scenes); err != nil {
		return err
	}
	if err := checkImageURL(v.AvatarURL, maxAvatarURLLen); err != nil {
		return err
	}
	return checkAudioURL(v.AudioURL)
}

// checkAudioURL and checkImageURL both delegate to checkAssetURL, keeping the
// per-field message ("音频地址…" / "头像地址…") an operator can act on.
func checkAudioURL(raw string) error {
	return checkAssetURL(raw, maxAudioURLLen, "音频")
}

func checkImageURL(raw string, maxLen int) error {
	return checkAssetURL(raw, maxLen, "头像")
}

// checkLanguage validates the optional language tag: a short lower-case code
// ("zh", "en", "pt", "pt-BR"). The list stays open so a provider's own spelling
// (Volcengine's "mx" for Mexican Spanish, "tl" for Tagalog) is accepted.
func checkLanguage(tag string) error {
	if tag == "" {
		return nil
	}
	if len(tag) > maxLanguageLen {
		return fmt.Errorf("语言代码过长 (上限 %d 字符)", maxLanguageLen)
	}
	for i, r := range tag {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		case (r == '-' || r == '_') && i > 0:
		default:
			return fmt.Errorf("非法语言代码 %q (仅允许小写字母开头，可含数字、- 或 _)", tag)
		}
	}
	return nil
}

// normalizeVocabulary lower-cases a controlled-vocabulary value so "Male" from
// a spreadsheet and "male" from the API are stored — and later filtered — as
// the same thing. Values outside the vocabulary are rejected by validateVoice.
func normalizeVocabulary(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
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

// checkAssetURL keeps an asset field (the sample audio, the voice avatar) to
// something the browser can actually load: the gateway's own "/uploads/..."
// path or an absolute http(s) URL. Anything else (javascript:, data:, a bare
// host) is rejected instead of being stored and echoed to every catalog caller.
func checkAssetURL(raw string, maxLen int, label string) error {
	if raw == "" {
		return nil
	}
	if len(raw) > maxLen {
		return fmt.Errorf("%s地址过长 (上限 %d 字符)", label, maxLen)
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

// isUniqueViolation reports whether err is a unique-constraint failure on the
// hinted column. It is a best-effort cross-dialect text match (SQLite, MySQL
// and PostgreSQL each phrase it differently); a false negative only falls back
// to the generic database error.
func isUniqueViolation(err error, columnHint string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return (strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")) &&
		strings.Contains(msg, strings.ToLower(columnHint))
}
