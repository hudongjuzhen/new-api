package model

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CheckInviteCode is the single gate every registration entry point (web,
// OAuth, WeChat, third-party app API) calls, so its decision table is the
// registration contract: an invite-only site must never create an account
// without a resolvable invite code, and a site that does not ask for one must
// keep accepting registrations without a code.
func TestCheckInviteCode(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)

	inviter := User{
		Username: "invite-code-inviter",
		Password: "unused-password-hash",
		Status:   common.UserStatusEnabled,
		AffCode:  "inv1",
	}
	require.NoError(t, DB.Create(&inviter).Error)

	deleted := User{
		Username: "invite-code-deleted",
		Password: "unused-password-hash",
		Status:   common.UserStatusEnabled,
		AffCode:  "gone1",
	}
	require.NoError(t, DB.Create(&deleted).Error)
	require.NoError(t, DB.Delete(&deleted).Error)

	previousRequired := common.InviteCodeRequired
	t.Cleanup(func() { common.InviteCodeRequired = previousRequired })

	t.Run("optional mode keeps the historical behaviour", func(t *testing.T) {
		common.InviteCodeRequired = false

		for _, code := range []string{"", "   ", "unknown-code"} {
			inviterID, err := CheckInviteCode(code)
			require.NoError(t, err, "code %q must not block registration", code)
			assert.Zero(t, inviterID)
		}

		inviterID, err := CheckInviteCode("inv1")
		require.NoError(t, err)
		assert.Equal(t, inviter.Id, inviterID)
	})

	t.Run("required mode refuses empty and unknown codes", func(t *testing.T) {
		common.InviteCodeRequired = true

		_, err := CheckInviteCode("")
		assert.ErrorIs(t, err, ErrInviteCodeRequired)

		_, err = CheckInviteCode("   ")
		assert.ErrorIs(t, err, ErrInviteCodeRequired)

		_, err = CheckInviteCode("unknown-code")
		assert.ErrorIs(t, err, ErrInviteCodeInvalid)

		_, err = CheckInviteCode("gone1")
		assert.ErrorIs(t, err, ErrInviteCodeInvalid, "a deleted user's code must not stay redeemable")

		_, err = CheckInviteCode(strings.Repeat("x", maxInviteCodeLength+1))
		assert.ErrorIs(t, err, ErrInviteCodeInvalid, "a code longer than the column width can never match")

		inviterID, err := CheckInviteCode(" inv1 ")
		require.NoError(t, err)
		assert.Equal(t, inviter.Id, inviterID, "surrounding whitespace is trimmed")
	})
}

// The admin switch is only useful if the option table's write path actually
// reaches the runtime flag: the system settings UI toggles it live, and the
// registration gate reads common.InviteCodeRequired on every request.
func TestUpdateOptionInviteCodeRequiredReachesRuntimeFlag(t *testing.T) {
	previousRequired := common.InviteCodeRequired
	previousOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	t.Cleanup(func() {
		common.InviteCodeRequired = previousRequired
		common.OptionMap = previousOptionMap
	})

	require.NoError(t, DB.AutoMigrate(&Option{}))

	require.NoError(t, UpdateOption("InviteCodeRequired", "true"))
	assert.True(t, common.InviteCodeRequired)
	assert.Equal(t, "true", common.OptionMap["InviteCodeRequired"])

	require.NoError(t, UpdateOption("InviteCodeRequired", "false"))
	assert.False(t, common.InviteCodeRequired)
	assert.Equal(t, "false", common.OptionMap["InviteCodeRequired"])
}
