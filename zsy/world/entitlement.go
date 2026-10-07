package world

import (
	"fmt"
	"strings"
)

// =========================================================================
// ★★ 插件文件上那一块"这是给谁的"（docs/22 §2.2）
//
// 服务端按「某个用户 + 某个插件」签发一份插件文件，用户导入时客户端会核对
// 账号 —— 于是"我给了谁"有一个**看得见的凭据**，而"运营把给 A 的文件发给了 B"
// 这件事会在**导入那一刻**被说出来，而不是装上一看菜单有、点进去说"你没有能力"。
//
// ⚠★ 它的作用是**一致性，不是安全**（`plugin-entitlement.js` 的文件头写了整段）：
// 文件里的账号谁都能改，改了也确实绕不过去 —— 但**改了也没用**，因为真正的闸门
// 是这里每次 op 现算的那一下（`docs/23` §8.3 判据 ①）。所以校验和（`check`）是
// **防误改**的，不是防伪的：算法公开、无密钥。
//
// # 这个文件与 JS 侧是**一对**
//
// `src/core/plugin-entitlement.js` 里的 `fnv1a64` / `canonicalFor` / `computeCheck`
// 与这里必须算出**逐字节相同**的结果。两边的规则写在下面，并且各有**已知向量**
// 钉着（`entitlement_test.go` 与 JS 那份测试里是同一组常数）。
//
// ⚠ 两边分叉的症状是**每一份文件都在导入时报"被改过"** —— 极难查，而且看起来
// 像文件坏了。所以那组向量不是形式主义，它是这条跨语言契约唯一的探针。
// =========================================================================

// PluginEntitlementField 是文件里那一格的键名（契约的一部分）。
//
// ⚠ 与 `plugin_issue.go` 的 `PluginEntitlementBlock` 的字段名一起构成**跨语言契约**
// （客户端 `src/core/plugin-entitlement.js` 读的就是它们）。改任一处都要同时改客户端。
const PluginEntitlementField = "entitlement"

// PluginChecksumAlgo 是校验和算法的标识。
//
// ⚠ 换算法要加**新前缀**（`sha256:` 之类），不许改这一条的语义：
// 已经发出去的文件上写着 `fnv1a64:`，改语义会让它们全部作废。
const PluginChecksumAlgo = "fnv1a64"

// pluginCanonicalVersion 是规范文本的第一行。
//
// 它管的是"盖住哪些字段"：以后想在哈希里加一格时，用它区分新旧规则，
// 而不是让老文件失效。与 `formatVersion` 是同一条思路。
const pluginCanonicalVersion = "v1"

// fnv1a64Hex 算 FNV-1a 64 位，返回 16 位小写十六进制。
//
// ⚠ Go 的 `uint64` 溢出回绕就是 FNV-1a 要的语义（无符号加/乘自然截断），
// 所以这里不需要显式掩码 —— 而 JS 那边**必须**手动 `& 0xffff...`（BigInt 不回绕）。
// 两边写出不同的形状、算出同一个数，正是下面那组向量的用处。
func fnv1a64Hex(input string) string {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	var hash uint64 = offset64
	for _, b := range []byte(input) {
		hash ^= uint64(b)
		hash *= prime64
	}
	return fmt.Sprintf("%016x", hash)
}

// pluginCanonical 是一份插件绑定信息要盖住的那串**规范文本**。
//
// # 形状（与 JS 侧 `canonicalFor` 逐字一致）
//
//	<算法版本>\n<插件 id>\n<账号 id>\n<用户名>\n<站点>
//
// | 行 | 为什么在里面 |
// |---|---|
// | `v1` | 以后改"盖住哪些字段"时，旧文件仍能按 v1 判 |
// | 插件 id | 把 A 插件的绑定块挪到 B 插件上 → 认得出 |
// | 账号 id | 判据本体 |
// | 用户名 | 只是**把提示说清楚**（"给 zsy 的"），不是判据 |
// | 站点 | 两个站点账号不通用（与 `session.js` 那条同源） |
//
// ⚠ **不**包含 `capabilities` / `issuedAt`：那两格是展示信息，改了不该判"被改过"。
//
// ⚠ 站点**只做 trim + 去尾斜杠**（`normalizePluginSite`）：这个值要参与哈希，
// 所以两边的归一化必须逐字节一致，而"大小写无关"在两种语言里不是同一个变换
// （`toLowerCase` 走 Unicode 映射，`strings.ToLower` 的规则并不逐字相同）。
func pluginCanonical(pluginID string, userID int, username, site string) string {
	return strings.Join([]string{
		pluginCanonicalVersion,
		pluginID,
		fmt.Sprintf("%d", userID),
		username,
		normalizePluginSite(site),
	}, "\n")
}

// normalizePluginSite 只去空白与尾斜杠 —— 与 JS 的 `normalizeSite` 逐字一致，
// 也与 `src-tauri` 的 `config::normalize_account_base` 同一个意图。
func normalizePluginSite(site string) string {
	return strings.TrimRight(strings.TrimSpace(site), "/")
}

// PluginCheck 算出文件里那个 `check` 值（形如 `fnv1a64:1a2b…`）。
func PluginCheck(pluginID string, userID int, username, site string) string {
	return PluginChecksumAlgo + ":" +
		fnv1a64Hex(pluginCanonical(pluginID, userID, username, site))
}
