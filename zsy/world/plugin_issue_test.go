package world

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// =========================================================================
// ★★ 后台按账号签发插件文件（docs/22 §2.3 / docs/23 §12.15）
//
// 这一组守的是**用户在导入那一刻看到的那句话**背后的东西：
//
//	把给 A 的文件发给了 B  →  B 导入时被告知"这是给 zsy（ID 7）的"
//
// 所以判据分三层，缺一层这个功能就不成立：
//
//	1. **模板怎么读**：文件名 = 插件 id、`x-capabilities` 必须写、写错了要说清哪一句；
//	2. ★ **签出来的文件里那个 `check` 必须与客户端算得出同一个值** ——
//	   算错了症状是"每一份文件都报被改过"，而那看起来像文件坏了；
//	3. ★★ **签发 ≠ 授权**：没被授予能力的账号照样拿得到一份文件（后台可以签），
//	   而那份文件对他一点用都没有（op 会回 E_ENTITLEMENT）。
//
// 第 3 条是这一组里最容易写错的一条：把"签发了"当成"给了"，会让运营以为
// 点一下"生成文件"就等于开通 —— 而用户导入之后什么都用不了。
// =========================================================================

// writeTemplate drops one template file into dir and returns its path.
func writeTemplate(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600), "write %s", path)
	return path
}

// useTemplateDir points the plugin at a temp directory of templates for this test.
//
// ⚠ 两个动作都要做：设环境变量（`resolveTemplateDir` 每次现读）**并且**重新加载
// （缓存是模块级的，上一个用例的目录还在里面）。少做第二个，症状是"用例串味"。
func useTemplateDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		writeTemplate(t, dir, name, body)
	}
	t.Setenv(pluginTemplateDirEnv, dir)
	t.Cleanup(func() {
		/* 用完把缓存清掉，免得下一组用例读到这个临时目录 */
		LoadPluginTemplates()
	})
	return dir
}

// aValidTemplate is a minimal, valid template (its content does not matter for
// most assertions — what matters is the shape and `x-capabilities`).
func aValidTemplate(id string, capabilities ...string) string {
	caps := ""
	for i, c := range capabilities {
		if i > 0 {
			caps += ", "
		}
		caps += fmt.Sprintf("%q", c)
	}
	return fmt.Sprintf(`{
  "format": "aimv-plugin",
  "formatVersion": 1,
  "id": %q,
  "name": "测试插件 %s",
  "x-capabilities": [%s],
  "screens": [
    { "id": %q, "title": "一屏", "kind": "world", "source": "worldOps",
      "card": { "title": "name" } }
  ]
}`, id, id, caps, id)
}

// ---------------------------------------------------------------------------
// 一、模板怎么读
// ---------------------------------------------------------------------------

func TestPluginTemplates_LoadAndReportProblems(t *testing.T) {
	useTemplateDir(t, map[string]string{
		"good.json": aValidTemplate("good", CapabilityWorldIP),
		/* 文件名说 a，里面的 id 说 b —— 这是最容易犯的那种错 */
		"mismatch.json": `{"format":"aimv-plugin","id":"other","x-capabilities":["world-ip"],` +
			`"screens":[{"id":"s","kind":"catalog","source":"voiceCatalog"}]}`,
		/* 少了 x-capabilities */
		"nocaps.json": `{"format":"aimv-plugin","id":"nocaps",` +
			`"screens":[{"id":"s","kind":"catalog","source":"voiceCatalog"}]}`,
		/* 不认识的数据源（写错一个字母） */
		"badsource.json": `{"format":"aimv-plugin","id":"badsource","x-capabilities":["world-ip"],` +
			`"screens":[{"id":"s","kind":"catalog","source":"voiceCatalogs"}]}`,
		/* 不是 JSON */
		"broken.json": `{ 这不是 JSON`,
	})

	rows := ReloadPluginTemplates()
	byID := map[string]*PluginTemplate{}
	for _, r := range rows {
		byID[r.ID] = r
	}

	require.Contains(t, byID, "good", "好模板没被读进来")
	require.Empty(t, byID["good"].Problem)
	require.Equal(t, []string{CapabilityWorldIP}, byID["good"].Capabilities)

	/*
	 * ★ 下面四条是**同一件事**：坏模板要**出现在列表里**并把原因说清楚。
	 * 不出现的话，运营看到的是"我放进去的文件不见了" —— 那看起来像后台坏了。
	 */
	for _, tc := range []struct {
		id      string
		want    string
		because string
	}{
		{"mismatch", "不一致", "文件名与 id 不一致要说清两个值"},
		{"nocaps", templateCapabilitiesKey, "少了能力那一格要点名它"},
		{"badsource", "source", "数据源写错要指出是哪一屏哪一格"},
		{"broken", "JSON", "不是 JSON 要这么说"},
	} {
		row := byID[tc.id]
		require.NotNil(t, row, "%s 应当被列出来（绝不静默）", tc.id)
		require.NotEmpty(t, row.Problem, "%s 应当有 problem", tc.id)
		require.Contains(t, row.Problem, tc.want, tc.because)
	}
}

