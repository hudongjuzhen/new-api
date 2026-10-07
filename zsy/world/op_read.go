package world

import (
	"encoding/json"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// The read ops: world.get / world.validate (docs/23 §4.2)
//
// Both are login-only. docs/23 §4.2 is explicit about why: they answer "我买过了，
// 我要用我的世界", and putting a capability gate in front of them would only make
// a paying user feel blocked. That is a deliberate exception to the gates in
// op_dispatch.go, not an oversight.
// =========================================================================

// worldGetData is the success payload of world.get.
//
// The document is embedded as raw JSON (`json.RawMessage`), so the bytes the
// engine wrote are the bytes the client receives — Go neither re-orders keys
// nor drops unknown fields. That matters for a format whose authority is a
// separate implementation: a Go round-trip through `map[string]any` would
// silently become a second, lossy definition of MTW (docs/23 §6.4).
type worldGetData struct {
	Op          string          `json:"op"`
	ProjectID   uint            `json:"project_id"`
	ProjectName string          `json:"project_name"`
	Version     int64           `json:"version"`
	Changed     bool            `json:"changed"`
	Doc         json.RawMessage `json:"doc,omitempty"`
}

// opWorldGet implements world.get.
//
//		{ "op": "world.get", "params": { "project_id": 7, "since_version": 3 } }
//
//	  - no since_version (0): always return the current document;
//	  - since_version == current: answer `changed: false` and no `doc`, which is
//	    the "静默重取" path of docs/23 §5.2 — nothing moved, so a multi-megabyte
//	    document stays off the wire;
//	  - since_version > current: E_STALE. A client cannot legitimately hold a
//	    version the server has never issued (restored backup, another deployment,
//	    a hand-edited snapshot), and answering it with an older document would let
//	    it overwrite good data later.
func opWorldGet(c *gin.Context, raw json.RawMessage) (any, error) {
	userID, _ := CurrentUserID(c)

	var params worldGetParams
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}
	if params.ProjectID == 0 {
		return nil, failInput("缺少 project_id：world.get 需要一个世界项目 id")
	}
	if params.SinceVersion < 0 {
		return nil, failInput("since_version 不能为负数")
	}

	// Owner-scoped on purpose: a foreign project answers E_INPUT, so the user
	// face cannot be used to probe which project ids exist (docs/23 §7 is where
	// cross-user reading belongs, behind AdminAuth).
	project, err := WorldProjectGetOwned(params.ProjectID, userID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, failInput("世界项目 %d 不存在或不属于当前账号", params.ProjectID)
		}
		return nil, err
	}

	if params.SinceVersion > project.CurrentVersion {
		return nil, failStale(
			"客户端快照版本 %d 高于服务端当前版本 %d：请重新获取世界后再试",
			params.SinceVersion, project.CurrentVersion)
	}

	data := worldGetData{
		Op:          "world.get",
		ProjectID:   project.ID,
		ProjectName: project.Name,
		Version:     project.CurrentVersion,
	}

	// A project exists before it has a document: project.create lands with an
	// empty world in step 3. Reading it is not an error, it is an empty answer —
	// the same reason 工具1's get_document returns Option<Value>.
	if project.CurrentVersion <= 0 {
		data.Changed = params.SinceVersion != 0
		return data, nil
	}

	if params.SinceVersion == project.CurrentVersion {
		data.Changed = false
		return data, nil
	}

	doc, err := WorldSnapshotDocRaw(project.ID, project.CurrentVersion)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) || errors.Is(err, ErrNoCurrentVersion) {
			return nil, failInput("世界项目 %d 的当前版本 %d 没有对应文档：请联系运营处理",
				project.ID, project.CurrentVersion)
		}
		return nil, err
	}
	data.Changed = true
	data.Doc = json.RawMessage(doc)
	return data, nil
}

// worldValidateData is the success payload of world.validate.
//
// ★ It is a **transport** of the authoritative validator's verdict, not a
// re-statement of it. Every field below is copied verbatim out of
// schema/1.1/validate.mjs's `report[0]` (the object the shim wraps), and the
// mapping is mechanical on purpose: Go renames nothing, adds no count, and takes
// no part in deciding what is wrong. docs/23 §9 step 2 makes "与本地跑
// validate.mjs 逐字一致" the acceptance criterion, and the only way to keep that
// true forever is for this struct to have no opinions.
//
// ⚠ Shape notes that are part of the contract:
//
//	ok            the authoritative verdict: no errors AND no shape errors.
//	              ⚠ It equals the validator's own `ok` for one file — see the
//	              mapping note in opWorldValidate about warnings.
//	shape_ok      the JSON-Schema layer passed.
//	errors        规范 18.3 的 12 项（code + msg pairs, verbatim).
//	warnings      规范 18.3 的 9 项；**不影响 ok**（未加 --strict，见 §4.2）。
//	shape_errors  Ajv messages, verbatim.
//
// ⚠ `file` is deliberately NOT exposed: it is the engine host's temp path, a
// server-internal detail (docs/23 §8.3 ④ — "没有服务端内部标识").
type worldValidateData struct {
	Op          string       `json:"op"`
	OK          bool         `json:"ok"`
	ShapeOK     bool         `json:"shape_ok"`
	ShapeErrors []string     `json:"shape_errors"`
	Errors      []worldIssue `json:"errors"`
	Warnings    []worldIssue `json:"warnings"`
	Strict      bool         `json:"strict"`
	ProjectID   uint         `json:"project_id,omitempty"`
	Version     int64        `json:"version,omitempty"`
	// Engine records which implementation produced the verdict, so a client (and
	// an operator) can never mistake this for 工具1's Rust-side
	// `validate_structure`, which is a *different* and weaker check (docs/23 §4.2).
	Engine string `json:"engine"`
}

