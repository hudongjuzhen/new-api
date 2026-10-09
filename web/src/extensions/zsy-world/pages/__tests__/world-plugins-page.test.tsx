/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

/**
 * 世界 IP · 插件管理那一屏（`zsy/world` 的后台面）。
 *
 * # 这一组守的是**几句话 + 接线**，不是布局
 *
 * | 判据 | 说错了会怎样 |
 * |---|---|
 * | ★★ **签发 ≠ 开通** | 运营以为点一下就开通了，把文件发出去，用户什么都用不了 |
 * | ★ 坏模板要列出来并说明 | 运营看到的是"我放进去的文件不见了"，以为后台坏了 |
 * | ★★ 左 ID / 右列表，一行放好几个 | 用户报的那件事：太浪费空间 |
 * | ★ 读不到状态时**按钮全禁用** | 对着一个其实已经有能力的人再开一次 |
 *
 * ⚠★ 断言查的是 `t()` 的**英文 key**（恒等翻译），所以"每一句话都有翻译"
 * 这件事由语言包负责 —— 而"有没有哪句话没走 `t()`"由这一组钉。
 */
const worldApi = vi.hoisted(() => ({
  listPluginTemplates: vi.fn(),
  issuePluginFile: vi.fn(),
  listEntitlements: vi.fn(),
  grantCapability: vi.fn(),
  revokeCapability: vi.fn(),
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  ...worldApi,
}))

/*
 * ⚠★ **签发那一段不 mock**（第一版 mock 了，而且它悄悄什么都没做）。
 *
 * `vi.mock('../../lib/plugin-download', …)` 只替掉**模块的导出**，而模块里
 * `signAndDownloadPluginFile` 调用 `saveTextAsFile` 走的是**词法作用域**里的那个名字
 * —— 于是那段浏览器代码照旧跑真的，`expect(saveTextAsFile).toHaveBeenCalled()` 永远是 0。
 * 那种失败看起来像"功能没实现"，其实是**测试的替身站错了位置**。
 *
 * 于是判据换成"**有没有真的存过东西**"：jsdom 有 `Blob` 与 `URL.createObjectURL`，
 * 所以那一段真跑得起来，而这一组断言的是它**跑到了**（以及由此产生的可见结果 ——
 * 提示行里那个文件名与 check）。
 */

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { WorldPluginsPage } = await import('../world-plugins-page')

/* 断言查的是组件传给 t() 的**英文原文**，所以把它们登记成恒等翻译 */
const identity: Record<string, string> = {}
for (const key of [
  'World IP · Plugin management',
  'Reload',
  'Signing is not granting',
  'Signing only produces a file. Whether the account can actually use the capability is decided by the opens below, and checked on every request by the server. A signed file with no open installs fine and then gets refused when opened.',
  'Sign a plugin file for one account, then send it to that person. The file records which account it is for — importing it under a different account is refused, so a file sent to the wrong person is caught immediately.',
  'Plugin templates',
  'Open plugins for an account',
  'Enter the account ID first: then this account’s every capability is listed at once, and the file to send him can be signed from the plugin template below. Signing is not opening — a signed file with nothing opened installs fine and then gets refused.',
  'Account ID',
  'e.g. 7',
  'Sign',
  'The account that will import this file. Signing needs it too — the file carries the account.',
  'Open',
  'Cancel access',
  'Open {{capability}} for this account',
  'Cancel access to {{capability}}',
  'Opened',
  'Not opened',
  'Status unreadable',
  'Required by {{plugins}}',
  'No plugin asks for it',
  'Signed and ready to send',
  'Signed — but this account cannot use it yet',
  'Signed “{{id}}”{{version}} for account {{username}} (ID {{userId}}): file {{fileName}}.',
  'This account now holds {{held}} — send the file over for import.',
  '⚠ But this account only holds {{held}}, and {{missing}} are still missing — the file installs fine, yet opening the world screen is refused by the server. Open those capabilities for it below.',
  '(none at all)',
  'History (including cancelled and expired rows)',
  'In effect',
  'Already cancelled',
  'Expired',
  'Read from {{dir}} — the file name is the plugin id.',
  'Fix the account ID above to open or cancel.',
  'Capabilities cannot be opened or cancelled until this is read.',
  'Reading what this account holds — opening is enabled in a moment.',
  'Opening a capability here is what decides whether the account can use it; the file to send him is signed on a plugin template below.',
  'No capability can be opened yet — the plugin templates below declare none.',
  'Public plugin — one-click install',
  'Private plugin — signed per account',
  'The server did not report a directory',
  'The server did not report',
  'The template directory {{dir}} holds no usable template yet. Put a plugin JSON into it (the file name is the plugin id, for example world-ip.json), then click Reload. Its source may only be one of these: {{sources}}.',
  'Could not read the plugin template list',
  'Could not read this account’s capabilities',
  'Pick a plugin first.',
  'Could not sign',
]) {
  identity[key] = key
}
/* ⚠ 顿号那句也是要翻译的（中文用「、」）—— 恒等翻译下直接用中文那一版 */
identity[', '] = '、'

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: { en: { translation: identity } },
})

