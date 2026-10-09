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
 * 模式管理那一屏（`zsy/mode` 的后台面）。
 *
 * # 这一组守的是**接线**，不是布局
 *
 * 判据（"该说什么"）在 `mode-admin-view.test.ts` 里用纯函数钉过；这一份钉
 * **点了之后屏幕上真的出现那句话**：
 *
 * | 判据 | 说错了会怎样 |
 * |---|---|
 * | ★★ 空着 ID 时右边是「编辑」，填了 ID 之后换成「开通 / 取消开通」 | 运营找不到入口，或对着一个已经有权限的人再开一次 |
 * | ★★ 读不到账号状态时**不画成"未开通"** | 运营对着一个其实已经有权限的人再开一次 |
 * | ★★ 开通 / 取消之后**重新读**（不按点击推） | 界面上写着"已开通"、用户那边看不到 |
 * | ★ 一行放好几个（不是一档一行） | 用户报的那件事：太浪费空间 |
 *
 * # ★★ 第三版（用户 2026-…）：左 ID / 右列表，而且文案跟着语言走
 *
 * 用户的原话：
 *
 * > "我希望左侧是输入ID的位置，右侧是选择的列表，不要每个模式或者每个插件单独占一行，
 * >  太浪费空间了。另外当输入account id的输入框是空的的时候，右侧的模式有编辑功能，
 * >  只要输入框有内容了，右侧模式的 编辑 按钮隐藏，从而显示 给权限 或者 取消权限 的按钮。
 * >  另外页面要适配国际化……"
 *
 * ⚠★ 断言查的是 `t()` 的**英文 key**（恒等翻译），所以"每一句话都有翻译"
 * 这件事由语言包负责 —— 而"有没有哪句话没走 `t()`"由这一组钉：
 * 只要界面上出现了硬编码的中文，这里的英文断言就会红。
 */
const modeApi = vi.hoisted(() => ({
  listModes: vi.fn(),
  listEntitlementsByUser: vi.fn(),
  listEntitlementsByMode: vi.fn(),
  grantMode: vi.fn(),
  revokeMode: vi.fn(),
  updateModeMeta: vi.fn(),
  /* ★★ 编辑弹窗如今要读**正文**（表单 + 原始 JSON 页签），所以这几条也要替身 */
  getModeContent: vi.fn(),
  saveModeContent: vi.fn(),
  listModeAiKeys: vi.fn(),
  createMode: vi.fn(),
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  ...modeApi,
}))

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { ModePlazaPage } = await import('../mode-plaza-page')

/* 断言查的是组件传给 t() 的**英文原文**，所以把它们登记成恒等翻译 */
const identity: Record<string, string> = {}
for (const key of [
  'Mode Plaza',
  'Reload',
  'Private modes are hidden, not greyed out',
  'Private modes are hidden, not greyed out. An account that was not opened for one does not see it in the plaza at all — it is not shown greyed out either. After you open it, that user has to refresh that screen for it to appear. And after you cancel it, the copy already downloaded to that machine is not deleted: that is deliberate, because that file is the user’s asset (they may have edited it or built a project on it), and cancelling only means “no new copies will be sent”.',
  'Mode library',
  'Modes that cannot be shipped',
  'These files are in the mode directory but cannot be handed to a client. Fix the file, then click Reload.',
  'Who holds “{{mode}}”',
  'No mode selected.',
  'Accounts',
  'View the accounts that hold {{mode}}',
  'Close',
  'Open modes for an account',
  'Account ID',
  'e.g. 7',
  'Leave it empty to edit how each mode is handed out; fill it in to open or cancel that account’s modes.',
  'Open',
  'Cancel access',
  'Open {{mode}} for this account',
  'Cancel access for {{mode}}',
  'Opened',
  'Not opened',
  'Status unreadable',
  'Modes opened for this account',
  'Who holds the selected mode',
  'Pick a mode above.',
  'No account holds this mode yet.',
  'This mode is public — every account can open it, so there is nobody to list.',
  'In effect',
  'Already cancelled',
  'Expired',
  'History (including cancelled and expired rows)',
  'Read from {{dir}} — the file name is the plugin id.',
  'Every mode the library holds, including the ones that cannot be shipped. Public modes are open to every account already; private ones appear in the plaza only for the accounts opened below.',
  'Public modes need no opening — every account can open them. The state here tells you whether this account was opened for it individually.',
  'Modes cannot be opened or cancelled until this is read.',
  'Public mode — anyone can open it',
  'Private mode — opened per account',
  'Public — no opening needed',
  'Private — nobody holds it yet',
  'Public — plus {{count}} account(s) opened individually',
  'Private — opened for {{count}} account(s)',
  'That count could not be read',
  'The server did not report a directory',
  'The server did not report',
  'The mode directory {{dir}} holds no usable mode yet. Put a mode JSON into it (the file name is the mode id, for example mv.json), then click Reload. Its medium may only be one of these: {{mediums}}.',
  'Could not read the mode list',
  'The library is a directory on the server: adding or removing a mode is adding or removing a file, never a database migration.',
  /* ★ 「编辑一档模式」那几张文案（`ModeMetaDialog`） */
  'Edit',
  'Edit mode',
  'Who can open it',
  'One-line summary',
  'Shown on the plaza card.',
  'Public: every account sees it in the plaza and can open it with one click — including accounts that are not signed in.',
  'Private: only the accounts you grant it to below can see it in the plaza at all. It does not appear greyed out for anyone else.',
  'Cancel',
  'Save',
  /* ★ 「编辑一档模式」改大之后那几张（表单 / 原始 JSON 页签） */
  'Left: who can open it and the one-line summary. Right: the mode itself — the words the interface uses, the tables it collects, and the raw JSON.',
  'These fields are the ones operators change most. Everything else (the planning instruction, the templates, the extra buttons…) is edited on the JSON tab — the server checks the whole file again before it is saved.',
  'This mode',
  'Name',
  'Type',
  'Work scale',
  'One of: {{values}}',
  'What this mode requires',
  'These decide which inputs the planning dialog insists on. Audio and segments are usually required; lyrics and references usually are not.',
  'The words the interface uses',
  '{{count}} fields',
  'Every one of them decides a label the user reads. An empty field falls back to what the app ships with, so leaving one blank is safe.',
  'The pre-flight checklist shown in the planning dialog:',
  'Library tables',
  '{{count}} tables',
  'Each table is one kind of thing this mode collects (characters, locations, products…). Only the id and the label are editable here; the fields of a table are on the JSON tab.',
  'Raw JSON',
  'Form',
  'This is the whole file. Editing it and saving writes every field, including the ones the form above does not show.',
  'The JSON is not valid yet: {{message}}',
  'Could not read this mode’s content',
  /* ★★ 「添加模式」（AI 生成）那几张 */
  'Add a mode',
  'Describe the kind of work this mode is for, pick one of your API keys, and the model writes the whole mode file. You can refine it afterwards with Edit.',
  'What is this mode for?',
  'Describe the work: what it is for, who it is for, and anything the model must get right. The more concrete this is, the better the generated mode fits.',
  'API key',
  'Choose a key',
  'This request goes through your own relay with model {{model}}.',
  'This request goes through your own relay.',
  'New modes start private — open them per account, or make them public here.',
  'Generate with AI',
  'Generating…',
  'e.g. Food tour shorts',
  'Could not read your API keys',
  'Could not generate this mode',
  'Video',
  'Audio',
  'Text',
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
      <ModePlazaPage />
    </I18nextProvider>
  )
}

