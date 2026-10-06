# zsy-tone 文风广场 接口文档

> 配套阅读：[`zsy-tone-standard.md`](./zsy-tone-standard.md) —— **文风标准**，
> 定义取值有哪些、各自什么意思、怎么演进。本文只讲**怎么调**。
>
> 同为「公共数据」的另两份：[`zsy-voiceplaza-api.md`](./zsy-voiceplaza-api.md)（音色）、
> [`zsy-avatarplaza-api.md`](./zsy-avatarplaza-api.md)（形象）。

---

## 1. 一句话说明

**文风广场** = 一份后台维护的**写作风格目录** + 一个**可分页的公开列表接口**
+ 一个**版本化的公开标准接口**。

一条文风 = 名称 + 简介 + **提示词（写作指令正文）** + 类别 + 语气 + 语言
+ 适合场景标签 + **示例对照（原文 / 改写）** + 上架状态 + 排序。

★ 与音色 / 形象最大的结构差别：**文风没有媒体文件** ——
所以这个插件**没有上传接口**，也不占任何上传目录。
`routes_test.go` 里那条路由表断言把"没有 upload"也钉住了。

| 面相 | 路径前缀 | 鉴权 |
|---|---|---|
| 公开（给第三方应用） | `/api/zsy/tone` | 无 |
| 后台（给管理员） | `/dashboard/zsy/tone` | `AdminAuth` |

---

## 2. 核心接口（对外）

### 2.1 `GET /api/zsy/tone/list` —— 分页文风列表

只返回**已上架**（`enabled = true`）的文风。
⚠ `enabled` 查询参数在这一面上**被忽略**（它属于后台面）。

**查询参数**

| 参数 | 说明 |
|---|---|
| `keyword` | 模糊匹配：名称 / 简介 / **提示词** / 类别 / 语气 / 语言 / 场景 / **示例原文** / **示例改写** |
| `category` | 类别，见标准 §3（大小写不敏感） |
| `tone` | 语气，见标准 §4（大小写不敏感） |
| `language` | 语言短标签（大小写不敏感） |
| `page` / `p` | 页码，默认 1；小于 1 归 1 |
| `page_size` / `size` | 每页条数，默认 20，最大 100 |

★ `keyword` 覆盖 **prompt 与示例**是刻意的差异（音色那边只搜命名字段）：
操作员找一个"写得挺克制的"文风时，记得住的是**指令里的那句话**或**示例里的那个细节**，
不是它的名字。只搜名字，等于让表里最宽的两列成为唯二搜不到的列。

```http
GET /api/zsy/tone/list?category=literary&tone=calm&page=1&page_size=20
```

