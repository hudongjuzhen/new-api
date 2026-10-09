package mode

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// =========================================================================
// ★★ 「AI 一键生成模式」那条链（用户 2026-…）
//
// 用户的原话：
//
//	"然后能够 AI一键生成，选择一个API密钥，然后调用 glm-5.3-flash 这个模型生成"
//
// # ★★ 密钥从哪来：**站点上已有的令牌**，而不是"让运营再填一把上游密钥"
//
// 用户选了"后端用站内中继自己调"，所以这里**不**另存任何密钥：
//
//	弹窗里选的是**本站的一个令牌** → 后端拿它的 key → 请求本机 `/v1/chat/completions`
//	→ 中继按渠道把请求发到真正的上游（glm-5.3-flash 在哪个渠道里由运营配）
//
// 这样做的三个好处**都不是省事**：
//
//	1. **不新增一份密钥存法**：站点已经有令牌体系（额度、模型限制、分组、过期），
//	   再存一把上游密钥就是第二套权限与第二处会漏的地方；
//	2. 运营**看得见花了多少**：这一趟走的是普通的中继计费，日志里那一条就是它；
//	3. ★ 换模型 / 换渠道**不用改代码**：模型名与渠道都是运营在界面上配的。
//
// # ⚠★ 为什么走 HTTP 打自己，而不是在进程内直接调 relay 包
//
// 进程内调 relay 要构造 `RelayInfo` / 选渠道 / 走计费结算那一整套，而那些住在
// `relay/` 与 `service/` 里、**依赖宿主的一堆全局状态**；本插件（`zsy/*`）到现在
// 一行宿主内部结构都没碰过（这是它能独立装卸的全部理由）。
// 打自己那一个公开入口则把这一整摊交给**正常那条路**去处理 ——
// 代价是多一次本机上的 HTTP 往返（几百毫秒），而这是一个人点一下的动作。
//
// ⚠ 地址由 `ZSY_MODE_AI_BASE` 覆盖，默认 `http://127.0.0.1:<PORT>`：
// 部署在容器里时 `127.0.0.1` 可能不是"自己"，而运营改一个环境变量就好过改代码。
// =========================================================================

const (
	// envAIBase overrides the loopback address the generation request goes to.
	envAIBase = "ZSY_MODE_AI_BASE"
	// envAIModel overrides the model name (default `glm-5.3-flash`).
	envAIModel = "ZSY_MODE_AI_MODEL"
	// aiRequestTimeout is how long one generation may take.
	//
	// ⚠ 一份模式几万字，模型写它要几十秒 —— 超时给短了会**从中间截断**，
	// 而截断的表现是"JSON 解析失败"（看起来像模型不会写 JSON，其实是没等够）。
	aiRequestTimeout = 5 * time.Minute
)

// AIModelName answers which model generation uses.
func AIModelName() string {
	if name := strings.TrimSpace(os.Getenv(envAIModel)); name != "" {
		return name
	}
	return modeGenerationModel
}

// aiBaseURL answers the loopback address generation requests go to.
func aiBaseURL() string {
	if base := strings.TrimSpace(os.Getenv(envAIBase)); base != "" {
		return strings.TrimSuffix(base, "/")
	}
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = fmt.Sprintf("%d", *common.Port)
	}
	return "http://127.0.0.1:" + port
}

// ModeAIKey is one of the site's tokens, as the picker needs it.
//
// ⚠★ **明文 key 一格都不回**：弹窗只需要"选哪一个"，
// 而回明文意味着那一把 key 会出现在 DOM、浏览器历史、以及任何一份前端日志里。
type ModeAIKey struct {
	ID int `json:"id"`
	// Name is the token's name (what the operator recognises it by).
	Name string `json:"name"`
	// KeyPrefix is the first few characters, so two same-named tokens can be told apart.
	KeyPrefix string `json:"keyPrefix"`
	// Status mirrors the token's status (1 = enabled).
	Status int `json:"status"`
	// Usable says whether this token may call the generation model at all.
	Usable bool `json:"usable"`
	// Problem explains why it may not (empty when usable).
	Problem string `json:"problem"`
}