const DIR = '/srv/new-api/modes'

/** 三档：一公共、一私有、一坏的。 */
const MODES = {
  directory: DIR,
  knownMediums: ['video', 'audio', 'text'],
  note: '模式库是一个目录：一份模式 = 一个 .json（文件名就是模式 id）。',
  items: [
    {
      id: 'mv',
      label: 'MV',
      medium: 'video',
      workScale: 'single',
      summary: '一句话说明',
      bytes: 12000,
      visibility: 'public',
      granted: 0,
      problem: '',
      source: `${DIR}/mv.json`,
    },
    {
      id: 'audiobook',
      label: '有声书',
      medium: 'audio',
      workScale: 'single',
      summary: '音频那一档',
      bytes: 13000,
      visibility: 'private',
      granted: 0,
      problem: '',
      source: `${DIR}/audiobook.json`,
    },
    {
      id: 'bad',
      label: 'bad',
      medium: 'video',
      workScale: 'single',
      summary: '',
      bytes: 10,
      visibility: 'private',
      granted: 0,
      problem: '文件名与 id 不一致：文件名说 "bad"，而里面的 id 是 "other"。',
      source: `${DIR}/bad.json`,
    },
  ],
}

/**
 * 编辑弹窗要读的那一份**正文**（一份最小的合法模式文件）。
 *
 * ⚠ 它是**真的形状**：表单那几个输入框靠它渲染，而"同一份草稿在两种画法之间
 * 不许各说各的"这件事也正是靠它验的。
 */
const MODE_CONTENT = {
  format: 'aimv-work-mode',
  formatVersion: 1,
  'x-visibility': 'private',
  'x-summary': '音频那一档',
  id: 'audiobook',
  label: '有声书',
  medium: 'audio',
  workScale: 'single',
  words: { audio: '导入音频', text: '字幕', planSelfCheck: { musicName: '音频' } },
  requires: { audio: true, lyrics: false, refs: false, segments: true },
  libraries: [{ id: 'works', label: '作品库', fields: [{ key: 'name', label: '名称', type: 'text' }] }],
  planDialog: {
    label: 'AI 全稿规划',
    instruction: { skeleton: '（骨架）' },
    templates: ['（默认模板）'],
  },
}

function byUser(active: string[] = [], items: unknown[] = []) {
  return { userId: 7, items, active }
}

function byMode(modeId: string, items: unknown[] = []) {
  return { modeId, items }
}

/**
 * 右栏里某一档的那一行。
 *
 * ⚠ 按钮上的**字**只有两三个字（一行放好几个的前提），所以按 `aria-label`
 * （"Open MV for this account"）找它 —— 那也正是读屏用户听到的名字。
 */
function rowOf(modeId: string): HTMLElement {
  return screen.getByTestId(`mode-grant-${modeId}`)
}

/**
 * 某一档的「开通 / 取消开通」那一枚按钮。
 *
 * ⚠★ 按钮上的**字**只有两三个字（一行放好几个的前提），所以按 `aria-label`
 * （"Open MV for this account"）找它 —— 那也正是读屏用户听到的名字。
 */
function actionOf(modeId: string): HTMLElement {
  const row = rowOf(modeId)
  const name = row.querySelector('button[aria-label]')?.getAttribute('aria-label')
  return within(row).getByRole('button', { name: name as string })
}

/**
 * 打开某一档的「谁有这一档的权限」弹窗（用户 2026-… 点名要的那颗按钮）。
 *
 * ⚠★ 它**只在左边那一格空着时**出现 —— 填了 ID 之后同一个位置放的是
 * 「开通 / 取消开通」（运营那时关心的是"这个人"，不是"谁还有这一档"）。
 */