```json
{
  "success": true,
  "data": {
    "items": [
      {
        "id": 1,
        "createdAt": 1791000000,
        "updatedAt": 1791000000,
        "name": "克制的长文",
        "description": "把情绪收着写，让细节自己说话。适合公众号长文与散文。",
        "prompt": "以克制、平视的笔调写作：多用短句，少用形容词……",
        "category": "literary",
        "tone": "calm",
        "language": "zh",
        "scenes": ["公众号长文", "散文", "随笔"],
        "sampleInput": "老屋在村子的东头，是祖父留下的。……",
        "sampleOutput": "老屋在东头。祖父留下的。……",
        "enabled": true,
        "sortOrder": 0
      }
    ],
    "total": 1,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

**排序**：`sortOrder` 升序，并列时按 `id` 升序（保证翻页稳定）。

### 2.2 `GET /api/zsy/tone/:id` —— 单条文风

返回一条**已上架**的文风。

⚠ 一条**已下架**的文风返回 **404**，与"不存在"完全一样
（HTTP 状态与 `code` 都相同），所以公开面永远不会泄露未发布的草稿。

```http
GET /api/zsy/tone/1
```

### 2.3 `GET /api/zsy/tone/standard` —— ★ 文风标准

返回**文风标准**的机器可读版本：词表 + 上限 + 兼容性承诺 + 示例约定。

```http
GET /api/zsy/tone/standard
```

```json
{
  "success": true,
  "data": {
    "version": "1.0.0",
    "categories": [
      { "value": "literary", "label": "文学", "labelEn": "Literary",
        "desc": "小说、散文、随笔等以感受与叙事为主的写作" }
    ],
    "tones": [
      { "value": "warm", "label": "温暖", "labelEn": "Warm",
        "desc": "有体温、有体谅，先接住情绪再讲事情" }
    ],
    "limits": {
      "name": 191, "description": 2000, "prompt": 8000, "sample": 4000,
      "language": 16, "scenes": 8, "sceneLength": 24,
      "sortOrderAbs": 1000000, "maxPageSize": 100
    },
    "compatibility": [
      "已发布的取值不会改名、不会删除",
      "可以新增取值，新增不属于破坏性变更",
      "空值始终表示「未分类 / 未指定」",
      "客户端必须容忍不认识的取值：照原样显示，不要丢弃该条目",
      "版本号只在形状变化时前进（新增字段或调整上限），新增取值不前进"
    ],
    "examplePair": {
      "field": "sampleInput / sampleOutput",
      "convention": "同一次维护里，多条文风共用同一段 sampleInput，各自的 sampleOutput 是它按本文风改写后的样子",
      "whyItMatters": "文风既不能像音色那样试听、也不能像形象那样看脸；共用同一段原文，横向比较才成立，而这份比较是存下来的、不花钱"
    }
  }
}
```

★ **这是公开面里唯一可缓存的一个**：响应带
`Cache-Control: public, max-age=3600`（其余公开接口都是 `no-store`）。
理由：它的内容只在发新版本时才变，而目录是可变数据 ——
一个可缓存的目录会把已下架的文风继续发给别人。

★ **客户端应当拉它、而不是把词表抄一份。** 理由与实测代价见标准 §7.1。

---

## 3. 后台管理接口

全部需要管理员会话（`AdminAuth`），全部 `Cache-Control: no-store`
（目录是可变的：另一处刚导入过的行不该被缓存挡住）。

| 方法 | 路径 | 说明 |
|---|---|---|
| `GET` | `/dashboard/zsy/tone/list` | 分页列表，**含未上架**，支持 `enabled` 过滤 |
| `POST` | `/dashboard/zsy/tone` | 新建 |
| `GET` | `/dashboard/zsy/tone/:id` | 详情（含未上架） |
| `PUT` | `/dashboard/zsy/tone/:id` | 部分更新 |
| `DELETE` | `/dashboard/zsy/tone/:id` | 硬删除 |
| `GET` | `/dashboard/zsy/tone/export` | CSV 导出（跟随当前筛选） |
| `POST` | `/dashboard/zsy/tone/import` | CSV 导入（`?mode=upsert\|create`） |

### 3.1 新建 / 更新

**新建**（`POST`）—— `enabled` / `sortOrder` 省略即用默认（上架、0）：

```json
{
  "name": "克制的长文",
  "description": "把情绪收着写，让细节自己说话。",
  "prompt": "以克制、平视的笔调写作：多用短句，少用形容词……",
  "category": "literary",
  "tone": "calm",
  "language": "zh",
  "scenes": ["公众号长文", "散文"],
  "sampleInput": "老屋在村子的东头……",
  "sampleOutput": "老屋在东头。祖父留下的。……",
  "sortOrder": 0
}
```

- `category` / `tone` 会**归一成小写**（电子表格里写 `Warm` 与接口传 `warm` 存成同一个值，
  否则筛选会静默漏掉那一行）。不在词表里的值被拒绝。
- `scenes` 接受**数组**或**分隔字符串**（`"公众号长文,散文"`，也认 `，` `、` `;` `；` `|`）。
- `prompt` **必填** —— 一条没有指令的文风，调用方发出去就是空的风格。

**更新**（`PUT`）是**部分更新**：

| 传什么 | 结果 |
|---|---|
| 不传某个字段（缺席） | **保留**原值 |
| 传 `""`（显式空串） | **清空**它 |
| 传 `prompt: ""` | ❌ 被拒（`文风提示词不能为空`） |

⚠ 上半条与下半条都重要：一条只改提示词的两列更新**不得**顺手清掉示例与分类。

### 3.2 CSV 导出

```http
GET /dashboard/zsy/tone/export?category=marketing
```

```http
HTTP/1.1 200 OK
Content-Type: text/csv; charset=utf-8
Content-Disposition: attachment; filename="zsy-tones-20261006-120000.csv"
Cache-Control: no-store
```

- 带 **UTF-8 BOM**（否则 Windows 上的 Excel 会按本地代码页读，中文全乱）。
- 导出的是**整个筛选结果**，不是当前那一页。
- 不带筛选时它同时是**导入模板**（空目录导出只有表头）。

**列顺序**（这也是标准 §5 的字段顺序）：

```text
name, description, prompt, category, tone, language, scenes,
sample_input, sample_output, enabled, sort_order
```

### 3.3 CSV 导入

```http
POST /dashboard/zsy/tone/import?mode=upsert
Content-Type: multipart/form-data