// ListModeAIKeys answers the site's tokens for the picker.
//
// ⚠ 只列**当前管理员自己的**令牌：这一趟请求是以他的身份走的（花的也是他的额度），
// 而列别人的令牌会让"这一趟用了谁的额度"变成一个看不出来的问题。
func ListModeAIKeys(userID int) ([]ModeAIKey, error) {
	tokens, err := model.GetAllUserTokens(userID, 0, 200)
	if err != nil {
		return nil, fmt.Errorf("mode: 读不到你的密钥列表：%w", err)
	}
	out := make([]ModeAIKey, 0, len(tokens))
	for _, token := range tokens {
		row := ModeAIKey{
			ID:        token.Id,
			Name:      token.Name,
			KeyPrefix: keyPrefixOf(token.Key),
			Status:    token.Status,
		}
		if token.Status != common.TokenStatusEnabled {
			row.Problem = "这个密钥被禁用了（在「令牌」页里把它开启）"
		} else if token.ExpiredTime > 0 && token.ExpiredTime < time.Now().Unix() {
			row.Problem = "这个密钥已经过期了"
		} else if !token.UnlimitedQuota && token.RemainQuota <= 0 {
			row.Problem = "这个密钥的额度用完了"
		}
		row.Usable = row.Problem == ""
		out = append(out, row)
	}
	return out, nil
}

// keyPrefixOf shows the first few characters of a token key.
func keyPrefixOf(key string) string {
	trimmed := strings.TrimSpace(key)
	if len(trimmed) <= 8 {
		return trimmed
	}
	return trimmed[:8] + "…"
}

// resolveTokenKey answers the plaintext key of one token **for one internal call**.
//
// ⚠★ 它**只在服务端内存里**用一次，绝不回给前端（`ModeAIKey` 里没有这一格）。
func resolveTokenKey(userID, tokenID int) (string, error) {
	if tokenID <= 0 {
		return "", fmt.Errorf("mode: 请选择一个 API 密钥（生成模式的这一趟请求要用它走站内中继）")
	}
	token, err := model.GetTokenById(tokenID)
	if err != nil {
		return "", fmt.Errorf("mode: 读不到那个密钥（id=%d）：%w", tokenID, err)
	}
	if token.UserId != userID {
		/*
		 * ⚠ 别人的令牌**不许用**：这一趟会以那个令牌的身份与额度去请求，
		 * 用它等于"拿别人的额度给这个模式买单"，而且日志里看起来完全正常。
		 */
		return "", fmt.Errorf("mode: 那个密钥不是你的（只能用自己账号下的密钥）")
	}
	if token.Status != common.TokenStatusEnabled {
		return "", fmt.Errorf("mode: 那个密钥被禁用了，请换一个（或去「令牌」页把它开启）")
	}
	if token.ExpiredTime > 0 && token.ExpiredTime < time.Now().Unix() {
		return "", fmt.Errorf("mode: 那个密钥已经过期了，请换一个")
	}
	return token.Key, nil
}

// chatMessage is one message of the generation request.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatCompletionRequest is the (minimal) body we send to our own relay.
//
// ⚠ 只写用得上的那几格：`response_format` 交给模型自己遵守（有些上游不认它，
// 带上它反而会让请求被拒）—— 提示词里已经把"只输出 JSON"写死了。
type chatCompletionRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens"`
}

