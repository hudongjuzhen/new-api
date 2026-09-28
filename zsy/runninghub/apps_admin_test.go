package runninghub_test

import (
	"bytes"
	"encoding/json"
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

// ---------------------------------------------------------------------------
// Unit tests for the store-layer validator. These bounce through
// applyDto+validate by constructing a DTO equivalent to the desired App and
// calling TestHookValidateAppCreate, which runs the same validator AppInsert
// uses but short-circuits before the database write. No gorm/sqlite required.
// ---------------------------------------------------------------------------

func TestValidateApp_Invariants(t *testing.T) {
	t.Parallel()
	type tc struct {
		name    string
		mutator func(dto *runninghub.AppCreateDTO)
		wantErr string
	}
	cases := []tc{
		{
			name:    "empty name rejected",
			mutator: func(d *runninghub.AppCreateDTO) { d.Name = "" },
			wantErr: "名称不能为空",
		},
		{
			name:    "bad kind rejected",
			mutator: func(d *runninghub.AppCreateDTO) { d.Kind = "weird" },
			wantErr: "非法应用 kind",
		},
		{
			name:    "empty upstreamId rejected",
			mutator: func(d *runninghub.AppCreateDTO) { d.UpstreamID = "" },
			wantErr: "upstreamId 不能为空",
		},
		{
			name:    "negative fixed quota rejected",
			mutator: func(d *runninghub.AppCreateDTO) { d.FixedQuotaPerCall = -3 },
			wantErr: "fixedQuotaPerCall 不能为负数",
		},
		{
			name:    "zero rate ratio rejected (explicit 0 falls back to 1.0)",
			mutator: func(d *runninghub.AppCreateDTO) { d.ModelBaseRateRatio = -1 },
			wantErr: "modelBaseRateRatio 必须为正数",
		},
		{
			name: "per-call and per-second mutually exclusive",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerSecondBilling = true
				d.QuotaPerSecond = 100
			},
			wantErr: "互斥",
		},
		{
			name: "per-second without positive quota rejected",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.PerSecondBilling = true
				d.QuotaPerSecond = 0
			},
			wantErr: "每单位秒额度",
		},
		{
			name: "negative per-second quota rejected",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.QuotaPerSecond = -5
			},
			wantErr: "quotaPerSecond 不能为负数",
		},
		{
			name: "valid per-second passes",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.PerSecondBilling = true
				d.QuotaPerSecond = 2_000
			},
			wantErr: "",
		},
		{
			name: "per-call and per-char mutually exclusive",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCharBilling = true
				d.QuotaPerChar = 10
				d.CharCountExpr = "len(122)"
			},
			wantErr: "互斥",
		},
		{
			name: "per-second and per-char mutually exclusive",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.PerSecondBilling = true
				d.QuotaPerSecond = 100
				d.PerCharBilling = true
				d.QuotaPerChar = 10
				d.CharCountExpr = "len(122)"
			},
			wantErr: "互斥",
		},
		{
			name: "per-char without a positive quota rejected",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.PerCharBilling = true
				d.QuotaPerChar = 0
				d.CharCountExpr = "len(122)"
			},
			wantErr: "每字符额度",
		},
		{
			name: "negative per-char quota rejected",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.QuotaPerChar = -5
			},
			wantErr: "quotaPerChar 不能为负数",
		},
		{
			name: "per-char without a character field rejected",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.PerCharBilling = true
				d.QuotaPerChar = 10
				d.CharCountExpr = ""
			},
			wantErr: "字符数字段",
		},
		{
			name: "malformed character field expression rejected",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.PerCharBilling = true
				d.QuotaPerChar = 10
				d.CharCountExpr = "len(122"
			},
			wantErr: "右括号",
		},
		{
			name: "valid per-char passes",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.PerCallBilling = false
				d.PerCharBilling = true
				d.QuotaPerChar = 10
				d.CharCountExpr = "len(122)"
			},
			wantErr: "",
		},
		{
			name:    "name 200 chars rejected",
			mutator: func(d *runninghub.AppCreateDTO) { d.Name = strings.Repeat("x", 200) },
			wantErr: "名称过长",
		},
		{
			name:    "valid ai_app passes",
			mutator: func(d *runninghub.AppCreateDTO) { /* noop */ },
			wantErr: "",
		},
		{
			name: "workflow kind passes",
			mutator: func(d *runninghub.AppCreateDTO) {
				d.Kind = runninghub.AppKindWorkflow
				d.UpstreamID = "wf_abc"
			},
			wantErr: "",
		},
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			dto := validAppDTO(t)
			c.mutator(&dto)
			err := runninghub.TestHookValidateAppCreate(&dto)
			if c.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), c.wantErr, "error=%v", err)
		})
	}
}

func validAppDTO(t *testing.T) runninghub.AppCreateDTO {
	t.Helper()
	return runninghub.AppCreateDTO{
		Name:               "测试 App",
		Slug:               "test-app",
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         "1877265245566922753",
		Description:        "desc",
		Published:          true,
		AdminOnly:          false,
		PerCallBilling:     true,
		FixedQuotaPerCall:  1_000_000,
		ModelBaseRateRatio: 1.0,
		ParamSchema: []rhparser.SchemaParam{
			{NodeID: "122", FieldName: "prompt", Type: "textarea", Required: true, Label: "提示词"},
		},
	}
}

// ---------------------------------------------------------------------------
// HTTP handler tests for parseCurlEndpoint via httptest.
// Auth is intentionally bypassed to test the parser + response envelope only.
// ---------------------------------------------------------------------------

