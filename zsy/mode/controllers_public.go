package mode

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// 公开面（docs/28 §4）
//
//	GET /api/zsy/mode/list        有哪些可以开通
//	GET /api/zsy/mode/:id/file    ★「开通」下的就是它
//
// # ★★ 这一面**认账号但不要求登录**（`docs/28` §3b 接上来了）
//
// 用户的定性（逐字）：
//
//	"公共免费的一键开通，私有不显示。当后台给权限之后才显示，并且能够一键开通"
//
// 三句话对应三件事，而它们都在下面：
//
//	公共免费的一键开通   → `public` 那些**任何访问者**都列得到、取得到（不看账号）
//	私有不显示           → 没有权限时它**不在列表里**（不是画成灰的，是根本不来）
//	给权限之后才显示     → ★ 所以这一面要**认账号**（`resolveModeAccount`）
//
// ⚠★ 于是"没有鉴权"这件事从"刻意的"变成了"**分情况的**"：
//
//	没有 Authorization 头 → 匿名（与加私有模式之前**逐字相同**）
//	有、而且有效          → public + 这个账号持有权限的那几档
//	有、但无效            → ★ 报错（`auth.go` 里写了为什么不能悄悄降级成匿名）
//
// ⚠ 取件那一条（`getModeFile`）**也要自己判一遍**：知道 id 的人可以直接打
// 那个 URL，靠"列表里没列它"挡不住任何东西。两处走的是**同一条 WHERE**
// （`entitlements.go` 的 `entitlementLiveQuery`），所以不会一个列得出来、
// 一个取不到。
// =========================================================================

// modeView is one row of the public catalog.
//
// ⚠★ **它不含 `visibility`**（那个字段在 `adminModeView` 上）：客户端不需要、
// 也不该拿分发策略做任何判断 —— 能不能看到这一条，是**服务端**决定的事。
// 给了它只会诱使客户端自己算一遍，而那必然会与服务端分叉。
type modeView struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Medium    string `json:"medium"`
	WorkScale string `json:"workScale"`
	Summary   string `json:"summary"`
	// Bytes 是那份模式文件的字节数 —— 广场卡片上显示它，让用户知道"要下多少"
	// （最大的一档 33 KB，最小的 7 KB；两个数量级之差是真实存在的）。
	Bytes int `json:"bytes"`
}

// viewOf builds the public row of one mode.
func viewOf(row *ModeFile) modeView {
	return modeView{
		ID:        row.ID,
		Label:     row.Label,
		Medium:    row.Medium,
		WorkScale: row.WorkScale,
		Summary:   row.Summary,
		Bytes:     len(row.Raw),
	}
}

// listPublicModes (GET /api/zsy/mode/list)
//
// ★ 有问题的文件**不列**，与后台那一面**相反** —— 这个方向是刻意的，而且与
// `zsy/world` 的公共插件目录**逐字同一条**：
//
//	后台（`listAdminModes`）  坏文件**要列出来并说明**，否则运营以为后台坏了
//	本函数                    坏文件**不列**：客户端拿到它只会得到一次必然失败的
//	                            开通，而用户没有任何办法修它
//
// ⚠ 但"不列"**不是静默**：那句话仍然在后台那一屏上（`problem`），运营看得见、也修得动。
func listPublicModes(c *gin.Context) {
	userID, _ := currentUserID(c)
	entitled, err := ModeEntitlementsActive(userID)
	if err != nil {
		/*
		 * ⚠★ 读不到授权表时**不许当成"他没有权限"继续往下走**：
		 * 那会把一次数据库故障表现成"我买的那两档不见了"，
		 * 而用户唯一的反应是去问运营 —— 一个查不出原因的工单。
		 */
		common.SysError(fmt.Sprintf("zsy-mode: 读账号 %d 的模式授权失败：%v", userID, err))
		common.ApiErrorMsg(c, "这一屏要按账号列出你能开通的模式，但服务端读授权失败了，请稍后重试。")
		return
	}

	rows := ReloadModes()

	items := make([]modeView, 0, len(rows))
	skipped := make([]string, 0)
	for _, row := range rows {
		/*
		 * ★★ 判据：**公开的，或者这个账号持有它**。
		 *
		 * ⚠ `row.Visibility` 而不是 `row.IsPublic()`：后者已经把"文件有没有毛病"
		 * 并进去了，而这里要分开问两件事 —— 一份**写着 public 但坏掉**的模式
		 * 必须走进下面那个 `skipped` 分支（运营看得见），而不是被当成"私有且没权限"
		 * 静默丢掉。
		 */
		visible := row.Visibility == VisibilityPublic || entitled[row.ID]
		if !visible {
			continue
		}
		if row.Problem != "" {
			/*
			 * ⚠ 坏掉的模式要单独记一笔：它既不在目录里、也不该被当成
			 * "运营设成了私有"。两者的区别是"运营的决定"与"一个笔误" ——
			 * 而后者只会在日志里留下痕迹，所以这一行必须有。
			 */
			skipped = append(skipped, fmt.Sprintf("%s（%s）", row.ID, row.Problem))
			continue
		}
		items = append(items, viewOf(row))
	}

	if len(skipped) > 0 {
		common.SysLog("zsy-mode: 有模式没能进入目录：" + strings.Join(skipped, "；"))
	}

	common.ApiSuccess(c, gin.H{
		"items": items,
		// ★ 目录也回给界面：它与后台那一屏同一个用途（"我该把文件放哪儿"），
		// 而这一面是**排查那条路**上唯一能回答它的地方。
		"directory":    ModeDir(),
		"knownMediums": NonNilStrings(knownMediums),
	})
}

