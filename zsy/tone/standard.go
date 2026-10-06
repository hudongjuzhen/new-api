package tone

// 文风标准 (Tone Standard) — the published, versioned definition of what a
// writing style is on this gateway.
//
// # Why this file exists separately from models.go
//
// models.go answers "what does the database row look like". This file answers a
// different question, one that outlives any single caller: **what are the
// allowed values, and what do they mean**. That question gets asked by client
// applications, by operators filling a spreadsheet, and by whoever edits the
// vocabulary next — and the moment two of them answer it from their own copy,
// the answers drift.
//
// It has already drifted once on this gateway. The voice and persona catalogs
// share `gender` / `ageRange`, and because the public voice endpoint publishes
// no vocabulary, the client had to keep its own label table (see this repo's
// `docs/zsy-voiceplaza-api.md` and the consuming app's `core/voiceCatalog.js`).
// The result is documented in that app's `docs/15` §3.3: two tables that must
// agree, with a test asserting they are "the same object" because nothing
// structural keeps them that way.
//
// The tone catalog does not repeat that. The vocabulary ships **as an endpoint**
// (GET /api/zsy/tone/standard) so a client reads it instead of retyping it, and
// this file is the single place the values are declared.
//
// # The compatibility promise
//
// A standard that changes freely is not a standard, so the rules are fixed here
// and repeated to callers in the endpoint's `compatibility` field:
//
//	1. A value that has been published is never renamed and never removed.
//	2. New values may be added; that is not a breaking change.
//	3. An empty value always means "not classified" / "not specified".
//	4. A client must tolerate a value it does not know — render it, do not drop
//	   the row. Unknown-value handling is what makes rule 2 safe to exercise.
//	5. The version moves when the *shape* changes (a new field, a new limit), not
//	   when a value is added.
//
// Rule 4 is the one that is easy to get wrong and expensive to discover: a
// client that filters out unrecognized values makes every vocabulary addition
// look like data loss to its users.

// ToneStandardVersion is the version of the *shape* this file publishes
// (fields, vocabularies and limits taken together), not of the catalog data.
//
// It moves when a field is added or a limit changes. Adding a value to an
// existing vocabulary does not move it — per compatibility rule 5 that is not a
// shape change, and clients following rule 4 accept it unchanged.
const ToneStandardVersion = "1.0.0"

// StandardOption is one published vocabulary entry.
//
// Labels are carried in the payload rather than left to each client's own i18n
// because the vocabulary is small, closed, and editorial: "warm" is 温暖 in the
// catalog's own words, and forcing every client to re-invent that translation is
// how the tables drift apart again. A client that localizes its UI still uses
// Value as the identity and may override the label.
type StandardOption struct {
	// Value is the wire value: what the API stores, filters on, and returns.
	// This is the identity — never translate it.
	Value string `json:"value"`
	// Label is the Chinese display name, matching the admin UI.
	Label string `json:"label"`
	// LabelEn is the English display name, for non-Chinese clients and logs.
	LabelEn string `json:"labelEn"`
	// Desc is one sentence on when to reach for this value, so an operator
	// filing a tone and a caller picking one read the same guidance.
	Desc string `json:"desc"`
}

// StandardLimits publishes the field caps the API enforces, so a client can
// refuse exactly what the server would reject instead of discovering it on
// submit (the consuming app's dialogs mirror these).
type StandardLimits struct {
	Name         int `json:"name"`
	Description  int `json:"description"`
	Prompt       int `json:"prompt"`
	Sample       int `json:"sample"`
	Language     int `json:"language"`
	Scenes       int `json:"scenes"`
	SceneLength  int `json:"sceneLength"`
	SortOrderAbs int `json:"sortOrderAbs"`
	MaxPageSize  int `json:"maxPageSize"`
}

// ToneStandard is the payload of GET /api/zsy/tone/standard.
type ToneStandard struct {
	Version    string           `json:"version"`
	Categories []StandardOption `json:"categories"`
	Tones      []StandardOption `json:"tones"`
	Limits     StandardLimits   `json:"limits"`
	// Compatibility restates the promise above in the payload itself, so the
	// rules travel with the data instead of living only in this repository.
	Compatibility []string `json:"compatibility"`
	// ExamplePair points at a published worked example: the SampleInput /
	// SampleOutput pair a client should expect to find on plaza rows.
	ExamplePair StandardExample `json:"examplePair"`
}

// StandardExample describes the worked-example convention: rows are expected to
// share one SampleInput so their SampleOutputs can be compared. It is a
// convention, not a constraint — the API accepts a row with no example at all —
// which is exactly why it belongs in the standard rather than in validation.
type StandardExample struct {
	Field        string `json:"field"`
	Convention   string `json:"convention"`
	WhyItMatters string `json:"whyItMatters"`
}

