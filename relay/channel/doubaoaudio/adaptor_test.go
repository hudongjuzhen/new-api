package doubaoaudio

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestGetRequestURLTargetsDoubaoSpeechNotArk pins the endpoint that the whole
// feature broke on: seed-audio-1.0 exists only on 豆包语音, and sending it to
// Ark's task endpoint answers InvalidEndpointOrModel.NotFound.
func TestGetRequestURLTargetsDoubaoSpeechNotArk(t *testing.T) {
	adaptor := &Adaptor{}

	url, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeAudioGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{},
	})
	require.NoError(t, err)
	assert.Equal(t, DefaultBaseURL+"/api/v3/tts/create", url)
	assert.Contains(t, url, "openspeech.bytedance.com")
	assert.NotContains(t, url, "ark.cn-beijing.volces.com")
	assert.NotContains(t, url, "contents/generations/tasks")

	// 渠道自定义 base URL 时只替换主机，路径不变。
	custom, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeAudioGenerations,
		ChannelMeta: &relaycommon.ChannelMeta{ChannelBaseUrl: "https://relay.example.com/"},
	})
	require.NoError(t, err)
	assert.Equal(t, "https://relay.example.com/api/v3/tts/create", custom)
}

// TestGetRequestURLRejectsNonAudioModes keeps this adaptor from answering for
// modes it does not implement, which would silently post a wrong body shape.
func TestGetRequestURLRejectsNonAudioModes(t *testing.T) {
	adaptor := &Adaptor{}
	_, err := adaptor.GetRequestURL(&relaycommon.RelayInfo{
		RelayMode:   relayconstant.RelayModeChatCompletions,
		ChannelMeta: &relaycommon.ChannelMeta{},
	})
	assert.Error(t, err)
}

// TestSetupRequestHeaderUsesApiKeyHeader fixes the auth contract: openspeech
// authenticates with X-Api-Key and rejects the Ark-style Authorization header.
func TestSetupRequestHeaderUsesApiKeyHeader(t *testing.T) {
	adaptor := &Adaptor{}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/generations", nil)
	c.Request.Header.Set("Content-Type", "application/json")

	header := http.Header{}
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiKey: "audio-key"}}
	require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))

	assert.Equal(t, "audio-key", header.Get("X-Api-Key"))
	assert.Equal(t, "application/json", header.Get("Content-Type"))
	assert.Empty(t, header.Get("Authorization"))
}

func convertForTest(t *testing.T, request dto.AudioGenerationRequest) dto.AudioGenerationRequest {
	t.Helper()
	adaptor := &Adaptor{}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{
		OriginModelName: "seed-audio-1.0",
		ChannelMeta:     &relaycommon.ChannelMeta{UpstreamModelName: "seed-audio-1.0"},
	}

	converted, err := adaptor.ConvertAudioGenerationRequest(c, info, request)
	require.NoError(t, err)
	payload, ok := converted.(dto.AudioGenerationRequest)
	require.True(t, ok)
	return payload
}

// TestConvertKeepsUpstreamFieldNames proves the built body uses the documented
// names (text_prompt / references / audio_config) rather than the chat-style
// prompt/metadata the previous task-based implementation sent.
func TestConvertKeepsUpstreamFieldNames(t *testing.T) {
	rate := 24000
	payload := convertForTest(t, dto.AudioGenerationRequest{
		TextPrompt:  "  深夜的废弃工厂，雨滴打在铁皮屋顶上  ",
		AudioConfig: &dto.AudioGenerationConfig{Format: "mp3", SampleRate: &rate},
	})

	assert.Equal(t, "seed-audio-1.0", payload.Model)
	assert.Equal(t, "深夜的废弃工厂，雨滴打在铁皮屋顶上", payload.TextPrompt, "prompt must be trimmed")
	require.NotNil(t, payload.AudioConfig)
	assert.Equal(t, "mp3", payload.AudioConfig.Format)
	assert.Equal(t, 24000, *payload.AudioConfig.SampleRate)

	body, err := common.Marshal(payload)
	require.NoError(t, err)
	assert.Contains(t, string(body), `"text_prompt"`)
	assert.Contains(t, string(body), `"audio_config"`)
	assert.NotContains(t, string(body), `"messages"`)
	assert.NotContains(t, string(body), `"metadata"`)
}

