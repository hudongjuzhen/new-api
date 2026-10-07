package world

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
)

// =========================================================================
// The sidecar boundary (docs/23 §6.4 option A)
//
// The engine — the extraction rules, the type system, the id strategy, the
// consistency algorithm, the validator — lives in two subprocesses:
//
//	Rust   world-parser-svc        the method
//	Node   world-validator-svc.mjs the authoritative verdict
//
// This file is the Go half of that boundary and it owns exactly four things:
//
//	how a request is framed      one JSON object in, one JSON object out
//	how long it may take         cfg.SidecarTimeout, enforced by context
//	how a failure is classified  exit code + stdout shape → a contract code
//	what the operator is told    the message must name the path to configure
//
// ⚠ It owns NO rule. There is no branch here that inspects a document, decides
// whether an element type is legal, or re-derives a verdict. docs/23 §6.4: "Go 里
// 出现第二份『什么算合法 MTW』就是错的." The single most important consequence:
// **a validation verdict is never computed here — it is transported.** The only
// thing Go decides is "did the engine answer", which is not a world question.
// =========================================================================

// sidecarRequest is the wire envelope both sidecars accept on stdin.
//
// Two ways in, and the difference is not cosmetic:
//
//	Doc   the document to act on. Typed as raw JSON so the bytes the engine wrote
//	      are the bytes the engine receives — a round trip through `any` would
//	      re-encode the document, and re-encoding is the beginning of a second
//	      definition of the format.
//	Raw   a pre-marshalled body, used by callEngine so that this file never has to
//	      know any op's parameter list.
type sidecarRequest struct {
	Op  string `json:"op,omitempty"`
	Doc any    `json:"doc,omitempty"`
	// Params is unused by the current ops and kept out of the way.
	Params json.RawMessage `json:"params,omitempty"`
	// Raw, when set, is sent verbatim and the fields above are ignored.
	Raw json.RawMessage `json:"-"`
}

// sidecarResponse is the wire envelope both sidecars answer with on stdout.
//
// ⚠★ The two engines were built at different times and originally disagreed here:
// the Node shim answered `{"ok":true,"report":…}` while the Rust service answered
// `{"status":"ok",…}`. Both were self-consistent, and the mismatch only surfaced
// when Go tried to call the Rust one — it read "no `ok` field" as a failure with no
// reason and reported E_UPSTREAM (docs/23 §12.7 deviation 19).
//
// The fix is to make the SHAPE one thing rather than to teach Go two shapes:
//
//	{"status":"ok",    …op-specific payload fields at the top level…}
//	{"status":"error", "error":{"code","message"}}
//
// `status` is the field both sides already had in the Rust case and the one the
// Go side now reads. `Report` is the rest of the object, verbatim — Go does not
// enumerate the fields, because enumerating them would be Go maintaining a mirror
// of the engine's result shapes.
type sidecarResponse struct {
	Status string          `json:"status"`
	Error  *sidecarError   `json:"error,omitempty"`
	Report json.RawMessage `json:"-"`
}

// UnmarshalJSON accepts the canonical shape and, for one release of tolerance,
// the older `{"ok":true,"report":…}` shape the Node shim used to emit.
//
// ⚠ The tolerance is deliberate rather than lazy: a deployment may update new-api
// and the engine repo at different moments, and a strict decoder would turn that
// ordinary sequencing into "the validator is unavailable".
//
// ⚠⚠ And the decoder is NOT strict about unknown fields — that was a bug worth
// recording: `DisallowUnknownFields` made the Rust engine's perfectly good answer
// fail, because its payload carries the op's own fields (`doc`, `engine`,
// `novel_title`) alongside the envelope. Rejecting those would mean Go maintaining
// a whitelist of every op's output shape, which is exactly the mirror-of-the-engine
// this design avoids. What IS checked is the one thing that makes the answer
// readable at all: that the envelope names a status.
func (r *sidecarResponse) UnmarshalJSON(data []byte) error {
	type envelope struct {
		Status string          `json:"status"`
		OK     *bool           `json:"ok"`
		Error  *sidecarError   `json:"error"`
		Report json.RawMessage `json:"report"`
	}

	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return err
	}

	switch {
	case env.Status != "":
		r.Status = env.Status
	case env.OK != nil:
		// Legacy Node shim shape: `ok` replaced `status`.
		if *env.OK {
			r.Status = "ok"
		} else {
			r.Status = "error"
		}
	default:
		return fmt.Errorf("world: sidecar answer has neither status nor ok: %s", trimForLog(string(data), 200))
	}
	r.Error = env.Error

	// The payload is the whole object (minus the envelope fields) in the canonical
	// shape, or the nested `report` in the legacy one. Both are carried as raw JSON
	// so nothing is re-encoded on the way through.
	if len(env.Report) > 0 {
		r.Report = env.Report
		return nil
	}
	var whole map[string]json.RawMessage
	if err := json.Unmarshal(data, &whole); err != nil {
		return err
	}
	delete(whole, "status")
	delete(whole, "ok")
	delete(whole, "error")
	trimmed, err := json.Marshal(whole)
	if err != nil {
		return err
	}
	r.Report = trimmed
	return nil
}

