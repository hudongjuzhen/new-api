package voice_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/voice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// CSV export / import — the spreadsheet round trip an operator depends on.
// ---------------------------------------------------------------------------

const (
	exportPath = "/dashboard/zsy/voice/export"
	importPath = "/dashboard/zsy/voice/import"
)

// exportedCSV runs the export store path over the current rows.
func exportedCSV(t *testing.T) []byte {
	t.Helper()
	rows, err := voice.VoiceExportRows(voice.VoiceListQuery{})
	require.NoError(t, err)

	buffer := &bytes.Buffer{}
	require.NoError(t, voice.WriteVoicesCSV(buffer, rows))
	return buffer.Bytes()
}

// csvRecords parses an exported file, dropping the BOM and the header row.
func csvRecords(t *testing.T, raw []byte) ([]string, [][]string) {
	t.Helper()
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte("\ufeff"))))
	records, err := reader.ReadAll()
	require.NoError(t, err)
	require.NotEmpty(t, records)
	return records[0], records[1:]
}

func TestVoiceCSV_ExportAndImportRoundTrip(t *testing.T) {
	newVoiceTestDB(t)
	insertRichVoice(t)
	insertVoice(t, "普通音色", "vt-plain", 2, false)

	raw := exportedCSV(t)
	assert.True(t, bytes.HasPrefix(raw, []byte("\ufeff")), "the export must carry a UTF-8 BOM for Excel")

	header, records := csvRecords(t, raw)
	assert.Equal(t, []string{
		"name", "description", "voice_type", "gender", "age_range", "language",
		"scenes", "avatar_url", "audio_url", "audio_name", "audio_size", "enabled", "sort_order",
	}, header)
	require.Len(t, records, 2)

	// Plaza order: sort_order ascending.
	assert.Equal(t, "客服女声", records[0][0])
	assert.Equal(t, "female", records[0][3])
	assert.Equal(t, "young", records[0][4])
	assert.Equal(t, "zh", records[0][5])
	assert.Equal(t, "客服播报,有声书", records[0][6], "scenes stay one (quoted) cell")
	assert.Equal(t, "https://cdn.example.com/avatar/kefu.png", records[0][7])
	assert.Equal(t, "true", records[0][11])
	assert.Equal(t, "普通音色", records[1][0])
	assert.Equal(t, "false", records[1][11], "off-shelf rows are part of an admin export")

	// Re-importing the export must describe exactly the same voices.
	rows, warnings, err := voice.ParseVoicesCSV(bytes.NewReader(raw))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, rows, 2)
	assert.Equal(t, "客服女声", rows[0].Create.Name)
	assert.Equal(t, "female", rows[0].Create.Gender)
	assert.Equal(t, "young", rows[0].Create.AgeRange)
	assert.Equal(t, "zh", rows[0].Create.Language)
	assert.Equal(t, voice.ScenesInput{"客服播报", "有声书"}, rows[0].Create.Scenes)
	assert.Equal(t, "https://cdn.example.com/avatar/kefu.png", rows[0].Create.AvatarURL)
	assert.Equal(t, int64(4096), rows[0].Create.AudioSize)
	require.NotNil(t, rows[0].Create.Enabled)
	assert.True(t, *rows[0].Create.Enabled)
	require.NotNil(t, rows[1].Create.Enabled)
	assert.False(t, *rows[1].Create.Enabled)
}

func TestVoiceCSV_QuotesValuesThatContainSeparators(t *testing.T) {
	newVoiceTestDB(t)
	_, err := voice.VoiceInsert(&voice.VoiceCreateDTO{
		Name:        "含标点音色",
		VoiceType:   "vt-quote",
		Description: "简介里有逗号,还有\"引号\"\n和换行",
		Scenes:      []string{"客服播报", "有声书"},
	})
	require.NoError(t, err)

	header, records := csvRecords(t, exportedCSV(t))
	require.Len(t, header, 13)
	require.Len(t, records, 1)
	assert.Equal(t, "简介里有逗号,还有\"引号\"\n和换行", records[0][1])
	assert.Equal(t, "客服播报,有声书", records[0][6])
}