async function openHolders(user: ReturnType<typeof userEvent.setup>, modeId: string) {
  await user.click(screen.getByTestId(`mode-holders-${modeId}`))
}

beforeEach(() => {
  vi.clearAllMocks()
  modeApi.listModes.mockResolvedValue(MODES)
  modeApi.listEntitlementsByUser.mockResolvedValue(byUser([]))
  modeApi.listEntitlementsByMode.mockResolvedValue(byMode('mv'))
  modeApi.grantMode.mockResolvedValue(undefined)
  modeApi.revokeMode.mockResolvedValue(undefined)
  modeApi.updateModeMeta.mockResolvedValue({ ...MODES.items[0] })
  modeApi.getModeContent.mockResolvedValue(MODE_CONTENT)
  modeApi.saveModeContent.mockResolvedValue({ ...MODES.items[0] })
  modeApi.listModeAiKeys.mockResolvedValue({
    items: [
      { id: 3, name: '运维密钥', keyPrefix: 'sk-abcd…', status: 1, usable: true, problem: '' },
      { id: 4, name: '停用的', keyPrefix: 'sk-efgh…', status: 2, usable: false, problem: '这个密钥被禁用了（在「令牌」页里把它开启）' },
    ],
    model: 'glm-5.3-flash',
  })
  modeApi.createMode.mockResolvedValue({ ...MODES.items[0] })
})

