package world

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// Entitlements — "may this account use this capability?"
//
// The whole point of this file is a negative one: a capability verdict is
// computed **per request**. docs/23 §8.4 and §5.3 both say the same thing from
// two directions — "每次 ops 现算（不是登录时算一次存本地）" and "`requires` 只是
// UX". Nothing here may be cached, memoized, or handed to a client as a token.
//
// extcore has no user-level capability middleware (its whole surface is four
// registries: plugin metadata, migrate models, route mounters, task adaptors),
// so the gate lives here, inside the plugin, and core new-api is untouched —
// which is the entire value of the plugin shape (docs/23 §6.1).
// =========================================================================

// ErrNotAuthenticated means the request carried no usable account key.
var ErrNotAuthenticated = errors.New("world: not authenticated")

// bearerToken extracts the account key from the Authorization header. It accepts
// the same forms the host's relay accepts ("Bearer <key>" with or without the
// `sk-` display prefix), so a client can reuse the exact key it uses for
// generation requests (docs/23 §4.1).
func bearerToken(c *gin.Context) string {
	raw := strings.TrimSpace(c.GetHeader("Authorization"))
	if raw == "" {
		return ""
	}
	if len(raw) >= 7 && strings.EqualFold(raw[:7], "bearer ") {
		raw = strings.TrimSpace(raw[7:])
	}
	return strings.TrimSpace(strings.TrimPrefix(raw, "sk-"))
}

// WorldAuthenticate resolves the account key of a user-face request.
//
// It is deliberately its own implementation rather than middleware.TokenAuth():
// TokenAuth answers in the OpenAI error shape, while every answer on this
// plugin's face must carry a docs/23 §4.1 code, and E_AUTH is one of them. The
// checks themselves are the host's (model.ValidateUserToken, the same call
// TokenAuth makes), so "which keys are valid" is still defined in one place.
func WorldAuthenticate(c *gin.Context) (int, error) {
	key := bearerToken(c)
	if key == "" {
		return 0, ErrNotAuthenticated
	}
	token, err := model.ValidateUserToken(key)
	if err != nil || token == nil {
		return 0, ErrNotAuthenticated
	}
	user, err := model.GetUserById(token.UserId, false)
	if err != nil || user == nil {
		return 0, ErrNotAuthenticated
	}
	if user.Status != common.UserStatusEnabled {
		return 0, ErrNotAuthenticated
	}
	// The user id goes into the request context so downstream code never has to
	// re-derive it; the capability verdict deliberately does NOT.
	c.Set(contextKeyUserID, user.Id)
	c.Set(contextKeyTokenID, token.Id)
	return user.Id, nil
}

// requireWorldAuth is the middleware wrapper around WorldAuthenticate.
func requireWorldAuth(c *gin.Context) {
	if _, err := WorldAuthenticate(c); err != nil {
		respondError(c, CodeAuth, "登录状态无效或密钥已失效：请重新登录后再试")
		c.Abort()
		return
	}
	c.Next()
}

// Context keys set by this plugin. They live here (not in constant/) because
// they belong to the plugin, not to the host.
const (
	contextKeyUserID  = "zsy_world_user_id"
	contextKeyTokenID = "zsy_world_token_id"
)

// CurrentUserID reads the user id resolved by requireWorldAuth. The second
// return value is false when the middleware did not run — which is a
// programming error in a route wiring, not a client error.
func CurrentUserID(c *gin.Context) (int, bool) {
	value, ok := c.Get(contextKeyUserID)
	if !ok {
		return 0, false
	}
	id, ok := value.(int)
	return id, ok
}

// ---------------------------------------------------------------------------
// The capability check itself
// ---------------------------------------------------------------------------

// WorldEntitlementActive reports whether userID holds capability right now.
//
// "Right now" is literal: revoked_at must be unset and expires_at must be either
// unset or in the future. Both are read from the database on every call, so the
// admin faces in controllers_admin.go take effect on the very next op — that is
// enforcement criterion C (docs/23 §9 step 7: "撤销之后立刻不能").
func WorldEntitlementActive(userID int, capability string) (bool, error) {
	if userID <= 0 || capability == "" {
		return false, nil
	}
	var count int64
	err := db().Model(&WorldEntitlement{}).
		Where("user_id = ? AND capability = ?", userID, capability).
		Where("revoked_at IS NULL").
		Where("expires_at IS NULL OR expires_at > ?", nowStamp()).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("world: check capability %q for user %d: %w", capability, userID, err)
	}
	return count > 0, nil
}

