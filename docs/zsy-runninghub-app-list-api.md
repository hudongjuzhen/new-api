# RunningHub 应用列表接口（第三方接入文档）

> 适用版本：当前 `zsy/runninghub` 分支源码树
> 结论：**一个请求即可拿到全部 RH 应用**——`GET /api/zsy/rh/app-catalog` 返回所有「已发布、非管理员专属」的应用，包含**名称、介绍、封面、分类、站点、调用类型、计费方式**以及**完整参数表单定义（paramSchema）**，并按分类在站点里的展示顺序分好组。列表接口**无需鉴权**，浏览与提交是两个独立步骤。

---

## 1. 一分钟上手

```bash
# 1) 拉全量应用列表（含参数定义），无需鉴权
curl -s 'https://<你的网关域名>/api/zsy/rh/app-catalog'

# 2) 按参数定义填值提交一次运行（需要凭据，见 §4）
curl -X POST 'https://<你的网关域名>/api/zsy/rh/apps/42/run' \
  -H 'Authorization: Bearer sk-xxxxxxxx' \
  -H 'Content-Type: application/json' \
  -d '{"values":{"122.prompt":"一只小城堡"},"instanceType":"default"}'

# 3) 轮询任务（需要凭据）
curl -s 'https://<你的网关域名>/api/zsy/rh/apps/task/task_xxxxxxxxxxxxxxxx' \
  -H 'Authorization: Bearer sk-xxxxxxxx'
```

关键约定：

| 项 | 值 |
| --- | --- |
| 列表接口 | `GET /api/zsy/rh/app-catalog`（**零鉴权、不分页、一次拿全**） |
| 应用对象形状 | `AppView`，与分页列表 `GET /api/zsy/rh/apps` 的 `items[]` **完全一致**，可互换解析 |
| 参数键 | `values` 的键固定为 `"<nodeId>.<fieldName>"`（扁平模型应用为 `".<fieldName>"`，详见 §7） |
| 计费单位 | 所有 `quota` 都是**网关额度单位**（`common.QuotaPerUnit`），不是 RH 币 |
| 时间戳 | 全部为 **Unix 秒**（整数） |

---

## 2. 凭据模型

`/api/zsy/rh/**` 的同一枚 `Authorization: Bearer` 头接受三类凭据（分类逻辑见 `zsy/runninghub/auth_helpers.go`）：

| 凭据 | 形态 | 有效期 | 能做什么 |
| --- | --- | --- | --- |
| 中继 API Key | `sk-` + 48 位 | 由 Key 的 `expired_time` 决定（`-1` 永久） | **第三方接入首选**：提交 / 查询 / 取消自己提交的任务。分组、额度、IP 白名单、模型白名单等既有约束全部生效 |
| 面板访问令牌（PAT） | 29–32 位串 | 不自动过期 | 面板语义：账号级可见范围 |
| 面板会话 JWT | JWT | 15 分钟 | 浏览器里的面板本身 |

**浏览类接口（`app-catalog` / `apps` / `apps/:id`）不需要任何凭据**——应用中心的可见性策略就是「游客可见的都已发布且非管理员专属」。只有提交、查询、取消、上传需要凭据。

API Key 的三条硬约束（返回带机器码的错误，见 §9）：

- 只能查询 / 取消**它自己提交**的任务，否则 `403 task_not_owned_by_key`；
- **不能**在 body 里指定别的 Key（`tokenId` 必须等于本 Key），否则 `400 token_selection_not_allowed`；
- Key 若配置了模型白名单，`upstreamId` 不在白名单内则 `403 token_model_forbidden`。

---

## 3. 统一响应信封

所有 `/api/zsy/rh/**` 响应都是同一信封：

```json
// 成功
{ "success": true, "message": "", "data": { /* 见各接口 */ } }

// 业务失败（HTTP 状态码可能仍是 200，请判 success 而不是只看状态码）
{ "success": false, "message": "应用不存在" }

// 带机器码的失败（第三方应分支 code，不要匹配 message 文案）
{ "success": false, "code": "token_model_forbidden", "message": "当前密钥未被授权使用模型 ..." }
```

> 判错建议：先看 HTTP 状态码，再判 `success`；`code` 存在时优先用 `code`。

---

