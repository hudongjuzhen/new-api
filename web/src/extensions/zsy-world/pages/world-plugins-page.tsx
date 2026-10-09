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
import { useEffect, useState } from 'react'
import { Download, Loader2, RefreshCw, ShieldCheck, ShieldOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Skeleton } from '@/components/ui/skeleton'
import { SectionPageLayout } from '@/components/layout'

import {
  grantCapability,
  listEntitlements,
  listPluginTemplates,
  revokeCapability,
  type EntitlementList,
  type IssueResult,
  type PluginTemplateList,
} from '../api'
import { signAndDownloadPluginFile } from '../lib/plugin-download'
import {
  accountCapabilityState,
  accountCapabilityStateLabel,
  bucketTemplates,
  capabilityHintFallback,
  capabilityHints,
  emptyTemplatesHint,
  entitlementState,
  entitlementStateLabel,
  issueSuccessLine,
  missingCapabilities,
  parseUserId,
  templatesOfCapabilities,
  visibilityLabel,
} from '../lib/plugin-issue-view'

/**
 * 世界 IP · 插件管理（`zsy/world` 的后台那一屏）
 *
 * # 这一屏做三件事，而且**顺序就是运营的动作顺序**
 *
 * ⚠★ 这一屏最要紧的一句话是"**签发不等于开通**"：
 *
 * 	1. **签发**：按「某个账号 + 某个插件」产出一份插件 JSON，下载下来发给那个人。
 * 	   导入时客户端会核对账号 —— 把给 A 的文件发给了 B，B 当场就知道。
 * 	   而这份文件**不改变任何权限**。
 * 	2. **开通 / 取消**：真正决定"他能不能用"的那一步。没开通的话，文件装得上、
 * 	   菜单也出得来，**但打开那一屏会被服务端拒绝**（`E_ENTITLEMENT`）。
 * 	3. **看现在是什么状态**：每次现算，不是从历史行推的。
 *
 * 所以界面上把第 1 步与第 2 步分开，并在签发结果里明确写出"还差哪个能力"。
 * 把它们合成一个按钮（"生成并开通"）会省一次点击，代价是**再也看不出**某个人
 * 到底是被开通了还是只是拿到了一份文件。
 *
 * # ★★ 第三版：左边填 ID、右边就是这个人的能力表（用户 2026-… 报的那件事）
 *
 * 用户的原话：
 *
 * > "我希望左侧是输入ID的位置，右侧是选择的列表，不要每个模式或者每个插件单独占一行，
 * >  太浪费空间了。……另外页面要适配国际化，我看现在很多就是英文，
 * >  我希望适配上其他语言"
 *
 * 于是这一屏的形状是：
 *
 *	┌─ 左：账号 ID ─┬─ 右：这个账号的每一个能力 ──────────────┐
 *	│  [ 7        ]  │  world-ip     已开通 需要它的插件：…  [取消开通] │
 *	│  说明那一行     │  world-ip-ai  未开通 需要它的插件：…  [开通]     │
 *	│                │  …（**一行好几个**，不再一个能力一行）           │
 *	└────────────────┴────────────────────────────────────────────────┘
 *
 * ⚠★ 这一屏**没有**"编辑"那一副面孔（模式那一屏有）：插件模板要怎么发出去写在
 * 模板文件里，而这一面从第一天起就不改那几份文件 —— 所以左边那一格空着时，
 * 右边只是"还没读到状态"的列表，而不是"可编辑"的列表。
 *
 * # ⚠★ 这里的文案一律走 `t()`（绝不写死中文）
 *
 * 见 `lib/plugin-issue-view.ts` 文件头那段：那一组曾经把中文写死在代码里，
 * 于是中文界面上正常、换任何语言都还是中文，而旁边那些走 `t()` 的字跟着语言变
 * —— 一块屏幕上两种语言并存，用户报的就是这件事。
 */
