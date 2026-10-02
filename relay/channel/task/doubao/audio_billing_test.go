package doubao

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsAudioModelRoutesSeedAudioToDurationBilling(t *testing.T) {
	cases := []struct {
		model string
		want  bool
	}{
		{model: "seed-audio-1.0", want: true},
		{model: "Seed-Audio-1.0", want: true},
		{model: "doubao-seed-audio-1-0", want: true},
		{model: "seed-audio", want: true},
		{model: "doubao-seedance-2-0-260128", want: false},
		{model: "doubao-seed-2-1-pro-260915", want: false},
		{model: "", want: false},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, IsAudioModel(tc.model), "model %q", tc.model)
	}
}

// TestEstimateAudioSecondsIsConservativeAndBounded fixes the two properties the
// pre-charge depends on: the estimate never falls below a second (a zero
// multiplier would pre-charge nothing and turn the settlement into a no-op),
// and it never exceeds the upstream output ceiling (the estimate is also the
// refund cap, so an inflated one would over-reserve the user's wallet).
func TestEstimateAudioSecondsIsConservativeAndBounded(t *testing.T) {
	cases := []struct {
		name       string
		reqSeconds int
		textChars  int
		speechRate int
		want       float64
	}{
		{name: "no signal falls back to output ceiling", want: maxAudioSeconds},
		{name: "explicit request duration wins", reqSeconds: 30, textChars: 4000, want: 30},
		{name: "explicit duration is capped", reqSeconds: 99999, want: maxAudioSeconds},
		{name: "text at normal speed", textChars: 40, want: 12},
		{name: "text at double speed is shorter", textChars: 40, speechRate: 100, want: 6},
		{name: "text at half speed is longer", textChars: 40, speechRate: -50, want: 24},
		{name: "long text is capped", textChars: 100000, want: maxAudioSeconds},
		{name: "tiny text still reserves a second", textChars: 1, want: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EstimateAudioSeconds(tc.reqSeconds, tc.textChars, tc.speechRate)
			assert.InDelta(t, tc.want, got, 0.001)
			assert.GreaterOrEqual(t, got, 1.0)
			assert.LessOrEqual(t, got, float64(maxAudioSeconds))
		})
	}
}

func TestParseSpeechRateClampsToUpstreamRange(t *testing.T) {
	cases := []struct {
		name  string
		value any
		want  int
	}{
		{name: "absent", value: nil, want: 0},
		{name: "float", value: float64(20), want: 20},
		{name: "string", value: "20", want: 20},
		{name: "above range", value: float64(500), want: 100},
		{name: "below range", value: float64(-500), want: -50},
		{name: "unparseable", value: "fast", want: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, ParseSpeechRate(tc.value))
		})
	}
}

// TestParseTaskResultReportsAudioDurationAndURL pins the polling contract: a
// successful audio task must surface its produced URL and, when the upstream
// states it, the produced duration the metered settlement re-prices from —
// expressed in the billing dimension's own unit (minutes), not seconds.
func TestParseTaskResultReportsAudioDurationAndURL(t *testing.T) {
	body := `{
		"id": "cgt-audio-1",
		"model": "seed-audio-1.0",
		"status": "succeeded",
		"content": {"audio_url": "https://example.com/a.mp3"},
		"usage": {"audio_seconds": 90}
	}`

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"})
	result, err := adaptor.ParseTaskResult([]byte(body))
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
	assert.Equal(t, "https://example.com/a.mp3", result.Url)
	// 90 s reported upstream ⇒ 1.5 minutes billed.
	assert.InDelta(t, 1.5, result.MeteredUsage[audioBillingKey], 0.0001)
}

// TestParseTaskResultAudioDurationIsBounded proves an upstream-reported
// duration cannot reach quota arithmetic unbounded: an absurd value is clamped
// to MaxTaskMeteredSeconds (3600 s = 60 min) rather than dropped, because
// dropping it would silently keep the pre-charge in place.
func TestParseTaskResultAudioDurationIsBounded(t *testing.T) {
	body := `{"id":"cgt-audio-2","status":"succeeded","content":{"audio_url":"https://example.com/a.mp3"},"duration":999999999}`

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"})
	result, err := adaptor.ParseTaskResult([]byte(body))
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.InDelta(t, float64(relaycommon.MaxTaskMeteredSeconds)/secondsPerMinute, result.MeteredUsage[audioBillingKey], 0.0001)
}

