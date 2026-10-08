package world

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// ★★ GET /api/zsy/plugins —— 公共插件目录（docs/27 §3）
//
// 这一组守的是用户 2026-… 点名的那两类里的**公共**那一半：
//
//	"一类是公共类型……能够直接在插件这里看到，能够直接点击一键安装，
//	 任何账号都能直接一键安装"
//
// 判据分三层，缺一层这个功能就不成立：
//
//	1. ★★ **私有模板一个字节都不出现在这一面**（连它的名字也不）——
//	   这一面是**公开的**（没有鉴权中间件），漏一份出去等于把付费插件
//	   变成人人可装；
//	2. ★ **两个出口读的是同一份源**：公共目录发出去的那一份，与后台签发的那一份
//	   必须是同一个模板（否则"一键装的是旧那一版、后台签发的是新那一版"）；
//	3. ★ 坏模板**不进目录**（与后台那一面**相反**，理由见 plugin_public.go）。
// =========================================================================

// publicTemplate builds a template with an explicit visibility.
//
// ⚠ 它写 `x-capabilities: []`（**空数组**，不是省略）—— 那不是随手的：
// 服务端把"写了 []"读成"这份插件不需要任何能力"，而"没写"是**错误**。
// 见 plugin_template.go 的 loadOneTemplate 那一段。
func publicTemplate(id, visibility, medium string) string {
	screens := fmt.Sprintf(`{ "id": %q, "title": "一屏", "kind": "engineering", "medium": %q }`, id, medium)
	return fmt.Sprintf(`{
  "format": "aimv-plugin",
  "formatVersion": 1,
  "id": %q,
  "name": "测试插件 %s",
  "version": "1.2.3",
  "category": "mode",
  "x-capabilities": [],
  "x-visibility": %q,
  "screens": [%s]
}`, id, id, visibility, screens)
}

// publicIDs reads the catalog's item ids (order-independent).
func publicIDs(t *testing.T, env *testEnv) []string {
	t.Helper()
	rec, payload := env.getJSON("/api/zsy/plugins", "")
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())
	items, ok := dataMap(t, payload)["items"].([]any)
	require.True(t, ok, "items 不是一个数组：%v", payload)
	out := make([]string, 0, len(items))
	for _, raw := range items {
		one, ok := raw.(map[string]any)
		require.True(t, ok, "items 里有一项不是对象")
		out = append(out, fmt.Sprint(one["id"]))
	}
	return out
}

// publicManifestOf returns the plugin file text the catalog ships for one id.
func publicManifestOf(t *testing.T, env *testEnv, id string) map[string]any {
	t.Helper()
	_, payload := env.getJSON("/api/zsy/plugins", "")
	items, _ := dataMap(t, payload)["items"].([]any)
	for _, raw := range items {
		one, _ := raw.(map[string]any)
		if fmt.Sprint(one["id"]) != id {
			continue
		}
		text, ok := one["manifest"].(string)
		require.True(t, ok, "manifest 应当是**文本**（与签发那一条同源，见 plugin_public.go）")
		out := map[string]any{}
		require.NoError(t, json.Unmarshal([]byte(text), &out), "那一份文本不是合法 JSON：%s", text)
		return out
	}
	t.Fatalf("目录里没有 %q", id)
	return nil
}

func TestPublicCatalog_OnlyPublicTemplatesAppear(t *testing.T) {
	env := newTestEnv(t)
	useTemplateDir(t, map[string]string{
		"video-mode.json": publicTemplate("video-mode", VisibilityPublic, "video"),
		"audio-mode.json": publicTemplate("audio-mode", VisibilityPrivate, "audio"),
		/* ★ 没写 x-visibility 的那一份 —— 默认必须是 private */
		"text-mode.json": `{"format":"aimv-plugin","id":"text-mode","x-capabilities":[],` +
			`"screens":[{"id":"text-mode","kind":"engineering","medium":"text"}]}`,
	})

	ids := publicIDs(t, env)
	require.Contains(t, ids, "video-mode", "public 的那一份没进目录")
	require.NotContains(t, ids, "audio-mode", "★ 私有模板漏进了公开目录")
	require.NotContains(t, ids, "text-mode", "★ 没写 x-visibility 的模板被当成了公共（默认必须是 private）")
}

