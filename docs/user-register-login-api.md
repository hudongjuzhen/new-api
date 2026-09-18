# 用户注册 / 登录开放接口（第三方应用接入）

> 适用版本：当前 `zsy/runninghub` 分支源码树
> 结论：**项目自带可用的注册与登录 HTTP 接口**（纯 JSON，无需浏览器），第三方应用可直接调用；但**核心注册接口默认不创建任何 API Key**，也无法指定 Key 的名称与分组。
> 因此有两种用法：**方案 A** 直接组合核心接口（零改动，见 §6）；**方案 B** 使用 `zsy/appauth` 插件（已实现，注册即建 Key「默认密钥 / 官方渠道」并返回明文，见 §7）。

---

## 1. 结论与现状对照

| 能力 | 现状 | 说明 |
| --- | --- | --- |
| 第三方应用注册用户 | ✅ 已有 | `POST /api/user/register` |
| 第三方应用登录用户 | ✅ 已有 | `POST /api/user/login`（返回 Bearer access_token，不依赖浏览器 Cookie） |
| 登录态续期 | ✅ 已有 | `POST /api/user/auth/refresh`（轮换 refresh Cookie） |
| 服务端长期凭据 | ✅ 已有 | `GET /api/user/token` 生成 PAT（29–32 位，长期有效） |
| 创建中继 API Key（`sk-…`） | ✅ 已有 | `POST /api/token/` 创建 + `POST /api/token/:id/key` 取明文 |
| 注册后自动创建指定名称/分组的 Key | ⚠️ 核心接口**不支持** | 核心注册接口只在 `GENERATE_DEFAULT_TOKEN=true` 时建 Key，名称固定为 `<用户名>的初始令牌`，分组固定为 `auto` 或空，且**不返回 Key**；`zsy/appauth` 插件补齐了该能力（§7） |

注册接口中「默认令牌」的实现见 `controller/user.go:290-317`；开关默认关闭，见 `common/init.go:193-194`（`GENERATE_DEFAULT_TOKEN`，默认 `false`）。

---

## 2. 凭据模型（先看这一节）

项目共有 4 类凭据，用途互不重叠：

| 凭据 | 获取方式 | 有效期 | 携带方式 | 用途 |
| --- | --- | --- | --- | --- |
| **会话 access_token** | 登录 / 刷新的响应体 `data.access_token` | **15 分钟**（`service/auth_token.go:18`） | `Authorization: Bearer <token>` | 调用全部 `/api/*` 管理接口 |
| **refresh token**（Cookie） | 登录响应 `Set-Cookie: new_api_refresh=…` | **30 天**（`LoginSessionTTL`） | Cookie，仅 `Path=/api/user/auth` | 只能调 `POST /api/user/auth/refresh` |
| **PAT**（个人访问令牌） | `GET /api/user/token`（需先登录） | 不自动过期，重新生成即失效 | `Authorization: Bearer <29–32位串>` | 服务端长期调用 `/api/*` |
| **中继 API Key**（`sk-…`） | `POST /api/token/` 创建 | 由 `expired_time` 决定（`-1` = 永久） | `Authorization: Bearer sk-<48位>` | 调用 `/v1/*` 中继接口（真正干活的 Key） |

鉴权解析顺序见 `middleware/auth.go:150-192`（Bearer → 会话 token 或 PAT）。中继 Key 的鉴权见 `middleware/auth.go:279-319`（自动剥离 `Bearer ` 与 `sk-` 前缀）。

---

## 3. 服务端前置开关

### 3.1 先用 `GET /api/status` 探测（无需鉴权）

```bash
curl -s https://<网关域名>/api/status
```

关键字段（`controller/misc.go:54-104`）：

| 字段 | 含义 |
| --- | --- |
| `register_enabled` | 注册总开关 |
| `password_register_enabled` | 密码注册开关（关闭则注册接口直接拒绝） |
| `password_login_enabled` | 密码登录开关（关闭则登录接口直接拒绝） |
| `email_verification` | 是否强制邮箱验证码 |
| `invite_code_required` | 是否强制邀请码（开启后 `aff_code` 必填且必须有效） |
| `turnstile_check` | 是否强制 Cloudflare Turnstile 人机校验 |

### 3.2 需要管理员开启的选项