## 4. 接口一览

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| **GET** | **`/api/zsy/rh/app-catalog`** | **无** | **全量应用目录（含分类与参数定义），一次拿全，见 §5** |
| GET | `/api/zsy/rh/apps` | 无 | 分页应用列表（`AppListResult`），见 §6 |
| GET | `/api/zsy/rh/apps/:id` | 无 | 单个应用详情（`AppView`） |
| POST | `/api/zsy/rh/apps/:id/run` | 必需 | 提交运行，见 §8 |
| GET | `/api/zsy/rh/apps/task/:task_id` | 必需 | 查询任务状态与结果，见 §8 |
| POST | `/api/zsy/rh/apps/task/:task_id/cancel` | 必需 | 取消任务（排队中本地取消；已派发先调上游确认） |
| GET | `/api/zsy/rh/apps/task/:task_id/content?url=` | 必需 | 结果文件文本预览（仅限该任务自身结果 URL，上限 1 MiB） |
| GET | `/api/zsy/rh/apps/tasks` | 必需（仅面板） | 本人历史任务分页；**API Key 调用 403 `key_scoped_list_unsupported`** |
| POST | `/api/zsy/rh/upload?site=cn\|intl` | 必需 | 媒体上传代理（multipart `file` 字段），返回 `fileName` |
| GET | `/api/zsy/rh/upload-channel?site=cn\|intl` | 必需 | 该站点是否已有可用渠道（`{available, count}`） |

---

## 5. `GET /api/zsy/rh/app-catalog` —— 应用目录（核心接口）

### 5.1 请求

```
GET /api/zsy/rh/app-catalog
```

可选查询参数（都可省略，省略即「全部」）：

| 参数 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `kind` | string | 全部 | 调用类型：`ai_app` / `workflow` / `model`。非法值 → `400 invalid_kind` |
| `site` | string | 全部 | 站点：`cn` / `intl`（也接受 `runninghub`、`runninghub.cn`、`国际` 等别名）。非法值 → `400 invalid_site` |
| `categoryId`（或 `category_id`） | int | 全部 | 只返回该分类下的应用。非整数 → `400 invalid_category_id` |

> **`site` 是精确匹配**：`site` 为空串的 legacy 应用（不绑定站点池）**不会**出现在 `?site=cn` 的结果里。要连它们一起拿，请省略 `site` 或拉全量后再按返回值自行分组。
>
> `kind` / `site` 过滤在 SQL 层完成，`categories` 数组**不跟着过滤**：分类导航栏始终是完整的，每个分类的 `appCount` 才是过滤后的数量（可能是 0）。

### 5.2 响应

```json
{
  "success": true,
  "message": "",
  "data": {
    "generatedAt": 1767225600,
    "totalApps": 2,
    "totalCategories": 2,
    "categories": [
      {
        "id": 10,
        "name": "图像",
        "sortOrder": 10,
        "appCount": 1,
        "apps": [
          {
            "id": 42,
            "createdAt": 1766000000,
            "updatedAt": 1766000000,
            "name": "文生图 Pro",
            "slug": "text2image-pro",
            "kind": "ai_app",
            "upstreamId": "1877265245566922753",
            "description": "输入提示词生成高清图片，支持 1K/2K 分辨率。",
            "coverUrl": "https://cdn.example.com/covers/text2image.png",
            "published": true,
            "adminOnly": false,
            "paramSchema": [
              {
                "nodeId": "122",
                "fieldName": "prompt",
                "label": "提示词",
                "type": "textarea",
                "required": true,
                "placeholder": "描述你想要的画面"
              },
              {
                "nodeId": "275",
                "fieldName": "reference_image",
                "label": "参考图",
                "type": "image",
                "required": false,
                "defaultValue": "openapi/example.png"
              },
              {
                "nodeId": "301",
                "fieldName": "resolution",
                "label": "分辨率",
                "type": "select",
                "required": true,
                "options": [
                  { "label": "1K", "value": "1k" },
                  { "label": "2K", "value": "2k" }
                ]
              }
            ],
            "perCallBilling": true,
            "fixedQuotaPerCall": 500000,
            "perSecondBilling": false,
            "quotaPerSecond": 0,
            "secondsExpr": "",
            "perCharBilling": false,
            "quotaPerChar": 0,
            "charCountExpr": "",
            "modelBaseRateRatio": 1,
            "site": "cn",
            "categoryId": 10,
            "categoryName": "图像"
          }
        ]
      },
      {
        "id": 0,
        "name": "未分类",
        "sortOrder": 0,
        "appCount": 0,
        "apps": []
      }
    ]
  }
}
```

