package mode

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// =========================================================================
// ★★ 公开面**认账号**（`docs/28` §3b 的第 ①②③ 里的 ①）
//
// 加了私有模式之后，`GET /api/zsy/mode/list` 的回答**取决于你是谁**：
//
//	没有 Authorization 头   → 只列 public（与加这一层之前**逐字相同**）
//	有、而且有效            → public + 这个账号持有权限的那几档私有模式
//	有、但无效 / 已失效      → ★ **报错**，不是当成匿名
//
// # ⚠★ 为什么"无效的密钥"必须报错，而不是悄悄降级成匿名
//
// 这一条是这一层全部的价值所在。降级成匿名的表现是：
//
//	一个买过 `audiobook` 的账号，密钥过期之后打开广场，
//	看到的**与从没买过的人一模一样** —— 少了两档，而屏幕上**一个字都没说**。
//
// 那正是本工程最忌讳的坏法（"同一个问题有两个答案，而其中一个不报错"）：
// 用户会以为自己的东西丢了，然后去问运营。而报错那句话是能照做的
// （"重新登录"），代价只是那一屏当时刷不出来。
//
// # ⚠★ 为什么不用 `middleware.TokenAuth()`
//
// 它是对的（校验用的就是同一个 `model.ValidateUserToken`），但它有**两个**
// 不合适的地方：① 它要求**一定有**密钥（这一面必须允许匿名）；
// ② 它回的是 OpenAI 那套错误形状，而这一面回的是宿主的
// `common.ApiErrorMsg`（与 `getModeFile` 那几条错误同一种）。所以自己写一层，
// 但校验那一句仍然走宿主 —— "哪些密钥是有效的"依旧只有一处定义。
// =========================================================================

// contextKeyUserID is where the resolved account id is parked for the two
// public handlers. It lives here (not in `constant/`) because it belongs to this
// plugin, not to the host.
const contextKeyUserID = "zsy_mode_user_id"

// bearerToken extracts the account key from the Authorization header.
//
// ⚠ 它接受宿主 relay 认的那两种写法（`Bearer <key>`，带不带 `sk-` 显示前缀），
// 于是客户端可以**原样复用**它调生成接口那把密钥 —— 与 `zsy/world`
// 的 `bearerToken` 逐字同形（两处都抄的宿主 relay 的取法）。
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

// resolveModeAccount is the optional-auth middleware of the public face.
//
//	无头   → 匿名，继续（这一面按定义要允许"还没登录也能看见有哪些公共模式"）
//	有头   → 校验；有效就把账号 id 放进 context，无效就**当场报错、不继续**
//
// ⚠★ 它会读一次数据库（`model.GetUserById`），而**匿名那一条路一次都不读**
// —— 广场是打开就要刷的一屏，而绝大多数访问是匿名的（`docs/28` §4：
// "还没登录也能看见广场里有什么"是刻意的）。
//
// ⚠ 用户被禁用也算无效：一个被停用的账号不该还能看见他买过的私有模式清单。
//
//	判据（`Status != common.UserStatusEnabled`）与 `zsy/world` 那一条同源。
func resolveModeAccount(c *gin.Context) {
	key := bearerToken(c)
	if key == "" {
		c.Next()
		return
	}
	token, err := model.ValidateUserToken(key)
	if err != nil || token == nil {
		common.ApiErrorMsg(c, "登录状态无效或密钥已失效：请重新登录后再试（这一屏要按账号列出你能开通的模式）。")
		c.Abort()
		return
	}
	user, err := model.GetUserById(token.UserId, false)
	if err != nil || user == nil || user.Status != common.UserStatusEnabled {
		common.ApiErrorMsg(c, "登录状态无效或密钥已失效：请重新登录后再试（这一屏要按账号列出你能开通的模式）。")
		c.Abort()
		return
	}
	c.Set(contextKeyUserID, user.Id)
	c.Next()
}

// currentUserID reads the account id resolved by resolveModeAccount.
//
// The second return value is false for an anonymous request — which is a
// **normal** state on this face, not a wiring error (compare
// `world.CurrentUserID`, where it would be one).
func currentUserID(c *gin.Context) (int, bool) {
	value, ok := c.Get(contextKeyUserID)
	if !ok {
		return 0, false
	}
	id, ok := value.(int)
	return id, ok
}
