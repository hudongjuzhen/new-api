package world

import (
	"time"

	"github.com/QuantumNous/new-api/common"
)

// Environment knobs. Every one has a working default, so the plugin boots with no
// configuration at all (the same convention as zsy/appauth and zsy/voice) — but
// the engine paths below have no *useful* default on a new-api deployment, and
// that is deliberate: an operator must point them at the engine, and until they
// do, the ops that need it answer E_UPSTREAM naming the missing path.
const (
	envEnabled          = "ZSY_WORLD_ENABLED"
	envOpBodyLimitBytes = "ZSY_WORLD_OP_BODY_LIMIT"
	envNodeBinary       = "ZSY_WORLD_NODE"
	envValidatorSvc     = "ZSY_WORLD_VALIDATOR_SVC"
	envEnginePath       = "ZSY_WORLD_ENGINE"
	envSidecarTimeout   = "ZSY_WORLD_SIDECAR_TIMEOUT_SECONDS"
	envDefaultPageSize  = "ZSY_WORLD_PAGE_SIZE"
)

// ValidatorSvcPathDefault is where the validator's protocol shim (docs/23 §6.4
// A1) is expected to sit relative to the deployment.
//
// ⚠★ It is a *placeholder default*, not a working one: the shim lives in the
// upstream engine repo (`schema/1.1/world-validator-svc.mjs`), which is not part
// of a new-api checkout. A deployment therefore MUST set envValidatorSvc — and
// the failure mode when it does not is an E_UPSTREAM message that names this
// exact path, so the operator is told what to configure instead of seeing an
// opaque "node: cannot find module".
//
// This is deliberately not "resolve at boot and refuse to start": routes must
// mount so the rest of the plugin (world.get, entitlements, admin) keeps working
// on a deployment that has not deployed the engine yet.
const ValidatorSvcPathDefault = "schema/1.1/world-validator-svc.mjs"

// EnginePathDefault likewise has no working default: the engine CLI is produced
// by the upstream Rust workspace (`cargo build --release -p world-parser`).
const EnginePathDefault = "world-parser-svc"

// nodeBinaryDefault assumes node is on PATH, which is true in this gateway's own
// container image and in most deployments. It is overridable because it is not
// true everywhere.
const nodeBinaryDefault = "node"

// Config holds the resolved plugin configuration.
type Config struct {
	// Enabled mounts (true) or skips (false) the plugin routes.
	Enabled bool
	// OpBodyLimitBytes caps one op request body. A world document is sent only
	// by world.mutate / world.validate, both size-bounded by construction; the
	// read path sends a project id.
	OpBodyLimitBytes int64

	// NodeBinary is the interpreter used to run ValidatorSvcPath.
	NodeBinary string
	// ValidatorSvcPath points at the validator's protocol shim
	// (world-validator-svc.mjs). That shim holds no rule: it pipes the document
	// to schema/1.1/validate.mjs, which is the one authoritative implementation.
	ValidatorSvcPath string
	// EnginePath points at the Rust engine's service binary, consumed by
	// world.mutate / assets.plan / ingest.run (docs/23 §9 steps 3–4).
	EnginePath string

	// SidecarTimeout bounds one sidecar call. A validation of a multi-megabyte
	// document is fast; an extraction is not, which is why this is per-call and
	// configurable rather than a fixed constant.
	SidecarTimeout time.Duration

	// DefaultPageSize / MaxPageSize bound the admin list faces.
	DefaultPageSize int
	MaxPageSize     int
}

// sidecarTimeoutDefault is generous next to a validator run (measured in tens of
// milliseconds) and still bounded, so a wedged child cannot pin a request
// goroutine forever.
const sidecarTimeoutDefault = 30 * time.Second

// cfg is resolved once at import time; the environment does not change while the
// process runs. Tests overwrite it directly.
var cfg = Config{
	Enabled:          common.GetEnvOrDefaultBool(envEnabled, true),
	OpBodyLimitBytes: int64(common.GetEnvOrDefault(envOpBodyLimitBytes, 1<<20)),
	NodeBinary:       common.GetEnvOrDefaultString(envNodeBinary, nodeBinaryDefault),
	ValidatorSvcPath: common.GetEnvOrDefaultString(envValidatorSvc, ValidatorSvcPathDefault),
	EnginePath:       common.GetEnvOrDefaultString(envEnginePath, EnginePathDefault),
	SidecarTimeout:   time.Duration(common.GetEnvOrDefault(envSidecarTimeout, int(sidecarTimeoutDefault/time.Second))) * time.Second,
	DefaultPageSize:  common.GetEnvOrDefault(envDefaultPageSize, 20),
	MaxPageSize:      100,
}
