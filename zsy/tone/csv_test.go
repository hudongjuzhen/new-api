package tone_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/tone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CSV round trip: what an export writes, an import must read back. This is also
// the authoring path for the 文风标准 (the seeded catalog ships as a CSV), so the
// header aliases and the required-column rule are contract, not convenience.
// ---------------------------------------------------------------------------

func TestWriteTonesCSV_RoundTrips(t *testing.T) {
	newToneTestDB(t)
	seeded := insertRichTone(t)

	rows, err := tone.ToneExportRows(tone.ToneListQuery{})
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, tone.WriteTonesCSV(&buf, rows))

	text := buf.String()
	assert.True(t, strings.HasPrefix(text, "\ufeff"), "the BOM is what makes Excel read Chinese correctly")
	assert.Contains(t, text, "name,description,prompt,category,tone,language,scenes,sample_input,sample_output,enabled,sort_order")

	parsed, _, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err)
	require.Len(t, parsed, 1)

	got := parsed[0].Create
	assert.NoError(t, parsed[0].Err)
	assert.Equal(t, seeded.Name, got.Name)
	assert.Equal(t, seeded.Description, got.Description)
	assert.Equal(t, seeded.Prompt, got.Prompt, "the instruction must survive the round trip byte for byte")
	assert.Equal(t, seeded.Category, got.Category)
	assert.Equal(t, seeded.Tone, got.Tone)
	assert.Equal(t, seeded.Language, got.Language)
	assert.Equal(t, seeded.Scenes, []string(got.Scenes))
	assert.Equal(t, seeded.SampleInput, got.SampleInput)
	assert.Equal(t, seeded.SampleOutput, got.SampleOutput)
	require.NotNil(t, got.Enabled)
	assert.True(t, *got.Enabled)
	require.NotNil(t, got.SortOrder)
	assert.Equal(t, seeded.SortOrder, *got.SortOrder)
}

// TestParseTonesCSV_AcceptsChineseHeaders covers the hand-filled spreadsheet: an
// operator writes 文风名称 / 提示词 / 语气, and the import must not make them
// rename columns to the canonical English ones.
func TestParseTonesCSV_AcceptsChineseHeaders(t *testing.T) {
	text := "\ufeff文风名称,简介,提示词,类别,语气,语言,适合场景,示例原文,示例改写,是否上架,排序\n" +
		"克制的长文,适合公众号,以克制的笔调写作,literary,calm,zh,\"公众号长文,散文\"," +
		"老屋在东头。,老屋，在东头。东头。那间老屋。,是,3\n"

	rows, warnings, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, rows, 1)

	got := rows[0].Create
	assert.Equal(t, "克制的长文", got.Name)
	assert.Equal(t, "以克制的笔调写作", got.Prompt)
	assert.Equal(t, "literary", got.Category)
	assert.Equal(t, "calm", got.Tone)
	assert.Equal(t, "zh", got.Language)
	assert.Equal(t, []string{"公众号长文", "散文"}, []string(got.Scenes))
	require.NotNil(t, got.Enabled)
	assert.True(t, *got.Enabled)
	require.NotNil(t, got.SortOrder)
	assert.Equal(t, 3, *got.SortOrder)
}

func TestParseTonesCSV_HeaderSpellingIsForgiving(t *testing.T) {
	// Spaces, dashes, case and a leading BOM all normalize to the same column.
	// Without this a file touched by Excel or typed by hand would fail to import
	// for reasons the operator cannot see in the cells.
	text := "\ufeff Name ,Sample-Input,SAMPLE OUTPUT,SORT ORDER,Prompt\n甲,原文,改写,4,写短句\n"

	rows, warnings, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err)
	assert.Empty(t, warnings, "every header in this file normalizes onto a known column")
	require.Len(t, rows, 1)

	got := rows[0].Create
	assert.Equal(t, "甲", got.Name)
	assert.Equal(t, "写短句", got.Prompt)
	assert.Equal(t, "原文", got.SampleInput)
	assert.Equal(t, "改写", got.SampleOutput)
	require.NotNil(t, got.SortOrder)
	assert.Equal(t, 4, *got.SortOrder)
}

// TestParseTonesCSV_AMisnamedNameColumnIsAStructuralError pins the other side of
// the forgiving-header rule: forgiving about spelling, strict about presence. An
// unrecognized "name" spelling must fail the file, not import rows with no name
// (which would then collide on the unique index or land as blank entries).
func TestParseTonesCSV_AMisnamedNameColumnIsAStructuralError(t *testing.T) {
	text := "Tone Name,Prompt\n甲,写短句\n"

	_, _, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name（文风名称）")
}

