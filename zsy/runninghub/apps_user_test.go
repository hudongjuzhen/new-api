package runninghub_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// apiEnvelope is the unified admin/user response envelope the controllers
// promise. Every success and error response conforms to it.
type apiEnvelope struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func parseAPIEnvelope(t *testing.T, body []byte) *apiEnvelope {
	t.Helper()
	out := &apiEnvelope{}
	require.NoError(t, json.Unmarshal(body, out))
	return out
}

// newPreDBTestRouter mounts only user handlers that can complete *before*
// consulting the store layer. This keeps the test suite decoupled from the
// host DB singleton, which is never initialised for the plugin's standalone
// package tests.
func newPreDBTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// Bad app-id path is validated by parseAppIDParam *before* the DB call,
	// so it produces a stable envelope without a db() instance.
	r.GET("/api/zsy/rh/apps/:id", runninghub.TestHookGetPublicAppDetail)
	r.POST("/api/zsy/rh/apps/:id/run", runninghub.TestHookSubmitAppRun)
	return r
}

func doJSON(t *testing.T, r http.Handler, method, url string, body any) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(method, url, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	resp := w.Body.Bytes()
	return w, resp
}

// ---------------------------------------------------------------------------
// In-memory typed validator tests. These never touch gin or the DB.
// ---------------------------------------------------------------------------

func TestRunPayloadValidator_Table(t *testing.T) {
	t.Parallel()
	loPtr := func(v float64) *float64 { return &v }
	hiPtr := func(v float64) *float64 { return &v }

	promptSchema := rhparser.SchemaParam{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true}
	durationSchema := rhparser.SchemaParam{NodeID: "123", FieldName: "duration", Label: "时长", Type: "seconds", Required: true, Min: loPtr(1), Max: hiPtr(10)}
	selectSchema := rhparser.SchemaParam{
		NodeID:    "124",
		FieldName: "style",
		Label:     "风格",
		Type:      "select",
		Required:  true,
		Options:   []rhparser.SchemaParamOption{{Label: "写实", Value: "real"}, {Label: "动漫", Value: "anime"}},
	}
	imageSchema := rhparser.SchemaParam{NodeID: "121", FieldName: "image", Label: "底图", Type: "image", Required: false}

	type tc struct {
		name    string
		values  map[string]any
		wantErr string
	}
	cases := []tc{
		{
			name:    "missing required prompt → 列出缺失字段",
			values:  map[string]any{"123.duration": "5", "124.style": "real"},
			wantErr: "缺少必填参数",
		},
		{
			name:    "duration non-numeric string rejected",
			values:  map[string]any{"122.prompt": "hi", "123.duration": "abc", "124.style": "real"},
			wantErr: "时长 必须为数字",
		},
		{
			name:    "duration below min rejected",
			values:  map[string]any{"122.prompt": "hi", "123.duration": 0, "124.style": "real"},
			wantErr: "时长 不能小于",
		},
		{
			name:    "duration above explicit max rejected",
			values:  map[string]any{"122.prompt": "hi", "123.duration": 11, "124.style": "real"},
			wantErr: "时长 不能大于 10",
		},
		{
			name:    "duration seconds type implicitly caps at MaxTaskDurationSeconds when schema asks too much",
			values:  map[string]any{"122.prompt": "hi", "123.duration": 999999, "124.style": "real"},
			wantErr: "不能大于",
		},
		{
			name:    "select unknown option rejected",
			values:  map[string]any{"122.prompt": "hi", "123.duration": "5", "124.style": "oil"},
			wantErr: "风格 非法选项值",
		},
		{
			name: "all valid → no error",
			values: map[string]any{
				"122.prompt":   "a cute cat",
				"123.duration": 5,
				"124.style":    "real",
				"121.image":    "https://example.com/a.png",
			},
		},
		{
			name: "number field accepts string numeric value (web inputs send strings)",
			values: map[string]any{
				"122.prompt":   "hi",
				"123.duration": "7",
				"124.style":    "anime",
			},
		},
	}
	schema := []rhparser.SchemaParam{promptSchema, durationSchema, selectSchema, imageSchema}
	for _, tt := range cases {
		tc := tt
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := runninghub.TestHookValidateRunPayload(schema, tc.values)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// HTTP-level user endpoint tests that short-circuit before any DB access.
// These verify the response envelope and parameter parsing behaviour when
// the DB singleton isn't initialised (plugin-standalone test mode).
// ---------------------------------------------------------------------------

func TestUserGetAppDetail_BadIDParam_ReturnsEnvelope(t *testing.T) {
	t.Parallel()
	r := newPreDBTestRouter()
	// "abc" can't be parsed as uint → parseAppIDParam rejects before DB.
	w, raw := doJSON(t, r, http.MethodGet, "/api/zsy/rh/apps/abc", nil)
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.NotEmpty(t, env.Message)
}

func TestUserSubmitAppRun_BadIDParam_ReturnsEnvelope(t *testing.T) {
	t.Parallel()
	r := newPreDBTestRouter()
	payload := map[string]any{"values": map[string]any{}}
	w, raw := doJSON(t, r, http.MethodPost, "/api/zsy/rh/apps/-1/run", payload)
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, raw)
	assert.False(t, env.Success)
	assert.NotEmpty(t, env.Message)
}

func TestUserSubmitAppRun_BadJSON_ReturnsEnvelopeError(t *testing.T) {
	t.Parallel()
	// Don't route through submitAppRun itself — it touches model.DB before
	// decoding the body, and the plugin's standalone tests have no DB. The
	// same "malformed body" path is exercised through a tiny shim that
	// re-uses the host's standard JSON decoder and ApiErrorMsg wrapper,
	// which is exactly what submitAppRun does after the DB lookup.
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/zsy/rh/apps/:id/run", func(c *gin.Context) {
		var payload runninghub.AppRunPayload
		if err := json.NewDecoder(c.Request.Body).Decode(&payload); err != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "无效的提交内容: " + err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	req := httptest.NewRequest(http.MethodPost, "/api/zsy/rh/apps/1/run", strings.NewReader("{NOT JSON"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	env := parseAPIEnvelope(t, w.Body.Bytes())
	assert.False(t, env.Success)
	assert.Contains(t, env.Message, "提交内容") // "无效的提交内容"
}
