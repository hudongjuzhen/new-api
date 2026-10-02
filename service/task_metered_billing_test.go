package service

import (
	"context"
	"testing"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Metered (per-unit) task settlement
//
// A metered task pre-charges from a submit-time estimate and then re-prices
// itself from what the upstream actually produced. These tests fix the
// contract the polling layer relies on: the charge always follows the frozen
// submit-time unit price, never exceeds what was pre-charged for the dimension,
// and falls back to the pre-charged amount when no measurement is available.
// ---------------------------------------------------------------------------

// meteredTask builds a task whose billed dimension was pre-charged from
// `declared` units at `unitPricePerUnit` quota each.
func meteredTask(userId, channelId, tokenId int, declared float64, preChargedQuota int) *model.Task {
	task := makeTask(userId, channelId, preChargedQuota, tokenId, BillingSourceWallet, 0)
	task.PrivateData.BillingContext.OtherRatios = map[string]float64{"audio_seconds": declared}
	task.PrivateData.BillingContext.MeteredPreChargeQuota = map[string]int{"audio_seconds": preChargedQuota}
	task.PrivateData.BillingContext.PerCallBilling = false
	return task
}

// TestSettleMeteredTaskQuota_ConfiguredPriceProducesExactCharge pins the whole
// audio-generation money chain against real configured numbers rather than
// arbitrary test values:
//
//	seed-audio-1.0 = $0.375 per generated minute (2× the $0.1875 official rate)
//	→ 0.375 × QuotaPerUnit(500000) = 187500 quota per minute = 3125 per second
//
// A request whose text estimates 40 s reserves 125000 quota; producing 40 s
// must settle at exactly that, and producing the 120 s upstream ceiling must
// settle at 375000.
func TestSettleMeteredTaskQuota_ConfiguredPriceProducesExactCharge(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 45, 45, 45
	const declaredSeconds = 40.0
	const preConsumed = 125000 // 40 s × 3125
	const initQuota, tokenRemain = 1_000_000, 900_000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-metered-configured", tokenRemain)
	seedChannel(t, channelID)

	task := meteredTask(userID, channelID, tokenID, declaredSeconds, preConsumed)

	// Exactly the estimated duration: the pre-charge already equals the answer,
	// so nothing moves.
	require.True(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": declaredSeconds}))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))

	// A shorter than estimated render refunds the difference at the same rate:
	// 10 s produced ⇒ 31250 quota.
	require.True(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": 10}))
	assert.Equal(t, 31250, task.Quota)
	assert.Equal(t, initQuota+preConsumed-31250, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed-31250, getTokenRemainQuota(t, tokenID))

	// The upstream ceiling still bills the full upstream length, never more.
	require.True(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": 1200}))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
}

func TestSettleMeteredTaskQuota_UndershootRefundsToActualDuration(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 40, 40, 40
	const initQuota, tokenRemain = 100000, 90000
	// 100 s estimated and pre-charged, 40 s actually produced.
	const preConsumed = 5000
	const wantQuota = 2000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-metered-under", tokenRemain)
	seedChannel(t, channelID)

	task := meteredTask(userID, channelID, tokenID, 100, preConsumed)

	settled := SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": 40})
	require.True(t, settled)

	assert.Equal(t, wantQuota, task.Quota)
	assert.Equal(t, initQuota+preConsumed-wantQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed-wantQuota, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumed-wantQuota, log.Quota)
}

func TestSettleMeteredTaskQuota_OvershootIsCappedAtPreChargedAmount(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 41, 41, 41
	const initQuota, tokenRemain = 100000, 90000
	const preConsumed = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-metered-over", tokenRemain)
	seedChannel(t, channelID)

	task := meteredTask(userID, channelID, tokenID, 100, preConsumed)

	// The upstream produced far longer audio than the estimate reserved. The
	// charge may not exceed what was pre-charged for the dimension.
	settled := SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": 100000})
	require.True(t, settled)

	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
}

func TestSettleMeteredTaskQuota_UnknownMeasurementKeepsPreChargedAmount(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 42, 42, 42
	const initQuota, tokenRemain = 100000, 90000
	const preConsumed = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-metered-unknown", tokenRemain)
	seedChannel(t, channelID)

	task := meteredTask(userID, channelID, tokenID, 100, preConsumed)

	// No measurement: nothing to re-price, so the pre-charge stands untouched.
	assert.False(t, SettleMeteredTaskQuota(ctx, task, nil))
	assert.False(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{}))
	// A dimension the task never priced cannot be charged either.
	assert.False(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{"unpriced": 30}))
	// Non-positive and non-numeric quantities are refused, never billed.
	assert.False(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": 0}))
	assert.False(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": -5}))

	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSettleMeteredTaskQuota_TaskWithoutMeteredContextIsNotRepriced(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 43, 43, 43
	const initQuota, tokenRemain = 100000, 90000
	const preConsumed = 5000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-metered-legacy", tokenRemain)
	seedChannel(t, channelID)

	// A task submitted before metered billing existed carries no frozen
	// pre-charge record, so an adaptor reporting a duration must not re-price it.
	task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)

	assert.False(t, SettleMeteredTaskQuota(ctx, task, map[string]float64{"audio_seconds": 1}))
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, initQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
}

func TestSettle_NonPerCallBilling_MeteredUsageTakesPrecedenceOverAdaptorQuota(t *testing.T) {
	truncate(t)
	ctx := context.Background()

	const userID, tokenID, channelID = 44, 44, 44
	const initQuota, tokenRemain = 100000, 90000
	const preConsumed = 5000
	const wantQuota = 3000

	seedUser(t, userID, initQuota)
	seedToken(t, tokenID, userID, "sk-metered-precedence", tokenRemain)
	seedChannel(t, channelID)

	task := meteredTask(userID, channelID, tokenID, 100, preConsumed)
	task.Status = model.TaskStatus(model.TaskStatusInProgress)
	require.NoError(t, model.DB.Create(task).Error)

	// The adaptor would also settle, but the frozen metered measurement is the
	// authoritative basis for this task and must win.
	adaptor := &mockAdaptor{adjustReturn: 999}
	taskResult := &relaycommon.TaskInfo{
		Status:       model.TaskStatusSuccess,
		MeteredUsage: map[string]float64{"audio_seconds": 60},
	}

	settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)

	assert.Equal(t, wantQuota, task.Quota)
	assert.Equal(t, initQuota+preConsumed-wantQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumed-wantQuota, getTokenRemainQuota(t, tokenID))
}
