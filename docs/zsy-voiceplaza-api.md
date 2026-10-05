# zsy-voice 音色广场 接口文档

> 适用版本：当前 `zsy/runninghub` 分支源码树
> 插件目录：后端 `zsy/voice/`，前端 `web/src/extensions/zsy-voice/`
> 数据表：`zsy_voices`（由 `extcore` 注册进宿主的 AutoMigrate，无需手工建表）

## 1. 一句话说明

音色广场是一个 **后台维护、前台/开放接口读取** 的音色目录：管理员在后台维护音色的
**名称、简介、voice_type、头像、示例音频**，以及 **性别、年龄段、语言、适合场景** 四个属性
（还有上架状态与排序），第三方通过一个分页接口读取已上架的全部音色；
后台支持 **CSV 导入 / 导出** 批量维护。

## 2. 核心接口（对外）

| 维度 | 值 |
| --- | --- |
| 方法 / 路径 | `GET /api/zsy/voice/list` |
| 鉴权 | **无需鉴权**（只返回已上架音色） |
| 分页参数 | `page`（默认 1）、`page_size`（默认 20，最大 100） |
| 兼容别名 | `p` = `page`，`size` = `page_size`（与宿主列表接口一致，二选一即可） |
| 过滤参数 | `keyword`（模糊匹配名称 / voice_type / 简介 / 性别 / 年龄段 / 语言 / 适合场景）、`voice_type`（精确）、`gender`（精确，可选 `male`/`female`/`neutral`）、`age_range`（精确，可选 `child`/`teen`/`young`/`middle`/`senior`）、`language`（精确，如 `zh`、`en`） |
| 备注 | 公开接口忽略 `enabled` 参数，只会返回 `enabled=true` 的音色；`gender` / `age_range` / `language` 大小写不敏感 |

```bash
curl -s 'https://<你的网关域名>/api/zsy/voice/list?page=1&page_size=20'
curl -s 'https://<你的网关域名>/api/zsy/voice/list?keyword=xiaomei&page=1&page_size=10'
curl -s 'https://<你的网关域名>/api/zsy/voice/list?voice_type=zh_female_xiaomei'
curl -s 'https://<你的网关域名>/api/zsy/voice/list?gender=female&age_range=young&language=zh'
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
        "name": "小美",
        "description": "温柔女声，适合客服播报",
        "voiceType": "zh_female_xiaomei",
        "gender": "female",
        "ageRange": "young",
        "language": "zh",
        "scenes": ["客服播报", "有声书"],
        "avatarUrl": "/uploads/voices/202601/3f9c…a1.png",
        "audioUrl": "/uploads/voices/202601/3f9c…a1.mp3",
        "audioName": "sample.mp3",
        "audioSize": 20480,
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
| `items[].id` | number | 音色 ID |
| `items[].name` | string | 音色名称（唯一，最长 191 字符） |
| `items[].description` | string | 简介（最长 2000 字符，可为空） |
| `items[].voiceType` | string | 上游 TTS 请求实际发送的 `voice_type` 值（必填） |
| `items[].gender` | string | 性别，受控词表：`male` / `female` / `neutral`，空串表示未标注 |
| `items[].ageRange` | string | 年龄段，受控词表：`child`(儿童) / `teen`(少年) / `young`(青年) / `middle`(中年) / `senior`(老年)，空串表示未标注 |
| `items[].language` | string | 语言标签，小写短代码（`zh`、`en`、`pt-br`…），空串表示未标注。**开放取值**：火山引擎用 `mx` 表示墨西哥西班牙语、`tl` 表示他加禄语，因此不做白名单限制 |
| `items[].scenes` | string[] | 适合场景标签数组（如 `["客服播报","有声书"]`，最多 8 个、每个 ≤24 字符）；无场景时为 `[]`，不会是 `null` |
| `items[].avatarUrl` | string | 头像地址（规则同 `audioUrl`），可为空 |
| `items[].audioUrl` | string | 示例音频地址：网关自身的 `/uploads/voices/...` 或 `http(s)://` 绝对地址，可为空 |
| `items[].audioName` / `audioSize` | string / number | 上传时的原始文件名与字节数 |
| `items[].enabled` | boolean | 是否上架（公开接口恒为 `true`） |
| `items[].sortOrder` | number | 广场排序值，越小越靠前，同值按 `id` 升序 |
| `total` / `page` / `pageSize` / `totalPages` | number | 分页元信息；`total=0` 时 `totalPages=0` |