describe('模式管理', () => {
  test('★ 坏模式列出来、并带上它自己的那句话与文件名', async () => {
    renderPage()

    /*
     * ★ 坏模式在**自己那一张卡**里（用户 2026-… 撤掉了「模式库」那一块，
     * 而坏文件不能跟着它一起消失：它是运营唯一要动手去修的东西）。
     */
    await screen.findByTestId('mode-broken-bad')
    expect(screen.getByText(/文件名与 id 不一致/)).toBeInTheDocument()
    expect(screen.getByText(`${DIR}/bad.json`)).toBeInTheDocument()
  })

  test('★★ 一进这一屏就写明"私有模式是看不到，不是灰的"', async () => {
    renderPage()

    /*
     * ⚠ 这一条横幅是常驻的，不是点完才出现 —— 它回答的是运营**动手之前**就该知道的
     * 那件事（他会以为用户能看见、只是点不动）。
     */
    await screen.findByText('Private modes are hidden, not greyed out')
    expect(screen.getByTestId('mode-private-note')).toBeInTheDocument()
  })

  test('★★ 已开通那一行是**深色（反色）**的；未开通与没填 ID 的**保持原样**', async () => {
    /*
     * ★★ 用户 2026-… 点名要的：
     *
     * > "输入了ID之后，已开通和未开通的背景颜色换一下吧，未开通和没有输入ID的颜色
     * >  是一样的，保持现在这样就行，已开通的弄个深色主题，是不是就更好了"
     *
     * ⚠★ 判据落在 `data-tone` 上（那是**这一行对外的状态**），而不是某串 class：
     * 具体用哪个颜色 token 是排版细节（这里用 `bg-foreground text-background`，
     * 因为站点有十套主题预设 + 各自的暗色版，写死一种深灰会在暗色下糊掉），
     * 而"哪一行被染了色"才是这一条要守的东西。
     */
    const user = userEvent.setup()
    /* 这个账号手上有 mv（公共那一档），audiobook 没有 */
    modeApi.listEntitlementsByUser.mockResolvedValue(byUser(['mv']))
    renderPage()

    /* ① 没填 ID：两行都保持原样（用户点名要"保持现在这样"） */
    await screen.findByTestId('mode-grant-mv')
    expect(rowOf('mv')).toHaveAttribute('data-tone', 'plain')
    expect(rowOf('audiobook')).toHaveAttribute('data-tone', 'plain')
    expect(rowOf('audiobook').className).not.toContain('bg-foreground')

    /* ② 填了 ID：已开通那一行反色，未开通那一行**仍然原样** */
    await user.type(screen.getByLabelText('Account ID'), '7')
    await waitFor(() => expect(rowOf('mv')).toHaveAttribute('data-tone', 'inverted'))

    expect(rowOf('mv').className).toContain('bg-foreground')
    expect(rowOf('mv').className).toContain('text-background')
    expect(rowOf('audiobook')).toHaveAttribute('data-tone', 'plain')
    expect(rowOf('audiobook').className).not.toContain('bg-foreground')

    /*
     * ⚠★ 反色那一行里**每一处字**都要跟着换色：`text-muted-foreground` 是浅灰，
     * 压在深色底上几乎看不见 —— 而它**不会报错**，只是有一个字读不出来。
     */
    const inverted = rowOf('mv')
    expect(within(inverted).getByTestId('mode-state-mv').className).toContain('text-background')
    /* ⚠ 而"未开通"那一行的状态签**不许**跟着换（它是原来的样子） */
    expect(within(rowOf('audiobook')).getByTestId('mode-state-audiobook').className).not.toContain(
      'text-background'
    )
  })

  test('★★ 状态读不到时也反色（"未知"不许画成"没有"）', async () => {
    /*
     * ⚠★ 它与"已开通"是**同一句"这个人现在可能能用"**：读不到时那一格写着
     * "状态读不到"、按钮禁用，而底色若画成"未开通"的样子，运营会把它读成
     * "他没有这一档" —— 然后去点开通（服务端会多插一行，哪里都不报错）。
     */
    const user = userEvent.setup()
    modeApi.listEntitlementsByUser.mockRejectedValue({
      response: { data: { message: '读不到这个账号的模式授权（数据库连接失败）。' } },
    })
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')

    await waitFor(() => expect(rowOf('mv')).toHaveAttribute('data-tone', 'inverted'))
    expect(rowOf('mv')).toHaveAttribute('data-state', 'unknown')
    expect(screen.getByTestId('mode-state-mv')).toHaveTextContent('Status unreadable')
  })

  test('★★ 「模式库」那一张卡没有了：不再是"下面一块、上面一块"两份名单', async () => {
    /*
     * 用户 2026-…：
     *
     * > "下面的 模式库 就没有必要显示了吧，直接在上面右侧的模式列表就可以了"
     *
     * ⚠ 判据是"屏幕上关于"有哪几档"的名单**只有一处**"：那一处就是右栏的网格。
     */
    renderPage()

    const grid = await screen.findByTestId('mode-account-grid')
    expect(within(grid).getAllByTestId(/^mode-grant-/)).toHaveLength(2)
    /* ⚠ 原来模式库那一块的 testid 一个都不该在 */
    expect(screen.queryByTestId('mode-row-mv')).toBeNull()
    expect(screen.queryByTestId('mode-edit-library-mv')).toBeNull()
  })

  test('★★ 左 ID / 右列表：两块在**同一个横向容器**里，窄屏才叠起来', async () => {
    renderPage()

    const input = await screen.findByLabelText('Account ID')
    /* 左栏 → 外面那个"左 ID / 右列表"的容器 */
    const left = input.parentElement as HTMLElement
    const split = left.parentElement as HTMLElement

    /*
     * ⚠ 判据是**排列方向与断点**，不是某一串完整 class：这一条要保护的就是
     * "左边填 ID、右边是列表"这件事。
     * ⚠ 断点用 `md`（48rem）—— 左边只有一格输入框，早点并排右边才能早点多一列。
     */
    expect(split.className).toContain('flex')
    expect(split.className).toContain('flex-col')
    expect(split.className).toContain('md:flex-row')
    /* 左边那一格是窄栏，右边那一栏吃掉剩下的宽度 */
    expect(left.className).toContain('md:w-56')

    /* ⚠ 而且右栏确实在 split 里面（不是被挪到别处去了） */
    const grid = screen.getByTestId('mode-account-grid')
    expect(split.contains(grid)).toBe(true)
  })

  test('★★ 一行放好几个：列数按**可用宽度**算，不看视口断点', async () => {
    /*
     * ⚠★★ 这一条是**回归**判据，而它保护的是一个真发生过的错（用户 2026-…）：
     *
     * > "模式库还是在上面一行一个，太丑了，也没变化呀"
     *
     * 原因：列数写成了 `xl:grid-cols-2`，而 Tailwind 4 的 `xl` = 1280px 看的是
     * **视口**宽度 —— 这一屏的内容区被侧边栏与内边距吃掉一截，那个断点**从来没生效**，
     * 于是永远只有一列。测试当时查的是"class 里有没有 xl:grid-cols-2"，
     * 于是它绿着、而界面上是一行一个。
     *
     * 所以判据换成**真正决定列数的那件事**：`auto-fill` + 一个**最小列宽**
     * （大屏自动多列、窄屏自动一列，不再需要猜一个宽度）。
     */
    renderPage()

    /* ⚠ 现在**只有一处**网格了（「模式库」那一张卡已经撤掉）——判据落在它身上 */
    const grid = await screen.findByTestId('mode-account-grid')

    {
      expect(grid.className).toContain('grid')
      expect(grid.style.gridTemplateColumns).toContain('auto-fill')
      expect(grid.style.gridTemplateColumns).toContain('minmax(')
      /* ⚠ 不许再用"按视口分列"的写法（那正是失效的那一个） */
      expect(grid.className).not.toContain('grid-cols-1')
      expect(grid.className).not.toMatch(/xl:grid-cols/)
    }
  })

  test('★★ 空着 ID：右边给的是「编辑」；填上 ID：换成「开通 / 取消开通」', async () => {
    /*
     * ★★ 用户点名要的那一条：
     *
     * > "当输入account id的输入框是空的的时候，右侧的模式有编辑功能，只要输入框有内容了，
     * >  右侧模式的 编辑 按钮隐藏，从而显示 给权限 或者 取消权限 的按钮。"
     */
    const user = userEvent.setup()
    renderPage()

    await screen.findByTestId('mode-grant-audiobook')

    /*
     * 空着：每一行是「编辑」**加**「Accounts」（用户 2026-… 要的那颗按钮），
     * 而且它们就在这一行里。
     */
    await waitFor(() => expect(screen.getByTestId('mode-edit-audiobook')).toBeInTheDocument())
    expect(screen.getByTestId('mode-holders-audiobook')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /for this account$/ })).toBeNull()
    expect(rowOf('audiobook')).toHaveAttribute('data-state', 'editing')

    await user.type(screen.getByLabelText('Account ID'), '7')

    /* 填了：编辑与 Accounts 都没了，换成开通（这一档没开通） */
    await waitFor(() =>
      expect(screen.queryByTestId('mode-edit-audiobook')).not.toBeInTheDocument()
    )
    expect(screen.queryByTestId('mode-holders-audiobook')).not.toBeInTheDocument()
    expect(actionOf('audiobook')).toHaveTextContent('Open')
    await waitFor(() => expect(rowOf('audiobook')).toHaveAttribute('data-state', 'closed'))
  })

  test('★★ 填一次 ID：每一档都带出"已开通 / 未开通"（不必先选模式）', async () => {
    const user = userEvent.setup()
    /* 这个账号手上有 mv（公共那一档），而 audiobook 没开通 */
    modeApi.listEntitlementsByUser.mockResolvedValue(byUser(['mv']))
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')

    /*
     * ★★ 这一条就是用户要的那件事：**一次输入，全部模式的状态一起出来**。
     */
    await waitFor(() => expect(rowOf('mv')).toHaveAttribute('data-state', 'open'))
    expect(rowOf('audiobook')).toHaveAttribute('data-state', 'closed')
    expect(screen.getByTestId('mode-state-mv')).toHaveTextContent('Opened')
    expect(screen.getByTestId('mode-state-audiobook')).toHaveTextContent('Not opened')

    /* 已开通那一档给的是「取消」，未开通那一档给的是「开通」 */
    expect(actionOf('mv')).toHaveTextContent('Cancel access')
    expect(actionOf('audiobook')).toHaveTextContent('Open')
  })

  test('★★ 空 ID 时点「Accounts」→ 弹窗列出这一档给了谁（含空名单那句话）', async () => {
    /*
     * ★★ 用户 2026-… 点名要的那颗按钮：
     *
     * > "再加上一个 点击查看拥有权限的账号列表 的按钮，点击出来对应的弹窗可以显示"
     */
    const user = userEvent.setup()
    modeApi.listEntitlementsByMode.mockResolvedValue(
      byMode('audiobook', [
        { id: 1, userId: 7, modeId: 'audiobook', source: 'admin', createdAt: 1, expiresAt: null, revokedAt: null },
        { id: 2, userId: 9, modeId: 'audiobook', source: 'admin', createdAt: 1, expiresAt: null, revokedAt: null },
      ])
    )
    renderPage()

    await screen.findByTestId('mode-holders-audiobook')
    await openHolders(user, 'audiobook')

    /* 弹窗打开，而且**是按这一档**去问的（服务端拒绝全表列举） */
    expect(await screen.findByTestId('mode-holders-list')).toBeInTheDocument()
    expect(modeApi.listEntitlementsByMode).toHaveBeenCalledWith('audiobook')
    const list = screen.getByTestId('mode-holders-list')
    expect(within(list).getByText('7')).toBeInTheDocument()
    expect(within(list).getByText('9')).toBeInTheDocument()
  })

  test('★★ 空名单那句话按可见性分开：私有说"还没有人"，公共说"谁都能开通"', async () => {
    const user = userEvent.setup()
    /* mv 是公共那一档，而且名单是空的 */
    modeApi.listEntitlementsByMode.mockResolvedValue(byMode('mv', []))
    renderPage()

    await screen.findByTestId('mode-holders-mv')
    await openHolders(user, 'mv')

    expect(await screen.findByTestId('mode-no-holders')).toHaveTextContent(
      'This mode is public — every account can open it'
    )
  })

  test('★ 名单读不到时说读不到（**不画成"还没有人"**）', async () => {
    /*
     * ⚠★ 那个空名单是一个**结论**（"这一档没人有权限"），而运营会照着它去重复授权
     * —— 服务端会多插一行，哪里都不报错。所以读失败必须说读失败。
     */
    const user = userEvent.setup()
    modeApi.listEntitlementsByMode.mockRejectedValue({
      response: { data: { message: '读不到这一档的开通名单。' } },
    })
    renderPage()

    await screen.findByTestId('mode-holders-audiobook')
    await openHolders(user, 'audiobook')

    expect(await screen.findByTestId('mode-holders-error')).toHaveTextContent(
      '读不到这一档的开通名单。'
    )
    expect(screen.queryByTestId('mode-no-holders')).toBeNull()
  })

  test('★ 名单看完之后，那一行上"已给 N 个账号开通"跟着更新', async () => {
    const user = userEvent.setup()
    modeApi.listEntitlementsByMode.mockResolvedValue(
      byMode('audiobook', [
        { id: 1, userId: 7, modeId: 'audiobook', source: 'admin', createdAt: 1, expiresAt: null, revokedAt: null },
        { id: 2, userId: 9, modeId: 'audiobook', source: 'admin', createdAt: 1, expiresAt: null, revokedAt: null },
      ])
    )
    renderPage()

    /* 列表里那份说"还没有给任何账号开通" */
    expect(await screen.findByTestId('mode-granted-audiobook')).toHaveTextContent(
      'Private — nobody holds it yet'
    )

    await openHolders(user, 'audiobook')
    await screen.findByTestId('mode-holders-list')

    /* ⚠ 要么重读列表、要么用名单那一次的结果回填 —— 两条路都行，**但必须在界面上变** */
    await waitFor(() =>
      expect(screen.getByTestId('mode-granted-audiobook')).toHaveTextContent('2')
    )
  })

  test('★ 账号 ID 填了非数字：当场说清，而且一个请求都不发', async () => {
    const user = userEvent.setup()
    renderPage()

    await screen.findByTestId('mode-grant-mv')
    await user.type(screen.getByLabelText('Account ID'), 'abc')

    await waitFor(() =>
      expect(
        screen.getAllByText((_, el) => /run of digits/.test(el?.textContent ?? '')).length
      ).toBeGreaterThan(0)
    )
    /*
     * ⚠ 填了东西就**不是**"编辑那一副面孔"了（按钮换成开通/取消）；
     * 而 ID 不合法时那一枚按钮**点不动** —— 否则会发出一个 userId=0 的请求。
     */
    expect(screen.queryByTestId('mode-edit-mv')).not.toBeInTheDocument()
    expect(within(rowOf('mv')).getByRole('button')).toBeDisabled()
    expect(modeApi.grantMode).not.toHaveBeenCalled()
    expect(modeApi.listEntitlementsByUser).not.toHaveBeenCalled()
  })

  test('★ 第一次读还没回来时：每一行已经在，状态写着"读不到"，按钮点不动', async () => {
    /*
     * ⚠ 行是**马上**就画出来的（名册来自模式库），而"这个账号有没有开通"还在路上。
     * 那一刻的状态是**未知** —— 未知不许画成"未开通"（点一下"开通"可能给一个已经有
     * 权限的人再插一行），所以那一格写着"状态读不到"、按钮禁用。
     */
    let release: (value: { userId: number; items: never[]; active: string[] }) => void = () => {}
    modeApi.listEntitlementsByUser.mockImplementation(
      () =>
        new Promise((resolve) => {
          release = resolve
        })
    )
    const user = userEvent.setup()
    renderPage()

    await screen.findByTestId('mode-grant-mv')
    await user.type(screen.getByLabelText('Account ID'), '7')

    await waitFor(() => expect(rowOf('audiobook')).toHaveAttribute('data-state', 'unknown'))
    expect(screen.getByTestId('mode-state-audiobook')).toHaveTextContent('Status unreadable')
    for (const row of [rowOf('mv'), rowOf('audiobook')]) {
      expect(within(row).getByRole('button')).toBeDisabled()
    }

    /* 读回来之后**还是那一块**（不换实例），只是状态变成"未开通" */
    release({ userId: 7, items: [], active: [] })
    await waitFor(() => expect(rowOf('audiobook')).toHaveAttribute('data-state', 'closed'))
  })

  test('★★ 在某一档上点「开通」→ 发出的是 user_id + mode_id + expires_at:0，然后**重新读**', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    /* 私有那一档（`audiobook`）—— 它才是"要开通才能看见"的那一类 */
    await waitFor(() => expect(rowOf('audiobook')).toHaveAttribute('data-state', 'closed'))

    const grantCalls = modeApi.listEntitlementsByUser.mock.calls.length
    await user.click(actionOf('audiobook'))

    await waitFor(() => expect(modeApi.grantMode).toHaveBeenCalledTimes(1))
    /*
     * ⚠★ 第三个参数**必须是 0**（不是省略）：服务端的 `expires_at` 缺省即"永不过期"，
     * 所以少传**不会报错** —— 但"参数个数"因此不再是契约的一部分，而界面看起来
     * 完全正常。与 `zsy-world` 那一处同一条教训。
     */
    expect(modeApi.grantMode).toHaveBeenCalledWith(7, 'audiobook', 0)

    /*
     * ★★ 开通之后一律**重新读**（不按点击推）：服务端的判据是每次现算的
     * （有过期、有取消），本地推一份就可能分叉 —— 而分叉的表现是界面上写着
     * "已开通"、用户那边什么都看不到。
     */
    await waitFor(() =>
      expect(modeApi.listEntitlementsByUser.mock.calls.length).toBeGreaterThan(grantCalls)
    )
    /* 每档几个人那个数（模式库上那个）也要重读 */
    await waitFor(() => expect(modeApi.listModes.mock.calls.length).toBeGreaterThan(1))
  })

  test('★★ 已经开通的那一档：给的是「取消」，点它发的是 revoke', async () => {
    const user = userEvent.setup()
    modeApi.listEntitlementsByUser.mockResolvedValue(byUser(['audiobook']))
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')

    await waitFor(() => expect(rowOf('audiobook')).toHaveAttribute('data-state', 'open'))
    await user.click(actionOf('audiobook'))

    await waitFor(() => expect(modeApi.revokeMode).toHaveBeenCalledWith(7, 'audiobook'))
    /* 取消之后也是**重读**（不是把那一行就地改掉） */
    await waitFor(() =>
      expect(modeApi.listEntitlementsByUser.mock.calls.length).toBeGreaterThan(1)
    )
  })

  test('★★ 读不到账号的授权时：说读不到，而不是画成"未开通"', async () => {
    const user = userEvent.setup()
    modeApi.listEntitlementsByUser.mockRejectedValue({
      response: { data: { message: '读不到这个账号的模式授权（数据库连接失败）。' } },
    })
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')

    /*
     * ⚠★ 把"没读到"画成"没有权限"会让运营**对着一个其实已经有权限的人再开一次**
     * （服务端会多插一行，看不出错）。所以读不到时必须说读不到，并把按钮禁用 ——
     * 行本身**照样画**（用户看得见有哪几档），只是那一格写着"状态读不到"。
     */
    await screen.findByTestId('mode-entitlements-error')
    expect(screen.getByText(/数据库连接失败/)).toBeInTheDocument()
    expect(screen.getByTestId('mode-state-mv')).toHaveTextContent('Status unreadable')
    /* ★ 每一档都在，而每一档的按钮都点不动（不是"那一块不见了"） */
    expect(rowOf('mv')).toHaveAttribute('data-state', 'unknown')
    for (const row of [rowOf('mv'), rowOf('audiobook')]) {
      expect(within(row).getByRole('button')).toBeDisabled()
    }
  })

  test('★ 服务端拒绝时把它那句话原样显示出来（不做二次解释）', async () => {
    const user = userEvent.setup()
    modeApi.grantMode.mockRejectedValue({
      response: {
        data: {
          message: 'mode: 模式库里没有 "ghost" 这一档，所以现在给它授权不会生效。',
        },
      },
    })
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await waitFor(() => expect(rowOf('audiobook')).toHaveAttribute('data-state', 'closed'))
    await user.click(actionOf('audiobook'))

    expect(await screen.findByTestId('mode-action-error')).toBeInTheDocument()
    expect(screen.getByText(/模式库里没有/)).toBeInTheDocument()
  })

  test('★★ 服务端回 200 + success:false 也算失败（不许画成"什么都没发生"）', async () => {
    const user = userEvent.setup()
    /*
     * ⚠★ `common.ApiErrorMsg` 是 **HTTP 200 + success:false**，而 axios 的拦截器
     * 只弹一个 toast、**不会**让 promise 失败 —— 于是"服务端拒绝了"会被读成
     * "调用成功了"。这一条钉住那一层转换（`ok()`）。
     */
    modeApi.grantMode.mockRejectedValue({
      response: { data: { success: false, message: '账号 7 不存在。' } },
    })
    renderPage()

    await user.type(await screen.findByLabelText('Account ID'), '7')
    await waitFor(() => expect(rowOf('audiobook')).toHaveAttribute('data-state', 'closed'))
    await user.click(actionOf('audiobook'))

    expect(await screen.findByTestId('mode-action-error')).toHaveTextContent('账号 7 不存在。')
  })

  test('★ 服务端没说目录时那一屏照样画得出来（不崩、不留白屏）', async () => {
    modeApi.listModes.mockResolvedValue({ items: [], directory: '', knownMediums: [], note: '' })
    renderPage()

    /* 一档都没有时给一句能照做的话，而不是一个空框 */
    expect(await screen.findByText(/holds no usable mode yet/)).toBeInTheDocument()
  })
})

