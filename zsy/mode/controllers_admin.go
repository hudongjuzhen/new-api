package mode

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// 后台面（`docs/28` §4）
//
//	GET  /dashboard/zsy/mode/list                 全部模式（含 private 与坏文件）
//	GET  /dashboard/zsy/mode/entitlements         ★ 谁被授权了哪几档
//	POST /dashboard/zsy/mode/entitlements/grant   ★ 后台给权限
//	POST /dashboard/zsy/mode/entitlements/revoke  ★ 收回来
//
// # 它与公开面**处置相反**，而这是刻意的
//
// | 面 | 坏文件 | 为什么 |
// |---|---|---|
// | 公开面（`listPublicModes`） | **不列** | 客户端拿到它只会得到一次必然失败的开通，而用户没有任何办法修它 |
// | 这一面 | ★ **列出来 + 带上 `problem`** | 否则运营看到的是"我放进去的文件不见了"，以为后台坏了 |
//
// "不列"不是静默：那句话仍然在这里。
//
// ⚠★ 这一屏也是运营**唯一**看得出"我给谁看过这一档"的地方 —— 所以
// `visibility` 必须回报（`public` / `private`），而 `granted`（现在有几个账号
// 持有它）随之一起回：一个 `private` 而**零授权**的模式是运营多半会想知道的事
// （他可能忘了给谁开），把那个 0 摆在他眼前比让他自己去翻授权表强。
// =========================================================================

// adminModeView is one row of the admin list.
//
// ⚠ 它比 `modeView` 多四格，而四格各有各的用处：
//
//	visibility  这一档是"任何账号都能开通"还是"后台给了权限才显示"（运营唯一看得见它的地方）
//	granted     ★ **现在有几个账号持有它**（private 而 0 是一个值得看见的数）
//	problem     坏文件**为什么**没生效（绝不静默）
//	source      它来自哪个文件 —— 运营要改的就是它
type adminModeView struct {
	modeView
	Visibility string `json:"visibility"`
	Granted    int    `json:"granted"`
	Problem    string `json:"problem"`
	Source     string `json:"source"`
}

// listAdminModes (GET /dashboard/zsy/mode/list)
func listAdminModes(c *gin.Context) {
	rows := ReloadModes()

	/*
	 * ★ 一次查询拿到"每一档各有几个人"，而不是每一行查一次
	 * （`ModeEntitlementLiveCounts` 说的就是这件事）。
	 *
	 * ⚠★ 读不到时**整屏报错**，不回落成"全是 0"：那个 0 在这一屏上是
	 * **一个结论**（"这一档没人有权限"），把它当成一次数据库故障的表现
	 * 会让运营照着它去重复授权。
	 */
	counts, err := ModeEntitlementLiveCounts()
	if err != nil {
		common.ApiErrorMsg(c, "读模式授权失败："+err.Error())
		return
	}

	items := make([]adminModeView, 0, len(rows))
	for _, row := range rows {
		items = append(items, adminModeView{
			modeView:   viewOf(row),
			Visibility: row.Visibility,
			Granted:    counts[row.ID],
			Problem:    row.Problem,
			Source:     row.Source,
		})
	}

	common.ApiSuccess(c, gin.H{
		"items": items,
		// 目录与公开面回的是**同一份**（`ModeDir()` 只有一处定义）——
		// 两处各读一次环境变量的话，"后台显示的目录"与"实际读的目录"迟早分叉。
		"directory": ModeDir(),
		// 写模式时能用的取值（客户端认可的那几个白名单）。
		"knownMediums": NonNilStrings(knownMediums),
		/*
		 * ⚠ 这一屏也顺带说清"存量在哪儿"：模式库**不是数据库**，是磁盘上一个目录
		 * （`docs/28` §4 那条"服务端那一份 = 客户端那一份"的取舍）。
		 * 运营最可能问的下一个问题是"我怎么加一档新的" —— 答案是"把一份模式文件
		 * 放进上面那个目录"，而那句话必须由界面说出来。
		 */
		"note": "模式库是一个目录：一份模式 = 一个 .json（文件名就是模式 id）。" +
			"放进去之后点「重新读取」即可生效；`x-visibility` 决定它是公开可开通还是后台给权限才显示。",
	})
}