// chatCompletionResponse is the part of the answer we read.
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// callGenerationModel sends one prompt through this site's own relay.
//
// ⚠★ 出错时**说清是哪一段坏的**（密钥不行 / 模型没有渠道 / 上游拒绝 / 超时）：
// 这四种的下一步动作完全不同，而它们在界面上都只是"生成失败"。
func callGenerationModel(tokenKey, modelName, prompt string) (string, error) {
	body, err := common.Marshal(chatCompletionRequest{
		Model: modelName,
		Messages: []chatMessage{
			{Role: "system", Content: "你是一个只输出 JSON 的助手。除了那一份 JSON，不要输出任何解释。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens: modeGenerationMaxTokens,
	})
	if err != nil {
		return "", fmt.Errorf("mode: 拼请求失败：%w", err)
	}

	url := aiBaseURL() + "/v1/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("mode: 拼请求失败：%w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tokenKey)

	client := &http.Client{Timeout: aiRequestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mode: 连不上站内中继（%s）：%w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return "", fmt.Errorf("mode: 读中继的回复失败：%w", err)
	}

	var parsed chatCompletionResponse
	if err := common.Unmarshal(raw, &parsed); err != nil {
		/* ⚠ 不是 JSON：多半是中继自己回的一句错误（网关、鉴权中间件），原样带出去 */
		return "", fmt.Errorf("mode: 中继回的不是 JSON（HTTP %d）：%s", resp.StatusCode, truncate(string(raw), 400))
	}
	if parsed.Error != nil && strings.TrimSpace(parsed.Error.Message) != "" {
		return "", fmt.Errorf("mode: 中继拒绝了这一趟：%s", strings.TrimSpace(parsed.Error.Message))
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mode: 中继回了 HTTP %d：%s", resp.StatusCode, truncate(string(raw), 400))
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("mode: 中继回了一个空的 choices（模型没给出内容）")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("mode: 模型回了一句空话（内容为空）—— 请再试一次，或者换一个密钥")
	}
	return content, nil
}

// extractModeJSON pulls the first complete JSON object out of a model answer.
//
// ⚠★ **不抛异常**：模型偶尔会包一层 ```json 代码块，或者前后带两句客套话。
// 那不算失败 —— 这里尽力取出第一个完整的 `{ … }`，取不到时回一句**能照做的话**
// （与 `modeShapeProblem` 同一条纪律：每一种失败都要是一句人话）。
func extractModeJSON(answer string) (map[string]any, error) {
	text := strings.TrimSpace(answer)
	/* 去掉 ```json / ``` 围栏（模型最常见的包装） */
	if strings.HasPrefix(text, "```") {
		text = strings.TrimPrefix(text, "```")
		text = strings.TrimPrefix(text, "json")
		if idx := strings.LastIndex(text, "```"); idx >= 0 {
			text = text[:idx]
		}
		text = strings.TrimSpace(text)
	}

	start := strings.Index(text, "{")
	if start < 0 {
		return nil, fmt.Errorf("mode: 模型这次没有给出 JSON（它回的是：%s）—— 请再试一次", truncate(text, 200))
	}
	/*
	 * ⚠ 从第一个 `{` 起找一个**配平**的结尾：模型常在 JSON 后面再接一段解释，
	 * 直接 `Unmarshal` 整段会因为尾部那些字失败（而那一段 JSON 其实是好的）。
	 * 配平要认字符串里的花括号（`"words": {"a": "{}"}` 这种会骗过朴素计数）。
	 */
	end := balancedJSONEnd(text, start)
	if end < 0 {
		return nil, fmt.Errorf("mode: 模型给的 JSON 没有闭合（可能被截断了）—— 请再试一次，"+
			"或者换一个输出更长的模型。它回的长度是 %d 字。", len(text))
	}

	var out map[string]any
	if err := common.UnmarshalJsonStr(text[start:end], &out); err != nil {
		return nil, fmt.Errorf("mode: 模型给的 JSON 解析不了：%v", err)
	}
	return out, nil
}

// balancedJSONEnd answers the index just past the object that starts at `start`.
//
// ⚠ 认字符串与转义：`"他说：\"{\""` 里的花括号**不算**括号 —— 朴素计数会在这种地方
// 提前收尾，切出一段解析不了的半份 JSON。
func balancedJSONEnd(text string, start int) int {
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(text); i++ {
		ch := text[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case ch == '\\':
				escaped = true
			case ch == '"':
				inString = false
			}
			continue
		}
		switch ch {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i + 1
			}
		}
	}
	return -1
}

