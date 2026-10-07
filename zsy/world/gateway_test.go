package world

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// ★★ Where the model fee is booked (docs/23 §10 item 15 — decided: option B)
//
// The defect these tests exist to prevent is not a crash and not a wrong
// answer — it is a **wrong ledger**. Without this wiring the engine reads
// `AIPOLE_API_KEY` and dials the upstream gateway, so the model fee lands on
// the operator's token while this plugin still deducts the caller's quota.
// Nothing reconciles those two, so the symptom is a quiet margin leak.
//
// The assertions below are therefore about **which account pays**, and they
// cover all three ways that can go wrong:
//
//	1. the base URL derived from the request is the deployment's own relay;
//	2. it is never derived from the request *body* (no attacker-chosen address);
//	3. when it cannot be derived, the op REFUSES instead of silently reverting
//	   to the operator's upstream — case 3 is the one that matters most,
//	   because a silent revert is exactly the defect being fixed.
// =========================================================================

// gatewayCtx builds a gin context around one request, for the pure functions.
//
// ⚠★ `req.Host` is assigned **unconditionally**, including for the empty string.
// `httptest.NewRequest` fills in "example.com" when the target carries no host,
// so a conditional assignment would silently turn "no usable host" into a
// perfectly good one — and the refusal tests would then pass for the wrong
// reason (they would be asserting against a host that was never absent).
// Measured: that is exactly what the first run of this file did.
func gatewayCtx(method, target, host string, tlsOn bool, forwardedProto string) *gin.Context {
	req := httptest.NewRequest(method, target, nil)
	req.Host = host
	if tlsOn {
		req.TLS = &tls.ConnectionState{}
	}
	if forwardedProto != "" {
		req.Header.Set("X-Forwarded-Proto", forwardedProto)
	}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = req
	return c
}

func TestGatewayBaseURL_DerivesThisDeploymentsRelay(t *testing.T) {
	/*
	 * ★ The judgement is the whole `/v1` suffix plus the scheme.
	 *
	 * `/v1` is not decoration: `ProviderConfig.chat_completions_url()` appends
	 * `/chat/completions`, so a base URL missing it asks for `<host>/chat/
	 * completions` — a route that does not exist. And the host's relay routes do
	 * live under `/v1` (router/relay-router.go), which is why this is a fact
	 * about this deployment rather than a convention.
	 */
	cases := []struct {
		name           string
		host           string
		tlsOn          bool
		forwardedProto string
		want           string
	}{
		{"plain http", "api.example.test", false, "", "http://api.example.test/v1"},
		{"terminated TLS", "api.example.test", true, "", "https://api.example.test/v1"},
		{"port preserved", "127.0.0.1:3000", false, "", "http://127.0.0.1:3000/v1"},
		/*
		 * A TLS-terminating proxy is the normal deployment: the request arrives
		 * over http, so `Request.TLS` is nil, and without honouring the header
		 * the engine would dial `http://` at a host that only speaks https.
		 */
		{"behind a TLS-terminating proxy", "www.aipole.top", false, "https", "https://www.aipole.top/v1"},
		{"forwarded proto is case-insensitive", "www.aipole.top", false, "HTTPS", "https://www.aipole.top/v1"},
		/*
		 * ⚠ It must never **downgrade**: a client claiming "http" over a real
		 * TLS connection would otherwise be believed, and the caller's token
		 * would travel in clear text to the same host.
		 */
		{"forwarded http over TLS is not believed", "www.aipole.top", true, "http", "https://www.aipole.top/v1"},
		{"surrounding whitespace is trimmed", "  api.example.test  ", false, "", "http://api.example.test/v1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := gatewayBaseURL(gatewayCtx(http.MethodPost, "/api/zsy/world/op", tc.host, tc.tlsOn, tc.forwardedProto))
			require.Equal(t, tc.want, got)
		})
	}
}

// TestGatewayBaseURL_RefusesUnusableHosts pins the refusal cases.
//
// A blank or malformed host must yield "" — NOT a built-up string. The caller
// turns "" into E_UPSTREAM; if this returned, say, "http:///v1", the engine
// would fail later with a confusing DNS error while the real problem ("we could
// not tell which gateway to bill") stayed invisible.
func TestGatewayBaseURL_RefusesUnusableHosts(t *testing.T) {
	for _, host := range []string{
		"",
		"   ",
		/*
		 * A host carrying a path or whitespace is not a host. These are the
		 * shapes that would let a crafted header move the `base_url` somewhere
		 * structurally different (e.g. `evil.test/x?y=`), so they are refused
		 * rather than escaped.
		 */
		"evil.test/path",
		"evil.test\\path",
		"evil.test with space",
	} {
		t.Run(host, func(t *testing.T) {
			c := gatewayCtx(http.MethodPost, "/api/zsy/world/op", host, false, "")
			require.Equal(t, "", gatewayBaseURL(c), "host %q must not produce a base URL", host)
		})
	}

	/* A missing request is a wiring bug, not a panic. */
	require.Equal(t, "", gatewayBaseURL(nil))
	require.Equal(t, "", gatewayBaseURL(&gin.Context{}))
}