| 选项键（系统设置） | 作用 | 代码位置 |
| --- | --- | --- |
| `RegisterEnabled` | 注册总开关 | `model/option.go:330-331` |
| `PasswordRegisterEnabled` | 密码注册 | `model/option.go:314-315` |
| `PasswordLoginEnabled` | 密码登录 | `model/option.go:316-317` |
| `EmailVerificationEnabled` | 强制邮箱验证码 | `model/option.go:318-319` |
| `InviteCodeRequired` | 强制邀请码（邀请制站点） | `model/option.go` (`case "InviteCodeRequired"`)、校验逻辑 `model/user.go:CheckInviteCode` |
| `TurnstileCheckEnabled` | 强制 Turnstile | `model/option.go:328-329` |

### 3.3 相关环境变量

| 环境变量 | 默认值 | 说明 |
| --- | --- | --- |
| `GENERATE_DEFAULT_TOKEN` | `false` | 注册时是否生成默认令牌（名称不可定制） |
| `CRITICAL_RATE_LIMIT` | `20` | 敏感接口限流次数，按客户端 IP |
| `CRITICAL_RATE_LIMIT_DURATION` | `1200`（20 分钟） | 限流窗口（秒） |
| `ANONYMOUS_REQUEST_BODY_LIMIT_KB` | `512` | 匿名接口请求体上限 |
| `USER_SESSION_ACTIVE_LIMIT` | `50` | 单用户同时活跃会话数上限 |
| `USER_SESSION_ISSUANCE_LIMIT` | `100` | 单用户滚动窗口内登录次数上限 |
| `SESSION_COOKIE_SECURE` | `false` | 置 `true` 后，refresh/logout 强制校验 `Origin`/`Referer` |
| `SESSION_COOKIE_TRUSTED_URL` | 空 | `SESSION_COOKIE_SECURE=true` 时必填，声明可信来源域名 |

---

## 4. 接口详情

> 所有接口的 HTTP 状态码**几乎恒为 200**，业务成败必须判响应体的 `success` 字段（`common/gin.go:199-239`）。

统一响应结构：

```json
{ "success": true,  "message": "",      "data": { } }
{ "success": false, "message": "错误信息" }
```

`message` 会按请求语言本地化（`middleware.I18n` 中间件，取 `Accept-Language`）。

---

### 4.1 注册

```
POST /api/user/register
```

中间件链（`router/api-router.go:79`）：`GlobalAPIRateLimit` → `CriticalRateLimit` → `RequestBodyLimit(512KB)` → `TurnstileCheck`

**Query 参数**

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `turnstile` | 仅当 `turnstile_check=true` | Cloudflare Turnstile 校验结果 token |

**请求体**（字段来自 `model.User`，`model/user.go:79-112`；实际只读取下列字段）

| 字段 | 类型 | 必填 | 约束 |
| --- | --- | --- | --- |
| `username` | string | 是 | 去空格后非空，最长 20 |
| `password` | string | 是 | 长度 8–20 |
| `email` | string | 仅当 `email_verification=true` | 最长 50 |
| `verification_code` | string | 仅当 `email_verification=true` | 邮箱验证码（见 4.9） |
| `aff_code` | string | **仅当 `invite_code_required=true`** | 邀请人的邀请码（= 对方的 `aff_code`），必须能解析到已存在的用户 |

```bash
curl -s -X POST 'https://<网关域名>/api/user/register' \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo_user","password":"pass12345"}'

# 邀请制站点（/api/status 返回 invite_code_required=true）必须带邀请码：
curl -s -X POST 'https://<网关域名>/api/user/register' \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo_user","password":"pass12345","aff_code":"<邀请码>"}'
```

**成功响应**

```json
{ "success": true, "message": "" }
```

**注意**

- 成功响应**不包含会话、不包含任何 Key**，需要再调登录接口。
- 新用户额度取 `QuotaForNewUser`；新用户默认分组为 `default`（`model/user.go:98`）。
- 常见失败：`success=false` + 「用户名已存在 / 注册功能已关闭 / 邮箱验证码错误 / 已达到最大令牌数量限制」等。
- `invite_code_required=true` 时，`aff_code` 缺失返回「本站注册需要邀请码，请填写邀请码」，无法解析到用户返回「邀请码无效，请核对后重试」；两种情况都**不会创建账号**。