// ModeGenerateInput is what the create dialog sends.
type ModeGenerateInput struct {
	Label   string
	Medium  string
	Summary string
	// Request is the operator's prose ("我要做美食探店类的短视频").
	Request string
	// TokenID is which of the site's tokens pays for this call.
	TokenID int
	// Visibility / Summary are the two `x-` keys written into the new file.
	Visibility string
	ID         string
}

// GenerateMode creates one mode by asking the model to write it.
//
// ★★ 这一整条链的次序**不能换**（每一步的失败都要在下一步之前挡住）：
//
//	① 校验那三个输入（名称 / 类型 / 简介 —— 有的话模型连"给谁写"都不知道）
//	② 定 id（★ **我们给**，不让模型造：它是文件名）
//	③ 问模型（走站内中继，用运营选的那个密钥）
//	④ 从回包里取出 JSON（去围栏、配平、解析）
//	⑤ **覆盖身份那几格**（id / label / medium / x-*）—— 模型写错也不认，以运营填的为准
//	⑥ 跑一遍与读盘同源的校验（`modeShapeProblem`）
//	⑦ 写盘（`WriteModeContent` 那条路：原子替换 + 留 `.bak`）
//
// ⚠★ 第 ⑤ 步是刻意的：模型很爱"顺手改一下名字"，而 id 与文件名不一致的那一份
// **客户端会拒收**（判据在 `modeShapeProblem` 里）。运营填的那三个输入才是权威。
func GenerateMode(userID int, in ModeGenerateInput) (*ModeFile, string, error) {
	label := strings.TrimSpace(in.Label)
	if label == "" {
		return nil, "", fmt.Errorf("mode: 请填模式名称（它会成为广场卡片上那一行字）")
	}
	medium := strings.TrimSpace(in.Medium)
	if !isKnownMedium(medium) {
		return nil, "", fmt.Errorf("mode: 模式类型只能是 %s（收到的是 %q）—— "+
			"它决定这一档出现在哪一家。", strings.Join(knownMediums, " / "), medium)
	}
	visibility := strings.TrimSpace(in.Visibility)
	if visibility == "" {
		visibility = VisibilityPrivate
	}
	if visibility != VisibilityPublic && visibility != VisibilityPrivate {
		return nil, "", fmt.Errorf("mode: 可见性只能写 %s / %s（收到的是 %q）",
			VisibilityPublic, VisibilityPrivate, visibility)
	}

	id := strings.TrimSpace(in.ID)
	if id == "" {
		var err error
		id, err = suggestModeID(label)
		if err != nil {
			return nil, "", err
		}
	}
	if !isValidModeID(id) {
		return nil, "", fmt.Errorf("mode: %q 不是一个能用的模式 id —— "+
			"它要用小写字母开头，只能是字母、数字、下划线、短横线，最长 32 个字符（例如 mv、food-shop）。", id)
	}
	if _, exists := ModeByID(id); exists {
		return nil, "", fmt.Errorf("mode: 已经有一档叫 %q 了（文件名就是 id）。"+
			"换一个名字，或者先把它删掉/改名。", id)
	}

	tokenKey, err := resolveTokenKey(userID, in.TokenID)
	if err != nil {
		return nil, "", err
	}

	modelName := AIModelName()
	prompt := buildModeCreateInstruction(in.Request, label, medium, in.Summary, id)

	answer, err := callGenerationModel(tokenKey, modelName, prompt)
	if err != nil {
		return nil, "", err
	}
	generated, err := extractModeJSON(answer)
	if err != nil {
		return nil, "", err
	}

	/* ⑤ 身份那几格以**运营填的**为准（模型写错也不认） */
	generated["format"] = modeFormat
	generated["formatVersion"] = modeFormatVersion
	generated["id"] = id
	generated["label"] = label
	generated["medium"] = medium
	if summary := strings.TrimSpace(in.Summary); summary != "" {
		generated[summaryKey] = summary
	}
	generated[visibilityKey] = visibility

	/* ⑥ 与读盘同源的校验（保存前那一道闸） */
	if problem := modeShapeProblem(generated, id); problem != "" {
		return nil, "", fmt.Errorf("mode: 模型生成的这一份不合法：%s —— "+
			"请把需求写得更具体一点再试一次。", problem)
	}

	/* ⑦ 落盘 */
	if _, err := writeNewMode(id, generated); err != nil {
		return nil, "", err
	}

	row, found := ModeByID(id)
	if !found {
		return nil, "", fmt.Errorf("mode: 写完之后反而读不到 %q 了 —— 文件可能被别的进程动过", id)
	}
	return row, prompt, nil
}

