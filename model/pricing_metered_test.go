package model

import (
	"testing"

	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/billing_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPricingCarriesMeteredBillingMode guards the contract the pricing page
// consumes: for a model billed by the duration it actually produced, the pricing
// response must say so and name the unit `model_price` is quoted in.
//
// Without the marker the page can only see a `model_price`, falls through to its
// "per request" branch, and understates the price of a two-minute render by that
// factor — the display bug this contract exists to prevent.
func TestPricingCarriesMeteredBillingMode(t *testing.T) {
	resetPricingEndpointTestTables(t)
	initPricingRatioSettings(t)

	const modelName = "seed-audio-1.0"
	const channelID = 801
	insertPricingEndpointChannel(t, channelID, 1, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, channelID, modelName)

	pricing, ok := findPricing(t, modelName)
	require.True(t, ok, "%s missing from pricing response", modelName)

	assert.Equal(t, billing_setting.BillingModeMetered, pricing.BillingMode)
	assert.Equal(t, billing_setting.MeteredUnitMinute, pricing.MeteredUnit)
	// The unit is only meaningful next to the price it qualifies.
	assert.Positive(t, pricing.ModelPrice)
}

// TestPricingLeavesNonMeteredModelsUnmarked keeps the marker from spreading: a
// model that is not registered as metered must keep the per-request display so
// its price is not presented as a rate.
func TestPricingLeavesNonMeteredModelsUnmarked(t *testing.T) {
	resetPricingEndpointTestTables(t)
	initPricingRatioSettings(t)

	const modelName = "sora-2"
	const channelID = 802
	insertPricingEndpointChannel(t, channelID, 1, dto.ChannelOtherSettings{})
	insertPricingEndpointAbility(t, channelID, modelName)

	pricing, ok := findPricing(t, modelName)
	require.True(t, ok, "%s missing from pricing response", modelName)

	assert.Empty(t, pricing.BillingMode)
	assert.Empty(t, pricing.MeteredUnit)
}

// initPricingRatioSettings installs the ratio tables this test needs, because the
// pricing response resolves model_price from ratio_setting rather than the DB.
func initPricingRatioSettings(t *testing.T) {
	t.Helper()
	ratio_setting.InitRatioSettings()
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(`{"seed-audio-1.0":0.375,"sora-2":0.3}`))
	InvalidatePricingCache()
	t.Cleanup(InvalidatePricingCache)
}

func findPricing(t *testing.T, modelName string) (Pricing, bool) {
	t.Helper()
	InitChannelCache()
	for _, pricing := range GetPricing() {
		if pricing.ModelName == modelName {
			return pricing, true
		}
	}
	return Pricing{}, false
}
