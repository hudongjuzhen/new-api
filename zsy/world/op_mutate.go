package world

import (
	"encoding/json"
	"errors"

	"github.com/gin-gonic/gin"
)

// =========================================================================
// world.mutate — 改类型 / 改名 / 合并 / 拆分（docs/23 §4.2）
//
// 四种改法都在引擎里（`id_engine` / `extract` 的库函数），Go 只做编排：
//
//	base_version 冲突判定 → 调引擎 → **过权威校验闸门** → 落库 → 返回
//
// ★ 闸门是这一条 op 的关键（§10 第 10 项，按 A 做）：改完的文档必须先过权威校验，
// 不合格就**不落库**。见 gate.go 与 docs/23 §12.3。
//
// ⚠ 一条必须说清的语义：`world.mutate` **不猜**怎么改元素的实例字段。
// 拆分只分配新 ID（库函数明确不切 `metadata`/提示词/声音，规范 19.4.1 属人工编辑），
// 改派只改类型与 ID/引用。所以"改了之后文档不合法"是**可能发生的正常结果**，
// 而闸门的作用正是把它变成一句能照做的提示，而不是一份静悄悄落库的坏数据。
// =========================================================================

// worldMutateData is the success payload of world.mutate.
//
// `doc` is the full new document: a mutation rewrites references across the whole
// document (a rename can touch relations, chapters, events, anchors), so returning
// a delta would push the reconciliation onto the client — and the client is not
// allowed to compute anything about the format.
type worldMutateData struct {
	Op        string          `json:"op"`
	Kind      string          `json:"kind"`
	ProjectID uint            `json:"project_id"`
	Version   int64           `json:"version"`
	Doc       json.RawMessage `json:"doc"`
	Notes     []string        `json:"notes"`
	Warnings  []worldIssue    `json:"warnings"`
	// Result carries the engine's op-specific outcome (new_id, kept_id,
	// relations_rewritten, explanation …) exactly as the engine reported it.
	//
	// ⚠ It is not re-typed into a Go struct on purpose: the field set differs per
	// kind, and enumerating it here would mean Go maintaining a mirror of the
	// engine's result shapes — exactly the drift this design exists to avoid.
	Result json.RawMessage `json:"result"`
}

// opWorldMutate implements world.mutate.
//
//	{ "op": "world.mutate", "params": {
//	    "project_id": 7, "base_version": 3, "kind": "rename",
//	    "element_id": "char_001", "new_name": "新名字" } }
//
// Capability-gated by `world-ip` (docs/23 §4.2).
//
// # base_version is mandatory, and that is the point
//
// docs/23 §4.4: without it the write is "last writer wins", and that loss is
// SILENT. A client that has not read the document cannot know what it is changing,
// so a missing base_version is a client bug, not a convenience — reported as
// E_INPUT with the current version so the client can refetch.
func opWorldMutate(c *gin.Context, raw json.RawMessage) (any, error) {
	userID, _ := CurrentUserID(c)

	var params worldMutateParams
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateWorldMutate(&params); err != nil {
		return nil, err
	}

	project, err := WorldProjectGetOwned(params.ProjectID, userID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, failInput("世界项目 %d 不存在或不属于当前账号", params.ProjectID)
		}
		return nil, err
	}

	// ── the concurrency contract (docs/23 §4.4) ────────────────────────────
	if params.BaseVersion != project.CurrentVersion {
		return nil, failConflict(
			"版本不一致：客户端基于第 %d 版，服务端当前是第 %d 版。请重新获取世界后再改（服务端不会替你决定保留哪一份）",
			params.BaseVersion, project.CurrentVersion)
	}

	// The current document, verbatim — Go never decodes or inspects its fields.
	doc, err := WorldSnapshotDocRaw(project.ID, project.CurrentVersion)
	if err != nil {
		if errors.Is(err, ErrSnapshotNotFound) || errors.Is(err, ErrNoCurrentVersion) {
			return nil, failInput("世界项目 %d 还没有任何文档版本：请先创建内容再改", project.ID)
		}
		return nil, err
	}

	// ── the engine does the change ─────────────────────────────────────────
	engineDoc, err := callEngine(c.Request.Context(), engineRequest{
		Op:    "mutate",
		Doc:   doc,
		Extra: mutateEngineArgs(&params),
	})
	if err != nil {
		sidecarSysLog("world.mutate", err)
		// Unreachable vs. refused are different answers for the client, so they are
		// checked by TYPE rather than by matching text.
		var unavailable *sidecarUnavailableError
		if errors.As(err, &unavailable) {
			return nil, failUpstream("%s", clientFacingSidecarMessage(err))
		}
		var refused *sidecarRefusedError
		if errors.As(err, &refused) {
			// The engine's sentence is the most useful thing to show, and its
			// messages are written for a person to act on ("刷新一下元素列表再试").
			return nil, failInput("%s", refused.Detail.Message)
		}
		var timedOut *sidecarTimeoutError
		if errors.As(err, &timedOut) {
			return nil, failUpstream("%s", clientFacingSidecarMessage(err))
		}
		return nil, failInput("世界引擎未能完成这次改动；详细原因见服务端日志")
	}

	newDoc, err := engineField(engineDoc, "doc")
	if err != nil {
		sidecarSysLog("world.mutate", err)
		return nil, failUpstream("世界引擎没有返回改动后的文档：请检查引擎版本与 %s 是否匹配", envEnginePath)
	}
	notes := engineStringArray(engineDoc, "notes")

	// `result` is informative; its absence must not fail an otherwise good
	// mutation, so it degrades to an empty object rather than an error.
	result, err := engineField(engineDoc, "result")
	if err != nil {
		result = json.RawMessage("{}")
	}

	// ── ★ THE GATE (docs/23 §10 item 10, decided: option A) ────────────────
	verdict, err := gateOnAuthority(c.Request.Context(), newDoc)
	if err != nil {
		sidecarSysLog("world.mutate", err)
		return nil, failUpstream("%s", clientFacingSidecarMessage(err))
	}
	if !verdict.OK {
		// ★ Nothing is written. The project keeps its current version, and the
		// client is told what the authority objected to — in the authority's words.
		return nil, failInput(
			"这次改动会让世界不合法，已放弃（服务端没有写入任何东西，第 %d 版仍然是当前版本）：%s",
			project.CurrentVersion, verdict.Summary())
	}

	// ── commit: a NEW version, never an update in place ────────────────────
	// 与工具1 的 `save_document` 一致：每一版都留着，回退 = 以新的一版写入
	// （docs/23 §6.3）。`reason` 直接用 kind，与工具1 的 vocabulary 同一套。
	snapshot, err := WorldSnapshotAppend(project.ID, newDoc, params.Kind)
	if err != nil {
		return nil, err
	}

	return worldMutateData{
		Op:        "world.mutate",
		Kind:      params.Kind,
		ProjectID: project.ID,
		Version:   snapshot.Version,
		Doc:       json.RawMessage(newDoc),
		Notes:     notes,
		Warnings:  verdict.Warnings,
		Result:    result,
	}, nil
}