### 5.3 字段说明

顶层：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `generatedAt` | int64 | 本次响应的生成时间（Unix 秒），用于缓存判定 |
| `totalApps` | int | 本次响应里的应用总数（已按 `kind`/`site`/`categoryId` 过滤后） |
| `totalCategories` | int | `categories` 数组长度，**含空分类** |
| `categories[]` | array | 分类分组，顺序：先按 `sortOrder` 升序、再按 `id` 升序；**「未分类」(`id: 0`) 永远排在最后** |

分类分组：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | uint | 分类 ID；`0` 表示「未分类」这个虚拟分组 |
| `name` | string | 分类名；未分类分组固定为 `未分类` |
| `sortOrder` | int | 面板里的展示顺序（未分类为 0） |
| `appCount` | int | 该分组内**本次过滤后**的应用数 |
| `apps[]` | array | `AppView` 数组，分类内按 `name` 升序；空分类为 `[]`（不是 `null`） |

`AppView`（应用对象）：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `id` | uint | 网关侧应用 ID，**提交 / 详情接口用的就是它** |
| `name` | string | 应用名称（全站唯一） |
| `slug` | string | 可选短名，可为空 |
| `kind` | string | `ai_app`（AI 应用）/ `workflow`（工作流）/ `model`（模型 API） |
| `upstreamId` | string | 上游 RunningHub 应用 / 工作流 ID。**第三方一般不需要它**（用 `id` 即可）；它也是模型名与价格表的键 |
| `description` | string | 应用介绍（应用卡片与详情页的正文） |
| `coverUrl` | string | 封面图 URL，可为空。后台新增 / 编辑应用时可直接上传图片，上传结果存为网关自身地址（`/uploads/…` 或 OSS 全地址）；也接受 `http(s)://` 外链 |
| `published` | bool | 恒为 `true`（目录只含已发布应用） |
| `adminOnly` | bool | 恒为 `false`（目录只含非管理员专属应用） |
| `paramSchema` | array | **参数定义**，见 §7，是渲染表单的唯一依据 |
| `perCallBilling` | bool | 按次计费 |
| `fixedQuotaPerCall` | int64 | 按次计费时每次调用的额度 |
| `perSecondBilling` | bool | 按秒计费 |
| `quotaPerSecond` | int64 | 按秒计费时每秒额度 |
| `secondsExpr` | string | 按秒计费时，从哪些参数取秒数（`"212"` / `"nodeId=212"` / `"229-212"`），空则回退扫描 `seconds`/`duration` 参数 |
| `perCharBilling` | bool | 按字符计费 |
| `quotaPerChar` | int64 | 按字符计费时每字符额度 |
| `charCountExpr` | string | 按字符计费时，计费文本取自哪里（`"len(122)"`、`"len(122) + len(123)"`） |
| `modelBaseRateRatio` | float64 | 动态计费时的价格倍率（1.0 = 1× 基础价） |
| `site` | string | `cn`（国内站）/ `intl`（国际站）/ `""`（legacy，按模型→渠道选择）。**提交时路由按它决定走哪个站点渠道池** |
| `categoryId` | uint | 分类 ID，`0` 表示未分类（与所属分组 `id` 一致） |
| `categoryName` | string | 分类名，已就地 join 好（无分类时为空字符串） |
| `createdAt` / `updatedAt` | int64 | Unix 秒 |

计费字段的用法：`perCallBilling` / `perSecondBilling` / `perCharBilling` 三者**互斥**；三者都为 `false` 表示动态计费（按 `modelBaseRateRatio` × 渠道基础模型价）。调用方一般只需**原样展示**这些字段，计费由网关在提交时完成，不需要自己算价。

### 5.4 为什么用目录接口而不是分页列表

| | `GET /api/zsy/rh/app-catalog` | `GET /api/zsy/rh/apps` |
| --- | --- | --- |
| 分页 | 无，一次全量 | 有，默认 20/页，上限 100/页 |
| 分类 | 已分组 + 分类元数据（名称、顺序、计数） | 只有 `categoryId` / `categoryName`，需自行聚合，且分类列表要另调管理接口 |
| 适用 | 客户端缓存目录、渲染完整表单、生成自己的文档 | 面板的懒加载分页网格 |

