package runninghub_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/QuantumNous/new-api/zsy/runninghub/rhparser"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// §9.2 integration tests: real DB + real relay/billing pipeline + a fake
// RunningHub upstream served over httptest. They cover the three billing
// lifecycles of the dev plan:
//
//  1. dynamic billing: submit (pre-consume) → RUNNING → SUCCESS polls with a
//     usage.consumeCoins diff → user balance settles to the actual charge
//  2. per-call billing: flat pre-charge, no diff settlement on completion
//  3. failure: full refund of the pre-consumed quota, CAS-idempotent under
//     repeated polling
// ---------------------------------------------------------------------------

const (
	itestChannelID  = 9101
	itestUserID     = 9101
	itestTokenID    = 9101
	itestTokenID2   = 9102
	itestInitQuota  = 2_000_000
	itestChannelKey = "rh-key-itest"
	itestTokenKey   = "sk-rh-itest"
)

// capturedRHRequest records one upstream call for later assertions.
type capturedRHRequest struct {
	Path          string
	Authorization string
	Body          string
}

// pseudoRH is the fake RunningHub upstream: it accepts the V2 submit paths and
// the query endpoint, records every call, and delegates response selection to
// per-test closures.
type pseudoRH struct {
	server *httptest.Server

	mu      sync.Mutex
	submits []capturedRHRequest
	queries []capturedRHRequest

	submitResp func(appID string) (int, any)
	queryResp  func(taskID string) (int, any)
}

func writeRHJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func newPseudoRH(t *testing.T) *pseudoRH {
	t.Helper()
	p := &pseudoRH{}

	record := func(r *http.Request) string {
		raw, _ := io.ReadAll(r.Body)
		p.mu.Lock()
		defer p.mu.Unlock()
		entry := capturedRHRequest{Path: r.URL.Path, Authorization: r.Header.Get("Authorization"), Body: string(raw)}
		return entry.Path + "\x00" + entry.Authorization + "\x00" + entry.Body
	}

	handle := func(w http.ResponseWriter, r *http.Request, appID string) {
		raw := record(r)
		parts := strings.SplitN(raw, "\x00", 3)
		p.mu.Lock()
		p.submits = append(p.submits, capturedRHRequest{Path: parts[0], Authorization: parts[1], Body: parts[2]})
		fn := p.submitResp
		p.mu.Unlock()
		require.NotNil(t, fn, "unexpected submit call")
		status, body := fn(appID)
		writeRHJSON(w, status, body)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/openapi/v2/run/ai-app/", func(w http.ResponseWriter, r *http.Request) {
		handle(w, r, strings.TrimPrefix(r.URL.Path, "/openapi/v2/run/ai-app/"))
	})
	mux.HandleFunc("/openapi/v2/run/workflow/", func(w http.ResponseWriter, r *http.Request) {
		handle(w, r, strings.TrimPrefix(r.URL.Path, "/openapi/v2/run/workflow/"))
	})
	mux.HandleFunc("/openapi/v2/query", func(w http.ResponseWriter, r *http.Request) {
		raw := record(r)
		parts := strings.SplitN(raw, "\x00", 3)
		var qb runninghub.QueryBody
		_ = json.Unmarshal([]byte(parts[2]), &qb)
		p.mu.Lock()
		p.queries = append(p.queries, capturedRHRequest{Path: parts[0], Authorization: parts[1], Body: parts[2]})
		fn := p.queryResp
		p.mu.Unlock()
		require.NotNil(t, fn, "unexpected query call")
		status, body := fn(qb.TaskID)
		writeRHJSON(w, status, body)
	})

	p.server = httptest.NewServer(mux)
	t.Cleanup(p.server.Close)
	return p
}

// rhITestEnv bundles everything one integration test needs.
type rhITestEnv struct {
	t      *testing.T
	db     *gorm.DB
	rh     *pseudoRH
	router *gin.Engine
}

