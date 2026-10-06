package tone_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/tone"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// HTTP faces. The handlers are mounted one at a time on a bare router (see
// testenv_test.go), so these assert the response contract without depending on
// the auth middlewares.
// ---------------------------------------------------------------------------

func TestListPublicTones_ServesOnlyOnShelf(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "上架的", "p", 0, true)
	insertTone(t, "下架的", "p", 1, false)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/list",
		Handler: tone.TestHookListPublicTones,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)
	assert.EqualValues(t, 1, jsonInt(t, data, "total"))
	assert.Equal(t, []string{"上架的"}, namesOf(t, payload))
}

// TestListPublicTones_IgnoresTheEnabledParameter pins that `enabled` belongs to
// the admin face only. If the public list honoured it, `?enabled=false` would
// turn the catalog into a list of unpublished drafts.
func TestListPublicTones_IgnoresTheEnabledParameter(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "上架的", "p", 0, true)
	insertTone(t, "下架的", "p", 1, false)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/list?enabled=false",
		Handler: tone.TestHookListPublicTones,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []string{"上架的"}, namesOf(t, payload))
}

func TestListPublicTones_HonoursFiltersAndPagination(t *testing.T) {
	newToneTestDB(t)
	insertRichTone(t)
	insertTone(t, "别的", "p", 5, true)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/list?category=literary&page=1&page_size=1",
		Handler: tone.TestHookListPublicTones,
	})

	require.Equal(t, http.StatusOK, status)
	data := dataMap(t, payload)
	assert.EqualValues(t, 1, jsonInt(t, data, "total"))
	assert.EqualValues(t, 1, jsonInt(t, data, "pageSize"))
	assert.Equal(t, []string{"克制的长文"}, namesOf(t, payload))
}

// TestPublicTone_ShapeOfARow pins the whole documented row, field by field, so a
// renamed or dropped JSON key fails here rather than silently breaking every
// integrator.
func TestPublicTone_ShapeOfARow(t *testing.T) {
	newToneTestDB(t)
	insertRichTone(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/list",
		Handler: tone.TestHookListPublicTones,
	})
	require.Equal(t, http.StatusOK, status)

	items := itemsOf(t, payload)
	require.Len(t, items, 1)
	row := items[0]

	assert.NotZero(t, jsonInt(t, row, "id"))
	assert.NotZero(t, jsonInt(t, row, "createdAt"))
	assert.NotZero(t, jsonInt(t, row, "updatedAt"))
	assert.Equal(t, "克制的长文", row["name"])
	assert.Equal(t, "适合公众号长文，情绪收着写", row["description"])
	assert.Contains(t, row["prompt"], "克制")
	assert.Equal(t, "literary", row["category"])
	assert.Equal(t, "calm", row["tone"])
	assert.Equal(t, "zh", row["language"])
	assert.Equal(t, []string{"公众号长文", "散文"}, jsonStrings(t, row, "scenes"))
	assert.Equal(t, sharedSampleInput, row["sampleInput"])
	assert.NotEmpty(t, row["sampleOutput"])
	assert.Equal(t, true, row["enabled"])
	assert.EqualValues(t, 1, jsonInt(t, row, "sortOrder"))
}

func TestGetPublicTone_OffShelfAnswers404LikeMissing(t *testing.T) {
	newToneTestDB(t)
	offShelf := insertTone(t, "下架的", "p", 0, false)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/:id",
		Path:    "/api/zsy/tone/1",
		Handler: tone.TestHookGetPublicTone,
	})

	require.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, false, payload["success"])
	assert.Equal(t, "TONE_NOT_FOUND", payload["code"],
		"an unpublished draft must be indistinguishable from a missing row")
	assert.NotZero(t, offShelf.ID)
}

func TestGetPublicTone_NotFound(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/:id",
		Path:    "/api/zsy/tone/9999",
		Handler: tone.TestHookGetPublicTone,
	})

	require.Equal(t, http.StatusNotFound, status)
	assert.Equal(t, "TONE_NOT_FOUND", payload["code"])
}

func TestGetPublicTone_RejectsABadID(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/:id",
		Path:    "/api/zsy/tone/abc",
		Handler: tone.TestHookGetPublicTone,
	})

	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "INVALID_PARAMS", payload["code"])
}

func TestGetToneStandard_ServesTheVocabulary(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/tone/standard",
		Handler: tone.TestHookGetToneStandard,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)

	assert.Equal(t, tone.ToneStandardVersion, data["version"])

	categories, ok := data["categories"].([]any)
	require.True(t, ok, "categories missing in %v", data)
	assert.Len(t, categories, len(tone.AllowedCategories))

	tones, ok := data["tones"].([]any)
	require.True(t, ok, "tones missing in %v", data)
	assert.Len(t, tones, len(tone.AllowedTones))

	first, ok := categories[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "literary", first["value"])
	assert.Equal(t, "文学", first["label"])
	assert.Equal(t, "Literary", first["labelEn"])
	assert.NotEmpty(t, first["desc"])

	limits, ok := data["limits"].(map[string]any)
	require.True(t, ok, "limits missing in %v", data)
	assert.EqualValues(t, 8000, jsonInt(t, limits, "prompt"))
	assert.EqualValues(t, 191, jsonInt(t, limits, "name"))

	compatibility, ok := data["compatibility"].([]any)
	require.True(t, ok, "compatibility missing in %v", data)
	assert.NotEmpty(t, compatibility)
}

func TestListTonesAdmin_IncludesOffShelfRows(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "上架的", "p", 0, true)
	insertTone(t, "下架的", "p", 1, false)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/dashboard/zsy/tone/list",
		Handler: tone.TestHookListTonesAdmin,
	})

	require.Equal(t, http.StatusOK, status)
	assert.EqualValues(t, 2, jsonInt(t, dataMap(t, payload), "total"))
}