function renderPage() {
  return render(
    <I18nextProvider i18n={i18n}>
      <WorldPluginsPage />
    </I18nextProvider>
  )
}

const TEMPLATES = {
  directory: '/srv/new-api/plugin-templates',
  knownKinds: ['catalog', 'world'],
  knownSources: ['voiceCatalog', 'avatarCatalog', 'appCatalog', 'worldOps'],
  items: [
    {
      id: 'world-ip',
      name: '世界 IP 资源管理器',
      capabilities: ['world-ip', 'world-ip-ai'],
      screens: ['世界 IP'],
      visibility: 'private',
      problem: '',
      source: '/srv/new-api/plugin-templates/world-ip.json',
    },
    {
      id: 'broken-one',
      name: 'broken-one',
      capabilities: [],
      screens: [],
      visibility: 'private',
      problem: '文件名与 id 不一致：文件名说 "broken-one"，而里面的 id 是 "other"。',
      source: '/srv/new-api/plugin-templates/broken-one.json',
    },
  ],
}

function issued(overrides: Record<string, unknown> = {}) {
  return {
    /* ★ 文件名里带版本 —— 与新增的那一格一起测（见下面"文件名带版本"那条） */
    fileName: 'world-ip-v0.1.0-zsy-user7-20261007.aimv-plugin.json',
    file: '{"format":"aimv-plugin"}',
    pluginId: 'world-ip',
    pluginVersion: '0.1.0',
    userId: 7,
    username: 'zsy',
    site: 'https://www.aipole.top',
    check: 'fnv1a64:0cbcf8114de19bfa',
    capabilities: ['world-ip', 'world-ip-ai'],
    granted: [],
    ...overrides,
  }
}

function entitlements(active: string[] = []) {
  return { userId: 7, items: [], active }
}

/**
 * 右栏里某一个能力的那一行。
 *
 * ⚠ 按钮上的**字**只有两三个字（一行放好几个的前提），所以按 `aria-label`
 * （"Open world-ip for this account"）找它 —— 那也正是读屏用户听到的名字。
 */
function rowOf(capability: string): HTMLElement {
  return screen.getByTestId(`world-capability-${capability}`)
}

function actionOf(capability: string): HTMLElement {
  const row = rowOf(capability)
  const name = row.querySelector('button[aria-label]')?.getAttribute('aria-label')
  return within(row).getByRole('button', { name: name as string })
}

beforeEach(() => {
  vi.clearAllMocks()
  worldApi.listPluginTemplates.mockResolvedValue(TEMPLATES)
  worldApi.listEntitlements.mockResolvedValue(entitlements([]))
  worldApi.issuePluginFile.mockResolvedValue(issued())
})

