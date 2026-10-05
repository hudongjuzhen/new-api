package voice_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/zsy/voice"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newVoiceTestDB swaps the host connection for a per-test in-memory SQLite
// database carrying only the plugin's table, and restores every global it
// touched when the test ends. It deliberately avoids t.Parallel: model.DB is a
// process-wide global.
func newVoiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	origDB := model.DB
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:voice_test_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// One connection keeps the shared in-memory database alive for the whole
	// test even when GORM drops an idle connection.
	sqlDB.SetMaxOpenConns(1)
	model.DB = db

	t.Cleanup(func() {
		model.DB = origDB
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(&voice.Voice{}))
	return db
}

// insertVoice seeds one row through the production create path.
func insertVoice(t *testing.T, name string, voiceType string, sortOrder int, enabled bool) *voice.VoiceView {
	t.Helper()
	view, err := voice.VoiceInsert(&voice.VoiceCreateDTO{
		Name:      name,
		VoiceType: voiceType,
		SortOrder: &sortOrder,
		Enabled:   &enabled,
	})
	require.NoError(t, err)
	return view
}

// insertRichVoice seeds the fixture carrying every optional attribute (gender,
// age range, language, suitable scenes, avatar, sample audio), so a test can
// assert the whole documented row shape. It sorts right after the first plain
// fixture.
func insertRichVoice(t *testing.T) *voice.VoiceView {
	t.Helper()
	sortOrder := 1
	view, err := voice.VoiceInsert(&voice.VoiceCreateDTO{
		Name:        "客服女声",
		Description: "温柔女声，适合客服播报",
		VoiceType:   "zh_female_kefu",
		Gender:      "female",
		AgeRange:    "young",
		Language:    "zh",
		Scenes:      []string{"客服播报", "有声书"},
		AvatarURL:   "https://cdn.example.com/avatar/kefu.png",
		AudioURL:    "/uploads/voices/202601/kefu.mp3",
		AudioName:   "kefu.mp3",
		AudioSize:   4096,
		SortOrder:   &sortOrder,
	})
	require.NoError(t, err)
	return view
}

// request describes one test request. Pattern is the route to mount and may
// carry a query string (it is mounted without it and requested with it); Path
// defaults to Pattern and must be filled in when the pattern has a :param.
type request struct {
	Method      string
	Pattern     string
	Path        string
	Handler     gin.HandlerFunc
	JSON        string
	Body        []byte
	ContentType string
}

// doRequest mounts exactly one production handler on a bare router and decodes
// the JSON envelope it answers with.
func doRequest(t *testing.T, spec request) (int, map[string]any) {
	t.Helper()
	status, _, body := doRawRequest(t, spec)
	return status, decodeEnvelope(t, body)
}

// doRawRequest mounts exactly one production handler on a bare router and
// returns the status, headers and raw body — for the endpoints that answer with
// a file rather than an envelope.
func doRawRequest(t *testing.T, spec request) (int, http.Header, []byte) {
	t.Helper()

	pattern := spec.Pattern
	if i := strings.Index(pattern, "?"); i >= 0 {
		pattern = pattern[:i]
	}
	router := gin.New()
	router.Handle(spec.Method, pattern, spec.Handler)

	path := spec.Path
	if path == "" {
		path = spec.Pattern
	}
	body, contentType := spec.Body, spec.ContentType
	if spec.JSON != "" {
		body, contentType = []byte(spec.JSON), "application/json"
	}

	req := httptest.NewRequest(spec.Method, path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	return w.Code, w.Header(), w.Body.Bytes()
}

// decodeEnvelope decodes a JSON response body into its envelope map.
func decodeEnvelope(t *testing.T, body []byte) map[string]any {
	t.Helper()
	payload := map[string]any{}
	require.NoError(t, json.Unmarshal(body, &payload), "response is not JSON: %s", body)
	return payload
}

// multipartFile builds a multipart body carrying one `file` field — the field
// name both the audio upload and the CSV import read.
func multipartFile(t *testing.T, filename string, content []byte) ([]byte, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	return body.Bytes(), writer.FormDataContentType()
}

// dataMap reads the `data` object of an envelope.
func dataMap(t *testing.T, payload map[string]any) map[string]any {
	t.Helper()
	data, ok := payload["data"].(map[string]any)
	require.True(t, ok, "data missing in %v", payload)
	return data
}

// itemsOf reads data.items as a list of objects.
func itemsOf(t *testing.T, payload map[string]any) []map[string]any {
	t.Helper()
	raw, ok := dataMap(t, payload)["items"].([]any)
	require.True(t, ok, "items missing in %v", payload)
	items := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		item, ok := entry.(map[string]any)
		require.True(t, ok, "item is not an object: %v", entry)
		items = append(items, item)
	}
	return items
}

// jsonInt reads an integer-valued JSON number.
func jsonInt(t *testing.T, m map[string]any, key string) int64 {
	t.Helper()
	v, ok := m[key].(float64)
	require.True(t, ok, "%s missing or not a number in %v", key, m)
	return int64(v)
}

// messageOf reads the envelope message.
func messageOf(t *testing.T, payload map[string]any) string {
	t.Helper()
	msg, _ := payload["message"].(string)
	return msg
}