/* ==========================================================================
 * ★★ 授权（`docs/28` §3b）—— "后台给权限"那一下
 * ======================================================================== */

// adminGrantParams is the body of POST …/entitlements/grant.
//
// ⚠★ **请求体用下划线、响应体用驼峰** —— 这看着不一致，但它就是宿主的规矩，
// 两半各有各的来源：
//
//	请求体  宿主 dashboard 那一整套都是 `user_id` / `expires_at`（`zsy/world`
//	        的 `adminGrantParams` 逐字同形），而这个面是宿主 dashboard 的一部分
//	响应体  回的是本插件的模型（`json:"userId"` 那一套，与公开面的 `workScale`
//	        同一种），界面直接照着字段名读
//
// 两半各按各的来是**两次都对的抄法**；把请求体也改成驼峰，就是在本插件里
// 发明第二套宿主请求约定。
type adminGrantParams struct {
	UserID int `json:"user_id"`
	// ModeID is the mode id (`audiobook`), NOT a capability name — see models.go
	// for why modes get their own table instead of reusing zsy_world_entitlements.
	ModeID string `json:"mode_id"`
	Source string `json:"source"`
	// ExpiresAt is a unix second; 0 or absent means "never expires".
	// A negative value is refused rather than silently read as "never".
	ExpiresAt int64 `json:"expires_at"`
}

// adminRevokeParams is the body of POST …/entitlements/revoke.
type adminRevokeParams struct {
	UserID int    `json:"user_id"`
	ModeID string `json:"mode_id"`
}

// listAdminEntitlements (GET /dashboard/zsy/mode/entitlements?user_id=… 或 ?mode_id=…)
//
// ★★ 两种问法，因为它服务两个不同的界面动作：
//
//	?user_id=N   "这个账号手上有什么" → 后台的账号页（含过期与已撤销的历史）
//	?mode_id=x   "这一档给了谁"       → ★ 模式管理页那一行点开的那一栏（**只列生效中的**）
//
// ⚠★ 两者**不许合成一个"全表列举"**：授权表会随着账号数线性长大，而
// "把所有授权都拉出来"既没有界面用得上、也会在某一天变成一次慢查询。
// 与 `zsy/world` 的 `listAdminEntitlements` 拒绝全表是同一条纪律。
func listAdminEntitlements(c *gin.Context) {
	userID, err := parseOptionalIntQuery(c, "user_id")
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	modeID := strings.TrimSpace(c.Query("mode_id"))

	if userID > 0 && modeID != "" {
		common.ApiErrorMsg(c, "user_id 与 mode_id 只能给一个：前者问「这个账号手上有什么」，"+
			"后者问「这一档给了谁」，两个问题的答案形状不一样。")
		return
	}

	switch {
	case userID > 0:
		rows, err := ModeEntitlementListByUser(userID)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		/*
		 * ⚠ 生效中的那一份**现算**（不是从上面那几行里推出来的）：
		 * 于是后台显示的就恰好是"此刻用户会看到什么" —— 与
		 * `WorldCapabilitiesActive` 那一条同源。
		 */
		live, err := ModeEntitlementsActive(userID)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{
			"userId": userID,
			"items":  rows,
			"active": nonNilModeIDs(live),
		})
	case modeID != "":
		rows, err := ModeEntitlementLiveByMode(modeID)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		common.ApiSuccess(c, gin.H{
			"modeId": modeID,
			"items":  rows,
		})
	default:
		common.ApiErrorMsg(c, "请给一个 user_id 或 mode_id：授权是挂在"+
			"「账号 × 模式」这一对上，不接受全表列举。")
	}
}

