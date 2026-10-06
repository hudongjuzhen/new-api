package avatar_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/avatar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

func TestWriteAvatarsCSV_WritesTheCanonicalHeaderAndRows(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t)

	rows, err := avatar.AvatarExportRows(avatar.AvatarListQuery{})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	require.NoError(t, avatar.WriteAvatarsCSV(buf, rows))

	out := buf.String()
	assert.True(t, strings.HasPrefix(out, "\ufeff"), "Excel needs the BOM to read Chinese")
	lines := strings.Split(strings.TrimPrefix(out, "\ufeff"), "\n")
	assert.Equal(t,
		"name,description,image_url,full_body_url,four_view_url,expression_url,"+
			"gender,age_range,race,scenes,voice_id,voice_sample,enabled,sort_order",
		strings.TrimSpace(lines[0]))
	assert.Contains(t, lines[1], "客服小雨")
	assert.Contains(t, lines[1], "zh_female_vv_uranus_bigtts")
	assert.Contains(t, lines[1], "asian")
	// All four pictures are exported, not only the cover.
	assert.Contains(t, lines[1], "https://cdn.example.com/persona/xiaoyu.png")
	assert.Contains(t, lines[1], "https://cdn.example.com/persona/xiaoyu-full.png")
	assert.Contains(t, lines[1], "https://cdn.example.com/persona/xiaoyu-four-view.png")
	assert.Contains(t, lines[1], "https://cdn.example.com/persona/xiaoyu-expression.png")
	// The scenes list is quoted because it carries a comma.
	assert.Contains(t, lines[1], `"客服播报,有声书"`)
}

func TestWriteAvatarsCSV_ExportsTheHeaderAloneForAnEmptyCatalog(t *testing.T) {
	newAvatarTestDB(t)

	rows, err := avatar.AvatarExportRows(avatar.AvatarListQuery{})
	require.NoError(t, err)

	buf := &bytes.Buffer{}
	require.NoError(t, avatar.WriteAvatarsCSV(buf, rows))

	lines := strings.Split(strings.TrimSpace(strings.TrimPrefix(buf.String(), "\ufeff")), "\n")
	require.Len(t, lines, 1, "an empty catalog doubles as the import template")
	assert.Contains(t, lines[0], "voice_sample")
}

// ---------------------------------------------------------------------------
// Import
// ---------------------------------------------------------------------------

func TestParseAvatarsCSV_AcceptsTheChineseHeaderSpellings(t *testing.T) {
	csv := "形象名称,简介,封面图,全身照,四视图,表情图,性别,年龄段,种族,场景,音色ID,是否上架,排序\n" +
		"客服小雨,温柔,https://cdn.example.com/x.png,https://cdn.example.com/full.png," +
		"https://cdn.example.com/four-view.png,https://cdn.example.com/expression.png," +
		"女,青年,亚洲人,\"客服播报、有声书\",zh_female_vv_uranus_bigtts,是,3\n"

	rows, warnings, err := avatar.ParseAvatarsCSV(strings.NewReader(csv))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, rows, 1)
	require.NoError(t, rows[0].Err)

	dto := rows[0].Create
	assert.Equal(t, "客服小雨", dto.Name)
	assert.Equal(t, "https://cdn.example.com/x.png", dto.ImageURL)
	assert.Equal(t, "https://cdn.example.com/full.png", dto.FullBodyURL, "全身照 maps onto fullBodyUrl")
	assert.Equal(t, "https://cdn.example.com/four-view.png", dto.FourViewURL)
	assert.Equal(t, "https://cdn.example.com/expression.png", dto.ExpressionURL)
	assert.Equal(t, "female", dto.Gender, "the Chinese spelling maps onto the wire vocabulary")
	assert.Equal(t, "young", dto.AgeRange)
	assert.Equal(t, "asian", dto.Race)
	assert.Equal(t, []string{"客服播报", "有声书"}, []string(dto.Scenes), "the 、 separator splits scenes")
	assert.Equal(t, "zh_female_vv_uranus_bigtts", dto.VoiceID)
	require.NotNil(t, dto.Enabled)
	assert.True(t, *dto.Enabled)
	require.NotNil(t, dto.SortOrder)
	assert.Equal(t, 3, *dto.SortOrder)
}

