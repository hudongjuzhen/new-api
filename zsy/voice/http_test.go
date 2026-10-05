package voice_test

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/voice"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Public face — the contract a third-party caller depends on.
// ---------------------------------------------------------------------------

const publicListPath = "/api/zsy/voice/list"

func TestPublicVoiceList_ReturnsOnShelfPage(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "音色甲", "vt-a", 0, true)
	insertRichVoice(t)
	insertVoice(t, "音色乙", "vt-b", 2, true)
	insertVoice(t, "下架音色", "vt-off", 3, false)

	code, payload := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?page=1&page_size=2",
		Handler: voice.TestHookListPublicVoices,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, payload["success"])

	data := dataMap(t, payload)
	assert.EqualValues(t, 3, jsonInt(t, data, "total"), "off-shelf voices never reach the public list")
	assert.EqualValues(t, 1, jsonInt(t, data, "page"))
	assert.EqualValues(t, 2, jsonInt(t, data, "pageSize"))
	assert.EqualValues(t, 2, jsonInt(t, data, "totalPages"))

	items := itemsOf(t, payload)
	require.Len(t, items, 2)
	for _, key := range []string{
		"id", "createdAt", "updatedAt", "name", "description",
		"voiceType", "gender", "ageRange", "language", "scenes",
		"avatarUrl", "audioUrl", "audioName", "audioSize", "enabled", "sortOrder",
	} {
		assert.Contains(t, items[0], key, "documented row field %q is missing", key)
	}
	assert.Equal(t, "音色甲", items[0]["name"])
	assert.Equal(t, "vt-a", items[0]["voiceType"])
	assert.Equal(t, true, items[0]["enabled"])
	assert.Equal(t, []any{}, items[0]["scenes"], "a voice without scenes still answers an array")

	// The attributes ride along with the row, scenes as a real array.
	rich := items[1]
	assert.Equal(t, "female", rich["gender"])
	assert.Equal(t, "young", rich["ageRange"])
	assert.Equal(t, "zh", rich["language"])
	assert.Equal(t, "https://cdn.example.com/avatar/kefu.png", rich["avatarUrl"])
	assert.Equal(t, []any{"客服播报", "有声书"}, rich["scenes"])

	_, page2 := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?page=2&page_size=2",
		Handler: voice.TestHookListPublicVoices,
	})
	page2Items := itemsOf(t, page2)
	require.Len(t, page2Items, 1)
	assert.Equal(t, "音色乙", page2Items[0]["name"], "the second page continues the plaza order")

	// The public face is a catalog of what is on shelf: `enabled` is an admin
	// filter and must not change this answer.
	_, ignored := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?enabled=false",
		Handler: voice.TestHookListPublicVoices,
	})
	assert.EqualValues(t, 3, jsonInt(t, dataMap(t, ignored), "total"))
}

func TestPublicVoiceList_AcceptsHostPaginationAliases(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "音色甲", "vt-a", 0, true)
	insertVoice(t, "音色乙", "vt-b", 1, true)

	_, payload := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?p=2&size=1",
		Handler: voice.TestHookListPublicVoices,
	})
	data := dataMap(t, payload)
	assert.EqualValues(t, 2, jsonInt(t, data, "page"))
	assert.EqualValues(t, 1, jsonInt(t, data, "pageSize"))
	items := itemsOf(t, payload)
	require.Len(t, items, 1)
	assert.Equal(t, "音色乙", items[0]["name"])
}

func TestPublicVoiceList_FiltersByKeywordVoiceTypeGenderAndAge(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "小美", "zh_female_xiaomei", 0, true)
	insertVoice(t, "小北", "zh_male_xiaobei", 1, true)
	insertRichVoice(t)

	_, byVoiceType := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?voice_type=zh_female_xiaomei",
		Handler: voice.TestHookListPublicVoices,
	})
	assert.Len(t, itemsOf(t, byVoiceType), 1)

	_, byKeyword := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?keyword=xiaobei",
		Handler: voice.TestHookListPublicVoices,
	})
	items := itemsOf(t, byKeyword)
	require.Len(t, items, 1)
	assert.Equal(t, "小北", items[0]["name"])

	// The attribute filters are the reason the three fields exist: they let a
	// caller ask for "female voices for young audiences" in one request.
	_, byGender := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?gender=female",
		Handler: voice.TestHookListPublicVoices,
	})
	genderItems := itemsOf(t, byGender)
	require.Len(t, genderItems, 1)
	assert.Equal(t, "客服女声", genderItems[0]["name"])

	_, byAge := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?age_range=young",
		Handler: voice.TestHookListPublicVoices,
	})
	assert.Len(t, itemsOf(t, byAge), 1)

	_, byScene := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?keyword=有声书",
		Handler: voice.TestHookListPublicVoices,
	})
	assert.Len(t, itemsOf(t, byScene), 1)

	// Language is its own filter because a plaza of 377 voices spans 17 of them.
	_, byLanguage := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath + "?language=ZH",
		Handler: voice.TestHookListPublicVoices,
	})
	languageItems := itemsOf(t, byLanguage)
	require.Len(t, languageItems, 1)
	assert.Equal(t, "客服女声", languageItems[0]["name"])
}

