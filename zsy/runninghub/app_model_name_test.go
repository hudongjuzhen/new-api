package runninghub_test

import (
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/zsy/runninghub"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// `rh-app-<id>` 是"用模型名指定网关里的 RunningHub 应用"这一约定（实现在
// resolveUpstreamIDForModel）。标准中继端点（/v1/video/generations 等）只有模型名
// 这一个入口，所以这条约定必须真的能用、且出错时说得清为什么。

func TestResolveUpstreamIDForModel_PlainModelPassesThrough(t *testing.T) {
	// 历史约定：裸的上游应用 ID 就是模型名（线上渠道的模型表里现在就是它）。
	// 它必须原样透传 —— 否则已配好的渠道会在升级后立刻失效。
	for _, model := range []string{"2085006876680503297", "some-workflow-id"} {
		got, err := runninghub.TestHookResolveUpstreamIDForModel(model)
		require.NoError(t, err)
		assert.Equal(t, model, got)
	}
}

func TestResolveUpstreamIDForModel_MalformedAppNamesAreRejected(t *testing.T) {
	// 不带数据库也要能判出来：这些错误必须发生在查库之前，且提示要指向模型名。
	for _, tc := range []struct {
		model string
		hint  string
	}{
		{"rh-app-", "缺少应用 ID"},
		{"rh-app-abc", "不是数字"},
		{"rh-app-3x", "不是数字"},
	} {
		_, err := runninghub.TestHookResolveUpstreamIDForModel(tc.model)
		require.Error(t, err, "模型名 %q 应当被拒绝", tc.model)
		assert.Contains(t, err.Error(), tc.hint)
	}
}

func TestResolveUpstreamIDForModel_ResolvesGatewayAppRow(t *testing.T) {
	const upstreamID = "9401-rh-app-model"
	env := newRHITestEnv(t, upstreamID)
	appID := env.createApp("rh-app-model", upstreamID, true, 1000, 1.0)

	got, err := runninghub.TestHookResolveUpstreamIDForModel(
		runninghub.RhAppModelPrefix + strconv.FormatUint(uint64(appID), 10),
	)

	require.NoError(t, err)
	assert.Equal(t, upstreamID, got, "rh-app-<行号> 必须解析成该行的上游应用 ID")
}

func TestResolveUpstreamIDForModel_MissingAppSaysWhichOne(t *testing.T) {
	_ = newRHITestEnv(t, "9402-rh-app-missing")

	_, err := runninghub.TestHookResolveUpstreamIDForModel("rh-app-424242")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "rh-app-424242", "报错要点名是哪个模型名")
	assert.NotContains(t, err.Error(), "openapi", "不该把上游路径塞进用户可见的错误里")
}

func TestGetModelList_ExposesGatewayFormAndUpstreamID(t *testing.T) {
	const upstreamID = "9403-rh-list"
	env := newRHITestEnv(t, upstreamID)
	appID := env.createApp("rh-list", upstreamID, true, 1000, 1.0)

	models := (&runninghub.TaskAdaptor{}).GetModelList()

	assert.Contains(t, models, runninghub.RhAppModelPrefix+strconv.FormatUint(uint64(appID), 10),
		"网关形态的模型名要可用（客户端就是按这个名字选模型的）")
	assert.Contains(t, models, upstreamID, "裸上游 ID 也要保留（历史渠道表里就是它）")
}