分页约定（与后台列表共用同一套逻辑）：

- `page < 1` → 按 1 处理；`page_size < 1` → 按 20 处理；`page_size > 100` → 按 100 处理；
- 页码超出范围不会报错，返回空 `items`，并原样回显 `page`。

单条详情：

| 方法 / 路径 | 鉴权 | 说明 |
| --- | --- | --- |
| `GET /api/zsy/voice/:id` | 无需鉴权 | 返回单个已上架音色；未上架或不存在均返回 `404` + `{"success":false,"code":"VOICE_NOT_FOUND"}` |

## 3. 后台管理接口

挂在 `/dashboard/zsy/voice/**`，由宿主的 `middleware.AdminAuth()` 保护（管理员会话 / 访问令牌）。
后台管理页面：**侧边栏「Voice Plaza」（仅管理员可见）→ `/voice-plaza`**。

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| `GET` | `/dashboard/zsy/voice/list` | 分页列表，参数同上，另支持 `enabled=true\|false` 过滤（含未上架音色） |
| `POST` | `/dashboard/zsy/voice` | 新建音色 |
| `GET` | `/dashboard/zsy/voice/:id` | 单条详情（含未上架） |
| `PUT` | `/dashboard/zsy/voice/:id` | 局部更新：只有请求体里出现的字段会被修改，显式传空串可清空 |
| `DELETE` | `/dashboard/zsy/voice/:id` | 物理删除（音色名称立即可复用；示例音频文件保留在磁盘上） |
| `POST` | `/dashboard/zsy/voice/upload` | 上传示例音频（multipart 字段 `file`） |
| `GET` | `/dashboard/zsy/voice/export` | **导出 CSV**（参数与列表一致，导出当前筛选结果的全部行，不分页） |
| `POST` | `/dashboard/zsy/voice/import` | **导入 CSV**（multipart 字段 `file`，可选 `?mode=upsert\|create`，默认 `upsert`） |

新建请求体（`enabled` 省略时默认 `true`，`sortOrder` 省略时默认 `0`；
`scenes` 既可用 JSON 数组也可用逗号分隔字符串，`gender` / `ageRange` / `language`
大小写不敏感，空串表示未标注）：

```bash
curl -X POST 'https://<你的网关域名>/dashboard/zsy/voice' \
  -H 'Authorization: Bearer <管理员访问令牌>' \
  -H 'Content-Type: application/json' \
  -d '{
    "name": "小美",
    "description": "温柔女声，适合客服播报",
    "voiceType": "zh_female_xiaomei",
    "gender": "female",
    "ageRange": "young",
    "language": "zh",
    "scenes": ["客服播报", "有声书"],
    "avatarUrl": "/uploads/voices/202601/3f9c….png",
    "audioUrl": "/uploads/voices/202601/3f9c….mp3",
    "audioName": "sample.mp3",
    "audioSize": 20480,
    "enabled": true,
    "sortOrder": 0
  }'
```

上传示例音频：

```bash
curl -X POST 'https://<你的网关域名>/dashboard/zsy/voice/upload' \
  -H 'Authorization: Bearer <管理员访问令牌>' \
  -F 'file=@sample.mp3'
# → {"success":true,"data":{"url":"/uploads/voices/202601/3f9c….mp3","filename":"3f9c….mp3",
#     "originalName":"sample.mp3","size":20480,"mimeType":"audio/mpeg"}}
```

