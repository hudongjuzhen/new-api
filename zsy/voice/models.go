package voice

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Voice is one entry of the 音色广场 (Voice Plaza): a display name, an
// introduction, the upstream `voice_type` value a TTS request sends, the
// demographic attributes an operator sorts the plaza by (gender, age range,
// suitable scenes), and an optional sample audio file stored by this gateway
// (uploads/voices/...).
//
// Rows are hard-deleted: the catalog is editorial content, not an audit trail,
// and a freed name must be reusable right away (a soft-deleted row would keep
// occupying the unique index on name).
type Voice struct {
	ID        uint  `gorm:"primarykey"   json:"id"`
	CreatedAt int64 `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt int64 `gorm:"autoUpdateTime" json:"updatedAt"`

	Name        string `gorm:"type:varchar(191);not null;uniqueIndex" json:"name"`
	Description string `gorm:"type:text"                              json:"description"`
	VoiceType   string `gorm:"type:varchar(191);not null;index"       json:"voiceType"`

	// Gender and AgeRange are controlled vocabularies (see the constants
	// below); an empty value means "未标注" and is allowed. They are indexed so
	// the plaza can be segmented (客服库里的女声 / 老年音色 …) without a scan.
	Gender   string `gorm:"type:varchar(16);index" json:"gender"`
	AgeRange string `gorm:"type:varchar(16);index" json:"ageRange"`

	// Language is the voice's language tag ("zh", "en", "pt"…), lower-case and
	// optional. A short tag rather than a controlled list: providers name their
	// languages differently ("mx" for Mexican Spanish, "tl" for Tagalog), and a
	// closed vocabulary would reject the next provider's spelling.
	Language string `gorm:"type:varchar(16);index" json:"language"`

	// Scenes is the "适合场景" tag list persisted as a comma-separated string
	// ("客服播报,有声书"). CSV keeps the column portable across SQLite, MySQL
	// and PostgreSQL and makes the CSV import/export round trip lossless; the
	// API exposes it as an array (see VoiceView.Scenes).
	Scenes string `gorm:"type:varchar(255)" json:"scenes"`

	// Portrait of the voice ("头像"), shown in the plaza. Like the sample audio
	// it is either a gateway-served "/uploads/..." path or an absolute http(s)
	// URL.
	AvatarURL string `gorm:"type:varchar(768)" json:"avatarUrl"`

	// Sample audio: URL as served by the gateway (a local "/uploads/voices/..."
	// path returned by the upload endpoint, or an absolute http(s) URL the
	// operator pasted in).
	AudioURL  string `gorm:"type:varchar(768)"  json:"audioUrl"`
	AudioName string `gorm:"type:varchar(255)"  json:"audioName"`
	AudioSize int64  `gorm:"default:0;not null" json:"audioSize"`

	// Enabled selects whether the voice is visible through the public list.
	// New rows default to enabled in VoiceInsert — the default lives in code
	// (not a gorm tag) so AutoMigrate never fights a dialect-specific boolean
	// default.
	Enabled bool `gorm:"index"              json:"enabled"`
	// SortOrder orders the plaza; ties fall back to id ascending.
	SortOrder int `gorm:"default:0;not null" json:"sortOrder"`
}

// TableName pins the table to a plugin-prefixed name. The host shares one
// database between core tables and every installed plugin, and a bare `voices`
// is too generic to claim there.
func (Voice) TableName() string { return "zsy_voices" }

// Controlled vocabularies for gender and age range. The wire values are short
// English identifiers so a third-party caller can branch on them; the admin UI
// renders localized labels.
const (
	GenderMale    = "male"
	GenderFemale  = "female"
	GenderNeutral = "neutral"

	AgeChild  = "child"
	AgeTeen   = "teen"
	AgeYoung  = "young"
	AgeMiddle = "middle"
	AgeSenior = "senior"
)

var allowedGenders = map[string]struct{}{
	GenderMale:    {},
	GenderFemale:  {},
	GenderNeutral: {},
}

var allowedAgeRanges = map[string]struct{}{
	AgeChild:  {},
	AgeTeen:   {},
	AgeYoung:  {},
	AgeMiddle: {},
	AgeSenior: {},
}

// Validation bounds. Names/types stay inside the indexed varchar width so
// MySQL (utf8mb4) accepts them; the free-text fields are capped to keep a
// public catalog payload bounded.
const (
	maxNameLen        = 191
	maxVoiceTypeLen   = 191
	maxDescriptionLen = 2000
	maxAudioURLLen    = 768
	maxAvatarURLLen   = 768
	maxAudioNameLen   = 255
	maxLanguageLen    = 16
	maxSortOrderAbs   = 1_000_000

	// Scenes: at most 8 tags of 24 characters each, which keeps the stored
	// comma-separated string inside its 255 character column.
	maxScenes       = 8
	maxSceneLen     = 24
	maxScenesCSVLen = 255
)

// Pagination bounds shared by the public and admin list endpoints.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// VoiceView is the read shape returned by every endpoint. CreatedAt/UpdatedAt
// are unix seconds (the host's own DTO convention) so the frontend can format
// them without a timezone guess.
type VoiceView struct {
	ID          uint     `json:"id"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	VoiceType   string   `json:"voiceType"`
	Gender      string   `json:"gender"`
	AgeRange    string   `json:"ageRange"`
	Language    string   `json:"language"`
	Scenes      []string `json:"scenes"`
	AvatarURL   string   `json:"avatarUrl"`
	AudioURL    string   `json:"audioUrl"`
	AudioName   string   `json:"audioName"`
	AudioSize   int64    `json:"audioSize"`
	Enabled     bool     `json:"enabled"`
	SortOrder   int      `json:"sortOrder"`
}

