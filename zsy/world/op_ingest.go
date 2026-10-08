package world

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/gin-gonic/gin"
)

// usingGroupContextKey is the host's own "which group is this request billed in"
// key. The plugin reads the host's key rather than inventing one, so a host that
// starts setting it (or a future plugin auth path that does) is picked up for free.
const usingGroupContextKey = constant.ContextKeyUsingGroup

// =========================================================================
// ingest.run — 导入 + 分章 + 题材识别 / 类型发现 / 元素抽取（docs/23 §4.2）
//
// ★ 这是唯一一条**会花钱**的 op：引擎真的去调模型。所以它比别的 op 多三件事：
//
//	1. 能力是 `world-ip-ai`（不是 `world-ip`）—— AI 那部分有真实边际成本（§6.3）；
//	2. 跑之前**查余额**，不够直接 `E_QUOTA`（不是 `E_ENTITLEMENT`，两个码两件事）；
//	3. 跑完按**实际 token** 记账 —— 复用 new-api 已有的那本账（§6.5）。
//
// ⚠ 编排顺序是有意的，不能换：
//
//	判权（在 op_dispatch）→ 输入/所有权 → base_version → **查余额** → 引擎 → **闸门** → 结算 → 落库
//
// 查余额在引擎之前（别让没钱的账号烧掉模型调用）；结算在闸门之后（被闸门拒掉的
// 结果不落库，但**模型调用已经发生**，那笔钱照记）。
// =========================================================================

// ingestRunData is the success payload of ingest.run.
type ingestRunData struct {
	Op        string          `json:"op"`
	ProjectID uint            `json:"project_id"`
	Version   int64           `json:"version"`
	Doc       json.RawMessage `json:"doc"`
	// Result mirrors the engine's own counts (chapters_parsed / elements /
	// relations / prompts / anchors / remaining_chapters / structure_ok).
	Result json.RawMessage `json:"result"`
	Notes  []string        `json:"notes"`
	// Progress is the engine's progress event list. ⚠ It is a *finished* list, not
	// a stream: this op is one request/one response. Real-time progress for the UI
	// is a client-side design topic (docs/23 §12.4.4).
	Progress []json.RawMessage `json:"progress"`
	Warnings []worldIssue      `json:"warnings"`
	// Billing is the account's side of what happened. The client shows it; the
	// operator reconciles it against the host's consume log.
	Billing ingestBilling `json:"billing"`
}

// ingestBilling is what the caller is told about the spend.
//
// ★ It reports **tokens** (what the engine actually measured) and the **gateway**
// they were dialled through — but deliberately **no money amount**.
//
// ⚠ Why `quota` is gone (2026-10-08): this struct used to carry a number the plugin
// computed from the engine's tokens with the **local** `ModelRatio` table, and the
// failure message printed it as "本次已消耗 N 积分". Two things were wrong with it:
//
//  1. **它不是积分。** 那是宿主的内部 `quota` 单位，写成"积分"差一个 `QuotaPerUnit`
//     （默认 500,000）：用户当时在「积分记录」里看到 2.23154 积分，而消息说 757166。
//  2. **它是第二笔账。** 模型调用打回本机中继、用的是发起者的令牌，中继已经把那笔钱
//     扣掉了（见 billing.go 文件头）。那一跑实测中继扣 111,577 quota，插件自己算出
//     757,166 —— 两张价目表，差 6.8 倍。
//
// So "这一次花了多少" is now answered by the gateway's own ledger, where the user can
// see every call（「积分记录」）。 A plugin-side number can only ever be a second opinion,
// and this one was measurably the wrong one.
type ingestBilling struct {
	Model            string `json:"model"`
	Group            string `json:"group"`
	Calls            int    `json:"calls"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
	// Gateway is the LLM endpoint the engine actually dialled (docs/23 §10 item 15).
	//
	// ★★ It exists so "which ledger did the model fee land on" is **in the data**
	// rather than reconstructed afterwards. Under option B this is always this
	// deployment's own relay (`…/v1`), because that is what makes the model fee
	// and the user's quota a single charge. Anything else means the run was
	// booked to whoever owns the engine's `AIPOLE_API_KEY` — a difference that
	// otherwise only shows up as an unexplained margin, long after the fact.
	//
	// ⚠ It is a **destination**, not a credential: the token is never reported,
	// in either direction (the engine reports only whether one was supplied).
	Gateway string `json:"gateway"`
	// UsageByStage keeps the engine's per-stage breakdown so "钱花在哪了" survives
	// all the way to whoever asks (docs/23 §6.5).
	UsageByStage json.RawMessage `json:"usage_by_stage"`
}

// stageUsage mirrors the engine's `StageUsage` (runner::StageUsage).
//
// It is typed — unlike most engine payloads, which this plugin transports raw —
// because billing must *read* these numbers. That is not a rule about the world:
// it is arithmetic on the account's money, which is the host's business and
// therefore legitimately the plugin's.
type stageUsage struct {
	Stage            string `json:"stage"`
	Calls            int    `json:"calls"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
}

