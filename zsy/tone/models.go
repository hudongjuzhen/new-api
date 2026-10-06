package tone

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Tone is one entry of the 文风广场 (Tone Plaza): a named writing style an
// operator publishes for other applications to pick from.
//
// # What a tone row actually carries
//
// A voice row carries a `voice_type` a TTS request sends; a tone row carries the
// thing a *text* request sends — Prompt, the writing instruction itself. The
// rest of the row exists so an operator can browse and filter a catalog of them:
// a display name, an introduction, two controlled facets (Category, Tone), a
// language tag, a "适合场景" tag list, and the on-shelf state.
//
// # Prompt is the payload; everything else is signage
//
// Prompt is the only field a consumer must send upstream. It is written as an
// instruction ("以温暖而克制的笔调……"), not as a description of one, because the
// consumer pastes it straight into a system prompt without rewriting it. A
// description of a style ("a warm, restrained tone") changes the model's output
// far less than the instruction does — the catalog would look right and read
// wrong.
//
// # SampleInput / SampleOutput: the answer to "what does this tone look like"
//
// A voice can be auditioned and a persona has a face; a tone has neither. These
// two columns are that missing sense: SampleInput is a passage, SampleOutput is
// the same passage rewritten in this tone. They are a *pair*, and the plaza is
// worth browsing only because operators are expected to publish **the same
// SampleInput across rows** — a shared passage is what lets a reader compare two
// tones side by side instead of reading two unrelated paragraphs.
//
// They are also what keeps the plaza free: the comparison is stored, not
// generated, so browsing the catalog costs nothing. (Live "rewrite my own
// sentence in this tone" is the caller's job, not this table's.)
//
// # Rows are hard-deleted
//
// Same reason as zsy/voice: the catalog is editorial content, not an audit
// trail, and a freed name must be reusable right away (a soft-deleted row would
// keep occupying the unique index on name).
type Tone struct {
	ID        uint  `gorm:"primarykey"   json:"id"`
	CreatedAt int64 `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt int64 `gorm:"autoUpdateTime" json:"updatedAt"`

	Name        string `gorm:"type:varchar(191);not null;uniqueIndex" json:"name"`
	Description string `gorm:"type:text"                              json:"description"`

	// Prompt is the writing instruction a consumer sends upstream. Required:
	// a tone without its instruction is a name with nothing behind it, and the
	// consumer would have no way to tell that from a working row.
	Prompt string `gorm:"type:text;not null" json:"prompt"`

	// Category and Tone are controlled vocabularies (see the constants below);
	// an empty value means "未分类" / "未指定" and is allowed. Both are indexed so
	// the plaza can be segmented without a scan.
	//
	// ⚠ The Tone field shares its name with the Tone type. That is deliberate and
	// legal in Go (a field lives in the type's own scope): `t.Tone` reads as
	// "this tone's 语气", which is the clearest name the column has. Renaming
	// either one would cost more than it buys — see the wire contract: the JSON
	// key, the CSV column and the query parameter are all `tone`.
	Category string `gorm:"type:varchar(32);index" json:"category"`
	Tone     string `gorm:"type:varchar(32);index" json:"tone"`

	// Language is the tone's language tag ("zh", "en", "pt-br"…), lower-case and
	// optional. A short tag rather than a controlled list, for the same reason as
	// zsy/voice: providers and writers spell languages their own way, and a
	// closed vocabulary would reject the next one.
	Language string `gorm:"type:varchar(16);index" json:"language"`

	// Scenes is the "适合场景" tag list persisted as a comma-separated string
	// ("公众号长文,口播稿"), exposed by the API as an array. CSV keeps the column
	// portable across SQLite, MySQL and PostgreSQL and makes the CSV import and
	// export a lossless round trip.
	Scenes string `gorm:"type:varchar(255)" json:"scenes"`

	// The worked example pair; either or both may be empty (see the type doc).
	SampleInput  string `gorm:"type:text" json:"sampleInput"`
	SampleOutput string `gorm:"type:text" json:"sampleOutput"`

	// Enabled selects whether the tone is visible through the public list.
	// New rows default to enabled in ToneInsert — the default lives in code (not
	// a gorm tag) so AutoMigrate never fights a dialect-specific boolean default.
	Enabled bool `gorm:"index"              json:"enabled"`
	// SortOrder orders the plaza; ties fall back to id ascending.
	SortOrder int `gorm:"default:0;not null" json:"sortOrder"`
}

// TableName pins the table to a plugin-prefixed name. The host shares one
// database between core tables and every installed plugin, and a bare `tones`
// is too generic to claim there.
func (Tone) TableName() string { return "zsy_tones" }

// Controlled vocabulary: the kind of writing a tone is for. Wire values are
// short English identifiers so a third-party caller can branch on them; the
// admin UI renders localized labels.
//
// The set is deliberately the shapes a text request actually arrives in
// (a 公众号 post, a report, a script) rather than a taxonomy of prose — an
// operator picks a category while filing a tone, and a caller picks one while
// asking "what am I writing today".
const (
	CategoryLiterary  = "literary"  // 文学
	CategoryBusiness  = "business"  // 商务
	CategoryAcademic  = "academic"  // 学术
	CategoryMedia     = "media"     // 媒体
	CategorySpoken    = "spoken"    // 口播
	CategoryTechnical = "technical" // 技术
	CategoryMarketing = "marketing" // 营销
)

// Controlled vocabulary: the 语气 of a tone. These are the axes a writer
// actually chooses between, and the ones the plaza's filter row offers.
const (
	ToneWarm     = "warm"     // 温暖
	ToneCalm     = "calm"     // 冷静
	ToneSharp    = "sharp"    // 犀利
	ToneHumorous = "humorous" // 幽默
	ToneSolemn   = "solemn"   // 庄重
	ToneLively   = "lively"   // 活泼
	TonePlain    = "plain"    // 平实
)

var allowedCategories = map[string]struct{}{
	CategoryLiterary:  {},
	CategoryBusiness:  {},
	CategoryAcademic:  {},
	CategoryMedia:     {},
	CategorySpoken:    {},
	CategoryTechnical: {},
	CategoryMarketing: {},
}

var allowedTones = map[string]struct{}{
	ToneWarm:     {},
	ToneCalm:     {},
	ToneSharp:    {},
	ToneHumorous: {},
	ToneSolemn:   {},
	ToneLively:   {},
	TonePlain:    {},
}

// AllowedCategories and AllowedTones expose the vocabularies in a stable order
// for the seed tooling and for tests; the maps above stay the authority for
// validation. Order is the declaration order above, which is also the order the
// admin filter row shows.
var (
	AllowedCategories = []string{
		CategoryLiterary,
		CategoryBusiness,
		CategoryAcademic,
		CategoryMedia,
		CategorySpoken,
		CategoryTechnical,
		CategoryMarketing,
	}
	AllowedTones = []string{
		ToneWarm,
		ToneCalm,
		ToneSharp,
		ToneHumorous,
		ToneSolemn,
		ToneLively,
		TonePlain,
	}
)

// Validation bounds. Names stay inside the indexed varchar width so MySQL
// (utf8mb4) accepts them.
//
// Prompt is the widest field in the plugin on purpose: a usable writing
// instruction runs to several hundred characters once it names the voice, the
// structure and the things to avoid, and truncating it would silently strip the
// constraints at the end — which is where the "不要……" clauses live.
const (
	maxNameLen        = 191
	maxDescriptionLen = 2000
	maxPromptLen      = 8000
	maxSampleLen      = 4000
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

// ToneView is the read shape returned by every endpoint. CreatedAt/UpdatedAt
// are unix seconds (the host's own DTO convention) so the frontend can format
// them without a timezone guess.
type ToneView struct {
	ID           uint     `json:"id"`
	CreatedAt    int64    `json:"createdAt"`
	UpdatedAt    int64    `json:"updatedAt"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Prompt       string   `json:"prompt"`
	Category     string   `json:"category"`
	Tone         string   `json:"tone"`
	Language     string   `json:"language"`
	Scenes       []string `json:"scenes"`
	SampleInput  string   `json:"sampleInput"`
	SampleOutput string   `json:"sampleOutput"`
	Enabled      bool     `json:"enabled"`
	SortOrder    int      `json:"sortOrder"`
}

