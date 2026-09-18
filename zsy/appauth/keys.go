package appauth

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"gorm.io/gorm"
)

// keyCollisionRetries bounds the attempts to insert a freshly generated key
// when the random value collides with an existing row (tokens.key is unique).
const keyCollisionRetries = 3

// ensureDefaultKey returns the API key backing a third-party account, creating
// it on first use. The key takes the configured name and group and leaves every
// other field at the host's key defaults: unlimited quota, never expires, no
// model limits, no IP limits.
//
// The second return value reports whether this call created the key.
func ensureDefaultKey(userID int) (*model.Token, bool, error) {
	token, err := findDefaultKey(userID)
	if err != nil {
		return nil, false, err
	}
	if token != nil {
		return token, false, nil
	}
	token, err = insertDefaultKey(userID)
	if err != nil {
		return nil, false, err
	}
	return token, true, nil
}

// findDefaultKey looks the account key up by the configured name, then falls
// back to the configured group so that renaming the key in the dashboard does
// not orphan a third-party integration. Returns (nil, nil) when the account has
// no such key yet.
func findDefaultKey(userID int) (*model.Token, error) {
	var token model.Token
	err := model.DB.Where(&model.Token{UserId: userID, Name: cfg.DefaultKeyName}).
		Order("id desc").First(&token).Error
	if err == nil {
		return &token, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if cfg.DefaultKeyGroup == "" {
		return nil, nil
	}
	// A struct condition keeps GORM quoting the reserved `group` column per dialect.
	err = model.DB.Where(&model.Token{UserId: userID, Group: cfg.DefaultKeyGroup}).
		Order("id desc").First(&token).Error
	if err == nil {
		return &token, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return nil, err
}

// insertDefaultKey creates the account key. GORM generates the primary key, so
// the table stays portable across SQLite, MySQL and PostgreSQL.
func insertDefaultKey(userID int) (*model.Token, error) {
	for attempt := 0; attempt < keyCollisionRetries; attempt++ {
		key, err := common.GenerateKey()
		if err != nil {
			return nil, err
		}
		now := common.GetTimestamp()
		token := &model.Token{
			UserId:             userID,
			Name:               cfg.DefaultKeyName,
			Key:                key,
			Status:             common.TokenStatusEnabled,
			CreatedTime:        now,
			AccessedTime:       now,
			ExpiredTime:        -1,
			RemainQuota:        0,
			UnlimitedQuota:     true,
			ModelLimitsEnabled: false,
			Group:              cfg.DefaultKeyGroup,
		}
		if err := token.Insert(); err != nil {
			if isUniqueKeyCollision(err) {
				continue
			}
			return nil, err
		}
		return token, nil
	}
	return nil, errors.New("zsy-appauth: generated key collided with an existing token on every attempt")
}

// isUniqueKeyCollision is a best-effort cross-dialect detection of a duplicate
// tokens.key insert: SQLite reports "UNIQUE constraint failed", MySQL
// "Duplicate entry", PostgreSQL "duplicate key value".
func isUniqueKeyCollision(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique") || strings.Contains(msg, "duplicate")
}

// keyGroupWarning describes why the key's group is unreachable for this user,
// or returns an empty string when it is fine. A key whose group is not in the
// user's usable groups is rejected with 403 at relay time
// (middleware/auth.go), so the condition has to surface to the caller.
func keyGroupWarning(userGroup, keyGroup string) string {
	if keyGroup == "" || service.IsUserSelectableGroup(userGroup, keyGroup) {
		return ""
	}
	msg := fmt.Sprintf("Key 分组 %q 对用户分组 %q 不可用：请确认该分组已配置分组倍率并加入「用户可用分组」，否则使用该 Key 的中继请求会返回 403", keyGroup, userGroup)
	common.SysError("zsy-appauth: " + msg)
	return msg
}

func newUserView(user *model.User) userView {
	return userView{
		ID:          user.Id,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Group:       user.Group,
		Quota:       user.Quota,
		UsedQuota:   user.UsedQuota,
		Status:      user.Status,
	}
}

func newKeyView(token *model.Token, userGroup string) keyView {
	return keyView{
		ID:             token.Id,
		Name:           token.Name,
		Group:          token.Group,
		APIKey:         token.GetFullKey(),
		Status:         token.Status,
		UnlimitedQuota: token.UnlimitedQuota,
		ExpiredTime:    token.ExpiredTime,
		GroupUsable:    token.Group == "" || service.IsUserSelectableGroup(userGroup, token.Group),
	}
}