> 同一套邀请码规则也作用于其它注册入口：OAuth 回调（邀请码随 `/api/oauth/state` 的 `aff` 字段进入流程）、微信注册（`GET /api/oauth/wechat?code=…&aff=<邀请码>`，仅首次创建用户时校验）、以及插件接口 `POST /api/zsy/auth/register`。判定逻辑集中在 `model.CheckInviteCode`。

---

### 4.2 登录

```
POST /api/user/login
```

中间件链（`router/api-router.go:80`）：`CriticalRateLimit` → `DisableCache` → `RequestBodyLimit` → `TurnstileCheck`（**注意：登录也走 Turnstile 校验**）

**请求体**（`controller/user.go:30-33`）

```json
{ "username": "demo_user", "password": "pass12345" }
```

**成功响应（未开启 2FA）**（`controller/user.go:193-203`）

```json
{
  "success": true,
  "message": "",
  "data": {
    "access_token": "…",
    "token_type": "Bearer",
    "access_expires_at": 1767225600,
    "session": { "sid": "…", "current": true, "login_method": "password", "ip": "…", "user_agent": "…", "created_at": 0, "last_active_at": 0, "expires_at": 0 },
    "user": { "id": 12, "username": "demo_user", "quota": 0, "group": "default", "…": "…" }
  }
}
```

同时下发刷新 Cookie：

```
Set-Cookie: new_api_refresh=<sid>.<secret>; Path=/api/user/auth; Max-Age=2592000; HttpOnly; SameSite=Strict
```

**成功响应（已开启 2FA）**（`controller/user.go:100-109`）

```json
{ "success": true, "message": "…", "data": { "require_2fa": true, "flow_token": "…", "expires_at": 1767225900 } }
```

此时需继续调用 4.3。

**失败**：`success=false`，如 `password_login_enabled=false` 时返回「密码登录已关闭」。

---

### 4.3 两步验证（2FA）登录

```
POST /api/user/login/2fa
```

**请求体**（`controller/twofa.go:22-25`）

```json
{ "code": "123456", "flow_token": "<登录响应中的 flow_token>" }
```

成功时返回与 4.2 相同的会话响应（含 `access_token` 与刷新 Cookie）。`flow_token` 有效期 5 分钟，且与用户 `auth_version` 绑定（`controller/twofa.go:437-468`）。

---

### 4.4 刷新登录态

```
POST /api/user/auth/refresh
```

中间件链（`router/api-router.go:77`）：`SessionCookieOriginGuard` → `CriticalRateLimit` → `DisableCache`

**请求头**

| 头 | 必填 | 说明 |
| --- | --- | --- |
| `Cookie` | 是 | `new_api_refresh=<登录时下发的值>` |
| `X-Auth-Session` | 否 | 传入 `session.sid` 时校验会话一致性 |
| `Origin` / `Referer` | 当 `SESSION_COOKIE_SECURE=true` 时必填 | 必须是网关自身域名或 `SESSION_COOKIE_TRUSTED_URL` 中声明的域名 |

**响应**：与登录相同的 `access_token` + `session`，并轮换下发新的 refresh Cookie（旧的立即失效）。

> ⚠️ Cookie 为 `SameSite=Strict` + `Path=/api/user/auth`，浏览器跨站场景不可用；服务端应用可自行保存 `Set-Cookie` 的值，用 `Cookie:` 头回放。

---

### 4.5 退出登录

```
POST /api/user/auth/logout
```

携带 `Authorization: Bearer <access_token>` 或 refresh Cookie；成功返回 `{"success":true,"data":{"revoked_sid":"…","cookie_cleared":true}}`（`controller/auth_session.go:47-95`）。

---

### 4.6 查询当前用户

```
GET /api/user/self
Authorization: Bearer <access_token 或 PAT>
```

返回用户资料（含 `quota`、`group`、`used_quota` 等），`controller/user.go:481`。

---

### 4.7 生成长期凭据 PAT

```
GET /api/user/token
Authorization: Bearer <access_token>
```

中间件：`CriticalRateLimit` + `UserCriticalRateLimit("access-token")`（`router/api-router.go:100`）

**响应**

```json
{ "success": true, "message": "", "data": "<29–32 位访问令牌>" }
```

