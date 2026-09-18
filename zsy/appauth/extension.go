// Package appauth is a new-api plugin that exposes a registration / login face
// for third-party applications: an external service can create an account and
// immediately receive a usable API key, without driving the dashboard.
//
// Behaviour that distinguishes it from the host's POST /api/user/register:
//
//   - a successful registration also provisions the account's API key, named
//     默认密钥 and pinned to the 官方渠道 group (both configurable through
//     ZSY_AUTH_DEFAULT_KEY_NAME / ZSY_AUTH_DEFAULT_KEY_GROUP), with every other
//     field left at the host's key defaults;
//   - the plaintext key is returned to the caller, so no second round trip to
//     the dashboard token API is needed;
//   - failures use real HTTP status codes plus a stable machine-readable
//     `code`, instead of the dashboard's flatten-to-200 envelope;
//   - the browser Turnstile challenge is not applied (an external service
//     cannot solve it); ZSY_AUTH_APP_SECRET provides a server-to-server secret
//     in its place.
//
// Registration and login still honour the host's own switches: RegisterEnabled,
// PasswordRegisterEnabled, PasswordLoginEnabled and EmailVerificationEnabled all
// gate the matching path.
package appauth

import "github.com/QuantumNous/new-api/extcore"

const (
	pluginName    = "ZSY AppAuth"
	pluginVersion = "0.1.0"
	pluginDesc    = "第三方应用注册 / 登录开放接口：注册即创建「默认密钥」（分组 官方渠道）并返回 Key 明文"
)

func init() {
	extcore.RegisterPlugin(extcore.PluginInfo{
		Name:    pluginName,
		Version: pluginVersion,
		Desc:    pluginDesc,
	})

	// Route mounting goes through extcore, so installing or removing the plugin
	// never edits a core file.
	extcore.RegisterRouteMounter(mountRoutes)
}
