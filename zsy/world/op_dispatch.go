package world

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// The op surface: one entrance, six ops (docs/23 §4.1 / §4.2)
//
//	POST /api/zsy/world/op   { "op": "world.get", "params": { … } }
//
// This file owns exactly three things:
//
//	1. which op names exist, and the capability each one needs;
//	2. the fixed order of a request (input → capability → execute);
//	3. turning a handler error into one of the seven contract codes.
//
// It owns NO world rule. There is no branch here that reads a document, decides
// whether a type is legal, allocates an id, or composes a prompt — docs/23 §6.4
// keeps those in the Rust engine and schema/1.1/validate.mjs, and §11 restates
// it as "规则只有一处". A Go branch that answers "is this a valid MTW" would be
// the plugin failing at its only job.
// =========================================================================

// unsupportedEngineError marks an op that is declared in the contract but whose
// engine call is not wired up yet. It is reported as E_UPSTREAM with a message
// that names the missing piece, never as a fabricated success — a placeholder
// that pretends to work would be worse than a placeholder that admits it.
type unsupportedEngineError struct {
	// Op is the op name that was requested.
	Op string
	// Need names the engine component that has to be reachable first.
	Need string
	// Step is the docs/23 §9 implementation phase that wires it up.
	Step string
}

func (e *unsupportedEngineError) Error() string {
	return fmt.Sprintf(
		"%s 暂不可用：服务端引擎尚未接入（需要 %s；见 docs/23 §9 第 %s 步）。这一步不会返回任何伪造的成功结果。",
		e.Op, e.Need, e.Step)
}

// opFailure is a handler error that already carries its contract code and the
// sentence a client should show verbatim (docs/23 §4.1: the client must not
// parse the Chinese text, but it does display it).
type opFailure struct {
	Code    string
	Message string
}

func (e *opFailure) Error() string { return e.Message }

// failInput / failConflict / failStale / failUpstream build the four failures an
// op handler raises by itself. E_ENTITLEMENT, E_AUTH and E_QUOTA are raised by
// the gate and by the billing path, not by handlers.
func failInput(format string, args ...any) error {
	return &opFailure{Code: CodeInput, Message: fmt.Sprintf(format, args...)}
}

func failConflict(format string, args ...any) error {
	return &opFailure{Code: CodeConflict, Message: fmt.Sprintf(format, args...)}
}

func failStale(format string, args ...any) error {
	return &opFailure{Code: CodeStale, Message: fmt.Sprintf(format, args...)}
}

func failUpstream(format string, args ...any) error {
	return &opFailure{Code: CodeUpstream, Message: fmt.Sprintf(format, args...)}
}

// opHandler runs one op. It returns the value that goes into `data` or a
// classified error. It never writes to the response itself.
type opHandler func(c *gin.Context, params json.RawMessage) (any, error)

// opSpec is one row of the op contract: the externally stable name, the
// capability that gates it, and its implementation.
type opSpec struct {
	// Op is the wire name, spelled exactly as docs/23 §4.2 spells it.
	Op string
	// Capability is the capability required to call it; empty means login-only
	// (docs/23 §4.2: world.get / world.validate are "我买过了，我要用我的世界").
	Capability string
	// Handler is a Go function only where the work is orchestration, storage or
	// authentication. Anything that computes a world is a sidecar call.
	Handler opHandler
}

// opTable is the whole surface, in one place, so "which op needs which
// capability" is auditable by reading eight lines.
//
// ⚠ Steps 1–3 implemented four of the six. The two still missing are declared with
// their real capabilities and a handler that reports "engine not wired yet":
// declaring them costs nothing, and it means the gate is already correct for them
// — an account without world-ip-ai is refused ingest.run today, not after step 4
// lands (enforcement criterion C, docs/23 §8.4).
var opTable = []opSpec{
	// ── 已实现 ──────────────────────────────────────────────────────────
	{Op: "world.get", Capability: "", Handler: opWorldGet},
	{Op: "world.validate", Capability: "", Handler: opWorldValidate},
	// 与 GET /api/zsy/world/entitlements 同一份答案（两个入口，一个出处）。
	// 客户端只有一条网络路（Rust 的 world_op），所以给一条只读的 op 比让客户端
	// 长出第二条路更省事，也少一处"插件的地址"。
	{Op: "world.entitlements", Capability: "", Handler: opWorldEntitlements},
	// 客户端要"从我的世界里挑一个"。没有它，客户端只能"没有就建一个" ——
	// 而那样**每次冷启动都会多出一个空世界**（docs/23 §12.4.6）。
	// 同样登录即可：它答的是"我有哪些世界"，不是"我能不能解析"。
	{Op: "project.list", Capability: "", Handler: opProjectList},
	{Op: "project.create", Capability: CapabilityWorldIP, Handler: opProjectCreate},
	{Op: "world.mutate", Capability: CapabilityWorldIP, Handler: opWorldMutate},
	{Op: "ingest.run", Capability: CapabilityWorldIPAI, Handler: opIngestRun},

	// ── 尚未接（docs/23 §9 的第 3 步尾巴）────────────────────────────────
	{Op: "assets.plan", Capability: CapabilityWorldIPAI, Handler: opNotWiredYet("assets.plan", "素材层（提示词 / 一致性锚点 / 声音规划）的批量入口 + 计费", "3")},
}