`data` 即为 PAT，长期有效（每次生成会覆盖旧值，用户表 `access_token` 列为 `char(32)`）。适合第三方应用服务端保存后长期调用 `/api/*`。

---

### 4.8 创建中继 API Key（`sk-…`）

```
POST /api/token/
Authorization: Bearer <access_token 或 PAT>
Content-Type: application/json
```

请求体为 `model.Token` 结构（`controller/token.go:35-38`、`model/token.go:14-33`）：

| 字段 | 类型 | 说明 | 默认/限制 |
| --- | --- | --- | --- |
| `name` | string | **Key 名称** | 最长 50 |
| `group` | string | **Key 分组** | 空字符串 = 跟随用户分组；见 §5 分组校验 |
| `expired_time` | int64 | 过期时间（秒级时间戳） | `-1` = 永不过期 |
| `unlimited_quota` | bool | 是否无限额度 | — |
| `remain_quota` | int | 剩余额度 | `unlimited_quota=false` 时须 `0 ≤ remain_quota ≤ maxTokenQuota` |
| `model_limits_enabled` | bool | 是否限制可用模型 | — |
| `model_limits` | string | 模型白名单，逗号分隔 | — |
| `allow_ips` | string | IP 白名单，换行/逗号分隔 | — |
| `cross_group_retry` | bool | 跨分组重试（仅 `group=auto` 有效） | 非 `auto` 会被强制置 false |
| `auto_groups` | string[] | `group=auto` 时的分组顺序 | 受全局 `MaxTokenAutoGroups` 限制 |

```bash
curl -s -X POST 'https://<网关域名>/api/token/' \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
        "name": "默认密钥",
        "group": "官方渠道",
        "expired_time": -1,
        "unlimited_quota": true,
        "remain_quota": 0,
        "model_limits_enabled": false
      }'
```

**成功响应（注意：不含明文 Key）**

```json
{ "success": true, "message": "" }
```

限制：单用户令牌数量上限（`operation_setting.GetMaxUserTokens()`），超出返回 `success=false` + 提示。

---

### 4.9 取 Key 明文 / 列表

```bash
# 列表（Key 已脱敏，可拿到 id）：GET /api/token/?p=1&page_size=100
curl -s 'https://<网关域名>/api/token/?p=1&page_size=100' -H "Authorization: Bearer $ACCESS_TOKEN"

# 取指定 Key 明文
curl -s -X POST 'https://<网关域名>/api/token/<ID>/key' -H "Authorization: Bearer $ACCESS_TOKEN"
```

第二个接口返回：

```json
{ "success": true, "message": "", "data": { "key": "<48 位 Key 明文>" } }
```

调用中继时使用 `Authorization: Bearer sk-<48 位 Key>`。服务端解析规则（`middleware/auth.go:391-408`）：先剥离 `Bearer `，再剥离 `sk-`，再按 `-` 切分取第一段 —— 因此 `sk-<48 位 Key>` 与 `sk-<48 位 Key>-任意后缀` 都可用；`/v1/models` 系列还额外支持 `?key=<Key>` 传参。

---

### 4.10 邮箱验证码（仅 `email_verification=true` 时）

```
GET /api/verification?email=<邮箱>&turnstile=<token>
```

中间件：`EmailVerificationRateLimit` + `TurnstileCheck`（`router/api-router.go:42`）。成功后用户邮箱收到 6 位验证码，注册时通过 `verification_code` 字段提交。

---

## 5. 分组（`group`）校验 —— 最容易踩的坑

Key 的 `group` 不是随便填的。中继请求鉴权时会做两级校验（`middleware/auth.go:460-474`）：

1. `token.group` 必须在该用户所属用户组的「可用分组」内，否则 **403**：`无权访问 <group> 分组`；
2. `token.group` 必须存在于分组倍率表中，否则 **403**：`分组 <group> 已被弃用`（`auto` 除外）。

因此要让 `group = 官方渠道` 的 Key 真正可用，需要同时满足：

- 「系统设置 → 分组倍率」中存在 `官方渠道` 这一分组（名称以实际配置为准）；
- 「系统设置 → 用户可用分组」中该分组对目标用户组可见。