// TestConvertRejectsEmptyPrompt keeps a request that upstream would refuse out
// of the billing chain.
func TestConvertRejectsEmptyPrompt(t *testing.T) {
	adaptor := &Adaptor{}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	info := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}

	_, err := adaptor.ConvertAudioGenerationRequest(c, info, dto.AudioGenerationRequest{TextPrompt: "   "})
	assert.Error(t, err)

	tooLong := strings.Repeat("字", dto.MaxAudioGenerationPromptChars+1)
	_, err = adaptor.ConvertAudioGenerationRequest(c, info, dto.AudioGenerationRequest{TextPrompt: tooLong})
	assert.Error(t, err)
}

// TestConvertNormalizesAudioConfig pins the format/sample-rate compatibility
// rule: ogg_opus accepts 48000 only, so an incompatible rate must be replaced
// rather than sent (upstream rejects the whole request) or silently kept.
func TestConvertNormalizesAudioConfig(t *testing.T) {
	unsupported := 44100
	payload := convertForTest(t, dto.AudioGenerationRequest{
		TextPrompt:  "hello",
		AudioConfig: &dto.AudioGenerationConfig{Format: "OGG_OPUS", SampleRate: &unsupported},
	})

	require.NotNil(t, payload.AudioConfig)
	assert.Equal(t, "ogg_opus", payload.AudioConfig.Format)
	assert.Equal(t, 48000, *payload.AudioConfig.SampleRate)

	// 未知格式回落到上游默认值，而不是把非法值透传上去。
	payload = convertForTest(t, dto.AudioGenerationRequest{
		TextPrompt:  "hello",
		AudioConfig: &dto.AudioGenerationConfig{Format: "flac"},
	})
	require.NotNil(t, payload.AudioConfig)
	assert.Equal(t, DefaultFormat, payload.AudioConfig.Format)
}

// TestConvertClampsRateKnobs proves user-supplied rates are bounded before they
// reach the upstream, matching the documented ranges.
func TestConvertClampsRateKnobs(t *testing.T) {
	speech := 500
	loudness := -500
	pitch := 40
	payload := convertForTest(t, dto.AudioGenerationRequest{
		TextPrompt: "hello",
		AudioConfig: &dto.AudioGenerationConfig{
			SpeechRate:   &speech,
			LoudnessRate: &loudness,
			PitchRate:    &pitch,
		},
	})

	require.NotNil(t, payload.AudioConfig)
	assert.Equal(t, MaxSpeechRate, *payload.AudioConfig.SpeechRate)
	assert.Equal(t, MinLoudnessRate, *payload.AudioConfig.LoudnessRate)
	assert.Equal(t, MaxPitchRate, *payload.AudioConfig.PitchRate)
}

// TestConvertSeparatesAudioAndImageReferences pins the upstream mutual
// exclusion: a mixed list is rejected upstream, so the stricter image mode wins.
func TestConvertSeparatesAudioAndImageReferences(t *testing.T) {
	payload := convertForTest(t, dto.AudioGenerationRequest{
		TextPrompt: "hello",
		References: []dto.AudioGenerationReference{
			{AudioURL: "https://cdn.example.com/a.mp3"},
			{ImageURL: "https://cdn.example.com/ref.png"},
		},
	})

	require.Len(t, payload.References, 1)
	assert.Equal(t, "https://cdn.example.com/ref.png", payload.References[0].ImageURL)
	assert.Empty(t, payload.References[0].AudioURL)
}

// TestConvertCapsAndDropsEmptyReferences bounds the reference list: at most
// three audio references, each carrying exactly one field.
func TestConvertCapsAndDropsEmptyReferences(t *testing.T) {
	payload := convertForTest(t, dto.AudioGenerationRequest{
		TextPrompt: "hello",
		References: []dto.AudioGenerationReference{
			{AudioURL: " "},
			{AudioURL: "https://cdn.example.com/1.mp3"},
			{AudioURL: "https://cdn.example.com/2.mp3"},
			{AudioURL: "https://cdn.example.com/3.mp3"},
			{AudioURL: "https://cdn.example.com/4.mp3"},
		},
	})

	require.Len(t, payload.References, MaxAudioReferences)
	assert.Equal(t, "https://cdn.example.com/1.mp3", payload.References[0].AudioURL)
	assert.Equal(t, "https://cdn.example.com/3.mp3", payload.References[2].AudioURL)
}

