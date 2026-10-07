package world

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"
)

// =========================================================================
// project.create — 建一个世界项目（docs/23 §4.2）
//
// ★ 这一条 op 有一半是"服务端记账"（Go 的活：插一行、给所有者、记时间），
// 另一半是**空文档**——那一半**不是** Go 的活。
//
// 空文档必须由引擎产（`document::new_document`），因为"一份合法 MTW 长什么样"只能有
// 一处实现（docs/23 §6.4）。在 Go 里手写 `{format, version, meta, …}` 字面量就是第二份
// 定义，而且它会偷偷漂移——第 1 步那份手写夹具就是这么翻车的（§12.7 偏差 10）：
// 我照着上游夹具抄，结果 `meta.id_registry`、元素的 `metadata`、`element_type.source`
// 三处都不对，只有跑权威校验器才发现。
//
// ⚠ 因此标题为空时**这里不拦**：`novel_title` 为空产出的文档过不了 JSON Schema
// （`minLength: 1`）。拦它需要在 Go 里写一条"标题不能为空"的规则——那正是红线。
// 由下面的闸门用**权威校验器**去拦，Go 只看结论。这条分工在实测里被验证过
// （见 §12.3 与 TestProjectCreate_EmptyTitleIsRefusedByTheAuthority）。
// =========================================================================

// projectCreateData is the success payload of project.create.
type projectCreateData struct {
	Op      string          `json:"op"`
	Project projectView     `json:"project"`
	Version int64           `json:"version"`
	Doc     json.RawMessage `json:"doc"`
}

// projectView is the secret-free projection of a project row.
type projectView struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	NovelTitle     string `json:"novel_title"`
	CurrentVersion int64  `json:"current_version"`
	Status         string `json:"status"`
	CreatedAt      int64  `json:"created_at"`
	UpdatedAt      int64  `json:"updated_at"`
}

func newProjectView(p *WorldProject) projectView {
	return projectView{
		ID:             p.ID,
		Name:           p.Name,
		NovelTitle:     p.NovelTitle,
		CurrentVersion: p.CurrentVersion,
		Status:         p.Status,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}

// opProjectCreate implements project.create.
//
//	{ "op": "project.create", "params": { "name": "我的世界", "novel_title": "某小说" } }
//
// Capability-gated by `world-ip` (docs/23 §4.2). The op is idempotent in the only
// sense that matters: calling it twice creates two projects, which is what a user
// pressing "新建" twice should get.
func opProjectCreate(c *gin.Context, raw json.RawMessage) (any, error) {
	userID, _ := CurrentUserID(c)

	var params projectCreateParams
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}
	if err := validateProjectCreate(&params); err != nil {
		return nil, err
	}

	// ── 1. the empty world comes from the engine ──────────────────────────
	engineDoc, err := callEngine(c.Request.Context(), engineRequest{
		Op: "doc.new",
		Extra: map[string]any{
			"novel_title": params.NovelTitle,
		},
	})
	if err != nil {
		sidecarSysLog("project.create", err)
		return nil, failUpstream("%s", clientFacingSidecarMessage(err))
	}

	doc, err := engineField(engineDoc, "doc")
	if err != nil {
		sidecarSysLog("project.create", err)
		return nil, failUpstream("世界引擎没有返回空文档：请检查引擎版本与 %s 是否匹配", envEnginePath)
	}

	// ── 2. the gate: the engine's own output must pass the authority ───────
	//
	// ★ 这一步是 §10 第 10 项拍板的结果（"按 A 做"），在这里与 world.mutate 里各用一次。
	// 它把"不合法"挡在**落库之前**，而且不需要 Go 写任何一条世界规则——Go 只调用权威
	// 校验器并读取结论。
	verdict, err := gateOnAuthority(c.Request.Context(), doc)
	if err != nil {
		sidecarSysLog("project.create", err)
		return nil, failUpstream("%s", clientFacingSidecarMessage(err))
	}
	if !verdict.OK {
		return nil, failInput(
			"无法创建这个世界：参数产出的空文档未通过权威校验，%s",
			verdict.Summary())
	}

	// ── 3. the server-side record ──────────────────────────────────────────
	project, err := WorldProjectCreate(userID, params.Name, params.NovelTitle)
	if err != nil {
		return nil, err
	}
	snapshot, err := WorldSnapshotAppend(project.ID, doc, SnapshotReasonCreate)
	if err != nil {
		return nil, err
	}

	return projectCreateData{
		Op:      "project.create",
		Project: newProjectView(project),
		Version: snapshot.Version,
		Doc:     json.RawMessage(doc),
	}, nil
}

