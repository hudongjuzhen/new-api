package world

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// ★★ GET /api/zsy/plugins — 「公共插件目录」（docs/27 §3）
//
// 用户 2026-… 的要求原话：
//
//	"一类是公共类型，比如 视频模式，文本模式，能够直接在插件这里看到，
//	 能够直接点击一键安装，任何账号都能直接一键安装"
//
// # 它与「按账号签发」是同一个模板目录的两个出口
//
//	                    plugin-templates/<id>.json
//	                    ↙                        ↘
//	  GET /api/zsy/plugins               POST /dashboard/…/plugins/issue
//	  （x-visibility = public）           （x-visibility = private）
//	  原样发出去，谁都能装               填上 entitlement 块，按账号发出去
//
// ⚠★ **两个出口读的必须是同一份源**，理由与 "为什么模板是源文件而不是生成物"
// 是同一条（见 `plugin_template.go` 文件头）：运营改一处，两条路一起变。
// 若公共目录另存一份"公共版"，那份副本迟早与模板分叉，而分叉的表现是
// "一键装上的是旧那一版、后台签发的是新那一版" —— 没人会想到去比对。
//
// # ⚠ 它是**公开的**（没有鉴权中间件）
//
// 与 `zsy/voice/list` 同一条：这一面的内容按定义就是要发给所有人的
// （`x-visibility: public`）。加一道鉴权买不到任何东西 —— 它拦不住任何人
// 拿到一份本来就该公开的 JSON —— 却会让"还没登录的人也能看见插件页里有什么"
// 变成一件要额外解释的事。
//
// ⚠ **私有模板一个字节都不在这里出现**（包括它的名字）。这不是"藏起来"，
// 是"这一面根本不回答那个问题"：私有插件的入口是运营手上的那份签发文件。
//
// # ⚠★ 它与 `E_ENTITLEMENT` 的关系：**装得上 ≠ 有权用**
//
// 公共插件里完全可以有一个带能力的（比如将来某个"公开试用版"的世界插件）。
// 装上它只会多出那一屏；那一屏点进去照样会被每个 op 现算的判据挡住
// （docs/23 §8.3 ①）。所以这个目录**不是**授权面，也不该被当成授权面
// —— 与 `plugin_issue.go` 开头那句"签发 ≠ 授权"是同一条。
// =========================================================================

// publicPluginView is one item of the public catalog.
//
// ⚠★ 只有两格，**是刻意的**。名字 / 版本 / 描述 / 有几屏全在 `manifest` 里
// —— 而客户端本来就要拿 `validatePlugin` 把那串文本解析一遍才敢装
// （那是插件格式唯一的权威校验器）。在这里再抄一份 `name` / `version`，
// 就多了一处可能说谎的地方：两处不一致时，列表上写着一版、装上去是另一版，
// 而**谁都不会想到去比对**（与 `plugin_template.go` 里 `id` 与文件名必须
// 一致那条是同一个形状的教训）。
type publicPluginView struct {
	ID string `json:"id"`
	// Manifest is the plugin file **itself**, as text.
	//
	// ⚠ 给的是**文本**而不是嵌套对象，与签发那一条逐字同一条理由：
	// 用户拿到的就是一个文件，而"再序列化一次"会让它变成另一种排版
	// （缩进、键序），与"下载下来就是它"这件事脱钩。
	Manifest string `json:"manifest"`
}

// listPublicPlugins (GET /api/zsy/plugins)
//
// ★ 有问题的模板**不列**，与后台那一屏**相反** —— 这个方向也是刻意的：
//
//	后台（`listPluginTemplates`）  坏模板**要列出来并说明**，否则运营以为后台坏了
//	公共目录（本函数）            坏模板**不列**：客户端拿到它只会得到一次
//	                              必然失败的安装，而用户没有任何办法修它
//
// ⚠ 但"不列"**不是静默**：那句话仍然在后台那一屏上（`problem`），
// 运营看得见、也修得动。这一面只是不给用户一个点不动的按钮 ——
// 与 `voicePlaza` 那句"一颗点不动的灰按钮会让人一直试"同一条。
func listPublicPlugins(c *gin.Context) {
	rows := ReloadPluginTemplates()

	items := make([]publicPluginView, 0, len(rows))
	skipped := make([]string, 0)
	for _, row := range rows {
		if !row.IsPublic() {
			/*
			 * ⚠ 坏掉的**公共**模板要单独记一笔：它既不在目录里、也不该被当成
			 * "运营设成了私有"。两者的区别是"运营的决定"与"一个笔误" ——
			 * 而后者只会在日志里留下痕迹，所以这一行必须有。
			 */
			if row.Visibility == VisibilityPublic && row.Problem != "" {
				skipped = append(skipped, fmt.Sprintf("%s（%s）", row.ID, row.Problem))
			}
			continue
		}
		manifest, err := renderPublicPluginFile(row.Manifest)
		if err != nil {
			common.SysError(fmt.Sprintf("zsy-world: 公共插件 %s 序列化失败：%v", row.ID, err))
			continue
		}
		items = append(items, publicPluginView{ID: row.ID, Manifest: manifest})
	}

	if len(skipped) > 0 {
		common.SysLog("zsy-world: 有公共模板没能进入公共目录：" + strings.Join(skipped, "；"))
	}

	common.ApiSuccess(c, gin.H{
		"items": items,
		// ★ 目录也回给界面：它与后台那一屏同一个用途（"我该把文件放哪儿"），
		// 而这一面是**排查那条路**上唯一能回答它的地方（一个 public 模板
		// 没出现在目录里时，运营要能看出它读的是哪个目录）。
		"directory": PluginTemplateDir(),
	})
}

// renderPublicPluginFile writes a template out as a plugin file.
//
// # 它与签发那一份的两处不同，都要说得清
//
//	① **不加 `entitlement` 块** —— 那一块说的是"这份文件是给谁的"，
//	   而公共插件没有"给谁的"这回事（谁都能装）。
//	② **拿掉宿主专用的扩展格**（`hostOnlyKeys`，与签发那一条**同一份清单**）
//	   —— 那几格是服务端的分发策略，不是插件格式的一部分。
//
// ⚠ 其余字段**一个都不动**（含模板作者写的任何不认识的东西）：客户端承诺
// "不认识的字段原样保留"，服务端这一侧也必须守同一条 —— 在这里丢掉一格，
// 表现是"我写的模板明明有那一格，装上去就没了"，而且不报错。
func renderPublicPluginFile(manifest map[string]any) (string, error) {
	out := make(map[string]any, len(manifest))
	for k, v := range manifest {
		out[k] = v
	}
	for _, k := range hostOnlyKeys {
		delete(out, k)
	}
	raw, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", fmt.Errorf("world: encode public plugin: %w", err)
	}
	return string(raw) + "\n", nil
}