// worldIssue is one `{code, msg}` pair. The field names match the validator's
// output exactly (not Go's usual snake_case) so the wire shape needs no
// translation layer and cannot drift from the authority.
type worldIssue struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// validatorReportEntry mirrors one element of validate.mjs's `report` array.
// Only the fields the op transports are declared; extra fields (if the validator
// ever adds one) are ignored rather than guessed at.
type validatorReportEntry struct {
	File        string       `json:"file"`
	ShapeOK     bool         `json:"shapeOk"`
	ShapeErrors []string     `json:"shapeErrors"`
	Errors      []worldIssue `json:"errors"`
	Warnings    []worldIssue `json:"warnings"`
}

// validatorReport mirrors validate.mjs's top-level `--json` object.
type validatorReport struct {
	OK     bool                   `json:"ok"`
	Strict bool                   `json:"strict"`
	Report []validatorReportEntry `json:"report"`
}

// opWorldEntitlements implements world.entitlements.
//
//	{ "op": "world.entitlements", "params": {} }
//
// ★ Why this op exists at all, when `GET /api/zsy/world/entitlements` already
// answers the same question: the desktop client has exactly **one** network path
// to this plugin (`world_op` in Rust — one command, one envelope). Making a
// single read use a second path would mean a second place in the client that
// knows the plugin's URL, and a second place to update when it changes.
//
// ⚠ It is login-only and read-only, and — like the GET route — it is **纯 UX**
// (docs/23 §4.2). It is NOT a gate: a client that deletes this call or fakes its
// answer gains nothing, because the real verdict is recomputed inside
// opDispatch on every op (docs/23 §8.3 ②).
func opWorldEntitlements(c *gin.Context, _ json.RawMessage) (any, error) {
	userID, _ := CurrentUserID(c)
	payload, err := entitlementsPayload(userID)
	if err != nil {
		return nil, failUpstream("能力查询暂时不可用：请稍后重试")
	}
	return payload, nil
}