func TestPublicCatalog_KeepsTemplateFieldsButDropsHostKeys(t *testing.T) {
	env := newTestEnv(t)
	useTemplateDir(t, map[string]string{
		"video-mode.json": publicTemplate("video-mode", VisibilityPublic, "video"),
	})

	file := publicManifestOf(t, env, "video-mode")

	/*
	 * ⚠★ 那两个 `x-` 格**必须拿掉**（`hostOnlyKeys`，与签发那一条**同一份清单**）：
	 * 客户端承诺"不认识的字段原样保留"，留着它们会被原样存进用户本机的插件表、
	 * 再原样写回磁盘。而 `x-visibility` 更该拿掉 —— 它是**服务端的分发策略**，
	 * 对一个已经拿到文件的用户没有任何意义。
	 */
	require.NotContains(t, file, templateCapabilitiesKey, "x-capabilities 跟到了用户机器上")
	require.NotContains(t, file, templateVisibilityKey, "x-visibility 跟到了用户机器上")
	/* ★ 公共插件**没有** entitlement 块（它不是"给谁的"） */
	require.NotContains(t, file, PluginEntitlementField)

	/* 其余字段一个都不许丢（含 category 那种"客户端要读"的格） */
	require.Equal(t, "aimv-plugin", file["format"])
	require.Equal(t, "video-mode", file["id"])
	require.Equal(t, "mode", file["category"])
	require.Equal(t, "1.2.3", file["version"])
	screens, ok := file["screens"].([]any)
	require.True(t, ok, "screens 丢了：%v", file)
	require.Len(t, screens, 1)
}

func TestPublicCatalog_SkipsBrokenTemplatesEvenWhenPublic(t *testing.T) {
	env := newTestEnv(t)
	useTemplateDir(t, map[string]string{
		"good.json": publicTemplate("good", VisibilityPublic, "video"),
		/*
		 * ★ 写着 public 但**有毛病**（source 写错一个字母）—— 它不该出现在目录里：
		 * 客户端拿到它只会得到一次必然失败的安装，而用户没有任何办法修它。
		 * ⚠ 但后台那一面**照样列出来并说明**（`problem`），否则运营以为后台坏了。
		 */
		"badsource.json": `{"format":"aimv-plugin","id":"badsource","x-capabilities":[],` +
			`"x-visibility":"public",` +
			`"screens":[{"id":"badsource","kind":"catalog","source":"voiceCatalogs"}]}`,
	})

	ids := publicIDs(t, env)
	require.Contains(t, ids, "good")
	require.NotContains(t, ids, "badsource", "坏模板进了公共目录（用户点下去必然失败）")

	/* ★ 而它在后台那一面**必须在**，并且带着那句话（绝不静默） */
	rows := ReloadPluginTemplates()
	byID := map[string]*PluginTemplate{}
	for _, r := range rows {
		byID[r.ID] = r
	}
	require.Contains(t, byID, "badsource")
	require.NotEmpty(t, byID["badsource"].Problem, "坏模板没带上原因 —— 运营只能去猜")
}

func TestPublicCatalog_EmptyIsNotAnError(t *testing.T) {
	env := newTestEnv(t)
	useTemplateDir(t, map[string]string{
		"audio-mode.json": publicTemplate("audio-mode", VisibilityPrivate, "audio"),
	})

	rec, payload := env.getJSON("/api/zsy/plugins", "")
	/*
	 * ★ "站上一份公共插件都没有"是一个**完全正常的状态**（刚部署完、运营还没把
	 * 任何模板设成 public）。把它报成错误会让插件页写着"拉取失败"，
	 * 而真相是"站上确实还没有" —— 两句话让用户做的事完全不同。
	 */
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, true, payload["success"])
	require.Empty(t, dataMap(t, payload)["items"])
	/* ⚠ 目录那一格仍然要给：一个模板没出现时，运营要看得出它该放在哪儿 */
	require.NotEmpty(t, dataMap(t, payload)["directory"])
}

