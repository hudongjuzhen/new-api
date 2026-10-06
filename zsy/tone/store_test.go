package tone_test

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/tone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Store layer: validation, pagination contract, filters, update/delete
// semantics. These are the rules the admin UI and third-party callers both
// depend on.
// ---------------------------------------------------------------------------

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

func TestToneInsert_RejectsInvalidFields(t *testing.T) {
	newToneTestDB(t)

	// Every accepted case inserts a row, so each case gets its own name: the
	// unique index on name would otherwise decide the outcome instead of the
	// rule under test.
	valid := func(name string) *tone.ToneCreateDTO {
		return &tone.ToneCreateDTO{
			Name:        name,
			Prompt:      "以克制的笔调写作",
			Description: "适合长文",
		}
	}

	cases := []struct {
		name    string
		mutate  func(dto *tone.ToneCreateDTO)
		wantErr string
	}{
		{
			name:    "empty name",
			mutate:  func(d *tone.ToneCreateDTO) { d.Name = "   " },
			wantErr: "文风名称不能为空",
		},
		{
			name:    "name longer than the indexed column",
			mutate:  func(d *tone.ToneCreateDTO) { d.Name = strings.Repeat("风", 192) },
			wantErr: "文风名称过长",
		},
		{
			name:    "empty prompt",
			mutate:  func(d *tone.ToneCreateDTO) { d.Prompt = "  " },
			wantErr: "文风提示词不能为空",
		},
		{
			name:    "prompt longer than the cap",
			mutate:  func(d *tone.ToneCreateDTO) { d.Prompt = strings.Repeat("字", 8001) },
			wantErr: "文风提示词过长",
		},
		{
			name:    "description longer than the cap",
			mutate:  func(d *tone.ToneCreateDTO) { d.Description = strings.Repeat("a", 2001) },
			wantErr: "简介过长",
		},
		{
			name:    "sample input longer than the cap",
			mutate:  func(d *tone.ToneCreateDTO) { d.SampleInput = strings.Repeat("样", 4001) },
			wantErr: "示例原文过长",
		},
		{
			name:    "sample output longer than the cap",
			mutate:  func(d *tone.ToneCreateDTO) { d.SampleOutput = strings.Repeat("样", 4001) },
			wantErr: "示例改写过长",
		},
		{
			name:    "unknown category",
			mutate:  func(d *tone.ToneCreateDTO) { d.Category = "poetry" },
			wantErr: "非法类别",
		},
		{
			name:    "unknown tone",
			mutate:  func(d *tone.ToneCreateDTO) { d.Tone = "sassy" },
			wantErr: "非法语气",
		},
		{
			name:    "language code starting with a digit",
			mutate:  func(d *tone.ToneCreateDTO) { d.Language = "1zh" },
			wantErr: "非法语言代码",
		},
		{
			name:    "language code with a space",
			mutate:  func(d *tone.ToneCreateDTO) { d.Language = "pt br" },
			wantErr: "非法语言代码",
		},
		{
			name:    "sort order out of range",
			mutate:  func(d *tone.ToneCreateDTO) { d.SortOrder = intPtr(1_000_001) },
			wantErr: "排序值超出范围",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dto := valid("用例-" + tc.name)
			tc.mutate(dto)
			_, err := tone.ToneInsert(dto)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

// TestToneInsert_AcceptsEmptyOptionalFields pins that the optional half really
// is optional: a tone with nothing but a name and its instruction is valid, and
// every unset vocabulary lands as an empty string (not as a made-up default the
// filter row would then have to special-case).
func TestToneInsert_AcceptsEmptyOptionalFields(t *testing.T) {
	newToneTestDB(t)

	view, err := tone.ToneInsert(&tone.ToneCreateDTO{Name: "只有提示词", Prompt: "写短句"})
	require.NoError(t, err)

	assert.Equal(t, "", view.Category)
	assert.Equal(t, "", view.Tone)
	assert.Equal(t, "", view.Language)
	assert.Equal(t, []string{}, view.Scenes, "scenes is always an array so a caller can iterate without a nil check")
	assert.Equal(t, "", view.SampleInput)
	assert.Equal(t, "", view.SampleOutput)
	assert.True(t, view.Enabled, "a new tone is on the shelf unless the caller says otherwise")
	assert.Equal(t, 0, view.SortOrder)
}

func TestToneInsert_RejectsDuplicateName(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "重复的文风", "p", 0, true)

	_, err := tone.ToneInsert(&tone.ToneCreateDTO{Name: "重复的文风", Prompt: "另一段"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "已存在")
}

// TestToneInsert_NormalizesVocabularyCase covers the spreadsheet path: an
// operator typing "Warm" must land on the same stored value as the API's "warm",
// otherwise the filter row silently stops matching that row.
func TestToneInsert_NormalizesVocabularyCase(t *testing.T) {
	newToneTestDB(t)

	view, err := tone.ToneInsert(&tone.ToneCreateDTO{
		Name:     "大小写",
		Prompt:   "p",
		Category: " Literary ",
		Tone:     "WARM",
		Language: "ZH",
	})
	require.NoError(t, err)

	assert.Equal(t, "literary", view.Category)
	assert.Equal(t, "warm", view.Tone)
	assert.Equal(t, "zh", view.Language)

	// And the normalized value is what a filter actually matches on.
	result, err := tone.ToneSearch(tone.ToneListQuery{Tone: "warm"})
	require.NoError(t, err)
	assert.EqualValues(t, 1, result.Total)
}

func TestToneUpdate_PartialKeepsOmittedFields(t *testing.T) {
	newToneTestDB(t)
	seeded := insertRichTone(t)

	view, err := tone.ToneUpdate(seeded.ID, &tone.ToneUpdateDTO{Tone: strPtr("sharp")})
	require.NoError(t, err)

	assert.Equal(t, "sharp", view.Tone)
	assert.Equal(t, seeded.Prompt, view.Prompt, "an omitted field keeps its stored value")
	assert.Equal(t, seeded.SampleInput, view.SampleInput)
	assert.Equal(t, seeded.Scenes, view.Scenes)
}

// TestToneUpdate_ExplicitEmptyClears covers the other half of the partial-update
// contract: an empty string is a clear, not an omission, so a tone can be
// detached from its worked example without being recreated.
func TestToneUpdate_ExplicitEmptyClears(t *testing.T) {
	newToneTestDB(t)
	seeded := insertRichTone(t)

	view, err := tone.ToneUpdate(seeded.ID, &tone.ToneUpdateDTO{
		SampleInput:  strPtr(""),
		SampleOutput: strPtr(""),
		Category:     strPtr(""),
	})
	require.NoError(t, err)

	assert.Equal(t, "", view.SampleInput)
	assert.Equal(t, "", view.SampleOutput)
	assert.Equal(t, "", view.Category)
	assert.Equal(t, seeded.Tone, view.Tone, "clearing one field must not clear its neighbours")
}

func TestToneUpdate_RejectsClearingThePrompt(t *testing.T) {
	newToneTestDB(t)
	seeded := insertRichTone(t)

	_, err := tone.ToneUpdate(seeded.ID, &tone.ToneUpdateDTO{Prompt: strPtr("")})
	require.Error(t, err, "a tone cannot be emptied of its instruction")
	assert.Contains(t, err.Error(), "文风提示词不能为空")
}

func TestToneUpdate_NotFound(t *testing.T) {
	newToneTestDB(t)

	_, err := tone.ToneUpdate(9999, &tone.ToneUpdateDTO{Tone: strPtr("warm")})
	assert.ErrorIs(t, err, tone.ErrToneNotFound)
}

func TestToneDelete_ReturnsNameAndRemovesRow(t *testing.T) {
	newToneTestDB(t)
	seeded := insertTone(t, "待删除", "p", 0, true)

	name, err := tone.ToneDelete(seeded.ID)
	require.NoError(t, err)
	assert.Equal(t, "待删除", name)

	_, err = tone.ToneGetByID(seeded.ID)
	assert.ErrorIs(t, err, tone.ErrToneNotFound)

	// Hard delete: the name is free again immediately (a soft delete would keep
	// occupying the unique index and the re-create below would fail).
	_, err = tone.ToneInsert(&tone.ToneCreateDTO{Name: "待删除", Prompt: "p2"})
	assert.NoError(t, err)
}

func TestToneDelete_NotFound(t *testing.T) {
	newToneTestDB(t)

	_, err := tone.ToneDelete(9999)
	assert.ErrorIs(t, err, tone.ErrToneNotFound)
}

// TestToneSearch_PaginationContract pins the documented clamping: bad input is
// normalized rather than rejected, and paging is stable under a shared
// sort_order.
func TestToneSearch_PaginationContract(t *testing.T) {
	newToneTestDB(t)
	// All three share sort_order 0, so ordering must fall back to id ascending.
	insertTone(t, "甲", "p", 0, true)
	insertTone(t, "乙", "p", 0, true)
	insertTone(t, "丙", "p", 0, true)

	t.Run("page below one is clamped", func(t *testing.T) {
		result, err := tone.ToneSearch(tone.ToneListQuery{Page: 0, PageSize: 2})
		require.NoError(t, err)
		assert.Equal(t, 1, result.Page)
		assert.Equal(t, []string{"甲", "乙"}, viewNames(result.Items))
	})

	t.Run("page size above the cap is clamped", func(t *testing.T) {
		result, err := tone.ToneSearch(tone.ToneListQuery{PageSize: 500})
		require.NoError(t, err)
		assert.Equal(t, 100, result.PageSize)
	})

	t.Run("page size below one falls back to the default", func(t *testing.T) {
		result, err := tone.ToneSearch(tone.ToneListQuery{PageSize: 0})
		require.NoError(t, err)
		assert.Equal(t, 20, result.PageSize)
	})

	t.Run("total pages rounds up", func(t *testing.T) {
		result, err := tone.ToneSearch(tone.ToneListQuery{PageSize: 2})
		require.NoError(t, err)
		assert.EqualValues(t, 3, result.Total)
		assert.Equal(t, 2, result.TotalPages)

		second, err := tone.ToneSearch(tone.ToneListQuery{Page: 2, PageSize: 2})
		require.NoError(t, err)
		assert.Equal(t, []string{"丙"}, viewNames(second.Items))
	})
}

// TestToneSearch_SortOrderWinsOverID pins that an operator's explicit ordering
// is honoured: without it the plaza would ignore the sort column entirely and
// every row would read as an insertion-order list.
func TestToneSearch_SortOrderWinsOverID(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "后来的", "p", 1, true)
	insertTone(t, "先排的", "p", 0, true)

	result, err := tone.ToneSearch(tone.ToneListQuery{})
	require.NoError(t, err)
	assert.Equal(t, []string{"先排的", "后来的"}, viewNames(result.Items))
}

func TestToneSearch_Filters(t *testing.T) {
	newToneTestDB(t)
	rich := insertRichTone(t)            // literary / calm / zh / 公众号长文,散文
	insertTone(t, "犀利的短评", "p", 2, true) // no facets
	insertTone(t, "下架的", "p", 3, false)  // off shelf

	cases := []struct {
		name  string
		query tone.ToneListQuery
		want  []string
	}{
		{
			name:  "no filter returns everything",
			query: tone.ToneListQuery{},
			want:  []string{rich.Name, "犀利的短评", "下架的"},
		},
		{
			name:  "enabled true",
			query: tone.ToneListQuery{Enabled: boolPtr(true)},
			want:  []string{rich.Name, "犀利的短评"},
		},
		{
			name:  "enabled false",
			query: tone.ToneListQuery{Enabled: boolPtr(false)},
			want:  []string{"下架的"},
		},
		{
			name:  "category",
			query: tone.ToneListQuery{Category: "literary"},
			want:  []string{rich.Name},
		},
		{
			name:  "tone",
			query: tone.ToneListQuery{Tone: "calm"},
			want:  []string{rich.Name},
		},
		{
			name:  "language",
			query: tone.ToneListQuery{Language: "zh"},
			want:  []string{rich.Name},
		},
		{
			name:  "keyword matches the name",
			query: tone.ToneListQuery{Keyword: "犀利"},
			want:  []string{"犀利的短评"},
		},
		{
			name:  "keyword matches a scene tag",
			query: tone.ToneListQuery{Keyword: "散文"},
			want:  []string{rich.Name},
		},
		{
			name:  "combined filters intersect",
			query: tone.ToneListQuery{Category: "literary", Tone: "sharp"},
			want:  []string{},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tone.ToneSearch(tc.query)
			require.NoError(t, err)
			assert.Equal(t, tc.want, viewNames(result.Items))
		})
	}
}

// TestToneSearch_KeywordReachesPromptAndSample is the deliberate difference from
// zsy/voice, where the keyword only covers the naming fields. An operator hunting
// for "that restrained one" remembers a phrase from the instruction or the worked
// example — searching names alone makes the widest columns in the table the only
// ones you cannot search by.
func TestToneSearch_KeywordReachesPromptAndSample(t *testing.T) {
	newToneTestDB(t)
	insertRichTone(t)

	t.Run("a phrase from the prompt", func(t *testing.T) {
		result, err := tone.ToneSearch(tone.ToneListQuery{Keyword: "少用形容词"})
		require.NoError(t, err)
		assert.Len(t, result.Items, 1)
	})

	t.Run("a phrase only in the sample output", func(t *testing.T) {
		result, err := tone.ToneSearch(tone.ToneListQuery{Keyword: "凹痕"})
		require.NoError(t, err)
		assert.Len(t, result.Items, 1)
	})

	t.Run("a phrase only in the sample input", func(t *testing.T) {
		result, err := tone.ToneSearch(tone.ToneListQuery{Keyword: "枣树"})
		require.NoError(t, err)
		assert.Len(t, result.Items, 1, "the shared sample passage must be searchable too")
	})
}

func TestToneExportRows_IgnoresPagination(t *testing.T) {
	newToneTestDB(t)
	for i := 0; i < 25; i++ {
		insertTone(t, "批量-"+string(rune('A'+i)), "p", i, true)
	}

	rows, err := tone.ToneExportRows(tone.ToneListQuery{Page: 1, PageSize: 5})
	require.NoError(t, err)
	assert.Len(t, rows, 25, "an export is the whole selection, not one page of it")
}

func TestToneGetByID_NotFound(t *testing.T) {
	newToneTestDB(t)

	_, err := tone.ToneGetByID(9999)
	assert.ErrorIs(t, err, tone.ErrToneNotFound)
}

// viewNames lists the names of a result page in order.
func viewNames(items []*tone.ToneView) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name)
	}
	return out
}