/* ==========================================================================
 * ★★ 「编辑一档模式」—— 公开 / 私有那一下（用户 2026-… 点名要的）
 *
 * > "new-api 的后台，模式管理 应该是可以编辑的，可以设置 权限是公开还是私有"
 * ======================================================================== */

describe('★★ 编辑一档的分发策略（公开 / 私有）', () => {
  test('★★ 点「编辑」→ 弹窗里的初值就是**这一档现在**的可见性与说明', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('mode-edit-audiobook'))

    /* 弹窗里那两格 */
    expect(await screen.findByTestId('mode-meta-visibility')).toBeInTheDocument()
    /*
     * ⚠ 初值必须是**那一档自己的**（`audiobook` 是私有 + "音频那一档"）——
     * 拿第一档（`mv`）的值灌进去的话，运营一打开就看到错的可见性，
     * 而"保存"会把那个错的值写回磁盘。
     *
     * ⚠ 查 trigger 用 `getByLabelText`（`Label htmlFor` ↔ `SelectTrigger id` 那一对）：
     * 直接按**文字**查会命中别处（模式库那一行上也有同一句）。
     */
    expect(screen.getByLabelText('Who can open it')).toHaveTextContent(
      'Private mode — opened per account'
    )
    expect((screen.getByLabelText('One-line summary') as HTMLInputElement).value).toBe(
      '音频那一档'
    )
  })

  test('★★ 选「公开」+ 填说明 → 保存：发出的是那两格，而且**重读列表**', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('mode-edit-audiobook'))
    await screen.findByTestId('mode-meta-visibility')
    /* ⚠ 正文那一边也读回来了才动（表单与 JSON 页签都挂在那一份草稿上） */
    await screen.findByTestId('mode-tab-form')
    const loadCalls = modeApi.listModes.mock.calls.length

    /*
     * ⚠ Select 是 shadcn 那一套（点 trigger → 点 `role=option` 那一项），
     * 不是原生 `<select>` —— 所以这里按**角色 + 可访问名**去点，
     * 而不是 set value（与 `zsy-voice` 的过滤条那几条用例逐字同形）。
     */
    await user.click(screen.getByLabelText('Who can open it'))
    await user.click(
      await screen.findByRole('option', { name: 'Public mode — anyone can open it' })
    )

    const summary = screen.getByLabelText('One-line summary') as HTMLInputElement
    await user.clear(summary)
    await user.type(summary, '有声书那一档（公开版）')

    await user.click(screen.getByTestId('mode-meta-save'))

    await waitFor(() => expect(modeApi.updateModeMeta).toHaveBeenCalledTimes(1))
    expect(modeApi.updateModeMeta).toHaveBeenCalledWith('audiobook', {
      visibility: 'public',
      summary: '有声书那一档（公开版）',
    })
    /* ★★ 保存之后**重读列表**（服务端那份才是权威，本地推一份会与磁盘分叉） */
    await waitFor(() =>
      expect(modeApi.listModes.mock.calls.length).toBeGreaterThan(loadCalls)
    )
    /* ★ 成功之后弹窗关掉（不关的话运营会对着一个已经存过的表单再点一次） */
    await waitFor(() => expect(screen.queryByTestId('mode-meta-save')).toBeNull())
  })

  test('★ 只改可见性时，说明**带着原值**一起发出去（不是空串）', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('mode-edit-audiobook'))
    await screen.findByTestId('mode-meta-visibility')
    await screen.findByTestId('mode-tab-form')
    await user.click(screen.getByTestId('mode-meta-save'))

    await waitFor(() => expect(modeApi.updateModeMeta).toHaveBeenCalledTimes(1))
    /*
     * ⚠★ 这一条钉的是"**空串 ≠ 不传**"这件事的界面那一半：说明那一格原样带着
     * `音频那一档`。若界面发的是空串，服务端会**把那一格清掉** ——
     * 而运营只是点了一下保存、什么都没改说明。
     */
    expect(modeApi.updateModeMeta).toHaveBeenCalledWith('audiobook', {
      visibility: 'private',
      summary: '音频那一档',
    })
  })

  test('★ 服务端拒绝时：那句原话显示在弹窗里，而且**弹窗不关**', async () => {
    const user = userEvent.setup()
    modeApi.updateModeMeta.mockRejectedValue({
      response: {
        data: {
          message: 'mode: 可见性只能写 public / private（收到的是 "publik"）—— 它决定这一档是「任何账号都能一键开通」还是「后台给了权限才显示」。',
        },
      },
    })
    renderPage()

    await user.click(await screen.findByTestId('mode-edit-audiobook'))
    await screen.findByTestId('mode-meta-visibility')
    await screen.findByTestId('mode-tab-form')
    await user.click(screen.getByTestId('mode-meta-save'))

    /* ⚠ 原话透出来（不做二次解释），而且弹窗**留着**让运营改 —— 关掉就等于把他填的东西扔了 */
    expect(await screen.findByTestId('mode-meta-error')).toBeInTheDocument()
    expect(screen.getByText(/可见性只能写/)).toBeInTheDocument()
    expect(screen.getByTestId('mode-meta-save')).toBeInTheDocument()
  })
})