func TestPluginTemplates_MissingDirectoryIsNotAnError(t *testing.T) {
	/*
	 * 还没配过模板的部署**照样要能打开那一屏** —— 它会显示"目录里还没有模板"，
	 * 并给出目录路径（运营据此知道该往哪儿放）。把"目录不存在"当错误会让
	 * 一个正常的初始状态看起来像故障。
	 */
	dir := filepath.Join(t.TempDir(), "not-created-yet")
	t.Setenv(pluginTemplateDirEnv, dir)
	t.Cleanup(func() { LoadPluginTemplates() })

	require.Empty(t, ReloadPluginTemplates())
	require.Equal(t, dir, PluginTemplateDir())
}

// ---------------------------------------------------------------------------
// 二、★ 签出来的文件：那个 check 必须与客户端算得出同一个值
// ---------------------------------------------------------------------------

func TestIssuePluginFile_WritesABindingTheClientCanVerify(t *testing.T) {
	useTemplateDir(t, map[string]string{"world-ip.json": aValidTemplate("world-ip", CapabilityWorldIP, CapabilityWorldIPAI)})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	writerID := env.idOf["writer"]

	body := `{"user_id":` + fmt.Sprint(writerID) + `,"plugin_id":"world-ip"}`
	rec, payload := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, true, payload["success"], rec.Body.String())

	data := dataMap(t, payload)
	fileText, _ := data["file"].(string)
	require.NotEmpty(t, fileText, "签发没给出文件内容")

	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(fileText), &out), "签出来的不是合法 JSON")

	block, ok := out[PluginEntitlementField].(map[string]any)
	require.True(t, ok, "文件里没有 %s 那一块：%s", PluginEntitlementField, fileText)

	require.Equal(t, float64(writerID), block["userId"])
	require.Equal(t, "writer", block["username"])
	/* 站点来自请求（httptest 给的是 example.com）—— 不是写死的域名 */
	require.Equal(t, "http://example.com", block["site"])
	require.Equal(t, []any{CapabilityWorldIP, CapabilityWorldIPAI}, block["capabilities"])
	require.NotEmpty(t, block["issuedAt"])

	/*
	 * ★★ 最后一条是最要紧的：**用同一个公式重算一遍**。
	 *
	 * 这正是客户端会做的事（`computeCheck`），而它算不出来的话，
	 * 用户导入时看到的是"这份文件被改过" —— 一份完全正常的文件被判成坏的，
	 * 而运营完全无从理解。所以这里按**规范文本**重算，而不是抄一遍 response 里的值。
	 */
	require.Equal(t,
		PluginCheck("world-ip", writerID, "writer", "http://example.com"),
		block["check"],
		"文件里的 check 与重算出来的不一致 —— 客户端会判「被改过」")

	/* 模板专用那一格**不许**出现在签发的文件里（它不属于插件格式） */
	require.NotContains(t, out, templateCapabilitiesKey,
		"x-capabilities 是后台用的，不该跟着文件走到用户的机器上")

	/* 文件名要能区分账号 —— 运营硬盘上会躺着好几份 */
	name, _ := data["fileName"].(string)
	require.Contains(t, name, "world-ip")
	require.Contains(t, name, "writer")
	require.Contains(t, name, fmt.Sprint(writerID))
}