file=<CSV>
```

| `mode` | 行为 |
|---|---|
| `upsert`（默认） | 名称不存在就建，存在就**按文件里出现过的列**更新 |
| `create` | 只新建；名称已存在的行报为失败行（绝不意外合并） |

★ **upsert 只覆盖文件里真的有的列**：一份两列的文件（只有 `name` 与 `prompt`）
改的是提示词，**不会**清掉那一行的类别、示例与排序。
（CSV 的每一格都有值，所以"这一列文件里没有"是唯一能表达"别动它"的方式。）

**表头别名**（大小写、空格、`-` `_` 都归一后再匹配）：

| 规范列 | 也认这些写法 |
|---|---|
| `name` | 文风名称、名称、文风名、风格名称 |
| `description` | 简介、介绍、描述、说明 |
| `prompt` | 提示词、文风提示词、写作指令、指令、提示词正文 |
| `category` | 类别、分类、体裁、文种 |
| `tone` | 语气、语调、情绪、调性 |
| `language` | 语言、语言代码 |
| `scenes` | scene、适合场景、场景、适用场景 |
| `sample_input` | 示例原文、原文示例、样例原文、示例输入、原样文 |
| `sample_output` | 示例改写、改写示例、样例改写、示例输出、改写后 |
| `enabled` | 是否上架、上架、启用 |
| `sort_order` | 排序、排序值、顺序 |

- **必需列**：`name`、`prompt`。缺任一列 → 整个文件被拒（猜列映射会导错数据）。
- `enabled` 认 `true/false/1/0/yes/no/是/否/上架/下架/启用/停用/已上架/已下架`。
- 无法识别的列 → 一条 warning，**不影响导入**。
- 单个格子读不出来（比如上架状态写了"也许"）→ **只废掉那一行**，不废整个文件。

**响应**（逐行报告，而不是整份文件成败）：

```json
{
  "success": true,
  "data": {
    "total": 18,
    "created": 18,
    "updated": 0,
    "failed": 0,
    "errors": [],
    "warnings": []
  }
}
```

**示例对照的两条 warning**（标准 §6，只是提示、从不报错）：
- 文件里 `sample_input` 有多个不同写法 → 提醒"标准建议共用同一段原文，便于横向比较"
- 有行缺 `sample_input` → 提醒"缺示例的条目在广场上只能凭名字选"

### 3.4 种子数据与灌库

参考数据：`seedmodel/toneplaza-tones.csv` —— **18 条文风，覆盖全部 7 个类别 × 7 种语气**，
**共用同一段示例原文**（这正是标准 §6 那条约定的示范）。

```bash
go run ./_scripts/toneplaza-seed -dry      # 只解析校验，写库全部回滚
go run ./_scripts/toneplaza-seed           # 按 name upsert（默认）
go run ./_scripts/toneplaza-seed -replace  # 清空重建（在一个事务里，读者看不到空目录）
go run ./_scripts/toneplaza-seed -stats    # 看存量、按词表分布、覆盖情况
```

它走的是**生产的导入路径**（`tone.ParseTonesCSV` → `tone.ImportTones`），
所以表头别名、逐行校验、upsert 语义与 `POST /import` 逐字相同 ——
文件与接口一旦对不上，这条命令会以**和导入按钮一样的方式**失败，
而不是写进一批接口根本收不了的行。

⚠ 它**默认要求词表覆盖完整**（种子是标准的示范），少一个取值就退出非零；
导一份有意精简的目录请加 `-allow-partial`。

也可以直接用后台的**导入**按钮（同一个文件）。

---

## 4. 校验与错误约定

### 4.1 两个面的错误形状不同（有意的）

| 面 | 形状 | 为什么 |
|---|---|---|
| 公开（`/api/…`） | 真的 HTTP 状态码 + 稳定的 `code` | 第三方要按 `code` 分支，不能靠解析中文文案 |
| 后台（`/dashboard/…`） | `200` + `{"success":false,"message":"…"}` | 复用宿主 axios 拦截器与 toast，后台页面一行不用改 |

公开面的稳定错误码：

| `code` | 状态 | 什么时候 |
|---|---|---|
| `INVALID_PARAMS` | 400 | `:id` 不是正整数 |
| `TONE_NOT_FOUND` | 404 | 不存在**或**已下架（两者刻意不区分） |
| `DATABASE_ERROR` | 500 | 存储层失败 |

### 4.2 字段上限

| 字段 | 上限 |
|---|---|
| `name` | 191 字符 |
| `description` | 2000 字符 |
| `prompt` | **8000 字符** |
| `sampleInput` / `sampleOutput` | 各 4000 字符 |
| `language` | 16 字符（`^[a-z][a-z0-9_-]*$`） |
| `scenes` | 最多 8 个，每个 ≤ 24 字符 |
| `sortOrder` | 绝对值 ≤ 1000000 |

★ 这份上限**通过接口发布**（`limits`），客户端应当照着它做表单校验，
而不是在自己的代码里再写一遍 —— 那样两处会分叉，而分叉的表现是
"前端拦住了后端能收的值"（或反过来，提交才失败）。
`standard_test.go` 里有一条测试**拿发布的值去撞真的校验器**，两边不一致会当场红。

### 4.3 分页

- `page < 1` → 1
- `page_size < 1` → 20
- `page_size > 100` → 100

越界是**归一**，不是报错 —— 一个页码填错不该让整页列不出来。

---

## 5. 代码位置

| 层 | 文件 |
|---|---|
| 插件注册（表迁移 + 路由挂载） | `zsy/tone/extension.go` |
| 数据模型与受控词表 | `zsy/tone/models.go` |
| ★ **文风标准**（词表 + 上限 + 兼容性承诺的**权威声明**） | `zsy/tone/standard.go` |
| 存储层（搜索 / 筛选 / CRUD / 校验） | `zsy/tone/store.go` |
| CSV 导入导出 | `zsy/tone/csv.go` |
| 公开控制器（含 `/standard`） | `zsy/tone/controllers_public.go` |
| 后台控制器（含导入导出） | `zsy/tone/controllers_admin.go` |
| 查询参数解析 | `zsy/tone/params.go` |
| 路由表 | `zsy/tone/routes.go` |
| 测试钩子（给单测挂真实 handler 用） | `zsy/tone/test_hooks.go` |
| 测试 | `zsy/tone/*_test.go`（含标准的守卫与种子数据的守卫） |
| 种子数据 | `seedmodel/toneplaza-tones.csv` |
| 灌库命令 | `_scripts/toneplaza-seed/main.go` |
| 前端页面 | `web/src/extensions/zsy-tone/` |
| 数据表 | `zsy_tones` |

⚠ **安装 / 卸载只碰一个核心文件**：`zsy/extbootstrap/extbootstrap.go` 里那行空白导入。
插件不 import 别的插件、核心也不按名字引用插件包
（这是本仓库 `extcore` 那套插件契约，见 `zsy/voice/extension.go` 的说明）。

---

## 6. 与 `zsy-voice` / `zsy-avatar` 的异同

| | voice | avatar | **tone** |
|---|---|---|---|
| 媒体文件 | 示例音频 + 头像图 | 四张形象图 | **无** |
| 上传接口 | 有 | 有 | **没有** |
| 业务主键字段 | `voice_type`（上游要的值） | — | ★ **`prompt`（写作指令正文）** |
| 受控词表 | `gender` / `ageRange` | `gender` / `ageRange` / `race` | ★ **`category` / `tone`** |
| 发布词表接口 | ✗（客户端只能硬编码） | ✗ | ★ **有：`/standard`** |
| 「被感知」的方式 | 试听 | 看脸 | ★ **示例对照（§标准 6）** |
| CSV | 有 | 有 | 有（列不同） |

★ 最后三行是这一份**刻意做得不一样**的地方：
词表发布出去，客户端就不必各存一份；示例对照存下来，浏览广场就不花钱。
两者的理由都写在 [`zsy-tone-standard.md`](./zsy-tone-standard.md) §2.2 / §6.2。
