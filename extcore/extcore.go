// Package extcore provides a tiny compile-time registration centre that lets
// plugins (installed under zsy/*) hook into the new-api host process without
// requiring per-plugin edits to core files beyond a single blank import in
// zsy/extbootstrap.
//
// extcore is deliberately business-logic free. It contains only registries and
// stable hooks; plugin-specific behaviour lives in the plugin packages.
//
// IMPORTANT: extcore MUST NOT import relay/channel (or anything else that
// transitively imports the model package). The model package imports extcore
// to append extra migration targets, so any dep from extcore back into model
// (or relay/channel -> model) creates an import cycle. The adaptor registry
// therefore stores opaque factories and performs a checked type assertion on
// the read side (see relay.GetTaskAdaptor in relay/relay_adaptor.go).
package extcore

import (
	"sync"

	"github.com/gin-gonic/gin"
)

// AdaptorFactory returns the plugin's concrete task adaptor. The concrete
// type must satisfy relay/channel.TaskAdaptor; the host performs a checked
// assertion before use. Using `any` here keeps extcore free of any relay or
// model imports and therefore avoids the model <-> relay cycle.
type AdaptorFactory func() any

// ── Task adaptor registry ──────────────────────────────────────────────────
//
// platform is the decimal string of a constant.ChannelType value (matching the
// strconv convention used by relay.GetTaskAdaptor).

var (
	taskAdaptorMu      sync.Mutex
	taskAdaptorFactory = map[string]AdaptorFactory{}
)

// RegisterTaskAdaptor registers a TaskAdaptor factory for the given platform.
// Typically called from a plugin's init(). Registering the same platform twice
// overwrites the previous factory.
//
// The value returned by factory MUST implement relay/channel.TaskAdaptor. A
// host-side checked assertion validates this on GetTaskAdaptor.
func RegisterTaskAdaptor(platform string, factory AdaptorFactory) {
	taskAdaptorMu.Lock()
	defer taskAdaptorMu.Unlock()
	taskAdaptorFactory[platform] = factory
}

// GetTaskAdaptorRaw returns the raw adaptor instance registered for the
// platform, or nil if absent. Callers inside the relay package are expected
// to perform a checked type-assertion to channel.TaskAdaptor (this indirection
// avoids extcore depending on relay/channel).
func GetTaskAdaptorRaw(platform string) any {
	taskAdaptorMu.Lock()
	factory, ok := taskAdaptorFactory[platform]
	taskAdaptorMu.Unlock()
	if !ok {
		return nil
	}
	return factory()
}

// ── Database migration registry ────────────────────────────────────────────

var (
	migrateMu     sync.Mutex
	migrateModels []any
)

// RegisterMigrateModels registers GORM model pointers to be migrated together
// with the core schema. Duplicate pointers are preserved on the assumption
// plugins register at most once per pointer type.
func RegisterMigrateModels(models ...any) {
	migrateMu.Lock()
	defer migrateMu.Unlock()
	migrateModels = append(migrateModels, models...)
}

// ExtraMigrateModels returns a copy of all registered models, ready to be
// spread into DB.AutoMigrate(...).
func ExtraMigrateModels() []any {
	migrateMu.Lock()
	defer migrateMu.Unlock()
	out := make([]any, len(migrateModels))
	copy(out, migrateModels)
	return out
}

// ── Route mounter registry ─────────────────────────────────────────────────

var (
	routeMu  sync.Mutex
	mounters []func(*gin.Engine)
)

// RegisterRouteMounter registers a callback that receives the root *gin.Engine
// near the end of router.SetRouter. Plugins should attach their own gin.Router
// groups and middleware inside the callback.
func RegisterRouteMounter(m func(*gin.Engine)) {
	routeMu.Lock()
	defer routeMu.Unlock()
	mounters = append(mounters, m)
}

// MountRoutes invokes every registered mounter in registration order.
// Called once from router.SetRouter after the core routes are installed.
func MountRoutes(router *gin.Engine) {
	routeMu.Lock()
	snapshot := make([]func(*gin.Engine), len(mounters))
	copy(snapshot, mounters)
	routeMu.Unlock()
	for _, m := range snapshot {
		m(router)
	}
}

// ── Plugin metadata registry ───────────────────────────────────────────────
//
// Used by the admin UI to list installed plugins.

type PluginInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Desc    string `json:"desc"`
}

var (
	pluginMu sync.Mutex
	plugins  []PluginInfo
)

// RegisterPlugin registers metadata for an installed plugin.
func RegisterPlugin(info PluginInfo) {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	plugins = append(plugins, info)
}

// Plugins returns a copy of all registered plugin metadata.
func Plugins() []PluginInfo {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	out := make([]PluginInfo, len(plugins))
	copy(out, plugins)
	return out
}