/* ==========================================================================
 * ★★ 「添加模式」：四格输入 + AI 一键生成（用户 2026-… 点名要的）
 *
 * > "右上角增加一个添加模式的功能，点击添加模式，可以输入模式名称，模式类型，模式简介，
 * >  然后能够 AI一键生成，选择一个API密钥，然后调用 glm-5.3-flash 这个模型生成"
 * ======================================================================== */

describe('★★ 添加模式（AI 一键生成）', () => {
  test('★★ 点右上角「添加模式」→ 那四格与密钥下拉都在', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('mode-create-open'))

    expect(await screen.findByTestId('mode-create-label')).toBeInTheDocument()
    expect(screen.getByTestId('mode-create-medium')).toBeInTheDocument()
    expect(screen.getByTestId('mode-create-summary')).toBeInTheDocument()
    expect(screen.getByTestId('mode-create-request')).toBeInTheDocument()
    expect(screen.getByTestId('mode-create-key')).toBeInTheDocument()

    /* ★ 密钥列表是**打开时**读的（那是这一刻的账号状态，不该跟着页面加载去读） */
    await waitFor(() => expect(modeApi.listModeAiKeys).toHaveBeenCalledTimes(1))
    /* ★ 而且把"这一趟用哪个模型"念出来（运营一定会问） */
    expect(await screen.findByText(/glm-5\.3-flash/)).toBeInTheDocument()
  })

  test('★ 没填名称时「AI 一键生成」是禁用的（点了会发一个 label 为空的请求）', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('mode-create-open'))
    const generate = await screen.findByTestId('mode-create-generate')
    expect(generate).toBeDisabled()

    await user.type(screen.getByTestId('mode-create-label'), '美食探店')
    await waitFor(() => expect(generate).toBeEnabled())
  })

  test('★★ 生成：发出的那几格对得上，成功之后弹窗关掉并**重读列表**', async () => {
    const user = userEvent.setup()
    renderPage()

    await user.click(await screen.findByTestId('mode-create-open'))
    await screen.findByTestId('mode-create-label')

    await user.type(screen.getByTestId('mode-create-label'), '美食探店')
    await user.type(screen.getByTestId('mode-create-summary'), '一分钟讲一家店')
    await user.type(screen.getByTestId('mode-create-request'), '我要做美食探店类的短视频')

    const loadCalls = modeApi.listModes.mock.calls.length
    await user.click(screen.getByTestId('mode-create-generate'))

    await waitFor(() => expect(modeApi.createMode).toHaveBeenCalledTimes(1))
    /*
     * ⚠★ 判据是**整个调用记录**：那几格少一个都会让服务端生成出一份不对的东西
     * （少了 `token_id` 这一趟就没有身份、落不到任何渠道上）。
     */
    expect(modeApi.createMode).toHaveBeenCalledWith({
      label: '美食探店',
      medium: 'video',
      summary: '一分钟讲一家店',
      request: '我要做美食探店类的短视频',
      visibility: 'private',
      token_id: 3,
    })

    /* ★★ 新那一档就在磁盘上了 —— 界面得跟着出现它 */
    await waitFor(() => expect(modeApi.listModes.mock.calls.length).toBeGreaterThan(loadCalls))
    /* ★ 成功之后弹窗关掉（不关的话运营会对着一个已经生成过的表单再点一次） */
    await waitFor(() => expect(screen.queryByTestId('mode-create-generate')).toBeNull())
  })

  test('★★ 生成失败时把服务端那句话**原样**摆出来（不做二次解释）', async () => {
    /*
     * ⚠ 那几句话是这一屏最要紧的反馈：它要说清是**哪一格**不对、该去哪儿改
     * （密钥被禁用 / 额度用完 / 没有渠道 / 模型不认这份需求）。
     */
    const user = userEvent.setup()
    modeApi.createMode.mockRejectedValue({
      response: {
        data: {
          message: 'mode: 那个密钥不是你的（只能用自己账号下的密钥）',
        },
      },
    })
    renderPage()

    await user.click(await screen.findByTestId('mode-create-open'))
    await user.type(await screen.findByTestId('mode-create-label'), '美食探店')
    await user.click(screen.getByTestId('mode-create-generate'))

    expect(await screen.findByTestId('mode-create-error')).toHaveTextContent(
      '那个密钥不是你的'
    )
    /* ⚠ 失败时**弹窗不关**（关掉就等于把运营填的那几格扔了） */
    expect(screen.getByTestId('mode-create-generate')).toBeInTheDocument()
  })

  test('★ 读不到密钥列表时说清楚（不画成一个空下拉让人发呆）', async () => {
    const user = userEvent.setup()
    modeApi.listModeAiKeys.mockRejectedValue({
      response: { data: { message: '读不到你的密钥列表（数据库连接失败）。' } },
    })
    renderPage()

    await user.click(await screen.findByTestId('mode-create-open'))

    expect(await screen.findByTestId('mode-keys-error')).toHaveTextContent('数据库连接失败')
  })
})
