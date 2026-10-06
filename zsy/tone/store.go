package tone

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// The store layer is deliberately shaped like zsy/voice/store.go. The two are
// near-copies and that is the intended cost of this gateway's plugin contract:
// a plugin is self-contained, so installing or removing one touches exactly one
// core file (the blank import in zsy/extbootstrap) and never drags another
// plugin's package in with it. A shared "catalog" package would have to be
// imported by both, which is precisely what the contract forbids.

// ErrToneNotFound is returned when a required Tone row is missing, so callers
// can answer with a stable "not found" instead of a database error.
var ErrToneNotFound = errors.New("tone: tone not found")

// db exposes the host connection to the store layer only; controllers never
// touch GORM directly.
func db() *gorm.DB { return model.DB }

// applyToneFilters turns a list query into the shared WHERE clause. ToneSearch
// (one page) and ToneExportRows (the whole selection) must answer the same
// filters, so the clause lives here rather than in either caller.
//
// The keyword search covers Prompt and SampleOutput as well as the naming
// fields. That is the difference from zsy/voice: an operator hunting for "那段
// 写得很克制的" remembers a phrase from the instruction or the worked example,
// not the display name — searching names alone would make the widest column in
// the table the one thing you cannot search by.
func applyToneFilters(base *gorm.DB, q ToneListQuery) *gorm.DB {
	if q.Enabled != nil {
		base = base.Where("enabled = ?", *q.Enabled)
	}
	if q.Category != "" {
		base = base.Where("category = ?", normalizeVocabulary(q.Category))
	}
	if q.Tone != "" {
		base = base.Where("tone = ?", normalizeVocabulary(q.Tone))
	}
	if q.Language != "" {
		base = base.Where("language = ?", normalizeVocabulary(q.Language))
	}
	if q.Keyword != "" {
		kw := "%" + q.Keyword + "%"
		base = base.Where(
			"(name LIKE ? OR description LIKE ? OR prompt LIKE ? OR category LIKE ? OR tone LIKE ? OR language LIKE ? OR scenes LIKE ? OR sample_input LIKE ? OR sample_output LIKE ?)",
			kw, kw, kw, kw, kw, kw, kw, kw, kw,
		)
	}
	return base
}

// ToneSearch filters, paginates and counts the catalog. Out-of-range pagination
// is normalized here rather than rejected, so both the public list and the admin
// list share one contract:
//
//	page < 1        → 1
//	page_size < 1   → 20
//	page_size > 100 → 100
//
// Ordering is sort_order ascending with id ascending as the tie-break, which
// keeps paging stable when several tones share the same sort value.
func ToneSearch(q ToneListQuery) (ToneListResult, error) {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = defaultPageSize
	}
	if q.PageSize > maxPageSize {
		q.PageSize = maxPageSize
	}

	base := applyToneFilters(db().Model(&Tone{}), q)

	var total int64
	if err := base.Count(&total).Error; err != nil {
		return ToneListResult{}, fmt.Errorf("tone list count: %w", err)
	}

	var rows []Tone
	offset := (q.Page - 1) * q.PageSize
	if err := base.Order("sort_order asc, id asc").Limit(q.PageSize).Offset(offset).Find(&rows).Error; err != nil {
		return ToneListResult{}, fmt.Errorf("tone list query: %w", err)
	}

	items := make([]*ToneView, 0, len(rows))
	for i := range rows {
		items = append(items, toneToView(&rows[i]))
	}

	totalPages := 0
	if total > 0 {
		totalPages = int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	}
	return ToneListResult{
		Items:      items,
		Total:      total,
		Page:       q.Page,
		PageSize:   q.PageSize,
		TotalPages: totalPages,
	}, nil
}

// ToneExportRows returns every tone matching the filters, in plaza order. An
// export is the whole selection rather than one page of it, so there is no
// pagination here; the CSV handler streams the rows straight out.
func ToneExportRows(q ToneListQuery) ([]Tone, error) {
	rows := []Tone{}
	query := applyToneFilters(db().Model(&Tone{}), q)
	if err := query.Order("sort_order asc, id asc").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("tone export query: %w", err)
	}
	return rows, nil
}

// ToneGetByID loads one tone by primary key.
func ToneGetByID(id uint) (*ToneView, error) {
	t := &Tone{}
	if err := db().First(t, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrToneNotFound
		}
		return nil, err
	}
	return toneToView(t), nil
}

// ToneInsert validates and creates one tone. The unique index on name is the
// authority; a collision is translated into a message an operator can act on.
func ToneInsert(dto *ToneCreateDTO) (*ToneView, error) {
	enabled := true
	if dto.Enabled != nil {
		enabled = *dto.Enabled
	}
	sortOrder := 0
	if dto.SortOrder != nil {
		sortOrder = *dto.SortOrder
	}

	t := &Tone{
		Name:         strings.TrimSpace(dto.Name),
		Description:  strings.TrimSpace(dto.Description),
		Prompt:       strings.TrimSpace(dto.Prompt),
		Category:     normalizeVocabulary(dto.Category),
		Tone:         normalizeVocabulary(dto.Tone),
		Language:     normalizeVocabulary(dto.Language),
		Scenes:       scenesToCSV(dto.Scenes),
		SampleInput:  strings.TrimSpace(dto.SampleInput),
		SampleOutput: strings.TrimSpace(dto.SampleOutput),
		Enabled:      enabled,
		SortOrder:    sortOrder,
	}
	if err := validateTone(t); err != nil {
		return nil, err
	}
	if err := db().Create(t).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("文风名称 %q 已存在", t.Name)
		}
		return nil, fmt.Errorf("create tone: %w", err)
	}
	return toneToView(t), nil
}

