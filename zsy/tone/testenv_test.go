package tone_test

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
	"github.com/QuantumNous/new-api/zsy/tone"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newToneTestDB swaps the host connection for a per-test in-memory SQLite
// database carrying only the plugin's table, and restores every global it
// touched when the test ends. It deliberately avoids t.Parallel: model.DB is a
// process-wide global.
func newToneTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	origDB := model.DB
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:tone_test_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
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

	require.NoError(t, db.AutoMigrate(&tone.Tone{}))
	return db
}

// insertTone seeds one row through the production create path. Prompt is
// required by the validator, so the fixture takes it as an argument rather than
// defaulting it: a helper that silently filled it in would let a test assert on
// a shape the API can never actually store.
func insertTone(t *testing.T, name string, prompt string, sortOrder int, enabled bool) *tone.ToneView {
	t.Helper()
	view, err := tone.ToneInsert(&tone.ToneCreateDTO{
		Name:      name,
		Prompt:    prompt,
		SortOrder: &sortOrder,
		Enabled:   &enabled,
	})
	require.NoError(t, err)
	return view
}

// insertRichTone seeds the fixture carrying every optional attribute (category,
// tone, language, suitable scenes, the worked example pair), so a test can
// assert the whole documented row shape. It sorts right after the first plain
// fixture.
func insertRichTone(t *testing.T) *tone.ToneView {
	t.Helper()
	sortOrder := 1
	view, err := tone.ToneInsert(&tone.ToneCreateDTO{
		Name:        "克制的长文",
		Description: "适合公众号长文，情绪收着写",
		Prompt:      "以克制、平视的笔调写作：多用短句，少用形容词，让细节承载情绪，不要替读者下结论。",
		Category:    "literary",
		Tone:        "calm",
		Language:    "zh",
		Scenes:      []string{"公众号长文", "散文"},
		SampleInput: sharedSampleInput,
		SampleOutput: "老屋在东头。院子里那棵枣树还在，秋天照例结一树果子，没人去数。" +
			"石阶被踩得发亮，中间那道凹痕，比记忆里又深了一点。",
		SortOrder: &sortOrder,
	})
	require.NoError(t, err)
	return view
}

// sharedSampleInput is the passage the worked-example convention expects every
// seeded tone to reuse (see zsy/tone/standard.go). Tests that assert the
// convention reuse it instead of pasting their own text, so a change to the
// canonical passage is a one-line change.
const sharedSampleInput = "老屋在村子的东头，是祖父留下的。院子里的那棵枣树还在，" +
	"每年秋天结一树果子，没有人数过有多少。屋檐下的石阶被踩得发亮，中间有一道浅浅的凹痕。"

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
// name the CSV import reads.
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

// jsonStrings reads a JSON array of strings (the `scenes` field).
func jsonStrings(t *testing.T, m map[string]any, key string) []string {
	t.Helper()
	raw, ok := m[key].([]any)
	require.True(t, ok, "%s missing or not an array in %v", key, m)
	out := make([]string, 0, len(raw))
	for _, entry := range raw {
		s, ok := entry.(string)
		require.True(t, ok, "%s holds a non-string: %v", key, entry)
		out = append(out, s)
	}
	return out
}

// namesOf lists the `name` of every item, in response order.
func namesOf(t *testing.T, payload map[string]any) []string {
	t.Helper()
	items := itemsOf(t, payload)
	out := make([]string, 0, len(items))
	for _, item := range items {
		name, _ := item["name"].(string)
		out = append(out, name)
	}
	return out
}

// messageOf reads the envelope message.
func messageOf(t *testing.T, payload map[string]any) string {
	t.Helper()
	msg, _ := payload["message"].(string)
	return msg
}
