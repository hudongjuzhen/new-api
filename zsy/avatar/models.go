package avatar

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// Avatar is one entry of the 形象广场 (Avatar Plaza): a named persona an
// application can pick — its four pictures (封面图 plus the 全身照 / 四视图 /
// 表情图 reference material), the demographic attributes an operator sorts by
// (gender, age range, ethnicity, suitable scenes), an introduction, and the voice
// the persona speaks with.
//
// The voice is referenced by its `voice_type` value (VoiceID, e.g.
// "zh_female_vv_uranus_bigtts") rather than by a foreign key: the 音色广场 owns
// the voice rows, a persona only names the one it belongs to, and a persona
// whose voice has not been imported yet is still a valid, publishable row. The
// sample audio is therefore never copied onto this row — it is resolved from the
// voice catalog on read (see VoiceSampleResolver in store.go), so re-uploading a
// voice sample immediately updates every persona that uses that voice.
//
// Rows are hard-deleted, exactly like the voice catalog: this is editorial
// content, not an audit trail, and a freed name must be reusable right away.
type Avatar struct {
	ID        uint  `gorm:"primarykey"   json:"id"`
	CreatedAt int64 `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt int64 `gorm:"autoUpdateTime" json:"updatedAt"`

	Name        string `gorm:"type:varchar(191);not null;uniqueIndex" json:"name"`
	Description string `gorm:"type:text"                              json:"description"`

	// The persona carries four pictures, stored one URL each: either a
	// gateway-served "/uploads/images/..." path (the host's own image upload
	// endpoint) or an absolute http(s) URL the operator pasted in. ImageURL is
	// the 封面图 (cover) an application lists the persona with; the three others
	// are the reference material a likeness is built from — 全身照 (full body),
	// 四视图 (the four-view sheet) and 表情图 (the expressions). All four are
	// optional and validated by the same rule.
	ImageURL      string `gorm:"type:varchar(768)" json:"imageUrl"`
	FullBodyURL   string `gorm:"type:varchar(768)" json:"fullBodyUrl"`
	FourViewURL   string `gorm:"type:varchar(768)" json:"fourViewUrl"`
	ExpressionURL string `gorm:"type:varchar(768)" json:"expressionUrl"`

	// Gender, AgeRange and Race are controlled vocabularies (see the constants
	// below); an empty value means "未标注" and is allowed. They are indexed so
	// the plaza can be segmented without a scan.
	Gender   string `gorm:"type:varchar(16);index" json:"gender"`
	AgeRange string `gorm:"type:varchar(16);index" json:"ageRange"`
	Race     string `gorm:"type:varchar(16);index" json:"race"`

	// Scenes is the "适合场景" tag list persisted as a comma-separated string
	// ("客服播报,有声书"), the same portable encoding the voice catalog uses: it
	// survives SQLite, MySQL and PostgreSQL identically and makes the CSV
	// import/export round trip lossless. The API exposes it as an array.
	Scenes string `gorm:"type:varchar(255)" json:"scenes"`

	// VoiceID is the `voice_type` of the voice this persona speaks with. It is
	// free text so a persona can name a provider voice that is not (yet) in the
	// 音色广场 catalog.
	VoiceID string `gorm:"type:varchar(191);index" json:"voiceId"`

	// Enabled selects whether the persona is visible through the public list.
	// New rows default to enabled in AvatarInsert — the default lives in code
	// (not a gorm tag) so AutoMigrate never fights a dialect-specific boolean
	// default.
	Enabled bool `gorm:"index"              json:"enabled"`
	// SortOrder orders the plaza; ties fall back to id ascending.
	SortOrder int `gorm:"default:0;not null" json:"sortOrder"`
}

// TableName pins the table to a plugin-prefixed name. The host shares one
// database between core tables and every installed plugin, and a bare `avatars`
// is too generic to claim there.
func (Avatar) TableName() string { return "zsy_avatars" }

// Controlled vocabularies for gender, age range and ethnicity. The wire values
// are short English identifiers so a third-party caller can branch on them; the
// admin UI renders localized labels.
const (
	GenderMale    = "male"
	GenderFemale  = "female"
	GenderNeutral = "neutral"

	AgeChild  = "child"
	AgeTeen   = "teen"
	AgeYoung  = "young"
	AgeMiddle = "middle"
	AgeSenior = "senior"

	RaceAsian         = "asian"
	RaceBlack         = "black"
	RaceWhite         = "white"
	RaceLatino        = "latino"
	RaceMiddleEastern = "middle_eastern"
	RaceSouthAsian    = "south_asian"
	RaceMixed         = "mixed"
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

// allowedRaces is the shared ethnicity vocabulary, deliberately the one the
// common Chinese AIGC persona libraries use, so an imported spreadsheet does not
// have to be reworded.
var allowedRaces = map[string]struct{}{
	RaceAsian:         {},
	RaceBlack:         {},
	RaceWhite:         {},
	RaceLatino:        {},
	RaceMiddleEastern: {},
	RaceSouthAsian:    {},
	RaceMixed:         {},
}

// Validation bounds. Names/identifiers stay inside the indexed varchar width so
// MySQL (utf8mb4) accepts them; the free-text fields are capped to keep a public
// catalog payload bounded.
const (
	maxNameLen        = 191
	maxDescriptionLen = 2000
	maxImageURLLen    = 768
	maxVoiceIDLen     = 191
	maxSortOrderAbs   = 1_000_000

	// Scenes: at most 8 tags of 24 characters each, which keeps the stored
	// comma-separated string inside its 255 character column.
	maxScenes       = 8
	maxSceneLen     = 24
	maxScenesCSVLen = 255
)

// The four pictures, named as the operator sees them in the admin form. The
// labels are only used to build a validation message that says which picture to
// fix, so rejecting 全身照 cannot be mistaken for rejecting 封面图.
const (
	labelCoverImage      = "封面图"
	labelFullBodyImage   = "全身照"
	labelFourViewImage   = "四视图"
	labelExpressionImage = "表情图"
)

// Pagination bounds shared by the public and admin list endpoints.
const (
	defaultPageSize = 20
	maxPageSize     = 100
)

// AvatarView is the read shape returned by every endpoint. CreatedAt/UpdatedAt
// are unix seconds (the host's own DTO convention) so the frontend can format
// them without a timezone guess.
//
// VoiceSampleURL, VoiceSampleName and VoiceAvailable are resolved from the
// 音色广场 catalog on every read rather than stored on the row, so a persona
// always reports the voice sample the linked voice currently carries.
type AvatarView struct {
	ID          uint   `json:"id"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageURL    string `json:"imageUrl"`
	// The three reference pictures, '' when the persona does not carry one yet.
	FullBodyURL   string   `json:"fullBodyUrl"`
	FourViewURL   string   `json:"fourViewUrl"`
	ExpressionURL string   `json:"expressionUrl"`
	Gender        string   `json:"gender"`
	AgeRange      string   `json:"ageRange"`
	Race          string   `json:"race"`
	Scenes        []string `json:"scenes"`
	VoiceID       string   `json:"voiceId"`
	// VoiceAvailable reports whether VoiceID exists in the 音色广场 catalog.
	// VoiceName is that voice's display name ('' when it is not catalogued).
	VoiceAvailable  bool   `json:"voiceAvailable"`
	VoiceName       string `json:"voiceName"`
	VoiceSampleURL  string `json:"voiceSampleUrl"`
	VoiceSampleName string `json:"voiceSampleName"`
	Enabled         bool   `json:"enabled"`
	SortOrder       int    `json:"sortOrder"`
}