// sidecarError is a protocol-level failure reported *by the sidecar itself* (as
// opposed to a non-zero exit with no parseable stdout).
type sidecarError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *sidecarError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// sidecarRefusedError means the sidecar understood the request and declined it.
//
// ★ This type exists so callers can answer E_INPUT instead of E_UPSTREAM. The
// distinction is the one docs/23 §4.1 draws: "参数不合法 → 把服务端那句话原样显示"
// versus "引擎出错 → 原样显示 + 重试按钮". Retrying "注册表里没有 char_999" can never
// succeed, so offering a retry button for it would be actively misleading.
type sidecarRefusedError struct {
	What   string
	Detail *sidecarError
}

func (e *sidecarRefusedError) Error() string {
	return fmt.Sprintf("%s拒绝了这次请求：%s", e.What, e.Detail.Error())
}

func (e *sidecarRefusedError) Unwrap() error { return e.Detail }

// sidecarUnavailableError means the plugin could not even start the engine.
//
// It is a distinct type because the operator-facing message differs: this is a
// *deployment* problem (a path is not configured, or a binary is missing), and
// the message must say which path — otherwise the failure surfaces as an opaque
// spawn error and the operator has to read the source to find out what to set.
type sidecarUnavailableError struct {
	// What names the component in operator language ("权威校验器").
	What string
	// Path is what the plugin tried to run or import.
	Path string
	// EnvVar is the knob that fixes it.
	EnvVar string
	// Err is the underlying cause, kept for the log (not for the client).
	Err error
}

func (e *sidecarUnavailableError) Error() string {
	return fmt.Sprintf("%s不可用：找不到或无法执行 %s（%v）；请把 %s 指向它",
		e.What, e.Path, e.Err, e.EnvVar)
}

func (e *sidecarUnavailableError) Unwrap() error { return e.Err }

// sidecarTimeoutError means the call outlived cfg.SidecarTimeout.
//
// It is its own type because it is the one failure whose *fix* is a knob rather
// than a deployment, and because the client-facing sentence is actionable — so it
// survives redaction (see clientFacingSidecarMessage).
type sidecarTimeoutError struct {
	What    string
	Path    string
	Timeout time.Duration
	EnvVar  string
}

func (e *sidecarTimeoutError) Error() string {
	return fmt.Sprintf("%s超时（超过 %s；%s 可调）：%s", e.What, e.Timeout, e.EnvVar, e.Path)
}

// sidecarSpec describes how to launch one sidecar.
//
// The two engines launch differently, and the difference is not incidental:
//
//	the validator is a Node *script*  → nodeBinary + [shimPath]   (the interpreter runs it)
//	the Rust engine is a *binary*     → enginePath + []           (no interpreter)
//
// Collapsing the two would mean special-casing "is this file executable" by
// extension, which is exactly the kind of guess this layer must not make.
type sidecarSpec struct {
	// What names the component in operator language.
	What string
	// Binary is the program to execute (an interpreter, or the engine itself).
	Binary string
	// Args are fixed arguments placed before any Go-side arguments.
	Args []string
	// EnvVar is the knob an operator sets to fix a missing piece.
	EnvVar string
	// CheckPath is a filesystem path that must exist before launching, reported as
	// a deployment problem when it does not.
	//
	// ★ This exists because `exec.Command` cannot detect a missing *script*: when
	// `node <missing>.mjs` runs, node itself starts fine and then exits 1 with a
	// module-not-found on stderr. Without this check the failure would be
	// classified as "the engine crashed", the actionable path would be swallowed
	// by the generic E_UPSTREAM message, and an operator would have no way to tell
	// "not deployed yet" from "deployed and broken". Verified by test.
	CheckPath string
}

// validatorSidecar is how this deployment launches the authoritative validator.
func validatorSidecar() sidecarSpec {
	return sidecarSpec{
		What:      "权威校验器",
		Binary:    cfg.NodeBinary,
		Args:      []string{cfg.ValidatorSvcPath},
		EnvVar:    envValidatorSvc,
		CheckPath: cfg.ValidatorSvcPath,
	}
}

// engineSidecar is how this deployment launches the Rust engine (docs/23 §9
// steps 3–4 consume this; nothing in step 2 does).
func engineSidecar() sidecarSpec {
	return sidecarSpec{
		What:      "世界引擎",
		Binary:    cfg.EnginePath,
		EnvVar:    envEnginePath,
		CheckPath: cfg.EnginePath,
	}
}