// writeNewMode writes one **new** mode file.
//
// ⚠★ 它与 `WriteModeContent` 有两点不同：
//
//	① 文件**必须还不存在**（覆盖一档已有的模式是另一件事：那要走编辑弹窗，
//	   而那里的判据是"运营看着它改的"）；
//	② 没有 `.bak` 可留（它是新的）。
func writeNewMode(id string, content map[string]any) (string, error) {
	/*
	 * ⚠★ 这里用的是 `resolveModeDir()` 而**不是** `ModeDir()` —— 后者在第一次调用之后
	 * 会缓存（那是给列目录用的），而这一条路要的是**磁盘上现在那个目录**。
	 *
	 * 这一条是**实测**改的：先写"目录存在不存在"那一步用的是 `ModeDir()`
	 * （缓存的），而下面 `ModeByID` 用的是 `resolveModeDir()`（现读环境变量）——
	 * 两者不一致时的表现是"明明已经有一档叫这个名，却照样写下去了"，
	 * 而那正是"悄悄覆盖别人那一档"。判据只留一处：现读。
	 */
	dir := resolveModeDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("mode: 建模式目录 %s 失败：%w", dir, err)
	}
	path := filepath.Join(dir, id+".json")
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("mode: %s 已经在了（文件名就是 id）—— 请换一个名称", path)
	}

	rendered, err := renderOrderedMode(content, "")
	if err != nil {
		return "", err
	}
	if len(rendered) > maxModeContentBytes {
		return "", fmt.Errorf("mode: 生成出来的那一份太大了（%d 字节，上限 %d）", len(rendered), maxModeContentBytes)
	}

	/*
	 * ★ 先写临时文件再 `rename`（原子替换）：中途失败时那个位置要么没有文件、
	 * 要么是一份完整的模式 —— **不会**留下半份 JSON（客户端读它只会说"这一档坏了"）。
	 */
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(rendered), 0o600); err != nil {
		return "", fmt.Errorf("mode: 写临时文件失败：%w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("mode: 建 %s 失败：%w", path, err)
	}
	/* ⚠ 写完**重读一遍**：让缓存与新文件一致（下一句 `ModeByID` 就会读到它） */
	LoadModes()
	return rendered, nil
}

// suggestModeID derives a mode id from a name.
//
// ⚠★ 中文名推不出有意义的 slug（"美食探店" → 空），那种时候回落到 `mode-<时间戳>`：
// 一个**能用**的 id 好过让运营去手写一个（而 id 只是文件名，改名很便宜）。
func suggestModeID(label string) (string, error) {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(strings.TrimSpace(label)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 32 {
		slug = strings.Trim(slug[:32], "-")
	}
	if slug == "" || !isValidModeID(slug) {
		return fmt.Sprintf("mode-%d", time.Now().Unix()), nil
	}
	if _, exists := ModeByID(slug); !exists {
		return slug, nil
	}
	/* ⚠ 撞名就加一个后缀（不许悄悄覆盖别人那一档） */
	for i := 2; i < 100; i++ {
		candidate := fmt.Sprintf("%s-%d", slug, i)
		if !isValidModeID(candidate) {
			continue
		}
		if _, exists := ModeByID(candidate); !exists {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("mode: 从 %q 推不出一个没被占用的 id，请直接填一个", label)
}

// truncate shortens a string for an error message.
func truncate(text string, max int) string {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) <= max {
		return trimmed
	}
	return trimmed[:max] + "…"
}