// avatarToView projects a stored row onto the API shape. The voice fields stay
// empty here: they are filled by the read path once the resolver has answered.
func avatarToView(a *Avatar) *AvatarView {
	return &AvatarView{
		ID:            a.ID,
		CreatedAt:     a.CreatedAt,
		UpdatedAt:     a.UpdatedAt,
		Name:          a.Name,
		Description:   a.Description,
		ImageURL:      a.ImageURL,
		FullBodyURL:   a.FullBodyURL,
		FourViewURL:   a.FourViewURL,
		ExpressionURL: a.ExpressionURL,
		Gender:        a.Gender,
		AgeRange:      a.AgeRange,
		Race:          a.Race,
		Scenes:        scenesFromCSV(a.Scenes),
		VoiceID:       a.VoiceID,
		Enabled:       a.Enabled,
		SortOrder:     a.SortOrder,
	}
}

// AvatarListQuery is the normalized filter/pagination input of AvatarSearch.
// Enabled is a pointer so the admin list can filter on either value while the
// public list pins it to true; Page/PageSize are clamped by AvatarSearch.
type AvatarListQuery struct {
	Keyword  string `json:"keyword"`
	Gender   string `json:"gender"`
	AgeRange string `json:"ageRange"`
	Race     string `json:"race"`
	VoiceID  string `json:"voiceId"`
	Enabled  *bool  `json:"enabled"`
	Page     int    `json:"page"`
	PageSize int    `json:"pageSize"`
}

// AvatarListResult is the paginated list envelope. Field names follow the other
// zsy plugins' list shapes (items / total / page / pageSize / totalPages).
type AvatarListResult struct {
	Items      []*AvatarView `json:"items"`
	Total      int64         `json:"total"`
	Page       int           `json:"page"`
	PageSize   int           `json:"pageSize"`
	TotalPages int           `json:"totalPages"`
}

