package avatar_test

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/zsy/avatar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Public face
// ---------------------------------------------------------------------------

func TestListPublicAvatars_ServesOnlyOnShelfPersonas(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t)
	insertAvatar(t, "下架形象", 2, false)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/avatar/list?page=1&page_size=10",
		Handler: avatar.TestHookListPublicAvatars,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])
	assert.Equal(t, []string{"客服小雨"}, itemNames(t, payload))
	assert.EqualValues(t, 1, jsonInt(t, dataMap(t, payload), "total"))
}

func TestListPublicAvatars_IgnoresTheEnabledParameter(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "在架形象", 0, true)
	insertAvatar(t, "下架形象", 1, false)

	_, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/avatar/list?enabled=false",
		Handler: avatar.TestHookListPublicAvatars,
	})

	assert.Equal(t, []string{"在架形象"}, itemNames(t, payload),
		"the public face pins enabled=true, so a caller cannot list drafts")
}

func TestListPublicAvatars_AnswersTheDocumentedFilters(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t)
	insertAvatar(t, "客服小刚", 2, true)

	_, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/avatar/list?gender=female&age_range=young&race=asian",
		Handler: avatar.TestHookListPublicAvatars,
	})

	assert.Equal(t, []string{"客服小雨"}, itemNames(t, payload))
}

func TestListPublicAvatars_AcceptsTheCamelCaseVoiceFilter(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t)
	insertAvatar(t, "客服小刚", 2, true)

	_, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/avatar/list?voiceId=zh_female_vv_uranus_bigtts",
		Handler: avatar.TestHookListPublicAvatars,
	})

	assert.Equal(t, []string{"客服小雨"}, itemNames(t, payload))
}

func TestListPublicAvatars_ServesTheResolvedVoiceSample(t *testing.T) {
	db := newAvatarTestDB(t)
	insertCatalogVoice(t, db, "Vivi 2.0", "zh_female_vv_uranus_bigtts",
		"/uploads/voices/202601/vivi.wav", "vivi.wav")
	insertRichAvatar(t)

	_, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/avatar/list",
		Handler: avatar.TestHookListPublicAvatars,
	})

	items := itemsOf(t, payload)
	require.Len(t, items, 1)
	assert.Equal(t, true, items[0]["voiceAvailable"])
	assert.Equal(t, "Vivi 2.0", items[0]["voiceName"])
	assert.Equal(t, "/uploads/voices/202601/vivi.wav", items[0]["voiceSampleUrl"])
	assert.Equal(t, "vivi.wav", items[0]["voiceSampleName"])
}

func TestGetPublicAvatar_AnswersOneOnShelfPersona(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/avatar/:id",
		Path:    "/api/zsy/avatar/1",
		Handler: avatar.TestHookGetPublicAvatar,
	})

	require.Equal(t, http.StatusOK, status)
	data := dataMap(t, payload)
	assert.EqualValues(t, created.ID, jsonInt(t, data, "id"))
	assert.Equal(t, "客服小雨", data["name"])
	assert.Equal(t, "asian", data["race"])
	assert.Equal(t, "zh_female_vv_uranus_bigtts", data["voiceId"])
	// Third parties read the four pictures straight off the public face.
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu.png", data["imageUrl"])
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-full.png", data["fullBodyUrl"])
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-four-view.png", data["fourViewUrl"])
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-expression.png", data["expressionUrl"])
}

func TestGetPublicAvatar_HidesOffShelfAndMissingRowsBehind404(t *testing.T) {
	newAvatarTestDB(t)
	offShelf := insertAvatar(t, "下架形象", 0, false)
	insertAvatar(t, "在架形象", 1, true)

	for _, tc := range []struct {
		name string
		path string
		want string
	}{
		{"off shelf", "/api/zsy/avatar/" + itoa(offShelf.ID), "AVATAR_NOT_FOUND"},
		{"missing", "/api/zsy/avatar/999", "AVATAR_NOT_FOUND"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, payload := doRequest(t, request{
				Method:  http.MethodGet,
				Pattern: "/api/zsy/avatar/:id",
				Path:    tc.path,
				Handler: avatar.TestHookGetPublicAvatar,
			})
			require.Equal(t, http.StatusNotFound, status)
			assert.Equal(t, false, payload["success"])
			assert.Equal(t, tc.want, payload["code"])
		})
	}
}