上传规则：允许 `mp3 / wav / m4a / aac / ogg / opus / flac / webm`，单文件上限 **20MB**；
文件名无扩展名时按内容嗅探格式；落盘路径为 `uploads/voices/YYYYMM/<16 字节随机名>.<ext>`，
由宿主 `router.Static("/uploads", "uploads")` 直接提供访问；随机文件名保证上传的原始
文件名无法穿越目录。

## 3.1 CSV 导出

```bash
curl -s -D - -o voices.csv \
  'https://<你的网关域名>/dashboard/zsy/voice/export?gender=female&enabled=true' \
  -H 'Authorization: Bearer <管理员访问令牌>'
# Content-Type: text/csv; charset=utf-8
# Content-Disposition: attachment; filename="zsy-voices-20260101-120000.csv"
```

- **筛选与列表完全一致**（`keyword` / `voice_type` / `gender` / `age_range` / `enabled`），
  但导出的是筛选结果**全部行**，不分页；不带筛选即全量导出。
- 文件为 **UTF-8 带 BOM**，Excel 直接双击即可正确显示中文；首行为表头，
  列顺序固定为：
  `name, description, voice_type, gender, age_range, language, scenes, avatar_url, audio_url, audio_name, audio_size, enabled, sort_order`。
- 空库导出只含表头，因此该文件同时可当作 **导入模板**。
- `scenes` 在文件中是**一个单元格**（内部用逗号分隔，CSV 会加引号），`enabled` 写 `true/false`。

## 3.2 CSV 导入

```bash
curl -X POST 'https://<你的网关域名>/dashboard/zsy/voice/import?mode=upsert' \
  -H 'Authorization: Bearer <管理员访问令牌>' \
  -F 'file=@voices.csv'
```

- 上传方式：`multipart/form-data`，字段名 `file`；上限 **5MB / 5000 行**。
- `mode` 两种取值：
  - `upsert`（默认）：按 `name` 匹配——已存在则**更新**，不存在则**新建**；
  - `create`：只新建，重名行记为失败行（不会覆盖已有数据）。
- 更新时**只有文件中出现的列会被修改**：例如只导入 `name,voice_type` 两列，
  不会清空已存的简介、性别、场景或示例音频；某列存在但单元格为空则表示清空该字段。
- 表头既可用英文（上面的列名，大小写/空格/下划线/连字符不敏感，
  `Voice Type`、`age-range` 均可），也可用中文：
  `音色名称, 简介, voice_type(或 音色标识), 性别, 年龄段, 语言(或 语种), 适合场景(或 场景), 头像(或 头像URL), 音频地址(或 音频示例URL), 音频文件名, 音频大小, 是否上架, 排序`。
  未识别的列会被忽略并在 `warnings` 中列出。`name` 与 `voice_type` 为必需列。
- 单元格取值：
  - `性别`：`male`/`female`/`neutral`，或中文 `男`/`女`/`中性` 之外的值会作为**失败行**报错；
  - `年龄段`：`child`/`teen`/`young`/`middle`/`senior`；
  - `语言`：小写短代码，可含 `-` 或 `_`（`zh`、`en`、`pt-br`；火山引擎的 `mx`、`tl` 同样接受）；
  - `适合场景`：可用 `,`、`，`、`、`、`;`、`；`、`|` 分隔；
  - `头像` / `音频地址`：`http(s)://` 绝对地址或网关自身的 `/uploads/...` 路径；
  - `是否上架`：`true/false`、`1/0`、`yes/no`、`是/否`、`上架/下架`、`启用/停用`；
  - `音频大小`、`排序` 必须是整数，否则该行报错。
- **逐行处理**：单行出错只记录该行，其余行照常写入。响应示例：

```json
{
  "success": true,
  "message": "",
  "data": {
    "total": 3, "created": 1, "updated": 1, "failed": 1,
    "errors": [{ "row": 4, "name": "坏音色", "message": "非法性别 \"robot\" (可选 male / female / neutral，留空表示未标注)" }],
    "warnings": ["已忽略无法识别的列: 备注列"]
  }
}
```

