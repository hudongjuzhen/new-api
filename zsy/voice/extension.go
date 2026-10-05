// Package voice is a new-api plugin that provides the 音色广场 (Voice Plaza): an
// admin-managed catalog of TTS voices plus a paginated public list endpoint.
//
// A voice row carries the four things an operator needs to publish a voice —
// its display name, an introduction, the upstream `voice_type` value a TTS
// request actually sends, and a sample audio file uploaded through the plugin —
// plus on-shelf state and a plaza sort order.
//
// Faces:
//
//	GET    /api/zsy/voice/list        public, paginated (page / page_size)
//	GET    /api/zsy/voice/:id         public, one on-shelf voice
//	GET    /dashboard/zsy/voice/list  admin, every voice + enabled filter
//	POST   /dashboard/zsy/voice       admin, create
//	POST   /dashboard/zsy/voice/upload admin, sample-audio upload
//	GET    /dashboard/zsy/voice/:id   admin, detail
//	PUT    /dashboard/zsy/voice/:id   admin, partial update
//	DELETE /dashboard/zsy/voice/:id   admin, delete
//
// A voice row lives in the plugin's own `zsy_voices` table, registered for the
// host's AutoMigrate pass through extcore, and every route is mounted through
// the same registry. Installing or removing the plugin therefore touches exactly
// one core file: the blank import in zsy/extbootstrap.
package voice

import "github.com/QuantumNous/new-api/extcore"

const (
	pluginName    = "ZSY 音色广场"
	pluginVersion = "0.1.0"
	pluginDesc    = "音色广场：后台管理音色（名称 / 简介 / voice_type / 示例音频），并提供可分页的公开音色列表接口"
)

func init() {
	extcore.RegisterPlugin(extcore.PluginInfo{
		Name:    pluginName,
		Version: pluginVersion,
		Desc:    pluginDesc,
	})

	// The Voice table joins the host's AutoMigrate pass, so the plugin needs no
	// migration file and works on SQLite, MySQL and PostgreSQL alike.
	extcore.RegisterMigrateModels(&Voice{})

	extcore.RegisterRouteMounter(mountRoutes)
}