// runSidecar executes one sidecar call and returns its raw `report` payload.
//
// Failure taxonomy — every branch is deliberate, and none of them is "guess":
//
//	the program cannot run                   → *sidecarUnavailableError (E_UPSTREAM, names the env var)
//	the sidecar answers {ok:false,error}    → error                    (E_UPSTREAM, its own message)
//	the sidecar exceeds cfg.SidecarTimeout  → error                    (E_UPSTREAM, "超时")
//	the sidecar exits non-zero with no JSON → error                    (E_UPSTREAM, stderr excerpt)
//	the sidecar answers {ok:true,report}    → nil                      (caller transports the verdict)
//
// ★ A non-zero exit code is NOT automatically a failure. The validator
// legitimately exits 1 to mean "this document has errors" — docs/23 §4.2 makes
// that a *result*, not an outage. Only "non-zero AND no parseable answer" is an
// outage.
func runSidecar(ctx context.Context, spec sidecarSpec, req sidecarRequest) (json.RawMessage, error) {
	// A missing piece is checked BEFORE launching, precisely because the
	// interpreter case cannot be detected from the launch error (see sidecarSpec).
	if spec.CheckPath != "" {
		if _, err := os.Stat(spec.CheckPath); err != nil {
			return nil, &sidecarUnavailableError{
				What: spec.What, Path: spec.CheckPath, EnvVar: spec.EnvVar, Err: err,
			}
		}
	}

	payload := req.Raw
	if len(payload) == 0 {
		encoded, err := json.Marshal(req)
		if err != nil {
			return nil, fmt.Errorf("world: encode %s request: %w", spec.What, err)
		}
		payload = encoded
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.SidecarTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, spec.Binary, spec.Args...)
	cmd.Stdin = bytes.NewReader(payload)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	// The sidecars read UTF-8 JSON. Go passes bytes through untouched, and no
	// shell is involved: exec.Command runs the program directly, so a path with
	// spaces or a document containing quotes can never be reinterpreted.
	cmd.Env = os.Environ()

	runErr := cmd.Run()

	// A spawn failure of the *interpreter itself* (node not installed) is a
	// deployment problem too, and is reported with the same actionable shape.
	var execErr *exec.Error
	if errors.As(runErr, &execErr) {
		return nil, &sidecarUnavailableError{What: spec.What, Path: spec.Binary, EnvVar: envNodeBinary, Err: execErr.Err}
	}
	if errors.Is(runErr, exec.ErrNotFound) {
		return nil, &sidecarUnavailableError{What: spec.What, Path: spec.Binary, EnvVar: envNodeBinary, Err: runErr}
	}

	// A timeout leaves no usable answer; distinguish it from a crash so the
	// operator knows to raise the knob rather than hunt a bug. It carries its own
	// type because "raise ZSY_WORLD_SIDECAR_TIMEOUT_SECONDS" is an instruction a
	// client can act on, so it is forwarded rather than redacted.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, &sidecarTimeoutError{
			What:    spec.What,
			Path:    spec.CheckPath,
			Timeout: cfg.SidecarTimeout,
			EnvVar:  envSidecarTimeout,
		}
	}

	out := strings.TrimSpace(stdout.String())

	// The sidecar always answers with one JSON object on stdout, including when it
	// fails — that is the whole point of the protocol. An empty stdout therefore
	// means it died before answering (panic, missing module, killed), and its
	// stderr is the only diagnostic worth forwarding.
	if out == "" {
		excerpt := trimForLog(stderr.String(), 400)
		if excerpt == "" {
			excerpt = "无 stderr"
		}
		return nil, fmt.Errorf("%s没有输出结论（退出码 %s）：%s", spec.What, exitCodeText(runErr), excerpt)
	}

	var resp sidecarResponse
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		return nil, fmt.Errorf("%s的结论不是合法 JSON（退出码 %s）：%v；stdout 片段：%s",
			spec.What, exitCodeText(runErr), err, trimForLog(out, 400))
	}

	if resp.Status != "ok" {
		if resp.Error != nil {
			// The sidecar named the problem itself: pass its words through, and
			// keep the typed error so the caller can tell "the engine refused this
			// request" from "the engine is unreachable".
			return nil, &sidecarRefusedError{What: spec.What, Detail: resp.Error}
		}
		return nil, fmt.Errorf("%s报告失败，但没有给出原因（退出码 %s）", spec.What, exitCodeText(runErr))
	}
	if len(resp.Report) == 0 {
		return nil, fmt.Errorf("%s声称成功但没有给出结论（退出码 %s）", spec.What, exitCodeText(runErr))
	}
	return resp.Report, nil
}