// getModeFile (GET /api/zsy/mode/:id/file)
//
// ★★ **这就是"开通"那一下。** 它回的是**那份模式文件本身**（加一个建议的文件名），
// 而不是一个链接：客户端拿到就写进 `<数据目录>/modes/<id>.json`，落盘走的是
// **既有的那条模式写盘路径**（`work_modes.rs` 的 `write`，带 `.bak`、带 id 校验）。
//
// ⚠ 于是"服务端发一份、客户端存一份"这条路上**只有一个东西在变**：文件从哪儿来。
// 格式、校验、落点、备份——全是现成的。
func getModeFile(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	row, ok := ModeByID(id)
	if !ok {
		common.ApiErrorMsg(c, fmt.Sprintf("没有 %q 这一档模式。可以用 GET /api/zsy/mode/list 看有哪些。", id))
		return
	}
	if row.Problem != "" {
		/*
		 * ★ 把文件自己那句话原样透出去：它指名了哪个文件、哪一格写错了。
		 * 换一句"这一份不可用"会让运营去猜。
		 */
		common.ApiErrorMsg(c, fmt.Sprintf("模式 %s 有问题，先修好它再开通：%s", id, row.Problem))
		return
	}

	/*
	 * ★★ 私有模式**没权限时连文件都取不到**。
	 *
	 * ⚠★ 这一道**不能省**：`listPublicModes` 里"没列它"挡不住知道 id 的人
	 * 直接打这个 URL（`GET /api/zsy/mode/audiobook/file` 就是一个手打的请求）。
	 * 两处判的是同一件事、走的是同一条 WHERE（`ModeEntitlementActive`），
	 * 所以"列表里有它、取件却 403"这种自相矛盾不会出现。
	 *
	 * ⚠ 那句话必须**能照做**：它说清了"这是私有模式"（用户才知道该找谁）
	 * 与"去哪儿开"（后台）。只回一句"没权限"会变成一张没人能处理的工单。
	 */
	if !row.IsPublic() {
		userID, _ := currentUserID(c)
		allowed, err := ModeEntitlementActive(userID, row.ID)
		if err != nil {
			common.SysError(fmt.Sprintf("zsy-mode: 判账号 %d 能不能开通 %s 失败：%v", userID, row.ID, err))
			common.ApiErrorMsg(c, "服务端判权限失败了，请稍后重试。")
			return
		}
		if !allowed {
			common.ApiErrorMsg(c, fmt.Sprintf(
				"「%s」是私有模式，你这个账号还没有开通它的权限 —— 请让运营在后台给你授权，"+
					"授权之后回到「模式广场」刷新一下就能一键开通了。", row.Label))
			return
		}
	}

	file, err := RenderModeFile(row)
	if err != nil {
		common.SysError(fmt.Sprintf("zsy-mode: 渲染模式 %s 失败：%v", id, err))
		common.ApiErrorMsg(c, "这一份模式暂时取不出来：服务端渲染失败，请稍后重试。")
		return
	}

	common.ApiSuccess(c, gin.H{
		"id": row.ID,
		// ⚠ 名字里带 id 与 medium：用户的 `<数据目录>/modes/` 里会同时躺着十几份
		// 看着一样的 json，而"哪一份是哪个"只能靠文件名（内容是给人看的，不是给眼睛扫的）。
		"fileName": row.ID + ".json",
		/*
		 * `file` 是**文本**而不是嵌套对象 —— 与 `zsy/world` 签发插件那一条
		 * **逐字同一条理由**：用户拿到的就是一个文件，而"再序列化一次"会让它变成
		 * 另一种排版（缩进、键序），与"下载下来就是它"这件事脱钩。
		 * 客户端要写盘的正是这串文本。
		 */
		"file":   file,
		"bytes":  len(file),
		"medium": row.Medium,
		"label":  row.Label,
	})
}

// NonNilStrings keeps a JSON array from coming back as null.
//
// ⚠ 它是**导出**的：后台那一面也要用（两处各写一份 `if out == nil { out = []string{} }`
// 迟早只在其中一处漏掉，而漏掉的表现是"界面上那一格是 null"）。
func NonNilStrings(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}
