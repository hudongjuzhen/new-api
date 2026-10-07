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
  ENTITLEMENT_STATE_LABEL,
  bucketTemplates,
  emptyTemplatesHint,
  entitlementState,
  issueSuccessLine,
  missingCapabilities,
  parseUserId,
} from '../lib/plugin-issue-view'

/**
 * 世界 IP · 插件管理（`zsy/world` 的后台那一屏）
 *
 * # 这一屏做三件事，而且**顺序就是运营的动作顺序**
 *
 * ⚠★ 这一屏最要紧的一句话是"**签发不等于授权**"：
 *
 * 	1. **签发**：按「某个账号 + 某个插件」产出一份插件 JSON，下载下来发给那个人。
 * 	   导入时客户端会核对账号 —— 把给 A 的文件发给了 B，B 当场就知道。
 * 	   而这份文件**不改变任何授权**。
 * 	2. **授予 / 撤销**：真正决定"他能不能用"的那一步。没授予的话，文件装得上、
 * 	   菜单也出得来，**但打开那一屏会被服务端拒绝**（`E_ENTITLEMENT`）。
 * 	3. **看现在是什么状态**：每次现算，不是从历史行推的。
 *
 * 所以界面上把第 1 步与第 2 步分成两块，并在签发结果里明确写出"还差哪个能力"。
 * 把它们合成一个按钮（"生成并开通"）会省一次点击，代价是**再也看不出**某个人
 * 到底是被授权了还是只是拿到了一份文件。
 */