func TestPublicVoiceDetail_HidesOffShelfVoices(t *testing.T) {
	newVoiceTestDB(t)
	onShelf := insertVoice(t, "在架音色", "vt-on", 0, true)
	offShelf := insertVoice(t, "下架音色", "vt-off", 0, false)

	code, payload := doRequest(t, request{
		Method: http.MethodGet, Pattern: "/api/zsy/voice/:id",
		Path:    fmt.Sprintf("/api/zsy/voice/%d", onShelf.ID),
		Handler: voice.TestHookGetPublicVoice,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "在架音色", dataMap(t, payload)["name"])

	code, payload = doRequest(t, request{
		Method: http.MethodGet, Pattern: "/api/zsy/voice/:id",
		Path:    fmt.Sprintf("/api/zsy/voice/%d", offShelf.ID),
		Handler: voice.TestHookGetPublicVoice,
	})
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "VOICE_NOT_FOUND", payload["code"])
	assert.Equal(t, false, payload["success"])

	code, payload = doRequest(t, request{
		Method: http.MethodGet, Pattern: "/api/zsy/voice/:id", Path: "/api/zsy/voice/424242",
		Handler: voice.TestHookGetPublicVoice,
	})
	assert.Equal(t, http.StatusNotFound, code)
	assert.Equal(t, "VOICE_NOT_FOUND", payload["code"])

	code, payload = doRequest(t, request{
		Method: http.MethodGet, Pattern: "/api/zsy/voice/:id", Path: "/api/zsy/voice/abc",
		Handler: voice.TestHookGetPublicVoice,
	})
	assert.Equal(t, http.StatusBadRequest, code)
	assert.Equal(t, "INVALID_PARAMS", payload["code"])
}

// ---------------------------------------------------------------------------
// Admin face — CRUD as the dashboard page drives it.
// ---------------------------------------------------------------------------

const adminListPath = "/dashboard/zsy/voice/list"