// opIngestRun implements ingest.run.
//
//	{ "op": "ingest.run", "params": {
//	    "project_id": 7, "base_version": 3, "source_text": "……",
//	    "max_chapters": 5, "media": true, "model": "gpt-5-nano" } }
func opIngestRun(c *gin.Context, raw json.RawMessage) (any, error) {
	userID, _ := CurrentUserID(c)

	var params ingestRunParams
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateIngestRun(&params); err != nil {
		return nil, err
	}

	project, err := WorldProjectGetOwned(params.ProjectID, userID)
	if err != nil {
		if errors.Is(err, ErrProjectNotFound) {
			return nil, failInput("世界项目 %d 不存在或不属于当前账号", params.ProjectID)
		}
		return nil, err
	}
	if params.BaseVersion != project.CurrentVersion {
		return nil, failConflict(
			"版本不一致：客户端基于第 %d 版，服务端当前是第 %d 版。请重新获取世界后再解析（否则这次解析会建在旧注册表上，ID 会漂）",
			params.BaseVersion, project.CurrentVersion)
	}

	// The base document, verbatim: the engine takes its registry from here so that
	// ids do not drift across runs.
	var baseDoc json.RawMessage
	if project.CurrentVersion > 0 {
		baseDoc, err = WorldSnapshotDocRaw(project.ID, project.CurrentVersion)
		if err != nil && !errors.Is(err, ErrSnapshotNotFound) {
			return nil, err
		}
	}

	// ── 2. affordability, BEFORE spending anything ─────────────────────────
	group := ingestingGroup(c, params.Group)
	if err := requireQuota(userID, estimateIngestQuotaFloor(params)); err != nil {
		return nil, err
	}

	/*
	 * ── 2.5 which ledger this run is booked against (docs/23 §10 item 15) ──
	 *
	 * ★★ Both values are derived from **this request**, and both must be
	 * present for the model fee to land on the caller's account:
	 *
	 *	api_base — this host's own relay base. The engine dials it, so the
	 *	           model call comes back through this deployment's relay.
	 *	api_key  — the token that authenticated this op. The relay therefore
	 *	           deducts **this user's** quota.
	 *
	 * ⚠ Ordering matters: this runs BEFORE the engine is started, because its
	 * failure mode is "the run would be booked to the wrong account". Starting
	 * the engine first and discovering that afterwards would already have spent
	 * the operator's money.
	 *
	 * Without this pair the engine falls back to `AIPOLE_API_KEY` and dials the
	 * upstream gateway — the operator pays, the user's quota is still deducted,
	 * and nothing reconciles the two. That is the defect option B removes; see
	 * gateway.go for the full reasoning and for why a client-chosen Host is not
	 * an escalation.
	 */
	apiBase, err := requireGatewayBaseURL(c)
	if err != nil {
		logLedgerRouting("")
		return nil, err
	}
	logLedgerRouting(apiBase)
	apiKey := callerToken(c)

	// ── 3. the engine spends the money ─────────────────────────────────────
	extra := map[string]any{
		"source_text": params.SourceText,
		// Media defaults to ON: it is the default in the engine's own RunOptions, and
		// "解析但不做素材" is the exception rather than the rule.
		"media": params.Media == nil || *params.Media,
		/*
		 * ★ The two ledger fields. The engine's semantics (and the parameter
		 * names) are documented in its own `service.rs::op_ingest`; this side
		 * only supplies them. Nothing here decides whether a credential is
		 * acceptable — the engine's provider config owns that.
		 */
		"api_base": apiBase,
	}
	if apiKey != "" {
		extra["api_key"] = apiKey
	}
	putIfNonEmpty(extra, "novel_title", params.NovelTitle)
	putIfNonEmpty(extra, "model", params.Model)
	if params.MaxChapters > 0 {
		extra["max_chapters"] = params.MaxChapters
	}
	if len(params.MockScript) > 0 {
		// Test-only; the engine ignores it unless built with `--features
		// test-provider`. Passing it through unconditionally keeps this op's code
		// free of build-tag branches — the decision about whether a mock is
		// acceptable belongs to whoever built the engine binary.
		extra["mock_script"] = params.MockScript
	}
	if len(baseDoc) > 0 {
		extra["base_doc"] = baseDoc
	}

	engineDoc, err := callEngine(c.Request.Context(), engineRequest{
		Op:    "ingest",
		Extra: extra,
	})
	if err != nil {
		sidecarSysLog("ingest.run", err)
		var unavailable *sidecarUnavailableError
		if errors.As(err, &unavailable) {
			return nil, failUpstream("%s", clientFacingSidecarMessage(err))
		}
		var refused *sidecarRefusedError
		if errors.As(err, &refused) {
			// The engine's own refusal. Most common cause by far: the gateway has no
			// credentials, or the model name is not one the gateway offers — both are
			// operator-facing, and the engine's sentence already says which.
			return nil, failUpstream("%s", refused.Detail.Message)
		}
		return nil, failUpstream("%s", clientFacingSidecarMessage(err))
	}

	newDoc, err := engineField(engineDoc, "doc")
	if err != nil {
		sidecarSysLog("ingest.run", err)
		return nil, failUpstream("世界引擎没有返回解析结果：请检查引擎版本与 %s 是否匹配", envEnginePath)
	}
	result, _ := engineField(engineDoc, "result")
	if len(result) == 0 {
		result = json.RawMessage("{}")
	}
	usageStages := engineStageUsage(engineDoc)
	notes := engineStringArray(engineDoc, "notes")
	progress := engineRawArray(engineDoc, "progress")

	// ── 4. the gate (docs/23 §10 item 10/11) ───────────────────────────────
	//
	// ★ 这里**没有结算**，而且是刻意的：模型调用打回本机中继、用的是发起者的令牌，
	// 中继已经按它自己的价目表逐次扣过那个用户的额度了（billing.go 文件头记着那一跑
	// 两笔账的实测差额）。插件再扣一次，就是同一次解析收两次钱。
	verdict, err := gateOnAuthority(c.Request.Context(), newDoc)
	if err != nil {
		sidecarSysLog("ingest.run", err)
		return nil, failUpstream("%s", clientFacingSidecarMessage(err))
	}
	if !verdict.OK {
		// ⚠ 措辞有讲究：**不给金额**。这一跑的钱是中继扣的，插件手里没有那个数，
		// 猜一个出来就是又一次"对不上账的提示"——那条"已消耗 757166 积分"就是这么来的。
		// 能给、也该给的是"钱已经花了，而且看得见花在哪"这件事本身。
		return nil, failInput(
			"解析结果未通过权威校验，已放弃落库（项目仍是第 %d 版）。"+
				"⚠ 这一次的模型调用已经发生、也已经由网关计费（逐笔明细见「积分记录」），"+
				"但结果没有落库：%s",
			project.CurrentVersion, verdict.Summary())
	}

	// ── 6. commit ──────────────────────────────────────────────────────────
	snapshot, err := WorldSnapshotAppend(project.ID, newDoc, "full_run")
	if err != nil {
		return nil, err
	}

	calls, promptTokens, completionTokens := usageTotals(usageStages)
	byStage, _ := engineField(engineDoc, "usage_by_stage")
	if len(byStage) == 0 {
		byStage = json.RawMessage("[]")
	}
	/*
	 * ★ Preferred source is the engine's own echo (it knows what its provider
	 * config resolved to, including a default we did not send). Falling back to
	 * the value we sent keeps this honest if an older engine does not report it.
	 */
	gateway, _ := engineProviderField(engineDoc, "base_url")
	if gateway == "" {
		gateway = apiBase
	}
	return ingestRunData{
		Op:        "ingest.run",
		ProjectID: project.ID,
		Version:   snapshot.Version,
		Doc:       json.RawMessage(newDoc),
		Result:    result,
		Notes:     notes,
		Progress:  progress,
		Warnings:  verdict.Warnings,
		Billing: ingestBilling{
			Model:            ingestingModel(params.Model),
			Group:            group,
			Calls:            calls,
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			Gateway:          gateway,
			UsageByStage:     byStage,
		},
	}, nil
}

