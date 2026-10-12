package runninghub_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// instanceType contract (app-level compute instance).
//
// The instance type is an *application* setting an administrator picks on
// /rh-apps (default / plus / ultra), not a per-request parameter: whatever the
// app row says is what the upstream submit body has to carry. These tests drive
// the real handler → relay → adaptor chain against the pseudo RunningHub
// upstream and read the body it received.
// ---------------------------------------------------------------------------

// newInstanceTypeApp inserts a published app whose only configurable parameter is
// the prompt, with the instance type under test.
func newInstanceTypeApp(t *testing.T, env *rhITestEnv, name, upstreamID, instanceType string, perCallQuota int64) uint {
	t.Helper()
	view, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:               name,
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         upstreamID,
		Published:          true,
		ParamSchema:        []rhparser.SchemaParam{{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true}},
		PerCallBilling:     true,
		FixedQuotaPerCall:  perCallQuota,
		ModelBaseRateRatio: 1.0,
		InstanceType:       instanceType,
	})
	require.NoError(t, err)
	return view.ID
}

// submittedInstanceType decodes the instanceType out of the last submit body the
// pseudo upstream received.
func (e *rhITestEnv) submittedInstanceType(t *testing.T) string {
	t.Helper()
	require.NotEmpty(t, e.rh.submits, "the run must have reached the upstream")
	last := e.rh.submits[len(e.rh.submits)-1]
	var body runninghub.SubmitBody
	require.NoError(t, json.Unmarshal([]byte(last.Body), &body), "body: %s", last.Body)
	return body.InstanceType
}

// TestInstanceType_AppSettingReachesTheUpstream pins the rule itself: the app's
// configured instance type is what RH is asked to run on. `ultra` is the case a
// hardcoded default/plus allow-list would silently downgrade.
func TestInstanceType_AppSettingReachesTheUpstream(t *testing.T) {
	for _, instanceType := range []string{
		runninghub.InstanceDefault,
		runninghub.InstancePlus,
		runninghub.InstanceUltra,
	} {
		t.Run(instanceType, func(t *testing.T) {
			upstreamID := "9401-instance-" + instanceType
			env := newRHITestEnv(t, upstreamID)
			enableRunningTask(env, "rh-up-"+instanceType)
			appID := newInstanceTypeApp(t, env, "instance "+instanceType, upstreamID, instanceType, itestPerCallQuota)

			code, envResp, data := env.submitApp(appID)
			require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
			require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
			require.NotEmpty(t, data["taskId"])

			assert.Equal(t, instanceType, env.submittedInstanceType(t))
		})
	}
}

// TestInstanceType_LegacyRowWithoutTheColumnSubmitsDefault covers the upgrade
// path: every app saved before this column existed reads back as the default
// instance, and a run of such an app must still be submitted on it.
func TestInstanceType_LegacyRowWithoutTheColumnSubmitsDefault(t *testing.T) {
	const upstreamID = "9402-instance-legacy"
	env := newRHITestEnv(t, upstreamID)
	enableRunningTask(env, "rh-up-legacy")

	// A row as the previous release wrote it: the column was not part of the
	// insert, so the database default applies.
	require.NoError(t, model.DB.Exec(
		"INSERT INTO apps (name, kind, upstream_id, published, admin_only, param_schema, per_call_billing, fixed_quota_per_call, model_base_rate_ratio) "+
			"VALUES (?, 'ai_app', ?, 1, 0, ?, 1, ?, 1.0)",
		"legacy instance app", upstreamID,
		`[{"nodeId":"122","fieldName":"prompt","label":"提示词","type":"text","required":true}]`,
		itestPerCallQuota,
	).Error)

	var legacyID uint
	require.NoError(t, model.DB.Raw("SELECT id FROM apps WHERE upstream_id = ?", upstreamID).Scan(&legacyID).Error)
	require.NotZero(t, legacyID)

	view, err := runninghub.AppGetByID(legacyID)
	require.NoError(t, err, "the read path must cope with a row that predates the column")
	assert.Equal(t, runninghub.InstanceDefault, view.InstanceType)

	// A raw insert skips syncAppBillingPrice, so the per-call price the billing
	// chain reads is registered here the way the store layer would.
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(
		fmt.Sprintf(`{%q: %v}`, upstreamID, float64(itestPerCallQuota)/common.QuotaPerUnit)))

	code, envResp, _ := env.submitApp(legacyID)
	require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)

	assert.Equal(t, runninghub.InstanceDefault, env.submittedInstanceType(t))
}

