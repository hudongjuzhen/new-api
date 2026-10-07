package world

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// Two diagnostics that exist because this round's hardest defect was a NAME.
//
// Go sent `api_base`; the engine read `base_url`. The parameter was silently
// ignored, the engine fell back to the operator's upstream gateway, and the op
// still returned 200 with a complete, correct-looking parse. Every layer looked
// right on its own:
//
//	Go's own assertion — "we put api_base in the request body"            true
//	engine by hand     — "it honours this parameter"                      true
//	the locator        — "we picked the build that knows the parameter"   true
//
// and the composed result was a **wrong ledger** — a defect with no error
// message anywhere. Finding it took a bisect; these two tests make the next
// occurrence a single run.
//
// ⚠ Neither is about the wire format being "nice". They answer two questions
// that are otherwise unobservable: *which build is loaded*, and *what does that
// build do with the exact bytes we send*.
// =========================================================================

// TestEngineLocatorPicksAnExistingBuild makes "which engine is loaded" visible.
//
// # Why this is worth a test
//
// `findEngineBinary` walks four candidate repo roots × three target dirs and
// takes whatever exists first. A stale `target/debug` binary can win over the
// test build, and the resulting failures are then about the *engine's behaviour*
// — they point at the engine, or at the Go code, but never at "you loaded the
// wrong file". Printing the choice costs nothing and ends that class of hunt.
//
// The assertion is deliberately weak (located ⇒ exists): the strong check lives
// in `requireMockableEngine`, which proves the located build really honours
// `mock_script`. This one guarantees only that the choice is *reported* and *real*.
func TestEngineLocatorPicksAnExistingBuild(t *testing.T) {
	located := findEngineBinary(t)
	if located == "" {
		t.Skipf("没有找到 %s：这台机器上没构建过引擎（见 engineTestTargetDir 的注释）", engineBinaryName)
	}
	t.Logf("本次测试加载的引擎 = %s", located)
	require.True(t, fileExists(located), "定位到的引擎不存在：%s", located)

	/*
	 * ⚠ The build that knows `api_base` / `mock_script` is the test-provider one.
	 * Naming the file here means a stale selection is visible in the log instead
	 * of surfacing much later as an assertion about behaviour.
	 */
	if !strings.Contains(filepath.ToSlash(located), engineTestTargetDir) {
		t.Logf("⚠ 加载的不是专用测试构建（%s）而是 %s —— 账目与计费测试需要 --features test-provider 的那一份",
			engineTestTargetDir, located)
	}
}

// TestTheEngineReceivesTheLedgerFieldsWeActuallySend closes the loop.
//
// # Why the two halves must be compared **inside one test**
//
// The defect was a name mismatch between the two ends, so the judgement has to be
// "what we sent" against "what the engine says it will dial". Writing the record
// in one test and replaying it in another *looks* equivalent and is not: this
// suite runs `t.Parallel` in places, so a reader can look before the writer has
// written — which is what the first version of these diagnostics did, and it
// skipped every time.
//
// The recording itself is a nil-by-default seam on the production path
// (`engineRequestRecorder`), so the shipped code has no branch that can be asked
// to write request bodies — `api_key` travels in those bodies, and a disk-dump
// switch is a credential leak waiting for an env var to be set by accident.
func TestTheEngineReceivesTheLedgerFieldsWeActuallySend(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireMockableEngine(t)
	writerID := env.idOf["writer"]
	env.mustGrant("admin", writerID, CapabilityWorldIPAI, 0)
	seedQuota(t, writerID, 10_000_000)

	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)

	body := fmt.Sprintf(
		`{"op":"ingest.run","params":{"project_id":%d,"base_version":%d,"source_text":%s,"media":false,"model":%q,"mock_script":%s}}`,
		projectID, version, mustJSONString(t, smallNovel), ingestDefaultModel,
		mustJSONString(t, ingestMockScript))
	rec, payload, sent := env.recordedCallOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	require.NotEmpty(t, sent, "没有录到任何引擎请求 —— 这条断言的整个前提就没了")

	/* ── 我们发了什么 ── */
	sentBase := requestField(sent, "api_base")
	require.Equal(t, "http://example.com/v1", sentBase,
		"❌ 我们根本没把网关地址发出去：这次解析的模型费会落在运营的 token 上，而用户的 quota 照样被扣")

	/* ── 引擎说它会拨哪儿（它自己那份 provider 配置的回显） ── */
	billing, ok := dataMap(t, payload)["billing"].(map[string]any)
	require.True(t, ok, "ingest.run 的返回里没有 billing")

	require.Equal(t,
		sentBase, billing["gateway"],
		"★ 两端对同一个参数用了不同的名字：我们发 %q，引擎说它会拨 %v。参数被静默忽略 —— "+
			"这次解析的模型费记在运营的 token 上，而用户的 quota 照样被扣，两笔账不对账（docs/23 §10 第 15 项）",
		sentBase, billing["gateway"])

	/* ── 而凭据不许回给客户端 ── */
	require.NotContains(t, rec.Body.String(), env.keyOf["writer"],
		"用户的令牌出现在响应里了 —— 凭据绝不许回给客户端")
}

// TestReplayingARecordedRequestReproducesTheSameProvider pins the replay path
// itself: the same bytes through the same helper must give the same answer.
//
// ⚠ It is the *mechanism* check, not the ledger check — the ledger is the test
// above. Its value is diagnostic: when an engine answer looks wrong, having a
// proven replay path is what turns "re-run the whole suite and guess" into
// "send these exact bytes".
func TestReplayingARecordedRequestReproducesTheSameProvider(t *testing.T) {
	env := newTestEnv(t)
	env.seedAccount("admin", common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	requireMockableEngine(t)
	writerID := env.idOf["writer"]
	env.mustGrant("admin", writerID, CapabilityWorldIPAI, 0)
	seedQuota(t, writerID, 10_000_000)

	projectID, version := env.mustSeedWorld("writer", emptyWorldDoc)
	body := fmt.Sprintf(
		`{"op":"ingest.run","params":{"project_id":%d,"base_version":%d,"source_text":%s,"media":false,"model":%q,"mock_script":%s}}`,
		projectID, version, mustJSONString(t, smallNovel), ingestDefaultModel,
		mustJSONString(t, ingestMockScript))
	_, _, sent := env.recordedCallOp("writer", body)
	require.NotEmpty(t, sent)

	answer, err := runSidecar(context.Background(), engineSidecar(), sidecarRequest{Raw: json.RawMessage(sent)})
	require.NoError(t, err, "replay 失败")
	var parsed map[string]any
	require.NoError(t, json.Unmarshal(answer, &parsed), "引擎回答不是 JSON：%s", answer)

	provider, ok := parsed["provider"].(map[string]any)
	require.True(t, ok, "★ replay 的回答里没有 provider —— 引擎那一侧的回显不见了，账目就无法被观测：%s", answer)
	require.Equal(t, requestField(sent, "api_base"), provider["base_url"],
		"★ 同一份字节，两次回答说的目的地不一样 —— replay 这条路本身不可信")
}

// requestField reads one top-level string field out of a raw request body.
func requestField(raw []byte, name string) string {
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	value, _ := obj[name].(string)
	return value
}