- `row` 为 CSV 中的记录序号（表头为第 1 行），修正这些行后重新导入即可；
  失败行超过 200 条时只返回前 200 条明细，`failed` 仍为真实总数。
- 结构性错误（文件为空、缺少 `name`/`voice_type` 列、行数或体积超限、`mode` 非法）
  会直接返回 `success:false`，此时**不会写入任何数据**。

## 4. 校验与错误约定

后台接口沿用面板信封：HTTP 200 + `{"success":false,"message":"…"}`，`message` 为可直接展示的中文提示；
公开接口使用真实 HTTP 状态码 + 稳定错误码：

| 场景 | 状态码 | `code` |
| --- | --- | --- |
| `:id` 非正整数 | 400 | `INVALID_PARAMS` |
| 音色不存在或未上架 | 404 | `VOICE_NOT_FOUND` |
| 服务端读取失败 | 500 | `DATABASE_ERROR` |

字段校验（`zsy/voice/store.go` 的 `validateVoice`）：

- `name` 必填、去首尾空格、最长 191 字符，全表唯一（重名报「音色名称已存在」）；
- `voiceType` 必填、最长 191 字符；
- `description` 最长 2000 字符；
- `gender` 仅接受 `male` / `female` / `neutral`（大小写不敏感，空串 = 未标注），否则报「非法性别」；
- `ageRange` 仅接受 `child` / `teen` / `young` / `middle` / `senior`（同上），否则报「非法年龄段」；
- `language` 为可选短标签：小写字母开头，可含数字、`-`、`_`，最长 16 字符（`zh`、`en`、`pt-br`、`mx`）；
- `scenes` 最多 8 个、每个 ≤24 字符，存储为去重后的逗号分隔串（总长 ≤255 字符）；
- `avatarUrl` / `audioUrl` 必须是 `/uploads/...` 相对路径或 `http(s)://` 绝对地址（拒绝 `javascript:`、`data:`、`//host` 等），最长 768 字符；
- `audioSize ≥ 0`；`sortOrder` 取值区间 `[-1000000, 1000000]`。

## 4.1 批量导入既有音色库（Excel → CSV → 库）

`seedmodel/` 下保留了火山引擎音色库的一次性导入工具链：

| 步骤 | 命令 | 说明 |
| --- | --- | --- |
| 1. Excel → CSV | `C:\Python313\python.exe seedmodel\build_voice_csv.py` | 读取 `seedmodel/火山引擎音色库列表.xlsx`（列：场景 / 音色名称 / 简介 / 音色ID / 头像URL / 音频示例URL），输出 `seedmodel/火山引擎音色库列表.csv`：场景 → `scenes`（丢弃 `#N/A`）、音色ID → `voice_type`、头像URL → `avatar_url`、音频示例URL → `audio_url`，并从音色 ID 推导 `gender`（`_female`/`_male`）与 `language`（`zh`/`en`/`pt`/`mx`…），按 `age_range_rules.py` 推导 `age_range`。整库 `enabled=true`、`sort_order` 按表格行序 |
| 2. 写库 | `go run ./_scripts/voiceplaza-seed-volc -dry` 先试跑；去掉 `-dry` 正式写入 | 复用插件自身的 CSV 导入链路（表头别名、逐行校验、按名称 upsert）。`SQL_DSN` 取自环境变量或 `.env`，即"跑在哪个环境就写哪个库"；`-mode=create` 可改为只新增 |
| 3. 清空重建（可选） | `go run ./_scripts/voiceplaza-seed-volc -replace` | 先**物理删除库里全部音色**（含 CSV 之外的记录）再整表重建，删除 + 导入在**同一个事务**里完成，因此对外始终看到「旧的 377 条」或「新的 377 条」，不会出现空窗或半截列表 |
| 4. 核对 | `go run ./_scripts/voiceplaza-seed-volc -stats` | 直接调用插件的 `VoiceSearch`（与公开列表同一条代码路径），打印按年龄段/性别/语言的命中数并联样输出，等于预演 `?age_range=…` 的返回 |

