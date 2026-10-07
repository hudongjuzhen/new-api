package world

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// GET /api/zsy/world/entitlements — which capabilities this account holds.
//
// docs/23 §4.2 is explicit that this endpoint is **纯 UX**: it exists so the
// client can draw the right entrance ("去购买 / 兑换" vs. the real screen). It is
// NOT a gate and must never be treated as one — a client that deletes the call,
// or hard-codes the answer, gains exactly nothing, because the verdict is
// recomputed inside opDispatch on every op (docs/23 §8.3 ②).
//
// It reads through the same WorldCapabilitiesActive the gate reads, so the two
// can never disagree.
//
// ⚠★ There are two ways in, on purpose — the GET route here, and the
// `world.entitlements` op (op_read.go):
//
//	GET  /api/zsy/world/entitlements   for a browser or a plain HTTP caller
//	POST /api/zsy/world/op  {op:…}     for the desktop client, which has exactly
//	                                   one network path (`world_op` in Rust) and
//	                                   should not grow a second one for a single
//	                                   read
//
// Both build their answer through entitlementsPayload, so the two cannot drift.
// =========================================================================

func getEntitlements(c *gin.Context) {
	userID, ok := CurrentUserID(c)
	if !ok {
		respondError(c, CodeAuth, "登录状态无效或密钥已失效：请重新登录后再试")
		return
	}
	payload, err := entitlementsPayload(userID)
	if err != nil {
		respondError(c, CodeUpstream, "能力查询暂时不可用：请稍后重试")
		return
	}
	respondOK(c, payload)
}

// entitlementsPayload builds the capability answer.
//
// It returns the error so each caller can answer in its own envelope (the GET
// route uses respondError, the op path raises an opFailure) — but the *content*
// is built in exactly one place.
func entitlementsPayload(userID int) (gin.H, error) {
	capabilities, err := WorldCapabilitiesActive(userID)
	if err != nil {
		common.SysError(fmt.Sprintf("zsy-world: list capabilities of user %d failed: %v", userID, err))
		return nil, err
	}
	return gin.H{
		"user_id":      userID,
		"capabilities": capabilities,
		// The vocabulary, so a client can offer the right purchase entries
		// without shipping a copy of this plugin's constants.
		"known": KnownCapabilities,
	}, nil
}