func TestAdminVoiceCRUD_EndToEnd(t *testing.T) {
	newVoiceTestDB(t)

	code, payload := doRequest(t, request{
		Method: http.MethodPost, Pattern: "/dashboard/zsy/voice",
		Handler: voice.TestHookCreateVoice,
		JSON: `{"name":"新音色","voiceType":"vt-new","description":"简介文本",
		        "gender":"female","ageRange":"young","scenes":["客服播报","有声书"],
		        "audioUrl":"/uploads/voices/202601/a.mp3","audioName":"a.mp3","audioSize":1234,"sortOrder":7}`,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, payload["success"], "create failed: %s", messageOf(t, payload))
	created := dataMap(t, payload)
	id := int64(created["id"].(float64))
	require.NotZero(t, id)
	assert.Equal(t, true, created["enabled"], "an omitted enabled flag means on shelf")
	assert.EqualValues(t, 7, jsonInt(t, created, "sortOrder"))
	assert.EqualValues(t, 1234, jsonInt(t, created, "audioSize"))
	assert.Equal(t, "female", created["gender"])
	assert.Equal(t, "young", created["ageRange"])
	assert.Equal(t, []any{"客服播报", "有声书"}, created["scenes"])

	detailPath := fmt.Sprintf("/dashboard/zsy/voice/%d", id)
	_, detail := doRequest(t, request{
		Method: http.MethodGet, Pattern: "/dashboard/zsy/voice/:id", Path: detailPath,
		Handler: voice.TestHookGetVoice,
	})
	assert.Equal(t, "新音色", dataMap(t, detail)["name"])

	// Partial update: only the introduction and the age range travel.
	code, updated := doRequest(t, request{
		Method: http.MethodPut, Pattern: "/dashboard/zsy/voice/:id", Path: detailPath,
		Handler: voice.TestHookUpdateVoice,
		JSON:    `{"description":"改过的简介","ageRange":"middle"}`,
	})
	require.Equal(t, http.StatusOK, code)
	updatedData := dataMap(t, updated)
	assert.Equal(t, "改过的简介", updatedData["description"])
	assert.Equal(t, "middle", updatedData["ageRange"])
	assert.Equal(t, "vt-new", updatedData["voiceType"], "untouched fields survive a partial update")
	assert.Equal(t, "female", updatedData["gender"])
	assert.Equal(t, []any{"客服播报", "有声书"}, updatedData["scenes"])

	// A scenes string is accepted as well as an array, and an empty value clears.
	_, cleared := doRequest(t, request{
		Method: http.MethodPut, Pattern: "/dashboard/zsy/voice/:id", Path: detailPath,
		Handler: voice.TestHookUpdateVoice,
		JSON:    `{"scenes":""}`,
	})
	assert.Equal(t, []any{}, dataMap(t, cleared)["scenes"])

	// Take it off the shelf and prove each face agrees.
	_, offShelf := doRequest(t, request{
		Method: http.MethodPut, Pattern: "/dashboard/zsy/voice/:id", Path: detailPath,
		Handler: voice.TestHookUpdateVoice,
		JSON:    `{"enabled":false}`,
	})
	assert.Equal(t, false, dataMap(t, offShelf)["enabled"])

	_, publicList := doRequest(t, request{
		Method: http.MethodGet, Pattern: publicListPath, Handler: voice.TestHookListPublicVoices,
	})
	assert.Zero(t, jsonInt(t, dataMap(t, publicList), "total"), "an off-shelf voice disappears from the public list")

	_, adminOffShelf := doRequest(t, request{
		Method: http.MethodGet, Pattern: adminListPath + "?enabled=false", Handler: voice.TestHookListVoicesAdmin,
	})
	adminItems := itemsOf(t, adminOffShelf)
	require.Len(t, adminItems, 1)
	assert.Equal(t, "新音色", adminItems[0]["name"])

	// Delete, then confirm both the row and its name are gone.
	code, deleted := doRequest(t, request{
		Method: http.MethodDelete, Pattern: "/dashboard/zsy/voice/:id", Path: detailPath,
		Handler: voice.TestHookDeleteVoice,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, "新音色", dataMap(t, deleted)["name"])

	_, missing := doRequest(t, request{
		Method: http.MethodGet, Pattern: "/dashboard/zsy/voice/:id", Path: detailPath,
		Handler: voice.TestHookGetVoice,
	})
	assert.Equal(t, false, missing["success"])
	assert.Equal(t, "音色不存在", messageOf(t, missing))
}

func TestAdminVoiceCreate_ReportsValidationAndDuplicateErrors(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "已存在音色", "vt-existing", 0, true)

	_, conflict := doRequest(t, request{
		Method: http.MethodPost, Pattern: "/dashboard/zsy/voice",
		Handler: voice.TestHookCreateVoice,
		JSON:    `{"name":"已存在音色","voiceType":"vt-other"}`,
	})
	assert.Equal(t, false, conflict["success"])
	assert.Contains(t, messageOf(t, conflict), "已存在")

	_, invalid := doRequest(t, request{
		Method: http.MethodPost, Pattern: "/dashboard/zsy/voice",
		Handler: voice.TestHookCreateVoice,
		JSON:    `{"name":"","voiceType":"vt-x"}`,
	})
	assert.Equal(t, false, invalid["success"])
	assert.Contains(t, messageOf(t, invalid), "音色名称不能为空")

	_, badGender := doRequest(t, request{
		Method: http.MethodPost, Pattern: "/dashboard/zsy/voice",
		Handler: voice.TestHookCreateVoice,
		JSON:    `{"name":"性别错误","voiceType":"vt-x","gender":"unknown"}`,
	})
	assert.Equal(t, false, badGender["success"])
	assert.Contains(t, messageOf(t, badGender), "非法性别")

	_, badBody := doRequest(t, request{
		Method: http.MethodPost, Pattern: "/dashboard/zsy/voice",
		Handler: voice.TestHookCreateVoice,
		JSON:    `{NOT JSON`,
	})
	assert.Equal(t, false, badBody["success"])
	assert.Contains(t, messageOf(t, badBody), "请求体错误")
}

func TestAdminVoiceList_SearchesEveryVoice(t *testing.T) {
	newVoiceTestDB(t)
	insertVoice(t, "小美", "zh_female_xiaomei", 0, true)
	insertVoice(t, "小北", "zh_male_xiaobei", 1, false)

	_, payload := doRequest(t, request{
		Method: http.MethodGet, Pattern: adminListPath, Handler: voice.TestHookListVoicesAdmin,
	})
	assert.EqualValues(t, 2, jsonInt(t, dataMap(t, payload), "total"), "the admin list includes off-shelf voices")

	_, searched := doRequest(t, request{
		Method: http.MethodGet, Pattern: adminListPath + "?keyword=xiaomei", Handler: voice.TestHookListVoicesAdmin,
	})
	items := itemsOf(t, searched)
	require.Len(t, items, 1)
	assert.Equal(t, "小美", items[0]["name"])
}

// ---------------------------------------------------------------------------
// Sample-audio upload
// ---------------------------------------------------------------------------

const uploadPath = "/dashboard/zsy/voice/upload"

// id3Sample is the smallest thing Go's content sniffer reports as audio/mpeg.
func id3Sample() []byte {
	return append([]byte("ID3\x03\x00\x00\x00\x00\x00\x00"), []byte("fake-mp3-payload")...)
}

func TestVoiceUpload_StoresAudioAndReportsItsUrl(t *testing.T) {
	newVoiceTestDB(t)
	// The handler writes under uploads/, relative to the process working
	// directory: run inside a throwaway directory so the repository stays clean.
	t.Chdir(t.TempDir())

	content := id3Sample()
	body, contentType := multipartFile(t, "sample.mp3", content)

	code, payload := doRequest(t, request{
		Method: http.MethodPost, Pattern: uploadPath, Handler: voice.TestHookUploadVoiceAudio,
		Body: body, ContentType: contentType,
	})
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, payload["success"], "upload failed: %s", messageOf(t, payload))

	data := dataMap(t, payload)
	url, _ := data["url"].(string)
	assert.True(t, strings.HasPrefix(url, "/uploads/voices/"), "unexpected url %q", url)
	assert.True(t, strings.HasSuffix(url, ".mp3"), "unexpected url %q", url)
	assert.Equal(t, "audio/mpeg", data["mimeType"])
	assert.EqualValues(t, len(content), jsonInt(t, data, "size"))
	assert.Equal(t, "sample.mp3", data["originalName"])

	// The file must really be on disk under the gateway's static root.
	stored := filepath.Join(".", filepath.FromSlash(strings.TrimPrefix(url, "/")))
	written, err := os.ReadFile(stored)
	require.NoError(t, err, "uploaded file is not reachable through /uploads")
	assert.Equal(t, content, written)
	assert.Equal(t, filepath.Base(url), data["filename"], "the stored name is randomised, not the uploaded one")
}

