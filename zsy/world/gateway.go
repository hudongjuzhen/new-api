package world

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// ★★ Where the money is booked (docs/23 §10 item 15 — decided: option B)
//
// `ingest.run` makes the engine call a model. By default the engine reads
// `AIPOLE_API_KEY` from its environment and dials the upstream gateway — which
// means the model fee lands on the **operator's** token while this plugin
// separately deducts the user's `quota`. Two ledgers, no reconciliation: a
// wrong model ratio is absorbed by the operator and **nothing in the books
// shows it**.
//
// Option B closes that gap with two values travelling together on every
// `ingest.run`:
//
//	api_base = this host's own relay base   → the model call comes back here
//	api_key  = the token that made this op  → so the charge lands on that user
//
// The engine already had the explicit-credential path (`RunOptions.api_key`,
// `ProviderConfig.with_api_key`); what it lacked was a `base_url` knob, which
// is why this file pairs with `service.rs`'s `base_url` parameter. Nothing in
// either place decides *what a legal request is* — this is account arithmetic,
// which docs/23 §6.4 leaves to the host by design.
//
// # What this file deliberately does NOT do
//
//   - It does not validate the host against an allow-list. The value is derived
//     from the request's own `Host`, never from the body, so there is no
//     attacker-chosen address to validate. A `Host` header the client controls
//     can only make the engine dial *the client's own hostname* — see the
//     reasoning in gatewayBaseURL.
//   - It does not cache. Deriving is two field reads.
//   - It does not fall back to the operator's upstream when derivation fails.
//     Silently switching ledgers is precisely the defect this file exists to
//     remove, so failing to derive is E_UPSTREAM (retryable, operator-facing).
// =========================================================================

// relayBasePathSuffix is appended to the host to form the base URL.
//
// ⚠ `/v1` is part of the **OpenAI-compatible address**, not decoration:
// `ProviderConfig.chat_completions_url()` appends `/chat/completions`, and the
// host's relay routes live under `/v1` (`router/relay-router.go`). A base URL
// without it would ask for `<host>/chat/completions`, which is not a route.
const relayBasePathSuffix = "/v1"

// gatewayBaseURL derives the OpenAI-compatible base URL of **this deployment**,
// for the engine to call back into.
//
// It returns "" when the request carries no usable host. Callers must treat that
// as a refusal (see requireGatewayBaseURL) rather than as "use the default".
//
// # Why the request's own Host, and not configuration
//
// The alternative is an operator-set env var (`ZSY_WORLD_GATEWAY_BASE`) naming
// the public origin. That is one more thing to configure, and one more thing to
// get wrong in a way that *silently* reverts to charging the wrong account.
// The request already knows which hostname the caller reached, and that
// hostname is by construction one that routes to this process.
//
// ⚠★ The one adversarial case, stated honestly: a caller may send an arbitrary
// `Host` header. Then this returns a base URL pointing at *that* host, and the
// engine's model call goes there carrying **the caller's own token**.
//
//	- It cannot make the engine read a local file or reach an internal-only
//	  address on behalf of a *different* user: the token in the `Authorization`
//	  header is the caller's own, so the worst case is "I made my own parse
//	  bill somebody else's gateway".
//	- It is not a new capability: the same caller can already point its own
//	  client at any host it likes. Nothing the plugin holds is disclosed.
//   - The failure mode is a failed model call on their next op, reported to
//     them.
//
// The host's own CORS/session code reasons the same way about `request.Host`
// (`middleware/auth_origin.go`), which is why no proxy-header handling is
// invented here: behind a reverse proxy the `Host` header is the public one.
func gatewayBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}
	host := strings.TrimSpace(c.Request.Host)
	if host == "" || strings.ContainsAny(host, " \t/\\") {
		return ""
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	/*
	 * ⚠ `X-Forwarded-Proto` is honoured **only to upgrade**: a TLS-terminating
	 * proxy is the normal deployment, and without this the engine would dial
	 * `http://` at a host that only speaks `https://` — a failure that looks
	 * like an engine fault.
	 *
	 * It never downgrades: a client claiming "http" over a TLS connection is
	 * either confused or probing, and believing it would send the caller's
	 * token in clear text to the same host.
	 */
	if scheme == "http" && strings.EqualFold(strings.TrimSpace(c.GetHeader("X-Forwarded-Proto")), "https") {
		scheme = "https"
	}

	return scheme + "://" + host + relayBasePathSuffix
}

// requireGatewayBaseURL is gatewayBaseURL plus the refusal.
//
// The message names what could not be read and what to do, because the operator
// is the only one who can act on it — the same convention as every other
// deployment fault in this plugin (`clientFacingSidecarMessage`).
func requireGatewayBaseURL(c *gin.Context) (string, error) {
	base := gatewayBaseURL(c)
	if base == "" {
		return "", failUpstream(
			"无法从这次请求推断本机网关地址（Host 头为空或不合法），"+
				"因此不能把模型调用转回本机记账。请从站点域名访问本接口后重试",
		)
	}
	return base, nil
}

// callerToken returns the account key that authenticated this request.
//
// ★ It is the **user's own** token, not an operator key: `ingest.run` passes it
// to the engine so the model call is billed to whoever asked for the parse
// (docs/23 §10 item 15). `bearerToken` is the same extraction the auth gate
// uses, so "which keys are valid" stays defined in one place.
//
// ⚠ It travels to the engine on stdin and into `ProviderConfig`, whose `Debug`
// is hand-written and masks it. It is never logged and never returned to a
// client — `ingest.run`'s response carries counts and quota, not credentials.
func callerToken(c *gin.Context) string {
	return bearerToken(c)
}

// logLedgerRouting records which ledger an ingest is about to be booked against.
//
// It logs the **destination**, never the token: "which account paid" is the one
// fact an operator needs when reconciling, and the token is the one fact that
// must never reach a log file.
func logLedgerRouting(baseURL string) {
	if baseURL == "" {
		common.SysError("zsy-world: ingest.run: no gateway base URL could be derived; refusing rather than charging the operator's upstream token")
		return
	}
	common.SysLog(fmt.Sprintf("zsy-world: ingest.run: model calls will be billed through %s (the caller's own token)", baseURL))
}