// TestRequestBillingRatioIsMinutes pins the unit of the pre-charge multiplier.
//
// The configured price is per generated minute, so the ratio must be a fraction
// of a minute: returning seconds here multiplied a per-minute price by up to
// 120, over-reserving the user's wallet by that same factor.
func TestRequestBillingRatioIsMinutes(t *testing.T) {
	// 40 characters at 4 chars/s × 1.2 safety = 12 s = 0.2 minute.
	request := &dto.AudioGenerationRequest{TextPrompt: strings.Repeat("字", 40)}
	meta := request.GetTokenCountMeta()

	require.Contains(t, meta.BillingRatios, types.AudioMinutesRatioKey)
	assert.InDelta(t, 0.2, meta.BillingRatios[types.AudioMinutesRatioKey], 1e-9)

	// 也是同一份估算的唯一边界来源：时长永远落在 [1 秒, 120 秒] 内。
	minutes := meta.BillingRatios[types.AudioMinutesRatioKey]
	assert.GreaterOrEqual(t, minutes, 1.0/60.0)
	assert.LessOrEqual(t, minutes, float64(types.AudioMaxSeconds)/60.0)
}

// TestRequestBillingRatioHonoursSpeechRate proves a longer speech setting
// reserves more, since a slower read produces more audio to pay for.
func TestRequestBillingRatioHonoursSpeechRate(t *testing.T) {
	slow := -50
	request := &dto.AudioGenerationRequest{
		TextPrompt:  strings.Repeat("字", 40),
		AudioConfig: &dto.AudioGenerationConfig{SpeechRate: &slow},
	}
	meta := request.GetTokenCountMeta()

	assert.InDelta(t, 0.4, meta.BillingRatios[types.AudioMinutesRatioKey], 1e-9)
}

func callDoResponse(t *testing.T, body string, status int, relayMode int) (*gin.Context, *types.NewAPIError) {
	t.Helper()
	adaptor := &Adaptor{}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/generations", nil)
	info := &relaycommon.RelayInfo{RelayMode: relayMode}

	resp := &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     http.Header{},
	}
	_, err := adaptor.DoResponse(c, resp, info)
	return c, err
}

// TestDoResponseSurfacesBusinessErrorInSuccessfulHTTP pins the upstream quirk:
// failures arrive as HTTP 200 with a non-zero code, so the body has to be read
// before the status code is trusted. Treating 200 as success would return an
// empty audio and bill for it.
func TestDoResponseSurfacesBusinessErrorInSuccessfulHTTP(t *testing.T) {
	_, apiErr := callDoResponse(t,
		`{"code":40000001,"message":"text_prompt is invalid"}`, http.StatusOK,
		relayconstant.RelayModeAudioGenerations)

	require.NotNil(t, apiErr)
	assert.Contains(t, apiErr.Error(), "text_prompt is invalid")
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
}

// TestDoResponsePublishesParsedAudioResult proves the parsed result reaches the
// handler through the context, since DoResponse is the only place holding the
// response body — the billing duration lives in the same payload.
func TestDoResponsePublishesParsedAudioResult(t *testing.T) {
	c, apiErr := callDoResponse(t,
		`{"code":0,"message":"success","audio":"QUJD","duration":29.5,"original_duration":30,"url":"https://example.com/a.mp3"}`,
		http.StatusOK, relayconstant.RelayModeAudioGenerations)

	require.Nil(t, apiErr)
	parsed, ok := c.Get(ContextKeyAudioResult)
	require.True(t, ok, "parsed result must be published for the handler")

	result, ok := parsed.(dto.AudioGenerationResponse)
	require.True(t, ok)
	assert.Equal(t, "QUJD", result.Audio)
	assert.InDelta(t, 30.0, result.OriginalDuration, 1e-9)
	assert.Equal(t, "https://example.com/a.mp3", result.URL)
}

// TestDoResponseRejectsUnexpectedRelayMode keeps the adaptor from answering on
// a surface it does not implement.
func TestDoResponseRejectsUnexpectedRelayMode(t *testing.T) {
	_, apiErr := callDoResponse(t, `{}`, http.StatusOK, relayconstant.RelayModeChatCompletions)
	require.NotNil(t, apiErr)
}

// TestDoResponseSurfacesNonBusinessFailureBody proves a failure that is not the
// upstream's JSON envelope is still reported instead of being read as success.
func TestDoResponseSurfacesNonBusinessFailureBody(t *testing.T) {
	_, apiErr := callDoResponse(t, `upstream unavailable`, http.StatusBadGateway,
		relayconstant.RelayModeAudioGenerations)

	require.NotNil(t, apiErr)
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Contains(t, apiErr.Error(), "upstream unavailable")
}