func TestParseTonesCSV_RequiresNameAndPrompt(t *testing.T) {
	cases := []struct {
		name    string
		text    string
		wantErr string
	}{
		{
			name:    "missing prompt column",
			text:    "name,description\n甲,说明\n",
			wantErr: "prompt（文风提示词）",
		},
		{
			name:    "missing name column",
			text:    "prompt,description\n写短句,说明\n",
			wantErr: "name（文风名称）",
		},
		{
			name:    "empty file",
			text:    "",
			wantErr: "CSV 文件为空",
		},
		{
			name:    "header but no data rows",
			text:    "name,prompt\n\n",
			wantErr: "没有可导入的数据行",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tone.ParseTonesCSV(strings.NewReader(tc.text))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestParseTonesCSV_UnknownColumnsBecomeAWarning(t *testing.T) {
	text := "name,prompt,备注,内部编号\n甲,写短句,随手记的,z-1\n"

	rows, warnings, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "无法识别的列")
	assert.Contains(t, warnings[0], "备注")
	assert.Contains(t, warnings[0], "内部编号")
}

func TestParseTonesCSV_UnreadableCellsFailTheRowNotTheFile(t *testing.T) {
	text := "name,prompt,enabled,sort_order\n" +
		"好的,写短句,是,1\n" +
		"坏的行,写短句,也许,2\n"

	rows, _, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err, "one unreadable cell must not cost the operator the whole file")
	require.Len(t, rows, 2)

	assert.NoError(t, rows[0].Err)
	require.Error(t, rows[1].Err)
	assert.Contains(t, rows[1].Err.Error(), "无法识别的上架状态")
	assert.Equal(t, 3, rows[1].Row, "the record number counts the header as row 1")
}

// ---------------------------------------------------------------------------
// The worked-example convention (see standard.go). It is a warning and never an
// error: a standard that rejected non-conforming files would just teach people
// to leave the column empty.
// ---------------------------------------------------------------------------

func TestParseTonesCSV_WarnsWhenSampleInputsDiverge(t *testing.T) {
	text := "name,prompt,sample_input,sample_output\n" +
		"甲,写短句,老屋在东头。,老屋，东头。\n" +
		"乙,写长句,春天来了。,春天，就这么来了。\n"

	_, warnings, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err, "divergent examples are legitimate, so this must not fail the import")
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "示例原文")
	assert.Contains(t, warnings[0], "共用")
}

func TestParseTonesCSV_NoWarningWhenSampleInputIsShared(t *testing.T) {
	text := "name,prompt,sample_input,sample_output\n" +
		"甲,写短句,老屋在东头。,老屋，东头。\n" +
		"乙,写长句,老屋在东头。,那间老屋，就在村子的东头。\n"

	_, warnings, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err)
	assert.Empty(t, warnings, "a file following the convention must import silently")
}

func TestParseTonesCSV_WarnsWhenSamplesAreMissing(t *testing.T) {
	text := "name,prompt,sample_input,sample_output\n" +
		"甲,写短句,老屋在东头。,老屋，东头。\n" +
		"乙,写长句,,\n"

	_, warnings, err := tone.ParseTonesCSV(strings.NewReader(text))
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "没有填示例原文")
}

// ---------------------------------------------------------------------------
// Import modes
// ---------------------------------------------------------------------------

func TestImportTones_UpsertUpdatesAnExistingName(t *testing.T) {
	newToneTestDB(t)
	seeded := insertTone(t, "会被覆盖", "旧提示词", 0, true)

	rows, _, err := tone.ParseTonesCSV(strings.NewReader(
		"name,prompt\n会被覆盖,新提示词\n"))
	require.NoError(t, err)

	result, err := tone.ImportTones(rows, tone.ImportModeUpsert, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.Updated)

	view, err := tone.ToneGetByID(seeded.ID)
	require.NoError(t, err)
	assert.Equal(t, "新提示词", view.Prompt)
}

// TestImportTones_UpsertDoesNotBlankOmittedColumns is the reason the update shape
// carries only the columns the file actually had: a two-column file fixing a
// typo in the prompt must not wipe every other field of the row.
func TestImportTones_UpsertDoesNotBlankOmittedColumns(t *testing.T) {
	newToneTestDB(t)
	seeded := insertRichTone(t)

	rows, _, err := tone.ParseTonesCSV(strings.NewReader(
		"name,prompt\n" + seeded.Name + ",改过的提示词\n"))
	require.NoError(t, err)

	result, err := tone.ImportTones(rows, tone.ImportModeUpsert, nil)
	require.NoError(t, err)
	require.Equal(t, 1, result.Updated)

	view, err := tone.ToneGetByID(seeded.ID)
	require.NoError(t, err)
	assert.Equal(t, "改过的提示词", view.Prompt)
	assert.Equal(t, seeded.Category, view.Category, "a column the file omitted must keep its stored value")
	assert.Equal(t, seeded.SampleOutput, view.SampleOutput)
	assert.Equal(t, seeded.Scenes, view.Scenes)
}

func TestImportTones_CreateModeRefusesAnExistingName(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "已经在了", "旧提示词", 0, true)

	rows, _, err := tone.ParseTonesCSV(strings.NewReader(
		"name,prompt\n已经在了,新提示词\n新来的,提示词\n"))
	require.NoError(t, err)

	result, err := tone.ImportTones(rows, tone.ImportModeCreate, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 0, result.Updated)
	assert.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Message, "已存在")
}

func TestImportTones_RejectsAnUnknownMode(t *testing.T) {
	newToneTestDB(t)

	_, err := tone.ImportTones(nil, "replace-everything", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "非法导入模式")
}

func TestImportTones_ReportsPerRowValidationFailures(t *testing.T) {
	newToneTestDB(t)

	rows, _, err := tone.ParseTonesCSV(strings.NewReader(
		"name,prompt,category\n" +
			"合法的,写短句,literary\n" +
			"坏类别,写短句,poetry\n" +
			"没提示词,,literary\n"))
	require.NoError(t, err)

	result, err := tone.ImportTones(rows, tone.ImportModeUpsert, nil)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 2, result.Failed)
	require.Len(t, result.Errors, 2)
	assert.Contains(t, result.Errors[0].Message, "非法类别")
	assert.Contains(t, result.Errors[1].Message, "文风提示词不能为空")
}