func TestGetPublicAvatar_RejectsANonNumericID(t *testing.T) {
	newAvatarTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/api/zsy/avatar/:id",
		Path:    "/api/zsy/avatar/abc",
		Handler: avatar.TestHookGetPublicAvatar,
	})

	require.Equal(t, http.StatusBadRequest, status)
	assert.Equal(t, "INVALID_PARAMS", payload["code"])
}

// ---------------------------------------------------------------------------
// Admin face
// ---------------------------------------------------------------------------

func TestCreateAvatar_StoresTheDocumentedPayload(t *testing.T) {
	newAvatarTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/avatar",
		Handler: avatar.TestHookCreateAvatar,
		JSON: `{
			"name":"客服小雨",
			"description":"温柔的客服形象",
			"imageUrl":"https://cdn.example.com/x.png",
			"fullBodyUrl":"https://cdn.example.com/x-full.png",
			"fourViewUrl":"https://cdn.example.com/x-four-view.png",
			"expressionUrl":"https://cdn.example.com/x-expression.png",
			"gender":"Female",
			"ageRange":"young",
			"race":"asian",
			"scenes":["客服播报","有声书"],
			"voiceId":"zh_female_vv_uranus_bigtts"
		}`,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)
	assert.Equal(t, "客服小雨", data["name"])
	assert.Equal(t, "https://cdn.example.com/x.png", data["imageUrl"])
	assert.Equal(t, "https://cdn.example.com/x-full.png", data["fullBodyUrl"])
	assert.Equal(t, "https://cdn.example.com/x-four-view.png", data["fourViewUrl"])
	assert.Equal(t, "https://cdn.example.com/x-expression.png", data["expressionUrl"])
	assert.Equal(t, "female", data["gender"], "the vocabulary is normalized to the wire value")
	assert.Equal(t, "zh_female_vv_uranus_bigtts", data["voiceId"])
	assert.Equal(t, true, data["enabled"], "a new persona is published by default")

	assert.Equal(t, []string{"客服小雨"}, searchNames(t, avatar.AvatarListQuery{}))
}

func TestCreateAvatar_AnswersARejectedPictureAsAMessage(t *testing.T) {
	newAvatarTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/avatar",
		Handler: avatar.TestHookCreateAvatar,
		JSON:    `{"name":"小雨","expressionUrl":"javascript:alert(1)"}`,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "表情图地址必须是",
		"the rejection names the picture the operator has to fix")
}

func TestUpdateAvatar_ReplacesOnePictureWithoutTouchingTheOthers(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)

	_, payload := doRequest(t, request{
		Method:  http.MethodPut,
		Pattern: "/dashboard/zsy/avatar/:id",
		Path:    "/dashboard/zsy/avatar/" + itoa(created.ID),
		Handler: avatar.TestHookUpdateAvatar,
		JSON:    `{"fourViewUrl":"https://cdn.example.com/persona/xiaoyu-four-view-v2.png"}`,
	})

	data := dataMap(t, payload)
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-four-view-v2.png", data["fourViewUrl"])
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu.png", data["imageUrl"],
		"an omitted picture keeps its stored value")
	assert.Equal(t, "https://cdn.example.com/persona/xiaoyu-expression.png", data["expressionUrl"])
}

func TestCreateAvatar_AcceptsScenesAsASeparatedString(t *testing.T) {
	newAvatarTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/avatar",
		Handler: avatar.TestHookCreateAvatar,
		JSON:    `{"name":"小雨","scenes":"客服播报,有声书"}`,
	})

	require.Equal(t, http.StatusOK, status)
	scenes, ok := dataMap(t, payload)["scenes"].([]any)
	require.True(t, ok)
	assert.Equal(t, []any{"客服播报", "有声书"}, scenes)
}

func TestCreateAvatar_AnswersARejectedFieldAsAMessage(t *testing.T) {
	newAvatarTestDB(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/avatar",
		Handler: avatar.TestHookCreateAvatar,
		JSON:    `{"name":"小雨","race":"martian"}`,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "非法种族")
}

func TestCreateAvatar_AnswersADuplicateNameAsAMessage(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "小雨", 0, true)

	_, payload := doRequest(t, request{
		Method:  http.MethodPost,
		Pattern: "/dashboard/zsy/avatar",
		Handler: avatar.TestHookCreateAvatar,
		JSON:    `{"name":"小雨"}`,
	})

	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "已存在")
}