// opNotWiredYet returns the honest placeholder of an op whose engine stage has
// not been built. It is a *handler*, so the op still travels the real path:
// entitlement check, input decode, contract code. Only the last mile is missing.
func opNotWiredYet(op string, need string, step string) opHandler {
	return func(_ *gin.Context, _ json.RawMessage) (any, error) {
		return nil, &unsupportedEngineError{Op: op, Need: need, Step: step}
	}
}

// opByName resolves an op name, or nil when the contract has no such op.
func opByName(name string) *opSpec {
	for i := range opTable {
		if opTable[i].Op == name {
			return &opTable[i]
		}
	}
	return nil
}

// knownMutationsText renders the mutation vocabulary for a client-facing message.
func knownMutationsText() string { return "rename / retype / merge / split" }

// knownOpsText renders the op vocabulary for a client-facing message.
func knownOpsText() string {
	out := ""
	for i := range opTable {
		if i > 0 {
			out += " / "
		}
		out += opTable[i].Op
	}
	return out
}

// opDispatch is POST /api/zsy/world/op.
//
// ⚠ The order below is the contract, not an implementation detail:
//
//  1. the op must exist           → E_INPUT
//  2. the capability is re-derived → E_ENTITLEMENT   ← per request, never cached
//  3. the params are decoded       → E_INPUT
//  4. the op runs
//
// The capability check deliberately precedes parameter decoding and every side
// effect. Enforcement criterion A (docs/23 §8.3 ①) says an account without
// `world-ip` must be turned away with a body that contains no document, no
// element and no prompt; a gate that ran after the work would have to *undo*
// the answer, while this one never produces one.
func opDispatch(c *gin.Context) {
	userID, ok := CurrentUserID(c)
	if !ok {
		// requireWorldAuth always sets this. Reaching here means a route was
		// mounted without the auth middleware, which is a wiring bug.
		respondError(c, CodeAuth, "登录状态无效或密钥已失效：请重新登录后再试")
		return
	}

	req, err := decodeOpRequest(c)
	if err != nil {
		respondError(c, CodeInput, err.Error())
		return
	}

	spec := opByName(strings.TrimSpace(req.Op))
	if spec == nil {
		respondErrorf(c, CodeInput, "未知的 op %q；本接口支持：%s", req.Op, knownOpsText())
		return
	}

	// ── The gate. Every op, every time, straight from the database. ─────────
	if spec.Capability != "" {
		allowed, entitlementErr := WorldEntitlementActive(userID, spec.Capability)
		if entitlementErr != nil {
			common.SysError(fmt.Sprintf("zsy-world: capability check for user %d op %s failed: %v", userID, spec.Op, entitlementErr))
			respondError(c, CodeUpstream, "能力校验暂时不可用：请稍后重试")
			return
		}
		if !allowed {
			respondErrorf(c, CodeEntitlement,
				"当前账号没有 %q 能力，无法执行 %s：请先获取该能力后再试。", spec.Capability, spec.Op)
			return
		}
	}

	data, err := spec.Handler(c, req.Params)
	if err != nil {
		respondOpError(c, spec.Op, err)
		return
	}
	respondOK(c, data)
}

// decodeOpRequest reads and decodes the op envelope, enforcing the body cap.
func decodeOpRequest(c *gin.Context) (*opRequest, error) {
	if c.Request.ContentLength > cfg.OpBodyLimitBytes {
		return nil, fmt.Errorf("请求体过大：单个 op 不能超过 %d 字节", cfg.OpBodyLimitBytes)
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, cfg.OpBodyLimitBytes)
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, fmt.Errorf("请求体过大：单个 op 不能超过 %d 字节", cfg.OpBodyLimitBytes)
		}
		return nil, fmt.Errorf("读取请求体失败：%v", err)
	}
	if len(strings.TrimSpace(string(raw))) == 0 {
		return nil, fmt.Errorf("请求体为空：需要 {\"op\": \"world.get\", \"params\": {…}}")
	}
	var req opRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("请求体不是合法 JSON：%v", err)
	}
	if strings.TrimSpace(req.Op) == "" {
		return nil, fmt.Errorf("缺少 op 字段；本接口支持：%s", knownOpsText())
	}
	return &req, nil
}

// respondOpError maps a handler error onto one of the seven contract codes.
//
// Unknown errors become E_UPSTREAM rather than a fifth code: the contract's
// vocabulary is closed (docs/23 §4.1), and an unexpected server-side failure is
// exactly the "原样显示 + 重试按钮" case the client already knows how to draw.
func respondOpError(c *gin.Context, op string, err error) {
	var failure *opFailure
	if errors.As(err, &failure) {
		respondError(c, failure.Code, failure.Message)
		return
	}
	var notWired *unsupportedEngineError
	if errors.As(err, &notWired) {
		respondError(c, CodeUpstream, notWired.Error())
		return
	}
	common.SysError(fmt.Sprintf("zsy-world: op %s failed: %v", op, err))
	respondError(c, CodeUpstream, "服务端处理失败：请稍后重试")
}

// decodeParams binds an op's params into its own struct. A field that belongs to
// another op fails here instead of being ignored, which is what makes the
// E_INPUT message actionable ("客户端按 E_INPUT 的原话去改", docs/23 §4.3).
func decodeParams(raw json.RawMessage, target any) error {
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "null" {
		// An op with no optional fields still needs its required ones, so an
		// absent params object is reported by the op's own validation.
		raw = json.RawMessage("{}")
	}
	if err := common.Unmarshal(raw, target); err != nil {
		return failInput("参数不合法：%v", err)
	}
	return nil
}
