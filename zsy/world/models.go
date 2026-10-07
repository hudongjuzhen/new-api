package world

import "github.com/QuantumNous/new-api/common"

// The three tables of docs/23 §6.3. They join the host's AutoMigrate pass
// through extcore.RegisterMigrateModels, so the plugin ships no migration file
// and works on SQLite, MySQL and PostgreSQL alike.
//
// ⚠ What is deliberately NOT in these tables: element types, ID allocation
// rules, prompt templates, consistency verdicts. Those are the engine's
// (`Rust 引擎 + schema/1.1/validate.mjs`), and docs/23 §6.4 keeps exactly one
// implementation of every rule. Go stores *results* — a document, a version, a
// capability — never a rule.

// WorldProject is one world project: the server-side anchor a client's world
// belongs to.
//
// `NovelTitle` is the display title the user typed; it is not derived from the
// document (the document stays authoritative for anything MTW-shaped).
type WorldProject struct {
	ID        uint  `gorm:"primarykey"           json:"id"`
	CreatedAt int64 `gorm:"autoCreateTime"       json:"createdAt"`
	UpdatedAt int64 `gorm:"autoUpdateTime"       json:"updatedAt"`

	UserID     int    `gorm:"not null;index"             json:"userId"`
	Name       string `gorm:"type:varchar(191);not null" json:"name"`
	NovelTitle string `gorm:"type:varchar(191)"          json:"novelTitle"`

	// CurrentVersion is the version of the newest snapshot. It is the value
	// clients echo back as `base_version`, so it must never be bumped without a
	// snapshot row landing at the same number (see WorldSnapshotAppend).
	CurrentVersion int64  `gorm:"not null;default:0"      json:"currentVersion"`
	Status         string `gorm:"type:varchar(16);index"  json:"status"`
}

// TableName pins the table to a plugin-prefixed name. The host shares one
// database between core tables and every installed plugin, and a bare
// `projects` is too generic to claim there.
func (WorldProject) TableName() string { return "zsy_world_projects" }

// WorldSnapshot is one version of one document — the support for "roll back"
// (docs/23 §6.3): every version is kept, and a revert writes a NEW version
// rather than deleting history, exactly like 工具1's `document_history` /
// `revert_document`.
//
// Doc is the serialized document as the engine produced it. It is stored as
// opaque text and handed back verbatim: Go never parses it for meaning
// (docs/23 §6.4 — "Go 里出现第二份『什么算合法 MTW』就是错的").
type WorldSnapshot struct {
	ID        uint   `gorm:"primarykey"             json:"id"`
	ProjectID uint   `gorm:"not null;index"         json:"projectId"`
	Version   int64  `gorm:"not null"               json:"version"`
	Doc       string `gorm:"type:longtext;not null" json:"doc"`
	// Reason mirrors 工具1's vocabulary: `full_run` / `retype` / `merge` /
	// `split` / `revert_to_N`. A snapshot written by a path that allocates no
	// IDs uses `seed` / `import`.
	Reason    string `gorm:"type:varchar(64)" json:"reason"`
	CreatedAt int64  `gorm:"autoCreateTime"   json:"createdAt"`
}

func (WorldSnapshot) TableName() string { return "zsy_world_snapshots" }

// WorldEntitlement is a **capability**, not a quota: "this account may use
// world-ip" (docs/23 §6.5 — 能力是"能不能用"，额度是"能用多少"). Spend is
// accounted by the host's existing billing, so no counter lives here.
//
// Revocation and expiry are evaluated per request (WorldEntitlementActive),
// never cached into the client and never snapshotted at login.
type WorldEntitlement struct {
	ID        uint  `gorm:"primarykey"     json:"id"`
	CreatedAt int64 `gorm:"autoCreateTime" json:"createdAt"`

	UserID     int    `gorm:"not null;index"                  json:"userId"`
	Capability string `gorm:"type:varchar(64);not null;index" json:"capability"`
	// Source records how the capability was obtained: purchase / redeem /
	// admin. It is for operators, not for authorization decisions.
	Source string `gorm:"type:varchar(32)" json:"source"`
	// ExpiresAt is a unix second, or NULL for "never expires".
	ExpiresAt *int64 `json:"expiresAt"`
	// RevokedAt is a unix second, or NULL while the capability is live.
	RevokedAt *int64 `json:"revokedAt"`
}

func (WorldEntitlement) TableName() string { return "zsy_world_entitlements" }

// Capabilities (docs/23 §6.3). Two of them, because their cost differs by an
// order of magnitude: `world-ip` is pure function, `world-ip-ai` burns model
// money on every call.
const (
	// CapabilityWorldIP covers project.create / world.mutate / world.get /
	// world.validate.
	CapabilityWorldIP = "world-ip"
	// CapabilityWorldIPAI covers ingest.run / assets.plan.
	CapabilityWorldIPAI = "world-ip-ai"
)

// Entitlement sources.
const (
	SourcePurchase = "purchase"
	SourceRedeem   = "redeem"
	SourceAdmin    = "admin"
)

// Project lifecycle states. Only the two the read path needs are defined here;
// a later phase adds whatever ingest needs without changing these.
const (
	ProjectStatusActive   = "active"
	ProjectStatusArchived = "archived"
)

// Field bounds, kept inside the indexed varchar widths so MySQL (utf8mb4)
// accepts them.
const (
	maxProjectNameLen = 191
	maxNovelTitleLen  = 191
	maxReasonLen      = 64
)

// KnownCapabilities is the capability vocabulary the plugin will accept from the
// admin face — a typo there must fail loudly instead of silently granting an
// inert row. It is deliberately NOT used to authorize ops: ops are gated by
// requiredCapability (op_dispatch.go), which is derived from docs/23 §4.2.
var KnownCapabilities = []string{CapabilityWorldIP, CapabilityWorldIPAI}

// isKnownCapability reports whether name is one of KnownCapabilities.
func isKnownCapability(name string) bool {
	for _, c := range KnownCapabilities {
		if c == name {
			return true
		}
	}
	return false
}

// knownCapabilitiesText renders the vocabulary for an operator-facing message.
func knownCapabilitiesText() string {
	out := ""
	for i, c := range KnownCapabilities {
		if i > 0 {
			out += " / "
		}
		out += c
	}
	return out
}

// nowStamp is the single clock read of the plugin's write paths, so a row's
// timestamps can be reasoned about without chasing time.Now() calls. It is the
// host's helper, i.e. the same second-resolution clock the rest of new-api uses.
func nowStamp() int64 { return common.GetTimestamp() }
