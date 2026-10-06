// Package avatar is a new-api plugin that provides the 形象广场 (Avatar Plaza): an
// admin-managed catalog of personas — picture, gender, age range, ethnicity,
// suitable scenes, an introduction and the voice they speak with — plus a
// paginated public list endpoint for other applications.
//
// A persona row carries a picture (either the gateway's own
// /uploads/images/... path or an absolute URL), an introduction, the demographic
// attributes an operator sorts the plaza by, and the `voice_type` of the voice it
// belongs to. The sample audio is not copied onto the row: it is resolved from
// the 音色广场 catalog on read, so a persona always reports the sample the linked
// voice currently carries.
//
// Faces:
//
//	GET    /api/zsy/avatar/list        public, paginated (page / page_size)
//	GET    /api/zsy/avatar/:id         public, one on-shelf persona
//	GET    /dashboard/zsy/avatar/list  admin, every persona + enabled filter
//	POST   /dashboard/zsy/avatar       admin, create
//	GET    /dashboard/zsy/avatar/export admin, CSV export
//	POST   /dashboard/zsy/avatar/import admin, CSV import
//	GET    /dashboard/zsy/avatar/:id   admin, detail
//	PUT    /dashboard/zsy/avatar/:id   admin, partial update
//	DELETE /dashboard/zsy/avatar/:id   admin, delete
//
// Persona rows live in the plugin's own `zsy_avatars` table, registered for the
// host's AutoMigrate pass through extcore, and every route is mounted through the
// same registry. Installing or removing the plugin therefore touches exactly one
// core file: the blank import in zsy/extbootstrap.
package avatar

import "github.com/QuantumNous/new-api/extcore"

const (
	pluginName    = "ZSY 形象广场"
	pluginVersion = "0.1.0"
	pluginDesc    = "形象广场：后台管理形象（图片 / 性别 / 简介 / 年龄段 / 场景 / 音色 ID / 种族），音色示例取自音色广场，并提供可分页的公开形象列表接口"
)

func init() {
	extcore.RegisterPlugin(extcore.PluginInfo{
		Name:    pluginName,
		Version: pluginVersion,
		Desc:    pluginDesc,
	})

	// The Avatar table joins the host's AutoMigrate pass, so the plugin needs no
	// migration file and works on SQLite, MySQL and PostgreSQL alike.
	extcore.RegisterMigrateModels(&Avatar{})

	extcore.RegisterRouteMounter(mountRoutes)
}
