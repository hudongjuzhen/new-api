package tone_test

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/tone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// The 文风标准 (Tone Standard) — what it publishes, and the guard that keeps the
// published copy honest.
//
// The whole reason the vocabulary ships as an endpoint instead of living only in
// Go constants is that a client must be able to read it rather than retype it.
// That only holds if the endpoint and the validators agree, so the two most
// important tests here assert agreement **through behaviour**: they push a value
// through the real validator rather than comparing two constants, because two
// constants compared to each other would still agree after both drifted from the
// rule they were supposed to encode.
// ---------------------------------------------------------------------------

func TestToneStandard_PublishesBothVocabularies(t *testing.T) {
	std := tone.ToneStandardOf()

	assert.Equal(t, tone.ToneStandardVersion, std.Version)
	assert.Regexp(t, regexp.MustCompile(`^\d+\.\d+\.\d+$`), std.Version,
		"the version is part of the published contract; keep it semver")

	assert.Equal(t, tone.AllowedCategories, optionValues(std.Categories))
	assert.Equal(t, tone.AllowedTones, optionValues(std.Tones))

	// Every option needs all three labels: a value with no Chinese label shows up
	// in the admin UI as a raw identifier, and one with no description gives the
	// operator nothing to choose by.
	for _, option := range append(append([]tone.StandardOption{}, std.Categories...), std.Tones...) {
		assert.NotEmpty(t, option.Value, "an option is missing its wire value")
		assert.NotEmpty(t, option.Label, "%s is missing its Chinese label", option.Value)
		assert.NotEmpty(t, option.LabelEn, "%s is missing its English label", option.Value)
		assert.NotEmpty(t, option.Desc, "%s is missing its one-line description", option.Value)
	}
}

func TestToneStandard_ValuesAreLowercaseWireIdentifiers(t *testing.T) {
	std := tone.ToneStandardOf()
	pattern := regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

	for _, option := range append(append([]tone.StandardOption{}, std.Categories...), std.Tones...) {
		assert.Regexp(t, pattern, option.Value,
			"%s is not a lowercase wire identifier; values are the identity clients branch on", option.Value)
	}
}