describe('世界 IP · 插件管理', () => {
  test('★ 坏模板列出来、并带上它自己的那句话与文件名', async () => {
    renderPage()

    // 好模板：能看见
    await screen.findByText('世界 IP 资源管理器')

    /*
     * ★ 坏模板**必须露面**。不出现的话运营看到的是"我放进去的文件不见了" ——
     * 那看起来像后台坏了，而真相是那个文件里有个笔误。
     */
    expect(await screen.findByText(/文件名与 id 不一致/)).toBeInTheDocument()
    expect(
      screen.getByText('/srv/new-api/plugin-templates/broken-one.json')
    ).toBeInTheDocument()
  })

  test('★ 一进这一屏就写明「签发不等于授权」', async () => {
    renderPage()

    /*
     * 这一条横幅是常驻的，不是点完才出现 —— 它回答的是运营**动手之前**就该知道的
     * 那件事（否则他会以为点一下就开通了）。
     */
    expect(await screen.findByText('Signing is not granting')).toBeInTheDocument()
  })

  test('★★ 左 ID / 右列表：两块在**同一个横向容器**里', async () => {
    renderPage()

    const input = await screen.findByLabelText('Account ID')
    /* 左栏 → 外面那个"左 ID / 右列表"的容器 */
    const left = input.parentElement as HTMLElement
    const split = left.parentElement as HTMLElement

    expect(split.className).toContain('flex')
    expect(split.className).toContain('flex-col')
    expect(split.className).toContain('md:flex-row')
    expect(left.className).toContain('md:w-56')

    const grid = screen.getByTestId('world-capability-grid')
    /* ⚠ 而且它确实在 split 里面 */
    expect(split.contains(grid)).toBe(true)
  })

  test('★★ 一行放好几个：列数按**可用宽度**算，不看视口断点', async () => {
    /*
     * ⚠★★ 这一条是**回归**判据，保护的是一个真发生过的错（用户 2026-…）：
     *
     * > "模式库还是在上面一行一个，太丑了，也没变化呀"
     *
     * 原因：列数写成了 `xl:grid-cols-2`，而 Tailwind 4 的 `xl` = 1280px 看的是
     * **视口**宽度 —— 这一屏的内容区被侧边栏与内边距吃掉一截，那个断点**从来没生效**，
     * 于是永远只有一列；而当时的测试查的是"class 里有没有 xl:grid-cols-2",
     * 它绿着、界面上却是一行一个。
     *
     * 所以判据换成**真正决定列数的那件事**：`auto-fill` + 一个最小列宽。
     */
    renderPage()

    const grids = [
      await screen.findByTestId('world-capability-grid'),
      /* 模板那一块也是同一个形状 */
      (await screen.findByTestId('world-template-world-ip')).parentElement as HTMLElement,
    ]

    for (const grid of grids) {
      expect(grid.className).toContain('grid')
      expect(grid.style.gridTemplateColumns).toContain('auto-fill')
      expect(grid.style.gridTemplateColumns).toContain('minmax(')
      expect(grid.className).not.toContain('grid-cols-1')
      expect(grid.className).not.toMatch(/xl:grid-cols/)
    }
  })

  test('★★ 没填账号时：签发按钮是禁用的，并说明要先填 ID', async () => {
    const user = userEvent.setup()
    renderPage()

    await screen.findByTestId('world-template-world-ip')
    /*
     * ⚠ 签发是**按插件**的动作，而账号在左边那一格填 —— 没填就没有可签发的对象，
     * 所以先禁用（点了会发一个 userId=0 的请求）。
     */
    expect(screen.getByTestId('world-issue-world-ip')).toBeDisabled()
    expect(screen.getByTestId('world-open-hint')).toBeInTheDocument()

    await user.type(screen.getByLabelText('Account ID'), '7')
    expect(screen.getByTestId('world-issue-world-ip')).toBeEnabled()
  })

  test('★★ 填一次 ID：模板声明的每个能力都带出"已开通 / 未开通"（不必先选插件）', async () => {
    const user = userEvent.setup()
    worldApi.listEntitlements.mockResolvedValue(entitlements(['world-ip']))
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')

    /*
     * ★★ 这一条就是用户要的那件事：**一次输入，全部能力的状态一起出来**。
     * 两个能力同时看得见，一个 open 一个 closed。
     */
    await waitFor(() => expect(rowOf('world-ip')).toHaveAttribute('data-state', 'open'))
    expect(rowOf('world-ip-ai')).toHaveAttribute('data-state', 'closed')
    expect(screen.getByTestId('world-capability-state-world-ip')).toHaveTextContent('Opened')
    expect(screen.getByTestId('world-capability-state-world-ip-ai')).toHaveTextContent(
      'Not opened'
    )
    expect(actionOf('world-ip')).toHaveTextContent('Cancel access')
    expect(actionOf('world-ip-ai')).toHaveTextContent('Open')
  })

  test('★ 每一行上写着**这份能力是哪几份插件要的**（能力名与插件名不是一回事）', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')

    const row = await screen.findByTestId('world-capability-world-ip-ai')
    expect(within(row).getByText(/Required by/)).toHaveTextContent('世界 IP 资源管理器')
  })

  test('★ 账号 ID 填了非数字：当场说清（且两处都说了），不点也不发请求', async () => {
    const user = userEvent.setup()
    renderPage()

    await screen.findByText('世界 IP 资源管理器')
    await user.type(screen.getByLabelText('Account ID'), 'abc')

    /*
     * ⚠ 用 `getAllByText` + 一个函数匹配器：填错时这句话在**两处**出现
     * （输入框下面那行 + 列表上面那条说明）—— 那是**有意的**，而 `getByText` 会因此红。
     */
    await waitFor(() =>
      expect(
        screen.getAllByText((_, el) => /run of digits/.test(el?.textContent ?? '')).length
      ).toBeGreaterThan(0)
    )
    expect(screen.getByRole('button', { name: /Sign/ })).toBeDisabled()
    expect(worldApi.issuePluginFile).not.toHaveBeenCalled()
  })

  test('★★ 签发之后：文件名与校验和都显示出来，而且真的走到了"存文件"那一步', async () => {
    /*
     * ⚠ 这一条**不**去断言 `saveTextAsFile` 被调用（见文件上方那段说明）。判据换成两件
     * **看得见**的事：
     *   ① 那一屏把文件名与 check 显示出来了（说明签发真的回来了）；
     *   ② 签发只发生**一次**（"下载 + 拿结果"要是写成两次签发，两次的 issuedAt 不同，
     *      界面上的 check 就与用户手上那份对不上）。
     */
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByTestId('world-issue-world-ip'))

    await waitFor(() => expect(worldApi.issuePluginFile).toHaveBeenCalledTimes(1))
    /* 文件名在两处出现（成功那句提示 + 底下那行详情），所以用 getAllByText */
    await waitFor(() =>
      expect(
        screen.getAllByText(/world-ip-v0\.1\.0-zsy-user7-20261007\.aimv-plugin\.json/).length
      ).toBeGreaterThan(0)
    )
    /* ★ 版本也要在那句话里出现 —— 运营会把它复制到聊天记录里 */
    expect(screen.getByText(/Signed “world-ip” v0\.1\.0 for account zsy/)).toBeInTheDocument()
    expect(screen.getByText(/fnv1a64:0cbcf8114de19bfa/)).toBeInTheDocument()
    /* ★ 签的是**这一行**上那份插件（按插件签发，账号来自左边那一格） */
    expect(worldApi.issuePluginFile).toHaveBeenCalledWith(7, 'world-ip', undefined)
  })

  test('★★ 还没开通能力时，明说"装得上、可是用不了，还差哪个"', async () => {
    /*
     * ★ 这一条是整屏最要紧的一句话。少了它，运营会把文件发出去，
     * 然后收到"你给我的东西用不了" —— 而他从界面上完全看不出为什么。
     */
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByTestId('world-issue-world-ip'))

    expect(
      await screen.findByText('Signed — but this account cannot use it yet')
    ).toBeInTheDocument()
    expect(screen.getByText(/world-ip、world-ip-ai are still missing/)).toBeInTheDocument()
    expect(screen.getByText(/refused by the server/)).toBeInTheDocument()
  })

  test('★ 能力都已开通时，说的是"可以发了"（而不是那半句警告）', async () => {
    worldApi.issuePluginFile.mockResolvedValue(
      issued({ granted: ['world-ip', 'world-ip-ai'] })
    )
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByTestId('world-issue-world-ip'))

    expect(await screen.findByText('Signed and ready to send')).toBeInTheDocument()
    expect(screen.queryByText(/refused by the server/)).not.toBeInTheDocument()
  })

  test('★ 签发**不**调用开通接口（两件事不许混）', async () => {
    /*
     * ⚠ 把两者合成一个动作会省一次点击，代价是"这个人到底被开通了没有"
     * 从此看不出来 —— 而取消与审计都要那个答案。
     */
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByTestId('world-issue-world-ip'))
    /* 等签发真的落地（回来过一次）再断言 */
    await waitFor(() => expect(worldApi.issuePluginFile).toHaveBeenCalled())

    expect(worldApi.grantCapability).not.toHaveBeenCalled()
    expect(worldApi.revokeCapability).not.toHaveBeenCalled()
  })

  test('★ 开通与取消各自打到对的接口，并在之后重新读一次状态', async () => {
    /*
     * ★ 记「开通**被调用过**」与「开通之后的状态」是**两件事**：
     * `active` 一变，下面读回来的就是"已经生效"，界面上那颗按钮随之变成「取消」——
     * 而"它调用了没有、参数对不对"与这个变化无关。
     */
    let granted = false
    let active: string[] = []
    worldApi.grantCapability.mockImplementation(async (_uid: number, capability: string) => {
      granted = true
      active = [...active, capability]
    })
    worldApi.listEntitlements.mockImplementation(async () => entitlements(active))

    const user = userEvent.setup()
    renderPage()

    /* 先等模板那一屏落定，再去填账号 —— 免得把"模板还在读"的那一帧当成界面状态 */
    await screen.findByText('世界 IP 资源管理器')

    await user.type(screen.getByLabelText('Account ID'), '7')
    await waitFor(() => expect(rowOf('world-ip')).toHaveAttribute('data-state', 'closed'))

    /*
     * ★ 按**能力名**定位那一行（`data-testid`），再点它里面的按钮 —— 不用索引。
     * ⚠ 也不能用 `getByText('world-ip')` 再往上找：页面上有好几处写着这几个字
     * （那一行、以及底下文件名里也含它），`getByText` 会因为"找到多个"而红。
     */
    await user.click(actionOf('world-ip'))

    /*
     * ★ 判据两件，都与实现无关：
     *   ① 开通被调用过、且参数是**这个账号 + 这个能力**；
     *   ② 改完之后界面**跟着变**了（那一行从「开通」变成「取消」）。
     */
    await waitFor(() => expect(granted).toBe(true))
    /*
     * ★ 参数**逐格**断言（含第三个 `0`）。
     *
     * ⚠ 这一条抓到过一个真错：页面原先只传了两个参数，而**签名里第三个是可选的**，
     * 于是少传不会报错、界面看起来完全正常 —— 只有"调用记录少一格"能发现它。
     */
    expect(worldApi.grantCapability.mock.calls, '开通的参数不对').toEqual([[7, 'world-ip', 0]])

    await waitFor(() => expect(rowOf('world-ip')).toHaveAttribute('data-state', 'open'))
    expect(actionOf('world-ip')).toHaveTextContent('Cancel access')
  })

  test('★ 账号 ID 填错时，开通按钮是**禁用**的（不是"点了没反应"）', async () => {
    /*
     * ⚠ 这一条是实测改的：第一版只按"忙不忙"渲染按钮，于是填了 "abc" 之后
     * 按钮仍是亮的，点下去什么也不发生 —— 用户会以为这个功能坏了。
     */
    const user = userEvent.setup()
    renderPage()

    /* 先等模板落定再填 —— 否则"按钮还没画出来"与"按钮被禁用了"分不清 */
    await screen.findByText('世界 IP 资源管理器')

    await user.type(screen.getByLabelText('Account ID'), 'abc')
    await waitFor(() => expect(rowOf('world-ip')).toHaveAttribute('data-state', 'closed'))

    for (const capability of ['world-ip', 'world-ip-ai']) {
      expect(actionOf(capability)).toBeDisabled()
    }
  })

  test('★ 已经生效的能力给的是「取消」，而不是再开一次', async () => {
    worldApi.listEntitlements.mockResolvedValue(entitlements(['world-ip']))
    worldApi.revokeCapability.mockResolvedValue(undefined)

    const user = userEvent.setup()
    renderPage()

    await screen.findByText('世界 IP 资源管理器')
    await user.type(screen.getByLabelText('Account ID'), '7')

    await waitFor(() => expect(rowOf('world-ip')).toHaveAttribute('data-state', 'open'))
    await user.click(actionOf('world-ip'))
    await waitFor(() =>
      expect(worldApi.revokeCapability).toHaveBeenCalledWith(7, 'world-ip')
    )
  })

  test('★ 服务端那句报错要原样显示（这一组接口出错也回 200）', async () => {
    /*
     * ⚠ `common.ApiErrorMsg` 是 **HTTP 200 + success:false**，所以 axios 的默认文案
     * （"Request failed with status code 200"）对运营毫无用处 —— 真正有用的是
     * 服务端写在 `message` 里的那一句。
     */
    worldApi.issuePluginFile.mockRejectedValue({
      response: { data: { success: false, message: '没有 "nope" 这份模板。现有的是：world-ip' } },
    })
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByTestId('world-issue-world-ip'))

    expect(await screen.findByText(/没有 "nope" 这份模板/)).toBeInTheDocument()
    expect(screen.queryByText(/status code/)).not.toBeInTheDocument()
  })

  test('★ 模板目录空着时，给出目录与能写的 source', async () => {
    worldApi.listPluginTemplates.mockResolvedValue({
      ...TEMPLATES,
      items: [],
    })
    renderPage()

    expect(await screen.findByText(/holds no usable template yet/)).toBeInTheDocument()
    expect(screen.getByText(/worldOps/)).toBeInTheDocument()
  })

  test('★ 「重新读取」真的再拉一次（运营刚放进去一个文件）', async () => {
    const user = userEvent.setup()
    renderPage()

    await screen.findByText('世界 IP 资源管理器')
    await user.click(screen.getByRole('button', { name: /Reload/ }))

    await waitFor(() => expect(worldApi.listPluginTemplates).toHaveBeenCalledTimes(2))
  })

  test('★ 读模板失败时说清是读不到，而不是画一屏空的', async () => {
    /*
     * ⚠ 用**真实的形状**：这一组接口出错也回 HTTP 200，所以那句有用的话在
     * `response.data.message` 里（axios 的默认文案对运营毫无用处 —— 这正是
     * `readError` 优先取它的原因）。
     */
    worldApi.listPluginTemplates.mockRejectedValue({
      response: { data: { success: false, message: '模板目录读不到' } },
    })
    renderPage()

    await waitFor(() =>
      expect(screen.getByTestId('world-templates-error')).toBeInTheDocument()
    )
    expect(screen.getByTestId('world-templates-error').textContent).toContain('模板目录读不到')
  })

  test('★ 读模板失败、服务端又没给话时，回落到自己那句（而且是**英文 key**）', async () => {
    worldApi.listPluginTemplates.mockRejectedValue(new Error(''))
    renderPage()

    await waitFor(() =>
      expect(screen.getByTestId('world-templates-error')).toBeInTheDocument()
    )
    /* ⚠ 回落那句也要走 t()（否则换语言时它还是写死的那一种语言） */
    expect(screen.getByTestId('world-templates-error').textContent).toContain(
      'Could not read the plugin template list'
    )
  })

  test('★ 读不到这个账号的能力时，说清读不到，而且**按钮全禁用**', async () => {
    /*
     * ⚠ 关键判据是"**点不动**"，不是"不画"。
     *
     * 第一版读失败时按"没有能力"渲染，于是服务端其实答话失败、界面上却亮着两个
     * 「开通」按钮 —— 抢在那一下点下去可能给一个已经有能力的人再开一次。
     */
    worldApi.listEntitlements.mockRejectedValue({
      response: { data: { success: false, message: '账号不存在' } },
    })
    const user = userEvent.setup()
    renderPage()

    await screen.findByText('世界 IP 资源管理器')
    await user.type(screen.getByLabelText('Account ID'), '7')

    await waitFor(() =>
      expect(screen.getByTestId('world-entitlements-error')).toBeInTheDocument()
    )
    expect(screen.getByTestId('world-entitlements-error').textContent).toContain('账号不存在')

    /*
     * ★★ 这一条盯的是**那一块还在不在**：读失败那次渲染**不许把行连根拆掉**。
     * 判据是"两行都在"，而按钮点不动、状态写着读不到。
     */
    expect(rowOf('world-ip')).toBeInTheDocument()
    expect(screen.getByTestId('world-capability-state-world-ip')).toHaveTextContent(
      'Status unreadable'
    )
    for (const capability of ['world-ip', 'world-ip-ai']) {
      expect(actionOf(capability)).toBeDisabled()
    }
  })

  test('★ 第一次读还没回来时说"正在读"，而不是留一块空白', async () => {
    /*
     * ⚠ 行是**马上**就画出来的（名册来自模板），而"这个账号有没有"还在路上 ——
     * 所以那一刻要有一句话说清在等什么；否则运营填完 ID 看到的是"什么都没有"。
     */
    let release: (value: { userId: number; items: never[]; active: string[] }) => void = () => {}
    worldApi.listEntitlements.mockImplementation(
      () =>
        new Promise((resolve) => {
          release = resolve
        })
    )
    const user = userEvent.setup()
    renderPage()

    await screen.findByText('世界 IP 资源管理器')
    await user.type(screen.getByLabelText('Account ID'), '7')

    expect(await screen.findByTestId('world-entitlements-loading')).toBeInTheDocument()
    /* ★ 还没读到就按"未开通"画按钮是不行的 —— 那一下点下去可能给已有能力的人再开一次 */
    expect(actionOf('world-ip')).toBeDisabled()

    /* 读回来之后那句话消失，行照旧在 */
    release({ userId: 7, items: [], active: [] })
    await waitFor(() =>
      expect(screen.queryByTestId('world-entitlements-loading')).toBeNull()
    )
    expect(rowOf('world-ip')).toHaveAttribute('data-state', 'closed')
  })

  test('★ 历史行把「已取消」与「已过期」分开说', async () => {
    /*
     * ⚠ 两件事的下一步动作完全不同（一个要问为什么，一个要续期），
     * 合成一句"不生效"会让运营查错方向。
     */
    worldApi.listEntitlements.mockResolvedValue({
      userId: 7,
      active: [],
      items: [
        { id: 1, userId: 7, capability: 'world-ip', source: 'admin', createdAt: 1, expiresAt: null, revokedAt: 2 },
        { id: 2, userId: 7, capability: 'world-ip-ai', source: 'admin', createdAt: 1, expiresAt: 1, revokedAt: null },
      ],
    })
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')

    const history = await screen.findByTestId('world-history')
    expect(within(history).getByText(/Already cancelled/)).toBeInTheDocument()
    expect(within(history).getByText(/Expired/)).toBeInTheDocument()
  })
})