// engineProviderField reads one string out of the engine's `provider` block — the
// part of ingest's answer that says **where the model calls were sent**.
//
// ★ It answers "" instead of an error for every malformed case: this block is
// reporting, not payload. An engine too old to emit it (or emitting it oddly)
// must not fail an otherwise successful parse — the caller falls back to the
// value it sent. Contrast `doc`, which IS the payload and is required.
//
// ⚠ The engine reports a destination here, never a credential, so nothing in
// this block needs redacting. The second return value says whether the field was
// present at all, so "absent" stays distinguishable from "present but empty".
func engineProviderField(engineDoc json.RawMessage, field string) (string, bool) {
	provider, err := engineField(engineDoc, "provider")
	if err != nil {
		return "", false
	}
	value, err := engineField(provider, field)
	if err != nil {
		return "", false
	}
	var text string
	if err := json.Unmarshal(value, &text); err != nil {
		return "", false
	}
	/* ⚠ Only surrounding whitespace is trimmed; an address with internal spaces
	   is passed through verbatim so a misconfiguration stays visible. */
	return strings.TrimSpace(text), true
}

// validateIngestRun checks the request's own shape — not world rules.
func validateIngestRun(params *ingestRunParams) error {
	if params.ProjectID == 0 {
		return failInput("缺少 project_id")
	}
	if params.BaseVersion <= 0 {
		return failInput(
			"缺少 base_version（必须是你读到的那一版号）：没有它服务端无法判断你是不是在别人的新版本上解析（docs/23 §4.4）")
	}
	if len(params.SourceText) == 0 {
		return failInput("缺少 source_text：ingest.run 需要小说原文")
	}
	return nil
}

