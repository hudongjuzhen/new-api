package mode

import "github.com/QuantumNous/new-api/common"

// =========================================================================
// ★★ 唯一的一张表：`zsy_mode_entitlements` —— "这个账号能开通哪几档私有模式"
//
// 它是 `docs/28` §6 第 3b 段加上的，而**模式的正文一个字都不进数据库**：
// 存量仍然是磁盘上的 `modes/<id>.json`（`catalog.go` 文件头那一段说的是
// "服务端那一份 = 客户端本地那一份"，数据库里的大字段做不到这件事）。
//
// 表里只有**分发策略**那一半：`id` 是模式 id（`audiobook` 这种），
// 而"它是什么、正文写了什么"全在文件里。⚠★ 这一条分工很重要 ——
// 模式文件是可以整个删掉、换掉、拷给别人的，而这张表只回答一件事。
//
// # ⚠★ 为什么不是"给插件那张表加一列"
//
// `zsy_world_entitlements` 的回答单位是**能力**（`world-ip`），而这里的回答
// 单位是**一档模式**（几十个、运营随时会加减）。两者共表的话：
//
//	① 模式 id 得编成 `mode:audiobook` 这样的字符串塞进 `capability`，
//	   而那一列的类型与长度是照"能力名"定的（varchar(64)）；
//	② 更要紧的是**两个插件从此互相知道对方**（统一前缀一旦成为约定，
//	   改一处就得同时改两处），而 extcore 那套插件的全部价值就是各管各的。
//
// 所以是**另一张表**（表名前缀 `zsy_` + 插件名，与 `zsy_world_*` 同一条规矩）。
// =========================================================================

// ModeEntitlement is one grant: "account N may open mode X".
//
// ⚠★ 它**不是一个额度**（能开几次），而是**一个权限**（能不能开）——
// 与 `WorldEntitlement` 那一条逐字同源（`docs/23` §6.5：能力是"能不能用"，
// 额度是"能用多少"）。开通一份模式是一个**动作**，动作本身不计次。
type ModeEntitlement struct {
	ID        uint  `gorm:"primarykey"     json:"id"`
	CreatedAt int64 `gorm:"autoCreateTime" json:"createdAt"`

	UserID int `gorm:"not null;index" json:"userId"`
	// ModeID is the mode id — the file name without `.json`
	// (varchar(32) because that is `isValidModeID`'s own ceiling).
	ModeID string `gorm:"type:varchar(32);not null;index" json:"modeId"`
	// Source records how it was obtained: purchase / redeem / admin.
	// It is for operators, never for the authorization decision.
	Source string `gorm:"type:varchar(32)" json:"source"`
	// ExpiresAt is a unix second, or NULL for "never expires".
	ExpiresAt *int64 `json:"expiresAt"`
	// RevokedAt is a unix second, or NULL while the grant is live.
	RevokedAt *int64 `json:"revokedAt"`
}

// TableName pins the table to a plugin-prefixed name.
//
// ⚠ 宿主把一张数据库与每个插件共用，而裸的 `entitlements` 太泛了 ——
// 与 `WorldEntitlement.TableName` 同一条理由。
func (ModeEntitlement) TableName() string { return "zsy_mode_entitlements" }

// Grant sources. The vocabulary is copied from `zsy/world` on purpose: an
// operator reading two admin screens should not have to learn two spellings of
// "后台给的".
const (
	SourcePurchase = "purchase"
	SourceRedeem   = "redeem"
	SourceAdmin    = "admin"
)

// nowStamp is the single clock read of this plugin's write paths, so a row's
// timestamps can be reasoned about without chasing time.Now() calls (it is the
// host's helper, i.e. the same second-resolution clock the rest of new-api uses).
func nowStamp() int64 { return common.GetTimestamp() }