// ToneUpdate applies the provided fields of dto to the tone at id. Omitted
// (nil) fields keep their stored value; explicit empty strings clear them.
func ToneUpdate(id uint, dto *ToneUpdateDTO) (*ToneView, error) {
	t := &Tone{}
	if err := db().First(t, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrToneNotFound
		}
		return nil, err
	}

	if dto.Name != nil {
		t.Name = strings.TrimSpace(*dto.Name)
	}
	if dto.Description != nil {
		t.Description = strings.TrimSpace(*dto.Description)
	}
	if dto.Prompt != nil {
		t.Prompt = strings.TrimSpace(*dto.Prompt)
	}
	if dto.Category != nil {
		t.Category = normalizeVocabulary(*dto.Category)
	}
	if dto.Tone != nil {
		t.Tone = normalizeVocabulary(*dto.Tone)
	}
	if dto.Language != nil {
		t.Language = normalizeVocabulary(*dto.Language)
	}
	if dto.Scenes != nil {
		t.Scenes = scenesToCSV(*dto.Scenes)
	}
	if dto.SampleInput != nil {
		t.SampleInput = strings.TrimSpace(*dto.SampleInput)
	}
	if dto.SampleOutput != nil {
		t.SampleOutput = strings.TrimSpace(*dto.SampleOutput)
	}
	if dto.Enabled != nil {
		t.Enabled = *dto.Enabled
	}
	if dto.SortOrder != nil {
		t.SortOrder = *dto.SortOrder
	}

	if err := validateTone(t); err != nil {
		return nil, err
	}
	if err := db().Save(t).Error; err != nil {
		if isUniqueViolation(err, "name") {
			return nil, fmt.Errorf("文风名称 %q 已存在", t.Name)
		}
		return nil, fmt.Errorf("update tone: %w", err)
	}
	return toneToView(t), nil
}

// ToneDelete removes one tone by id and returns its name (for the admin
// confirmation message).
func ToneDelete(id uint) (string, error) {
	t := &Tone{}
	if err := db().Select("id", "name").First(t, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrToneNotFound
		}
		return "", fmt.Errorf("load tone: %w", err)
	}
	if err := db().Delete(&Tone{}, "id = ?", id).Error; err != nil {
		return t.Name, fmt.Errorf("delete tone: %w", err)
	}
	return t.Name, nil
}

// validateTone enforces the field contract shared by create and update. The
// same caps are published to clients by GET /api/zsy/tone/standard, so a
// mismatch here is a contract bug rather than a private implementation detail
// (and a test asserts the two agree).
func validateTone(t *Tone) error {
	switch {
	case t.Name == "":
		return errors.New("文风名称不能为空")
	case len([]rune(t.Name)) > maxNameLen:
		return fmt.Errorf("文风名称过长 (上限 %d 字符)", maxNameLen)
	case t.Prompt == "":
		// A tone is its instruction. Without one the row is a name and a
		// description, which a consumer cannot send anywhere — and the failure
		// would show up upstream as an empty style, not as a missing field.
		return errors.New("文风提示词不能为空")
	case len([]rune(t.Prompt)) > maxPromptLen:
		return fmt.Errorf("文风提示词过长 (上限 %d 字符)", maxPromptLen)
	case len([]rune(t.Description)) > maxDescriptionLen:
		return fmt.Errorf("简介过长 (上限 %d 字符)", maxDescriptionLen)
	case len([]rune(t.SampleInput)) > maxSampleLen:
		return fmt.Errorf("示例原文过长 (上限 %d 字符)", maxSampleLen)
	case len([]rune(t.SampleOutput)) > maxSampleLen:
		return fmt.Errorf("示例改写过长 (上限 %d 字符)", maxSampleLen)
	case t.SortOrder < -maxSortOrderAbs || t.SortOrder > maxSortOrderAbs:
		return fmt.Errorf("排序值超出范围 (-%d ~ %d)", maxSortOrderAbs, maxSortOrderAbs)
	}
	if t.Category != "" {
		if _, ok := allowedCategories[t.Category]; !ok {
			return fmt.Errorf("非法类别 %q (可选 %s，留空表示未分类)",
				t.Category, strings.Join(AllowedCategories, " / "))
		}
	}
	if t.Tone != "" {
		if _, ok := allowedTones[t.Tone]; !ok {
			return fmt.Errorf("非法语气 %q (可选 %s，留空表示未指定)",
				t.Tone, strings.Join(AllowedTones, " / "))
		}
	}
	if err := checkLanguage(t.Language); err != nil {
		return err
	}
	return checkScenes(t.Scenes)
}

// checkLanguage validates the optional language tag: a short lower-case code
// ("zh", "en", "pt-br"). The list stays open so a client's own spelling is
// accepted; the vocabulary endpoint publishes no language list for that reason.
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

// normalizeVocabulary lower-cases a controlled-vocabulary value so "Warm" from
// a spreadsheet and "warm" from the API are stored — and later filtered — as
// the same thing. Values outside the vocabulary are rejected by validateTone.
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
