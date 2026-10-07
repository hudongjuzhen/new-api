package world

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// =========================================================================
// The error-code contract (docs/23 §4.1)
//
// These seven literals are the *cross-language* contract: a client branches on
// `code` and never on the Chinese message text. They are therefore spelled
// exactly as the design document spells them, and no eighth code may be added
// without changing that document first.
//
// ⚠ Two of them exist because "capability" and "quota" are different questions
// (docs/23 §6.5):
//
//	E_ENTITLEMENT  this account may not use this capability at all
//	E_QUOTA        it may, but there is not enough credit left
//
// They must never be merged: the client draws "去购买" for the first and
// "积分不足（还差 N）" for the second.
// =========================================================================
const (
	// CodeOK is the success code. Error codes below are the failure vocabulary.
	CodeOK = "OK"

	// CodeEntitlement: the account lacks the capability this op requires.
	CodeEntitlement = "E_ENTITLEMENT"
	// CodeQuota: the account has the capability but not enough credit.
	CodeQuota = "E_QUOTA"
	// CodeAuth: not logged in, or the key is no longer valid.
	CodeAuth = "E_AUTH"
	// CodeInput: the request itself is malformed or references something absent.
	CodeInput = "E_INPUT"
	// CodeConflict: the caller's base_version is behind the server.
	CodeConflict = "E_CONFLICT"
	// CodeStale: the caller's snapshot is older than the server's current state.
	CodeStale = "E_STALE"
	// CodeUpstream: the engine (Rust sidecar / Node validator / LLM gateway)
	// failed. The client shows the message verbatim and offers a retry.
	CodeUpstream = "E_UPSTREAM"
)

// httpStatusForCode is this plugin's one place where a contract code becomes an
// HTTP status. docs/23 pins only two of them (403 for E_ENTITLEMENT in §8.3 ①,
// "引导重新登录" for E_AUTH); the rest follow the common sense of the table in
// §4.1 and are asserted by world_test.go so they cannot drift silently.
func httpStatusForCode(code string) int {
	switch code {
	case CodeEntitlement:
		return http.StatusForbidden
	case CodeAuth:
		return http.StatusUnauthorized
	case CodeQuota:
		return http.StatusPaymentRequired
	case CodeConflict, CodeStale:
		return http.StatusConflict
	case CodeInput:
		return http.StatusBadRequest
	case CodeUpstream:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// respondOK writes a success envelope: {success, code:"OK", message:"", data}.
//
// `code` is included on success on purpose — docs/23 §4.1 shows it in both
// branches, and a client that switches on `code` should not need a separate
// shape for the happy path.
func respondOK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"code":    CodeOK,
		"message": "",
		"data":    data,
	})
}

// respondError writes a failure envelope carrying a contract code.
//
// ⚠ `data` is deliberately absent on every failure. Enforcement criterion A
// (docs/23 §8.3 ①) is "the body contains no document, no element, no prompt":
// the cheapest way to keep that true forever is for the failure branch to have
// no place to put content at all.
func respondError(c *gin.Context, code string, message string) {
	c.JSON(httpStatusForCode(code), gin.H{
		"success": false,
		"code":    code,
		"message": message,
	})
}

// respondErrorf is respondError with a formatted message.
func respondErrorf(c *gin.Context, code string, format string, args ...any) {
	respondError(c, code, fmt.Sprintf(format, args...))
}