// TestIssuePluginFile_KeepsEveryTemplateField pins "读一遍再写回去不丢字段".
//
// ⚠ 少一格的表现不是报错，是"我发给他的那份能用，刚生成的这份不行" ——
// 而两边的差别在几百行 JSON 里看不出来。
func TestIssuePluginFile_KeepsEveryTemplateField(t *testing.T) {
	useTemplateDir(t, map[string]string{
		"rich.json": `{
  "format": "aimv-plugin", "formatVersion": 1, "id": "rich",
  "name": "字段多的一份", "version": "9.9.9", "author": "某人",
  "description": "带描述",
  "x-capabilities": ["world-ip"],
  "未来才有的顶层格子": { "随便": [1, 2, 3] },
  "screens": [
    { "id": "rich", "title": "一屏", "subtitle": "副标题",
      "kind": "world", "source": "worldOps",
      "card": { "title": "name", "subtitle": "typeName" },
      "未来才有的屏格子": true }
  ]
}`,
	})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	body := `{"user_id":` + fmt.Sprint(env.idOf["writer"]) + `,"plugin_id":"rich"}`
	rec, payload := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, true, payload["success"], rec.Body.String())

	fileText, _ := dataMap(t, payload)["file"].(string)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(fileText), &out))

	for _, key := range []string{
		"format", "formatVersion", "id", "name", "version", "author", "description",
		"未来才有的顶层格子",
	} {
		require.Contains(t, out, key, "签发的文件里丢了顶层字段 %q", key)
	}
	screens, _ := out["screens"].([]any)
	require.Len(t, screens, 1)
	first, _ := screens[0].(map[string]any)
	require.Contains(t, first, "未来才有的屏格子", "屏上的额外字段丢了")
	require.Contains(t, first, "subtitle")
}