// WorldCapabilitiesActive returns the capability names userID holds right now,
// sorted, de-duplicated. It backs GET /api/zsy/world/entitlements, which exists
// so the client can draw the right entrance — and which is explicitly NOT a
// gate (docs/23 §4.2: "画入口用，纯 UX").
func WorldCapabilitiesActive(userID int) ([]string, error) {
	if userID <= 0 {
		return []string{}, nil
	}
	var rows []WorldEntitlement
	err := db().Model(&WorldEntitlement{}).
		Select("capability").
		Where("user_id = ?", userID).
		Where("revoked_at IS NULL").
		Where("expires_at IS NULL OR expires_at > ?", nowStamp()).
		Order("capability asc").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("world: list capabilities of user %d: %w", userID, err)
	}
	seen := make(map[string]struct{}, len(rows))
	out := make([]string, 0, len(rows))
	for i := range rows {
		name := rows[i].Capability
		if _, dup := seen[name]; dup {
			continue
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	return out, nil
}

// WorldCapabilityRequirement names the capability an op needs, plus whether the
// op needs one at all.
//
// docs/23 §4.2 makes `world.get` / `world.validate` login-only on purpose: they
// are "I already bought this, let me use my world", and gating them would only
// annoy paying users. An empty capability therefore means "login is the whole
// gate", not "forgot to set it".
type WorldCapabilityRequirement struct {
	Capability string
	LoginOnly  bool
}

// ---------------------------------------------------------------------------
// Grant / revoke (the admin face's write side)
// ---------------------------------------------------------------------------

// WorldEntitlementGrantInput describes one manual grant.
type WorldEntitlementGrantInput struct {
	UserID     int
	Capability string
	// Source defaults to SourceAdmin when empty.
	Source string
	// ExpiresAt is a unix second; nil means "never expires".
	ExpiresAt *int64
}

// WorldEntitlementGrant inserts a capability row.
//
// It grants rather than upserts: an expired or revoked row is history an
// operator may still need to read ("卖了要能收回" — docs/23 §8.4), and GORM's
// AutoMigrate owns no unique index that would forbid a second live row. Live
// duplicates are harmless because every read is an existence check.
func WorldEntitlementGrant(in WorldEntitlementGrantInput) (*WorldEntitlement, error) {
	if in.UserID <= 0 {
		return nil, fmt.Errorf("world: invalid user id %d", in.UserID)
	}
	if !isKnownCapability(in.Capability) {
		return nil, fmt.Errorf("world: unknown capability %q (known: %s)", in.Capability, knownCapabilitiesText())
	}
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = SourceAdmin
	}
	if in.ExpiresAt != nil && *in.ExpiresAt <= 0 {
		return nil, fmt.Errorf("world: expires_at must be a positive unix second")
	}
	row := &WorldEntitlement{
		UserID:     in.UserID,
		Capability: in.Capability,
		Source:     source,
		ExpiresAt:  in.ExpiresAt,
	}
	if err := db().Create(row).Error; err != nil {
		return nil, fmt.Errorf("world: grant %q to user %d: %w", in.Capability, in.UserID, err)
	}
	return row, nil
}

// WorldEntitlementRevoke marks every live row of (userID, capability) as revoked
// as of now and reports how many rows it touched.
//
// It stamps `revoked_at` instead of deleting: the row is the record of why the
// account used to have access, and "撤销" must be reversible by a later grant
// without losing that trail. A count of 0 means there was nothing live to revoke
// — reported to the operator rather than treated as success.
func WorldEntitlementRevoke(userID int, capability string) (int64, error) {
	if userID <= 0 || capability == "" {
		return 0, fmt.Errorf("world: revoke needs a user id and a capability")
	}
	res := db().Model(&WorldEntitlement{}).
		Where("user_id = ? AND capability = ?", userID, capability).
		Where("revoked_at IS NULL").
		Update("revoked_at", nowStamp())
	if res.Error != nil {
		return 0, fmt.Errorf("world: revoke %q from user %d: %w", capability, userID, res.Error)
	}
	return res.RowsAffected, nil
}

// WorldEntitlementListByUser returns every entitlement row of a user, newest
// first, including expired and revoked ones — the admin face needs the history,
// not just the current verdict.
func WorldEntitlementListByUser(userID int) ([]WorldEntitlement, error) {
	if userID <= 0 {
		return nil, nil
	}
	var rows []WorldEntitlement
	err := db().Model(&WorldEntitlement{}).
		Where("user_id = ?", userID).
		Order("id desc").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("world: list entitlements of user %d: %w", userID, err)
	}
	return rows, nil
}