---

## 6. `GET /api/zsy/rh/apps` —— 分页列表（面板在用）

| 参数 | 类型 | 默认 | 说明 |
| --- | --- | --- | --- |
| `keyword` | string | — | 在 `name` / `slug` / `upstreamId` / `description` 上模糊匹配 |
| `kind` | string | — | 同上，按 `kind` 精确过滤 |
| `p` | int | `1` | 页码 |
| `page_size` | int | `20` | 每页条数，`>100` 会被压回 20 |
| `sort_by` | string | `id` | `id` / `name` / `created_at` |
| `sort_order` | string | `desc` | `asc` / `desc` |

响应 `data`：

```json
{
  "items": [ /* AppView 数组，字段与 §5.3 完全一致 */ ],
  "total": 137,
  "page": 1,
  "pageSize": 20,
  "totalPages": 7,
  "kindCounts": { "ai_app": 90, "workflow": 40, "model": 7 }
}
```

> 注意：`page_size` 的硬上限是 **100**，因此面板式「翻一页拿全部」的做法在应用数超过 100 时会漏数据——需要全量时请用 `app-catalog`。

---

## 7. `paramSchema` —— 参数定义

每个条目描述一个可提交参数。**参数的顺序即面板里的表单顺序**；`required: true` 的必须提交。

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `nodeId` | string | 上游工作流节点 ID。**唯一例外**：`model` 类型（扁平请求体）的应用这里为空串 |
| `fieldName` | string | 上游字段名（扁平模型应用即顶层 JSON key） |
| `label` | string | 面向用户的字段名 |
| `type` | string | 控件类型，见下表 |
| `required` | bool | 是否必填 |
| `defaultValue` | string | 默认值（可省略）。**建议用它做表单初值** |
| `placeholder` | string | 输入提示（可省略） |
| `min` / `max` | number | 数值上下界（可省略）；`seconds` / `duration` 还会被网关的时长上界二次收紧 |
| `options` | array | `select` 的选项：`[{ "label": "1K", "value": "1k" }]`（可省略） |

`type` → 值形态（**这是最容易踩坑的一节**）：

| `type` | 提交值形态 | 说明 |
| --- | --- | --- |
| `text` / `textarea` / `string` / `password` | JSON 字符串 | 纯文本 |
| `number` / `int` / `integer` / `float` | 数字或数字字符串 | 会被校验 `min`/`max` |
| `seconds` / `duration` | 数字 | 按秒计费应用用它计量，上界被收紧到网关最大任务时长 |
| `select` / `radio` / `enum` | 字符串 | 值必须是某个 `options[].value`（或 `label`），否则 400 |
| `switch` / `boolean` / `bool` / `checkbox` / `toggle` | **字符串 `"true"` / `"false"`** | 提交时统一序列化成字符串；值为 `true`/`false` 的 JSON 布尔也能被接受 |
| `image` / `audio` / `video` / `file` | 字符串 | 上游 `fileName`（如 `openapi/xxxx.png`）或可直接访问的 URL；用 §4 的上传接口拿 `fileName` |

### values 的键

- `kind = ai_app` / `workflow`：键 = `"<nodeId>.<fieldName>"`，例如 `"122.prompt"`、`"275.reference_image"`。
- `kind = model`：`nodeId` 为空，但**键的拼法不变**，即 `".<fieldName>"`，例如 `".prompt"`。

缺必填、传未知键（未知键会被忽略）、数值越界、`select` 非法值都会返回可读的 400 文案，例如：

```json
{ "success": false, "message": "缺少必填参数: 提示词" }
```

---

## 8. 提交与查询（完整链路）

### 8.1 `POST /api/zsy/rh/apps/:id/run`

```json
{
  "values": {
    "122.prompt": "一只小城堡",
    "301.resolution": "2k"
  },
  "instanceType": "default",
  "webhookUrl": "https://your.app/rh-hook",
  "tokenId": 17
}
```

| 字段 | 必填 | 说明 |
| --- | --- | --- |
| `values` | 是 | 键同 §7 |
| `instanceType` | 否 | `default`（24GB）/ `plus`（48GB），默认 `default` |
| `webhookUrl` | 否 | RH 完成回调地址（透传给上游） |
| `tokenId` | 否 | 面板调用时可指定由哪个 Key 计费；**API Key 调用时只能等于自己**，否则 400 |

