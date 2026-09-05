// Package runninghub is a new-api plugin that adds RunningHub (https://runninghub.ai
// / https://runninghub.cn) as a task upstream channel and application management.
//
// This file wires the plugin into the host process via extcore registries at init()
// time. Host-level anchors were installed in P1-P5 during the extcore phase, so no
// core files need edits when enabling / disabling this plugin.
package runninghub

import (
	"strconv"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/extcore"
)

// pluginName doubles as the i18n key prefix for UI strings shipped by the plugin.
const (
	pluginName    = "RunningHub"
	pluginVersion = "0.1.0"
	pluginDesc    = "RunningHub AI App / Workflow / Standard Model API 上游集成 + 应用 CRUD + 站点路由"
)

// pluginChannelType is the host-level channel id assigned in constant/channel.go.
// Keep in sync with constant.ChannelTypeRunningHub.
const pluginChannelType = constant.ChannelTypeRunningHub

// pluginChannelTypes is the set of channel types this plugin serves. All three
// (RunningHub 国内站 / 国际站 / LiblibAI) share the same RH V2 protocol, so the
// platform requests are issued under the host TaskPlatform derived from
// pluginChannelType; the upstream base URL differs per channel.
var pluginChannelTypes = map[int]bool{
	constant.ChannelTypeRunningHub:     true,
	constant.ChannelTypeRunningHubIntl: true,
	constant.ChannelTypeLiblib:         true,
}

func init() {
	extcore.RegisterPlugin(extcore.PluginInfo{
		Name:    pluginName,
		Version: pluginVersion,
		Desc:    pluginDesc,
	})

	// GORM models for the plugin-owned tables. Always registered here (before
	// main.go calls migrateDB) because extbootstrap is imported (and therefore
	// init() runs) in main() before model.InitDB.
	extcore.RegisterMigrateModels(&App{})

	// Task adaptor factory. The returned value must satisfy
	// relay/channel.TaskAdaptor; the host performs a checked type-assertion
	// before use. Passing a typed TaskAdaptor here keeps the implicit
	// satisfaction compile-time checked.
	//
	// The platform string is the decimal form of the *channel type* the request
	// routed through (relay.GetTaskPlatform strconv's channel_type). The
	// RunningHub family spans three channel types (国内站 / 国际站 / LiblibAI),
	// so every one of them must resolve to the same adaptor — otherwise a run
	// through an Intl (62) or Liblib (63) channel fails with
	// "invalid_api_platform" even though the base RunningHub (61) works.
	for _, channelType := range []int{
		constant.ChannelTypeRunningHub,
		constant.ChannelTypeRunningHubIntl,
		constant.ChannelTypeLiblib,
	} {
		platform := strconv.Itoa(channelType)
		extcore.RegisterTaskAdaptor(platform, func() any { return &TaskAdaptor{} })
	}

	// HTTP routes (public / management / user). RegisterRouteMounters calls
	// into mountRoutes which conditionally skips auth-gated paths when the
	// auth controller is not yet loaded.
	extcore.RegisterRouteMounter(mountRoutes)
}