// grantModeEntitlement (POST /dashboard/zsy/mode/entitlements/grant)
//
// ★ 它**下一次请求就生效**：没有任何地方缓存这个判断
// （`entitlements.go` 文件头那条"每次现算"）。
func grantModeEntitlement(c *gin.Context) {
	var in adminGrantParams
	if err := c.ShouldBindJSON(&in); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	var expiresAt *int64
	if in.ExpiresAt > 0 {
		value := in.ExpiresAt
		expiresAt = &value
	} else if in.ExpiresAt < 0 {
		common.ApiErrorMsg(c, "expires_at 必须是正的 unix 秒；不填表示永不过期。")
		return
	}

	row, err := ModeEntitlementGrant(ModeEntitlementGrantInput{
		UserID:    in.UserID,
		ModeID:    strings.TrimSpace(in.ModeID),
		Source:    strings.TrimSpace(in.Source),
		ExpiresAt: expiresAt,
	})
	if err != nil {
		/*
		 * ⚠ `ModeEntitlementGrant` 的那几句话已经是**能照做**的
		 * （比如"模式库里没有这一档，先把 x.json 放进去"），原样透出去。
		 */
		common.ApiErrorMsg(c, err.Error())
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-mode] granted mode=%q to user=%d source=%q expiresAt=%v by admin",
		row.ModeID, row.UserID, row.Source, row.ExpiresAt))
	common.ApiSuccess(c, row)
}

// revokeModeEntitlement (POST /dashboard/zsy/mode/entitlements/revoke)
//
// "卖了要能收回"（docs/23 §8.4）。⚠ 撤销之后**用户下一次刷新广场**就看不到
// 那一档了，而他已经下到本机的那一份**不会被删掉** —— 那是**刻意**的：
// 本机那份模式文件是他的资产（他可能已经在上面改过东西、已经用它建了工程），
// 而"收回权限"这件事能表达的是"以后不再发新的"，不是"把你机器上的删掉"。
// 那句话写在界面上了（后台那一屏的说明里），因为运营一定会问。
func revokeModeEntitlement(c *gin.Context) {
	var in adminRevokeParams
	if err := c.ShouldBindJSON(&in); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	modeID := strings.TrimSpace(in.ModeID)
	if in.UserID <= 0 || modeID == "" {
		common.ApiErrorMsg(c, "需要 user_id 与 mode_id。")
		return
	}

	affected, err := ModeEntitlementRevoke(in.UserID, modeID)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if affected == 0 {
		common.ApiErrorMsg(c, fmt.Sprintf(
			"账号 %d 当前没有生效中的「%s」授权：没有可撤销的行。", in.UserID, modeID))
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-mode] revoked mode=%q from user=%d rows=%d by admin",
		modeID, in.UserID, affected))
	common.ApiSuccess(c, gin.H{"userId": in.UserID, "modeId": modeID, "revoked": affected})
}