// TestParseTaskResultVideoTaskHasNoMeteredUsage keeps the audio path from
// leaking into video tasks, whose billing stays ratio-based: a duration field
// on a video task must not turn into a metered dimension.
func TestParseTaskResultVideoTaskHasNoMeteredUsage(t *testing.T) {
	body := `{"id":"cgt-video-1","status":"succeeded","content":{"video_url":"https://example.com/v.mp4"},"duration":5}`

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{OriginModelName: "doubao-seedance-2-0-260128"})
	result, err := adaptor.ParseTaskResult([]byte(body))
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "https://example.com/v.mp4", result.Url)
	assert.Empty(t, result.MeteredUsage)
}

// TestMeteredBillingBasisMatchesTheConfiguredPerMinutePrice pins the submit-time
// half of the audio money chain against real configured numbers:
//
//	seed-audio-1.0 = $0.375 per generated minute (2× the $0.1875 official rate)
//	→ 0.375 × QuotaPerUnit(500000) = 187500 quota per minute
//
// A frozen basis of 0.5 minute must therefore carry a unit price of 187500, and
// the quota relay_task.go pre-charged (base × OtherRatios[audioBillingKey]) must
// equal quantity × unitPrice. Equal, not merely close: the settlement re-prices
// straight from those two numbers.
func TestMeteredBillingBasisMatchesTheConfiguredPerMinutePrice(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	info.PriceData.ModelPrice = 0.375
	info.PriceData.UsePrice = true
	info.PriceData.GroupRatioInfo.GroupRatio = 1
	// A 30 s estimate enters PriceData as a billing multiplier in minutes.
	info.PriceData.AddOtherRatio(audioBillingKey, 30/secondsPerMinute)

	basis, ok := adaptor.MeteredBillingBasis(info)[audioBillingKey]
	require.True(t, ok)
	assert.InDelta(t, 0.5, basis.Quantity, 1e-9)
	assert.InDelta(t, 187500, basis.UnitPrice, 1e-6)

	// The basis must reproduce exactly the quota relay_task.go pre-charged.
	preCharged, clamp := common.QuotaFromFloatChecked(info.PriceData.ApplyOtherRatiosToFloat(
		info.PriceData.ModelPrice * common.QuotaPerUnit * info.PriceData.GroupRatioInfo.GroupRatio))
	require.Nil(t, clamp)
	assert.Equal(t, 93750, preCharged)
	assert.InDelta(t, float64(preCharged), basis.Quantity*basis.UnitPrice, 1e-6)

	// A video model has no metered dimension.
	videoInfo := &relaycommon.RelayInfo{OriginModelName: "doubao-seedance-2-0-260128"}
	videoInfo.PriceData.ModelPrice = 1
	videoInfo.PriceData.UsePrice = true
	assert.Nil(t, adaptor.MeteredBillingBasis(videoInfo))

	// No usable price, or no estimated quantity ⇒ nothing to settle against.
	unpriced := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	unpriced.PriceData.AddOtherRatio(audioBillingKey, 30/secondsPerMinute)
	assert.Nil(t, adaptor.MeteredBillingBasis(unpriced))

	unestimated := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	unestimated.PriceData.ModelPrice = 0.375
	unestimated.PriceData.UsePrice = true
	assert.Nil(t, adaptor.MeteredBillingBasis(unestimated))
}

// TestEstimateBillingUsesMinutesSoTheMultiplierMatchesTheBasePrice fixes the
// unit contract between EstimateBilling and the pre-charge:
//
//	charge = ModelPrice(per minute) × QuotaPerUnit × groupRatio × OtherRatios[key]
//
// The returned multiplier therefore has to be a fraction of a minute. Returning
// seconds here multiplied a per-minute price by up to 120, over-reserving the
// user's wallet by that same factor — the bug this test exists to prevent.
func TestEstimateBillingUsesMinutesSoTheMultiplierMatchesTheBasePrice(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}

	// "hello" is 5 chars ⇒ 5 / 4 chars-per-second × 1.2 safety = 1.5 s, so the
	// billing multiplier is 1.5 s worth of the per-minute price.
	ratios := estimateAudioRatiosFor(t, adaptor, info, `{"prompt":"hello","model":"seed-audio-1.0"}`)
	require.Contains(t, ratios, audioBillingKey)
	assert.InDelta(t, 1.5/secondsPerMinute, ratios[audioBillingKey], 1e-9)

	// Every key EstimateBilling contributes is a charge multiplier, so nothing
	// bookkeeping-only may appear here: ApplyOtherRatiosToFloat multiplies every
	// entry into the pre-charge. An estimate stashed here would be charged as if
	// it were a price factor.
	assert.Len(t, ratios, 1, "only the billing multiplier may enter OtherRatios: %v", ratios)

	// An explicit target duration is honoured and still expressed in minutes.
	explicit := estimateAudioRatiosFor(t, adaptor, info, `{"prompt":"hello","model":"seed-audio-1.0","duration":90}`)
	assert.InDelta(t, 90.0/secondsPerMinute, explicit[audioBillingKey], 1e-9)
	assert.Len(t, explicit, 1)
}

