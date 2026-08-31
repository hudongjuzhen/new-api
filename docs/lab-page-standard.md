# /lab 模型试用页（Lab）实现标准

> 本文档是 `/lab?model=<模型标识>` 页面的实现标准，用于约束后续迭代（新增参数、新增端点示例、样式调整等），保持页面行为与代码归属的一致性。

## 1. 定位与入口

- `/lab` 是**非公开路由**（需登录才能访问），用于按模型试用对话能力；未登录访问时由路由守卫重定向至 `/sign-in`（携带 `redirect=/lab?model=...`，登录后回跳）。
- 用户在调用能力时，需要**手动选择使用哪个 key**（从本人令牌列表中选择，见 §4.1）。
- URL 参数：`?model=<模型标识>`，通过 Zod schema（`validateSearch`）校验，空值/非法值回退为未选择模型。
- 入口：
  - 模型广场 `/pricing` 表格行的「试用」按钮；
  - 模型详情页头部「Try Now」按钮。
  - 两处均 `navigate({ to: '/lab', search: { model: modelName } })`。

## 2. 路由与代码归属

| 内容 | 位置 | 说明 |
| --- | --- | --- |
| 路由文件 | `web/src/routes/lab/index.tsx` | TanStack Router 文件路由：参数校验、登录保护（`beforeLoad` 检查 `auth.user`，未登录 `redirect` 至 `/sign-in`，参考 `/rankings` 模式）与组件挂载 |
| 页面代码 | `web/src/extensions/zsy-lab/` | zsy 系前端扩展目录（与 `zsy-runninghub` 同级约定） |
| 后端插件 | 根目录 `zsy/` | 仅承载后端 Go 插件代码，**禁止**放置前端文件 |

`web/src/extensions/zsy-lab/` 文件结构：

```
zsy-lab/
├── index.ts           # barrel：导出 Lab 组件
├── lab-page.tsx       # 页面骨架：模型头 + 四 Tab 容器
├── lab-playground.tsx # Playground Tab（INPUT/OUTPUT 双栏）
├── lab-examples.tsx   # Examples Tab（代码示例）
├── lab-api.tsx        # API Tab（端点与参数说明）
├── lab-history.tsx    # History Tab（本地使用记录）
├── lib/
│   └── history.ts     # 本地历史读写（localStorage，24h TTL）
└── __tests__/
    └── lab-page.test.tsx
```

## 3. 页面结构

1. **模型头部**（所有 Tab 共享）：模型图标（`getLobeIcon`）、模型名、描述、标签（`parseTags`）；右侧「切换模型」按钮跳转 `/pricing`。
2. **模型数据**：复用 `usePricingData()`（公开 `/api/pricing`），按 `model` 参数匹配 `PricingModel`；匹配不到时仅显示模型名，不阻塞其余功能。
3. **Tab 栏**：下划线式四个页签 `Playground / Examples / API / History`，默认 `Playground`。

## 4. Playground Tab

### 4.1 请求通道与 Key 选择

- 一律 `POST /v1/chat/completions`（OpenAI 兼容端点），请求头 `Authorization: Bearer <所选 Key>`，SSE 流式；请求封装由 `useLabRun` 承担。
- **Key 选择器**（Playground 必备控件）：
  - 数据源：`GET /api/token/?p=1&size=100`（`@/features/keys/api.ts`，本人令牌列表，含名称与 key）；
  - 仅列出启用状态的令牌，展示令牌名称，默认选中第一个可用项；
  - 所选 Key 仅保存在组件内存态，刷新后重置为默认，**禁止写入 localStorage**；
  - 无可用令牌时，Run 区域显示引导按钮，跳转「令牌」页创建。
- 未选择模型或未选择 Key 时 Run 禁用。

### 4.2 INPUT 面板

- **Form / JSON 双模式**：
  - Form：对话内容 Textarea、上传图片（复用 `/api/upload/image`，上限 4 张）、系统提示词、参数控件（采样温度滑条、最大输出长度数字输入、核采样滑条、频率惩罚/存在惩罚滑条）、流式输出开关。
  - JSON：直接编辑请求体 JSON（以 Form 当前值预填），Run 时解析发送，解析失败 toast 报错。
- **API FORMAT 选择器**：`OpenAI Chat` 可用；`Anthropic` 仅作端点格式展示并禁用（Playground 仅实现 OpenAI Chat 格式），tooltip 引导至 API/Examples 页签。
- 默认参数：`temperature=0.7`、`max_tokens=4096`、`top_p=1`、`frequency_penalty=0`、`presence_penalty=0`、`stream=true`。

### 4.3 OUTPUT 面板

- `Preview / JSON` 双视图 + 复制按钮：
  - Preview：渲染流式文本（含 reasoning 折叠展示，错误以 destructive 样式展示）。
  - JSON：展示原始 SSE 分片（每行一个上游 JSON 事件，仅保留最近 200 行）。
- 请求结束后（成功/失败）写入本地历史（见 §7）。

## 5. Examples Tab

- 两种调用方式卡片，内容必须是**本网关真实支持的端点**：
  - 方式一：OpenAI Chat 兼容 `POST /v1/chat/completions`（`Authorization: Bearer`）。
  - 方式二：Anthropic 兼容 `POST /v1/messages`（`x-api-key` + `anthropic-version`）。
- 每张卡片提供 `curl / Python / JavaScript` 三语言示例（`CodeBlock` + 复制按钮）；`baseUrl` 取 `useStatus().server_address`，回退 `window.location.origin`。
- 底部提示条：Playground 页签与示例调用同一端点 `/v1/chat/completions`；区别是 Playground 从页内选择器选用本人令牌，示例面向外部代码（令牌在「令牌」页生成）。
- **禁止**虚构 OpenRouter 特性（`provider.sort`、`:floor/:nitro/:stable` 后缀路由等）——后端不支持，除非后端先行实现并在本标准中登记。

## 6. API Tab

- ENDPOINT 卡：列出上述两个真实端点（method 徽标 + 路径）与认证头说明。
- Supported parameters 表：`model`、`messages`、`stream`、`temperature`、`top_p`、`max_tokens`、`frequency_penalty`、`presence_penalty`（类型/必填/默认值/说明）。
- 说明卡：Playground 调用方式说明（端点同上，Key 取自页内选择器的本人令牌，计费与配额计入所选令牌）。

## 7. History Tab

- 存储：`localStorage` key `zsy_lab_history`。
- 记录结构：`{ id, model, prompt, status: 'success' | 'error', createdAt, durationMs?, error? }`。
- 规则：TTL 24 小时（读取与写入时双向裁剪）、上限 50 条、时间倒序展示；空态文案「暂无记录，发起第一次请求后自动记录」。
- 「查看全部 / 收起」切换完整列表与最近 10 条。

## 8. i18n 与测试

- 所有用户可见文案 `t('English key')`，key 同步登记到 `web/src/i18n/locales/*.json`（en 为基准，zh 必填，其余语言经 `bun run i18n:sync` 处理）。
- 测试置于 `zsy-lab/__tests__/`，Vitest + React Testing Library，覆盖：model 参数展示、未选择模型空态、Key 选择器渲染与未选 Key 禁用 Run、History 记录裁剪（TTL/上限）。使用 `require`/`assert`（testify 风格对应前端为标准断言）。

## 9. 禁止事项

- 不提供手填/粘贴 Key 的输入框：Key 只能从本人令牌列表中选择。
- 不将所选 Key 明文写入 localStorage、URL 或任何持久层（仅内存态）。
- 不虚构后端不存在的路由/调度能力。
- 不在根目录 `zsy/` 下新增前端文件。