如果新注册用户落在 `default` 组而 `default` 组看不到 `官方渠道`，需要管理员调整可用分组配置；`zsy/appauth` 插件（§7）会为此显式返回 `warning` 与 `group_usable:false`，但同样不会替你修改用户分组。

---

## 6. 第三方应用完整接入流程

### 方案 A：仅使用现有接口（零改动，推荐先验证）

```bash
GW='https://<网关域名>'

# 0) 探测开关
curl -s "$GW/api/status"

# 1) 注册（邀请制站点需在 body 中补 "aff_code":"<邀请码>"，否则被拒）
curl -s -X POST "$GW/api/user/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo_user","password":"pass12345"}'
# → {"success":true,"message":""}

# 2) 登录，拿 access_token（-i 可同时看到 Set-Cookie）
ACCESS=$(curl -s -X POST "$GW/api/user/login" \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo_user","password":"pass12345"}' \
  | jq -r '.data.access_token')

# 2') 若返回 require_2fa=true，则用 flow_token 完成 2FA
# curl -s -X POST "$GW/api/user/login/2fa" -H 'Content-Type: application/json' \
#   -d '{"code":"123456","flow_token":"<flow_token>"}'

# 3) 创建 Key：名称「默认密钥」，分组「官方渠道」
curl -s -X POST "$GW/api/token/" \
  -H "Authorization: Bearer $ACCESS" \
  -H 'Content-Type: application/json' \
  -d '{"name":"默认密钥","group":"官方渠道","expired_time":-1,"unlimited_quota":true,"remain_quota":0,"model_limits_enabled":false}'
# → {"success":true,"message":""}

# 4) 取刚创建 Key 的 id，再取明文
ID=$(curl -s "$GW/api/token/?p=1&page_size=100" -H "Authorization: Bearer $ACCESS" | jq -r '.data.items[0].id')
KEY=$(curl -s -X POST "$GW/api/token/$ID/key" -H "Authorization: Bearer $ACCESS" | jq -r '.data.key')

# 5) 用 Key 调中继接口
curl -s "$GW/v1/chat/completions" \
  -H "Authorization: Bearer sk-$KEY" \
  -H 'Content-Type: application/json' \
  -d '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}'
```

该方案缺点：注册与建 Key 是两次调用，且 Key 分组依赖 §5 的分组配置；第三方应用需自行处理 access_token 过期（15 分钟，用 refresh 或 PAT 兜底）。

### 方案 B：注册即建键（`zsy/appauth` 插件，已实现）

一次调用完成注册 + 建 Key + 下发登录会话，并直接返回 Key 明文。完整接口见 §7。

```bash
GW='https://<网关域名>'

curl -s -X POST "$GW/api/zsy/auth/register" \
  -H 'Content-Type: application/json' \
  -d '{"username":"demo_user","password":"pass12345"}' \
  | jq -r '.data.key.api_key'      # → 48 位 Key 明文，直接用 Bearer sk-<它> 调 /v1/*
```

---

## 7. 方案 B：`zsy/appauth` 插件接口参考

插件源码：`zsy/appauth/`（`config.go` 配置、`routes.go` 路由、`controllers.go` 处理器、`keys.go` 建键、`respond.go` 响应约定），由 `zsy/extbootstrap` 空白导入安装，通过 `extcore` 注册，**不修改任何核心文件**。

### 7.1 与核心接口的差异

| 维度 | 核心 `/api/user/register` | 插件 `/api/zsy/auth/register` |
| --- | --- | --- |
| 建 Key | 默认不建（`GENERATE_DEFAULT_TOKEN=false`），建了也不返回 | **必建**：名称 `默认密钥`、分组 `官方渠道`、其余取 Key 默认值，并返回明文 |
| 登录会话 | 需再调一次 `/api/user/login` | 同一次响应直接下发 `access_token` + refresh Cookie（可用 `issue_session:false` 关闭） |
| Turnstile | 开启后强制校验 | **不校验**（服务端调用无法解浏览器挑战），改用可选的 `X-Zsy-App-Secret` |
| 错误语义 | 一律 HTTP 200 + `success:false` | **真实 HTTP 状态码** + 稳定 `code` 字段 |
| 2FA 账号登录 | 支持（`flow_token` + `/api/user/login/2fa`） | 明确拒绝（`MFA_REQUIRED`），不倒逼绕过 2FA；调用方改走核心两步流程 |