响应 `data`：

```json
{ "taskId": "task_xxxxxxxxxxxxxxxx", "status": "IN_PROGRESS",
  "upstreamTaskId": "1xxxxxxxxxxxxxxxxx", "raw": { "taskId": "1xxxx", "status": "RUNNING" } }
```

站点渠道**全部满额**时不会报错，而是受理为排队任务：

```json
{ "taskId": "task_xxxxxxxxxxxxxxxx", "status": "QUEUED", "queued": true }
```

排队任务由后台派发循环在有空位时自动提交；排队超过上界会置 `FAILURE` + 原因「排队超时」并**全额退款**。

### 8.2 `GET /api/zsy/rh/apps/task/:task_id`

`task_id` 用**网关公开 ID**（`run` 返回的那个），不是上游 ID。响应 `data` 是宿主 `TaskDto`：

```json
{
  "id": 987,
  "task_id": "task_xxxxxxxxxxxxxxxx",
  "platform": "61",
  "group": "default",
  "quota": 500000,
  "action": "rh_ai_app",
  "status": "SUCCESS",
  "fail_reason": "",
  "result_url": "https://rh-storage.example.com/out-0.png",
  "progress": "100%",
  "submit_time": 1767225600,
  "start_time": 1767225610,
  "finish_time": 1767225660,
  "properties": { "origin_model_name": "1877265245566922753" },
  "data": { "results": [ { "url": "https://rh-storage.example.com/out-0.png", "nodeId": "9", "outputType": "image" } ] },
  "billing_source": "wallet"
}
```

| 字段 | 说明 |
| --- | --- |
| `status` | `QUEUED` / `IN_PROGRESS` / `SUCCESS` / `FAILURE`（终态只有后两个） |
| `action` | `rh_ai_app` / `rh_workflow` / `rh_model`，与应用的 `kind` 对应 |
| `progress` | 百分比字符串：排队 `"20%"`、运行中 `"50%"`、终态 `"100%"` |
| `quota` | 本次实际扣除的额度（失败会全额退回） |
| `result_url` | 首个结果文件 URL（成功时） |
| `data.results[]` | 一次运行可能产出多个文件：`{url, nodeId, outputType}`；`outputType` ∈ `image`/`video`/`audio`/`text`/… |
| `billing_source` | `wallet`（钱包）或 `subscription`（订阅实例），另有 `subscription_id` |
| `fail_reason` | 失败 / 取消原因 |

轮询建议：`QUEUED`/`IN_PROGRESS` 时退避轮询（如 3→5→10 秒），命中终态即停。

### 8.3 `POST /api/zsy/rh/apps/task/:task_id/cancel`

- 仍在排队：纯本地取消，置 `FAILURE` + 原因「用户取消」并退款；
- 已派发：先调上游取消接口，**上游确认后**才置终态并退款；
- 上游拒绝（任务已结束/不可中断）→ 返回失败且**不退款**。

成功响应：

```json
{ "taskId": "task_xxxxxxxxxxxxxxxx", "status": "FAILURE", "refunded": true, "quota": 500000 }
```

### 8.4 媒体上传（`image` / `video` / `audio` 参数前置步骤）

```bash
# 先确认站点有可用渠道
curl -s 'https://<网关>/api/zsy/rh/upload-channel?site=cn' -H 'Authorization: Bearer sk-xxx'
# → {"success":true,"data":{"available":true,"count":2}}

# 上传（multipart，字段名固定 file），上限 50MB
curl -X POST 'https://<网关>/api/zsy/rh/upload?site=cn' \
  -H 'Authorization: Bearer sk-xxx' -F 'file=@./input.png'
# → {"success":true,"data":{"fileName":"openapi/abc123.png","url":"https://.../abc123.png"}}
```

把 `fileName` 原样作为该参数的值提交即可。`site` 必须与应用条目的 `site` 一致（`""` 视为 `cn`）。

---

## 9. 错误码