// TestInstanceType_RequestValueNeverOverridesTheAppSetting is the regression
// test for the bug this feature shipped with: the first app-center bundle sent
// `instanceType: "default"` on every run (the default was hardcoded in the
// client), which pinned every run to the default instance no matter what the
// admin had configured. The run endpoints now ignore the field entirely, so an
// app configured for plus submits plus even when an old client asks for
// default — and an unrecognised value cannot sneak upstream either.
func TestInstanceType_RequestValueNeverOverridesTheAppSetting(t *testing.T) {
	const upstreamID = "9411-instance-ignored-override"
	env := newRHITestEnv(t, upstreamID)
	enableRunningTask(env, "rh-up-ignored")
	appID := newInstanceTypeApp(t, env, "instance ignored override", upstreamID, runninghub.InstancePlus, itestPerCallQuota)

	for name, override := range map[string]string{
		"old bundle's default": runninghub.InstanceDefault,
		"another instance":     runninghub.InstanceUltra,
		"empty string":         "",
	} {
		t.Run(name, func(t *testing.T) {
			code, envResp, _ := env.submitAppWithPayload(appID, map[string]any{
				"values":       map[string]any{"122.prompt": "a tiny castle"},
				"instanceType": override,
			})
			require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
			require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
			assert.Equal(t, runninghub.InstancePlus, env.submittedInstanceType(t),
				"the app's configured instance must win")
		})
	}

	view, err := runninghub.AppGetByID(appID)
	require.NoError(t, err)
	assert.Equal(t, runninghub.InstancePlus, view.InstanceType, "a run must not rewrite the app setting")
}

// TestInstanceType_UnknownRequestValueIsIgnoredNotForwarded: ignoring the field
// must not turn into forwarding it. A caller that misspells the instance type
// gets a normal run on the app's configured instance, and the typo never reaches
// RunningHub.
func TestInstanceType_UnknownRequestValueIsIgnoredNotForwarded(t *testing.T) {
	const upstreamID = "9412-instance-typo"
	env := newRHITestEnv(t, upstreamID)
	enableRunningTask(env, "rh-up-typo")
	appID := newInstanceTypeApp(t, env, "instance typo", upstreamID, runninghub.InstancePlus, itestPerCallQuota)

	code, envResp, _ := env.submitAppWithPayload(appID, map[string]any{
		"values":       map[string]any{"122.prompt": "a tiny castle"},
		"instanceType": "pluss",
	})
	require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	assert.Equal(t, runninghub.InstancePlus, env.submittedInstanceType(t),
		"the app's instance is used and the typo is dropped")
}

// TestInstanceType_AbsentOrEmptyRequestValueUsesTheAppSetting pins the
// app-center payload shape: with the field absent, or present as an empty
// string, the app's configured instance is what goes upstream — which is what
// makes "编辑里设了 plus 就按 plus 跑" hold.
func TestInstanceType_AbsentOrEmptyRequestValueUsesTheAppSetting(t *testing.T) {
	const upstreamID = "9410-instance-absent"
	env := newRHITestEnv(t, upstreamID)
	enableRunningTask(env, "rh-up-absent")
	appID := newInstanceTypeApp(t, env, "instance absent", upstreamID, runninghub.InstancePlus, itestPerCallQuota)

	for name, payload := range map[string]map[string]any{
		"field absent": {"values": map[string]any{"122.prompt": "a tiny castle"}},
		"empty string": {
			"values":       map[string]any{"122.prompt": "a tiny castle"},
			"instanceType": "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			code, envResp, _ := env.submitAppWithPayload(appID, payload)
			require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
			require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
			assert.Equal(t, runninghub.InstancePlus, env.submittedInstanceType(t))
		})
	}
}