### 7.2 配置项（环境变量）

| 变量 | 默认值 | 说明 |
| --- | --- | --- |
| `ZSY_AUTH_ENABLED` | `true` | `false` 时不挂载插件路由（无需删代码即可停用） |
| `ZSY_AUTH_DEFAULT_KEY_NAME` | `默认密钥` | 账号 Key 的名称 |
| `ZSY_AUTH_DEFAULT_KEY_GROUP` | `官方渠道` | 账号 Key 的分组 |
| `ZSY_AUTH_APP_SECRET` | 空（不校验） | 非空时，所有插件请求必须携带 `X-Zsy-App-Secret: <值>`，否则 401 |

### 7.3 中间件链

`RouteTag("api")` → `GlobalAPIRateLimit` → `CriticalRateLimit`（按 IP，默认 20 次/20 分钟）→ `AnonymousRequestBodyLimit`（默认 512KB）→ `DisableCache` →（可选）`requireAppSecret`。

### 7.4 `POST /api/zsy/auth/register`

请求体：

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `username` | string | 是 | 去空格后非空，最长 20 |
| `password` | string | 是 | 长度 8–20 |
| `email` | string | 当 `EmailVerificationEnabled=true` | 最长 50 |
| `verification_code` | string | 当 `EmailVerificationEnabled=true` | 邮箱验证码（先调 `GET /api/verification`） |
| `aff_code` | string | 当 `InviteCodeRequired=true` | 邀请码（= 邀请人的 `aff_code`），必须有效 |
| `issue_session` | bool | 否 | 默认 `true`；`false` 时不创建登录会话、不消耗会话额度 |

成功响应（200）：

```json
{
  "success": true,
  "message": "",
  "data": {
    "user": { "id": 1, "username": "demo_user", "display_name": "demo_user", "group": "default", "quota": 0, "used_quota": 0, "status": 1 },
    "key": {
      "id": 1,
      "name": "默认密钥",
      "group": "官方渠道",
      "api_key": "9DLFM5Fkg7BdVCJjCuwKsn3CfY77BvFLsjZvWJPg67y2ufBb",
      "status": 1,
      "unlimited_quota": true,
      "expired_time": -1,
      "group_usable": true
    },
    "key_created": true,
    "session": { "access_token": "eyJ…", "token_type": "Bearer", "access_expires_at": 1789696557, "sid": "51b38a59-…" },
    "warning": ""
  }
}
```

`api_key` 与核心 `POST /api/token/:id/key` 一致：**不含 `sk-` 前缀**，调用中继时拼成 `Authorization: Bearer sk-<api_key>`。

错误响应（HTTP 状态码 + `code`）：

| 状态 | code | 触发条件 |
| --- | --- | --- |
| 400 | `INVALID_PARAMS` | 参数缺失、用户名过长、密码不在 8–20 位 |
| 400 | `EMAIL_VERIFICATION_REQUIRED` / `VERIFICATION_CODE_ERROR` | 启用了邮箱验证但邮箱/验证码缺失或错误 |
| 400 | `INVITE_CODE_REQUIRED` / `INVITE_CODE_INVALID` | 站点开启 `InviteCodeRequired` 但 `aff_code` 缺失或无法解析到用户 |
| 401 | `APP_SECRET_INVALID` | 配置了 `ZSY_AUTH_APP_SECRET` 但请求头缺失/不匹配 |
| 403 | `REGISTER_DISABLED` / `PASSWORD_REGISTER_DISABLED` | 站点关闭了注册 / 密码注册 |
| 409 | `USER_EXISTS` / `EMAIL_TAKEN` | 用户名或邮箱已占用 |
| 500 | `REGISTER_FAILED` / `KEY_CREATE_FAILED` / `DATABASE_ERROR` | 落库或建 Key 失败（见下） |

### 7.5 `POST /api/zsy/auth/login`

请求体：`{"username":"…","password":"…","issue_session":true}`（`issue_session` 默认 `true`）。

响应 `data` 与注册一致（`session` 仅在创建会话时出现）。行为要点：