func TestUpdateAvatar_ClearsTheVoiceLinkOnAnExplicitEmptyString(t *testing.T) {
	newAvatarTestDB(t)
	created := insertRichAvatar(t)

	status, payload := doRequest(t, request{
		Method:  http.MethodPut,
		Pattern: "/dashboard/zsy/avatar/:id",
		Path:    "/dashboard/zsy/avatar/" + itoa(created.ID),
		Handler: avatar.TestHookUpdateAvatar,
		JSON:    `{"voiceId":""}`,
	})

	require.Equal(t, http.StatusOK, status)
	data := dataMap(t, payload)
	assert.Equal(t, "", data["voiceId"])
	assert.Equal(t, "客服小雨", data["name"], "an omitted field keeps its stored value")
}

func TestDeleteAvatar_RemovesTheRow(t *testing.T) {
	newAvatarTestDB(t)
	created := insertAvatar(t, "小雨", 0, true)

	status, payload := doRequest(t, request{
		Method:  http.MethodDelete,
		Pattern: "/dashboard/zsy/avatar/:id",
		Path:    "/dashboard/zsy/avatar/" + itoa(created.ID),
		Handler: avatar.TestHookDeleteAvatar,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, "小雨", dataMap(t, payload)["name"])
	_, err := avatar.AvatarGetByID(created.ID, nil)
	assert.ErrorIs(t, err, avatar.ErrAvatarNotFound)
}

func TestListAvatarsAdmin_IncludesEveryShelfState(t *testing.T) {
	newAvatarTestDB(t)
	insertAvatar(t, "在架", 0, true)
	insertAvatar(t, "下架", 1, false)

	_, payload := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/dashboard/zsy/avatar/list",
		Handler: avatar.TestHookListAvatarsAdmin,
	})
	assert.Equal(t, []string{"在架", "下架"}, itemNames(t, payload))

	_, filtered := doRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/dashboard/zsy/avatar/list?enabled=false",
		Handler: avatar.TestHookListAvatarsAdmin,
	})
	assert.Equal(t, []string{"下架"}, itemNames(t, filtered))
}

func TestExportAvatars_StreamsTheFilteredCSV(t *testing.T) {
	newAvatarTestDB(t)
	insertRichAvatar(t)
	insertAvatar(t, "下架形象", 2, false)

	status, headers, body := doRawRequest(t, request{
		Method:  http.MethodGet,
		Pattern: "/dashboard/zsy/avatar/export?enabled=false",
		Handler: avatar.TestHookExportAvatars,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Contains(t, headers.Get("Content-Type"), "text/csv")
	assert.Contains(t, headers.Get("Content-Disposition"), "zsy-avatars-")
	assert.Contains(t, headers.Get("Cache-Control"), "no-store")

	out := string(body)
	assert.Contains(t, out, "下架形象")
	assert.NotContains(t, out, "客服小雨", "the export follows the active filters")
}

func TestImportAvatars_ReadsAMultipartCSVAndReportsTheResult(t *testing.T) {
	newAvatarTestDB(t)

	body, contentType := multipartFile(t, "avatars.csv",
		[]byte("name,image_url,gender\n小雨,https://cdn.example.com/x.png,female\n"))

	status, payload := doRequest(t, request{
		Method:      http.MethodPost,
		Pattern:     "/dashboard/zsy/avatar/import?mode=upsert",
		Handler:     avatar.TestHookImportAvatars,
		Body:        body,
		ContentType: contentType,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, true, payload["success"])
	data := dataMap(t, payload)
	assert.EqualValues(t, 1, jsonInt(t, data, "total"))
	assert.EqualValues(t, 1, jsonInt(t, data, "created"))
	assert.Equal(t, []string{"小雨"}, searchNames(t, avatar.AvatarListQuery{}))
}

func TestImportAvatars_AnswersABrokenFileWithoutWritingAnything(t *testing.T) {
	newAvatarTestDB(t)

	body, contentType := multipartFile(t, "avatars.csv", []byte("gender\nfemale\n"))

	status, payload := doRequest(t, request{
		Method:      http.MethodPost,
		Pattern:     "/dashboard/zsy/avatar/import",
		Handler:     avatar.TestHookImportAvatars,
		Body:        body,
		ContentType: contentType,
	})

	require.Equal(t, http.StatusOK, status)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "缺少必需列")
	assert.Empty(t, searchNames(t, avatar.AvatarListQuery{}))
}

// itoa keeps the request paths readable; the ids in these tests are small.
func itoa(n uint) string {
	return strconv.FormatUint(uint64(n), 10)
}
