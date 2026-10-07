package world

import "encoding/json"

// =========================================================================
// Request shapes.
//
// docs/23 §4.3 says the field list is NOT specified in the design document —
// "字段由 zsy/world 的 Go struct 定义，客户端按 E_INPUT 的原话去改". That makes
// this file the contract: one struct per op, one project id type, and an
// explicit error message for anything the struct cannot bind.
//
// A shape here is a *transport* decision (which field, which type). It is never
// a rule about the world: no field in this file is allowed to describe element
// types, ID formats, prompt templates or consistency — those live in the engine
// and the validator (docs/23 §6.4).
// =========================================================================

// opRequest is the envelope of POST /api/zsy/world/op.
//
// `params` is kept raw at this level: each op decodes it into its own struct so a
// field that belongs to a different op is rejected instead of silently ignored.
type opRequest struct {
	Op     string          `json:"op"`
	Params json.RawMessage `json:"params"`
}

// projectRef is the addressing shape shared by every op that names a project.
type projectRef struct {
	ProjectID uint `json:"project_id"`
}

// worldGetParams addresses one project's document.
//
// SinceVersion is optional: a client that already holds a snapshot sends the
// version it has, and the server answers `changed: false` with no `doc` when it
// is already current. That is the "静默重取" path of docs/23 §5.2, and it keeps a
// multi-megabyte document off the wire when nothing moved.
type worldGetParams struct {
	ProjectID    uint  `json:"project_id"`
	SinceVersion int64 `json:"since_version"`
}

// worldValidateParams carries the document to check. The document is passed
// through untouched: validation is the Node validator's job.
type worldValidateParams struct {
	ProjectID uint            `json:"project_id"`
	Doc       json.RawMessage `json:"doc"`
	// Version, when set, names the stored version to validate. When both Doc and
	// Version are given, Doc wins — the caller asked about that exact text.
	Version int64 `json:"version"`
}

// projectCreateParams is the address of a new world project.
//
// ⚠ `novel_title` is not required here, and that is deliberate: an empty title
// produces a document the JSON Schema refuses (`minLength: 1`), and refusing on
// that basis is a *world rule* — the gate answers it by asking the authority
// instead (see op_project.go).
type projectCreateParams struct {
	Name       string `json:"name"`
	NovelTitle string `json:"novel_title"`
}

// projectListParams is the address of the caller's world list.
//
// ⚠ There is deliberately **no** `user_id`: this op answers the caller's own
// worlds. The admin face has its own route for cross-user reading, and a user-face
// op that could name another account would be a cross-tenant read waiting to
// happen (see op_project.go).
type projectListParams struct {
	// Limit caps one page; 0 or out of range means the plugin's ceiling.
	Limit uint32 `json:"limit"`
}

// worldMutateParams is every field world.mutate accepts, across all four kinds.
//
// ⚠ One struct rather than four: the wire shape is one op with a `kind`
// discriminator (docs/23 §4.2 lists `world.mutate` as a single op covering
// retype/rename/merge/split), and the required subset per kind is enforced in
// validateWorldMutate. Four structs would make the discriminator invisible on the
// wire, where a client actually has to send it.
type worldMutateParams struct {
	ProjectID uint   `json:"project_id"`
	Kind      string `json:"kind"`
	// BaseVersion is the version the caller read. Mandatory — see docs/23 §4.4.
	BaseVersion int64 `json:"base_version"`

	// rename / retype
	ElementID string `json:"element_id"`
	NewName   string `json:"new_name"`
	ToType    string `json:"to_type"`
	Reason    string `json:"reason"`

	// merge
	IDA            string `json:"id_a"`
	IDB            string `json:"id_b"`
	Keep           string `json:"keep"`
	AllowCrossType bool   `json:"allow_cross_type"`

	// split
	ID           string   `json:"id"`
	NewAlias     []string `json:"new_alias"`
	NewTypeID    string   `json:"new_type_id"`
	FirstChapter uint32   `json:"first_chapter"`
}

// ingestRunParams is the request of the only op that costs money (docs/23 §4.2).
//
// ★ `source_text` carries the novel原文 itself. It is inline rather than a path on
// purpose: a path parameter would make "read any file on the server" part of the
// API. The engine writes it to a temp file internally (it must — the pipeline reads
// by path) and deletes it afterwards; that detail stays on the engine's side.
//
// ⚠ The body limit must accommodate a novel: see `ZSY_WORLD_OP_BODY_LIMIT`.
type ingestRunParams struct {
	ProjectID   uint   `json:"project_id"`
	BaseVersion int64  `json:"base_version"`
	SourceText  string `json:"source_text"`
	NovelTitle  string `json:"novel_title"`
	// MaxChapters caps how many chapters this run parses. Absent means "all".
	MaxChapters uint32 `json:"max_chapters"`
	// Media toggles prompt / anchor / sound planning (PRD 6.5–6.7). Default true;
	// it is the most expensive part and can be re-run later (it does not change ids).
	Media *bool `json:"media"`
	// Model overrides the model used by the run AND the model billed against.
	Model string `json:"model"`
	// Group overrides the group used for the model/group ratio lookup.
	Group string `json:"group"`

	// MockScript drives the engine's MockProvider instead of the real gateway.
	//
	// ⚠★ TEST ONLY, and unreachable in production: `world-parser-svc` only *reads*
	// this parameter when it was built with `--features test-provider`. A release
	// build ignores it entirely and goes to the real gateway — which is the point:
	// `ingest.run` is a paid op, so a switch that could inject a fake provider would
	// be a way to get results for free.
	//
	// It exists so the billing arithmetic can be tested against *real token counts*
	// without spending anything: the engine still reports usage, it just answers
	// from a script.
	MockScript []string `json:"mock_script"`
}

// =========================================================================
// Admin shapes — /dashboard/zsy/world/**
// =========================================================================

// adminGrantParams is the body of POST /dashboard/zsy/world/entitlements/grant.
type adminGrantParams struct {
	UserID     int    `json:"user_id"`
	Capability string `json:"capability"`
	Source     string `json:"source"`
	// ExpiresAt is a unix second; 0 / absent means "never expires".
	ExpiresAt int64 `json:"expires_at"`
}

// adminRevokeParams is the body of POST /dashboard/zsy/world/entitlements/revoke.
type adminRevokeParams struct {
	UserID     int    `json:"user_id"`
	Capability string `json:"capability"`
}