// validateProjectCreate checks the *transport* shape of the request.
//
// ⚠ It deliberately does NOT require `novel_title`: an empty title is a world-rule
// question (schema `minLength: 1`), and the gate answers it. Requiring a name is a
// transport question — a project with no name cannot be listed.
func validateProjectCreate(params *projectCreateParams) error {
	if strings.TrimSpace(params.Name) == "" {
		return failInput("缺少 name：世界项目需要一个名字，否则列表里认不出来")
	}
	return nil
}

// SnapshotReasonCreate is the reason written on the first snapshot of a project.
//
// It joins the vocabulary 工具1 already uses (`full_run` / `retype` / `merge` /
// `split` / `revert_to_N`) rather than inventing a parallel one.
const SnapshotReasonCreate = "create"

// =========================================================================
// project.list — 列这个账号的世界（docs/23 §12.4.6）
//
// ★ 这一条是**客户端要的**，不是设计文档里先有的：桌面客户端的网络全在 Rust 侧、
// 到这个世界插件只有一条 op 通道，而它需要"从我的世界里挑一个"这个动作。
//
// ⚠★ 它修掉的是一处**真实的坏行为**：在它之前，客户端没有别的办法知道
// "这个账号有没有世界"，于是只能"没有就建一个"——结果是**每次冷启动都会多出
// 一个世界**。那不是"界面小瑕疵"：用户会看见自己的世界越堆越多，而每个里面
// 都是空的。
//
// ⚠ 它**登录即可、不要能力**，与 `world.entitlements` 同一条理由：它答的是
// "我有哪些世界"，不是"我能不能解析"。一个能力过期的账号仍然该看得见自己的
// 世界列表（否则他会以为作品没了）。
// =========================================================================

// projectListData is the success payload of project.list.
type projectListData struct {
	Op    string        `json:"op"`
	Items []projectView `json:"items"`
	Total int64         `json:"total"`
}

// projectListItemLimit caps one page. A user with hundreds of worlds gets the
// newest ones; the number is deliberately the plugin's usual page ceiling so the
// client has one pagination story, not two.
const projectListItemLimit = 100

// opProjectList implements project.list.
//
//	{ "op": "project.list", "params": {} }
//
// It answers the caller's OWN worlds, newest first. There is no cross-user mode and
// no `user_id` parameter on purpose: the admin face already has
// `/dashboard/zsy/world/projects?user_id=…` for that, and a user-face op that could
// name another account would be a cross-tenant read waiting to happen.
func opProjectList(c *gin.Context, raw json.RawMessage) (any, error) {
	userID, _ := CurrentUserID(c)

	var params projectListParams
	if err := decodeParams(raw, &params); err != nil {
		return nil, err
	}
	limit := int(params.Limit)
	if limit <= 0 || limit > projectListItemLimit {
		limit = projectListItemLimit
	}

	rows, total, err := WorldProjectListByUser(userID, 0, limit)
	if err != nil {
		return nil, err
	}
	items := make([]projectView, 0, len(rows))
	for i := range rows {
		items = append(items, newProjectView(&rows[i]))
	}
	return projectListData{Op: "project.list", Items: items, Total: total}, nil
}