func TestVoiceCSV_ImportAcceptsChineseHeaders(t *testing.T) {
	raw := "\ufeff音色名称,简介,voice_type,性别,适合场景,年龄段,语言,头像URL,音频示例URL,音频大小,是否上架,排序,备注列\n" +
		"甜美女声,适合客服,zh_female_sweet,女,\"客服播报、有声书\",青年,zh,https://cdn.example.com/a.png,/uploads/voices/a.mp3,2048,下架,3,随便写\n"

	rows, warnings, err := voice.ParseVoicesCSV(strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Len(t, warnings, 1, "an unknown column is reported, not silently mapped")
	assert.Contains(t, warnings[0], "备注列")

	row := rows[0]
	assert.Equal(t, 2, row.Row, "the header counts as row 1")
	assert.Equal(t, "甜美女声", row.Create.Name)
	assert.Equal(t, "zh_female_sweet", row.Create.VoiceType)
	assert.Equal(t, "女", row.Create.Gender, "the vocabulary is validated after parsing")
	assert.Equal(t, "青年", row.Create.AgeRange)
	assert.Equal(t, "zh", row.Create.Language)
	assert.Equal(t, "https://cdn.example.com/a.png", row.Create.AvatarURL)
	assert.Equal(t, "/uploads/voices/a.mp3", row.Create.AudioURL)
	assert.Equal(t, voice.ScenesInput{"客服播报", "有声书"}, row.Create.Scenes)
	assert.Equal(t, int64(2048), row.Create.AudioSize)
	require.NotNil(t, row.Create.Enabled)
	assert.False(t, *row.Create.Enabled, "下架 reads as off shelf")
	require.NotNil(t, row.Create.SortOrder)
	assert.Equal(t, 3, *row.Create.SortOrder)
	assert.NoError(t, row.Err)
}

func TestVoiceCSV_ImportAcceptsEnglishAliasesAndSpellings(t *testing.T) {
	// Headers an operator may type: different case, spaces, dashes, and the
	// canonical names abbreviated.
	raw := "Name,Voice Type,Gender,Age-Range,Scene,Enabled,Sort Order\n" +
		"男声播报,zh_male_news,Male,middle,新闻;播报,YES,-2\n"

	rows, warnings, err := voice.ParseVoicesCSV(strings.NewReader(raw))
	require.NoError(t, err)
	assert.Empty(t, warnings)
	require.Len(t, rows, 1)
	assert.Equal(t, "男声播报", rows[0].Create.Name)
	assert.Equal(t, "zh_male_news", rows[0].Create.VoiceType)
	assert.Equal(t, "Male", rows[0].Create.Gender)
	assert.Equal(t, "middle", rows[0].Create.AgeRange)
	assert.Equal(t, voice.ScenesInput{"新闻", "播报"}, rows[0].Create.Scenes)
	require.NotNil(t, rows[0].Create.Enabled)
	assert.True(t, *rows[0].Create.Enabled)
	require.NotNil(t, rows[0].Create.SortOrder)
	assert.Equal(t, -2, *rows[0].Create.SortOrder)
}

func TestVoiceCSV_ImportRejectsStructuralProblems(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		wantErr string
	}{
		{name: "empty file", raw: "", wantErr: "CSV 文件为空"},
		{
			name:    "no header only",
			raw:     "name,voice_type\n",
			wantErr: "没有可导入的数据行",
		},
		{
			name:    "missing name column",
			raw:     "voice_type,gender\nvt-a,female\n",
			wantErr: "缺少必需列: name",
		},
		{
			name:    "missing voice_type column",
			raw:     "音色名称,性别\n小美,女\n",
			wantErr: "缺少必需列: voice_type",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := voice.ParseVoicesCSV(strings.NewReader(tc.raw))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func TestVoiceCSV_ImportEnforcesTheRowLimit(t *testing.T) {
	var builder strings.Builder
	builder.WriteString("name,voice_type\n")
	for i := 0; i <= 5000; i++ {
		fmt.Fprintf(&builder, "音色%d,vt-%d\n", i, i)
	}

	_, _, err := voice.ParseVoicesCSV(strings.NewReader(builder.String()))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "超过上限 5000")
}

func TestVoiceCSV_SkipsBlankRows(t *testing.T) {
	raw := "name,voice_type\n小美,vt-a\n,\n\n小北,vt-b\n"

	rows, _, err := voice.ParseVoicesCSV(strings.NewReader(raw))
	require.NoError(t, err)
	require.Len(t, rows, 2)
	// The "," record is blank and is skipped without producing a voice; a truly
	// empty line is dropped by the CSV reader itself, so it does not consume a
	// record number either. Numbering therefore matches the file lines of a
	// normally exported file (which has no blank lines).
	assert.Equal(t, []int{2, 4}, []int{rows[0].Row, rows[1].Row})
}

func TestImportVoices_UpsertUpdatesMatchesAndCreatesTheRest(t *testing.T) {
	newVoiceTestDB(t)
	existing := insertVoice(t, "已有音色", "vt-old", 0, true)

	raw := "name,voice_type,description,gender,age_range,scenes,enabled\n" +
		"已有音色,vt-old,改过的简介,female,young,客服播报,true\n" +
		"新音色,vt-new,新人,male,senior,有声书,false\n"
	rows, _, err := voice.ParseVoicesCSV(strings.NewReader(raw))
	require.NoError(t, err)

	result, err := voice.ImportVoices(rows, voice.ImportModeUpsert, nil)
	require.NoError(t, err)
	assert.Equal(t, voice.VoiceImportResult{
		Total: 2, Created: 1, Updated: 1, Failed: 0,
		Errors: []voice.VoiceImportRowError{},
	}, result)

	updated, err := voice.VoiceGetByID(existing.ID)
	require.NoError(t, err)
	assert.Equal(t, "改过的简介", updated.Description)
	assert.Equal(t, "female", updated.Gender)
	assert.Equal(t, []string{"客服播报"}, updated.Scenes)
	assert.True(t, updated.Enabled, "the file's enabled column is authoritative on an upsert")

	created, err := voice.VoiceSearch(voice.VoiceListQuery{Keyword: "新音色", Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, created.Items, 1)
	assert.False(t, created.Items[0].Enabled)
	assert.Equal(t, []string{"有声书"}, created.Items[0].Scenes)
}

func TestImportVoices_UpsertLeavesOmittedColumnsAlone(t *testing.T) {
	newVoiceTestDB(t)
	existing := insertRichVoice(t)

	// A two-column file must not blank the attributes it never mentioned.
	rows, _, err := voice.ParseVoicesCSV(strings.NewReader("name,voice_type\n客服女声,zh_female_kefu\n"))
	require.NoError(t, err)

	result, err := voice.ImportVoices(rows, voice.ImportModeUpsert, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Updated)

	after, err := voice.VoiceGetByID(existing.ID)
	require.NoError(t, err)
	assert.Equal(t, "young", after.AgeRange)
	assert.Equal(t, []string{"客服播报", "有声书"}, after.Scenes)
	assert.Equal(t, "温柔女声，适合客服播报", after.Description)
	assert.True(t, after.Enabled, "an omitted enabled column keeps the stored state")
}

func TestImportVoices_CreateModeRefusesExistingNames(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "已有音色", "vt-old", 0, true)

	rows, _, err := voice.ParseVoicesCSV(strings.NewReader(
		"name,voice_type\n已有音色,vt-old\n全新音色,vt-new\n"))
	require.NoError(t, err)

	result, err := voice.ImportVoices(rows, voice.ImportModeCreate, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Created)
	assert.Zero(t, result.Updated)
	assert.Equal(t, 1, result.Failed)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, 2, result.Errors[0].Row)
	assert.Equal(t, "已有音色", result.Errors[0].Name)
	assert.Contains(t, result.Errors[0].Message, "已存在")
}

func TestImportVoices_ReportsBadRowsAndKeepsTheGoodOnes(t *testing.T) {
	newVoiceTestDB(t)

	raw := "name,voice_type,gender,sort_order,audio_size\n" +
		"合法音色,vt-ok,female,1,100\n" +
		"性别错误,vt-bad,robot,2,100\n" +
		",vt-noname,female,3,100\n" +
		"排序错误,vt-sort,female,abc,100\n"
	rows, _, err := voice.ParseVoicesCSV(strings.NewReader(raw))
	require.NoError(t, err)

	result, err := voice.ImportVoices(rows, voice.ImportModeUpsert, nil)
	require.NoError(t, err)
	assert.Equal(t, 4, result.Total)
	assert.Equal(t, 1, result.Created)
	assert.Equal(t, 3, result.Failed)
	require.Len(t, result.Errors, 3)

	byRow := map[int]string{}
	for _, rowErr := range result.Errors {
		byRow[rowErr.Row] = rowErr.Message
	}
	assert.Contains(t, byRow[3], "非法性别")
	assert.Contains(t, byRow[4], "音色名称不能为空")
	assert.Contains(t, byRow[5], "排序值必须是整数")

	stored, err := voice.VoiceSearch(voice.VoiceListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Equal(t, []string{"合法音色"}, voiceNames(stored.Items), "one bad line must not discard the file")
}

func TestImportVoices_RejectsUnknownModeBeforeWriting(t *testing.T) {
	newVoiceTestDB(t)
	rows, _, err := voice.ParseVoicesCSV(strings.NewReader("name,voice_type\n音色,vt-a\n"))
	require.NoError(t, err)

	_, err = voice.ImportVoices(rows, "replace", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "非法导入模式")

	stored, err := voice.VoiceSearch(voice.VoiceListQuery{Page: 1, PageSize: 10})
	require.NoError(t, err)
	assert.Empty(t, stored.Items)
}

// ---------------------------------------------------------------------------
// HTTP face
// ---------------------------------------------------------------------------

func TestExportVoices_DownloadsCSVForTheActiveFilters(t *testing.T) {
	newVoiceTestDB(t)
	insertRichVoice(t)
	insertVoice(t, "小北", "zh_male_xiaobei", 2, true)

	code, header, body := doRawRequest(t, request{
		Method: http.MethodGet, Pattern: exportPath, Handler: voice.TestHookExportVoices,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Contains(t, header.Get("Content-Type"), "text/csv")
	assert.Contains(t, header.Get("Content-Disposition"), "attachment; filename=\"zsy-voices-")
	assert.Contains(t, header.Get("Content-Disposition"), ".csv\"")
	assert.Contains(t, header.Get("Cache-Control"), "no-store")

	_, records := csvRecords(t, body)
	assert.Len(t, records, 2, "no filters exports the whole plaza")

	// The export follows the same filters as the list, so an operator can hand
	// out just the slice they are looking at.
	code, _, filtered := doRawRequest(t, request{
		Method: http.MethodGet, Pattern: exportPath + "?gender=female", Handler: voice.TestHookExportVoices,
	})
	require.Equal(t, http.StatusOK, code)
	_, filteredRecords := csvRecords(t, filtered)
	require.Len(t, filteredRecords, 1)
	assert.Equal(t, "客服女声", filteredRecords[0][0])
}

func TestExportVoices_EmptyCatalogStillCarriesTheHeader(t *testing.T) {
	newVoiceTestDB(t)

	_, _, body := doRawRequest(t, request{
		Method: http.MethodGet, Pattern: exportPath, Handler: voice.TestHookExportVoices,
	})
	header, records := csvRecords(t, body)
	assert.Len(t, header, 13, "an empty export doubles as the import template")
	assert.Empty(t, records)
}

func TestImportVoices_HTTPUploadsCSV(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "已有音色", "vt-old", 0, true)

	content := []byte("\ufeffname,voice_type,gender,age_range,scenes\n" +
		"已有音色,vt-old,female,young,\"客服播报,有声书\"\n" +
		"新音色,vt-new,male,senior,新闻\n")
	body, contentType := multipartFile(t, "voices.csv", content)

	code, payload := doRequest(t, request{
		Method: http.MethodPost, Pattern: importPath, Handler: voice.TestHookImportVoices,
		Body: body, ContentType: contentType,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, payload["success"], "import failed: %s", messageOf(t, payload))

	result := dataMap(t, payload)
	assert.EqualValues(t, 2, jsonInt(t, result, "total"))
	assert.EqualValues(t, 1, jsonInt(t, result, "created"))
	assert.EqualValues(t, 1, jsonInt(t, result, "updated"))
	assert.EqualValues(t, 0, jsonInt(t, result, "failed"))
}

func TestImportVoices_HTTPSupportsCreateMode(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "已有音色", "vt-old", 0, true)

	body, contentType := multipartFile(t, "voices.csv", []byte("name,voice_type\n已有音色,vt-old\n"))
	_, payload := doRequest(t, request{
		Method: http.MethodPost, Pattern: importPath + "?mode=create", Handler: voice.TestHookImportVoices,
		Body: body, ContentType: contentType,
	})
	assert.EqualValues(t, 1, jsonInt(t, dataMap(t, payload), "failed"))

	_, created := doRequest(t, request{
		Method: http.MethodPost, Pattern: importPath + "?mode=create", Handler: voice.TestHookImportVoices,
		Body: body, ContentType: contentType,
	})
	assert.EqualValues(t, 1, jsonInt(t, dataMap(t, created), "failed"))
}

func TestImportVoices_HTTPRejectsUnusableUploads(t *testing.T) {
	newVoiceTestDB(t)

	// A file that is not a voice CSV at all.
	body, contentType := multipartFile(t, "notes.csv", []byte("hello\nworld\n"))
	code, payload := doRequest(t, request{
		Method: http.MethodPost, Pattern: importPath, Handler: voice.TestHookImportVoices,
		Body: body, ContentType: contentType,
	})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "缺少必需列")

	// No multipart body at all.
	_, payload = doRequest(t, request{
		Method: http.MethodPost, Pattern: importPath, Handler: voice.TestHookImportVoices,
		Body: []byte("not multipart"), ContentType: "text/plain",
	})
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "multipart")

	// An unknown mode is refused by the handler with the accepted values.
	csvBody, csvType := multipartFile(t, "voices.csv", []byte("name,voice_type\n音色,vt-a\n"))
	_, payload = doRequest(t, request{
		Method: http.MethodPost, Pattern: importPath + "?mode=replace", Handler: voice.TestHookImportVoices,
		Body: csvBody, ContentType: csvType,
	})
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "非法导入模式")
}