// TestInstanceType_UnknownValueIsRejectedBeforeSubmit keeps the field an enum of
// upstream machine classes: a typo is refused at save time, never stored for RH
// to reject (or silently downgrade) at run time.
func TestInstanceType_UnknownValueIsRejectedBeforeSubmit(t *testing.T) {
	const upstreamID = "9404-instance-invalid"
	env := newRHITestEnv(t, upstreamID)
	env.rh.submitResp = func(string) (int, any) {
		t.Error("an invalid instance type must be refused before the upstream call")
		return http.StatusOK, runninghub.SubmitResp{}
	}

	_, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:               "invalid instance app",
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         upstreamID,
		Published:          true,
		ParamSchema:        []rhparser.SchemaParam{{NodeID: "122", FieldName: "prompt", Type: "text", Required: true}},
		PerCallBilling:     true,
		FixedQuotaPerCall:  itestPerCallQuota,
		ModelBaseRateRatio: 1.0,
		InstanceType:       "marathon",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "instanceType")

	var count int64
	require.NoError(t, model.DB.Model(&runninghub.App{}).Where("upstream_id = ?", upstreamID).Count(&count).Error)
	assert.Zero(t, count, "a rejected instance type must not be persisted")
}

// TestInstanceType_AdminEditChangesWhatTheNextRunUses is the administrator's
// workflow end to end: the value edited on /rh-apps is what the following run is
// submitted with.
func TestInstanceType_AdminEditChangesWhatTheNextRunUses(t *testing.T) {
	const upstreamID = "9405-instance-edit"
	env := newRHITestEnv(t, upstreamID)
	enableRunningTask(env, "rh-up-edit")
	appID := newInstanceTypeApp(t, env, "instance edit", upstreamID, runninghub.InstanceDefault, itestPerCallQuota)

	code, envResp, _ := env.submitApp(appID)
	require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	assert.Equal(t, runninghub.InstanceDefault, env.submittedInstanceType(t))

	updated, err := runninghub.AppUpdate(appID, &runninghub.AppUpdateDTO{
		Name:               "instance edit",
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         upstreamID,
		Published:          true,
		ParamSchema:        []rhparser.SchemaParam{{NodeID: "122", FieldName: "prompt", Type: "text", Required: true}},
		PerCallBilling:     true,
		FixedQuotaPerCall:  itestPerCallQuota,
		ModelBaseRateRatio: 1.0,
		InstanceType:       runninghub.InstancePlus,
	})
	require.NoError(t, err)
	require.Equal(t, runninghub.InstancePlus, updated.InstanceType, "the admin list must show the saved value")

	code, envResp, _ = env.submitApp(appID)
	require.Equal(t, http.StatusOK, code, "submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	assert.Equal(t, runninghub.InstancePlus, env.submittedInstanceType(t))
}

// TestInstanceType_ModelNameSubmitUsesTheAppSetting covers the other way into the
// same adaptor: a client that picks `rh-app-<id>` on the standard relay endpoints
// sends no app id of its own, so the setting has to be read back from the row the
// model name points at.
func TestInstanceType_ModelNameSubmitUsesTheAppSetting(t *testing.T) {
	const upstreamID = "9406-instance-model-name"
	env := newRHITestEnv(t, upstreamID)
	appID := newInstanceTypeApp(t, env, "instance model name", upstreamID, runninghub.InstanceUltra, itestPerCallQuota)

	body := buildRelaySubmitBody(t, fmt.Sprintf("%s%d", runninghub.RhAppModelPrefix, appID), nil)
	assert.Equal(t, runninghub.InstanceUltra, body.InstanceType)
}

// TestInstanceType_ModelNameSubmitDefaultsWithoutAnAppRow: a model name that
// names no app row (the historical bare-upstream-id convention, and hand-written
// channel entries) has nothing to read and must submit on the default instance.
func TestInstanceType_ModelNameSubmitDefaultsWithoutAnAppRow(t *testing.T) {
	const upstreamID = "9407-instance-plain-model"
	newRHITestEnv(t, upstreamID)

	body := buildRelaySubmitBody(t, upstreamID, nil)
	assert.Equal(t, runninghub.InstanceDefault, body.InstanceType)
}

// TestInstanceType_ModelNameSubmitWithMetadataStillUsesTheAppSetting: naming an
// app — here through the `rh-app-<id>` model name — means that app's
// configuration decides, even when the request metadata states another instance.
func TestInstanceType_ModelNameSubmitWithMetadataStillUsesTheAppSetting(t *testing.T) {
	const upstreamID = "9408-instance-metadata"
	env := newRHITestEnv(t, upstreamID)
	appID := newInstanceTypeApp(t, env, "instance metadata", upstreamID, runninghub.InstancePlus, itestPerCallQuota)

	body := buildRelaySubmitBody(t, fmt.Sprintf("%s%d", runninghub.RhAppModelPrefix, appID), map[string]any{
		"rh": map[string]any{"instanceType": runninghub.InstanceDefault},
	})
	assert.Equal(t, runninghub.InstancePlus, body.InstanceType,
		"the named app's setting must win over the request metadata")
}

// TestInstanceType_MetadataIsHonouredWhenNoAppIsNamed: a submission that names no
// app at all has no row to read, so the explicit metadata value — when it is a
// known instance type — is what goes upstream. This is the historical
// bare-upstream-id model name; an unidentified value falls back to the default.
func TestInstanceType_MetadataIsHonouredWhenNoAppIsNamed(t *testing.T) {
	const upstreamID = "9413-instance-bare-model"
	newRHITestEnv(t, upstreamID)

	body := buildRelaySubmitBody(t, upstreamID, map[string]any{
		"rh": map[string]any{"instanceType": runninghub.InstanceUltra},
	})
	assert.Equal(t, runninghub.InstanceUltra, body.InstanceType)

	garbage := buildRelaySubmitBody(t, upstreamID, map[string]any{
		"rh": map[string]any{"instanceType": "pluss"},
	})
	assert.Equal(t, runninghub.InstanceDefault, garbage.InstanceType,
		"an unknown value must not be forwarded upstream")
}

// TestInstanceType_QueueCarriesTheSettingToDispatch covers the queued path: when
// every channel of a site is at its concurrency cap the body is built once at
// enqueue time and replayed by the dispatcher later, so the instance type has to
// be part of the stored body rather than resolved again at dispatch.
func TestInstanceType_QueueCarriesTheSettingToDispatch(t *testing.T) {
	const upstreamID = "9409-instance-queued"
	env := newRHITestEnv(t, upstreamID)
	env.setChannelMaxConcurrency(itestChannelID, 1)
	enableRunningTask(env, "rh-up-queued")

	view, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:               "instance queued",
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         upstreamID,
		Site:               "cn",
		Published:          true,
		ParamSchema:        []rhparser.SchemaParam{{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true}},
		PerCallBilling:     true,
		FixedQuotaPerCall:  itestPerCallQuota,
		ModelBaseRateRatio: 1.0,
		InstanceType:       runninghub.InstancePlus,
	})
	require.NoError(t, err)

	// First run takes the only slot; the second is accepted as a QUEUED record.
	code, envResp, firstData := env.submitApp(view.ID)
	require.Equal(t, http.StatusOK, code, "first submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "first submit failed: %s", envResp.Message)
	require.Len(t, env.rh.submits, 1)
	firstTaskID, _ := firstData["taskId"].(string)
	require.NotEmpty(t, firstTaskID)

	code, envResp, data := env.submitApp(view.ID)
	require.Equal(t, http.StatusOK, code, "queued submit failed: %s", envResp.Message)
	require.True(t, envResp.Success, "queued submit failed: %s", envResp.Message)
	require.Equal(t, true, data["queued"])
	require.Len(t, env.rh.submits, 1, "a queued run must not reach the upstream yet")

	// The first run reaches a terminal state → its slot frees → the dispatcher
	// replays the stored body of the queued one.
	require.NoError(t, env.db.Model(&model.Task{}).
		Where("task_id = ?", firstTaskID).
		Update("status", model.TaskStatusSuccess).Error)
	runninghub.TestHookDispatchQueue()
	require.Len(t, env.rh.submits, 2, "the freed slot must admit the queued run")

	assert.Equal(t, runninghub.InstancePlus, env.submittedInstanceType(t))
}

// buildRelaySubmitBody builds the upstream submit body for a raw relay call: the
// gin context carries only the parsed task request (no "rh_app"), which is what a
// caller on /v1/video/generations produces.
func buildRelaySubmitBody(t *testing.T, model string, metadata map[string]any) runninghub.SubmitBody {
	t.Helper()
	c, _ := newAdaptorContext(t)

	req := relaycommon.TaskSubmitReq{Model: model, Prompt: "a tiny castle"}
	if metadata != nil {
		req.Metadata = metadata
	}
	c.Set("task_request", req)

	info := newRelayInfo("task_public_instance_type")
	info.OriginModelName = model
	info.Action = "rh_ai_app"

	raw, err := runninghub.TestHookBuildSubmitBody(c, info)
	require.NoError(t, err)

	var body runninghub.SubmitBody
	require.NoError(t, json.Unmarshal(raw, &body), "body: %s", string(raw))
	return body
}