// TestToneStandard_LimitsMatchTheValidator is the guard that matters.
//
// It never reads maxPromptLen or any other constant: it asks the endpoint what
// the cap is, then sends exactly that value and one character more through the
// production create path. If someone raises the constant without republishing
// the standard, the "at the cap" case starts failing; if someone lowers it
// without republishing, the "one over" case starts failing. Either way the
// drift is caught here instead of by a client that validated against a stale
// number.
func TestToneStandard_LimitsMatchTheValidator(t *testing.T) {
	newToneTestDB(t)
	limits := tone.ToneStandardOf().Limits

	t.Run("name", func(t *testing.T) {
		atCap := &tone.ToneCreateDTO{Name: strings.Repeat("名", limits.Name), Prompt: "p"}
		_, err := tone.ToneInsert(atCap)
		require.NoError(t, err, "a name exactly at the published cap must be accepted")

		overCap := &tone.ToneCreateDTO{Name: strings.Repeat("名", limits.Name+1), Prompt: "p"}
		_, err = tone.ToneInsert(overCap)
		require.Error(t, err, "a name one character over the published cap must be rejected")
		assert.Contains(t, err.Error(), "文风名称过长")
	})

	t.Run("prompt", func(t *testing.T) {
		atCap := &tone.ToneCreateDTO{Name: "上限内的提示词", Prompt: strings.Repeat("字", limits.Prompt)}
		_, err := tone.ToneInsert(atCap)
		require.NoError(t, err, "a prompt exactly at the published cap must be accepted")

		overCap := &tone.ToneCreateDTO{Name: "超限的提示词", Prompt: strings.Repeat("字", limits.Prompt+1)}
		_, err = tone.ToneInsert(overCap)
		require.Error(t, err, "a prompt one character over the published cap must be rejected")
		assert.Contains(t, err.Error(), "文风提示词过长")
	})

	t.Run("sample", func(t *testing.T) {
		overCap := &tone.ToneCreateDTO{
			Name:         "超限的示例",
			Prompt:       "p",
			SampleOutput: strings.Repeat("字", limits.Sample+1),
		}
		_, err := tone.ToneInsert(overCap)
		require.Error(t, err, "a sample one character over the published cap must be rejected")
		assert.Contains(t, err.Error(), "示例改写过长")
	})

	t.Run("scenes", func(t *testing.T) {
		// ⚠ The names must differ: scenesToCSV de-duplicates, so nine copies of
		// "场景" normalize down to one and the row would be accepted — the test
		// would fail while the rule under test was working correctly.
		scenes := make([]string, 0, limits.Scenes+1)
		for i := 0; i <= limits.Scenes; i++ {
			scenes = append(scenes, fmt.Sprintf("场景%d", i))
		}
		_, err := tone.ToneInsert(&tone.ToneCreateDTO{Name: "场景过多的文风", Prompt: "p", Scenes: scenes})
		require.Error(t, err, "more scenes than the published cap must be rejected")
		assert.Contains(t, err.Error(), "适合场景最多")
	})

	t.Run("scene length", func(t *testing.T) {
		_, err := tone.ToneInsert(&tone.ToneCreateDTO{
			Name:   "场景过长的文风",
			Prompt: "p",
			Scenes: []string{strings.Repeat("场", limits.SceneLength+1)},
		})
		require.Error(t, err, "a scene longer than the published cap must be rejected")
		assert.Contains(t, err.Error(), "单个场景过长")
	})

	t.Run("sort order", func(t *testing.T) {
		over := limits.SortOrderAbs + 1
		_, err := tone.ToneInsert(&tone.ToneCreateDTO{Name: "排序越界", Prompt: "p", SortOrder: &over})
		require.Error(t, err, "a sort order outside the published range must be rejected")
		assert.Contains(t, err.Error(), "排序值超出范围")
	})

	t.Run("max page size", func(t *testing.T) {
		insertTone(t, "分页用", "p", 0, true)
		result, err := tone.ToneSearch(tone.ToneListQuery{PageSize: limits.MaxPageSize + 50})
		require.NoError(t, err)
		assert.Equal(t, limits.MaxPageSize, result.PageSize,
			"the list must clamp to the page size the standard publishes")
	})
}

// TestToneStandard_CompatibilityRulesArePublished pins the promise itself. These
// strings are the standard's contract with client authors (see standard.go), and
// an empty list would silently turn "we promise not to rename values" into
// nothing at all — the one field a client cannot infer from the data.
func TestToneStandard_CompatibilityRulesArePublished(t *testing.T) {
	std := tone.ToneStandardOf()

	require.NotEmpty(t, std.Compatibility)
	assert.Len(t, std.Compatibility, 5)
	joined := strings.Join(std.Compatibility, "\n")
	assert.Contains(t, joined, "不会改名", "the rename guarantee must be stated")
	assert.Contains(t, joined, "容忍不认识的取值", "the unknown-value rule must be stated")
}

// TestToneStandard_ExamplePairIsDocumented checks the worked-example convention
// travels with the standard. It is a convention rather than a constraint, so the
// only thing to assert is that it is stated and says what it is for.
func TestToneStandard_ExamplePairIsDocumented(t *testing.T) {
	example := tone.ToneStandardOf().ExamplePair

	assert.Contains(t, example.Field, "sampleInput")
	assert.Contains(t, example.Field, "sampleOutput")
	assert.NotEmpty(t, example.Convention)
	assert.NotEmpty(t, example.WhyItMatters)
	assert.Contains(t, example.Convention, "共用",
		"the comparable-examples rule is the whole point of the convention")
}

// optionValues projects the wire values, keeping order (the published order is
// also the order the admin filter row shows).
func optionValues(options []tone.StandardOption) []string {
	out := make([]string, 0, len(options))
	for _, option := range options {
		out = append(out, option.Value)
	}
	return out
}