// ingestDefaultModel is the model this deployment asks the engine for when the
// caller does not name one.
//
// ⚠ It must be a name the **gateway** actually serves: the model calls go back
// through this deployment's own relay with the caller's token (docs/23 §12.13),
// and the relay prices what it serves. The plugin itself no longer needs a
// 「模型倍率」 entry to bill — it does not bill at all (billing.go 文件头).
const ingestDefaultModel = "gpt-5-nano"

func ingestingModel(requested string) string {
	if requested != "" {
		return requested
	}
	return ingestDefaultModel
}

// ingestingGroup reads the group the request should be billed in.
//
// It follows the host's own precedence: an explicit request parameter wins, else
// the group the account key resolves to (`constant.ContextKeyUsingGroup`, which
// middleware.TokenAuth sets), else "default".
//
// ⚠ The plugin's own auth middleware does not set the host's using-group key (it
// authenticates through `model.ValidateUserToken` but keeps its own context keys),
// so in practice this resolves to the request parameter or "default". That is a
// deliberate simplification with a visible consequence: a deployment that prices
// groups differently must pass `group` explicitly. Recorded in docs/23 §12.4.4.
func ingestingGroup(c *gin.Context, requested string) string {
	if requested != "" {
		return requested
	}
	if g := common.GetContextKeyString(c, usingGroupContextKey); g != "" {
		return g
	}
	return "default"
}

// estimateIngestQuotaFloor is the affordability floor checked before the run.
//
// ⚠ It is a **floor, not a quote**, and deliberately conservative: a small fixed
// allowance per chapter, converted through the host's formula. Its job is to stop
// an obviously-broke account from burning model calls, not to predict the bill.
// The real charge is the measured token usage afterwards.
//
// ⚠ It does NOT go through `ratio_setting` for the model ratio, because at this
// point the goal is "is the account at zero" rather than an accurate price — and
// an accurate price would need `common.QuotaPerUnit` × a ratio, which is exactly
// the number that can be wrong at configuration time. A fixed floor cannot be
// misconfigured.
func estimateIngestQuotaFloor(params ingestRunParams) int {
	chapters := params.MaxChapters
	if chapters <= 0 {
		// Unknown chapter count (the engine splits it): assume a small batch so the
		// floor stays a floor instead of a full-novel estimate.
		chapters = 3
	}
	const floorPerChapter = 200
	return int(chapters) * floorPerChapter
}

// engineStageUsage reads the engine's `usage_by_stage` as typed rows.
//
// A missing or oddly shaped field yields an empty slice, which makes the charge
// zero — and a zero charge is then visible to the caller as `quota: 0` rather than
// silently wrong. (The engine always emits this field; the lenient path exists so a
// shape change degrades into "no charge" instead of a 500.)
func engineStageUsage(payload json.RawMessage) []stageUsage {
	raw, err := engineField(payload, "usage_by_stage")
	if err != nil {
		return nil
	}
	var stages []stageUsage
	if err := json.Unmarshal(raw, &stages); err != nil {
		return nil
	}
	return stages
}

// engineRawArray reads an optional array field, keeping each element raw.
func engineRawArray(payload json.RawMessage, field string) []json.RawMessage {
	raw, err := engineField(payload, field)
	if err != nil {
		return []json.RawMessage{}
	}
	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return []json.RawMessage{}
	}
	if items == nil {
		return []json.RawMessage{}
	}
	return items
}