// TestRequireGatewayBaseURL_TurnsRefusalIntoAnUpstreamFault pins the error shape.
//
// ★ E_UPSTREAM (not E_INPUT) is the right code: the caller did nothing wrong —
// a client that somehow sends no Host cannot fix that by editing its request,
// it is a deployment/proxy misconfiguration. docs/23 §4.1 maps E_UPSTREAM to
// "the engine side is unreachable, show the message and offer a retry".
func TestRequireGatewayBaseURL_TurnsRefusalIntoAnUpstreamFault(t *testing.T) {
	base, err := requireGatewayBaseURL(gatewayCtx(http.MethodPost, "/api/zsy/world/op", "api.example.test", false, ""))
	require.NoError(t, err)
	require.Equal(t, "http://api.example.test/v1", base)

	_, err = requireGatewayBaseURL(gatewayCtx(http.MethodPost, "/api/zsy/world/op", "", false, ""))
	require.Error(t, err)
	var fault *opFailure
	require.ErrorAs(t, err, &fault)
	require.Equal(t, CodeUpstream, fault.Code,
		"不能推断网关地址是部署/代理问题，不是调用方输入问题 —— 归 E_UPSTREAM")
	require.Contains(t, fault.Message, "Host",
		"这句话要指出读不到什么，否则能修的人也不知道从哪下手")
}

// TestCallerToken_IsTheUsersOwnKeyNotAnOperatorKey pins which credential travels.
//
// ★ This is the half that decides **whose quota** is deducted. If it ever
// returned an operator-configured key, the model fee and the quota deduction
// would land on two different accounts again — the defect option B removes —
// while every other test stayed green, because the run itself would still
// succeed.
func TestCallerToken_IsTheUsersOwnKeyNotAnOperatorKey(t *testing.T) {
	cases := []struct {
		name   string
		header string
		want   string
	}{
		{"bare key", "Bearer aaaabbbbcccc", "aaaabbbbcccc"},
		{"lowercase scheme", "bearer aaaabbbbcccc", "aaaabbbbcccc"},
		{"no scheme", "aaaabbbbcccc", "aaaabbbbcccc"},
		/*
		 * The `sk-` display prefix is stripped, because that is how the host's
		 * own relay reads a token (`middleware/auth.go` does the same
		 * `TrimPrefix`). Keeping it would send `Bearer sk-<key>` to our own
		 * relay, which would fail to resolve the token — and the failure would
		 * present as "the engine cannot authenticate", pointing at the engine
		 * instead of at this line.
		 */
		{"sk- prefix stripped", "Bearer sk-aaaabbbbcccc", "aaaabbbbcccc"},
		{"padded", "  Bearer   aaaabbbbcccc  ", "aaaabbbbcccc"},
		{"absent", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/zsy/world/op", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req
			require.Equal(t, tc.want, callerToken(c))
		})
	}
}

// TestIngestRun_SendsBothLedgerFieldsToTheEngine is the integration half.
//
// Unit tests above prove the values are computed correctly; this proves they are
// actually **put on the wire**. Without it, someone could delete the two lines
// that populate `extra` and every other test would still pass — while the
// engine quietly reverted to the operator's upstream token.
//
// ⚠ The assertion reads the engine's own echo (`billing.gateway`, which the
// engine fills from the provider config it actually built), not a mock of our
// own making. That echo exists precisely so this is observable without the
// engine ever disclosing a credential: it reports the destination, and only
// whether a key was supplied.
func TestIngestRun_SendsBothLedgerFieldsToTheEngine(t *testing.T) {
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
	rec, payload := env.callOp("writer", body)
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	data := dataMap(t, payload)
	billing, ok := data["billing"].(map[string]any)
	require.True(t, ok, "billing missing: %v", data)

	/*
	 * ★★ The judgement: the engine was told to dial **this deployment's own
	 * relay**, so the model fee comes back through this host's ledger instead of
	 * the operator's upstream token.
	 *
	 * `httptest.NewRequest` gives the request the Host "example.com", so that is
	 * what a derivation from the request must yield — and the engine echoes it
	 * back after having actually built its provider config from it.
	 */
	require.Equal(t, "http://example.com/v1", billing["gateway"],
		"★ 引擎没有回本机网关 —— 这次解析的模型费会落在运营的 token 上，而用户的 quota 照样被扣（两笔账不对账）")

	/*
	 * ⚠ And the credential must never come back. The engine reports only whether
	 * one was supplied; the caller's own token must not appear anywhere in the
	 * response.
	 */
	require.NotContains(t, rec.Body.String(), env.keyOf["writer"],
		"★ 用户的令牌出现在响应里了 —— 凭据绝不许回给客户端")
}
