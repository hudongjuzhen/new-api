package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func meteredInfo(modelPrice float64, groupRatio float64, preConsumed int) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{OriginModelName: "seed-audio-1.0"}
	info.PriceData.ModelPrice = modelPrice
	info.PriceData.UsePrice = true
	info.PriceData.GroupRatioInfo.GroupRatio = groupRatio
	info.PriceData.QuotaToPreConsume = preConsumed
	return info
}

// TestMeteredDurationQuotaUsesMinutes fixes the unit contract between the
// configured per-minute price and the produced duration:
//
//	quota = ModelPrice(per minute) × QuotaPerUnit × groupRatio × minutes
//
// Treating the reported seconds as the multiplier charged 60× too much; this is
// the regression guard for that arithmetic.
func TestMeteredDurationQuotaUsesMinutes(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	info := meteredInfo(0.375, 1, 0)

	// 30 seconds = 0.5 minute ⇒ 0.375 × 500000 × 0.5 = 93750.
	quota, clamp := MeteredDurationQuota(info, 30)
	require.Nil(t, clamp)
	assert.Equal(t, 93750, quota)

	// The full 120 s ceiling ⇒ 2 minutes ⇒ 375000.
	quota, clamp = MeteredDurationQuota(info, 120)
	require.Nil(t, clamp)
	assert.Equal(t, 375000, quota)

	// Group ratio multiplies the charge.
	grouped := meteredInfo(0.375, 2, 0)
	quota, clamp = MeteredDurationQuota(grouped, 30)
	require.Nil(t, clamp)
	assert.Equal(t, 187500, quota)
}

// TestMeteredDurationQuotaFallsBackToPreCharge proves a missing or unusable
// measurement keeps the pre-charged amount instead of turning the request into
// a free one — the upstream always states the duration, so "unknown" means
// something went wrong, not "charge nothing".
func TestMeteredDurationQuotaFallsBackToPreCharge(t *testing.T) {
	info := meteredInfo(0.375, 1, 4242)

	for _, seconds := range []float64{0, -1} {
		quota, clamp := MeteredDurationQuota(info, seconds)
		require.Nil(t, clamp)
		assert.Equal(t, 4242, quota, "seconds=%v must keep the pre-charge", seconds)
	}

	// 无有效单价时同样保留预扣。
	unpriced := meteredInfo(0, 1, 777)
	quota, clamp := MeteredDurationQuota(unpriced, 30)
	require.Nil(t, clamp)
	assert.Equal(t, 777, quota)
}

// TestMeteredDurationQuotaBoundsUpstreamDuration keeps an absurd upstream value
// from reaching quota arithmetic: it is clamped to the metered ceiling rather
// than dropped, so an over-long report still settles at the boundary.
func TestMeteredDurationQuotaBoundsUpstreamDuration(t *testing.T) {
	oldQuotaPerUnit := common.QuotaPerUnit
	common.QuotaPerUnit = 500000
	t.Cleanup(func() { common.QuotaPerUnit = oldQuotaPerUnit })

	info := meteredInfo(0.375, 1, 0)

	quota, clamp := MeteredDurationQuota(info, 999999999)
	require.Nil(t, clamp)
	ceilingSeconds := float64(relaycommon.MaxTaskMeteredSeconds)
	assert.Equal(t, common.QuotaRound(0.375*500000*(ceilingSeconds/60.0)), quota)
}

// TestMeteredRatioKeyMatchesTaskChain pins the shared dimension name: the sync
// audio endpoint and the audio task adaptor must agree on it, or a model billed
// through one path would settle against a dimension the other never froze.
func TestMeteredRatioKeyMatchesTaskChain(t *testing.T) {
	info := meteredInfo(0.375, 1, 0)
	info.PriceData.AddOtherRatio(types.AudioMinutesRatioKey, 0.5)
	require.True(t, info.PriceData.HasOtherRatio(types.AudioMinutesRatioKey))
	assert.Equal(t, "audio_minutes", types.AudioMinutesRatioKey)
}
