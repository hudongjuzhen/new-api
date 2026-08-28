# zsy-runninghub 插件开发方案

> 版本：v1.0（技术实现方案）
> 范围：核心补丁（extcore）+ zsy-runninghub 插件（后端 + 管理端 + 用户端）

---

## 1. 总体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        new-api 主程序                            │
│                                                                 │
│  ┌── 核心补丁（一次性，通用）──────────────────────────────────┐ │
│  │  extcore/            扩展点注册中心（Go 新包，非业务）       │ │
│  │  6 处 anchor 编辑：  main.go / model/main.go / router/main.go│ │
│  │                     relay/relay_adaptor.go / constant/channel │ │
│  │  web/src/extensions/ 前端注册表（菜单/渠道类型/i18n 合并）    │ │
│  └──────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  ┌── zsy-runninghub 插件包（可增删）────────────────────────────┐ │
│  │  zsy/extbootstrap/    插件装配入口（blank import）            │ │
│  │  zsy/runninghub/      后端：controller / adaptor / keypool / │ │
│  │                       parser / service / model               │ │
│  │  web/src/extensions/zsy-runninghub/  前端：管理端 + 用户端    │ │
│  └──────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

核心设计决策：**插件复用 new-api 原生渠道（Channel）体系承载上游 Key**，而非自建配置表。

| 需求 | 复用的原生机制 | 插件自建部分 |
|---|---|---|
| 多 Key 轮询 | `Channel.Key`（换行分隔多 Key）+ `GetNextEnabledKey()` | 每 Key 并发上限（内存池） |
| 渠道按模型选择、分组、重试 | `Distribute` 中间件 + `GetRandomSatisfiedChannel` | — |
| 预扣费 / 结算 / 退款 | `RelayTaskSubmit` → `PreConsumeBilling` / `SettleBilling` / `RefundTaskQuota` | — |
| 异步任务轮询 / CAS / 超时清扫 | `service/task_polling.go`（已原生支持 `PrivateData.Key` 按任务用 Key 查询） | — |
| 按次计费 | `ModelPrice`（按次价格表）→ `PriceData.UsePrice` → `PerCallBilling` 自动生效 | 应用模板表 |
| 任务记录 / 日志 | `tasks` 表 + 用户任务页 | 用户端动态表单 UI |

