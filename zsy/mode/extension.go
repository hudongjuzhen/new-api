// Package mode is a new-api plugin that hosts the **模式库**: the server side of
// "每个模式的 json 默认存放在服务端，本地没有就提示开通、点开通把那份 json 下到本地"
// (docs/28).
//
// # Why modes are not plugins
//
// They were, for one round (docs/27 §3): 视频模式 / 音频模式 / 文本模式 were three
// `kind: "engineering"` plugin files. That conflated two different things —
// "这一家的工程列表屏"（插件能表达的）与"有哪几档模式（MV / 有声书 / 小说…）"
// （插件表达不了的：一份模式的正文是词表 28 格 + 指令 + 库表 + 按钮）。
// The user separated them: 插件里只留世界 IP，模式自己一摊（docs/28 §1）。
//
// # What lives here, and what does not
//
//	存量      modes/<id>.json —— 一份模式文件，**与客户端要落盘的那一份逐字同格式**
//	公开面    GET /api/zsy/mode/list        有哪些可以开通（public + 你有权限的 private）
//	取件面    GET /api/zsy/mode/:id/file    ★「开通」下的就是它
//	后台面    GET /dashboard/zsy/mode/list  全部（含 private 与坏文件）
//	授权面    /dashboard/zsy/mode/entitlements{,/grant,/revoke}  ★ 后台给权限那一下
//	一张表    zsy_mode_entitlements         "这个账号能开通哪几档私有模式"
//
// ⚠★ Go 这一层**不解析模式正文**：`words` / `planDialog` / `libraries` 那些格子的
// 权威解释在客户端（`core/workModes.js` 的 `parseWorkMode` / `normalizeWorkMode`）。
// 这里只判"别把一份明显不成形的东西发出去"（format / id 与文件名 / label / medium），
// 与 `zsy/world` 的 `plugin_template.go` 同一条分工。
//
// # Install / uninstall
//
// Like every zsy plugin: one blank import in zsy/extbootstrap, routes through
// extcore. Its one table joins the host's AutoMigrate pass.
package mode

import "github.com/QuantumNous/new-api/extcore"

const (
	pluginName    = "ZSY 模式库"
	pluginVersion = "0.2.0"
	pluginDesc    = "模式库（服务端）：每个模式一份 json，客户端「模式广场」按权限列出、点开通把那份文件下到本地"
)

func init() {
	extcore.RegisterPlugin(extcore.PluginInfo{
		Name:    pluginName,
		Version: pluginVersion,
		Desc:    pluginDesc,
	})

	/*
	 * ★★ **一张表**：`zsy_mode_entitlements`（`docs/28` §6 第 3b 段）。
	 *
	 * ⚠ 模式的**正文一个字都不进数据库**：存量仍然是磁盘上的
	 * `<工作目录>/modes/*.json`（与 `plugin-templates/` 同构），理由是
	 * "服务端那一份 = 客户端那一份"——两份内容必须逐字一致，而数据库里的大字段
	 * 既不好比对、也不能直接拷出来发给别人。
	 *
	 * 表里只有**分发策略**那一半："这个账号能开通哪几档私有模式"
	 * （用户的定性："私有不显示。当后台给权限之后才显示，并且能够一键开通"）。
	 * 于是模式的加减**不需要迁移**（放一个文件、删一个文件），
	 * 而这张表只回答"谁能看到它"。
	 */
	extcore.RegisterMigrateModels(&ModeEntitlement{})

	extcore.RegisterRouteMounter(mountRoutes)
}