func TestListTonesAdmin_FiltersByEnabled(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "上架的", "p", 0, true)
	insertTone(t, "下架的", "p", 1, false)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/dashboard/zsy/tone/list?enabled=false",
		Handler: tone.TestHookListTonesAdmin,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, []string{"下架的"}, namesOf(t, payload))
}

func TestCreateTone_AnswersTheCreatedRow(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/tone",
		Handler: tone.TestHookCreateTone,
		JSON: `{"name":"新建的文风","description":"说明","prompt":"写短句，少用形容词",
		        "category":"Literary","tone":"CALM","language":"zh","scenes":["长文","散文"],
		        "sampleInput":"原文","sampleOutput":"改写","sortOrder":7}`,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)
	assert.Equal(t, "新建的文风", data["name"])
	assert.Equal(t, "literary", data["category"], "the vocabulary is normalized on the way in")
	assert.Equal(t, "calm", data["tone"])
	assert.Equal(t, []string{"长文", "散文"}, jsonStrings(t, data, "scenes"))
	assert.Equal(t, true, data["enabled"], "a tone is on the shelf unless the payload says otherwise")
	assert.EqualValues(t, 7, jsonInt(t, data, "sortOrder"))
}

func TestCreateTone_RejectsAMissingPrompt(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/tone",
		Handler: tone.TestHookCreateTone,
		JSON:    `{"name":"没有提示词"}`,
	})

	require.Equal(t, http.StatusOK, status, "the dashboard envelope reports business errors with success:false")
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "文风提示词不能为空")
}

func TestCreateTone_RejectsAMalformedBody(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/tone",
		Handler: tone.TestHookCreateTone,
		JSON:    `{"name":`,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "请求体错误")
}

func TestUpdateTone_PartialUpdateKeepsOtherFields(t *testing.T) {
	newToneTestDB(t)
	seeded := insertRichTone(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPut,
		Pattern: "/dashboard/zsy/tone/:id",
		Path:    "/dashboard/zsy/tone/1",
		Handler: tone.TestHookUpdateTone,
		JSON:    `{"tone":"sharp"}`,
	})

	require.Equal(t, http.StatusOK, status)
	data := dataMap(t, payload)
	assert.Equal(t, "sharp", data["tone"])
	assert.Equal(t, seeded.Prompt, data["prompt"])
	assert.Equal(t, seeded.Category, data["category"])
}

func TestUpdateTone_NotFound(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPut,
		Pattern: "/dashboard/zsy/tone/:id",
		Path:    "/dashboard/zsy/tone/9999",
		Handler: tone.TestHookUpdateTone,
		JSON:    `{"tone":"warm"}`,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "文风不存在")
}

func TestDeleteTone_AnswersTheDeletedName(t *testing.T) {
	newToneTestDB(t)
	insertTone(t, "要删的", "p", 0, true)

	status, payload := doRequest(t, request{
		Method:  http.MethodDelete,
		Pattern: "/dashboard/zsy/tone/:id",
		Path:    "/dashboard/zsy/tone/1",
		Handler: tone.TestHookDeleteTone,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])
	assert.Equal(t, "要删的", dataMap(t, payload)["name"])
}

func TestExportTones_SendsACSVAttachment(t *testing.T) {
	newToneTestDB(t)
	insertRichTone(t)

	status, headers, body := doRawRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/dashboard/zsy/tone/export",
		Handler: tone.TestHookExportTones,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, headers.Get("Content-Type"), "text/csv")
	assert.Contains(t, headers.Get("Content-Disposition"), "attachment")
	assert.Contains(t, headers.Get("Content-Disposition"), "zsy-tones-")
	assert.Contains(t, headers.Get("Cache-Control"), "no-store")
	assert.True(t, strings.HasPrefix(string(body), "\ufeff"))
	assert.Contains(t, string(body), "克制的长文")
}

func TestImportTones_ReportsPerRowOutcomes(t *testing.T) {
	newToneTestDB(t)

	csvText := "name,prompt,category\n" +
		"新来的,写短句,literary\n" +
		"坏类别,写短句,poetry\n"
	body, contentType := multipartFile(t, "tones.csv", []byte(csvText))

	status, payload := doRequest(t, request{
		Method:      http.MethodPost,
		Pattern:     "/dashboard/zsy/tone/import",
		Handler:     tone.TestHookImportTones,
		Body:        body,
		ContentType: contentType,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])

	data := dataMap(t, payload)
	assert.EqualValues(t, 2, jsonInt(t, data, "total"))
	assert.EqualValues(t, 1, jsonInt(t, data, "created"))
	assert.EqualValues(t, 1, jsonInt(t, data, "failed"))
}

// TestImportTones_EmptyFileIsABusinessError pins the difference between "the
// request was malformed" and "the file had nothing in it": the operator needs to
// read a sentence about the file, not a 400 from the framework.
func TestImportTones_EmptyFileIsABusinessError(t *testing.T) {
	newToneTestDB(t)

	body, contentType := multipartFile(t, "empty.csv", []byte(""))

	status, payload := doRequest(t, request{
		Method:      http.MethodPost,
		Pattern:     "/dashboard/zsy/tone/import",
		Handler:     tone.TestHookImportTones,
		Body:        body,
		ContentType: contentType,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "CSV 文件为空")
}

func TestImportTones_MissingFileField(t *testing.T) {
	newToneTestDB(t)

	status, payload := doRequest(t, request{
		Method:      http.MethodPost,
		Pattern:     "/dashboard/zsy/tone/import",
		Handler:     tone.TestHookImportTones,
		Body:        []byte("not multipart"),
		ContentType: "text/plain",
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "multipart")
}