理由：轮询按 `task.ChannelId` 分组取渠道（[task_polling.go#L140-L151](../service/task_polling.go#L140-L151)），任务必须落在渠道上才能被轮询、结算、退款；自建表会导致整条计费链路重写。

---

## 2. 核心补丁（extcore）设计

### 2.1 设计原则

1. **通用性**：extcore 不含任何 runninghub 业务，只提供"注册点"。未来接入其他平台（如 Liblib、即梦工作流等）时，核心补丁零改动，仅平台适配补丁（见 §8）。
2. **最小 anchor**：所有对既有文件的编辑均为"追加式"（新增 case / 追加一行调用），提供精确锚点与冲突检测说明。
3. **注册时机**：插件包通过 `init()` 自注册；`zsy/extbootstrap` blank-import 所有已安装插件；`main.go` 只 import `extbootstrap` 一次，此后安装/卸载插件不再改 `main.go`。

### 2.2 extcore 接口定义（`extcore/extcore.go`，新包）

```go
package extcore

import (
    "github.com/gin-gonic/gin"
)

// ── 任务平台适配器注册 ────────────────────────────────────────────
// platform 为渠道类型的十进制字符串（与 relay.GetTaskAdaptor 的
// strconv 分支约定一致），由 constant.ChannelTypeXxx 转换而来。
var taskAdaptorFactories = map[string]func() channel.TaskAdaptor{}

func RegisterTaskAdaptor(platform string, f func() channel.TaskAdaptor)
func GetTaskAdaptor(platform string) channel.TaskAdaptor // 查不到返回 nil

// ── 数据库迁移注册 ────────────────────────────────────────────────
var migrateModels []any

func RegisterMigrateModels(models ...any)
func ExtraMigrateModels() []any

// ── 路由挂载注册 ──────────────────────────────────────────────────
// 插件自行持有中间件链；extcore 仅负责在 SetRouter 末尾统一调用。
var routeMounters []func(*gin.Engine)

func RegisterRouteMounter(m func(*gin.Engine))
func MountRoutes(router *gin.Engine)

// ── 插件元信息（管理端"已装插件"展示用）──────────────────────────
type PluginInfo struct {
    Name    string `json:"name"`
    Version string `json:"version"`
    Desc    string `json:"desc"`
}

var plugins []PluginInfo

func RegisterPlugin(info PluginInfo)
func Plugins() []PluginInfo
```

注册中心内部使用 `sync.Mutex` 保护（init 顺序与多插件并发注册）。所有 map/slice 读取发生在启动期与请求期，写入只发生在 init 期，锁保护写写冲突即可。

### 2.3 核心补丁清单（全部既有文件编辑点）

| # | 文件 | 锚点 | 改动 | 行数 |
|---|---|---|---|---|
| P1 | `main.go` | import 块 | `_ "github.com/QuantumNous/new-api/zsy/extbootstrap"` | 1 |
| P2 | `model/main.go` | `migrateDB()` `DB.AutoMigrate(...)` 参数列表末尾 | 追加 `extcore.ExtraMigrateModels()...` 展开 | 1 |
| P3 | `router/main.go` | `SetRouter` 中 `SetVideoRouter(router)` 之后 | `extcore.MountRoutes(router)` | 1 |
| P4 | `relay/relay_adaptor.go` | `GetTaskAdaptor` 函数体首部 | 命中 extcore 注册表则优先返回 | 3 |
| P5 | `constant/channel.go` | 渠道类型常量区 / 名称映射 / `ChannelBaseURLs` | 追加 `ChannelTypeRunningHub = 61`（当前最大 `ChannelTypeNewAPI = 60`）。**必须同步追加 `ChannelBaseURLs` 数组项**——轮询路径 `constant.ChannelBaseURLs[ch.Type]`（[task_polling.go#L445](../service/task_polling.go#L445)）按索引直取，漏加将 panic（非显示问题）。无需同步 `ChannelType2APIType`（任务 adaptor 不经它） | 4 |
| P6 | `web/src/hooks/use-sidebar-data.ts` | `navGroups` 组装处 | 追加 `...EXT_MENU_GROUPS` 展开 | 2 |
| P7 | `web/src/features/channels/constants.ts` | `CHANNEL_TYPES` / `CHANNEL_TYPE_DISPLAY_ORDER` | 追加 `...EXT_CHANNEL_TYPES` 展开（类型契约见 §5）；label 为 i18n key，须同步登记 `src/i18n/static-keys.ts`（web/AGENTS.md 3.1） | 2 |
| P8 | `web/src/i18n/index.ts`（i18n 初始化） | resources 组装处 | 追加扩展 locale 合并循环 | 3 |

配套新增（非编辑，随补丁携带的通用骨架文件）：

```
extcore/extcore.go                     # §2.2
zsy/extbootstrap/extbootstrap.go       # blank import 已安装插件
web/src/extensions/index.ts            # 扩展模块声明（名称/版本）
web/src/extensions/menus.tsx           # EXT_MENU_GROUPS（初始为空数组）
web/src/extensions/channel-types.ts    # EXT_CHANNEL_TYPES（初始为空数组）
web/src/extensions/locales/            # 扩展 i18n 资源目录（按插件子目录组织）
```

P4 的具体形态：

```go
func GetTaskAdaptor(platform constant.TaskPlatform) channel.TaskAdaptor {
    if adaptor := extcore.GetTaskAdaptor(string(platform)); adaptor != nil {
        return adaptor
    }
    switch platform {
    // ... 原有 switch 保持不变
```

### 2.4 兼容性处理策略（补丁层面）

- **追加式编辑**：所有编辑点均位于既有列表/switch/数组的"末尾或首部"，上游合并时冲突面最小。
- **锚点校验**：补丁说明中为每处编辑给出唯一上下文片段（如 `case constant.APITypeNewAPI:`），应用前脚本先 grep 校验锚点存在；锚点漂移时人工重放（每处 ≤3 行）。
- **版本策略**：extcore 与插件随当前源码树维护；升级 new-api 后重放 P1–P8（预计全部为机械追加）。若上游重构 `GetTaskAdaptor`/`SetRouter` 签名，则该处适配随版本进行——这是可接受的差异成本。
- **禁改区**：不触碰 `relaykit/`（保持 `GOWORK=off go build ./...` 通过，插件不在 relaykit 内）、不改动计费核心算法、不改既有 DTO 字段语义。

---

## 3. 插件后端设计（`zsy/runninghub/`）

### 3.1 模块划分

```
zsy/runninghub/
├── extension.go        # init() 自注册：TaskAdaptor / MigrateModels / Routes / PluginInfo
├── controller/
│   ├── submit.go       # 用户提交（复制精简自 controller.RelayTask，叠加并发感知选 Key）
│   ├── query.go        # 用户任务查询 / 列表
│   ├── upload.go       # 上传代理（转发 RH media/upload/binary）
│   └── admin.go        # 管理端：应用模板 CRUD / 实例管理 / 连通性测试
├── adaptor/
│   └── task_adaptor.go # 实现 channel.TaskAdaptor（注册到 extcore）
├── keypool/
│   └── pool.go         # 每 Key 并发控制（DB 对账式，见 §3.4）
├── parser/
│   └── curl.go         # curl 示例解析器（URL kind/id + nodeInfoList → schema）
├── schema/
│   ├── validate.go     # 参数校验（类型/边界/必填/未知参数）
│   └── build.go        # schema + 用户参数 → nodeInfoList 请求体
├── model/
│   └── app.go          # RhApp 模板表 GORM 模型与 CRUD
└── router.go           # 路由与中间件链
```

依赖方向：`zsy/*` → `extcore` / `model` / `relay` / `service` / `middleware` / `common`；反向零依赖。

### 3.2 数据模型

**复用 `channels` 表**（实例即渠道）：

- 渠道 `type = ChannelTypeRunningHub`，`base_url` = `https://www.runninghub.cn` 或 `https://www.runninghub.ai`
- `key` = 多 Key，换行分隔（遵循 `Channel.GetKeys()` 约定）
- `models` = 该实例承载的应用模型名列表（由应用模板保存时自动同步）
- 实例级扩展配置存入渠道 `other_settings`（插件读写自有命名空间键，不干扰既有键）：

```json
{ "zsy_runninghub": {
    "max_concurrency_per_key": 2,
    "webhook_secret": "<32位随机串，实例保存时自动生成>",
    "max_upload_mb": 50
} }
```

（`max_upload_mb` 按实例配置，适配不同 RH 套餐的文件限额；`webhook_secret` 用于 §3.8 的 HMAC 回调校验。）

**新增 `rh_apps` 表**（应用模板）：

```go
type RhApp struct {
    ID             int    `json:"id" gorm:"primaryKey"` // GORM v2 整数主键三库默认自增；不写 AUTO_INCREMENT tag（AGENTS.md 约定；核心表历史惯例不在本插件范围）
    ModelName      string `json:"model_name" gorm:"type:varchar(191);uniqueIndex"` // 如 rh-aiapp-1975951975441412098
    Kind           string `json:"kind" gorm:"type:varchar(20)"`                    // "ai-app" | "workflow"
    AppID          string `json:"app_id" gorm:"type:varchar(64);index"`            // RH 应用/工作流 ID
    Name           string `json:"name" gorm:"type:varchar(191)"`
    Description    string `json:"description" gorm:"type:text"`
    ParamSchema    string `json:"param_schema" gorm:"type:text"`                   // 见 §3.3 JSON
    InstanceType   string `json:"instance_type" gorm:"type:varchar(20)"`           // default/plus，空为不传
    UsePersonalQueue string `json:"use_personal_queue" gorm:"type:varchar(10)"`    // "true"/"false"/""（透传原样）
    Enabled        bool   `json:"enabled"`
    ChannelID      int    `json:"channel_id" gorm:"index"`                         // 绑定实例（渠道）
    CreatedAt      int64  `json:"created_at" gorm:"index"`
    UpdatedAt      int64  `json:"updated_at"`
}
```

- JSON 列用 `text`（三库通用）；`bool` 不设 gorm default tag，由代码归一化（遵循项目 DB 兼容规则）。
- **绑定语义（N:1）**：一个应用模板归属一个渠道（`ChannelID` 为管理归属字段）；保存时把 `ModelName` 同步进该渠道 `models`。允许管理员手动把同一 model 名复制到其他 RunningHub 渠道实现实例级负载均衡（`Distribute` 原生支持）；运行时 schema 一律按 `ModelName` 全局唯一查询，不依赖 `ChannelID`。
- 任务本身复用核心 `tasks` 表（`platform = strconv(ChannelTypeRunningHub)`），插件控制器写入 `PrivateData.Key`（提交所用 Key），轮询时核心自动用它查询（[task_polling.go#L456-L461](../service/task_polling.go#L456-L461)），天然支持多 RH 账号 Key 混用。

**ParamSchema JSON 契约**（curl 解析产物，管理员可编辑）：

```json
{
  "params": [
    { "nodeId": "299", "name": "reference_image", "label": "参考图", "type": "image",
      "field": "image", "required": true },
    { "nodeId": "275", "name": "reference_video", "label": "参考视频", "type": "video",
      "field": "video", "required": true },
    { "nodeId": "293", "name": "posture_method", "label": "姿态计算方法", "type": "select",
      "field": "select", "default": "1",
      "options": [ { "value": "1", "label": "方法一" }, { "value": "2", "label": "方法二" } ] },
    { "nodeId": "297", "name": "intensity", "label": "姿态强度", "type": "number",
      "field": "value", "default": "1.0", "min": 0, "max": 2 },
    { "nodeId": "497", "name": "neck_length", "label": "颈部延长", "type": "boolean",
      "field": "value", "default": "false" }
  ],
  "billing": { "mode": "per_call" }
}
```

- `type ∈ {string, text, number, boolean, select, image, video}`；`field` 保留 RH 原始 `fieldName` 原样透传。
- `fieldValue` 一律以字符串保存/提交（RH 示例中 `"usePersonalQueue": "false"` 为字符串、数值带浮点尾巴 `1.0000000000000002`），解析与提交均**不做数值清洗**，仅做校验。

### 3.3 curl 示例解析器（`parser/curl.go`）

输入：管理员粘贴的完整 curl 命令。处理步骤：

1. 提取 URL：`/openapi/v2/run/ai-app/{id}` → `kind=ai-app`；`/run/workflow/{id}` → `kind=workflow`（兼容 `.cn` / `.ai` 域名）。
2. 提取 `--data-raw`（含单双引号包裹的转义变体）JSON 体。
3. `nodeInfoList[]` → 参数项：`description` → `label`（空则用 `fieldName+nodeId`）、`fieldName` → `field` 与类型推断（`select→select`、`image→image`、`video→video`、`value` 按样本值推断 `boolean`/`number`/`string`）、样本值 → `default`。
4. `instanceType` / `usePersonalQueue` 原样保留。
5. 输出 schema 预览（管理端可编辑后再保存，保存时经 `schema/validate.go` 校验 schema 自身合法性：nodeId 唯一、name 唯一且为合法标识符、select 必须有 options、number 的 min/max 合法）。

**支持的 curl 变体清单**：

| 类别 | 支持 | 说明 |
|---|---|---|
| 请求方式 | `--request POST` / `-X POST` / 省略 | 省略时按 POST 处理 |
| 数据体 | `--data-raw` / `--data` / `-d` | 三者等价 |
| 头部 | `--header` / `-H`（多个） | `Authorization` 仅识别不保存 |
| 引号 | 单引号、双引号、无引号 | 含 JSON 体内嵌套引号 |
| 续行/转义 | `\`（bash）、`^` 与 `\"`（cmd）、反引号（PowerShell） | |
| **明确不支持** | `-F` multipart、`@file` 文件引用 | 报错并提示直接粘贴 JSON |
| 变量插值 | `${VAR}` 原样保留并标记告警 | 由管理员在预览界面手工替换 |

解析失败一律返回结构化错误，并允许管理员在预览界面手工粘贴 JSON 兜底（定位为"尽力解析 + 人工修正"）。

### 3.4 Key 并发池（`keypool/pool.go`）

**对账式设计**（避免进程重启丢计数导致并发泄漏）：

- 有效占用 = `DB 中该渠道该 Key 的未完成任务数` + `提交中窗口 pending 集合`（提交开始到 `tasks` 行落库之间的短暂窗口，内存 set）。
- 后台每 30s 及关键事件点（任务落库、轮询发现终态）重算 `map[channelID+keyIdx]int`。
- `Acquire(ch)`：按 `GetNextEnabledKey()` 顺序遍历启用 Key，取第一个 `占用 < max_concurrency_per_key` 的 Key；全部打满返回 429 语义错误（"所有 Key 并发已满，请稍后再试"）。
- 单实例部署用内存实现；多节点部署时对账数据来自共享 DB，天然收敛（重算周期内可能短暂超发 1 次，属可接受误差，文档标注）。
- 轮询发现任务终态后无需显式释放——下一轮对账自动回落。

**崩溃恢复与幽灵任务窗口**：

- **并发计数恢复**：对账数据全部来自 DB（未完成任务按渠道+Key 分组重算），进程重启后最多一个对账周期（30s）自动收敛，无持久化状态可丢失，无人工干预。
- **提交窗口幽灵任务**（既有架构固有，非插件引入）：请求已达 RH（RH 侧开始运行并扣 RH 币）而 new-api 在落库前崩溃时，预扣费已发生但无任务行、无轮询与退款路径。该窗口为核心 `RelayTask` 流程固有（所有任务平台共有，窗口为毫秒级），插件不做结构性改造；缓解措施：提交前写一条审计日志（时间/用户/模型/参数摘要），供管理员与 RH 后台对账，文档标注该残余风险。

### 3.5 提交链路（`controller/submit.go`）

路由链（与 `/suno` 任务路由同级强度）：

```go
group := router.Group("/zsy/rh/v1")
group.Use(middleware.RouteTag("relay"))
group.Use(middleware.SystemPerformanceCheck())
group.Use(middleware.TokenAuth())
group.Use(middleware.ModelRequestRateLimit())
group.Use(middleware.Distribute()) // 请求体含 "model" 字段，由 Distribute 按模型选渠道
```

`POST /zsy/rh/v1/submit` 处理流程（在核心 `controller.RelayTask` 基础上做两处增强，其余流程一致）：

```text
1. 绑定请求体：{ model, params: {name: value}, instance_type?, use_personal_queue?, webhook_url? }
2. 校验 model → 查 RhApp（必须 enabled）；校验 params（schema/validate.go，全部拒绝式 400）
3. 解析图片/视频参数：URL / base64 data URI / 本地上传引用（见 §3.6）
4. 并发感知选 Key：
   ch := 渠道（Distribute 已选定；重试时 GetRandomSatisfiedChannel + SetupContextForSelectedChannel）
   key, release := keypool.Acquire(ch)          // 覆盖 Distribute 给出的默认 Key
   common.SetContextKey(c, constant.ContextKeyChannelKey, key)
5. 构建 RH 请求体：schema + params → nodeInfoList + instanceType/usePersonalQueue/webhookUrl
   存入 gin context（adaptor.BuildRequestBody 直接取用）
6. relay.RelayTaskSubmit(c, relayInfo)         // 预扣费→提交→AdjustBillingOnSubmit，全部原生
7. defer：提交失败 release（pending 集合移除）
8. 成功：SettleBilling → LogTaskConsumption → InitTask 落库
   task.PrivateData.Key = key                  // 轮询用同 Key 查询（多账号安全）
9. 响应：{ task_id, model, status }
```

重试策略（自实现简化版 `shouldRetry`）：429/5xx 且未超 `common.RetryTimes` → 换渠道重试；4xx/本地错误 → 终止。每次重试重新 `keypool.Acquire`。

**计费语义**：应用绑定模型名在"模型价格"表中配置按次价格 → `ModelPriceHelperPerCall` 得 `UsePrice=true` → `PerCallBilling=true`（[relay.go#L593-L600](../controller/relay.go#L593-L600)）→ 轮询阶段自动跳过差额结算；任务失败走 `RefundTaskQuota` 全额退款；超时清扫同样退款。整条链无插件侧算术，无负扣费风险面；v1.5 如引入参数倍率，必须走 `PriceData.AddOtherRatio`（核心已拒绝非正/NaN/Inf）并在 schema 中声明数值边界。

### 3.6 上传代理（`controller/upload.go`）

- `POST /zsy/rh/v1/upload`（TokenAuth）：multipart 单文件，大小上限取实例配置 `max_upload_mb`（默认 50，见 §3.2）。
- 转发至 `{base_url}/openapi/v2/media/upload/binary`，`Authorization: Bearer {key}`（Key 从绑定渠道轮询选取，不占并发额度——查询/上传类轻请求）。
- 返回 `{ file_name, url }`；`file_name`（如 `openapi/xxxx.png`）可直接作为 image/video 参数值提交。
- 仅允许转发到渠道配置的 RH 域名（SSRF 防护：目标固定来自渠道 `base_url`，不接受用户传 URL）。

### 3.7 Adaptor 实现（`adaptor/task_adaptor.go`）

`channel.TaskAdaptor` 逐方法实现（对照 [adapter.go#L35-L80](../relay/channel/adapter.go#L35-L80)）：

| 方法 | 实现 |
|---|---|
| `Init` | no-op |
| `ValidateRequestAndSetAction` | 从 context 取 RhApp，设置 `info.Action = app.Kind`；兜底校验请求体存在 |
| `EstimateBilling` | 返回 nil（纯按次，无 OtherRatios） |
| `AdjustBillingOnSubmit` | 返回 nil |
| `AdjustBillingOnComplete` | 返回 0（PerCallBilling 已跳过，此为兜底） |
| `BuildRequestURL` | `{base_url}/openapi/v2/run/ai-app/{appID}` 或 `/openapi/v2/run/workflow/{appID}`（按 kind；路径已由用户示例佐证） |
| `BuildRequestHeader` | `Authorization: Bearer {ContextKeyChannelKey}`、`Content-Type: application/json` |
| `BuildRequestBody` | 从 gin context 取 §3.5 第 5 步构建好的请求体（`common.Marshal` 产物） |
| `DoRequest` | 标准 POST（支持渠道代理设置），复用 `relay/channel/api_request.go` 既有请求器 |
| `DoResponse` | 解析 `{ taskId, status, errorCode, errorMessage }`（兼容 `taskId`/`task_id` 两种命名）；返回 `upstreamTaskID=taskId`，`taskData=原始 body` |
| `GetModelList` | 查询 rh_apps 表 enabled 的 model_name 列表 |
| `GetChannelName` | `"RunningHub"` |
| `FetchTask(baseURL, key, body, proxy)` | `POST {base_url}/openapi/v2/task/{task_id}/status`，body 含 `task_id`/`action`；Bearer 用传入 key（核心已保证为任务提交时的 Key）。**查询端点为待实测假设（阻塞项），见 §3.9** |
| `ParseTaskResult` | RH 状态映射：`QUEUED→QUEUED`、`RUNNING→IN_PROGRESS`、`SUCCESS→SUCCESS`（`Url=results[0].url`）、`FAILED→FAILURE`（`Reason=errorMessage`）；完整 results 保留在响应 body 中（核心轮询将原始 body 存入 `task.Data`，多文件结果随 `Data` 透出） |

### 3.8 Webhook（可选增强，v1 保留路由桩）

`POST /zsy/rh/webhook/:task_id`：校验 `?token=`（`HMAC(task_id, 该实例的 webhook_secret)`，secret 来源见 §3.2 `other_settings.zsy_runninghub.webhook_secret`，实例保存时自动生成；**存量兜底：保存时检测到已有实例该字段为空（旧数据/手动改库）则补生成**，避免回调校验失效。提交时把拼好的 `webhookUrl` 随请求体发给 RH）；命中后仅记录日志并触发一次任务状态刷新请求提示。v1 状态流转完全依赖既有轮询（系统任务驱动，默认分钟级），webhook 作为提速增强项后置。

### 3.9 RH 端点实测（编码前置，阻塞项）

| 端点 | 验证状态 | 来源 |
|---|---|---|
| 提交 `/openapi/v2/run/ai-app/{id}`、`/run/workflow/{id}` | 已验证 | 用户提供的示例 curl |
| 上传 `/openapi/v2/media/upload/binary` | 已验证 | 官方文档 |
| 查询任务状态（当前假设 `POST /openapi/v2/task/{task_id}/status`） | **未验证，阻塞轮询链路** | 文档页查询 curl 抓取为空，需实测 |

- **cn/ai 双域名**路径前缀可能不一致（上传文档用 cn 域、用户示例为 ai 域），实测须双域名覆盖；若不一致，则将 base path 做成实例配置项（默认随域名推导）。
- 实测内容：提交 → 查询全流程、状态字段枚举与 `taskId`/`task_id` 命名、`results[]` 结构、FAILED 错误样例（如 `errorCode 1501` 内容审核）。
- 实测结论回写本节，并作为 adaptor 常量与 §9.1 单测 golden 数据的唯一依据。

---

## 4. API 接口定义

### 4.1 用户端（TokenAuth）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/zsy/rh/v1/apps` | 可用应用列表（含 ParamSchema，供动态表单渲染） |
| POST | `/zsy/rh/v1/submit` | 提交任务（请求体见 §3.5） |
| GET | `/zsy/rh/v1/task/:task_id` | 单任务状态 + 结果（从 tasks 表 + task.Data 解析 results[] 全量输出） |
| GET | `/zsy/rh/v1/tasks?page=&status=` | 本人任务分页 |
| POST | `/zsy/rh/v1/upload` | 文件上传代理 |

submit 请求/响应示例：

```json
// POST /zsy/rh/v1/submit
{ "model": "rh-aiapp-1975951975441412098",
  "params": { "reference_image": "https://... 或 openapi/xxxx.png",
              "reference_video": "openapi/yyyy.mp4",
              "intensity": "1.5", "posture_method": "2" },
  "instance_type": "default" }

// 200
{ "task_id": "task_xxxxxxxxxxxxxxxx", "model": "rh-aiapp-...", "status": "SUBMITTED" }
// 错误（余额不足/校验失败/并发满）复用 taskdto.TaskError 结构与状态码
```

### 4.2 管理端（AdminAuth，挂载在 `/zsy/rh/admin`）

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST/PUT/DELETE | `/apps` | 应用模板 CRUD（保存时同步渠道 models 列表 + 模型价格按次配置） |
| POST | `/apps/parse-curl` | 粘贴 curl → 返回解析预览（不落库） |
| GET | `/instances` | 渠道类型=RunningHub 的实例列表（Key 脱敏展示） |
| PUT | `/instances/:id` | 更新 base_url / Keys / max_concurrency_per_key |
| GET | `/instances/:id/stats` | 每 Key 当前占用 / 上限 / 未完成任务数 |
| POST | `/instances/:id/test` | 连通性测试（调 RH 余额/用户信息接口，校验每个 Key 有效性） |

### 4.3 Webhook

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/zsy/rh/webhook/:task_id?token=` | RH TASK_END 回调接收（HMAC 校验） |

---

## 5. 前端设计（`web/src/extensions/zsy-runninghub/`）

```
web/src/extensions/zsy-runninghub/
├── index.ts              # 模块声明：向 extensions/menus.tsx、channel-types.ts 贡献条目
├── api.ts                # 上述 API 的类型化客户端
├── locales/{en,zh,zh-TW,fr,ru,ja,vi}.json   # 插件词条（由 i18n 初始化合并，见 P8）
├── components/
│   ├── ParamForm.tsx     # ★ 动态表单渲染器（schema → 控件）
│   ├── AppCard.tsx
│   └── TaskResult.tsx    # results[] 渲染：png/img 预览、mp4 播放器、text 展示、24h 过期提示
└── pages/
    ├── admin-apps.tsx    # 应用管理：列表 + curl 解析器 + schema 编辑表格 + 定价
    ├── admin-instances.tsx # 实例管理：Key 列表（并发占用进度条）+ 上限设置 + 测试
    ├── user-apps.tsx     # 用户端应用广场
    ├── user-run.tsx      # 应用运行页（ParamForm + 提交 + 进度轮询）
    └── user-tasks.tsx    # 任务记录
```

**扩展注入类型契约**（`web/src/extensions/` 导出，供 P6/P7 消费）：

```ts
// menus.tsx —— 与 web/src/hooks/use-sidebar-data.ts 的 NavGroup/NavItem 结构对齐
// （实施时以现有类型定义为准，此处为契约形状）
export interface ExtMenuItem {
  title: string // i18n key
  url: string
  icon?: React.ComponentType
}
export interface ExtMenuGroup {
  title: string
  items: ExtMenuItem[]
}
export const EXT_MENU_GROUPS: ExtMenuGroup[]

// channel-types.ts —— 与 features/channels/constants.ts 实际结构对齐：
// CHANNEL_TYPES 为 Record<number, string> 字典（label 即 i18n key），
// 另有 CHANNEL_TYPE_DISPLAY_ORDER: number[] 控制下拉顺序
export interface ExtChannelTypeContribution {
  types: Record<number, string> // 例：{ 61: 'RunningHub' }
  displayOrder: number[]        // 追加至 CHANNEL_TYPE_DISPLAY_ORDER 尾部
}
export const EXT_CHANNEL_TYPES: ExtChannelTypeContribution
```

扩展条目的 label（i18n key）必须同步登记 `src/i18n/static-keys.ts`，确保键扫描不遗漏（web/AGENTS.md 3.1）。

**动态表单渲染器（用户端工作量核心）**：`type → 控件` 映射：

| type | 控件 | 值处理 |
|---|---|---|
| string | Input | 原样字符串 |
| text | Textarea | 原样 |
| number | InputNumber | 输出十进制字符串，尊重 min/max/step |
| boolean | Switch | 输出 `"true"/"false"` |
| select | Select | options 单选 |
| image | 上传组件 + URL 输入二合一 | 先调 `/zsy/rh/v1/upload`，回填 `file_name`；或直接填公网 URL/base64 |
| video | 同上（视频） | 同上 |

路由：`routes/_authenticated/extensions/zsy-runninghub/...`（TanStack file-based，新增文件即注册，无核心编辑）；侧边栏条目经 `extensions/menus.tsx` 贡献（管理端一组、用户端一组）；所有用户可见文案走 `t('English key')`，词条进插件 locales。

---

## 6. 技术选型

| 层 | 选型 | 说明 |
|---|---|---|
| 后端语言/框架 | Go 1.22+ / Gin | 与主程序一致，同进程部署（无 IPC） |
| 扩展机制 | 编译期注册（init + 注册表） | Go 无运行时插件生态；`plugin` 包仅 Linux 且强版本耦合，排除 |
| ORM/DB | GORM v2，三库兼容 | JSON 用 `text`；索引字段 `varchar(191)` |
| 并发池 | 内存 + DB 对账 | 不引入 Redis 硬依赖；Redis 存在时可平滑升级为共享计数（预留接口） |
| 上游 HTTP | 复用 `relay/channel/api_request.go` 请求器 | 统一代理/超时/重定向行为 |
| JSON | `common.Marshal/Unmarshal` 全量 | 遵循项目 JSON 包装规范 |
| 前端 | React 19 + Base UI + Tailwind + TanStack Router | 与 web/ 现状一致 |
| 前端构建/包管理 | Bun | `bun install` / `bun run build` / `bun run i18n:sync` |

---

## 7. 开发规范与质量标准

### 7.1 规范（对齐 AGENTS.md）

- JSON 序列化/反序列化一律 `common.*` 包装函数；`json.RawMessage` 仅作类型引用。
- DTO：从用户 JSON 解析并转发上游的可选标量字段用指针 + `omitempty`；本方案中 `fieldValue`/`usePersonalQueue` 等以字符串原样透传，不做类型转换，规避零值丢失问题。
- 数量/倍率边界：任何进入计费的数值必须先有界（v1 为纯按次，无乘数；v1.5 参数倍率必须在 schema 声明 min/max 且经 `AddOtherRatio` 防护）；额度换算只用 `common/quota_math.go` 助手，禁止裸 cast。
- 数据库：仅 GORM 方法；新表进 `extcore` 迁移注册；不加 `gorm:"default:true"` 类布尔默认；不用方言特性。
- 单一职责：curl 解析、schema 校验、keypool 均为独立稳定业务概念，允许成包；包内禁止只有一个调用者的机械小函数。
- 后端测试：`testify/require`（致命断言）+ `assert`（非致命）；表驱动、确定性，禁止随机/睡眠式伪测试。
- 前端：TypeScript 严格模式；组件 ≤ 200 行拆分；无障碍（label 关联、键盘可达）；文案全量 i18n。
- 保护项：不改动任何 new-api / QuantumNous 品牌与署名信息。

### 7.2 质量标准（验收口径）

- `go build ./...`（根模块）与 `cd relaykit && GOWORK=off go build ./...` 双绿。
- `go vet ./...`、`gofmt` 无告警；`bun run build` 成功；`bun run i18n:sync` 无缺失键。
- 三库（SQLite/MySQL 5.7.8+/PostgreSQL 9.6+）迁移通过（CI 覆盖 SQLite，MySQL/PG 手工验证记录）。
- 计费不变量验收：提交前余额不足 → 4xx 且无扣费；提交成功 → 预扣；失败/超时 → 全额退款；按次计费无轮询差额；日志含任务消费记录。
- Key 泄漏验收：所有面向用户的响应（含 task.Data 解析输出）不含 Key/渠道信息；`PrivateData` 不出 admin 之外的视图。

---

## 8. 扩展性保障：新平台接入指引

extcore 让"接入下一个类似平台"收敛为固定四步（核心补丁零改动）：

1. **渠道类型注册**（唯一的核心文件编辑，属平台安装补丁）：`constant/channel.go` 追加类型常量、显示名、`ChannelBaseURLs` 默认地址；前端 `extensions/channel-types.ts` 追加展示条目。
2. **实现 `channel.TaskAdaptor`**（新包，如 `zsy/xxx/adaptor/`），在 `extension.go` 中 `extcore.RegisterTaskAdaptor(strconv.Itoa(constant.ChannelTypeXxx), ...)`。
3. **平台特有配置**：优先存渠道 `other_settings` 自有命名空间键；确需新表则 `extcore.RegisterMigrateModels`。
4. **前端**：在 `web/src/extensions/<plugin>/` 提供页面与表单渲染器（如 schema 结构不同，渲染器可完全独立）。

runninghub 与未来平台共享的部分（提交控制器骨架、keypool、webhook 桩、任务查询 API）在 `zsy/runninghub` 内保持可抽取状态；当出现第二个平台时再上提为 `zsy/common`，避免过早抽象。

---

## 9. 测试策略

### 9.1 单元测试（后端，testify + 表驱动）

| 目标 | 覆盖点 |
|---|---|
| `parser/curl.go` | 以用户提供的 ai-app 与 workflow 两条真实示例为 golden case（断言解析出的 kind/id/schema 全量字段）；覆盖 §3.3 变体清单（`-d`/`--data`/`-X`、cmd `^` 与 `\"`、PowerShell 反引号、无引号、`${VAR}` 告警）；`-F`/`@file` 拒绝；缺 --data-raw、URL 非法、JSON 损坏的报错路径 |
| `schema/validate.go` | 类型越界（number 超 min/max）、缺失必填、未知参数名拒绝、select 值不在 options、超长 fieldValue（>8192 拒绝）、schema 自身非法（nodeId 重复等） |
| `schema/build.go` | schema+params → nodeInfoList 的精确 JSON 输出；字符串数值原样透传（`"1.0000000000000002"` 不变形）；boolean→`"false"` 字符串 |
| `keypool/pool.go` | 首个 Key 打满后自动落到下一个；全部打满返回可识别 429 语义；DB 对账重算后占用回落；pending 窗口计数正确 |
| `adaptor` | `DoResponse` 兼容 `taskId`/`task_id` 与错误体；`ParseTaskResult` 五状态映射（含 FAILED `errorCode 1501` 内容审核样例）；`BuildRequestURL` 双 kind |
| extcore | 注册/查重/未注册返回 nil；迁移列表聚合顺序稳定 |
| 渠道类型注册 | `len(ChannelBaseURLs) > ChannelTypeRunningHub`（防索引越界回归断言）；名称映射、前端 `CHANNEL_TYPES[61]` 与 `displayOrder` 含 61 |

### 9.2 集成测试（httptest 伪 RH 上游）

- 全链路：TokenAuth 用户 → submit（预扣费断言）→ 伪上游返回 RUNNING → 伪轮询返回 SUCCESS → task.Data 含完整 results、PerCallBilling 不触发差额 → 结果 API 输出 URL 列表。
- 失败退款链：提交成功后伪上游返回 FAILED → 轮询置 FAILURE → 断言 `RefundTaskQuota` 恢复余额、CAS 幂等（重复轮询不重复退款）。
- 超时链：构造过期未完成任务 → sweep 置失败并退款（非 legacy）。
- 并发链：max_concurrency_per_key=1、2 个 Key → 第 3 个并发请求命中第二个 Key；3 个全满 → 429；任务终态后再次提交成功。
- 上传代理：伪 RH 校验 Authorization 与 multipart 透传完整性；超限文件 4xx。
- 迁移：SQLite 内存库 `migrateDB` 全量通过（含 rh_apps）。

### 9.3 前端测试与验证

- `bun run build` 必过；`bun run i18n:sync` 检查 7 语言键完整。
- ParamForm 渲染器：按 schema 快照断言控件类型与默认值；提交前值归一化（number→字符串）单测。
- 手工验收清单：管理端从粘贴 curl 到出应用 ≤ 3 步；用户端从选应用到拿到结果 ≤ 3 步；结果页含 24h 过期提示。

---

## 10. 开发顺序（依赖序，非排期）

0. **RH 端点实测**（§3.9：提交/上传/查询 × cn/ai 双域名），结论回写方案——编码前置阻塞项
1. extcore + 8 处补丁 + `extbootstrap`（补丁可重放性先验证）
2. 渠道类型注册 + adaptor 提交链（伪上游集成测试先行）
3. keypool + 提交控制器（并发语义测试）
4. 轮询联调（真实/伪 RH 双向）+ 退款链验证
5. 管理端（curl 解析器 → 应用 CRUD → 实例管理）
6. 用户端（apps 列表 → ParamForm → 任务页 → 上传）
7. webhook 桩、i18n 全量、三库迁移验证、文档收尾
