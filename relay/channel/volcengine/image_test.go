package volcengine

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 方舟的组图、提示词优化、联网搜索等参数在 dto.ImageRequest 里没有对应字段，
// 只能经由 ImageRequest.Extra 进来；这些参数必须出现在发给上游的请求体里，
// 否则上游按默认值只返回一张图，客户端请求的组图会被静默降级。
func TestForwardImageRequestKeepsArkOnlyParameters(t *testing.T) {
	clientBody := `{
		"model": "doubao-seedream-5-0-260128",
		"prompt": "三张同一只橘猫的组图",
		"size": "2K",
		"response_format": "url",
		"watermark": false,
		"sequential_image_generation": "auto",
		"sequential_image_generation_options": {"max_images": 3},
		"optimize_prompt_options": {"mode": "standard"},
		"tools": [{"type": "web_search"}]
	}`
	var request dto.ImageRequest
	require.NoError(t, common.UnmarshalJsonStr(clientBody, &request))
	require.Contains(t, request.Extra, "sequential_image_generation")

	body, err := forwardImageRequest(nil, request)
	require.NoError(t, err)
	encoded, err := common.Marshal(body)
	require.NoError(t, err)

	var out map[string]json.RawMessage
	require.NoError(t, common.Unmarshal(encoded, &out))

	assert.JSONEq(t, `"auto"`, string(out["sequential_image_generation"]))
	assert.JSONEq(t, `{"max_images":3}`, string(out["sequential_image_generation_options"]))
	assert.JSONEq(t, `{"mode":"standard"}`, string(out["optimize_prompt_options"]))
	assert.JSONEq(t, `[{"type":"web_search"}]`, string(out["tools"]))
	// 已声明字段仍然照常发出。
	assert.JSONEq(t, `"2K"`, string(out["size"]))
	assert.JSONEq(t, `false`, string(out["watermark"]))
	assert.JSONEq(t, `"doubao-seedream-5-0-260128"`, string(out["model"]))
}

func TestForwardImageRequestDeclaredFieldWinsOverExtra(t *testing.T) {
	request := dto.ImageRequest{
		Model:  "doubao-seedream-5-0-260128",
		Prompt: "cat",
		Size:   "2K",
		Extra: map[string]json.RawMessage{
			"size": json.RawMessage(`"4K"`),
		},
	}

	body, err := forwardImageRequest(nil, request)
	require.NoError(t, err)

	assert.JSONEq(t, `"2K"`, string(body["size"]))
}

func TestForwardImageRequestWithoutExtraKeepsStandardFields(t *testing.T) {
	clientBody := `{"model":"doubao-seedream-5-0-260128","prompt":"cat","size":"2048x2048"}`
	var request dto.ImageRequest
	require.NoError(t, common.UnmarshalJsonStr(clientBody, &request))
	require.Empty(t, request.Extra)

	body, err := forwardImageRequest(nil, request)
	require.NoError(t, err)

	encoded, err := common.Marshal(body)
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"model":"doubao-seedream-5-0-260128","prompt":"cat","size":"2048x2048"}`,
		string(encoded))
}

// ConvertImageRequest 返回的不再是 dto.ImageRequest，调用方无法再从类型推断出
// 转换链；适配器必须自己补记，日志里的 request_conversion 才不会丢。
func TestConvertImageRequestRecordsConversionChain(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesGenerations}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{
		Model:  "doubao-seedream-5-0-260128",
		Prompt: "cat",
		Size:   "2K",
	})
	require.NoError(t, err)

	_, isBodyMap := converted.(map[string]json.RawMessage)
	require.True(t, isBodyMap)
	assert.Contains(t, info.RequestConversionChain, types.RelayFormat(types.RelayFormatOpenAIImage))
}

// 方舟的图生图与文生图共用 /api/v3/images/generations，edits 也必须走到同一个
// 上游请求体（当前 edits 分支尚无 multipart 处理，走 default 分支即可）。
func TestConvertImageRequestEditsUsesSameUpstreamBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{RelayMode: relayconstant.RelayModeImagesEdits}

	converted, err := (&Adaptor{}).ConvertImageRequest(c, info, dto.ImageRequest{
		Model:  "doubao-seedream-5-0-260128",
		Prompt: "把背景换成海边",
		Image:  json.RawMessage(`"https://example.com/in.png"`),
	})
	require.NoError(t, err)

	body, isBodyMap := converted.(map[string]json.RawMessage)
	require.True(t, isBodyMap)
	assert.JSONEq(t, `"https://example.com/in.png"`, string(body["image"]))
}