| HTTP | `code` | 场景 | 处理建议 |
| --- | --- | --- | --- |
| 400 | `invalid_kind` | `kind` 不是 `ai_app` / `workflow` / `model` | 修正过滤参数 |
| 400 | `invalid_site` | `site` 不是 `cn` / `intl`（含别名） | 修正过滤参数 |
| 400 | `invalid_category_id` | `categoryId` 不是非负整数 | 修正过滤参数 |
| 400 | `token_selection_not_allowed` | API Key 调用时 body 的 `tokenId` 与本 Key 不一致 | 去掉 `tokenId` |
| 403 | `token_model_forbidden` | Key 的模型白名单不含该应用的 `upstreamId` | 换 Key 或调整白名单 |
| 403 | `task_not_owned_by_key` | 查询 / 取消了别的 Key 提交的任务 | 只操作自己创建的 `taskId` |
| 403 | `key_scoped_list_unsupported` | API Key 调用了 `/apps/tasks` 历史列表 | 用 `task_id` 查询单个任务 |
| 429 | `channel_concurrency_saturated` | 站点渠道满额（无站点应用的 legacy 路径） | 退避重试；站点应用通常改为返回排队 `taskId` |
| 200 | 无 `code` | 参数校验失败、应用不存在、任务不存在等 | 直接展示 `message` |

---

## 10. 第三方接入推荐流程

1. **拉目录**：`GET /api/zsy/rh/app-catalog`（可带 `site` / `kind` 缩小范围），本地缓存 `generatedAt` + 应用列表。
2. **渲染表单**：用每个应用的 `paramSchema` 生成控件（§7 的 `type` 映射），初值取 `defaultValue`，必填项按 `required` 校验。
3. **上传素材**：遇到 `image`/`video`/`audio` 参数时，先 `POST /api/zsy/rh/upload` 拿 `fileName`。
4. **提交**：`POST /api/zsy/rh/apps/{id}/run`，body 的 `values` 键为 `"<nodeId>.<fieldName>"`；记录返回的 `taskId`。
5. **轮询**：`GET /api/zsy/rh/apps/task/{taskId}`，直到 `status` 为 `SUCCESS` 或 `FAILURE`；成功时读 `data.results[]`（可能有多个文件）。
6. **可选**：`webhookUrl` 让上游在完成时回调，或提供用户取消入口调用 `.../cancel`。

**不要在第三方侧做计费计算**：`perSecondBilling` / `perCharBilling` / 动态计费都由网关在提交时按应用配置定价，`quota` 字段就是最终扣费。

---

## 11. 常见问题

**Q：目录接口要不要带凭据？**
不需要。它只是「游客可见的应用目录」。带上凭据也不会改变内容（内容范围只看 `published` 与 `adminOnly`）。

**Q：应用条目里的 `upstreamId` 能直接拿去调 RunningHub 官方接口吗？**
不建议。网关的价值就在参数校验、站点选路、并发准入与计费；直接用上游 ID 绕过这些约束后计费会退回基础模型价，且没有排队与并发保护。

**Q：为什么 `item.upstreamId` 和 `id` 是两个不同的值？**
`id` 是网关侧的应用主键（提交/详情用它），`upstreamId` 是上游应用/工作流 ID（模型名与价格表的键）。第三方只用 `id`。

**Q：`site` 为空的 legacy 应用怎么办？**
它不绑定站点池，按「模型 → 渠道」选择渠道；满额时直接 `429`，不会排队。

**Q：目录会包含未发布或管理员专属应用吗？**
不会。这与面板「应用中心」的可见性完全一致；管理员可在 `/dashboard/zsy/rh/app-catalog` 用同一逻辑预览第三方可见内容。

---

## 12. 相关文件

| 关注点 | 位置 |
| --- | --- |
| 路由注册 | `zsy/runninghub/routes.go` |
| 目录接口处理器 | `zsy/runninghub/controller_app_catalog.go` |
| 目录装配（分组 / 排序 / 计数） | `zsy/runninghub/app_catalog.go` |
| 应用读模型 `AppView` 与存储层 | `zsy/runninghub/apps.go` |
| 提交 / 查询 / 取消 | `zsy/runninghub/controllers_user.go` |
| 上传代理 | `zsy/runninghub/controllers_upload.go` |
| 凭据分类与错误信封 | `zsy/runninghub/auth_helpers.go` |
| 错误码常量 | `zsy/runninghub/consts.go` |
| 回归测试 | `zsy/runninghub/app_catalog_test.go` |
| 全量设计说明 | `docs/zsy-runninghub-dev-plan.md` |