// TestAudioRatiosNeverExceedThePerMinuteBasePrice is the regression guard for the
// over-charge that reached production: relay_task.go pre-charges
//
//	baseQuota × ∏OtherRatios
//
// where baseQuota is already the configured per-minute price. A multiplier
// expressed in seconds turned a 2-minute estimate into 120× the configured
// price ($45 instead of $0.375). This drives the real estimate through the real
// pre-charge formula, so the length can never again be multiplied in twice.
func TestAudioRatiosNeverExceedThePerMinuteBasePrice(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	info.PriceData.ModelPrice = 0.375
	info.PriceData.UsePrice = true
	info.PriceData.GroupRatioInfo.GroupRatio = 1

	ratios := estimateAudioRatiosFor(t, adaptor, info,
		`{"prompt":"hello","model":"seed-audio-1.0","duration":3}`)
	for key, value := range ratios {
		info.PriceData.AddOtherRatio(key, value)
	}

	// relay_task.go's pre-charge: base quota × ∏OtherRatios.
	baseQuota := info.PriceData.ModelPrice * common.QuotaPerUnit
	preCharged, clamp := common.QuotaFromFloatChecked(info.PriceData.ApplyOtherRatiosToFloat(baseQuota))
	require.Nil(t, clamp)

	// 3 s is 1/20 of a minute ⇒ 0.375 / 20 = $0.01875, not 3 × $0.375.
	assert.Equal(t, 9375, preCharged)
	assert.InDelta(t, 0.01875, float64(preCharged)/common.QuotaPerUnit, 1e-9)

	// The frozen basis the settlement re-prices from must describe that same
	// charge, otherwise the settlement adjusts at a different rate.
	basis := adaptor.MeteredBillingBasis(info)[audioBillingKey]
	assert.InDelta(t, float64(preCharged), basis.Quantity*basis.UnitPrice, 1e-6)
}

// estimateAudioRatiosFor drives the real EstimateBilling entry point for an
// audio request. It stages the parsed TaskSubmitReq on the gin context under the
// key relaycommon.GetTaskRequest reads, which is exactly what
// relaycommon.ValidateBasicTaskRequest does on the live path.
func estimateAudioRatiosFor(t *testing.T, adaptor *TaskAdaptor, info *relaycommon.RelayInfo, body string) map[string]float64 {
	t.Helper()

	var req relaycommon.TaskSubmitReq
	require.NoError(t, common.Unmarshal([]byte(body), &req))

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Set("task_request", req)

	return adaptor.EstimateBilling(c, info)
}

// TestMeteredUsageRequiresFrozenDeclaration proves the settlement basis is the
// value frozen at submit time: without a declared pre-charge the adaptor reports
// nothing, so a task that was never metered cannot be re-priced after the fact.
func TestMeteredUsageRequiresFrozenDeclaration(t *testing.T) {
	adaptor := &TaskAdaptor{}
	task := &model.Task{
		TaskID: "task_no_declaration",
		Properties: model.Properties{
			OriginModelName: "seed-audio-1.0",
		},
		PrivateData: model.TaskPrivateData{BillingContext: &model.TaskBillingContext{}},
	}

	assert.Nil(t, adaptor.MeteredUsage(task, &relaycommon.TaskInfo{Url: "https://example.com/a.mp3"}))
	assert.Nil(t, adaptor.MeteredUsage(nil, &relaycommon.TaskInfo{}))
}

// TestConvertToOpenAIVideoFallsBackToAudioURL keeps a completed audio task
// retrievable through the video-shaped response the task API returns.
func TestConvertToOpenAIVideoFallsBackToAudioURL(t *testing.T) {
	task := &model.Task{
		TaskID: "task_audio_done",
		Data:   []byte(`{"id":"cgt-audio-3","status":"succeeded","content":{"audio_url":"https://example.com/a.mp3"}}`),
		Properties: model.Properties{
			OriginModelName: "seed-audio-1.0",
		},
	}

	adaptor := &TaskAdaptor{}
	raw, err := adaptor.ConvertToOpenAIVideo(task)
	require.NoError(t, err)

	var video dto.OpenAIVideo
	require.NoError(t, common.Unmarshal(raw, &video))
	assert.Equal(t, "https://example.com/a.mp3", video.Metadata["url"])
}
