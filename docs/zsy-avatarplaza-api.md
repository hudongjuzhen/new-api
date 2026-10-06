# zsy-avatar 形象广场 接口文档

> 适用版本：当前 `zsy/runninghub` 分支源码树
> 插件目录：后端 `zsy/avatar/`，前端 `web/src/extensions/zsy-avatar/`
> 数据表：`zsy_avatars`（由 `extcore` 注册进宿主的 AutoMigrate，无需手工建表）
> 关联插件：音色广场 `zsy/voice`（本表按 `voice_type` 引用音色，示例音频取自 `zsy_voices`）

## 1. 一句话说明

形象广场是一个 **后台维护、前台/开放接口读取** 的形象（人设）目录：管理员维护形象的
**封面图与三张参考图（全身照 / 四视图 / 表情图）、名称、简介、性别、年龄段、种族、适合场景**
与 **音色 ID**，第三方通过一个分页接口读取已上架的全部形象；后台支持 **CSV 导入 / 导出**
批量维护。

每个形象最多带 **四张图**，均为可选，字段一一对应：

| 名称 | 接口字段 | 说明 |
| --- | --- | --- |
| 封面图 | `imageUrl` | 形象主图，应用列表/详情展示用（原「图片」字段，字段名未变） |
| 全身照 | `fullBodyUrl` | 全身照 |
| 四视图 | `fourViewUrl` | 四视图（正/侧/背/四分之三视图拼图） |
| 表情图 | `expressionUrl` | 表情图 |

四张图共用同一套校验与上传方式：取值必须是网关自身的 `/uploads/...` 路径或 `http(s)://`
绝对地址，最长 768 字符；校验失败时提示会指出具体是哪一张（如「全身照地址必须是 …」）。

形象与音色的关系：形象只保存 **音色 ID**（即上游 TTS 请求实际发送的 `voice_type`，例如
`zh_female_vv_uranus_bigtts`），**不复制音频文件**。列表/详情每次读取时都会去音色广场
（`zsy_voices`）按 `voice_type` 查出该音色的**名称与示例音频**，因此：

- 音色广场里重新上传示例音频后，所有引用它的形象**立刻**返回新音频，无需改形象数据；
- 音色 ID 在音色广场中不存在时，形象仍然有效可上架，只是 `voiceSampleUrl` 为空、
  `voiceAvailable` 为 `false`（前端会给出「该音色 ID 不在音色广场中」的提示）。

## 2. 核心接口（对外）

| 维度 | 值 |
| --- | --- |
| 方法 / 路径 | `GET /api/zsy/avatar/list` |
| 鉴权 | **无需鉴权**（只返回已上架形象） |
| 分页参数 | `page`（默认 1）、`page_size`（默认 20，最大 100） |
| 兼容别名 | `p` = `page`，`size` = `page_size`（与宿主列表接口一致，二选一即可） |
| 过滤参数 | `keyword`（模糊匹配名称 / 简介 / 性别 / 年龄段 / 种族 / 场景 / 音色 ID）、`gender`（精确，`male`/`female`/`neutral`）、`age_range`（精确，`child`/`teen`/`young`/`middle`/`senior`）、`race`（精确，见下方词表）、`voice_id`（精确，别名 `voiceId`） |
| 备注 | 公开接口忽略 `enabled` 参数，只会返回 `enabled=true` 的形象；受控词表大小写不敏感，中文写法（`女`/`青年`/`亚洲人`）也会被归一化为线上取值 |

```bash
curl -s 'https://<你的网关域名>/api/zsy/avatar/list?page=1&page_size=20'
curl -s 'https://<你的网关域名>/api/zsy/avatar/list?keyword=客服&page=1&page_size=10'
curl -s 'https://<你的网关域名>/api/zsy/avatar/list?voice_id=zh_female_vv_uranus_bigtts'
curl -s 'https://<你的网关域名>/api/zsy/avatar/list?gender=female&age_range=young&race=asian'
```