// runValidatorSidecar runs the authoritative validator through its shim.
func runValidatorSidecar(ctx context.Context, doc json.RawMessage) (json.RawMessage, error) {
	return runSidecar(ctx, validatorSidecar(), sidecarRequest{Op: "validate", Doc: doc})
}

// ---------------------------------------------------------------------------
// The Rust engine
// ---------------------------------------------------------------------------

// engineRequest is one call to the Rust engine.
//
// `Extra` carries the op's own fields (`novel_title`, `kind`, `element_id`, …).
// It stays a map on purpose: this layer must not learn any op's parameter list,
// or it would start looking like a place where an op's semantics live.
type engineRequest struct {
	Op    string
	Doc   json.RawMessage
	Extra map[string]any
}

// engineRequestRecorder, when non-nil, observes the exact request body about to
// be sent to the Rust engine.
//
// ★ It is a package-level seam **for tests only** and defaults to nil, so the
// production path has no branch and no way to be asked to record: a switch here
// that wrote request bodies to disk would be a credential leak waiting for an
// environment variable to be set by accident (`api_key` travels in these bodies).
// The test harness sets it, records one payload, and restores nil.
//
// Nothing reads it to make a decision — it cannot change behaviour, only observe it.
var engineRequestRecorder func([]byte)

// callEngine asks the Rust engine to do something and returns its raw payload.
//
// The wire shape is the one `world-parser-svc` speaks (see
// `apps/world-parser/src/bin/svc.rs`): one JSON object per line, `status:"ok"` or
// `status:"error"`. A response of `status:"error"` is the *engine's own* refusal
// (an unknown id, a registry inconsistency, a bad field) and is reported as
// E_INPUT: the request did not make sense to the engine, and the engine's own
// sentence is the most useful thing to show.
//
// ⚠ E_UPSTREAM stays for "the engine is unreachable" — a deployment problem. The
// two must not be conflated: a client that retries an E_UPSTREAM may succeed,
// while retrying "注册表里没有 char_999" never will.
func callEngine(ctx context.Context, req engineRequest) (json.RawMessage, error) {
	body := map[string]any{"op": req.Op}
	if len(req.Doc) > 0 {
		body["doc"] = req.Doc
	}
	for k, v := range req.Extra {
		body[k] = v
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("world: encode engine request %s: %w", req.Op, err)
	}
	if engineRequestRecorder != nil {
		engineRequestRecorder(payload)
	}

	raw, err := runSidecar(ctx, engineSidecar(), sidecarRequest{Raw: payload})
	if err != nil {
		return nil, err
	}
	return raw, nil
}

// engineField pulls one field out of an engine payload.
func engineField(payload json.RawMessage, field string) (json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return nil, fmt.Errorf("world: engine payload is not an object: %w", err)
	}
	value, ok := obj[field]
	if !ok || len(value) == 0 || string(value) == "null" {
		return nil, fmt.Errorf("world: engine payload has no %q field", field)
	}
	return value, nil
}

// engineStringArray reads an optional array-of-strings field, answering an empty
// (never nil) slice when the field is absent or of another shape.
//
// ⚠ It is deliberately lenient. `notes` and similar fields are *commentary* the
// engine produces for a human — a missing or oddly-shaped one must not fail an
// otherwise successful operation. Contrast with `doc`, which is the payload and is
// required (see engineField's caller).
func engineStringArray(payload json.RawMessage, field string) []string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return []string{}
	}
	raw, ok := obj[field]
	if !ok {
		return []string{}
	}
	var items []string
	if err := json.Unmarshal(raw, &items); err != nil {
		return []string{}
	}
	if items == nil {
		return []string{}
	}
	return items
}

// indexOf is strings.Index, kept local so this file's imports stay minimal and the
// call site reads as one thought.
func indexOf(s, sub string) int { return strings.Index(s, sub) }

// exitCodeText renders a process result for a log line without claiming a code
// that does not exist (a timeout or a signal leaves no exit status).
func exitCodeText(runErr error) string {
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return fmt.Sprintf("%d", exitErr.ExitCode())
	}
	if runErr == nil {
		return "0"
	}
	return "未知"
}

// trimForLog shortens a capture for a single-line message. It is applied to
// stderr/stdout excerpts only — never to a document, which is not logged at all.
func trimForLog(s string, limit int) string {
	compact := strings.Join(strings.Fields(s), " ")
	if len(compact) <= limit {
		return compact
	}
	return compact[:limit] + "…"
}

// sidecarSysLog records a sidecar failure server-side, where an operator can find
// it, without putting internals in the client's response.
func sidecarSysLog(op string, err error) {
	common.SysError(fmt.Sprintf("zsy-world: op %s: sidecar failure: %v", op, err))
}