- 账号尚未持有「默认密钥」时，**登录会补建**（`key_created:true`）；已存在则按名称优先、其次按分组复用，不会重复建 Key；
- 账号已开启 TOTP 时返回 403 `MFA_REQUIRED`，不签发会话、不建 Key；
- 错误：401 `LOGIN_FAILED`（用户名或密码错误）、403 `PASSWORD_LOGIN_DISABLED`、500 `DATABASE_ERROR`。

### 7.6 建 Key 的字段取值（「其余默认」）

| 字段 | 取值 | 来源 |
| --- | --- | --- |
| `name` | `默认密钥` | `ZSY_AUTH_DEFAULT_KEY_NAME` |
| `group` | `官方渠道` | `ZSY_AUTH_DEFAULT_KEY_GROUP` |
| `status` | `1` 启用 | `common.TokenStatusEnabled` |
| `unlimited_quota` | `true` | 与面板新建 Key 默认一致 |
| `expired_time` | `-1`（永不过期） | 同上 |
| `remain_quota` | `0` | 无限额度下无意义 |
| `model_limits_enabled` / `model_limits` | `false` / 空 | 不限制模型 |
| `allow_ips` | 空 | 不限制 IP |

### 7.7 分组前置条件（部署前必读）

`group_usable:false` 或响应里的 `warning` 表示该 Key **当前会被中继鉴权拒绝（403）**。让 `官方渠道` 真正可用需要同时满足：

1. 「系统设置 → 分组倍率」中存在 `官方渠道`；
2. 「系统设置 → 用户可用分组」中 `官方渠道` 对目标用户组可见（新用户默认落在 `default` 组）。

插件不会因此拒绝注册，也不会偷偷把分组改成别的值：它保留配置的分组，并在响应与后端日志中显式报告该问题（`keys.go:keyGroupWarning`）。若要连用户分组一起调整为 `官方渠道`，需要在插件基础上另行扩展（当前只固定 Key 的 group）。

### 7.8 测试与验证

```bash
# 仅插件测试（25 个用例：建键幂等、复用改名后的 Key、注册/登录全链路、
# 2FA 拒绝、开关与 app secret 门禁、分组可用性告警）
go test ./zsy/appauth/...

# 全量构建
go build ./...
```

---

## 8. 源码索引

| 关注点 | 位置 |
| --- | --- |
| 注册实现 | `controller/user.go:206-324` |
| 邀请码校验（所有注册入口共用） | `model/user.go:CheckInviteCode`（开关 `common.InviteCodeRequired`，选项 `InviteCodeRequired`） |
| 邀请码拦截点 | 网页 `controller/user.go:Register`、OAuth `controller/oauth.go:findOrCreateOAuthUser`、微信 `controller/wechat.go:WeChatAuth`、App 接口 `zsy/appauth/controllers.go:register` |
| 前端邀请码输入与透传 | `web/src/features/auth/sign-up/components/sign-up-form.tsx`、`web/src/features/auth/lib/storage.ts` |
| 登录实现 / 会话下发 | `controller/user.go:40-204` |
| 2FA 登录 | `controller/twofa.go:427-` |
| 刷新 / 登出 | `controller/auth_session.go:17-95` |
| 会话与 Access Token TTL | `service/auth_token.go:18-20` |
| 刷新 Cookie 属性 | `service/auth_session.go:290-326` |
| 鉴权中间件（Bearer / PAT） | `middleware/auth.go:139-192` |
| Turnstile 校验 | `middleware/turnstile-check.go` |
| Origin 校验 | `middleware/auth_origin.go` |
| 限流 / 匿名体积限制 | `middleware/rate-limit.go:174-190`、`middleware/request_body_limit.go` |
| 中继 Key 创建 / 取明文 | `controller/token.go:275-352`、`controller/token.go:188-203` |
| 分组鉴权 | `middleware/auth.go:455-505` |
| 路由清单 | `router/api-router.go:75-110`、`router/api-router.go:243-255` |
| 插件注册与路由 | `zsy/appauth/extension.go`、`zsy/appauth/routes.go` |
| 插件处理器 | `zsy/appauth/controllers.go` |
| 插件建 Key / 复用 / 分组告警 | `zsy/appauth/keys.go` |
| 插件配置项 | `zsy/appauth/config.go` |
| 插件响应与错误码 | `zsy/appauth/respond.go` |
| 插件测试 | `zsy/appauth/appauth_test.go` |