// ToneListQuery is the normalized filter/pagination input of ToneSearch.
// Enabled is a pointer so the admin list can filter on either value while the
// public list pins it to true; Page/PageSize are clamped by ToneSearch.
type ToneListQuery struct {
	Keyword  string `json:"keyword"`
	Category string `json:"category"`
	Tone     string `json:"tone"`
	Language string `json:"language"`
	Enabled  *bool  `json:"enabled"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// ToneListResult is the paginated list envelope. Field names follow the other
// zsy plugins' list shapes (items / total / page / pageSize / totalPages).
type ToneListResult struct {
	Items      []*ToneView `json:"items"`
	Total      int64       `json:"total"`
	Page       int         `json:"page"`
	PageSize   int         `json:"pageSize"`
	TotalPages int         `json:"totalPages"`
}

// ToneCreateDTO is the admin create payload. Enabled/SortOrder are pointers so
// an omitted field means "host default" (enabled, sort 0) while an explicit
// false/0 is honoured.
type ToneCreateDTO struct {
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Prompt       string      `json:"prompt"`
	Category     string      `json:"category"`
	Tone         string      `json:"tone"`
	Language     string      `json:"language"`
	Scenes       ScenesInput `json:"scenes"`
	SampleInput  string      `json:"sampleInput"`
	SampleOutput string      `json:"sampleOutput"`
	Enabled      *bool       `json:"enabled"`
	SortOrder    *int        `json:"sortOrder"`
}

// ToneUpdateDTO is the admin update payload: every field is optional, and an
// explicit empty string clears it (so a worked example can be detached from a
// tone without recreating the row).
type ToneUpdateDTO struct {
	Name         *string      `json:"name"`
	Description  *string      `json:"description"`
	Prompt       *string      `json:"prompt"`
	Category     *string      `json:"category"`
	Tone         *string      `json:"tone"`
	Language     *string      `json:"language"`
	Scenes       *ScenesInput `json:"scenes"`
	SampleInput  *string      `json:"sampleInput"`
	SampleOutput *string      `json:"sampleOutput"`
	Enabled      *bool        `json:"enabled"`
	SortOrder    *int         `json:"sortOrder"`
}

// ScenesInput accepts the "适合场景" tag list in either shape a caller
// naturally writes: a JSON array (["公众号长文","口播稿"]) or a separated string
// ("公众号长文,口播稿", also accepting 、 ； ; and spaces as separators). Both end
// up as the same normalized slice.
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
// form stored in Tone.Scenes and written to an export.
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

// toneToView projects a stored row onto the API shape.
func toneToView(t *Tone) *ToneView {
	return &ToneView{
		ID:           t.ID,
		CreatedAt:    t.CreatedAt,
		UpdatedAt:    t.UpdatedAt,
		Name:         t.Name,
		Description:  t.Description,
		Prompt:       t.Prompt,
		Category:     t.Category,
		Tone:         t.Tone,
		Language:     t.Language,
		Scenes:       scenesFromCSV(t.Scenes),
		SampleInput:  t.SampleInput,
		SampleOutput: t.SampleOutput,
		Enabled:      t.Enabled,
		SortOrder:    t.SortOrder,
	}
}