响应（宿主统一信封 `{success, message, data}`）：

```json
{
  "success": true,
  "message": "",
  "data": {
    "items": [
      {
        "id": 1,
        "createdAt": 1767225600,
        "updatedAt": 1767225600,
        "name": "客服小雨",
        "description": "温柔的客服形象，适合播报与问答",
        "imageUrl": "/uploads/images/202601/3f9c…a1.png",
        "fullBodyUrl": "/uploads/images/202601/8b21…f0.png",
        "fourViewUrl": "/uploads/images/202601/c4d7…92.png",
        "expressionUrl": "/uploads/images/202601/e5a3…71.png",
        "gender": "female",
        "ageRange": "young",
        "race": "asian",
        "scenes": ["客服播报", "有声书"],
        "voiceId": "zh_female_vv_uranus_bigtts",
        "voiceAvailable": true,
        "voiceName": "Vivi 2.0",
        "voiceSampleUrl": "/uploads/voices/202601/a1b2….wav",
        "voiceSampleName": "vivi.wav",
        "enabled": true,
        "sortOrder": 0
      }
    ],
    "total": 12,
    "page": 1,
    "pageSize": 20,
    "totalPages": 1
  }
}
```

字段说明：

| 字段 | 类型 | 说明 |
| --- | --- | --- |
| `items[].id` | number | 形象 ID |
| `items[].name` | string | 形象名称（唯一，最长 191 字符） |
| `items[].description` | string | 简介（最长 2000 字符，可为空） |
| `items[].imageUrl` | string | **封面图**地址：网关自身的 `/uploads/images/...` 或 `http(s)://` 绝对地址，可为空（字段名与原「图片」一致，未做破坏性重命名） |
| `items[].fullBodyUrl` | string | **全身照**地址，格式同 `imageUrl`，可为空 |
| `items[].fourViewUrl` | string | **四视图**地址，格式同 `imageUrl`，可为空 |
| `items[].expressionUrl` | string | **表情图**地址，格式同 `imageUrl`，可为空 |
| `items[].gender` | string | 性别，受控词表：`male` / `female` / `neutral`，空串表示未标注 |
| `items[].ageRange` | string | 年龄段，受控词表：`child`(儿童) / `teen`(少年) / `young`(青年) / `middle`(中年) / `senior`(老年)，空串表示未标注 |
| `items[].race` | string | 种族，受控词表：`asian`(亚洲人) / `black`(黑人) / `white`(白人) / `latino`(拉丁裔) / `middle_eastern`(中东人) / `south_asian`(南亚人) / `mixed`(混血)，空串表示未标注 |
| `items[].scenes` | string[] | 适合场景标签数组（最多 8 个、每个 ≤24 字符）；无场景时为 `[]`，不会是 `null` |
| `items[].voiceId` | string | 音色 ID，即音色广场的 `voice_type`（最长 191 字符，可为空）。**大小写敏感**，原样保存 |
| `items[].voiceAvailable` | boolean | `voiceId` 是否存在于音色广场 |
| `items[].voiceName` | string | 该音色在音色广场中的名称；未收录时为空串 |
| `items[].voiceSampleUrl` | string | **音色示例**：该音色的示例音频地址；音色未收录或该音色无音频时为空串 |
| `items[].voiceSampleName` | string | 该示例音频的原始文件名 |
| `items[].enabled` | boolean | 是否上架（公开接口恒为 `true`） |
| `items[].sortOrder` | number | 广场排序值，越小越靠前，同值按 `id` 升序 |
| `total` / `page` / `pageSize` / `totalPages` | number | 分页元信息；`total=0` 时 `totalPages=0` |

分页约定（与后台列表共用同一套逻辑）：

- `page < 1` → 按 1 处理；`page_size < 1` → 按 20 处理；`page_size > 100` → 按 100 处理；
- 页码超出范围不会报错，返回空 `items`，并原样回显 `page`。

单条详情：