// opWorldValidate implements world.validate — the authoritative verdict
// (规范 18.3 的 12 项错误 + 9 项警告) for one document.
//
//	{ "op": "world.validate", "params": { "doc": { …MTW… } } }
//	{ "op": "world.validate", "params": { "project_id": 7, "version": 3 } }
//	{ "op": "world.validate", "params": { "project_id": 7 } }   ← current version
//
// # What this function does, and the two things it must never do
//
// It resolves *which* document is being asked about (input, ownership, version),
// hands the bytes to the validator subprocess, and transports the answer.
//
// ⚠ It must never (1) inspect a document's fields to decide anything, or
// (2) adjust the verdict — including "helpfully" upgrading a warning to an error
// or vice versa. Both would create a second definition of a legal document, which
// docs/23 §6.4 forbids in one sentence.
//
// # Login-only, and why that is load-bearing
//
// docs/23 §4.2 puts no capability in front of this op: "我买过了，我要用我的世界".
// The consequence is a real obligation: even an account whose `world-ip` was
// revoked must still be able to validate its own document (enforcement criterion
// C's other half — see §8.3 ①). Adding a gate here would look like consistency
// and would be a contract violation.
func opWorldValidate(c *gin.Context, raw json.RawMessage) (any, error) {
	userID, _ := CurrentUserID(c)

	var params worldValidateParams
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}

	// ── input resolution: which document? ────────────────────────────────────
	var (
		doc       json.RawMessage
		projectID uint
		version   int64
	)
	switch {
	case len(params.Doc) > 0 && string(params.Doc) != "null":
		// The document came with the request. Its *contents* are not inspected
		// here — only that it decodes into a JSON object, which is the transport
		// shape a document has. A JSON string or array in `doc` is a client bug,
		// and catching it here is cheaper than a round trip to the validator. No
		// field of the object is read.
		var probe map[string]any
		if err := common.Unmarshal(params.Doc, &probe); err != nil {
			return nil, failInput("doc 必须是一个 JSON 对象：%v", err)
		}
		doc = params.Doc

	case params.ProjectID != 0:
		// A stored version (or the current one). Ownership is checked before the
		// document is read at all: a foreign version must not even be loaded.
		if _, err := WorldProjectGetOwned(params.ProjectID, userID); err != nil {
			if errors.Is(err, ErrProjectNotFound) {
				return nil, failInput("世界项目 %d 不存在或不属于当前账号", params.ProjectID)
			}
			return nil, err
		}
		var snapshot *WorldSnapshot
		var err error
		if params.Version > 0 {
			snapshot, err = WorldSnapshotGet(params.ProjectID, params.Version)
			if errors.Is(err, ErrSnapshotNotFound) {
				return nil, failInput("世界项目 %d 没有第 %d 版文档", params.ProjectID, params.Version)
			}
		} else {
			snapshot, err = WorldSnapshotCurrent(params.ProjectID)
			if errors.Is(err, ErrSnapshotNotFound) {
				return nil, failInput("世界项目 %d 还没有任何文档版本", params.ProjectID)
			}
		}
		if err != nil {
			return nil, err
		}
		if !isJSONValue(snapshot.Doc) {
			return nil, failInput("世界项目 %d 第 %d 版文档不是合法 JSON", params.ProjectID, snapshot.Version)
		}
		doc = json.RawMessage(snapshot.Doc)
		projectID = params.ProjectID
		version = snapshot.Version

	default:
		return nil, failInput("world.validate 需要 doc，或 project_id（可带 version）")
	}

	// ── transport: the verdict is the validator's ────────────────────────────
	report, err := runValidatorSidecar(c.Request.Context(), doc)
	if err != nil {
		sidecarSysLog("world.validate", err)
		// E_UPSTREAM is exactly this case (docs/23 §4.1): the engine side is
		// unreachable, the client shows the message verbatim and offers a retry.
		return nil, failUpstream("%s", clientFacingSidecarMessage(err))
	}

	var parsed validatorReport
	if err := common.Unmarshal(report, &parsed); err != nil {
		sidecarSysLog("world.validate", err)
		return nil, failUpstream("权威校验器返回的结论无法解析：请检查 %s 是否为本文档对应的版本", cfg.ValidatorSvcPath)
	}
	if len(parsed.Report) == 0 {
		// The op asked about exactly one document, so an empty report means the
		// shim or the validator changed shape. Reported as an upstream fault
		// instead of inventing a verdict.
		return nil, failUpstream("权威校验器没有返回任何结论（report 为空）：请检查 %s", cfg.ValidatorSvcPath)
	}

	// ★ The validator is invoked WITHOUT --strict (see the shim), so its `ok`
	// means "无错误"，warnings do not fail it — which is exactly the §4.2
	// semantics ("12 项错误 + 9 项警告"，警告要报出来但不判失败). Transporting
	// `entry.ShapeOK`/`parsed.OK` unchanged is therefore the whole job.
	entry := parsed.Report[0]
	return worldValidateData{
		Op:          "world.validate",
		OK:          entry.ShapeOK && len(entry.Errors) == 0,
		ShapeOK:     entry.ShapeOK,
		ShapeErrors: nonNilStrings(entry.ShapeErrors),
		Errors:      nonNilIssues(entry.Errors),
		Warnings:    nonNilIssues(entry.Warnings),
		Strict:      parsed.Strict,
		ProjectID:   projectID,
		Version:     version,
		Engine:      "schema/1.1/validate.mjs",
	}, nil
}

// clientFacingSidecarMessage turns a sidecar error into the sentence a client
// should show.
//
// ★ This is a redaction policy, and it is deliberately narrow. The rule:
//
//	a *deployment* or *timeout* failure  → forward verbatim; it names a path and a
//	                                       knob, which is exactly what the person
//	                                       who can fix it needs to read
//	anything else (crash, garbage stdout) → summary only; the raw excerpt stays in
//	                                       the server log, because a Go/Node stack
//	                                       fragment is not something a user or an
//	                                       operator can act on, and §8.3 ④ says no
//	                                       server-internal identifiers go out
//
// The failure mode this avoids: redacting *everything* turns "the engine is not
// deployed, set ZSY_WORLD_VALIDATOR_SVC" into "暂时不可用", which is unfalsifiable
// and sends the operator to the source code. Redacting *nothing* would ship paths
// and stack excerpts. Both directions are pinned by tests.
func clientFacingSidecarMessage(err error) string {
	var unavailable *sidecarUnavailableError
	if errors.As(err, &unavailable) {
		return unavailable.Error()
	}
	var timeout *sidecarTimeoutError
	if errors.As(err, &timeout) {
		return timeout.Error()
	}
	return "权威校验器暂时不可用：请稍后重试；若持续失败请联系运营"
}

// nonNilStrings guarantees a JSON array (never null) for a list field, so a
// client can iterate unconditionally.
func nonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

// nonNilIssues is nonNilStrings for issue lists.
func nonNilIssues(in []worldIssue) []worldIssue {
	if in == nil {
		return []worldIssue{}
	}
	return in
}
