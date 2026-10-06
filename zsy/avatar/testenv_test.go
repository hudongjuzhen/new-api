package avatar_test

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
	"github.com/QuantumNous/new-api/zsy/avatar"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newAvatarTestDB swaps the host connection for a per-test in-memory SQLite
// database carrying the plugin's own table plus the voice catalog's, and restores
// every global it touched when the test ends. It deliberately avoids t.Parallel:
// model.DB is a process-wide global.
func newAvatarTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	origDB := model.DB
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:avatar_test_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	// One connection keeps the shared in-memory database alive for the whole test
	// even when GORM drops an idle connection.
	sqlDB.SetMaxOpenConns(1)
	model.DB = db

	t.Cleanup(func() {
		model.DB = origDB
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(&avatar.Avatar{}))
	require.NoError(t, db.Exec(voiceCatalogDDL).Error)
	return db
}

// voiceCatalogDDL creates the 音色广场 table the sample lookup reads. The link is
// a table+column contract between two independently installable plugins, so the
// tests build the table from the catalog's own schema rather than importing the
// voice package: a rename there must break these tests loudly.
const voiceCatalogDDL = `CREATE TABLE IF NOT EXISTS zsy_voices (
	id integer PRIMARY KEY AUTOINCREMENT,
	created_at integer,
	updated_at integer,
	name varchar(191) NOT NULL,
	description text,
	voice_type varchar(191) NOT NULL,
	gender varchar(16),
	age_range varchar(16),
	language varchar(16),
	scenes varchar(255),
	avatar_url varchar(768),
	audio_url varchar(768),
	audio_name varchar(255),
	audio_size integer NOT NULL DEFAULT 0,
	enabled numeric,
	sort_order integer NOT NULL DEFAULT 0
)`

// insertCatalogVoice seeds one voice-catalog row, which is what a persona's
// voiceId resolves against.
func insertCatalogVoice(t *testing.T, db *gorm.DB, name string, voiceType string, audioURL string, audioName string) {
	t.Helper()
	require.NoError(t, db.Exec(
		`INSERT INTO zsy_voices (name, voice_type, audio_url, audio_name, enabled) VALUES (?, ?, ?, ?, ?)`,
		name, voiceType, audioURL, audioName, true).Error)
}

// insertAvatar seeds one row through the production create path.
func insertAvatar(t *testing.T, name string, sortOrder int, enabled bool) *avatar.AvatarView {
	t.Helper()
	view, err := avatar.AvatarInsert(&avatar.AvatarCreateDTO{
		Name:      name,
		ImageURL:  "https://cdn.example.com/persona/" + name + ".png",
		SortOrder: &sortOrder,
		Enabled:   &enabled,
	})
	require.NoError(t, err)
	return view
}

// insertRichAvatar seeds the fixture carrying every optional attribute (the four
// pictures, gender, age range, ethnicity, suitable scenes, voice id), so a test
// can assert the whole documented row shape. It sorts right after the first plain
// fixture.
func insertRichAvatar(t *testing.T) *avatar.AvatarView {
	t.Helper()
	sortOrder := 1
	view, err := avatar.AvatarInsert(&avatar.AvatarCreateDTO{
		Name:          "客服小雨",
		Description:   "温柔的客服形象",
		ImageURL:      "https://cdn.example.com/persona/xiaoyu.png",
		FullBodyURL:   "https://cdn.example.com/persona/xiaoyu-full.png",
		FourViewURL:   "https://cdn.example.com/persona/xiaoyu-four-view.png",
		ExpressionURL: "https://cdn.example.com/persona/xiaoyu-expression.png",
		Gender:        "female",
		AgeRange:      "young",
		Race:          "asian",
		Scenes:        []string{"客服播报", "有声书"},
		VoiceID:       "zh_female_vv_uranus_bigtts",
		SortOrder:     &sortOrder,
	})
	require.NoError(t, err)
	return view
}

// request describes one test request. Pattern is the route to mount and may carry
// a query string (it is mounted without it and requested with it); Path defaults
// to Pattern and must be filled in when the pattern has a :param.
type request struct {
	Method      string
	Pattern     string
	Path        string
	Handler     gin.HandlerFunc
	JSON        string
	Body        []byte
	ContentType string
}

// doRequest mounts exactly one production handler on a bare router and decodes the
// JSON envelope it answers with.
func doRequest(t *testing.T, spec request) (int, map[string]any) {
	t.Helper()
	status, _, body := doRawRequest(t, spec)
	return status, decodeEnvelope(t, body)
}

// doRawRequest mounts exactly one production handler on a bare router and returns
// the status, headers and raw body — for the endpoints that answer with a file
// rather than an envelope.
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

// multipartFile builds a multipart body carrying one `file` field — the field name
// both the CSV import and the image upload read.
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

// itemNames reads items[].name in order, which is how the ordering assertions
// state their expectation.
func itemNames(t *testing.T, payload map[string]any) []string {
	t.Helper()
	items := itemsOf(t, payload)
	names := make([]string, 0, len(items))
	for _, item := range items {
		name, _ := item["name"].(string)
		names = append(names, name)
	}
	return names
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