| 方法 / 路径 | 鉴权 | 说明 |
| --- | --- | --- |
| `GET /api/zsy/avatar/:id` | 无需鉴权 | 返回单个已上架形象；未上架或不存在均返回 `404` + `{"success":false,"code":"AVATAR_NOT_FOUND"}` |

> 多形象复用同一音色时，示例音频只存一份（在音色广场），形象侧只返回引用结果。

## 3. 后台管理接口

挂在 `/dashboard/zsy/avatar/**`，由宿主的 `middleware.AdminAuth()` 保护（管理员会话 / 访问令牌）。
后台管理页面：**侧边栏「公共数据」→「形象广场」（仅管理员可见）→ `/avatar-plaza`**
（与「音色广场」同级，见 `docs/zsy-voiceplaza-api.md`）。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/dashboard/zsy/avatar/list` | 分页列表，参数同上，另支持 `enabled=true\|false` 过滤（含未上架形象） |
| `POST` | `/dashboard/zsy/avatar` | 新建形象 |
| `GET` | `/dashboard/zsy/avatar/:id` | 单条详情（含未上架） |
| `PUT` | `/dashboard/zsy/avatar/:id` | 局部更新：只有请求体里出现的字段会被修改，显式传空串可清空（如 `"voiceId": ""` 解除音色关联） |
| `DELETE` | `/dashboard/zsy/avatar/:id` | 物理删除（名称立即可复用；图片文件保留在磁盘上） |
| `GET` | `/dashboard/zsy/avatar/export` | **导出 CSV**（参数与列表一致，导出当前筛选结果的全部行，不分页） |
| `POST` | `/dashboard/zsy/avatar/import` | **导入 CSV**（multipart 字段 `file`，可选 `?mode=upsert\|create`，默认 `upsert`） |

新建请求体（`enabled` 省略时默认 `true`，`sortOrder` 省略时默认 `0`；
`scenes` 既可用 JSON 数组也可用逗号分隔字符串；`gender` / `ageRange` / `race`
大小写不敏感、并接受中文写法；空串表示未标注）：

```bash
curl -X POST 'https://<你的网关域名>/dashboard/zsy/avatar' \
  -H 'Authorization: Bearer <管理员访问令牌>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "客服小雨",
    "description": "温柔的客服形象，适合播报与问答",
    "imageUrl": "/uploads/images/202601/3f9c….png",
    "fullBodyUrl": "/uploads/images/202601/8b21….png",
    "fourViewUrl": "/uploads/images/202601/c4d7….png",
    "expressionUrl": "/uploads/images/202601/e5a3….png",
    "gender": "female",
    "ageRange": "young",
    "race": "asian",
    "scenes": ["客服播报", "有声书"],
    "voiceId": "zh_female_vv_uranus_bigtts",
    "enabled": true,
    "sortOrder": 0
  }'
```

上传图片：后台页面直接复用宿主自带的 **`POST /api/upload/image`**（multipart 字段 `file`，
单文件上限 10MB，`png / jpg / gif / webp / bmp`，开启 OSS 时会上传到对象存储），
返回的 `data.url` 分别填入 `imageUrl` / `fullBodyUrl` / `fourViewUrl` / `expressionUrl` 即可，
四张图各自独立上传、也可只填其中一张。形象接口本身不提供上传端点。

### 3.1 CSV 导出

```bash
curl -s -D - -o avatars.csv \
  'https://<你的网关域名>/dashboard/zsy/avatar/export?race=asian&enabled=true' \
  -H 'Authorization: Bearer <管理员访问令牌>'