export function WorldPluginsPage() {
  const { t } = useTranslation()

  const [templates, setTemplates] = useState<PluginTemplateList | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  /** 账号 ID 那一格（**文本**：判据在 `parseUserId`，不在这里）。 */
  const [userIdText, setUserIdText] = useState('')
  const [issuing, setIssuing] = useState(false)
  const [issueError, setIssueError] = useState('')
  const [issued, setIssued] = useState<IssueResult | null>(null)

  const [entitlements, setEntitlements] = useState<EntitlementList | null>(null)
  const [entitlementsError, setEntitlementsError] = useState('')
  /*
   * ★ 这一份数据对应**哪个账号**。
   *
   * 它是"手上这份还算不算数"的判据：账号一变，`entitlements` 里的东西就属于
   * **上一个**账号了 —— 而两个都有 `world-ip` 时看起来完全正常，那正是最危险的一种错。
   *
   * ⚠ 用"记下它属于谁"而不是"在 effect 里把它清空"，是为了避开
   * `react(set-state-in-effect)`（在 effect 里同步 setState 会引发级联渲染，
   * 而那个 lint 规则在本仓库是开的、现成的扩展页一个都不犯）。清空的活交给读取方：
   * 下面用一个 `isStale` 判据把"过期的数据"当成"还没有数据"。
   */
  const [entitlementsOf, setEntitlementsOf] = useState('')
  /**
   * 判定"已取消 / 已过期"要用的"现在"。
   *
   * ⚠★ 它**不能**在渲染里 `Date.now()`：那是**渲染期读时钟**，同一份数据两次渲染
   * 可能得出不同的界面（`react(purity)` 报的就是这件事，而它不只是洁癖 ——
   * 并发渲染下 React 可以丢弃或重放一次渲染）。
   *
   * 所以时钟只在**数据落地那一刻**读一次：`listEntitlements` 回来的时间就是"现在"。
   * 初值 0 是安全的：那时还没有任何一份数据，所以没有人会去看状态标签。
   */
  const [nowSeconds, setNowSeconds] = useState(0)
  /**
   * ★ 正在开通 / 取消的那一个能力（**只锁那一行**）。
   *
   * ⚠ 它是"这一下点过了，等它回来"的凭据：没有它的话，运营连点两下会给同一个人
   * **插两行**权限（服务端不会报错，界面上也看不出多了一行）。
   */
  const [busyCapability, setBusyCapability] = useState('')

  /**
   * 读一次模板列表。
   *
   * # ⚠★ 为什么它是**普通函数**而不是 `useCallback`，而且里面**不**先 `setLoading(true)`
   *
   * `react(set-state-in-effect)` 会红在"effect 里同步 setState"上 —— 而两个来源
   * 都要为此负责：
   *
   *  1. `setLoading(true)` 在 effect 第一行**同步**跑 → 挂载时多一轮渲染；
   *  2. `useCallback` 认不出"这个回调只在 event handler 里用"，于是把它当成 effect 的
   *     依赖一起分析（`react(preserve-manual-memoization)`）。
   *
   * 所以：`loading` 的初值就是 `true`（挂载时本来就在读），这里只负责**读完**把它置回
   * `false`；而函数本身不 memo —— 它不是一个会被当依赖传下去的回调。
   *
   * ⚠ `preserve` 那个参数是给"重新读取"那颗按钮用的：用户按它时**要**看到 loading
   * （否则他不知道按下去有没有反应），而那条路是 event handler，不受上面那条规则管。
   */
  async function loadTemplates(preserve = false) {
    if (preserve) setLoading(true)
    setLoadError('')
    try {
      const data = await listPluginTemplates()
      setTemplates(data)
    } catch (err) {
      setLoadError(readError(err, t('Could not read the plugin template list')))
    } finally {
      setLoading(false)
    }
  }

  /*
   * ── 由状态推出来的那几格（全部放在任何 effect 之前 —— 见下面 TDZ 那条说明）──
   */
  const parsedId = parseUserId(t, userIdText)
  /** 只把**那个数字**取出来：effect 的依赖用它，不用 `parsedId`（每渲染都是新对象） */
  const targetUserId = parsedId.value
  const buckets = bucketTemplates(templates?.items || [])
  /**
   * ★★ 这一屏列的是**能力**，不是模板：一个能力可能被好几份插件要，
   * 也可能一份插件要好几个能力。所以右边那一块的名册来自**全部能用的模板**，
   * 而每一行上那句"哪几份插件要它"由 `templatesOfCapabilities` 现算。
   */
  const allCapabilities = capabilityHints(buckets.usable)
  const templatesByCapability = templatesOfCapabilities(t, buckets.usable)

  /** 手上那份能力数据属于谁；空串 = 没有任何一份数据算数 */
  const wantEntitlementsOf = targetUserId ? String(targetUserId) : ''
  /** ★ 过期的数据当"没有数据"用 —— 见 `entitlementsOf` 那段说明 */
  const isStale = entitlementsOf !== wantEntitlementsOf
  const currentEntitlements = isStale ? null : entitlements
  const currentError = isStale ? '' : entitlementsError

  /*
   * ── 那一块要画哪一支（**先算好，别在 JSX 里串三元**：`no-nested-ternary` 在本仓库
   *    是开着的，而串起来的三元读起来像谜题）──
   */
  const hasUserIdText = userIdText.trim().length > 0
  const isBlockedById = !!parsedId.problem
  /**
   * ★★ **两个不同的判据，别合成一个**（这一块踩过两次，都是同一类错）。
   *
   * | 判据 | 回答的问题 | 什么时候为真 |
   * |---|---|---|
   * | `showsCapabilityRows` | **有没有东西可画** | 模板里报出了能力 |
   * | `hasFreshEntitlements` | **手上那份数据算不算数** | 读回来了、没报错 |
   *
   * ⚠★★ **挂载判据只许用 `showsCapabilityRows`**（不许把 `hasFreshEntitlements` /
   * `!currentError` 搭进去）。这一条是**实测**得到的（与「模式管理」那一屏同一条）：
   * 读不到时会先落一次"还没有数据"的渲染（那时 `currentError` 还是空串），
   * 紧接着那次读失败落地 —— 条件是 `… && !currentError` 的话，第二次渲染就把整块
   * **连根拆掉**（React 把 `div` 换成 `null`，那是删除，不是隐藏）。症状是
   * "这一块从来没出现过"，而日志里能看到卡片**明明被构造过**。
   * 判据与"这一格写什么"分开，就没有这一出。
   */
  const showsCapabilityRows = allCapabilities.length > 0
  const hasFreshEntitlements = !isStale && !currentError

  /**
   * 拉一次这个账号的能力（每次现算 —— 与判权同源）。
   *
   * ⚠ 同上：它是**普通函数**，不是 `useCallback` —— 两条路（effect 与按钮）都要调它，
   * 而把它包成 memo 会让 `react(preserve-manual-memoization)` 有意见，收益却为零。
   */
  async function refreshEntitlements(userId: number) {
    const key = userId ? String(userId) : ''
    if (!key) return
    try {
      const data = await listEntitlements(userId)
      setEntitlements(data)
      setEntitlementsError('')
      setEntitlementsOf(key)
      /* ★ "现在"在数据落地这一刻读一次（渲染期不许读时钟，见 `nowSeconds` 的说明） */
      setNowSeconds(Math.floor(Date.now() / 1000))
    } catch (err) {
      /*
       * ★ 读不到就**说读不到**，而不是把"没读到"画成"没有能力"。
       *
       * ⚠ 这一条是实测改的：第一版读失败时只是把那一块留空，于是界面照样画出
       * 两个「开通」按钮 —— 而那时真实状态**未知**。运营按下"开通"，可能是在给一个
       * 已经有能力的人再开一次（服务端会多插一行，看不出错），也可能对着一个不存在的
       * 账号操作。两种都比"显示 loading"坏得多。
       *
       * 它**不打断签发**（签发与开通是两件事），所以只影响右边那一块。
       */
      setEntitlements(null)
      setEntitlementsError(readError(err, t('Could not read this account’s capabilities')))
      setEntitlementsOf(key)
    }
  }

  useEffect(() => {
    void loadTemplates()
    /*
     * ⚠ 依赖是空数组：`loadTemplates` 是**普通函数**（不是 memo 的回调），把它写进依赖里
     * 会让这个 effect 每一轮渲染都跑一次 —— 而它做的是**取一次模板列表**。
     * 首次挂载取一次就够了（要重取有「重新读取」那颗按钮）。
     */
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  /**
   * 账号变了就去读一次。
   *
   * ⚠ 这里**不**清空旧数据（那会犯 `set-state-in-effect`）：清空交给 `isStale` ——
   * 账号一变，`wantEntitlementsOf` 就与 `entitlementsOf` 不同，于是上面那几行
   * 立刻把它当成"还没有数据"（显示"正在读"），而 effect 只负责把新的读回来。
   *
   * 依赖用 `wantEntitlementsOf` 这一个字符串：它由账号 id 拼成，
   * 所以**输入没变它就不变**，而 `parsedId` 那种对象引用每次渲染都会变。
   */
  useEffect(() => {
    void refreshEntitlements(targetUserId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wantEntitlementsOf])

  /**
   * 签发一次并下载。
   *
   * ⚠ 它不是 `useCallback`：它只在按钮的 onClick 里用，没有任何东西把它当依赖传递 ——
   * 而包一层 memo 会让 `react(preserve-manual-memoization)` 有意见（它检查的是
   * "memo 的依赖数组有没有真的保住记忆化"，而这里的依赖里有每次渲染都新建的
   * `parsedId`），收益为零。
   */
  async function onIssue(pluginId: string) {
    if (parsedId.problem) {
      setIssueError(parsedId.problem)
      return
    }
    if (!pluginId) {
      setIssueError(t('Pick a plugin first.'))
      return
    }
    setIssuing(true)
    setIssueError('')
    setIssued(null)
    try {
      /*
       * ★ 一个动作、一次签发：它**下载**文件并把结果交回来。
       *
       * ⚠ 不要写成"先下载、再签一次拿结果" —— 两次签发的 `issuedAt` 不同，
       * 于是界面上显示的 `check` 与用户手上那份文件对不上，而运营照着核对时
       * 会以为文件被改过（见 `plugin-download.ts` 的说明）。
       */
      const result = await signAndDownloadPluginFile(parsedId.value, pluginId)
      setIssued(result)
      await refreshEntitlements(parsedId.value)
    } catch (err) {
      setIssueError(readError(err, t('Could not sign')))
    } finally {
      setIssuing(false)
    }
  }

  /** 开通 / 取消一个能力（同上：普通函数，不做 memo）。 */
  async function toggleCapability(capability: string, active: boolean) {
    if (parsedId.problem) {
      setIssueError(parsedId.problem)
      return
    }
    setBusyCapability(capability)
    setIssueError('')
    try {
      if (active) {
        await revokeCapability(targetUserId, capability)
      } else {
        /*
         * ⚠ 第三个参数（到期时间）**必须显式传 0**。
         *
         * 服务端的 `expires_at` 是可选字段，缺省即"永不过期" —— 而 0 与"不传"
         * 在那边是同一个意思，所以少传**不会报错**。
         * 但它会让"开通"这个方法在调用记录里少一格：一条测试当场红在
         * "参数不对"，而界面看起来完全正常（见本文件那条用例的说明）。
         * 显式写 0 是为了让**参数个数**也是契约的一部分。
         */
        await grantCapability(targetUserId, capability, 0)
      }
      await refreshEntitlements(targetUserId)
    } catch (err) {
      setIssueError(readError(err, active ? t('Could not revoke') : t('Could not open')))
    } finally {
      setBusyCapability('')
    }
  }

  return (
    /*
     * ★★ **必须包一层 `SectionPageLayout`**（用户 2026-… 报的那件事）：
     * `SidebarInset` 是 `h-[calc(100svh-…)] overflow-hidden`，**它自己不滚** ——
     * 滚动条由 `SectionPageLayout` 里那个 `overflow-auto` 的容器提供。
     * 少了这一层，内容一多**下面就看不见了，而且没有任何滚动条**
     * （不报错，只是"页面像被截断了"）。
     *
     * ⚠ 这里**不加 `fixedContent`**：那一档是给"页面内自己有滚动表格"的屏用的
     * （`overflow-hidden` + 内部滚动）—— 这一屏是普通的文档式滚动。
     */
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('World IP · Plugin management')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' size='sm' onClick={() => void loadTemplates(true)}>
          <RefreshCw className='mr-1 h-4 w-4' />
          {t('Reload')}
        </Button>
      </SectionPageLayout.Actions>

      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Sign a plugin file for one account, then send it to that person. The file records which account it is for — importing it under a different account is refused, so a file sent to the wrong person is caught immediately.'
            )}
          </p>

          {/*
        ★ 这一条横幅是整屏最要紧的一句话，所以它不是提示气泡而是常驻的 Alert：
        「签发」与「开通」是两件事，而把两者混起来会让"这个人到底有没有被授权"
        变成一个看不出来的问题。
      */}
      <Alert>
        <AlertTitle>{t('Signing is not granting')}</AlertTitle>
        <AlertDescription>
          {t(
            'Signing only produces a file. Whether the account can actually use the capability is decided by the opens below, and checked on every request by the server. A signed file with no open installs fine and then gets refused when opened.'
          )}
        </AlertDescription>
      </Alert>

      {/*
        ────────────────────────── 一、按账号签发 / 开通 ──────────────────────────

        ★★ 这一张卡是**这一版的形状**（用户 2026-…）：左边那一格是"这个人是谁"，
        右边就是"他手上每一个能力是怎么回事" —— 签发的入口也在右边（模板那一栏）。
      */}
      <Card>
        <CardHeader>
          <CardTitle>{t('Open plugins for an account')}</CardTitle>
          <CardDescription>
            {t(
              'Enter the account ID first: then this account’s every capability is listed at once, and the file to send him can be signed from the plugin template below. Signing is not opening — a signed file with nothing opened installs fine and then gets refused.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-4'>
          {/*
            ★★ 左 ID / 右列表（用户 2026-…："我希望左侧是输入ID的位置，右侧是选择的列表，
            不要每个插件单独占一行，太浪费空间了"）。

            ⚠ 窄屏（`lg` 以下）自动叠成一列 —— 那时把 ID 挤在左边会让右边只剩一条缝，
            而"一行放好几个"正是这一版要的。
          */}
          <div className='flex flex-col gap-4 lg:flex-row lg:items-start'>
            <div className='flex shrink-0 flex-col gap-2 lg:w-64'>
              <Label htmlFor='world-plugin-user-id'>{t('Account ID')}</Label>
              <Input
                id='world-plugin-user-id'
                value={userIdText}
                onChange={(e) => setUserIdText(e.target.value)}
                placeholder={t('e.g. 7')}
                className='w-full'
              />
              {/* 填错时**当场**说，而不是点下去之后由服务端说一句"需要 user_id" */}
              {userIdText && parsedId.problem ? (
                <p className='text-destructive text-xs'>{parsedId.problem}</p>
              ) : (
                <p className='text-muted-foreground text-xs' data-testid='world-open-hint'>
                  {t(
                    'The account that will import this file. Signing needs it too — the file carries the account.'
                  )}
                </p>
              )}
            </div>

            <div className='flex min-w-0 flex-1 flex-col gap-3'>
              {issueError ? (
                <Alert variant='destructive' data-testid='world-action-error'>
                  <AlertDescription>{issueError}</AlertDescription>
                </Alert>
              ) : null}

              {issued ? (
                <Alert variant={missingCapabilities(issued).length ? 'destructive' : 'default'}>
                  <AlertTitle>
                    {missingCapabilities(issued).length
                      ? t('Signed — but this account cannot use it yet')
                      : t('Signed and ready to send')}
                  </AlertTitle>
                  <AlertDescription>
                    <span className='block'>{issueSuccessLine(t, issued)}</span>
                    <span className='text-muted-foreground mt-1 block font-mono text-xs'>
                      {issued.fileName} · {issued.check} · {issued.site}
                    </span>
                  </AlertDescription>
                </Alert>
              ) : null}

              {/*
                ⚠ 账号 ID 还不合法时**照样把列表画出来**、按钮禁用，并在上面加一句说明。
                不画的话用户看到的是"这一块消失了"，而原因（ID 那一格填错了）在另一处。
              */}
              {hasUserIdText && isBlockedById ? (
                <Alert data-testid='world-entitlements-blocked'>
                  <AlertDescription>
                    {parsedId.problem} {t('Fix the account ID above to open or cancel.')}
                  </AlertDescription>
                </Alert>
              ) : null}

              {/*
                ★★ 还没读到（第一次读还在路上）时那一块是**空的**，所以要有一句话 ——
                否则运营填完 ID 看到的是"什么都没有"，而真相是它正在读。
                ⚠ 它与"读不到"那条 Alert（`currentError`）**不是同一句话**：那条说"读失败"，
                这一条说"正在读"。
              */}
              {showsCapabilityRows && !hasFreshEntitlements && !currentError ? (
                <p
                  className='text-muted-foreground text-sm'
                  data-testid='world-entitlements-loading'
                >
                  {t('Reading what this account holds — opening is enabled in a moment.')}
                </p>
              ) : null}

              {/* 读不到就说读不到 —— 绝不把"没读到"画成"没有能力"（见 refreshEntitlements） */}
              {showsCapabilityRows && currentError ? (
                <Alert variant='destructive' data-testid='world-entitlements-error'>
                  <AlertDescription>
                    {currentError}{' '}
                    {t('Capabilities cannot be opened or cancelled until this is read.')}
                  </AlertDescription>
                </Alert>
              ) : null}

              {/*
                ★★ **这一块的挂载判据只有 `showsCapabilityRows`**：还没读到 / 读失败都
                **不换掉它**。见上面那段说明（换掉就是"连根拆掉"，而下一帧再挂回来时
                运营看到的是"这一块刚才闪了一下 / 从来没有过"）。
              */}
              {showsCapabilityRows ? (
                /*
                 * ★★ **一行放好几个**（用户 2026-…："不要每个模式或者每个插件单独占一行，
                 * 太浪费空间了"）。每一格只有一行字那么高，所以一屏能看十几个能力。
                 */
                <div className='grid grid-cols-1 gap-1.5 xl:grid-cols-2'>
                  {allCapabilities.map((capability) => {
                    const state = accountCapabilityState({
                      readable: hasFreshEntitlements,
                      active: currentEntitlements?.active || [],
                      capability,
                    })
                    const active = state === 'open'
                    const busy = busyCapability === capability
                    /*
                     * ⚠ 只有两种情况下按钮禁用：账号 ID 不合法（那一下会发一个 userId=0 的
                     * 请求），或**读不到他的能力**（此刻未知）。忙的时候只锁**这一行**。
                     */
                    const disabled = busy || isBlockedById || !hasFreshEntitlements
                    /*
                     * ⚠★ 「开通 / 取消开通」那两枚按钮上的字只有两三个字（一行放好几个的
                     * 前提），所以它们的**可访问名**要带上这一份能力 —— 否则读屏用户听到的
                     * 是一长串一模一样的"开通"。`aria-label` 不占视觉空间，正是这里要的东西。
                     */
                    const actionLabel = active
                      ? t('Cancel access to {{capability}}', { capability })
                      : t('Open {{capability}} for this account', { capability })
                    return (
                      <div
                        key={capability}
                        data-testid={`world-capability-${capability}`}
                        data-state={active ? 'open' : 'closed'}
                        className='flex items-center justify-between gap-2 rounded-md border px-2.5 py-1.5'
                      >
                        <span className='flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1'>
                          <span className='font-mono text-sm'>{capability}</span>
                          <Badge
                            variant={active ? 'default' : 'secondary'}
                            data-testid={`world-capability-state-${capability}`}
                            className='gap-1'
                          >
                            {active ? (
                              <ShieldCheck className='size-3' />
                            ) : (
                              <ShieldOff className='size-3' />
                            )}
                            {accountCapabilityStateLabel(t, state)}
                          </Badge>
                          {/*
                            ★ 能力名与插件名不是一回事：这一句说清"这份能力是哪几份插件要的"，
                            运营才知道开通它之后那个人能打开什么、以及该签哪一份文件给他。
                          */}
                          <span className='text-muted-foreground text-xs'>
                            {t('Required by {{plugins}}', {
                              plugins:
                                templatesByCapability[capability] || capabilityHintFallback(t),
                            })}
                          </span>
                        </span>
                        <Button
                          variant={active ? 'outline' : 'default'}
                          size='xs'
                          disabled={disabled}
                          aria-label={actionLabel}
                          onClick={() => void toggleCapability(capability, active)}
                        >
                          {busy ? <Loader2 className='mr-1 size-3 animate-spin' /> : null}
                          {active ? t('Cancel access') : t('Open')}
                        </Button>
                      </div>
                    )
                  })}
                </div>
              ) : null}

              {/*
                ★ 这一屏的"开通"与"签发"是两件事（签发在下面那一栏、按插件走）——
                所以在右栏里补一句：那个人能打开什么，取决于这里开通了哪几个能力。
              */}
              {hasUserIdText && allCapabilities.length ? (
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Opening a capability here is what decides whether the account can use it; the file to send him is signed on a plugin template below.'
                  )}
                </p>
              ) : null}

              {hasUserIdText && !allCapabilities.length ? (
                <p className='text-muted-foreground text-sm'>
                  {t('No capability can be opened yet — the plugin templates below declare none.')}
                </p>
              ) : null}

              {/* 历史行（取消过的 / 过期的）—— 运营常问"他什么时候没的" */}
              {hasUserIdText && currentEntitlements?.items?.length ? (
                <details data-testid='world-history'>
                  <summary className='text-muted-foreground cursor-pointer text-xs'>
                    {t('History (including cancelled and expired rows)')}
                  </summary>
                  <div className='mt-1 flex flex-col gap-1'>
                    {currentEntitlements.items.map((row) => {
                      const state = entitlementState(row, nowSeconds)
                      return (
                        <p key={row.id} className='text-muted-foreground text-xs'>
                          <span className='font-mono'>{row.capability}</span>
                          {' · '}
                          {entitlementStateLabel(t, state)}
                          {' · '}
                          {row.source}
                        </p>
                      )
                    })}
                  </div>
                </details>
              ) : null}
            </div>
          </div>
        </CardContent>
      </Card>

      {/* ────────────────────────── 二、插件模板（有哪几份） ────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle>{t('Plugin templates')}</CardTitle>
          <CardDescription>
            {templates?.directory
              ? t('Read from {{dir}} — the file name is the plugin id.', {
                  dir: templates.directory,
                })
              : t('Loading…')}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3'>
          {loading ? <Skeleton className='h-20 w-full' /> : null}

          {!loading && loadError ? (
            <Alert variant='destructive' data-testid='world-templates-error'>
              <AlertDescription>{loadError}</AlertDescription>
            </Alert>
          ) : null}

          {!loading && !loadError ? (
            <>
              {/*
                ★ 坏模板**排在最前面**并带上原因：它是运营唯一需要动手去修的东西，
                而"我放进去的文件不见了"看起来像后台坏了。
              */}
              {buckets.broken.map((row) => (
                <Alert key={row.id} variant='destructive'>
                  <AlertTitle>{row.name || row.id}</AlertTitle>
                  <AlertDescription>
                    <span className='block'>{row.problem}</span>
                    <span className='text-muted-foreground mt-1 block text-xs'>
                      {row.source}
                    </span>
                  </AlertDescription>
                </Alert>
              ))}

              {buckets.usable.length ? (
                /*
                 * ★★ 一行放**好几份**（用户 2026-… 那条"不要占一行"的同一件事）——
                 * 与「模式管理」那一屏同一个形状。
                 *
                 * ⚠ 每一份上有一颗「签发」：插件文件是**按插件**签发的，
                 * 而上面那一块是按**能力**开通的 —— 两件事，两处动作，别合成一个。
                 */
                <div className='grid grid-cols-1 gap-1.5 xl:grid-cols-2'>
                  {buckets.usable.map((row) => (
                    <div
                      key={row.id}
                      data-testid={`world-template-${row.id}`}
                      className='flex items-center justify-between gap-2 rounded-md border px-2.5 py-1.5'
                    >
                      <div className='flex min-w-0 flex-1 flex-col gap-1'>
                        <span className='flex flex-wrap items-center gap-2'>
                          <span className='font-medium'>{row.name}</span>
                          <span className='text-muted-foreground font-mono text-xs'>{row.id}</span>
                        </span>
                        <span className='flex flex-wrap items-center gap-1'>
                          {/*
                            ★★ 可见性排在**最前面**：它是这一份模板"发给谁"的那一格，
                            而它只差一个词（public / private），后果却完全不同 ——
                            运营扫一眼就该看出这份是不是公开可装的（见 `visibilityLabel`）。
                          */}
                          <Badge
                            variant={row.visibility === 'public' ? 'default' : 'secondary'}
                            data-testid={`world-template-visibility-${row.id}`}
                          >
                            {visibilityLabel(t, row.visibility)}
                          </Badge>
                          {row.screens.map((s) => (
                            <Badge key={s} variant='secondary'>
                              {s}
                            </Badge>
                          ))}
                          {row.capabilities.map((c) => (
                            <Badge key={c} variant='outline'>
                              {c}
                            </Badge>
                          ))}
                        </span>
                      </div>
                      {/*
                        ★ 签发是**按插件**的：这一颗签的就是**这一张卡**上那份插件，
                        而账号在左边那一格里填（一个账号，一次填）。
                      */}
                      <Button
                        variant='outline'
                        size='xs'
                        data-testid={`world-issue-${row.id}`}
                        disabled={issuing || isBlockedById}
                        onClick={() => void onIssue(row.id)}
                      >
                        {issuing ? (
                          <Loader2 className='mr-1 size-3 animate-spin' />
                        ) : (
                          <Download className='mr-1 size-3' />
                        )}
                        {t('Sign')}
                      </Button>
                    </div>
                  ))}
                </div>
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {emptyTemplatesHint(
                    t,
                    templates?.directory || '',
                    templates?.knownSources || []
                  )}
                </p>
              )}
            </>
          ) : null}
        </CardContent>
      </Card>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

/**
 * 读一句能给运营看的错误。
 *
 * ⚠ 这一组接口**出错时也返回 HTTP 200**（`common.ApiErrorMsg` 就是这么写的），
 * 所以 `err.message` 里那句才是服务端真正说的话 —— 而 axios 的默认文案
 * （"Request failed with status code 200"）对运营毫无用处。所以优先取服务端那句。
 */
function readError(err: unknown, fallback: string): string {
  const anyErr = err as { response?: { data?: { message?: string } }; message?: string }
  const fromServer = anyErr?.response?.data?.message
  if (typeof fromServer === 'string' && fromServer.trim()) return fromServer.trim()
  const message = anyErr?.message
  if (typeof message === 'string' && message.trim()) return message.trim()
  return fallback
}
