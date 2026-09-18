package appauth

import "github.com/QuantumNous/new-api/common"

// Environment knobs. Every one of them has a working default, so the plugin is
// usable with no configuration at all.
const (
	envEnabled         = "ZSY_AUTH_ENABLED"
	envDefaultKeyName  = "ZSY_AUTH_DEFAULT_KEY_NAME"
	envDefaultKeyGroup = "ZSY_AUTH_DEFAULT_KEY_GROUP"
	envAppSecret       = "ZSY_AUTH_APP_SECRET"

	// DefaultKeyName is the name of the API key created for a third-party
	// account ("默认密钥").
	DefaultKeyName = "默认密钥"
	// DefaultKeyGroup pins the created key to the official upstream group
	// ("官方渠道") instead of the user's own group.
	DefaultKeyGroup = "官方渠道"

	// appSecretHeader carries the pre-shared secret when ZSY_AUTH_APP_SECRET is
	// configured.
	appSecretHeader = "X-Zsy-App-Secret"
)

// Config holds the resolved plugin configuration.
type Config struct {
	// Enabled mounts (true) or skips (false) the plugin routes.
	Enabled bool
	// DefaultKeyName / DefaultKeyGroup describe the API key every third-party
	// account receives.
	DefaultKeyName  string
	DefaultKeyGroup string
	// AppSecret, when non-empty, must be presented in the appSecretHeader of
	// every plugin request.
	AppSecret string
}

// cfg is resolved once at import time; the environment does not change while
// the process runs. Tests overwrite it directly.
var cfg = Config{
	Enabled:         common.GetEnvOrDefaultBool(envEnabled, true),
	DefaultKeyName:  common.GetEnvOrDefaultString(envDefaultKeyName, DefaultKeyName),
	DefaultKeyGroup: common.GetEnvOrDefaultString(envDefaultKeyGroup, DefaultKeyGroup),
	AppSecret:       common.GetEnvOrDefaultString(envAppSecret, ""),
}