// TestIssuePluginFile_TheClientWouldAcceptIt 是这一组**最要紧**的一条：
// 把真实签出来的文件交给**客户端的实现**，问它"这份文件给 zsy 对不对"。
//
// # 为什么不能只对 Go 自己重算一遍
//
// 那个只能证明"Go 与 Go 一致"。而这一整块是**跨语言**的：服务端签发、客户端校验，
// 所以真正要回答的是"**那个 JS 函数**会不会说 ok"。两边分叉的症状是
// **每一份文件都在导入时报「被改过」** —— 极难查，而且看起来像文件坏了。
//
// 所以这条用例通过 `node` 跑一遍 `src/core/plugin-entitlement.js` 的
// `checkEntitlementFor`（不是一个重写的小脚本：那又会变成第三处实现）。
//
// ⚠ 两个仓库的位置是**每台机器自己的事**，所以按环境变量与几个已知位置找；
// 找不到就**跳过并说清怎么让它可达** —— 而"接线本身有效"由上面那条
// `TestIssuePluginFile_WritesABindingTheClientCanVerify` 无条件钉着。
func TestIssuePluginFile_TheClientWouldAcceptIt(t *testing.T) {
	clientModuleURL := findClientEntitlementModuleURL(t)
	if clientModuleURL == "" {
		t.Skipf("找不到客户端的 plugin-entitlement.js。把 %s 指向那个文件（在 aimv-studio 仓库的 src/core/ 下）后重跑。",
			clientModuleEnv)
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skipf("找不到 node：%v", err)
	}

	useTemplateDir(t, map[string]string{"world-ip.json": aValidTemplate("world-ip", CapabilityWorldIP)})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	writerID := env.idOf["writer"]

	body := fmt.Sprintf(`{"user_id":%d,"plugin_id":"world-ip"}`, writerID)
	_, payload := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, true, payload["success"], "签发失败：%v", payload)

	fileText, _ := dataMap(t, payload)["file"].(string)
	require.NotEmpty(t, fileText)

	/*
	 * 两次问它：同一个账号（应当 ok）、另一个账号（应当说"账号不匹配"）。
	 * ⚠ 第二个问题与前一个同样重要：**判据要能两边都答对**，只答"ok"的实现在
	 * 换个人时也会说 ok（那样这一整块就是装饰）。
	 */
	ask := func(t *testing.T, accountJSON string) map[string]any {
		t.Helper()

		/*
		 * ⚠ 写一个**真的临时模块**，而不是 `node --input-type=module -e '…'`。
		 *
		 * 后者与"通过 `import()` 载入一个 file:// 模块"合起来在 node 上会打架
		 * （第一版就是这么写的，报出来的是"跑客户端实现失败："后面**什么都没有** ——
		 * 因为 `cmd.Output()` 把 stderr 丢了，而失败信息恰好全在 stderr 里）。
		 *
		 * 现在的形状：临时文件里 `import()` 那个模块 → 跑一次 → 打一行 JSON。
		 * 失败时用 `CombinedOutput` 把 node 自己的话原样带出来。
		 */
		dir := t.TempDir()
		scriptPath := filepath.Join(dir, "ask.mjs")
		/*
		 * ⚠ `process.argv` 的**前两格是 node 与脚本路径本身**，所以两个入参在
		 * `[2]` 与 `[3]`（第一版多塞了一个占位参数，于是脚本去解 `"x"`，
		 * 报出来是 `Unexpected token 'x'`）。
		 */
		script := fmt.Sprintf(`
const { checkEntitlementFor } = await import(%s);
const manifest = JSON.parse(process.argv[2]);
const account = JSON.parse(process.argv[3]);
process.stdout.write(JSON.stringify(checkEntitlementFor(manifest, account)));
`, mustJSONString(t, clientModuleURL))
		require.NoError(t, os.WriteFile(scriptPath, []byte(script), 0o600))

		out, err := exec.Command("node", scriptPath, fileText, accountJSON).CombinedOutput()
		require.NoError(t, err, "跑客户端实现失败：\n%s", string(out))
		var got map[string]any
		require.NoError(t, json.Unmarshal(out, &got), "客户端没答出 JSON：%s", string(out))
		return got
	}

	mine := fmt.Sprintf(`{"signedIn":true,"needsLogin":false,"userId":%d,"username":"writer","base":"http://example.com"}`, writerID)
	ok := ask(t, mine)
	require.Equal(t, true, ok["ok"],
		"★ 客户端**拒绝**了服务端刚签出来的文件（kind=%v who=%v error=%v）—— 两边的哈希公式分叉了",
		ok["kind"], ok["who"], ok["error"])

	other := `{"signedIn":true,"needsLogin":false,"userId":999999,"username":"someone-else","base":"http://example.com"}`
	bad := ask(t, other)
	require.Equal(t, false, bad["ok"],
		"★ 客户端**接受**了给别人的文件 —— 那这一整块绑定就是装饰：%v", bad)
	require.Equal(t, "wrong-user", bad["kind"])
	require.Contains(t, fmt.Sprint(bad["error"]), "writer", "那句提示要说清文件是给谁的")
	require.Contains(t, fmt.Sprint(bad["error"]), "someone-else", "也要说清你现在是谁")
}

// clientModuleEnv points at the client's plugin-entitlement.js explicitly.
const clientModuleEnv = "ZSY_WORLD_CLIENT_ENTITLEMENT"