// mutateEngineArgs turns the request's params into the engine's field set,
// **omitting the fields this kind does not use**.
//
// ⚠ Omitting matters: the engine's own reader treats an absent field as "not
// supplied" but an empty string as "supplied and empty" (e.g. `keep: ""` would
// fail its "is this one of the two ids" check). Sending the whole struct with
// zero values would therefore turn every rename into a merge that errors.
func mutateEngineArgs(p *worldMutateParams) map[string]any {
	args := map[string]any{"kind": p.Kind}
	putIfNonEmpty(args, "element_id", p.ElementID)
	putIfNonEmpty(args, "new_name", p.NewName)
	putIfNonEmpty(args, "to_type", p.ToType)
	putIfNonEmpty(args, "reason", p.Reason)
	putIfNonEmpty(args, "id_a", p.IDA)
	putIfNonEmpty(args, "id_b", p.IDB)
	putIfNonEmpty(args, "keep", p.Keep)
	putIfNonEmpty(args, "id", p.ID)
	putIfNonEmpty(args, "new_type_id", p.NewTypeID)
	if len(p.NewAlias) > 0 {
		args["new_alias"] = p.NewAlias
	}
	if p.AllowCrossType {
		args["allow_cross_type"] = true
	}
	if p.FirstChapter > 0 {
		args["first_chapter"] = p.FirstChapter
	}
	return args
}

func putIfNonEmpty(m map[string]any, key string, value string) {
	if value != "" {
		m[key] = value
	}
}

// validateWorldMutate checks the transport shape and the per-kind required fields.
//
// ⚠ These are the *request's* own shape requirements (which field addresses what),
// not world rules. "kind=rename needs an element_id" is a statement about this
// API's parameters; whether that element exists, and whether the rename is legal,
// is the engine's business.
func validateWorldMutate(params *worldMutateParams) error {
	if params.ProjectID == 0 {
		return failInput("缺少 project_id")
	}
	if params.BaseVersion <= 0 {
		return failInput(
			"缺少 base_version（必须是你读到的那一版号）：没有它服务端无法判断你是不是在改别人的新版本（docs/23 §4.4）")
	}
	switch params.Kind {
	case "rename":
		if params.ElementID == "" {
			return failInput("kind=rename 需要 element_id")
		}
		if params.NewName == "" {
			return failInput("kind=rename 需要 new_name")
		}
	case "retype":
		if params.ElementID == "" {
			return failInput("kind=retype 需要 element_id")
		}
		if params.ToType == "" {
			return failInput("kind=retype 需要 to_type")
		}
	case "merge":
		if params.IDA == "" || params.IDB == "" {
			return failInput("kind=merge 需要 id_a 与 id_b")
		}
		if params.IDA == params.IDB {
			return failInput("kind=merge 的 id_a 与 id_b 不能是同一个元素")
		}
	case "split":
		if params.ID == "" {
			return failInput("kind=split 需要 id")
		}
		if params.NewName == "" {
			return failInput("kind=split 需要 new_name（新实体的名字）")
		}
	default:
		return failInput("不支持的 kind %q；支持：%s", params.Kind, knownMutationsText())
	}
	return nil
}