// nonNilModeIDs turns the active-set map into a sorted array.
//
// ⚠ 它回**数组**而不是那个 map：Go 的 map 序列化成 JSON 时键序是随机的，
// 而这一格会被界面上"他现在有哪几档"那一行直接画出来 —— 每次刷新换个次序
// 会让人以为中间发生了什么。
func nonNilModeIDs(active map[string]bool) []string {
	out := make([]string, 0, len(active))
	for id, ok := range active {
		if ok && id != "" {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

// adminMetaParams is the body of POST …/:id/meta.
//
// ⚠ 请求体是**下划线**（宿主 dashboard 那一套），与 `adminGrantParams` 同一条。
//
// ⚠★ `summary` 用的是 `*string`：**不传**（nil）与"传一个空串"是两件事 ——
// 前者是"这次不动这一格"，后者是"把这一格清掉"。用一个 `string` 表示不了这个区别，
// 而那会让"只想改可见性"的一次保存**顺手把说明清空**（用户会以为是自己删的）。
type adminMetaParams struct {
	Visibility string  `json:"visibility"`
	Summary    *string `json:"summary"`
}

// updateModeMeta (POST /dashboard/zsy/mode/:id/meta)
//
// ★★ "后台可以编辑一档模式"（用户 2026-…）：目前开放的是**分发策略**那两格 ——
// `x-visibility`（公开 / 私有）与 `x-summary`（广场卡片上那句说明）。
//
// # ⚠★ 它改的是**磁盘上那份文件**，不是数据库
//
// 模式的存量是文件（`catalog.go` 文件头那段取舍），所以"公开 / 私有"这一格也写在
// 文件里。改完**下一次请求立刻生效**：公开面每一次都重新读盘、也重新判权限
// （`controllers_public.go`），没有任何缓存需要清。
//
// ⚠ 正文（`words` / `planDialog` / 指令…）**一个字节都不从后台改**：它是几千行
// 结构化数据，给运营一个 textarea 去改等于让他手写 JSON。要改正文就改文件
// —— 这一面**不假装**能改（`ModeMetaInput` 那条注释说的是同一件事）。
func updateModeMeta(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	var in adminMetaParams
	if err := c.ShouldBindJSON(&in); err != nil {
		common.ApiErrorMsg(c, "请求体错误: "+err.Error())
		return
	}
	if strings.TrimSpace(in.Visibility) == "" && in.Summary == nil {
		common.ApiErrorMsg(c, "这次没有任何要改的东西：请给 visibility（public / private）或 summary。")
		return
	}

	_, err := WriteModeMeta(id, ModeMetaInput{
		Visibility: strings.TrimSpace(in.Visibility),
		Summary:    in.Summary,
	})
	if err != nil {
		/* ⚠ 那几句话已经是**能照做**的（哪一档没有 / 可见性只能写什么），原样透出去 */
		common.ApiErrorMsg(c, err.Error())
		return
	}

	/*
	 * ★ 改完**重读一遍**回给界面，而不是把请求体回显出去：
	 * 文件里那份才是权威（它可能与运营填的不一样 —— 例如 trim 掉了空白），
	 * 而界面拿它去更新那一行，于是"看到的"与"磁盘上的"永远是同一个。
	 */
	row, found := ModeByID(id)
	if !found {
		common.ApiErrorMsg(c, fmt.Sprintf(
			"改完之后反而读不到 %q 了 —— 文件可能被别的进程动过，请点「重新读取」看一眼。", id))
		return
	}

	common.SysLog(fmt.Sprintf(
		"[zsy-mode] updated meta of mode=%q visibility=%q by admin", id, row.Visibility))
	common.ApiSuccess(c, adminModeView{
		modeView:   viewOf(row),
		Visibility: row.Visibility,
		Granted:    liveGrantCountOf(row.ID),
		Problem:    row.Problem,
		Source:     row.Source,
	})
}

// liveGrantCountOf reads one mode's live-grant count, tolerating a read failure.
//
// ⚠ 只在**改完之后的回读**里用（列表那条路走的是批量版 `ModeEntitlementLiveCounts`）：
// 那一格是给界面显示"这一档给了几个人"的，读不到时报 0 会**说错话**，
// 所以它把错误压成 -1，界面据此显示"人数读不到"而不是"0 个人"。
func liveGrantCountOf(modeID string) int {
	counts, err := ModeEntitlementLiveCounts()
	if err != nil {
		return -1
	}
	return counts[modeID]
}

// parseOptionalIntQuery reads an optional positive integer query parameter.
//
// ⚠ 与 `zsy/world` 的同名函数逐字同形：空值 = 0（"这次没问这个"），
// 而**写错**（`?user_id=abc`）必须报错而不是当成 0 —— 后者会让
// "我明明给了 user_id"变成一次结果为空的全表查询。
func parseOptionalIntQuery(c *gin.Context, name string) (int, error) {
	raw := strings.TrimSpace(c.Query(name))
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s 必须是整数（收到的是 %q）", name, raw)
	}
	if value < 0 {
		return 0, fmt.Errorf("%s 不能是负数（收到的是 %d）", name, value)
	}
	return value, nil
}