func TestVoiceUpload_SniffsExtensionWhenTheNameHasNone(t *testing.T) {
	newVoiceTestDB(t)
	t.Chdir(t.TempDir())

	body, contentType := multipartFile(t, "blob", id3Sample())
	_, payload := doRequest(t, request{
		Method: http.MethodPost, Pattern: uploadPath, Handler: voice.TestHookUploadVoiceAudio,
		Body: body, ContentType: contentType,
	})
	require.Equal(t, true, payload["success"], "upload failed: %s", messageOf(t, payload))
	assert.True(t, strings.HasSuffix(dataMap(t, payload)["url"].(string), ".mp3"))
}

func TestVoiceUpload_RejectsNonAudioAndOversizedFiles(t *testing.T) {
	newVoiceTestDB(t)
	workDir := t.TempDir()
	t.Chdir(workDir)

	body, contentType := multipartFile(t, "notes.txt", []byte("just some text, not audio"))
	code, payload := doRequest(t, request{
		Method: http.MethodPost, Pattern: uploadPath, Handler: voice.TestHookUploadVoiceAudio,
		Body: body, ContentType: contentType,
	})
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "不支持的音频格式")

	entries, err := os.ReadDir(workDir)
	require.NoError(t, err)
	assert.Empty(t, entries, "a rejected upload must not leave a file behind")

	oversized, oversizedType := multipartFile(t, "big.mp3", make([]byte, voice.MaxVoiceUploadBytes+(1<<20)))
	_, payload = doRequest(t, request{
		Method: http.MethodPost, Pattern: uploadPath, Handler: voice.TestHookUploadVoiceAudio,
		Body: oversized, ContentType: oversizedType,
	})
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "20MB")

	_, payload = doRequest(t, request{
		Method: http.MethodPost, Pattern: uploadPath, Handler: voice.TestHookUploadVoiceAudio,
		Body: []byte("not multipart"), ContentType: "text/plain",
	})
	assert.Equal(t, false, payload["success"])
	assert.Contains(t, messageOf(t, payload), "multipart")
}
