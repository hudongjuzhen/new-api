package mode

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

// =========================================================================
// ★★ 授权：「这个账号能不能开通这一档」（`docs/28` §3b）
//
// 用户对这一类模式的定性（逐字）：
//
//	"私有不显示。当后台给权限之后才显示，并且能够一键开通"
//
// 三件事，而它们是**同一件事的三个面**：
//
//	① **不显示** —— 没有权限时它不在 `GET /api/zsy/mode/list` 里；
//	② **给了权限之后才显示** —— 所以列表必须**认账号**（`auth.go`）；
//	③ **并且能够一键开通** —— 所以取件那一条也要放行（`getModeFile`）。
//
// # ⚠★ 判据一处：`ModeEntitlementActive`
//
// 列表与取件走的是**同一个函数**（列表用批量版 `ModeEntitlementsActive`，
// 两者共用同一条 WHERE）。两处各写一遍的后果很具体：
// **列得出来、却取不到**（或反过来，取得到而列表里没有），
// 而两种表现都只在"私有 + 有权限"这个组合上出现 —— 那是最少被测到的一条路。
//
// # ⚠★ 每次现算，绝不缓存
//
// 与 `zsy/world` 的 `WorldEntitlementActive` 逐字同一条纪律：撤销之后
// **下一次请求立刻**就看不到了。这里没有"登录时算一次存本地"那一套，
// 也没有任何内存缓存 —— 运营在后台点一下撤销，用户在广场上刷新一下就没了。
// =========================================================================

// db exposes the host connection to this plugin's store layer only.
func db() *gorm.DB { return model.DB }

/* ==========================================================================
 * 判权
 * ======================================================================== */

// entitlementLiveQuery is the one WHERE clause that decides "this grant counts".
//
// ⚠★ 它是**一个函数**而不是两处复制的条件串：上面那一段说的"判据一处"
// 靠的就是它。`revoked_at IS NULL`（没被撤销）与
// `expires_at IS NULL OR expires_at > now`（没到期）两条缺一不可 ——
// 只写前者的表现是"到期了还能用"，而那是**收费**那一类模式最要命的坏法。
func entitlementLiveQuery(q *gorm.DB, at int64) *gorm.DB {
	return q.Where("revoked_at IS NULL").
		Where("expires_at IS NULL OR expires_at > ?", at)
}

// ModeEntitlementActive reports whether userID may open modeID right now.
//
// ⚠ userID <= 0（匿名）一律 false，而且**不查库**：这一面在没有 Authorization
// 头时是公开的（`docs/28` §4），匿名请求不该产生一次数据库往返。
func ModeEntitlementActive(userID int, modeID string) (bool, error) {
	if userID <= 0 || modeID == "" {
		return false, nil
	}
	var count int64
	err := entitlementLiveQuery(db().Model(&ModeEntitlement{}), nowStamp()).
		Where("user_id = ? AND mode_id = ?", userID, modeID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("mode: check entitlement %q for user %d: %w", modeID, userID, err)
	}
	return count > 0, nil
}

// ModeEntitlementsActive returns the set of mode ids userID holds right now.
//
// ★★ It exists because the list face needs the answer for **every** row it is
// about to draw: asking `ModeEntitlementActive` once per mode would be an N+1
// query on a page whose whole point is "站上有哪几档".
//
// ⚠ 它与 `ModeEntitlementActive` **共用那一条 WHERE**（`entitlementLiveQuery`），
// 所以两条路不可能给出相反的回答。
func ModeEntitlementsActive(userID int) (map[string]bool, error) {
	out := map[string]bool{}
	if userID <= 0 {
		return out, nil
	}
	var rows []ModeEntitlement
	err := entitlementLiveQuery(db().Model(&ModeEntitlement{}), nowStamp()).
		Select("mode_id").
		Where("user_id = ?", userID).
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mode: list entitlements of user %d: %w", userID, err)
	}
	for i := range rows {
		out[rows[i].ModeID] = true
	}
	return out, nil
}

/* ==========================================================================
 * 授予 / 撤销（后台那一面的写侧）
 * ======================================================================== */

// ModeEntitlementGrantInput describes one manual grant.
type ModeEntitlementGrantInput struct {
	UserID int
	ModeID string
	// Source defaults to SourceAdmin when empty.
	Source string
	// ExpiresAt is a unix second; nil means "never expires".
	ExpiresAt *int64
}