# Content-Type: text/csv; charset=utf-8
# Content-Disposition: attachment; filename="zsy-avatars-20260101-120000.csv"
```

- **筛选与列表完全一致**（`keyword` / `gender` / `age_range` / `race` / `voice_id` / `enabled`），
  但导出的是筛选结果**全部行**，不分页；不带筛选即全量导出。
- 文件为 **UTF-8 带 BOM**，Excel 直接双击即可正确显示中文；首行为表头，
  列顺序固定为：
  `name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, voice_sample, enabled, sort_order`
  （四张图排在一起：`image_url` 为封面图，其余三列为全身照 / 四视图 / 表情图）。
- `voice_sample`（音色示例）**只导出不导入**：它是按 `voice_id` 从音色广场实时算出来的，
  导入时该列被忽略，改示例音频请到音色广场改。
- 空库导出只含表头，因此该文件同时可当作 **导入模板**。

### 3.2 CSV 导入

```bash
curl -X POST 'https://<你的网关域名>/dashboard/zsy/avatar/import?mode=upsert' \
  -H 'Authorization: Bearer <管理员访问令牌>' \
  -F 'file=@avatars.csv'
```

- 上传方式：`multipart/form-data`，字段名 `file`；上限 **5MB / 5000 行**。
- `mode` 两种取值：
  - `upsert`（默认）：按 `name` 匹配——已存在则**更新**，不存在则**新建**；
  - `create`：只新建，重名行记为失败行（不会覆盖已有数据）。
- 更新时**只有文件中出现的列会被修改**：例如只导入 `name,image_url` 两列，
  不会清空已存的简介、种族、场景、音色 ID 或另外三张图；某列存在但单元格为空则表示清空该字段。
- 表头既可用英文（上面的列名，大小写/空格/下划线/连字符不敏感，
  `Image URL`、`age-range` 均可），也可用中文：
  `形象名称, 简介, 封面图(或 图片/图片URL/封面/头像), 全身照(或 全身图/全身照片), 四视图(或 4视图/四视图图片), 表情图(或 表情/表情图片/表情包), 性别, 年龄段, 种族(或 人种/族裔), 适合场景(或 场景), 音色ID(或 音色标识/voice_type), 音色示例, 是否上架, 排序`。
  未识别的列会被忽略并在 `warnings` 中列出。`name` 与 `image_url` 为必需列；另外三张图的列可整列省略。
- 单元格取值：
  - `性别`：`male`/`female`/`neutral`，或 `男`/`女`/`中性`；
  - `年龄段`：`child`/`teen`/`young`/`middle`/`senior`，或 `儿童`/`少年`/`青年`/`中年`/`老年`；
  - `种族`：`asian`/`black`/`white`/`latino`/`middle_eastern`/`south_asian`/`mixed`，
    或 `亚洲人`/`黑人`/`白人`/`拉丁裔`/`中东人`/`南亚人`/`混血`；
  - `适合场景`：可用 `,`、`，`、`、`、`;`、`；`、`|` 分隔；
  - `封面图 / 全身照 / 四视图 / 表情图`：`http(s)://` 绝对地址或网关自身的 `/uploads/...` 路径，
    四列规则完全一致；留空表示该图未设置；
  - `音色ID`：原样保存（大小写敏感），无需事先存在于音色广场；
  - `是否上架`：`true/false`、`1/0`、`yes/no`、`是/否`、`上架/下架`、`启用/停用`；
  - `排序` 必须是整数，否则该行报错。
- **逐行处理**：单行出错只记录该行，其余行照常写入。响应示例：

```json
{
  "success": true,
  "message": "",
  "data": {
    "total": 3, "created": 1, "updated": 1, "failed": 1,
    "errors": [{ "row": 4, "name": "坏形象", "message": "非法种族 \"martian\" (可选 asian / black / white / latino / middle_eastern / south_asian / mixed，留空表示未标注)" }],
    "warnings": ["已忽略无法识别的列: 备注列"]
  }
}
```

- `row` 为 CSV 中的记录序号（表头为第 1 行），修正这些行后重新导入即可；
  失败行超过 200 条时只返回前 200 条明细，`failed` 仍为真实总数。
- 结构性错误（文件为空、缺少 `name`/`image_url` 列、行数或体积超限、`mode` 非法）
  会直接返回 `success:false`，此时**不会写入任何数据**。

## 4. 校验与错误约定