// The published vocabularies. Descriptions are the guidance an operator reads
// while filing; keep them short enough to render as one line in a picker.
var standardCategories = []StandardOption{
	{Value: CategoryLiterary, Label: "文学", LabelEn: "Literary", Desc: "小说、散文、随笔等以感受与叙事为主的写作"},
	{Value: CategoryBusiness, Label: "商务", LabelEn: "Business", Desc: "方案、汇报、邮件等以推进事情为目的的写作"},
	{Value: CategoryAcademic, Label: "学术", LabelEn: "Academic", Desc: "论文、综述、研究报告等要求可核查的写作"},
	{Value: CategoryMedia, Label: "媒体", LabelEn: "Media", Desc: "新闻、特稿、评论等面向公众发布的写作"},
	{Value: CategorySpoken, Label: "口播", LabelEn: "Spoken", Desc: "口播稿、播客、讲稿等要念出来的写作"},
	{Value: CategoryTechnical, Label: "技术", LabelEn: "Technical", Desc: "文档、教程、说明等以准确为先的写作"},
	// ⚠ The Chinese label is "市场营销", not the shorter "营销", because the
	// admin UI renders this category through the host's shared locale key
	// (`Marketing` in web/src/i18n/locales/*.json), which already says
	// "市场营销". Publishing a second, slightly different spelling here would be
	// exactly the two-copies drift this standard exists to prevent — the two must
	// read the same to someone comparing the API response with the screen.
	{Value: CategoryMarketing, Label: "市场营销", LabelEn: "Marketing", Desc: "文案、种草、品牌故事等以转化为目的的写作"},
}

var standardTones = []StandardOption{
	{Value: ToneWarm, Label: "温暖", LabelEn: "Warm", Desc: "有体温、有体谅，先接住情绪再讲事情"},
	{Value: ToneCalm, Label: "冷静", LabelEn: "Calm", Desc: "降速、去形容词，让事实自己说话"},
	{Value: ToneSharp, Label: "犀利", LabelEn: "Sharp", Desc: "短句、直给，敢下判断，不为冒犯道歉"},
	{Value: ToneHumorous, Label: "幽默", LabelEn: "Humorous", Desc: "靠反差与自嘲取胜，不靠谐音梗"},
	{Value: ToneSolemn, Label: "庄重", LabelEn: "Solemn", Desc: "有分量、少口语，适合需要被记住的场合"},
	{Value: ToneLively, Label: "活泼", LabelEn: "Lively", Desc: "节奏快、有动作感，像在跟人当面聊天"},
	{Value: TonePlain, Label: "平实", LabelEn: "Plain", Desc: "把话说清楚就走，不修饰、不表演"},
}

// standardCompatibility is the promise, restated for callers (see the file doc
// for the reasoning behind each rule).
var standardCompatibility = []string{
	"已发布的取值不会改名、不会删除",
	"可以新增取值，新增不属于破坏性变更",
	"空值始终表示「未分类 / 未指定」",
	"客户端必须容忍不认识的取值：照原样显示，不要丢弃该条目",
	"版本号只在形状变化时前进（新增字段或调整上限），新增取值不前进",
}

// ToneStandardOf returns the published standard. It hands out the declared
// slices directly rather than a copy: they are package-level read-only data with
// no writer, and the response is serialized immediately by its only caller.
func ToneStandardOf() ToneStandard {
	return ToneStandard{
		Version:       ToneStandardVersion,
		Categories:    standardCategories,
		Tones:         standardTones,
		Limits:        standardLimitsOf(),
		Compatibility: standardCompatibility,
		ExamplePair: StandardExample{
			Field: "sampleInput / sampleOutput",
			Convention: "同一次维护里，多条文风共用同一段 sampleInput，" +
				"各自的 sampleOutput 是它按本文风改写后的样子",
			WhyItMatters: "文风既不能像音色那样试听、也不能像形象那样看脸；" +
				"共用同一段原文，横向比较才成立，而这份比较是存下来的、不花钱",
		},
	}
}

// standardLimitsOf mirrors the constants the validators enforce. It is written
// as an explicit mapping rather than reflection so a renamed constant fails to
// compile here instead of quietly publishing a wrong number.
func standardLimitsOf() StandardLimits {
	return StandardLimits{
		Name:         maxNameLen,
		Description:  maxDescriptionLen,
		Prompt:       maxPromptLen,
		Sample:       maxSampleLen,
		Language:     maxLanguageLen,
		Scenes:       maxScenes,
		SceneLength:  maxSceneLen,
		SortOrderAbs: maxSortOrderAbs,
		MaxPageSize:  maxPageSize,
	}
}