// findClientEntitlementModuleURL locates the client repo's module and answers a
// **`file://` URL**, or "".
//
// ⚠★ 必须是 URL，不能是路径：Node 的 ESM 载入器在 Windows 上**不接受** `D:\…`
// 这种绝对路径（报 `ERR_UNSUPPORTED_ESM_URL_SCHEME`，理由是 "Received protocol 'd:'"）。
// 第一版直接给路径，而 `cmd.Output()` 把 stderr 丢了，于是报出来的是
// "跑客户端实现失败：" 后面**什么都没有** —— 查它花的时间比这段代码长。
//
// 中文路径也要能过：`url.URL` 会把非 ASCII 百分号编码，正是 ESM 要的形状。
func findClientEntitlementModuleURL(t *testing.T) string {
	t.Helper()

	candidates := []string{}
	if p := strings.TrimSpace(os.Getenv(clientModuleEnv)); p != "" {
		candidates = append(candidates, p)
	}
	/*
	 * 两个仓库的常见相对位置。⚠ 这一组只是**尽量找到** —— 找不到就跳过，
	 * 因为"客户端仓库在这个开发机上"不是一个可以假定的前提（CI 里就没有）。
	 */
	for _, root := range []string{
		"D:/MV项目/aimv-studio",
		"../aimv-studio",
		"../../aimv-studio",
	} {
		candidates = append(candidates, filepath.Join(root, "src", "core", "plugin-entitlement.js"))
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		return (&url.URL{Scheme: "file", Path: filepath.ToSlash(abs)}).String()
	}
	return ""
}
func TestIssuePluginFile_CapabilityOverride(t *testing.T) {
	useTemplateDir(t, map[string]string{"p.json": aValidTemplate("p", CapabilityWorldIP, CapabilityWorldIPAI)})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	uid := env.idOf["writer"]

	body := fmt.Sprintf(`{"user_id":%d,"plugin_id":"p","capabilities":["%s"]}`, uid, CapabilityWorldIP)
	rec, payload := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, true, payload["success"], rec.Body.String())

	out := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(dataMap(t, payload)["file"].(string)), &out))
	block := out[PluginEntitlementField].(map[string]any)
	require.Equal(t, []any{CapabilityWorldIP}, block["capabilities"])

	/* 而 check 是按实际写进去的那一份算的 —— 不是按模板算的 */
	require.Equal(t, PluginCheck("p", uid, "writer", "http://example.com"), block["check"])
}

// TestIssuePluginFile_RefusesUnknownCapability 不认识的能力**不许**写进文件。
//
// ⚠ 客户端只拿 `capabilities` 显示、不会因此拒绝，所以写错一个字母会**静默地**
// 跟着文件走到底 —— 用户看到的是一份"配了三个能力"的文件，而其中一个不存在。
func TestIssuePluginFile_RefusesUnknownCapability(t *testing.T) {
	useTemplateDir(t, map[string]string{"p.json": aValidTemplate("p", CapabilityWorldIP)})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	body := fmt.Sprintf(`{"user_id":%d,"plugin_id":"p","capabilities":["world-ip-tpyo"]}`, env.idOf["writer"])
	_, payload := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, false, payload["success"], "打错一个能力名却被接受了：%v", payload)
	require.Contains(t, fmt.Sprint(payload["message"]), "world-ip-tpyo")
}

// TestIssuePluginFile_RefusesUnknownPlugin 没有那份模板时要说清有哪些。
func TestIssuePluginFile_RefusesUnknownPlugin(t *testing.T) {
	useTemplateDir(t, map[string]string{"good.json": aValidTemplate("good", CapabilityWorldIP)})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	body := fmt.Sprintf(`{"user_id":%d,"plugin_id":"nope"}`, env.idOf["writer"])
	_, payload := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, false, payload["success"])
	msg := fmt.Sprint(payload["message"])
	require.Contains(t, msg, "nope")
	require.Contains(t, msg, "good", "要说清现有的是哪几份，否则运营只能去猜")
}

// ---------------------------------------------------------------------------
// 三、★★ 签发 ≠ 授权
// ---------------------------------------------------------------------------