试跑（`-dry`）会在事务里真实执行删除/插入/更新再回滚，因此报告的数字就是正式写入的结果，且不会改动数据
（`zsy_voices stored=...` 一行可确认回滚生效）。

### 年龄段是怎么补出来的

Excel 里没有年龄段这一列，`seedmodel/age_range_rules.py` 用音色**名称 + 简介**里的措辞分桶，
桶的优先级固定为 `senior > child > teen > middle > young`（"童声"比"甜美"更具体，"中年男老师"
比它同时提到的"青年"更明确），第二轮再看音色/人设语气（`叔音`/`霸总`/`磁性`/`解说` 偏中年，
`女神`/`公主`/`治愈`/`温柔` 偏青年），两轮都没命中的落入 `young` 兜底。分类结果与依据
（`name:少年`、`desc:大叔`、`fallback`）可离线复现审阅。

当前 377 条的分布：`child 14 / teen 44 / young 211 / middle 101 / senior 7`，其中 25 条为兜底值。

转换脚本同时修复了源表的两处导出缺陷（会打印在运行日志里）：

- 44 个单元格里带有替换字符 `U+FFFD`，已剔除；
- 205 行的简介末尾被导出时拼上了场景名（`…亲和力十足。有声阅读`），已从简介中裁掉；
  其中 36 行原本场景列为 `#N/A`，裁掉的场景名被还原进 `scenes` 列表。

当前线上库（`aiapi_hudongdian`）已按此流程导入 **377 条**火山引擎音色：
性别 female 160 / male 217，语言 17 种（zh 276），表内 `name` 与 `voice_type` 均唯一。

> 注：线上库是远程 MySQL，逐行 upsert 共约 700+ 次往返，一次运行可能超过 10 分钟；
> upsert 按名称幂等，超时后用同一条命令续跑即可，不会产生重复行。

## 5. 代码位置

| 内容 | 路径 |
| --- | --- |
| 插件装配（extcore 注册、表迁移、路由挂载） | `zsy/voice/extension.go` |
| 数据模型与 DTO（含性别/年龄段词表、场景解析） | `zsy/voice/models.go` |
| 存储层与字段校验 | `zsy/voice/store.go` |
| CSV 导入导出（列别名、逐行校验、导入报告） | `zsy/voice/csv.go` |
| 公开接口处理器 | `zsy/voice/controllers_public.go` |
| 后台接口处理器（含 export / import） | `zsy/voice/controllers_admin.go` |
| 示例音频上传 | `zsy/voice/controllers_upload.go` |
| 路由与中间件 | `zsy/voice/routes.go` |
| 回归测试 | `zsy/voice/store_test.go`、`zsy/voice/http_test.go`、`zsy/voice/csv_test.go`、`zsy/voice/routes_test.go` |
| 批量导入脚本（一键写库 / 统计核对） | `_scripts/voiceplaza-seed-volc/main.go` |
| 年龄段推导规则 | `seedmodel/age_range_rules.py` |
| Excel → CSV 转换 / 线上库核对 | `seedmodel/build_voice_csv.py`、`seedmodel/verify_live_db.py` |
| 后台管理页 | `web/src/extensions/zsy-voice/pages/voice-plaza-page.tsx` |
| 属性编辑与导入弹窗 | `web/src/extensions/zsy-voice/components/voice-form-dialog.tsx`、`components/voice-import-dialog.tsx` |
| 前端接口客户端 | `web/src/extensions/zsy-voice/api.ts` |
| 前端测试 | `web/src/extensions/zsy-voice/lib/__tests__/`、`web/src/extensions/zsy-voice/pages/__tests__/` |
