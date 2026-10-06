// Package tone is a new-api plugin that provides the 文风广场 (Tone Plaza): an
// admin-managed catalog of writing styles, a paginated public list endpoint, and
// a versioned public standard.
//
// A tone row carries the four things an operator needs to publish a writing
// style — its display name, an introduction, the writing instruction itself
// (Prompt), and a worked example pair showing the instruction applied — plus two
// controlled facets (Category, Tone), a language tag, a "适合场景" tag list,
// on-shelf state and a plaza sort order.
//
// Faces:
//
//	GET    /api/zsy/tone/list        public, paginated (page / page_size)
//	GET    /api/zsy/tone/standard    public, the versioned 文风标准
//	GET    /api/zsy/tone/:id         public, one on-shelf tone
//	GET    /dashboard/zsy/tone/list  admin, every tone + enabled filter
//	POST   /dashboard/zsy/tone       admin, create
//	GET    /dashboard/zsy/tone/:id   admin, detail
//	PUT    /dashboard/zsy/tone/:id   admin, partial update
//	DELETE /dashboard/zsy/tone/:id   admin, delete
//	GET    /dashboard/zsy/tone/export admin, CSV export
//	POST   /dashboard/zsy/tone/import admin, CSV import
//
// ⚠ There is no upload endpoint, and that is the structural difference from
// zsy/voice and zsy/avatar: a tone has no media file. Everything about a tone is
// text, so the plugin owns no upload directory and no file-serving surface.
//
// A tone row lives in the plugin's own `zsy_tones` table, registered for the
// host's AutoMigrate pass through extcore, and every route is mounted through
// the same registry. Installing or removing the plugin therefore touches exactly
// one core file: the blank import in zsy/extbootstrap.
package tone

import "github.com/QuantumNous/new-api/extcore"

const (
	pluginName    = "ZSY 文风广场"
	pluginVersion = "0.1.0"
	pluginDesc    = "文风广场：后台管理文风（名称 / 简介 / 提示词 / 类别 / 语气 / 示例对照），并提供可分页的公开文风列表接口与「文风标准」接口"
)

func init() {
	extcore.RegisterPlugin(extcore.PluginInfo{
		Name:    pluginName,
		Version: pluginVersion,
		Desc:    pluginDesc,
	})

	// The Tone table joins the host's AutoMigrate pass, so the plugin needs no
	// migration file and works on SQLite, MySQL and PostgreSQL alike.
	extcore.RegisterMigrateModels(&Tone{})

	extcore.RegisterRouteMounter(mountRoutes)
}