export function WorldPluginsPage() {
  const { t } = useTranslation()

  const [templates, setTemplates] = useState<PluginTemplateList | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  /** 选中的模板 id（默认第一个能用的）。 */
  const [selected, setSelected] = useState('')
  /** 账号 ID 那一格（**文本**：判据在 `parseUserId`，不在这里）。 */
  const [userIdText, setUserIdText] = useState('')
  const [issuing, setIssuing] = useState(false)
  const [issueError, setIssueError] = useState('')
  const [issued, setIssued] = useState<IssueResult | null>(null)

  const [entitlements, setEntitlements] = useState<EntitlementList | null>(null)
  const [entitlementsError, setEntitlementsError] = useState('')
  /*
   * ★ 这一份数据对应**哪个账号 + 哪个模板**。
   *
   * 它是"手上这份还算不算数"的判据：账号或模板一变，`entitlements` 里的东西就属于
   * **上一个**账号了 —— 而两个都有 `world-ip` 时看起来完全正常，那正是最危险的一种错。
   *
   * ⚠ 用"记下它属于谁"而不是"在 effect 里把它清空"，是为了避开
   * `react(set-state-in-effect)`（在 effect 里同步 setState 会引发级联渲染，
   * 而那个 lint 规则在本仓库是开的、现成的扩展页一个都不犯）。清空的活交给读取方：
   * 下面用一个 `isStale` 判据把"过期的数据"当成"还没有数据"。
   */
  const [entitlementsOf, setEntitlementsOf] = useState('')
  /**
   * 判定"已撤销 / 已过期"要用的"现在"。
   *
   * ⚠★ 它**不能**在渲染里 `Date.now()`：那是**渲染期读时钟**，同一份数据两次渲染
   * 可能得出不同的界面（`react(purity)` 报的就是这件事，而它不只是洁癖 ——
   * 并发渲染下 React 可以丢弃或重放一次渲染）。
   *
   * 所以时钟只在**数据落地那一刻**读一次：`listEntitlements` 回来的时间就是"现在"。
   * 初值 0 是安全的：那时还没有任何一份数据，所以没有人会去看状态标签。
   */
  const [nowSeconds, setNowSeconds] = useState(0)
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
      /*
       * 默认选中第一个**能用的**（不是第一个）：默认选一份坏的会让运营
       * 一进来就看见一句红色的话，而他什么都没做错。
       */
      const first = bucketTemplates(data.items || []).usable[0]
      setSelected((prev) => prev || first?.id || '')
    } catch (err) {
      setLoadError(readError(err, '读不到插件模板列表'))
    } finally {
      setLoading(false)
    }
  }

  /*
   * ── 由状态推出来的那几格（全部放在任何 effect 之前 —— 见下面 TDZ 那条说明）──
   */
  const parsedId = parseUserId(userIdText)
  /** 只把**那个数字**取出来：effect 的依赖用它，不用 `parsedId`（每渲染都是新对象） */
  const targetUserId = parsedId.value
  const buckets = bucketTemplates(templates?.items || [])
  const selectedTemplate = buckets.usable.find((row) => row.id === selected)
  const selectedTemplateId = selectedTemplate?.id || ''

  /** 手上那份能力数据属于谁；空串 = 没有任何一份数据算数 */
  const wantEntitlementsOf = targetUserId && selectedTemplateId
    ? `${targetUserId}:${selectedTemplateId}`
    : ''
  /** ★ 过期的数据当"没有数据"用 —— 见 `entitlementsOf` 那段说明 */
  const isStale = entitlementsOf !== wantEntitlementsOf
  const currentEntitlements = isStale ? null : entitlements
  const currentError = isStale ? '' : entitlementsError
  const activeCaps = new Set(currentEntitlements?.active || [])

  /*
   * ── 那一块要画哪一支（**先算好，别在 JSX 里串三元**：`no-nested-ternary` 在本仓库
   *    是开的，而串起来的三元读起来像谜题）──
   */
  const hasUserIdText = userIdText.trim().length > 0
  const isBlockedById = !!parsedId.problem
  /**
   * ★★ **两个不同的判据，别合成一个**（这一块踩过两次，都是同一类错）。
   *
   * | 判据 | 回答的问题 | 什么时候为真 |
   * |---|---|---|
   * | `showsCapabilityRows` | **有没有东西可画** | 填了账号、选好了模板 |
   * | `hasFreshEntitlements` | **手上那份数据算不算数** | 读回来了、没报错 |
   *
   * 能力行画的是**模板声明的那几个能力**（静态的事），不是账号的状态 ——
   * 所以"账号 id 还没填对"或"读不到账号状态"时**照样要把行画出来**（按钮禁用），
   * 否则运营看到的是"这一块消失了"，而原因在另一处。
   *
   * ⚠ 第一版只有一个判据（而且名字与含义还是反的），于是：
   *   · 账号 id 填错 → 行消失（本该画出来、按钮禁用）；
   *   · 数据到了 → 一直画骨架（本该画行）。
   * 两种都**不报错**，只是界面上那一块空着 —— 而测试红在"找不到按钮"上，
   * 很难看出是判据写错。
   */
  const showsCapabilityRows = hasUserIdText && !!selectedTemplate
  const hasFreshEntitlements = !isStale && !currentError

  /**
   * 拉一次这个账号的能力（每次现算 —— 与判权同源）。
   *
   * ⚠ 同上：它是**普通函数**，不是 `useCallback` —— 两条路（effect 与按钮）都要调它，
   * 而把它包成 memo 会让 `react(preserve-manual-memoization)` 有意见，收益却为零。
   */
  async function refreshEntitlements(userId: number, pluginId: string) {
    const key = userId && pluginId ? `${userId}:${pluginId}` : ''
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
       * 两个「授予」按钮 —— 而那时真实状态**未知**。运营按下"授予"，可能是在给一个
       * 已经有能力的人再授一次（服务端会多插一行，看不出错），也可能对着一个不存在的
       * 账号操作。两种都比"显示 loading"坏得多。
       *
       * 它**不打断签发**（签发与授权是两件事），所以只影响下面那一块。
       */
      setEntitlements(null)
      setEntitlementsError(readError(err, '读不到这个账号的能力'))
      setEntitlementsOf(key)
    }
  }

  useEffect(() => {
    void loadTemplates()
    /*
     * ⚠ 依赖是空数组：`loadTemplates` 是**普通函数**（不是 memo 的回调），把它写进
     * 依赖里会让这个 effect 每一轮渲染都跑一次 —— 而它做的是**取一次模板列表**。
     * 首次挂载取一次就够了（要重取有「重新读取」那颗按钮）。
     */
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  /**
   * 账号或模板变了就去读一次。
   *
   * ⚠ 这里**不**清空旧数据（那会犯 `set-state-in-effect`）：清空交给 `isStale` ——
   * 账号一变，`wantEntitlementsOf` 就与 `entitlementsOf` 不同，于是上面那几行
   * 立刻把它当成"还没有数据"（画骨架），而 effect 只负责把新的读回来。
   *
   * 依赖用 `wantEntitlementsOf` 这一个字符串：它由"账号 id + 模板 id"拼成，
   * 所以**输入没变它就不变**，而两个对象引用每次渲染都会变。
   */
  useEffect(() => {
    void refreshEntitlements(targetUserId, selectedTemplateId)
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
  async function onIssue() {
    if (parsedId.problem) {
      setIssueError(parsedId.problem)
      return
    }
    if (!selected) {
      setIssueError('请先选一份插件。')
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
      const result = await signAndDownloadPluginFile(parsedId.value, selected)
      setIssued(result)
      await refreshEntitlements(parsedId.value, selected)
    } catch (err) {
      setIssueError(readError(err, '签发失败'))
    } finally {
      setIssuing(false)
    }
  }

  /** 授予 / 撤销一个能力（同上：普通函数，不做 memo）。 */
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
         * 但它会让"授予"这个方法在调用记录里少一格：一条测试当场红在
         * "参数不对"，而界面看起来完全正常（见本文件那条用例的说明）。
         * 显式写 0 是为了让**参数个数**也是契约的一部分。
         */
        await grantCapability(targetUserId, capability, 0)
      }
      await refreshEntitlements(targetUserId, selectedTemplateId)
    } catch (err) {
      setIssueError(readError(err, active ? '撤销失败' : '授予失败'))
    } finally {
      setBusyCapability('')
    }
  }

  return (
    <div className='flex flex-col gap-4 p-4 md:p-6'>
      <div className='flex items-start justify-between gap-4'>
        <div>
          <h1 className='text-xl font-semibold'>{t('World IP · Plugins')}</h1>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t(
              'Sign a plugin file for one account, then send it to that person. The file records which account it is for — importing it under a different account is refused, so a file sent to the wrong person is caught immediately.'
            )}
          </p>
        </div>
        <Button variant='outline' size='sm' onClick={() => void loadTemplates(true)}>
          <RefreshCw className='mr-1 h-4 w-4' />
          {t('Reload')}
        </Button>
      </div>

      {/*
        ★ 这一条横幅是整屏最要紧的一句话，所以它不是提示气泡而是常驻的 Alert：
        「签发」与「授予」是两件事，而把两者混起来会让"这个人到底有没有被授权"
        变成一个看不出来的问题。
      */}
      <Alert>
        <AlertTitle>{t('Signing is not granting')}</AlertTitle>
        <AlertDescription>
          {t(
            'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.'
          )}
        </AlertDescription>
      </Alert>

      {/* ────────────────────────────── 一、模板 ────────────────────────────── */}
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
          {loading ? <Skeleton className='h-16 w-full' /> : null}

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

              {buckets.usable.length || buckets.broken.length ? (
                buckets.usable.map((row) => {
                  const on = row.id === selected
                  return (
                    <button
                      key={row.id}
                      type='button'
                      onClick={() => setSelected(row.id)}
                      className={`rounded-md border p-3 text-left transition-colors ${
                        on ? 'border-primary bg-accent/40' : 'hover:bg-accent/20'
                      }`}
                    >
                      <span className='flex items-center gap-2'>
                        <span className='font-medium'>{row.name}</span>
                        <span className='text-muted-foreground font-mono text-xs'>
                          {row.id}
                        </span>
                      </span>
                      <span className='mt-1 flex flex-wrap items-center gap-1'>
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
                    </button>
                  )
                })
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {emptyTemplatesHint(
                    templates?.directory || '',
                    templates?.knownSources || []
                  )}
                </p>
              )}
            </>
          ) : null}
        </CardContent>
      </Card>

      {/* ────────────────────────── 二、按账号签发 ────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle>{t('Sign a file for an account')}</CardTitle>
          <CardDescription>
            {t(
              'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-4'>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='world-plugin-user-id'>{t('Account ID')}</Label>
            <div className='flex flex-wrap items-center gap-2'>
              <Input
                id='world-plugin-user-id'
                value={userIdText}
                onChange={(e) => setUserIdText(e.target.value)}
                placeholder={t('e.g. 7')}
                className='w-40'
              />
              <Button
                onClick={() => void onIssue()}
                disabled={issuing || !selected || !!parsedId.problem}
              >
                {issuing ? (
                  <Loader2 className='mr-1 h-4 w-4 animate-spin' />
                ) : (
                  <Download className='mr-1 h-4 w-4' />
                )}
                {t('Sign and download')}
              </Button>
            </div>
            {/* 填错时**当场**说，而不是点下去之后由服务端说一句"需要 user_id" */}
            {userIdText && parsedId.problem ? (
              <p className='text-destructive text-xs'>{parsedId.problem}</p>
            ) : (
              <p className='text-muted-foreground text-xs'>
                {t('The account that will import this file.')}
              </p>
            )}
          </div>

          {issueError ? (
            <Alert variant='destructive'>
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
                <span className='block'>
                  {issueSuccessLine(issued, selectedTemplate)}
                </span>
                <span className='text-muted-foreground mt-1 block font-mono text-xs'>
                  {issued.fileName} · {issued.check} · {issued.site}
                </span>
              </AlertDescription>
            </Alert>
          ) : null}
        </CardContent>
      </Card>

      {/* ────────────────────── 三、这个账号的能力（真的那一步） ────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle>{t('Capabilities of this account')}</CardTitle>
          <CardDescription>
            {t(
              'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3'>
          {/*
            ★ 四支的次序是**实测**定下来的（第一版只有两支，于是"填了 abc"落到第一支，
            界面上连能力行都不见了 —— 而用户明明填了东西，看起来像这一块坏了）。

            ⚠ 写成"先算几个布尔量、再让 JSX 里最多一层三元"（`no-nested-ternary` 在本
            仓库是开的）：串起来的三元读起来像谜题，而这一块正是最需要一眼看懂的地方。
          */}
          {!hasUserIdText ? (
            <p className='text-muted-foreground text-sm'>
              {t('Fill in an account ID above to see and change its capabilities.')}
            </p>
          ) : null}

          {hasUserIdText && isBlockedById ? (
            /*
             * ⚠ 账号 ID 还不合法时**照样把行画出来**、按钮禁用，并在上面加一句说明。
             * 不画的话用户看到的是"这一块消失了"，而原因（ID 那一格填错了）在另一处。
             */
            <Alert data-testid='world-entitlements-blocked'>
              <AlertDescription>
                {parsedId.problem} {t('Fix the account ID above to grant or revoke.')}
              </AlertDescription>
            </Alert>
          ) : null}

          {/*
            ★★ 还没读到就画**骨架**，不画按钮。
            *
            * ⚠ 这一条也是实测改的：第一版在这里直接按"没生效"渲染，于是服务端
            * 还没答话的那一瞬间，界面上已经摆着两个「授予」按钮 —— 而真实状态未知。
            * 抢在那一下点下去，可能给一个已有能力的人再授一次。
          */}
          {showsCapabilityRows && !hasFreshEntitlements && !currentError ? (
            <Skeleton className='h-16 w-full' data-testid='world-entitlements-loading' />
          ) : null}

          {/* 读不到就说读不到 —— 绝不把"没读到"画成"没有能力"（见 refreshEntitlements） */}
          {showsCapabilityRows && currentError ? (
            <Alert variant='destructive' data-testid='world-entitlements-error'>
              <AlertDescription>{currentError}</AlertDescription>
            </Alert>
          ) : null}

          {showsCapabilityRows
            ? (selectedTemplate?.capabilities || []).map((capability) => {
                const active = activeCaps.has(capability)
                const busy = busyCapability === capability
                return (
                  <div
                    key={capability}
                    data-testid={`world-capability-${capability}`}
                    className='flex flex-wrap items-center justify-between gap-2 rounded-md border p-3'
                  >
                    <span className='flex items-center gap-2'>
                      {active ? (
                        <ShieldCheck className='h-4 w-4' />
                      ) : (
                        <ShieldOff className='text-muted-foreground h-4 w-4' />
                      )}
                      <span className='font-mono text-sm'>{capability}</span>
                      <Badge variant={active ? 'default' : 'secondary'}>
                        {active ? t('Active') : t('Not held')}
                      </Badge>
                    </span>
                    <Button
                      variant={active ? 'outline' : 'default'}
                      size='sm'
                      /*
                       * ⚠ 账号 ID 不合法、或**读不到这个账号的能力**时都要禁用：
                       * 两种情况下"此刻他有没有这个能力"都是**未知**，而未知不许画成
                       * "没有"（点一下"授予"可能给一个已经有能力的人再授一次）。
                       */
                      disabled={busy || isBlockedById || !!currentError}
                      onClick={() => void toggleCapability(capability, active)}
                    >
                      {busy ? <Loader2 className='h-4 w-4 animate-spin' /> : null}
                      {active ? t('Revoke') : t('Grant')}
                    </Button>
                  </div>
                )
              })
            : null}

          {/* 历史行（撤销过的 / 过期的）—— 运营常问"他什么时候没的" */}
          {currentEntitlements?.items?.length
            ? (
                <div className='flex flex-col gap-1'>
                  <p className='text-muted-foreground text-xs'>
                    {t('History (including revoked and expired rows)')}
                  </p>
                  {currentEntitlements.items.map((row) => {
                    const state = entitlementState(row, nowSeconds)
                    return (
                      <p key={row.id} className='text-muted-foreground text-xs'>
                        <span className='font-mono'>{row.capability}</span>
                        {' · '}
                        {ENTITLEMENT_STATE_LABEL[state]}
                        {' · '}
                        {row.source}
                      </p>
                    )
                  })}
                </div>
              )
            : null}
        </CardContent>
      </Card>
    </div>
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