后台接口沿用面板信封：HTTP 200 + `{"success":false,"message":"…"}`，`message` 为可直接展示的中文提示；
公开接口使用真实 HTTP 状态码 + 稳定错误码：

| 场景 | 状态码 | `code` |
| --- | --- | --- |
| `:id` 非正整数 | 400 | `INVALID_PARAMS` |
| 形象不存在或未上架 | 404 | `AVATAR_NOT_FOUND` |
| 服务端读取失败 | 500 | `DATABASE_ERROR` |

字段校验（`zsy/avatar/store.go` 的 `validateAvatar`）：

- `name` 必填、去首尾空格、最长 191 字符，全表唯一（重名报「形象名称已存在」）；
- `description` 最长 2000 字符；
- `imageUrl`（封面图）、`fullBodyUrl`（全身照）、`fourViewUrl`（四视图）、`expressionUrl`（表情图）
  均为可选；每个都必须是 `/uploads/...` 相对路径或 `http(s)://` 绝对地址（拒绝 `javascript:`、
  `data:`、`//host` 等），最长 768 字符。校验失败时提示会带上该图的名称，
  例如「全身照地址必须是 /uploads/... 路径或 http(s) 地址」；
- `gender` 仅接受 `male` / `female` / `neutral`，否则报「非法性别」；
- `ageRange` 仅接受 `child` / `teen` / `young` / `middle` / `senior`，否则报「非法年龄段」；
- `race` 仅接受 `asian` / `black` / `white` / `latino` / `middle_eastern` / `south_asian` / `mixed`，否则报「非法种族」；
- `voiceId` 可选、最长 191 字符，不做存在性校验（音色广场未收录也允许保存）；
- `scenes` 最多 8 个、每个 ≤24 字符，存储为去重后的逗号分隔串（总长 ≤255 字符）；
- `sortOrder` 取值区间 `[-1000000, 1000000]`。

## 5. 代码位置

| 内容 | 路径 |
| --- | --- |
| 插件装配（extcore 注册、表迁移、路由挂载） | `zsy/avatar/extension.go` |
| 数据模型与 DTO（含性别/年龄段/种族词表、中文别名、场景解析、四张图的字段与名称） | `zsy/avatar/models.go` |
| 存储层与字段校验 | `zsy/avatar/store.go` |
| 音色广场关联（示例音频解析） | `zsy/avatar/voice_link.go` |
| CSV 导入导出（列别名、逐行校验、导入报告） | `zsy/avatar/csv.go` |
| 公开接口 + 后台接口处理器 | `zsy/avatar/controllers.go` |
| 查询参数解析 | `zsy/avatar/params.go` |
| 路由与中间件 | `zsy/avatar/routes.go` |
| 回归测试 | `zsy/avatar/store_test.go`、`zsy/avatar/csv_test.go`、`zsy/avatar/http_test.go`、`zsy/avatar/routes_test.go` |
| 后台管理页 | `web/src/extensions/zsy-avatar/pages/avatar-plaza-page.tsx` |
| 编辑 / 导入弹窗、图片选择、表格行、接口卡片 | `web/src/extensions/zsy-avatar/components/` |
| 前端接口客户端 | `web/src/extensions/zsy-avatar/api.ts` |
| 属性与表单契约（含四张图的字段表） | `web/src/extensions/zsy-avatar/lib/avatar-fields.ts` |
| 前端测试 | `web/src/extensions/zsy-avatar/lib/__tests__/`、`web/src/extensions/zsy-avatar/pages/__tests__/` |
| 前端路由 | `web/src/routes/_authenticated/avatar-plaza/index.tsx` |
| 侧边栏菜单（公共数据 → 音色广场 / 形象广场） | `web/src/extensions/zsy-voice/index.ts` |
| 一次性 i18n 迁移脚本 | `web/scripts/add-avatar-plaza-i18n-keys.mjs`（形象广场首次接入）、`web/scripts/add-avatar-photo-fields-i18n-keys.mjs`（四张图的标签） |