func TestParseAvatarsCSV_RequiresNameAndImage(t *testing.T) {
	_, _, err := avatar.ParseAvatarsCSV(strings.NewReader("name,gender\n小雨,female\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image_url")
}

func TestParseAvatarsCSV_ReportsAnUnreadableRowWithoutFailingTheFile(t *testing.T) {
	csv := "name,image_url,sort_order\n" +
		"ok,https://cdn.example.com/a.png,1\n" +
		"bad,https://cdn.example.com/b.png,not-a-number\n"

	rows, _, err := avatar.ParseAvatarsCSV(strings.NewReader(csv))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NoError(t, rows[0].Err)
	require.Error(t, rows[1].Err)
	assert.Contains(t, rows[1].Err.Error(), "排序值必须是整数")
}

func TestImportAvatars_ReportsCreatedUpdatedAndFailedRows(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "已存在", 0, true)

	csv := "name,image_url,gender\n" +
		"已存在,https://cdn.example.com/existing.png,male\n" +
		"新形象,https://cdn.example.com/new.png,female\n" +
		"坏形象,not-a-url,female\n"

	rows, warnings, err := avatar.ParseAvatarsCSV(strings.NewReader(csv))
	require.NoError(t, err)

	result, err := avatar.ImportAvatars(rows, avatar.ImportModeUpsert, warnings)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 1, result.Updated)
	assert.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 4, result.Errors[0].Row, "the header is row 1, so the failing record is row 4")
	assert.Contains(t, result.Errors[0].Message, "封面图地址必须是")

	// A bad row never discards the good ones.
	assert.Equal(t, []string{"已存在", "新形象"}, searchNames(t, avatar.AvatarListQuery{}))
}

func TestImportAvatars_UpsertOnlyTouchesTheColumnsTheFileHad(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)

	csv := "name,image_url,sort_order\n客服小雨,https://cdn.example.com/updated.png,7\n"
	rows, warnings, err := avatar.ParseAvatarsCSV(strings.NewReader(csv))
	require.NoError(t, err)

	result, err := avatar.ImportAvatars(rows, avatar.ImportModeUpsert, warnings)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Updated)

	view, err := avatar.AvatarGetByID(created.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/updated.png", view.ImageURL)
	assert.Equal(t, 7, view.SortOrder)
	assert.Equal(t, "客服小雨", view.Name)
	assert.Equal(t, "asian", view.Race, "an omitted column keeps its stored value")
	assert.Equal(t, []string{"客服播报", "有声书"}, view.Scenes)
	assert.Equal(t, "zh_female_vv_uranus_bigtts", view.VoiceID)
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-expression.png", view.ExpressionURL,
		"an omitted picture column keeps its stored value too")
}

func TestImportAvatars_CreateModeRefusesAnExistingName(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "已存在", 0, true)

	csv := "name,image_url\n已存在,https://cdn.example.com/x.png\n"
	rows, _, err := avatar.ParseAvatarsCSV(strings.NewReader(csv))
	require.NoError(t, err)

	result, err := avatar.ImportAvatars(rows, avatar.ImportModeCreate, nil)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 1, result.Failed)
	assert.Contains(t, result.Errors[0].Message, "仅新增模式")
}

func TestImportAvatars_RejectsAnUnknownModeBeforeWriting(t *testing.T) {
	newAvatarTestDB(t)

	_, err := avatar.ImportAvatars(nil, "merge", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "非法导入模式")
}

func TestAvatarCSV_RoundTripsThroughExportAndImport(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)
	insertAvatar(t, "下架形象", 2, false)

	exported, err := avatar.AvatarExportRows(avatar.AvatarListQuery{})
	require.NoError(t, err)
	buf := &bytes.Buffer{}
	require.NoError(t, avatar.WriteAvatarsCSV(buf, exported))

	// Re-importing the very file the export produced must be a no-op update, not
	// a set of new rows or failures.
	rows, warnings, err := avatar.ParseAvatarsCSV(bytes.NewReader(buf.Bytes()))
	require.NoError(t, err)
	result, err := avatar.ImportAvatars(rows, avatar.ImportModeUpsert, warnings)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Updated)
	assert.Equal(t, 0, result.Created)
	assert.Equal(t, 0, result.Failed)
	assert.Equal(t, []string{"客服小雨", "下架形象"}, searchNames(t, avatar.AvatarListQuery{}))

	// The four pictures survive the round trip, which is what makes an exported
	// file usable as an import template.
	view, err := avatar.AvatarGetByID(created.ID, nil)
	require.NoError(t, err)
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu.png", view.ImageURL)
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-full.png", view.FullBodyURL)
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-four-view.png", view.FourViewURL)
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-expression.png", view.ExpressionURL)
}

func TestParseAvatarsCSV_ReportsUnknownColumnsAsAWarning(t *testing.T) {
	csv := "name,image_url,备注\n小雨,https://cdn.example.com/a.png,ignored\n"

	_, warnings, err := avatar.ParseAvatarsCSV(strings.NewReader(csv))
	require.NoError(t, err)
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "备注")
}