// ModeEntitlementGrant inserts one grant row.
//
// ⚠★ 它会**先确认这一档真的存在**（磁盘上有 `modes/<id>.json`）——
// 给一个不存在的 id 授权，产生的是一行**永远不起作用的记录**，而运营看到的是
// "已经授权了"，用户那边什么都没有。那种"两边都以为好了"的坏法正是本工程
// 最忌讳的一类，所以这里**拒绝**，并告诉他该先把文件放进去。
//
// ⚠ 它**不做 upsert**：过期/撤销的那几行是运营可能还要读的历史
// （与 `WorldEntitlementGrant` 同一条），而"当前生效"是每次都现算的。
// 重复授予产生的多行没有害处 —— 每一次判断都是存在性检查。
func ModeEntitlementGrant(in ModeEntitlementGrantInput) (*ModeEntitlement, error) {
	if in.UserID <= 0 {
		return nil, fmt.Errorf("mode: 需要 user_id（授权是挂在账号上的）")
	}
	modeID := strings.TrimSpace(in.ModeID)
	if !isValidModeID(modeID) {
		return nil, fmt.Errorf("mode: %q 不是一个模式 id（小写字母开头，字母数字下划线短横线，最长 32）", in.ModeID)
	}
	// ★ 存量判据只有一处（`ModeByID` 读的就是磁盘上那个文件）。
	if _, found := ModeByID(modeID); !found {
		return nil, fmt.Errorf("mode: 模式库里没有 %q 这一档，"+
			"所以现在给它授权不会生效。请先把 %s.json 放进 <%s> 再授权。",
			modeID, modeID, ModeDir())
	}
	source := strings.TrimSpace(in.Source)
	if source == "" {
		source = SourceAdmin
	}
	if in.ExpiresAt != nil && *in.ExpiresAt <= 0 {
		return nil, fmt.Errorf("mode: expires_at 必须是正的 unix 秒（不填表示永不过期）")
	}
	row := &ModeEntitlement{
		UserID:    in.UserID,
		ModeID:    modeID,
		Source:    source,
		ExpiresAt: in.ExpiresAt,
	}
	if err := db().Create(row).Error; err != nil {
		return nil, fmt.Errorf("mode: grant %q to user %d: %w", modeID, in.UserID, err)
	}
	return row, nil
}

// ModeEntitlementRevoke marks every live row of (userID, modeID) as revoked as
// of now and reports how many rows it touched.
//
// ⚠ 它盖 `revoked_at` 而不是删行：那一行是"这个账号曾经为什么能用"的记录
// （与 `WorldEntitlementRevoke` 同一条）。返回 0 表示本来就没有生效中的行 ——
// 那是运营的一个误操作，**必须报给他**，不能当成成功。
func ModeEntitlementRevoke(userID int, modeID string) (int64, error) {
	if userID <= 0 || strings.TrimSpace(modeID) == "" {
		return 0, fmt.Errorf("mode: 撤销需要 user_id 与 mode_id")
	}
	res := entitlementLiveQuery(db().Model(&ModeEntitlement{}), nowStamp()).
		Where("user_id = ? AND mode_id = ?", userID, strings.TrimSpace(modeID)).
		Update("revoked_at", nowStamp())
	if res.Error != nil {
		return 0, fmt.Errorf("mode: revoke %q from user %d: %w", modeID, userID, res.Error)
	}
	return res.RowsAffected, nil
}

/* ==========================================================================
 * 读（后台那一面）
 * ======================================================================== */

// ModeEntitlementListByUser returns every grant row of a user, newest first,
// **including expired and revoked ones** — the operator's question is usually
// "他到底有没有 / 什么时候没的", and a face that only showed live rows could not
// answer it.
func ModeEntitlementListByUser(userID int) ([]ModeEntitlement, error) {
	if userID <= 0 {
		return nil, nil
	}
	var rows []ModeEntitlement
	err := db().Model(&ModeEntitlement{}).
		Where("user_id = ?", userID).
		Order("id desc").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mode: list entitlements of user %d: %w", userID, err)
	}
	return rows, nil
}

// ModeEntitlementLiveByMode answers "who may open this mode right now", newest
// first. It backs the admin list's per-mode column ("这一档给了几个人").
//
// ⚠ 它只回**生效中**的那几行（与上面那个按账号读的相反）：这里的问题是
// "现在谁能用"，而不是"历史是什么" —— 两个问题的答案不一样，所以是两个函数。
func ModeEntitlementLiveByMode(modeID string) ([]ModeEntitlement, error) {
	if strings.TrimSpace(modeID) == "" {
		return nil, nil
	}
	var rows []ModeEntitlement
	err := entitlementLiveQuery(db().Model(&ModeEntitlement{}), nowStamp()).
		Where("mode_id = ?", strings.TrimSpace(modeID)).
		Order("id desc").
		Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mode: list live entitlements of %q: %w", modeID, err)
	}
	return rows, nil
}

// ModeEntitlementLiveCounts answers, for every mode id at once, how many live
// grants it has. A mode with no grants is simply absent from the map.
//
// ★★ It is the batch form of ModeEntitlementLiveByMode, and it exists for the
// same reason ModeEntitlementsActive does: the admin list draws a count on every
// row, so asking per row would be one query per mode file.
func ModeEntitlementLiveCounts() (map[string]int, error) {
	var rows []struct {
		ModeID string
		Total  int
	}
	err := entitlementLiveQuery(db().Model(&ModeEntitlement{}), nowStamp()).
		Select("mode_id, COUNT(*) AS total").
		Group("mode_id").
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("mode: count live entitlements: %w", err)
	}
	out := make(map[string]int, len(rows))
	for _, r := range rows {
		out[r.ModeID] = r.Total
	}
	return out, nil
}
