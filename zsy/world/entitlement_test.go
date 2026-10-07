package world

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// =========================================================================
// ★★ 跨语言的那半条判据：Go 与 JS 必须算出同一个 `check`
//
// 计划里这块绑定信息是**服务端签发、客户端校验**的，所以两边的哈希必须
// **逐字节相同**。而"两边各自自洽、合起来对不上"的症状是
// **每一份文件都在导入时报「被改过」** —— 极难查，而且看起来像文件坏了。
//
// 所以这里钉的是**已知向量**（不是一个说不清的值，是两边都能对照的常数）：
//   - FNV-1a 64 的两个公开标准向量（空串 / "a"）—— 它们能在**任何**实现上复算；
//   - 规范文本的形状（恰好五行、`\n` 分隔）；
//   - 中文用户名的 UTF-8（站点允许多字节用户名，这是最容易两边分叉的一处）；
//   - 站点归一化"**只做两件事**"（多做一步就会分叉，而分叉是静默的）。
//
// ⚠ 对应的 JS 侧断言在 `src/core/plugin-entitlement.test.js`，
// 两组用的是**同一批常数**。改这里就要同步改那边。
// =========================================================================

// TestFNV1a64_KnownVectors 钉住算法本身。
//
// ★ 这两个值是 FNV-1a 64 的**公开标准向量**（offset basis 与 "a"），
// 不是"跑出来是多少就写多少" —— 所以 JS 侧用的是另外一份实现，
// 却必须算出同一对值。这一条就是那份对照。
func TestFNV1a64_KnownVectors(t *testing.T) {
	require.Equal(t, "cbf29ce484222325", fnv1a64Hex(""), "空串应当是 offset basis")
	require.Equal(t, "af63dc4c8601ec8c", fnv1a64Hex("a"))
}

func TestFNV1a64_ShapeAndSensitivity(t *testing.T) {
	for _, s := range []string{"", "a", "hello world", "世界", "zsy-世界-2026"} {
		got := fnv1a64Hex(s)
		require.Len(t, got, 16, "%q → %q", s, got)
		require.Regexp(t, `^[0-9a-f]{16}$`, got)
	}
	/* 中文：换了字就该换值（编码要是搞错，这两个很可能撞在一起） */
	require.NotEqual(t, fnv1a64Hex("张伟"), fnv1a64Hex("张玮"))
}

// TestPluginCanonical_Shape 钉住"恰好五行"。
func TestPluginCanonical_Shape(t *testing.T) {
	got := pluginCanonical("world-ip", 7, "zsy", "https://www.aipole.top/")
	require.Equal(t,
		[]string{"v1", "world-ip", "7", "zsy", "https://www.aipole.top"},
		splitLines(got),
		"规范文本的形状变了 —— JS 侧 `canonicalFor` 要同步改，否则两边哈希分叉",
	)
}

// TestPluginCheck_CoversEveryField 每一格都真的被盖住了。
//
// ⚠ 少盖一格不会报错：那一格就变成了"改了也不影响校验和"，
// 而这一整块的意义正是"改了能认出来"。
func TestPluginCheck_CoversEveryField(t *testing.T) {
	const (
		id   = "world-ip"
		uid  = 7
		name = "zsy"
		site = "https://www.aipole.top"
	)
	base := PluginCheck(id, uid, name, site)
	require.True(t, hasPrefix(base, PluginChecksumAlgo+":"), base)

	for _, other := range []string{
		PluginCheck("world-ip2", uid, name, site),      // 换插件
		PluginCheck(id, 8, name, site),                 // 换账号
		PluginCheck(id, uid, "zsy2", site),             // 换用户名
		PluginCheck(id, uid, name, "https://b.test"),   // 换站点
	} {
		require.NotEqual(t, base, other, "有一格没被盖住 —— 改了它 check 不变")
	}
}

// TestPluginCheck_SiteNormalisationIsOnlyTwoSteps 站点归一化**不许多做**。
//
// ★ 这一条是给"顺手改好一点"的人看的：把 scheme / host 折成小写看着无害，
// 但 JS 的 `toLowerCase()` 与 Go 的 `strings.ToLower()` 规则不逐字相同
// （非 ASCII 域名就会分叉），而分叉的症状是每份文件都报"被改过"。
// 真需要判"是不是同一个站点"时，那条路在**客户端**、且**不参与哈希**。
func TestPluginCheck_SiteNormalisationIsOnlyTwoSteps(t *testing.T) {
	require.Equal(t, "https://www.aipole.top", normalizePluginSite("  https://www.aipole.top/  "))
	require.Equal(t, "https://www.aipole.top", normalizePluginSite("https://www.aipole.top///"))
	require.Equal(t, "", normalizePluginSite("   "))
	/* ★ 大小写**原样保留** */
	require.Equal(t, "HTTPS://Example.COM", normalizePluginSite("HTTPS://Example.COM/"))
}

// TestPluginCheck_UnicodeUsername 中文用户名（最容易两边分叉的一处）。
//
// ⚠ 判据是"同一份输入两次算出同一个值 + 不同输入算出不同值"，
// 外加**形状**。真正的跨语言对照靠上面那两条标准向量 ——
// 它们在任何实现上都能复算，所以能证明"两边是同一个算法"。
func TestPluginCheck_UnicodeUsername(t *testing.T) {
	a := PluginCheck("world-ip", 12, "张伟", "https://www.aipole.top")
	b := PluginCheck("world-ip", 12, "张伟", "https://www.aipole.top")
	c := PluginCheck("world-ip", 12, "李四", "https://www.aipole.top")

	require.Equal(t, a, b, "同一个输入两次算出不同的值 —— 那不是哈希")
	require.NotEqual(t, a, c)
	require.Regexp(t, `^fnv1a64:[0-9a-f]{16}$`, a)
}

func splitLines(s string) []string {
	out := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