// VoiceListQuery is the normalized filter/pagination input of VoiceSearch.
// Enabled is a pointer so the admin list can filter on either value while the
// public list pins it to true; Page/PageSize are clamped by VoiceSearch.
type VoiceListQuery struct {
	Keyword   string `json:"keyword"`
	VoiceType string `json:"voiceType"`
	Gender    string `json:"gender"`
	AgeRange  string `json:"ageRange"`
	Language  string `json:"language"`
	Enabled   *bool  `json:"enabled"`
	Page      int    `json:"page"`
	PageSize  int    `json:"pageSize"`
}

// VoiceListResult is the paginated list envelope. Field names follow the other
// zsy plugins' list shapes (items / total / page / pageSize / totalPages).
type VoiceListResult struct {
	Items      []*VoiceView `json:"items"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"pageSize"`
	TotalPages int          `json:"totalPages"`
}

// VoiceCreateDTO is the admin create payload. Enabled/SortOrder are pointers so
// an omitted field means "host default" (enabled, sort 0) while an explicit
// false/0 is honoured.
type VoiceCreateDTO struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	VoiceType   string      `json:"voiceType"`
	Gender      string      `json:"gender"`
	AgeRange    string      `json:"ageRange"`
	Language    string      `json:"language"`
	Scenes      ScenesInput `json:"scenes"`
	AvatarURL   string      `json:"avatarUrl"`
	AudioURL    string      `json:"audioUrl"`
	AudioName   string      `json:"audioName"`
	AudioSize   int64       `json:"audioSize"`
	Enabled     *bool       `json:"enabled"`
	SortOrder   *int        `json:"sortOrder"`
}

// VoiceUpdateDTO is the admin update payload: every field is optional, and an
// explicit empty string clears it (so a voice can be detached from its sample
// audio without being recreated).
type VoiceUpdateDTO struct {
	Name        *string      `json:"name"`
	Description *string      `json:"description"`
	VoiceType   *string      `json:"voiceType"`
	Gender      *string      `json:"gender"`
	AgeRange    *string      `json:"ageRange"`
	Language    *string      `json:"language"`
	Scenes      *ScenesInput `json:"scenes"`
	AvatarURL   *string      `json:"avatarUrl"`
	AudioURL    *string      `json:"audioUrl"`
	AudioName   *string      `json:"audioName"`
	AudioSize   *int64       `json:"audioSize"`
	Enabled     *bool        `json:"enabled"`
	SortOrder   *int         `json:"sortOrder"`
}

// ScenesInput accepts the "适合场景" tag list in either shape a caller
// naturally writes: a JSON array (["客服播报","有声书"]) or a separated string
// ("客服播报,有声书", also accepting 、 ； ; and spaces as separators). Both
// end up as the same normalized slice.
type ScenesInput []string

// UnmarshalJSON implements json.Unmarshaler for the two accepted shapes.
func (s *ScenesInput) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "null" {
		*s = nil
		return nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var list []string
		if err := common.Unmarshal(data, &list); err != nil {
			return err
		}
		*s = list
		return nil
	}
	var raw string
	if err := common.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = splitScenes(raw)
	return nil
}

// sceneSeparators covers the separators an operator may type in a single cell
// or paste from a spreadsheet: Chinese and Latin commas, semicolons and pipes.
const sceneSeparators = ",，、;；|"

// splitScenes splits a separated scene string and drops empty entries.
func splitScenes(raw string) []string {
	out := []string{}
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool {
		return strings.ContainsRune(sceneSeparators, r)
	}) {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// scenesToCSV normalizes a scene list (trim, drop blanks, de-duplicate keeping
// the first occurrence) and joins it for storage. The result is the canonical
// form stored in Voice.Scenes and written to an export.
func scenesToCSV(scenes []string) string {
	seen := make(map[string]struct{}, len(scenes))
	normalized := make([]string, 0, len(scenes))
	for _, scene := range scenes {
		scene = strings.TrimSpace(scene)
		if scene == "" {
			continue
		}
		if _, dup := seen[scene]; dup {
			continue
		}
		seen[scene] = struct{}{}
		normalized = append(normalized, scene)
	}
	return strings.Join(normalized, ",")
}

// scenesFromCSV decodes the stored comma-separated list. An empty column is a
// valid "no scenes" row, and the JSON shape is always an array (never null) so
// a consumer can iterate without a nil check.
func scenesFromCSV(stored string) []string {
	if strings.TrimSpace(stored) == "" {
		return []string{}
	}
	out := strings.Split(stored, ",")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

// voiceToView projects a stored row onto the API shape.
func voiceToView(v *Voice) *VoiceView {
	return &VoiceView{
		ID:          v.ID,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
		Name:        v.Name,
		Description: v.Description,
		VoiceType:   v.VoiceType,
		Gender:      v.Gender,
		AgeRange:    v.AgeRange,
		Language:    v.Language,
		Scenes:      scenesFromCSV(v.Scenes),
		AvatarURL:   v.AvatarURL,
		AudioURL:    v.AudioURL,
		AudioName:   v.AudioName,
		AudioSize:   v.AudioSize,
		Enabled:     v.Enabled,
		SortOrder:   v.SortOrder,
	}
}