func TestParseCurlEndpoint_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/parse-curl", runninghub.TestHookParseCurl)
	body := map[string]any{
		"curl": `curl --location 'https://www.runninghub.cn/openapi/v2/run/ai-app/1877265245566922753' ` +
			`-H 'Authorization: Bearer x' ` +
			`--data '{"nodeInfoList":[{"nodeId":"122","fieldName":"prompt","fieldValue":"a cat"}]}'`,
	}
	b, err := json.Marshal(body)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/parse-curl", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, true, resp["success"], "success must be true; body=%s", w.Body.String())
	data, ok := resp["data"].(map[string]any)
	require.True(t, ok, "resp.data missing; body=%s", w.Body.String())
	assert.Equal(t, "ai_app", data["kind"])
	assert.Equal(t, "1877265245566922753", data["upstreamId"])
	assert.Equal(t, "https://www.runninghub.cn", data["baseUrl"])
	nodes, ok := data["nodeInfoList"].([]any)
	require.True(t, ok)
	require.Len(t, nodes, 1)
	first := nodes[0].(map[string]any)
	assert.Equal(t, "122", first["nodeId"])
	assert.Equal(t, "prompt", first["fieldName"])
	assert.Equal(t, "a cat", first["fieldValue"])

	schema, ok := data["schema"].([]any)
	require.True(t, ok, "schema missing")
	assert.GreaterOrEqual(t, len(schema), 1, "draft schema should include the prompt param")
}

// The two node shapes the /rh-apps "Request Example" import has to cope with,
// taken verbatim from a real RunningHub curl: an aspect-ratio COMBO whose
// fieldData carries the choices plus the default, and a boolean node that has
// no fieldData at all. The drafted schema is what the admin form then edits and
// saves, so its inferred types are the contract.
func TestParseCurlEndpoint_SelectAndSwitchNodesAreDrafted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/parse-curl", runninghub.TestHookParseCurl)

	curl := `curl --location --request POST 'https://www.runninghub.cn/openapi/v2/run/ai-app/2051268528824700930' ` +
		`--header "Content-Type: application/json" ` +
		`--header "Authorization: Bearer x" ` +
		`--data-raw '{"nodeInfoList":[` +
		`{"nodeId":"13","fieldName":"aspect_ratio",` +
		`"fieldData":"[\"COMBO\", {\"default\": \"1:1 (Square)\", \"options\": [\"1:1 (Square)\", \"2:3 (Portrait Photo)\", \"3:2 (Photo)\", \"3:4 (Portrait Standard)\", \"4:3 (Standard)\", \"9:16 (Portrait Widescreen)\", \"16:9 (Widescreen)\", \"21:9 (Ultrawide)\"], \"tooltip\": \"The aspect ratio for the output dimensions.\", \"multiselect\": false}]",` +
		`"fieldValue":"16:9 (Widescreen)","description":null},` +
		`{"nodeId":"256","fieldName":"value","fieldValue":"false","description":null}` +
		`],"instanceType":"default","usePersonalQueue":"false"}'`

	b, err := json.Marshal(map[string]string{"curl": curl})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/parse-curl", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Schema []rhparser.SchemaParam `json:"schema"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp), "body: %s", w.Body.String())
	require.True(t, resp.Success, "body: %s", w.Body.String())
	require.Len(t, resp.Data.Schema, 2)

	ratio := resp.Data.Schema[0]
	assert.Equal(t, "select", ratio.Type)
	assert.Equal(t, "aspect_ratio", ratio.FieldName)
	assert.Equal(t, "1:1 (Square)", ratio.Default)
	assert.Equal(t, "The aspect ratio for the output dimensions.", ratio.Placeholder)
	require.Len(t, ratio.Options, 8)
	assert.Equal(t, "21:9 (Ultrawide)", ratio.Options[7].Value)

	toggle := resp.Data.Schema[1]
	assert.Equal(t, "switch", toggle.Type)
	assert.Equal(t, "value", toggle.FieldName)
	assert.Equal(t, "false", toggle.Default)
	assert.Empty(t, toggle.Options)
}

func TestParseCurlEndpoint_EmptyCurl_Fails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/parse-curl", runninghub.TestHookParseCurl)

	body, _ := json.Marshal(map[string]string{"curl": ""})
	req := httptest.NewRequest(http.MethodPost, "/parse-curl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	msg, _ := resp["message"].(string)
	assert.Contains(t, msg, "不能为空")
}

func TestParseCurlEndpoint_MalformedCurl_Fails(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/parse-curl", runninghub.TestHookParseCurl)

	body, _ := json.Marshal(map[string]string{"curl": "wget https://example.com/"})
	req := httptest.NewRequest(http.MethodPost, "/parse-curl", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
}

func TestParseCurlEndpoint_CreateWithCurlField_Rejected(t *testing.T) {
	// createApp rejects payloads that still carry a "curl" field, to force
	// admins to run /parse-curl and actually decide on the schema first.
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/apps", runninghub.TestHookCreateApp)

	dto := validAppDTO(t)
	payload := map[string]any{
		"curl": `curl 'https://www.runninghub.cn/openapi/v2/run/ai-app/1' -d '{}'`,
		"name": dto.Name, "kind": dto.Kind, "upstreamId": dto.UpstreamID,
		"fixedQuotaPerCall":  dto.FixedQuotaPerCall,
		"perCallBilling":     dto.PerCallBilling,
		"modelBaseRateRatio": dto.ModelBaseRateRatio,
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/apps", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["success"])
	msg, _ := resp["message"].(string)
	assert.Contains(t, msg, "/parse-curl")
}
