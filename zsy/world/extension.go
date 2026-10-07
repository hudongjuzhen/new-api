// Package world is a new-api plugin that hosts the 世界 IP 资源管理器: the
// **server side** of "把小说解析成一个可用世界".
//
// # What is being sold, and where it therefore lives
//
// The product is not a screen, it is a method (docs/23 §2.1). So the method —
// extraction rules, the type system, prompt templates, id allocation, the
// consistency verdict, media planning — lives in the engine (the Rust
// `world-parser` pipeline plus the authoritative `schema/1.1/validate.mjs`
// validator), and **only results travel**. This package is the orchestration
// around that engine and nothing more:
//
//	鉴权 (requireWorldAuth) · 判权 (entitlements.go) · 计费 (host billing,
//	step 4) · 落库 (store.go) · 编排 (op_dispatch.go) · 返回 (errors.go)
//
// ⚠ The line that decides every choice below: a Go function here may decide
// *who* may act and *whether the engine is reachable*. It may never decide what
// a legal world is. docs/23 §6.4: "Go 里出现第二份『什么算合法 MTW』就是错的."
//
// # Faces
//
//	POST /api/zsy/world/op                    the single op entrance (docs/23 §4.1)
//	GET  /api/zsy/world/entitlements          which capabilities this account holds (UX only)
//	GET  /dashboard/zsy/world/…               admin: projects, versions, entitlements
//
// # Install / uninstall
//
// This plugin touches exactly one core file: the blank import in
// zsy/extbootstrap. Its three tables join the host's AutoMigrate pass and its
// routes join the host's route registry, both through extcore.
package world

import "github.com/QuantumNous/new-api/extcore"

const (
	pluginName    = "ZSY 世界 IP 资源管理器"
	pluginVersion = "0.1.0"
	pluginDesc    = "世界 IP 资源管理器（服务端）：把小说解析成一个可用世界。引擎与规则只在服务端，客户端只留视图"
)

func init() {
	extcore.RegisterPlugin(extcore.PluginInfo{
		Name:    pluginName,
		Version: pluginVersion,
		Desc:    pluginDesc,
	})

	// The three tables of docs/23 §6.3 join the host's AutoMigrate pass, so the
	// plugin ships no migration file and works on SQLite, MySQL and PostgreSQL
	// alike. Order matters only for readability; the FK is expressed by the
	// store layer, not by a database constraint, so a snapshot can outlive a
	// project row that an operator removed by hand.
	extcore.RegisterMigrateModels(
		&WorldProject{},
		&WorldSnapshot{},
		&WorldEntitlement{},
	)

	extcore.RegisterRouteMounter(mountRoutes)
}