// newRHITestEnv boots a fully isolated stack: a per-test in-memory SQLite DB,
// a fake RH upstream, and a gin router carrying the same context keys the
// host's TokenAuth + Distribute middlewares would produce (wallet token in the
// "default" group). All process-wide globals are snapshotted and restored.
func newRHITestEnv(t *testing.T, upstreamID string) *rhITestEnv {
	t.Helper()

	origDB, origLogDB := model.DB, model.LOG_DB
	origRedis, origMem, origBatch := common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled
	origLogConsume, origRetry := common.LogConsumeEnabled, common.RetryTimes
	origQueryLimit, origTimeout := constant.TaskQueryLimit, constant.TaskTimeoutMinutes
	origGroupRatio := ratio_setting.GroupRatio2JSONString()
	origPrices := ratio_setting.GetModelPriceCopy()
	origRatios := ratio_setting.GetModelRatioCopy()
	origGetAdaptor := service.GetTaskAdaptorFunc
	// model.UpdateOption writes into common.OptionMap; the test binary never
	// runs InitOptionMap, so back it up and install a fresh (non-nil) map.
	origOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)

	common.RedisEnabled = false
	common.MemoryCacheEnabled = false
	common.BatchUpdateEnabled = false
	common.LogConsumeEnabled = true
	common.RetryTimes = 0
	// GetAllUnFinishSyncTasks guards with Limit(limit); the test binary never
	// runs common.init.go so the env default would be 0 (LIMIT 0 → no rows).
	constant.TaskQueryLimit = 100
	constant.TaskTimeoutMinutes = 0
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1}`))

	// The host wires this in main(); point it at the plugin adaptor so the
	// polling pass exercises the real FetchTask/ParseTaskResult/settle chain.
	service.GetTaskAdaptorFunc = func(constant.TaskPlatform) service.TaskPollingAdaptor {
		return &runninghub.TaskAdaptor{}
	}

	t.Cleanup(func() {
		model.DB, model.LOG_DB = origDB, origLogDB
		common.RedisEnabled, common.MemoryCacheEnabled, common.BatchUpdateEnabled = origRedis, origMem, origBatch
		common.LogConsumeEnabled, common.RetryTimes = origLogConsume, origRetry
		constant.TaskQueryLimit, constant.TaskTimeoutMinutes = origQueryLimit, origTimeout
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(origGroupRatio))
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(mustJSON(t, origPrices)))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(mustJSON(t, origRatios)))
		service.GetTaskAdaptorFunc = origGetAdaptor
		common.OptionMap = origOptionMap
	})

	rh := newPseudoRH(t)

	dsn := fmt.Sprintf("file:rh_itest_%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	// The DB-backed channel selection builds raw SQL with the package-private
	// quoted column helpers (commonGroupCol …), which only InitDB/InitLogDB
	// initialize. Pin LOG_SQL_DSN empty so InitLogDB takes the "LOG_DB = DB"
	// branch and runs initCol() against the in-memory database.
	t.Setenv("LOG_SQL_DSN", "")
	require.NoError(t, model.InitLogDB())
	t.Cleanup(func() { _ = sqlDB.Close() })

	_ = db.AutoMigrate(
		&model.Task{}, &model.User{}, &model.Token{}, &model.Log{}, &model.Channel{}, &model.Ability{},
		&model.SubscriptionPlan{}, &model.SubscriptionOrder{}, &model.UserSubscription{},
		&model.Option{},
		&runninghub.App{}, &runninghub.AppKeyPool{}, &runninghub.KeypoolPending{},
	)

	// Seed user / token / channel / ability wired to the fake upstream.
	require.NoError(t, db.Create(&model.User{
		Id: itestUserID, Username: "rh_itest", Quota: itestInitQuota, Status: common.UserStatusEnabled,
	}).Error)
	require.NoError(t, db.Create(&model.Token{
		Id: itestTokenID, UserId: itestUserID, Key: itestTokenKey, Name: "rh_itest_token",
		Status: common.TokenStatusEnabled, RemainQuota: itestInitQuota,
	}).Error)
	channelURL := rh.server.URL
	prio := int64(0)
	weight := uint(100)
	require.NoError(t, db.Create(&model.Channel{
		Id: itestChannelID, Type: constant.ChannelTypeRunningHub, Key: itestChannelKey,
		Status: common.ChannelStatusEnabled, Name: "rh-itest", Models: upstreamID, Group: "default",
		BaseURL: &channelURL, Priority: &prio, Weight: &weight,
	}).Error)
	require.NoError(t, db.Create(&model.Ability{
		Group: "default", Model: upstreamID, ChannelId: itestChannelID, Enabled: true, Priority: &prio, Weight: weight,
	}).Error)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(string(constant.ContextKeyUserId), itestUserID)
		c.Set(string(constant.ContextKeyUserGroup), "default")
		c.Set(string(constant.ContextKeyUsingGroup), "default")
		c.Set(string(constant.ContextKeyTokenId), itestTokenID)
		c.Set(string(constant.ContextKeyTokenKey), itestTokenKey)
		c.Set(string(constant.ContextKeyTokenGroup), "default")
		c.Set(string(constant.ContextKeyUserQuota), itestInitQuota)
		c.Set(string(constant.ContextKeyTokenUnlimited), false)
		c.Next()
	})
	router.POST("/api/zsy/rh/apps/:id/run", runninghub.TestHookSubmitAppRun)
	router.GET("/api/zsy/rh/apps/task/:task_id", runninghub.TestHookGetAppTaskResult)

	// A second token belonging to the same user, so the "select a token for
	// this run" path can be exercised (billed against it, not the default).
	require.NoError(t, db.Create(&model.Token{
		Id: itestTokenID2, UserId: itestUserID, Key: "sk-rh-itest-2", Name: "rh_itest_token_2",
		Status: common.TokenStatusEnabled, RemainQuota: 0,
	}).Error)

	return &rhITestEnv{t: t, db: db, rh: rh, router: router}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return string(data)
}

// ---------------------------------------------------------------------------
// Small assertion helpers
// ---------------------------------------------------------------------------

func (e *rhITestEnv) userQuota() int {
	e.t.Helper()
	var q int
	require.NoError(e.t, model.DB.Model(&model.User{}).Where("id = ?", itestUserID).Select("quota").Scan(&q).Error)
	return q
}

func (e *rhITestEnv) tokenRemain() int {
	e.t.Helper()
	var q int
	require.NoError(e.t, model.DB.Model(&model.Token{}).Where("id = ?", itestTokenID).Select("remain_quota").Scan(&q).Error)
	return q
}

func (e *rhITestEnv) taskByTaskID(publicTaskID string) *model.Task {
	e.t.Helper()
	var task model.Task
	require.NoError(e.t, model.DB.First(&task, "task_id = ?", publicTaskID).Error)
	return &task
}

func (e *rhITestEnv) createApp(name, upstreamID string, perCall bool, fixedQuota int64, rate float64) uint {
	e.t.Helper()
	view, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:               name,
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         upstreamID,
		Published:          true,
		ParamSchema:        []rhparser.SchemaParam{{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true}},
		PerCallBilling:     perCall,
		FixedQuotaPerCall:  fixedQuota,
		ModelBaseRateRatio: rate,
	})
	require.NoError(e.t, err)
	return view.ID
}

// createAppWithSchema is createApp with an explicit parameter schema, used by
// the per-second billing tests whose schema must carry a seconds parameter.
func (e *rhITestEnv) createAppWithSchema(name, upstreamID string, schema []rhparser.SchemaParam) uint {
	e.t.Helper()
	view, err := runninghub.AppInsert(&runninghub.AppCreateDTO{
		Name:               name,
		Kind:               runninghub.AppKindAICApp,
		UpstreamID:         upstreamID,
		Published:          true,
		ParamSchema:        schema,
		PerSecondBilling:   true,
		QuotaPerSecond:     10_000,
		ModelBaseRateRatio: 1.0,
	})
	require.NoError(e.t, err)
	return view.ID
}

// submitApp runs the user-side submit endpoint and returns the envelope data
// (taskId / status / upstreamTaskId).
func (e *rhITestEnv) submitApp(appID uint) (int, *apiEnvelope, map[string]any) {
	e.t.Helper()
	w, raw := doJSON(e.t, e.router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{"values": map[string]any{"122.prompt": "a tiny castle"}})
	env := parseAPIEnvelope(e.t, raw)
	data, _ := env.Data.(map[string]any)
	if data == nil {
		data = map[string]any{}
	}
	return w.Code, env, data
}

// pollOnce runs exactly one task-polling pass through the host pipeline.
func (e *rhITestEnv) pollOnce() {
	e.t.Helper()
	summary := service.RunTaskPollingOnce(context.Background(), nil)
	require.Equal(e.t, 1, summary.UnfinishedTasks, "polling pass must see exactly the one in-flight task")
	require.Equal(e.t, 1, summary.PlatformsScanned)
}

// ---------------------------------------------------------------------------
// 1. Dynamic billing: pre-consume → RUNNING → SUCCESS diff settlement
// ---------------------------------------------------------------------------

func TestIntegration_DynamicBilling_SuccessPollSettlesDiff(t *testing.T) {
	const upstreamID = "9001-dynamic-app"
	env := newRHITestEnv(t, upstreamID)

	// Dynamic billing: model ratio 2 → pre-consume = ratio/2 × QuotaPerUnit ×
	// groupRatio(1) = 500000.
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(fmt.Sprintf(`{%q: 2}`, upstreamID)))
	appID := env.createApp("itest-dynamic", upstreamID, false, 0, 1.0)

	// Pseudo upstream: submit returns RUNNING; the first poll answers RUNNING,
	// the second SUCCESS with usage.consumeCoins = 123456 quota.
	upstreamTaskID := "rh-task-dyn-1"
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: upstreamTaskID, Status: runninghub.StatusRunning}
	}
	pollResponses := []runninghub.QueryResp{
		{TaskID: upstreamTaskID, Status: runninghub.StatusRunning},
		{
			TaskID: upstreamTaskID, Status: runninghub.StatusSuccess,
			Usage:   &runninghub.TaskUsage{ConsumeCoins: "123456", ConsumeMoney: "0.246912"},
			Results: []runninghub.TaskResult{{URL: "https://img.rh-itest.local/out-0.png", NodeID: "9", OutputType: "image"}},
		},
	}
	env.rh.queryResp = func(string) (int, any) {
		require.NotEmpty(t, pollResponses, "unexpected extra poll")
		next := pollResponses[0]
		pollResponses = pollResponses[1:]
		return http.StatusOK, next
	}

	const preConsumed = 500000 // modelRatio 2 / 2 × QuotaPerUnit(500000)
	code, envResp, data := env.submitApp(appID)
	require.Equal(t, http.StatusOK, code)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	publicTaskID, _ := data["taskId"].(string)
	require.NotEmpty(t, publicTaskID)
	assert.Equal(t, upstreamTaskID, data["upstreamTaskId"])
	assert.Equal(t, string(model.TaskStatusInProgress), data["status"])

	// Submit pre-charged wallet + token, and recorded the task with the
	// upstream id and dynamic (non-per-call) billing context.
	assert.Equal(t, itestInitQuota-preConsumed, env.userQuota())
	assert.Equal(t, itestInitQuota-preConsumed, env.tokenRemain())
	task := env.taskByTaskID(publicTaskID)
	assert.Equal(t, preConsumed, task.Quota)
	assert.Equal(t, model.TaskStatusInProgress, string(task.Status))
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.False(t, task.PrivateData.BillingContext.PerCallBilling)
	assert.Equal(t, upstreamTaskID, task.PrivateData.UpstreamTaskID)

	// The submit reached RH with the channel key and the schema-built nodes.
	require.Len(t, env.rh.submits, 1)
	assert.Equal(t, "Bearer "+itestChannelKey, env.rh.submits[0].Authorization)
	var sentBody runninghub.SubmitBody
	require.NoError(t, json.Unmarshal([]byte(env.rh.submits[0].Body), &sentBody))
	require.Len(t, sentBody.NodeInfoList, 1)
	assert.Equal(t, "122", sentBody.NodeInfoList[0].NodeID)
	assert.Equal(t, "prompt", sentBody.NodeInfoList[0].FieldName)
	assert.Equal(t, "a tiny castle", sentBody.NodeInfoList[0].FieldValue)
	assert.Equal(t, runninghub.InstanceDefault, sentBody.InstanceType)

	// First poll: RUNNING — status moves along, no billing change.
	env.pollOnce()
	assert.Equal(t, itestInitQuota-preConsumed, env.userQuota())
	assert.Equal(t, itestInitQuota-preConsumed, env.tokenRemain())

	// Second poll: SUCCESS — diff settlement restores the unused part
	// (500000 − 123456 = 376544) so the user ends up paying exactly the
	// upstream-reported charge.
	env.pollOnce()
	assert.Equal(t, itestInitQuota-123456, env.userQuota())
	assert.Equal(t, itestInitQuota-123456, env.tokenRemain())

	task = env.taskByTaskID(publicTaskID)
	assert.Equal(t, model.TaskStatusSuccess, string(task.Status))
	assert.Equal(t, "100%", task.Progress)
	assert.Equal(t, 123456, task.Quota)
	assert.Equal(t, "https://img.rh-itest.local/out-0.png", task.PrivateData.ResultURL)

	// task.Data keeps the full upstream results and the result API exposes the
	// URL list through TaskDto.
	var dataView map[string]any
	require.NoError(t, json.Unmarshal(task.Data, &dataView))
	results, _ := dataView["results"].([]any)
	require.Len(t, results, 1)
	firstResult, _ := results[0].(map[string]any)
	assert.Equal(t, "https://img.rh-itest.local/out-0.png", firstResult["url"])

	// The diff settlement must be auditable as a refund log entry.
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).
		Where("user_id = ? AND type = ? AND quota = ?", itestUserID, model.LogTypeRefund, preConsumed-123456).
		Count(&logCount).Error)
	assert.Equal(t, int64(1), logCount)

	// User-facing result API: TaskDto envelope with status / result_url / data.
	w, raw := doJSON(t, env.router, http.MethodGet, fmt.Sprintf("/api/zsy/rh/apps/task/%s", publicTaskID), nil)
	require.Equal(t, http.StatusOK, w.Code)
	resultEnv := parseAPIEnvelope(t, raw)
	require.True(t, resultEnv.Success, "result API failed: %s", resultEnv.Message)
	dtoBytes, err := json.Marshal(resultEnv.Data)
	require.NoError(t, err)
	var dto struct {
		TaskID    string          `json:"task_id"`
		Status    string          `json:"status"`
		Quota     int             `json:"quota"`
		ResultURL string          `json:"result_url"`
		Progress  string          `json:"progress"`
		Data      json.RawMessage `json:"data"`
	}
	require.NoError(t, json.Unmarshal(dtoBytes, &dto))
	assert.Equal(t, publicTaskID, dto.TaskID)
	assert.Equal(t, "SUCCESS", dto.Status)
	assert.Equal(t, 123456, dto.Quota)
	assert.Equal(t, "https://img.rh-itest.local/out-0.png", dto.ResultURL)
	assert.Contains(t, string(dto.Data), "https://img.rh-itest.local/out-0.png")
}

// ---------------------------------------------------------------------------
// 2. Per-call billing: flat pre-charge, no diff settlement on completion
// ---------------------------------------------------------------------------

func TestIntegration_PerCallBilling_SuccessKeepsFlatCharge(t *testing.T) {
	const upstreamID = "9002-percall-app"
	const fixedQuota = int64(100_000)
	env := newRHITestEnv(t, upstreamID)

	// AppInsert syncs the host model price table (FixedQuotaPerCall /
	// QuotaPerUnit), which is what drives the per-call pre-charge.
	appID := env.createApp("itest-percall", upstreamID, true, fixedQuota, 1.0)
	prices := ratio_setting.GetModelPriceCopy()
	require.Contains(t, prices, upstreamID, "per-call app must sync its model price entry")

	upstreamTaskID := "rh-task-flat-1"
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: upstreamTaskID, Status: runninghub.StatusQueued}
	}
	pollResponses := []runninghub.QueryResp{
		{TaskID: upstreamTaskID, Status: runninghub.StatusRunning},
		{
			TaskID: upstreamTaskID, Status: runninghub.StatusSuccess,
			// Upstream consumed far more than the flat charge — per-call
			// billing must ignore it.
			Usage:   &runninghub.TaskUsage{ConsumeCoins: "999999"},
			Results: []runninghub.TaskResult{{URL: "https://img.rh-itest.local/flat.png"}},
		},
	}
	env.rh.queryResp = func(string) (int, any) {
		require.NotEmpty(t, pollResponses, "unexpected extra poll")
		next := pollResponses[0]
		pollResponses = pollResponses[1:]
		return http.StatusOK, next
	}

	code, envResp, data := env.submitApp(appID)
	require.Equal(t, http.StatusOK, code)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	publicTaskID, _ := data["taskId"].(string)
	require.NotEmpty(t, publicTaskID)

	assert.Equal(t, itestInitQuota-int(fixedQuota), env.userQuota())
	assert.Equal(t, itestInitQuota-int(fixedQuota), env.tokenRemain())
	task := env.taskByTaskID(publicTaskID)
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.True(t, task.PrivateData.BillingContext.PerCallBilling)

	env.pollOnce() // RUNNING
	env.pollOnce() // SUCCESS — settle must be skipped

	assert.Equal(t, itestInitQuota-int(fixedQuota), env.userQuota(), "per-call billing must not trigger diff settlement")
	assert.Equal(t, itestInitQuota-int(fixedQuota), env.tokenRemain())
	task = env.taskByTaskID(publicTaskID)
	assert.Equal(t, model.TaskStatusSuccess, string(task.Status))
	assert.Equal(t, int(fixedQuota), task.Quota)
	assert.Equal(t, "https://img.rh-itest.local/flat.png", task.PrivateData.ResultURL)

	// No settlement/refund log rows: the flat charge was final at submit time.
	var logCount int64
	require.NoError(t, model.DB.Model(&model.Log{}).Where("user_id = ?", itestUserID).Count(&logCount).Error)
	assert.Zero(t, logCount)
}

// ---------------------------------------------------------------------------
// 2b. Per-second billing: pre-charge = QuotaPerSecond × seconds, then diff
// settle against RH usage.consumeCoins on SUCCESS (dynamic-billing semantics).
// ---------------------------------------------------------------------------

func TestIntegration_PerSecondBilling_PrechargeAndDiffSettle(t *testing.T) {
	const upstreamID = "9004-persecond-app"
	const quotaPerSecond = int64(10_000)
	const seconds = 30
	const preConsumed = quotaPerSecond * seconds // 300000
	env := newRHITestEnv(t, upstreamID)

	appID := env.createAppWithSchema("itest-persecond", upstreamID, []rhparser.SchemaParam{
		{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true},
		{NodeID: "77", FieldName: "seconds", Label: "时长(秒)", Type: "seconds", Required: true},
	})

	// AppInsert must sync the per-second base price (QuotaPerSecond/QuotaPerUnit)
	// into both the in-memory price table AND the persisted ModelPrice option —
	// otherwise a restart reloads a stale table and the next submit regresses to
	// the model_price_error this feature is meant to fix.
	prices := ratio_setting.GetModelPriceCopy()
	require.Contains(t, prices, upstreamID)
	require.Equal(t, float64(quotaPerSecond)/common.QuotaPerUnit, prices[upstreamID])
	var opt model.Option
	require.NoError(t, model.DB.First(&opt, "key = ?", "ModelPrice").Error)
	stored := map[string]float64{}
	require.NoError(t, json.Unmarshal([]byte(opt.Value), &stored))
	require.Contains(t, stored, upstreamID)

	upstreamTaskID := "rh-task-per-sec-1"
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: upstreamTaskID, Status: runninghub.StatusRunning}
	}
	pollResponses := []runninghub.QueryResp{
		{TaskID: upstreamTaskID, Status: runninghub.StatusRunning},
		{
			TaskID:       upstreamTaskID,
			Status:       runninghub.StatusSuccess,
			Usage:        &runninghub.TaskUsage{ConsumeCoins: "123000"},
			Results:      []runninghub.TaskResult{{URL: "https://img.rh-itest.local/sec.png"}},
		},
	}
	env.rh.queryResp = func(string) (int, any) {
		require.NotEmpty(t, pollResponses, "unexpected extra poll")
		next := pollResponses[0]
		pollResponses = pollResponses[1:]
		return http.StatusOK, next
	}

	w, raw := doJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{"values": map[string]any{
			"122.prompt":  "a beating heart",
			"77.seconds":  fmt.Sprintf("%d", seconds),
		}})
	envResp := parseAPIEnvelope(t, raw)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	data, _ := envResp.Data.(map[string]any)
	publicTaskID, _ := data["taskId"].(string)
	require.NotEmpty(t, publicTaskID)

	// Pre-charge = QuotaPerSecond × seconds × groupRatio(1).
	assert.Equal(t, itestInitQuota-int(preConsumed), env.userQuota())
	assert.Equal(t, itestInitQuota-int(preConsumed), env.tokenRemain())
	task := env.taskByTaskID(publicTaskID)
	require.NotNil(t, task.PrivateData.BillingContext)
	assert.False(t, task.PrivateData.BillingContext.PerCallBilling,
		"per-second billing must keep settlement enabled (not per-call)")
	assert.Equal(t, int(preConsumed), task.Quota)

	env.pollOnce() // RUNNING
	env.pollOnce() // SUCCESS → diff settle against consumeCoins 123000

	assert.Equal(t, itestInitQuota-123000, env.userQuota())
	task = env.taskByTaskID(publicTaskID)
	assert.Equal(t, model.TaskStatusSuccess, string(task.Status))
	assert.Equal(t, 123000, task.Quota)

	// The seconds ratio must be visible in the submitted node fields too — the
	// schema says the seconds-bearing node goes upstream verbatim.
	require.Len(t, env.rh.submits, 1)
	var sentBody runninghub.SubmitBody
	require.NoError(t, json.Unmarshal([]byte(env.rh.submits[0].Body), &sentBody))
	fields := map[string]string{}
	for _, n := range sentBody.NodeInfoList {
		fields[n.NodeID] = n.FieldValue
	}
	assert.Equal(t, fmt.Sprintf("%d", seconds), fields["77"])
	assert.Equal(t, "a beating heart", fields["122"])
}

// ---------------------------------------------------------------------------
// 3. Failure: full refund, CAS-idempotent under repeated polling
// ---------------------------------------------------------------------------

func TestIntegration_FailureRefund_IsIdempotent(t *testing.T) {
	const upstreamID = "9003-fail-app"
	env := newRHITestEnv(t, upstreamID)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(fmt.Sprintf(`{%q: 2}`, upstreamID)))
	appID := env.createApp("itest-fail", upstreamID, false, 0, 1.0)

	upstreamTaskID := "rh-task-fail-1"
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: upstreamTaskID, Status: runninghub.StatusRunning}
	}
	pollResponses := []runninghub.QueryResp{
		{TaskID: upstreamTaskID, Status: runninghub.StatusRunning},
		{
			TaskID:       upstreamTaskID,
			Status:       runninghub.StatusFailed,
			ErrorCode:    "1501",
			ErrorMessage: "content moderation rejected the prompt",
		},
	}
	env.rh.queryResp = func(string) (int, any) {
		require.NotEmpty(t, pollResponses, "unexpected extra poll")
		next := pollResponses[0]
		pollResponses = pollResponses[1:]
		return http.StatusOK, next
	}

	const preConsumed = 500000
	code, envResp, data := env.submitApp(appID)
	require.Equal(t, http.StatusOK, code)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	publicTaskID, _ := data["taskId"].(string)
	require.NotEmpty(t, publicTaskID)
	assert.Equal(t, itestInitQuota-preConsumed, env.userQuota())

	env.pollOnce() // RUNNING
	env.pollOnce() // FAILED → refund

	assert.Equal(t, itestInitQuota, env.userQuota(), "failed task must refund the full pre-consumed quota")
	assert.Equal(t, itestInitQuota, env.tokenRemain())
	task := env.taskByTaskID(publicTaskID)
	assert.Equal(t, model.TaskStatusFailure, string(task.Status))
	assert.Zero(t, task.Quota, "refunded task must not retain a refundable quota")
	assert.Contains(t, task.FailReason, "1501")
	assert.Equal(t, "100%", task.Progress)

	// A refund log must exist for auditing.
	var refundLogs int64
	require.NoError(t, model.DB.Model(&model.Log{}).
		Where("user_id = ? AND type = ? AND quota = ?", itestUserID, model.LogTypeRefund, preConsumed).
		Count(&refundLogs).Error)
	assert.Equal(t, int64(1), refundLogs)

	// Repeated polling of the terminal task is a no-op (no double refund).
	env.pollOnceLoop(2)
	assert.Equal(t, itestInitQuota, env.userQuota())
	assert.Equal(t, itestInitQuota, env.tokenRemain())
	assert.Equal(t, int64(1), countRefundLogs(t, itestUserID, preConsumed),
		"repeated polling must not add refund logs")

	// CAS guard: force the task back to IN_PROGRESS with its quota already
	// cleared — a re-poll must move it to FAILURE again but must not refund
	// twice (RefundTaskQuota bails on the zero quota). Progress must also be
	// reset: GetAllUnFinishSyncTasks skips rows already at "100%".
	require.NoError(t, model.DB.Model(&model.Task{}).Where("task_id = ?", publicTaskID).
		Updates(map[string]any{"status": string(model.TaskStatusInProgress), "progress": ""}).Error)
	// The re-poll reaches the upstream again; keep answering FAILED.
	pollResponses = append(pollResponses, runninghub.QueryResp{
		TaskID: upstreamTaskID, Status: runninghub.StatusFailed,
	})
	env.pollOnce()
	assert.Equal(t, itestInitQuota, env.userQuota())
	assert.Equal(t, itestInitQuota, env.tokenRemain())
	assert.Equal(t, int64(1), countRefundLogs(t, itestUserID, preConsumed),
		"CAS re-poll must not refund twice")
	task = env.taskByTaskID(publicTaskID)
	assert.Equal(t, model.TaskStatusFailure, string(task.Status))
}

// ---------------------------------------------------------------------------
// 4. Selected-token billing: when the caller passes tokenId, the task bills
// against that token (pre-consume + settle + refund), and a token that does
// not belong to the user is rejected before any billing.
// ---------------------------------------------------------------------------

func TestIntegration_SelectedToken_BillsAgainstChosenToken(t *testing.T) {
	const upstreamID = "9005-tokenselect-app"
	const quotaPerSecond = int64(10_000)
	const seconds = 30
	const preConsumed = quotaPerSecond * seconds // 300000
	env := newRHITestEnv(t, upstreamID)

	appID := env.createAppWithSchema("itest-tokenselect", upstreamID, []rhparser.SchemaParam{
		{NodeID: "122", FieldName: "prompt", Label: "提示词", Type: "text", Required: true},
		{NodeID: "77", FieldName: "seconds", Label: "时长(秒)", Type: "seconds", Required: true},
	})
	// Give the second token enough quota for the pre-charge so the picked-token
	// semantics are observable (the default itestToken keeps its own quota).
	require.NoError(t, model.DB.Model(&model.Token{}).Where("id = ?", itestTokenID2).
		Update("remain_quota", int(preConsumed)).Error)

	upstreamTaskID := "rh-task-tokenselect-1"
	env.rh.submitResp = func(string) (int, any) {
		return http.StatusOK, runninghub.SubmitResp{TaskID: upstreamTaskID, Status: runninghub.StatusRunning}
	}
	pollResponses := []runninghub.QueryResp{
		{TaskID: upstreamTaskID, Status: runninghub.StatusRunning},
		{
			TaskID:  upstreamTaskID,
			Status:  runninghub.StatusSuccess,
			Results: []runninghub.TaskResult{{URL: "https://img.rh-itest.local/sel.png"}},
		},
	}
	env.rh.queryResp = func(string) (int, any) {
		require.NotEmpty(t, pollResponses, "unexpected extra poll")
		next := pollResponses[0]
		pollResponses = pollResponses[1:]
		return http.StatusOK, next
	}

	// Submit with tokenId=itestTokenID2 pinned; the run must pre-charge from
	// the picked token instead of the default one.
	w, raw := doJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{
			"values": map[string]any{
				"122.prompt": "a window",
				"77.seconds": fmt.Sprintf("%d", seconds),
			},
			"tokenId": itestTokenID2,
		})
	envResp := parseAPIEnvelope(t, raw)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, envResp.Success, "submit failed: %s", envResp.Message)
	data, _ := envResp.Data.(map[string]any)
	publicTaskID, _ := data["taskId"].(string)
	require.NotEmpty(t, publicTaskID)

	// Wallet is pre-charged as usual; the picked token was drawn down too.
	assert.Equal(t, itestInitQuota-int(preConsumed), env.userQuota())
	var tok2 model.Token
	require.NoError(t, model.DB.First(&tok2, "id = ?", itestTokenID2).Error)
	assert.Equal(t, 0, tok2.RemainQuota, "selected token 2 must have been fully pre-charged")

	// The task must record the picked token on PrivateData so the billing log
	// attributes the charge to the right API key.
	task := env.taskByTaskID(publicTaskID)
	assert.Equal(t, itestTokenID2, task.PrivateData.TokenId)
	var tok1 model.Token
	require.NoError(t, model.DB.First(&tok1, "id = ?", itestTokenID).Error)
	assert.Equal(t, itestInitQuota, tok1.RemainQuota, "default token must be untouched")

	env.pollOnce() // RUNNING
	env.pollOnce() // SUCCESS with no consumeCoins → keep the pre-charge.

	task = env.taskByTaskID(publicTaskID)
	assert.Equal(t, model.TaskStatusSuccess, string(task.Status))
	assert.Equal(t, int(preConsumed), task.Quota)
	var tok2b model.Token
	require.NoError(t, model.DB.First(&tok2b, "id = ?", itestTokenID2).Error)
	assert.Equal(t, 0, tok2b.RemainQuota)
	require.NoError(t, model.DB.First(&tok1, "id = ?", itestTokenID).Error)
	assert.Equal(t, itestInitQuota, tok1.RemainQuota, "default token must stay untouched")
}

// TestIntegration_SelectedToken_RejectsForeignToken covers the ownership
// guard: tokenId pointing at a token of another user is rejected before any
// billing happens.
func TestIntegration_SelectedToken_RejectsForeignToken(t *testing.T) {
	const upstreamID = "9006-tokenreject-app"
	env := newRHITestEnv(t, upstreamID)
	appID := env.createApp("itest-tokenreject", upstreamID, false, 0, 1.0)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(fmt.Sprintf(`{%q: 2}`, upstreamID)))

	// A token owned by a different user.
	require.NoError(t, model.DB.Create(&model.Token{
		Id: 9999, UserId: itestUserID + 1, Key: "sk-other-user", Name: "other",
		Status: common.TokenStatusEnabled, RemainQuota: 1_000_000,
	}).Error)

	w, raw := doJSON(t, env.router, http.MethodPost, fmt.Sprintf("/api/zsy/rh/apps/%d/run", appID),
		map[string]any{
			"values":  map[string]any{"122.prompt": "hi"},
			"tokenId": 9999,
		})
	envResp := parseAPIEnvelope(t, raw)
	require.Equal(t, http.StatusOK, w.Code)
	assert.False(t, envResp.Success)
	assert.Contains(t, envResp.Message, "不属于当前用户")
}

// pollOnceLoop runs polling passes that see no unfinished tasks (the task is
// already terminal) — unfinished count must be 0.
func (e *rhITestEnv) pollOnceLoop(times int) {
	e.t.Helper()
	for i := 0; i < times; i++ {
		summary := service.RunTaskPollingOnce(context.Background(), nil)
		require.Equal(e.t, 0, summary.UnfinishedTasks, "terminal task must not be picked up again")
	}
}

func countRefundLogs(t *testing.T, userID, quota int) int64 {
	t.Helper()
	var n int64
	require.NoError(t, model.DB.Model(&model.Log{}).
		Where("user_id = ? AND type = ? AND quota = ?", userID, model.LogTypeRefund, quota).
		Count(&n).Error)
	return n
}