// AvatarCreateDTO is the admin create payload. VoiceSampleURL is accepted and
// ignored: the sample always follows the linked voice. Enabled/SortOrder are
// pointers so an omitted field means "host default" (enabled, sort 0) while an
// explicit false/0 is honoured.
type AvatarCreateDTO struct {
	Name          string      `json:"name"`
	Description   string      `json:"description"`
	ImageURL      string      `json:"imageUrl"`
	FullBodyURL   string      `json:"fullBodyUrl"`
	FourViewURL   string      `json:"fourViewUrl"`
	ExpressionURL string      `json:"expressionUrl"`
	Gender        string      `json:"gender"`
	AgeRange      string      `json:"ageRange"`
	Race          string      `json:"race"`
	Scenes        ScenesInput `json:"scenes"`
	VoiceID       string      `json:"voiceId"`
	Enabled       *bool       `json:"enabled"`
	SortOrder     *int        `json:"sortOrder"`
}

// AvatarUpdateDTO is the admin update payload: every field is optional, and an
// explicit empty string clears it (so a persona can be detached from its voice —
// or from one of its pictures — without being recreated).
type AvatarUpdateDTO struct {
	Name          *string      `json:"name"`
	Description   *string      `json:"description"`
	ImageURL      *string      `json:"imageUrl"`
	FullBodyURL   *string      `json:"fullBodyUrl"`
	FourViewURL   *string      `json:"fourViewUrl"`
	ExpressionURL *string      `json:"expressionUrl"`
	Gender        *string      `json:"gender"`
	AgeRange      *string      `json:"ageRange"`
	Race          *string      `json:"race"`
	Scenes        *ScenesInput `json:"scenes"`
	VoiceID       *string      `json:"voiceId"`
	Enabled       *bool        `json:"enabled"`
	SortOrder     *int         `json:"sortOrder"`
}

// raceAliases and the age/gender tables below let a Chinese spelling reach the
// same wire value an English one does. Spreadsheets exported from the source
// persona libraries label the columns in Chinese ("女" / "青年" / "亚洲人"), and an
// operator importing one should not have to translate three columns by hand. The
// tables are deliberately explicit rather than generated: the wire vocabulary is
// the contract the public API publishes, and a wrong alias would silently
// mislabel every imported row.
var genderAliases = map[string]string{
	"男":  GenderMale,
	"男性": GenderMale,
	"女":  GenderFemale,
	"女性": GenderFemale,
	"中性": GenderNeutral,
	"无":  GenderNeutral,
}

var ageRangeAliases = map[string]string{
	"儿童":  AgeChild,
	"小孩":  AgeChild,
	"少年":  AgeTeen,
	"青少年": AgeTeen,
	"青年":  AgeYoung,
	"中年":  AgeMiddle,
	"老年":  AgeSenior,
	"老人":  AgeSenior,
}

var raceAliases = map[string]string{
	"亚洲":   RaceAsian,
	"亚洲人":  RaceAsian,
	"黑人":   RaceBlack,
	"黑人形象": RaceBlack,
	"白人":   RaceWhite,
	"白人形象": RaceWhite,
	"拉美":   RaceLatino,
	"拉丁裔":  RaceLatino,
	"中东":   RaceMiddleEastern,
	"中东人":  RaceMiddleEastern,
	"南亚":   RaceSouthAsian,
	"南亚人":  RaceSouthAsian,
	"混血":   RaceMixed,
	"混血人":  RaceMixed,
}

// normalizeVocabulary turns an operator's spelling of a controlled-vocabulary
// value into the wire value: trimmed, lower-cased, and — for the Chinese
// spellings above — translated. Values outside the vocabulary come back unchanged
// so validateAvatar can name them back to the operator verbatim.
func normalizeVocabulary(raw string, aliases map[string]string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	if mapped, ok := aliases[value]; ok {
		return mapped
	}
	return value
}

// ScenesInput accepts the "适合场景" tag list in either shape a caller naturally
// writes: a JSON array (["客服播报","有声书"]) or a separated string
// ("客服播报,有声书", also accepting 、 ； ; and spaces as separators). Both end up
// as the same normalized slice.
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

// sceneSeparators covers the separators an operator may type in a single cell or
// paste from a spreadsheet: Chinese and Latin commas, semicolons and pipes.
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
// form stored in Avatar.Scenes and written to an export.
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
// valid "no scenes" row, and the JSON shape is always an array (never null) so a
// consumer can iterate without a nil check.
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
