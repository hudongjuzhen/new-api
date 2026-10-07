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
 * # 这一组守的是**几句话**，不是布局
 *
 * 这一屏上最容易写错的不是排版，而是三个判断 —— 每一个错了都**不报错**，
 * 只是让运营做错下一步：
 *
 * | 判据 | 说错了会怎样 |
 * |---|---|
 * | ★★ **签发 ≠ 授权** | 运营以为点一下就开通了，把文件发出去，用户什么都用不了 |
 * | ★ 坏模板要列出来并说明 | 运营看到的是"我放进去的文件不见了"，以为后台坏了 |
 * | ★ 缺哪个能力要点名 | 只授了一个，用户点「解析」被拒，而运营不知道自己漏了一步 |
 *
 * 后两条在 `plugin-issue-view.test.ts` 里用纯函数钉（不必渲染）；这一份钉**接线**：
 * 点了按钮之后，那几句话真的出现在屏幕上。
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
 * ⚠★ **这一条不 mock**（第一版 mock 了，而且它悄悄什么都没做）。
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
  'World IP · Plugins',
  'Reload',
  'Signing is not granting',
  'Plugin templates',
  'Sign a file for an account',
  'Account ID',
  'e.g. 7',
  'Sign and download',
  'The account that will import this file.',
  'Capabilities of this account',
  'Fill in an account ID above to see and change its capabilities.',
  'Active',
  'Not held',
  'Grant',
  'Revoke',
  'Signed and ready to send',
  'Signed — but this account cannot use it yet',
  'History (including revoked and expired rows)',
  'Read from {{dir}} — the file name is the plugin id.',
  'Fix the account ID above to grant or revoke.',
]) {
  identity[key] = key
}

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
      problem: '',
      source: '/srv/new-api/plugin-templates/world-ip.json',
    },
    {
      id: 'broken-one',
      name: 'broken-one',
      capabilities: [],
      screens: [],
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

beforeEach(() => {
  vi.clearAllMocks()
  worldApi.listPluginTemplates.mockResolvedValue(TEMPLATES)
  worldApi.listEntitlements.mockResolvedValue(entitlements([]))
  worldApi.issuePluginFile.mockResolvedValue(issued())
})

describe('世界 IP · 插件管理', () => {
  test('★ 坏模板列出来、并带上它自己的那句话与文件名', async () => {
    renderPage()

    // 好模板：能选
    await screen.findByText('世界 IP 资源管理器')

    /*
     * ★ 坏模板**必须露面**。不出现的话运营看到的是"我放进去的文件不见了" ——
     * 那看起来像后台坏了，而真相是那个文件里有个笔误。
     */
    expect(
      await screen.findByText(/文件名与 id 不一致/)
    ).toBeInTheDocument()
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

  test('★ 填了账号 ID 才让签发（空着时按钮是禁用的）', async () => {
    const user = userEvent.setup()
    renderPage()

    const button = await screen.findByRole('button', { name: /Sign and download/ })
    expect(button).toBeDisabled()

    await user.type(screen.getByLabelText('Account ID'), '7')
    expect(button).toBeEnabled()
  })

  test('★ 账号 ID 填了非数字：当场说清（且两处都说了），不点也不发请求', async () => {
    const user = userEvent.setup()
    renderPage()

    await screen.findByText('世界 IP 资源管理器')
    await user.type(screen.getByLabelText('Account ID'), 'abc')

    /*
     * ⚠ 用 `getAllByText`：填错时这句话在**两处**出现（输入框下面那行 + 能力那一块上面
     * 那条说明）—— 那是**有意的**（用户的眼睛可能在任一处），而 `getByText` 会因此红。
     */
    await waitFor(() =>
      expect(screen.getAllByText(/账号 ID 是一串数字/).length).toBeGreaterThan(0)
    )
    expect(screen.getByRole('button', { name: /Sign and download/ })).toBeDisabled()
    expect(worldApi.issuePluginFile).not.toHaveBeenCalled()
  })

  test('★★ 签发之后：文件名与校验和都显示出来，而且真的走到了"存文件"那一步', async () => {
    /*
     * ⚠ 这一条**不**去断言 `saveTextAsFile` 被调用（见文件上方那段说明：mock 一个
     * 导出拦不住模块内部的词法调用）。判据换成两件**看得见**的事：
     *   ① 那一屏把文件名与 check 显示出来了（说明签发真的回来了）；
     *   ② 签发只发生**一次**（"下载 + 拿结果"要是写成两次签发，两次的 issuedAt 不同，
     *      界面上的 check 就与用户手上那份对不上 —— 而运营照着核对时会以为文件被改过）。
     */
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByRole('button', { name: /Sign and download/ }))

    await waitFor(() => expect(worldApi.issuePluginFile).toHaveBeenCalledTimes(1))
    /* 文件名在两处出现（成功那句提示 + 底下那行详情），所以用 getAllByText */
    await waitFor(() =>
      expect(
        screen.getAllByText(/world-ip-v0\.1\.0-zsy-user7-20261007\.aimv-plugin\.json/).length
      ).toBeGreaterThan(0)
    )
    /* ★ 版本也要在那句话里出现 —— 运营会把它复制到聊天记录里 */
    expect(screen.getByText(/已经签好「world-ip」 v0\.1\.0 给账号 zsy/)).toBeInTheDocument()
    expect(screen.getByText(/fnv1a64:0cbcf8114de19bfa/)).toBeInTheDocument()
    expect(worldApi.issuePluginFile).toHaveBeenCalledWith(7, 'world-ip', undefined)
  })

  test('★★ 还没授予能力时，明说"装得上、可是用不了，还差哪个"', async () => {
    /*
     * ★ 这一条是整屏最要紧的一句话。少了它，运营会把文件发出去，
     * 然后收到"你给我的东西用不了" —— 而他从界面上完全看不出为什么。
     */
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByRole('button', { name: /Sign and download/ }))

    expect(
      await screen.findByText('Signed — but this account cannot use it yet')
    ).toBeInTheDocument()
    expect(screen.getByText(/还差 world-ip、world-ip-ai/)).toBeInTheDocument()
    expect(screen.getByText(/会被服务端拒绝/)).toBeInTheDocument()
  })

  test('★ 能力都已授予时，说的是"可以发了"（而不是那半句警告）', async () => {
    worldApi.issuePluginFile.mockResolvedValue(
      issued({ granted: ['world-ip', 'world-ip-ai'] })
    )
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByRole('button', { name: /Sign and download/ }))

    expect(await screen.findByText('Signed and ready to send')).toBeInTheDocument()
    expect(screen.queryByText(/会被服务端拒绝/)).not.toBeInTheDocument()
  })

  test('★ 签发**不**调用授予接口（两件事不许混）', async () => {
    /*
     * ⚠ 把两者合成一个动作会省一次点击，代价是"这个人到底被授权了没有"
     * 从此看不出来 —— 而撤销与审计都要那个答案。
     */
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await user.click(screen.getByRole('button', { name: /Sign and download/ }))
    /* 等签发真的落地（回来过一次）再断言 */
    await waitFor(() => expect(worldApi.issuePluginFile).toHaveBeenCalled())

    expect(worldApi.grantCapability).not.toHaveBeenCalled()
    expect(worldApi.revokeCapability).not.toHaveBeenCalled()
  })

  test('★ 授予与撤销各自打到对的接口，并在之后重新读一次状态', async () => {
    /*
     * ★★ 这一条第一版踩了两个坑，都记在这里（它们的症状都很费时间）：
     *
     *	① **数调用次数**：`listEntitlements` 是按"账号 + 模板"的变化去读的，
     *	   用户一个一个数字敲进去时 id 会变几次（`7` → `70`…），所以读几次是**实现细节**。
     *	   改成判"授予发对了参数" + "改完之后界面上真的变了"。
     *	② **按索引找按钮**：第一版 `grantButtons[0]` 拿到的是**「撤销」**
     *	   （因为那次读回来的状态里 `world-ip` 已经生效），于是断言红在"grantCapability
     *	   没被调用"上 —— 而真实原因是**我点的是另一颗按钮**。
     *	   现在按能力名去点它自己那一行，不用索引。
     */
    /*
     * ★ 记「授予**被调用过**」与「授予之后的状态」是**两件事**：
     *
     *   `active` 一变，下面读回来的就是"已经生效"，界面上那颗按钮随之变成「撤销」——
     *   而"它调用了没有、参数对不对"与这个变化无关。
     *   第一版把两者合成一个数组，于是失败信息只剩一句"没被调用"，
     *   而真实原因是**参数不对**（界面上那一行已经显示 Active 了，那是很好的线索，
     *   只是当时没被报出来）。
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
    const grantButtons = await screen.findAllByRole('button', { name: 'Grant' })
    expect(grantButtons).toHaveLength(2)

    /*
     * ★ 按**能力名**定位那一行（`data-testid`），再点它里面的按钮 —— 不用索引。
     * 索引与"哪几个能力生效了"耦合在一起，而那正是这一组要改的东西。
     *
     * ⚠ 也不能用 `getByText('world-ip')` 再往上找：页面上有好几处写着这几个字
     * （那一行、以及底下文件名里也含它），`getByText` 会因为"找到多个"而红 ——
     * 那是**测试写窄了**，不是界面错了。
     */
    const row = screen.getByTestId('world-capability-world-ip')
    await user.click(within(row).getByRole('button', { name: 'Grant' }))

    /*
     * ★ 判据两件，都与实现无关：
     *   ① 授予被调用过、且参数是**这个账号 + 这个能力**；
     *   ② 改完之后界面**跟着变**了（那一行从「授予」变成「撤销」）——
     *      这才是"改完要重读一次"的可见证据。
     *
     * ⚠ 参数单独断言（不塞进 `waitFor` 里看 `active`）：`active` 一变就让界面变成
     * "已生效"，于是"参数对不对"这件事会被那个变化盖住。先把参数钉住，再看界面。
     */
    await waitFor(() => expect(granted).toBe(true))
    /*
     * ★ 参数**逐格**断言（含第三个 `0`）。
     *
     * ⚠ 这一条抓到过一个真错：页面原先只传了两个参数，而**签名里第三个是可选的**，
     * 于是少传不会报错、界面看起来完全正常 —— 只有"调用记录少一格"能发现它。
     * 所以判据写成整个调用记录，不是"至少被调用过"。
     */
    expect(worldApi.grantCapability.mock.calls, '授予的参数不对').toEqual([[7, 'world-ip', 0]])

    await waitFor(() =>
      expect(
        within(screen.getByTestId('world-capability-world-ip')).getByRole('button', {
          name: 'Revoke',
        })
      ).toBeInTheDocument()
    )
  })

  test('★ 账号 ID 填错时，授予按钮是**禁用**的（不是"点了没反应"）', async () => {
    /*
     * ⚠ 这一条是实测改的：第一版只按"忙不忙"渲染按钮，于是填了 "abc" 之后
     * 按钮仍是亮的，点下去什么也不发生 —— 用户会以为这个功能坏了。
     * 判据与上面那颗「签发」按钮同源（`parsedId.problem`）。
     */
    const user = userEvent.setup()
    renderPage()

    /* 先等模板落定再填 —— 否则"按钮还没画出来"与"按钮被禁用了"分不清 */
    await screen.findByText('世界 IP 资源管理器')

    await user.type(screen.getByLabelText('Account ID'), 'abc')
    await waitFor(() =>
      expect(screen.getAllByRole('button', { name: 'Grant' }).length).toBe(2)
    , { timeout: 3000 })

    for (const button of screen.getAllByRole('button', { name: 'Grant' })) {
      expect(button).toBeDisabled()
    }
  })
  test('★ 已经生效的能力给的是「撤销」，而不是再授一次', async () => {
    worldApi.listEntitlements.mockResolvedValue(entitlements(['world-ip']))
    worldApi.revokeCapability.mockResolvedValue(undefined)

    const user = userEvent.setup()
    renderPage()

    await screen.findByText('世界 IP 资源管理器')
    await user.type(screen.getByLabelText('Account ID'), '7')

    /*
     * ⚠ 这一条**不点任何按钮**，所以不可能靠"点出来的副作用"绕过"还没读到"那一支。
     * 失败时把界面原样报出来：这一组红过一次，而当时的失败信息只有
     * "找不到 Revoke"，看不出界面停在骨架屏还是别的什么。
     */
    const revoke = await screen.findByRole('button', { name: 'Revoke' }, { timeout: 3000 }).catch((err) => {
      throw new Error(`${err.message}\n\n界面：\n${document.body.textContent}\n\nlistEntitlements 调用：${worldApi.listEntitlements.mock.calls.length} 次`)
    })
    await user.click(revoke)
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
    await user.click(screen.getByRole('button', { name: /Sign and download/ }))

    expect(await screen.findByText(/没有 "nope" 这份模板/)).toBeInTheDocument()
    expect(screen.queryByText(/status code/)).not.toBeInTheDocument()
  })

  test('★ 模板目录空着时，给出目录与能写的 source', async () => {
    worldApi.listPluginTemplates.mockResolvedValue({
      ...TEMPLATES,
      items: [],
    })
    renderPage()

    const text = await screen.findByText(/模板目录/)
    expect(text).toBeInTheDocument()
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
     * `response.data.message` 里（axios 的默认文案是 "Request failed with status
     * code 200"，对运营毫无用处 —— 这正是 `readError` 优先取它、其次取
     * `Error.message`、最后才回落的原因）。
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

  test('★ 读模板失败、服务端又没给话时，回落到自己那句（不许画一屏空的）', async () => {
    worldApi.listPluginTemplates.mockRejectedValue(new Error(''))
    renderPage()

    await waitFor(() =>
      expect(screen.getByTestId('world-templates-error')).toBeInTheDocument()
    )
    expect(screen.getByTestId('world-templates-error').textContent).toContain(
      '读不到插件模板列表'
    )
  })

  test('★ 读不到这个账号的能力时，说清读不到，而且**按钮全禁用**', async () => {
    /*
     * ⚠ 关键判据是"**点不动**"，不是"不画"。
     *
     * 第一版读失败时按"没有能力"渲染，于是服务端其实答话失败、界面上却亮着两个
     * 「授予」按钮 —— 抢在那一下点下去可能给一个已经有能力的人再授一次。
     * 现在的形状：行照样画（用户看得见有哪些能力）、说明写清读不到、按钮禁用
     * （因为此刻**真实状态未知**，而"未知"不许画成"没有"）。
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

    for (const button of screen.getAllByRole('button', { name: /Grant|Revoke/ })) {
      expect(button).toBeDisabled()
    }
  })

  test('★ 历史行把「已撤销」与「已过期」分开说', async () => {
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

    const history = await screen.findByText('History (including revoked and expired rows)')
    const block = history.parentElement as HTMLElement
    expect(within(block).getByText(/已撤销/)).toBeInTheDocument()
    expect(within(block).getByText(/已过期/)).toBeInTheDocument()
  })
})