func TestTemplateVisibility_UnknownValueIsRefused(t *testing.T) {
	useTemplateDir(t, map[string]string{
		"p.json": `{"format":"aimv-plugin","id":"p","x-capabilities":[],"x-visibility":"publik",` +
			`"screens":[{"id":"p","kind":"engineering","medium":"video"}]}`,
	})

	rows := ReloadPluginTemplates()
	require.Len(t, rows, 1)
	/*
	 * ⚠★ 认不出的值**必须拦下来并说清**：这一格决定"任何账号能不能一键装"，
	 * 一个笔误（public → publik）会让一份付费插件变成公开可装 —— 或者反过来，
	 * 让一份本该公开的插件永远不出现。两种都不报错，所以只能在这里挡。
	 */
	require.Contains(t, rows[0].Problem, templateVisibilityKey)
	require.Contains(t, rows[0].Problem, "publik")
	/* 它也不该出现在公共目录里（Problem 非空 → IsPublic 为假） */
	require.False(t, rows[0].IsPublic())
}

func TestTemplateCapabilities_EmptyArrayIsAllowedButMissingIsNot(t *testing.T) {
	useTemplateDir(t, map[string]string{
		/* ★ 写了 [] = "这份插件不需要任何能力"（工程界面插件就是这样） */
		"none.json": `{"format":"aimv-plugin","id":"none","x-capabilities":[],` +
			`"screens":[{"id":"none","kind":"engineering","medium":"video"}]}`,
		/* ⚠ 没写 = 作者漏了（仍然拦下来：它签出去的文件上那一格会是空的，而没人会报错） */
		"missing.json": `{"format":"aimv-plugin","id":"missing",` +
			`"screens":[{"id":"missing","kind":"engineering","medium":"video"}]}`,
	})

	rows := ReloadPluginTemplates()
	byID := map[string]*PluginTemplate{}
	for _, r := range rows {
		byID[r.ID] = r
	}

	require.Empty(t, byID["none"].Problem, "空数组是合法的声明，不该被拦")
	require.Empty(t, byID["none"].Capabilities)

	require.NotEmpty(t, byID["missing"].Problem, "漏写能力那一格必须拦下来")
	require.Contains(t, byID["missing"].Problem, templateCapabilitiesKey)
}

func TestIssue_PrivateTemplateWithNoCapabilityIsAllowed(t *testing.T) {
	useTemplateDir(t, map[string]string{
		"audio-mode.json": publicTemplate("audio-mode", VisibilityPrivate, "audio"),
	})

	env := newTestEnv(t)
	/* ⚠ 后台那一面要管理员令牌（`seedAccount("admin", …)` 之后 `ensureAdminAccessToken` 才拿得到） */
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	uid := env.idOf["writer"]

	rec, payload := env.postJSON("/dashboard/zsy/world/plugins/issue",
		fmt.Sprintf(`{"user_id":%d,"plugin_id":"audio-mode"}`, uid),
		env.ensureAdminAccessToken())
	require.Equal(t, http.StatusOK, rec.Code, "body=%s", rec.Body.String())

	data := dataMap(t, payload)
	file, ok := data["file"].(string)
	require.True(t, ok, "没有回那份文件：%v", data)

	/*
	 * ★★ 「私有 + 一个能力都不要」是一种**明确的状态**：那份文件只是一份
	 * **按账号发的凭据**（"音频模式"那个界面插件一个需要能力的 op 都不调，
	 * 运营只是不想让它公开可装）。这一条原来被一句"这份文件要配至少一个能力"
	 * 挡着 —— 而那句话会让这类插件**只能**做成公共的，正是用户点名不要的形状。
	 */
	require.Contains(t, file, "audio-mode")
	require.Contains(t, file, PluginEntitlementField)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(file), &got))
	block, ok := got[PluginEntitlementField].(map[string]any)
	require.True(t, ok, "没有 entitlement 块：%v", got)
	/* 绑定信息照样要有（"这份是给谁的"是这一块的全部价值） */
	require.Equal(t, float64(uid), block["userId"])
	require.Equal(t, "writer", block["username"])
	require.NotEmpty(t, block["check"])
	require.Empty(t, block["capabilities"])
	/* ⚠ 两个 `x-` 格一个都不许跟到文件里 */
	require.NotContains(t, got, templateVisibilityKey)
	require.NotContains(t, got, templateCapabilitiesKey)
}
