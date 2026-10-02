package doubao

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
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
// states it, the produced duration the metered settlement re-prices from.
func TestParseTaskResultReportsAudioDurationAndURL(t *testing.T) {
	body := `{
		"id": "cgt-audio-1",
		"model": "seed-audio-1.0",
		"status": "succeeded",
		"content": {"audio_url": "https://example.com/a.mp3"},
		"usage": {"audio_seconds": 41.5}
	}`

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"})
	result, err := adaptor.ParseTaskResult([]byte(body))
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, string(model.TaskStatusSuccess), result.Status)
	assert.Equal(t, "https://example.com/a.mp3", result.Url)
	assert.InDelta(t, 41.5, result.MeteredUsage[audioBillingKey], 0.0001)
}

// TestParseTaskResultAudioDurationIsBounded proves an upstream-reported
// duration cannot reach quota arithmetic unbounded: an absurd value is clamped
// into range rather than dropped, because dropping it would silently keep the
// pre-charge in place.
func TestParseTaskResultAudioDurationIsBounded(t *testing.T) {
	body := `{"id":"cgt-audio-2","status":"succeeded","content":{"audio_url":"https://example.com/a.mp3"},"duration":999999999}`

	adaptor := &TaskAdaptor{}
	adaptor.Init(&relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"})
	result, err := adaptor.ParseTaskResult([]byte(body))
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.InDelta(t, float64(relaycommon.MaxTaskMeteredSeconds), result.MeteredUsage[audioBillingKey], 0.0001)
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

// TestMeteredPreChargeQuotaMatchesTheConfiguredPerMinutePrice pins the
// submit-time half of the audio money chain against real configured numbers:
//
//	seed-audio-1.0 = $0.375 per generated minute (2× the $0.1875 official rate)
//	→ 0.375 × QuotaPerUnit(500000) = 187500 quota per minute = 3125 per second
//
// so an estimate of 40 seconds must reserve 125000 quota. The adaptor must also
// refuse to claim a metered dimension for video models and for inputs with no
// usable price or estimate, because a claimed dimension with no reserved quota
// would let the settlement re-price a task that never charged for it.
func TestMeteredPreChargeQuotaMatchesTheConfiguredPerMinutePrice(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	adaptor := &TaskAdaptor{}
	info := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	info.PriceData.ModelPrice = 0.375
	info.PriceData.UsePrice = true
	info.PriceData.GroupRatioInfo.GroupRatio = 1
	info.PriceData.AddOtherRatio(audioBillingKey, 40)

	assert.Equal(t, map[string]int{audioBillingKey: 125000}, adaptor.MeteredPreChargeQuota(info))

	// A video model has no metered dimension.
	videoInfo := &relaycommon.RelayInfo{OriginModelName: "doubao-seedance-2-0-260128"}
	videoInfo.PriceData.ModelPrice = 1
	videoInfo.PriceData.UsePrice = true
	assert.Nil(t, adaptor.MeteredPreChargeQuota(videoInfo))

	// No usable price, or no estimated quantity ⇒ nothing to settle against.
	unpriced := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	unpriced.PriceData.AddOtherRatio(audioBillingKey, 40)
	assert.Nil(t, adaptor.MeteredPreChargeQuota(unpriced))

	unestimated := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	unestimated.PriceData.ModelPrice = 0.375
	unestimated.PriceData.UsePrice = true
	assert.Nil(t, adaptor.MeteredPreChargeQuota(unestimated))
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