// TestIssuePluginFile_DoesNotGrantAnything 是这一组里最重要的一条。
//
// ★ 用户可能以为"点一下生成文件"就等于开通了。**不是**：签发只产出一个文件，
// 而能力在 `zsy_world_entitlements` 里。所以：
//
//	· 一个**没有**能力的账号照样拿得到一份文件 —— 那是**对的**（后台可以先签、再授予）；
//	· 而那份文件对他**一点用都没有** —— 真正的 op 会回 E_ENTITLEMENT；
//	· 返回值里 `granted` 如实说明"现在生效的有哪些"，运营据此知道还差一步。
//
// 反过来（签发就顺手授予）会让"点了生成"变成一次不可见的授权动作，
// 而撤销与审计都无从下手。
func TestIssuePluginFile_DoesNotGrantAnything(t *testing.T) {
	useTemplateDir(t, map[string]string{"world-ip.json": aValidTemplate("world-ip", CapabilityWorldIP, CapabilityWorldIPAI)})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)
	writerID := env.idOf["writer"]
	env.mustSeedWorld("writer", emptyWorldDoc)

	/* 故意**不**授予任何能力 */
	body := fmt.Sprintf(`{"user_id":%d,"plugin_id":"world-ip"}`, writerID)
	rec, payload := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Equal(t, true, payload["success"], "签发本身不该依赖能力：%v", payload)

	data := dataMap(t, payload)
	require.Empty(t, data["granted"], "还没授予，granted 不该有东西：%v", data["granted"])

	/* ① 数据库里**没有**多出任何授权 */
	live, err := WorldCapabilitiesActive(writerID)
	require.NoError(t, err)
	require.Empty(t, live, "★ 签发顺手授予了能力 —— 那会让「生成文件」变成一次不可见的授权")

	/* ② 而那个账号**照样用不了**：op 会回 E_ENTITLEMENT（判据 ① 的原话） */
	opRec, opPayload := env.callOp("writer", `{"op":"project.create","params":{"name":"x"}}`)
	require.Equal(t, http.StatusForbidden, opRec.Code)
	require.Equal(t, CodeEntitlement, opPayload["code"],
		"签发了文件却仍然要能力才能用 —— 这条不对的话，用户会以为导入文件就等于开通")

	/* ③ 授予之后同一个 op 就通了（而且 granted 也随之变化） */
	env.mustGrant(adminUsername, writerID, CapabilityWorldIP, 0)
	_, again := env.postJSON("/dashboard/zsy/world/plugins/issue", body, env.ensureAdminAccessToken())
	require.Equal(t, []any{CapabilityWorldIP}, dataMap(t, again)["granted"],
		"授予之后 granted 应当只列出**真的生效**的那一个（world-ip-ai 还没给）")
}

// ---------------------------------------------------------------------------
// 四、列表接口与路由
// ---------------------------------------------------------------------------

func TestListPluginTemplates_AdminOnly(t *testing.T) {
	useTemplateDir(t, map[string]string{"good.json": aValidTemplate("good", CapabilityWorldIP)})

	env := newTestEnv(t)
	env.seedAccount(adminUsername, common.RoleAdminUser)
	env.seedAccount("writer", common.RoleCommonUser)

	/* 没带令牌 → 拒绝（后台面吃的是访问令牌，不是账号密钥） */
	rec, _ := env.getJSON("/dashboard/zsy/world/plugins", "")
	require.NotEqual(t, http.StatusOK, rec.Code, "后台面不许匿名访问")

	/* 普通用户的**账号密钥**也不行 */
	rec, _ = env.getJSON("/dashboard/zsy/world/plugins", env.keyOf["writer"])
	require.NotEqual(t, http.StatusOK, rec.Code, "普通用户不该看到后台面")

	rec, payload := env.getJSON("/dashboard/zsy/world/plugins", env.ensureAdminAccessToken())
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	data := dataMap(t, payload)
	items, ok := data["items"].([]any)
	require.True(t, ok, "items 应当是数组：%v", data)
	require.Len(t, items, 1)

	first, _ := items[0].(map[string]any)
	require.Equal(t, "good", first["id"])
	require.Equal(t, []any{CapabilityWorldIP}, first["capabilities"])
	require.Equal(t, []any{"一屏"}, first["screens"])
	/* 目录与两个白名单也要回给界面：写模板时要用 */
	require.NotEmpty(t, data["directory"])
	require.NotEmpty(t, data["knownKinds"])
	require.NotEmpty(t, data["knownSources"])
}
