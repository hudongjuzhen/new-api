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
import type { TFunction } from 'i18next'
import { Layers, Loader2, Pencil, RefreshCw, ShieldCheck, ShieldOff } from 'lucide-react'
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

import { ModeMetaDialog } from '../components/mode-meta-dialog'
import {
  grantMode,
  listEntitlementsByMode,
  listEntitlementsByUser,
  listModes,
  revokeMode,
  updateModeMeta,
  type ModeEntitlementByMode,
  type ModeEntitlementByUser,
  type ModeList,
  type ModeView,
} from '../api'
import {
  accountModeState,
  accountModeStateLabel,
  bucketModes,
  emptyModesHint,
  entitlementState,
  entitlementStateLabel,
  grantedLabel,
  isPublicMode,
  modeVisibilityLabel,
  parseUserId,
  privateModesNote,
} from '../lib/mode-admin-view'

/**
 * 模式管理（`zsy/mode` 的后台那一屏）
 *
 * 用户对这一整件事的原话：
 *
 * > "模式与插件分开吧……**模式单独来个后台菜单管理**，参考那些 应用广场，
 * >  出一个 模式广场，然后将**模式列表显示在这里**"
 *
 * 所以这一屏的主角是**模式列表**（第二张卡），而"给账号开通"是它旁边那一个动作。
 *
 * # ★★ 第三版：左边填 ID、右边就是那个人的权限表（用户 2026-… 报的那件事）
 *
 * 用户的原话：
 *
 * > "我希望左侧是输入ID的位置，右侧是选择的列表，不要每个模式或者每个插件单独占一行，
 * >  太浪费空间了。另外当输入account id的输入框是空的的时候，右侧的模式有编辑功能，
 * >  只要输入框有内容了，右侧模式的 编辑 按钮隐藏，从而显示 给权限 或者 取消权限 的按钮。
 * >  另外页面要适配国际化，我看现在很多就是英文，我希望适配上其他语言。"
 *
 * 于是这一屏的形状是：
 *
 *	┌─ 左：账号 ID ─┬─ 右：这个账号的每一档模式 ──────────────┐
 *	│  [ 7        ]  │  MV mv        公共 · 谁都能开通   [开通]   │
 *	│  说明那一行     │  有声书 audio 私有 · 按账号开通   [取消开通]│
 *	│                │  …（**一行好几个**，不再一档一行）        │
 *	└────────────────┴──────────────────────────────────────────┘
 *
 * ★★ **左边那一格是这一屏唯一的开关**：空着时右边给的是「编辑」（那是"改这一档
 * 怎么发出去"），填了之后右边换成「开通 / 取消开通」（那是"这个人有没有这一档"）。
 * 两件事共用同一块地方，是因为运营一次只会做其中一件 —— 而他不必先想清楚
 * "我现在是要编辑还是要开通"再去别处找入口。
 *
 * # ⚠★ 那一格"现在生效吗"必须**读回来**，不许按点击推
 *
 * 开通 / 取消之后一律重新读一遍（`refreshUser` + `refreshMode` + 模式列表），
 * 而不是把本地那一份改一改。理由是服务端的判据是**每次现算**的
 * （有过期、有取消），本地推一份就可能与服务端分叉 —— 而分叉的表现是
 * 界面上写着"已开通"、用户那边什么都看不到。
 *
 * # ⚠★ 这里的文案一律走 `t()`（绝不写死中文）
 *
 * 见 `lib/mode-admin-view.ts` 文件头那段：那一组曾经把中文写死在代码里，
 * 于是中文界面上正常、换任何语言都还是中文，而旁边那些走 `t()` 的字跟着语言变
 * —— 一块屏幕上两种语言并存，用户报的就是这件事。
 */
export function ModePlazaPage() {
  const { t } = useTranslation()

  const [modes, setModes] = useState<ModeList | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  /** 选中的模式 id（默认第一个能用的）—— 它只喂「这一档给了谁」那一张卡。 */
  const [selected, setSelected] = useState('')

  /** 账号 ID 那一格（**文本**：判据在 `parseUserId`，不在这里）。 */
  const [userIdText, setUserIdText] = useState('')
  /**
   * ★ 正在开通 / 取消的那一档（**只锁那一行**）。
   *
   * ⚠ 它是"这一下点过了，等它回来"的凭据：没有它的话，运营连点两下会给同一个人
   * **插两行**开通（服务端不会报错，界面上也看不出多了一行）。
   * 锁在**一行**而不是整块：这一屏的用法是"这个人的权限一把过一遍"。
   */
  const [busyModeId, setBusyModeId] = useState('')
  const [actionError, setActionError] = useState('')

  const [byUser, setByUser] = useState<ModeEntitlementByUser | null>(null)
  const [byUserOf, setByUserOf] = useState('')
  const [byUserError, setByUserError] = useState('')

  const [byMode, setByMode] = useState<ModeEntitlementByMode | null>(null)
  const [byModeOf, setByModeOf] = useState('')
  const [byModeError, setByModeError] = useState('')

  /**
   * ★★ **编辑那一档**（用户 2026-…："模式管理 应该是可以编辑的，可以设置
   * 权限是公开还是私有"）。
   *
   * ⚠ `editingId` 存的是 **id**，不是那一行对象：保存之后要重读列表，
   * 而那时手上那份行对象已经过期了 —— 存 id 才能在重读之后拿到**新的**那一行
   * （否则弹窗里显示的还是改之前那个可见性）。
   */
  const [editingId, setEditingId] = useState('')
  const [savingMeta, setSavingMeta] = useState(false)
  const [metaError, setMetaError] = useState('')

  /**
   * 判定"已取消 / 已过期"要用的"现在"。
   *
   * ⚠★ 它**不能**在渲染里 `Date.now()`：那是**渲染期读时钟**，同一份数据两次
   * 渲染可能得出不同的界面（`react(purity)` 报的就是这件事）。所以时钟只在
   * **数据落地那一刻**读一次。初值 0 是安全的：那时还没有任何一份数据。
   */
  const [nowSeconds, setNowSeconds] = useState(0)

  /*
   * ── 由状态推出来的那几格（全部放在任何 effect 之前 —— 见下面 TDZ 那条说明）──
   */
  const parsedId = parseUserId(t, userIdText)
  /** 只把**那个数字**取出来：effect 的依赖用它，不用 `parsedId`（每渲染都是新对象） */
  const targetUserId = parsedId.value
  const buckets = bucketModes(modes?.items || [])
  const selectedMode = buckets.usable.find((row) => row.id === selected)
  const selectedModeId = selectedMode?.id || ''
  const selectedVisibility = selectedMode ? modeVisibilityLabel(t, selectedMode.visibility) : ''

  /**
   * ★ 手上那份数据属于谁 / 属于哪一档 —— 三个键。
   *
   * 它是"手上这份还算不算数"的判据：账号（或选中的那一档）一变，旧数据就属于
   * **上一个**账号了 —— 而两个人都有同一档权限时看起来完全正常，那正是最危险的一种错。
   *
   * ⚠ 用"记下它属于谁"而不是"在 effect 里把它清空"，是为了避开
   * `react(set-state-in-effect)`（在 effect 里同步 setState 会引发级联渲染）。
   * 清空的活交给读取方：下面用 `isUserStale` / `isModeStale` 把"过期的数据"
   * 当成"还没有数据"。
   */
  const wantUserKey = targetUserId ? String(targetUserId) : ''
  const wantModeKey = selectedModeId
  const isUserStale = byUserOf !== wantUserKey
  const isModeStale = byModeOf !== wantModeKey
  const currentByUser = isUserStale ? null : byUser
  const currentByUserError = isUserStale ? '' : byUserError
  const currentByMode = isModeStale ? null : byMode
  const currentByModeError = isModeStale ? '' : byModeError

  const hasUserIdText = userIdText.trim().length > 0
  const isBlockedById = !!parsedId.problem

  /*
   * ── "这个账号每一档开通了没有"那一块的判据（**先在 JSX 外面算好**）──
   *
   * ⚠★ **两个不同的判据，别合成一个**（`zsy-world` 那一屏在这里踩过两次）。
   *
   * | 判据 | 回答的问题 | 什么时候为真 |
   * |---|---|---|
   * | `showsModeRows` | **有没有东西可画** | 模式库里有能用的档 |
   * | `hasFreshUser`  | **手上那份数据算不算数** | 读回来了、没报错 |
   *
   * ⚠★★ **挂载判据只许用 `showsModeRows`**（不许把 `hasFreshUser` /
   * `currentByUserError` 搭进去）。这一条是**实测**得到的：
   * 读不到时会先落一次"还没有数据"的渲染（那时 `currentByUserError` 还是空串），
   * 紧接着那次读失败落地 —— 条件是 `… && !currentByUserError` 的话，第二次渲染
   * 就把整块**连根拆掉**（React 把 `div` 换成 `null`，那是删除，不是隐藏）。
   * 症状是"这一块从来没出现过"，而日志里能看到它**明明被构造过**。
   * 判据与"这一行写什么"分开，就没有这一出。
   */
  const showsModeRows = buckets.usable.length > 0
  const hasFreshUser = !isUserStale && !currentByUserError

  /*
   * ── "这一档给了谁"那一块的三个判据（**先在 JSX 外面算好**）──
   *
   * ⚠ `no-nested-ternary` 在本仓库是开的，而"嵌套三元"写在这一块最容易读成谜题
   * （`zsy-world` 那一屏留了同一条注释）。所以三支各写一行，JSX 里最多一层三元。
   */
  const showsHolders = !currentByModeError && !!currentByMode
  const holderRows = currentByMode?.items || []
  /**
   * ★★ 空名单**不是同一句话**，取决于这一档是公共的还是私有的：
   *
   *	公共    "谁都能开通，所以没有名单可列"（这不是一个问题）
   *	私有    ★ "还没有账号被开通这一档"（★ 这多半意味着运营忘了给谁开）
   *
   * 两句合成一句（"暂无"）会把后者那件**要动手的事**说成一件平常事。
   */
  const noHoldersLine = isPublicMode(selectedMode || { visibility: '' })
    ? t('This mode is public — every account can open it, so there is nobody to list.')
    : t('No account holds this mode yet.')

  /**
   * 读一次模式列表。
   *
   * # ⚠★ 为什么它是**普通函数**而不是 `useCallback`，而且里面**不**先 `setLoading(true)`
   *
   * `react(set-state-in-effect)` 会红在"effect 里同步 setState"上 —— 而两个来源
   * 都要为此负责：`setLoading(true)` 在 effect 第一行同步跑；`useCallback` 认不出
   * "这个回调只在 event handler 里用"。所以 `loading` 的初值就是 `true`
   * （挂载时本来就在读），这里只负责**读完**把它置回 `false`。
   *
   * ⚠ `preserve` 那个参数是给「重新读取」那颗按钮用的：用户按它时**要**看到 loading。
   */
  async function loadAll(preserve = false) {
    if (preserve) setLoading(true)
    setLoadError('')
    try {
      const data = await listModes()
      setModes(data)
      /*
       * 默认选中第一个**能用的**（不是第一个）：默认选一份坏的会让运营
       * 一进来就看见一句"这一档发不出去"，而他什么都没做错。
       */
      const first = bucketModes(data.items || []).usable[0]
      setSelected((prev) => prev || first?.id || '')
    } catch (err) {
      setLoadError(readError(err, t('Could not read the mode list')))
    } finally {
      setLoading(false)
    }
  }

  /**
   * 拉一次这个账号的授权（每次现算 —— 与判权同源）。
   *
   * ⚠ 读不到就**说读不到**，而不是把"没读到"画成"没有权限"：后者会让运营
   * 对着一个其实已经有权限的人再点一次「开通」（服务端会多插一行，看不出错）。
   */
  async function refreshUser(userId: number) {
    const key = userId ? String(userId) : ''
    if (!key) return
    try {
      const data = await listEntitlementsByUser(userId)
      setByUser(data)
      setByUserError('')
      setByUserOf(key)
      /* ★ "现在"在数据落地这一刻读一次（渲染期不许读时钟，见 `nowSeconds`） */
      setNowSeconds(Math.floor(Date.now() / 1000))
    } catch (err) {
      setByUser(null)
      setByUserError(readError(err, t('Could not read this account’s mode opens')))
      setByUserOf(key)
    }
  }

  /** 拉一次这一档"给了谁"（同上：普通函数，不做 memo）。 */
  async function refreshMode(modeId: string) {
    if (!modeId) return
    try {
      const data = await listEntitlementsByMode(modeId)
      setByMode(data)
      setByModeError('')
      setByModeOf(modeId)
    } catch (err) {
      setByMode(null)
      setByModeError(readError(err, t('Could not read who holds this mode')))
      setByModeOf(modeId)
    }
  }

  useEffect(() => {
    void loadAll()
    /*
     * ⚠ 依赖是空数组：`loadAll` 是**普通函数**（不是 memo 的回调），把它写进依赖里
     * 会让这个 effect 每一轮渲染都跑一次 —— 而它做的是**取一次模式列表**。
     * 首次挂载取一次就够了（要重取有「重新读取」那颗按钮）。
     */
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  /*
   * 账号或模式一变就去读一次。⚠ 这里**不**清空旧数据（那会犯 `set-state-in-effect`）：
   * 清空交给 `isUserStale` / `isModeStale`。
   *
   * 依赖用那两个**字符串**：它们由"账号 id"与"模式 id"拼成，所以输入没变它们就不变，
   * 而 `parsedId` 那种对象引用每次渲染都会变。
   */
  useEffect(() => {
    void refreshUser(targetUserId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wantUserKey])

  useEffect(() => {
    void refreshMode(selectedModeId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wantModeKey])

  /**
   * 开通 / 取消**某一档**（列表里那一下 —— 用户要的就是"一次填 ID，全都在这儿点完"）。
   *
   * ⚠ 它不是 `useCallback`：只在按钮的 onClick 里用，没有任何东西把它当依赖传递。
   */
  async function toggle(modeId: string, active: boolean) {
    if (parsedId.problem) {
      setActionError(parsedId.problem)
      return
    }
    setBusyModeId(modeId)
    setActionError('')
    try {
      if (active) {
        await revokeMode(targetUserId, modeId)
      } else {
        /*
         * ⚠ 第三个参数（到期时间）**必须显式传 0**：服务端的 `expires_at` 缺省即
         * "永不过期"，所以少传**不会报错** —— 但"参数个数"因此不再是契约的一部分，
         * 而界面看起来完全正常（与 `zsy-world` 那一处同一条教训）。
         */
        await grantMode(targetUserId, modeId, 0)
      }
      /*
       * ★ 三件都要重读：账号手上有什么、这一档给了谁、以及**每档几个人**那一个数
       * （它显示在模式库上，不重读的话运营刚开通完看到的还是旧计数）。
       */
      await Promise.all([refreshUser(targetUserId), refreshMode(modeId), loadAll()])
    } catch (err) {
      setActionError(readError(err, active ? t('Could not cancel') : t('Could not open')))
    } finally {
      setBusyModeId('')
    }
  }

  /**
   * ★★ **保存那一档的分发策略**（公开 / 私有 + 说明）。
   *
   * ⚠★ 保存之后**重读列表**（不就地改本地那一行）：服务端回的才是权威
   * （它会把可见性 trim、把文件里真实的值读回来）—— 本地推一份就可能与磁盘分叉，
   * 而分叉的表现是"界面上写着公开、而客户端那边看不到"。
   *
   * ⚠ 成功之后**关掉弹窗**：不关的话运营会对着一个"已经存过了"的表单再点一次保存
   * （而第二次保存没有任何效果，看起来像"保存没反应"）。
   */
  async function saveMeta({ visibility, summary }: { visibility: string; summary: string }) {
    const id = editingId
    if (!id) return
    setSavingMeta(true)
    setMetaError('')
    try {
      await updateModeMeta(id, { visibility, summary })
      await loadAll()
      setEditingId('')
    } catch (err) {
      setMetaError(readError(err, t('Could not save')))
    } finally {
      setSavingMeta(false)
    }
  }

  return (
    /*
     * ★★ **必须包一层 `SectionPageLayout`**（用户 2026-… 报的那件事）：
     * `SidebarInset` 是 `h-[calc(100svh-…)] overflow-hidden`，**它自己不滚** ——
     * 滚动条由 `SectionPageLayout` 里那个 `overflow-auto` 的容器提供。
     * 少了这一层，内容一多**下面就看不见了，而且没有任何滚动条**（不报错，
     * 只是"页面像被截断了"）。
     *
     * ⚠ 这里**不加 `fixedContent`**：那一档是给"页面内自己有滚动表格"的屏用的
     * （`overflow-hidden` + 内部滚动）—— 这一屏是普通的文档式滚动。
     */
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Mode Plaza')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' size='sm' onClick={() => void loadAll(true)}>
          <RefreshCw className='mr-1 h-4 w-4' />
          {t('Reload')}
        </Button>
      </SectionPageLayout.Actions>

      <SectionPageLayout.Content>
        <div className='flex flex-col gap-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Every mode the library holds, including the ones that cannot be shipped. Public modes are open to every account already; private ones appear in the plaza only for the accounts opened below.'
            )}
          </p>

          {/*
        ★★ 这一条横幅是整屏最要紧的一句话：私有的模式**在客户端根本不显示**，
        而"取消"**不会**删掉用户机器上那一份。两件都与运营的直觉相反，
        所以它不是提示气泡而是常驻的 Alert。
      */}
      <Alert data-testid='mode-private-note'>
        <AlertTitle>{t('Private modes are hidden, not greyed out')}</AlertTitle>
        <AlertDescription>{privateModesNote(t)}</AlertDescription>
      </Alert>

      {/*
        ────────────────────── 一、给某个账号开通（真的那一步） ──────────────────────

        ★★ 这一张卡是**这一版的形状**（用户 2026-…）：
        左边那一格是"这个人是谁"，右边就是"他手上每一档是怎么回事"。
        ⚠ 模式库那一张卡（下面）仍然在：它是"磁盘上有哪几档"的账，
        而这一张是"这个人的权限"的账 —— 两件事，两个方向。
      */}
      <Card>
        <CardHeader>
          <CardTitle>{t('Open modes for an account')}</CardTitle>
          <CardDescription>
            {t(
              'Enter the account ID first, then this account’s every mode is listed at once — open it, or cancel it, right here. Decided here, and re-checked on every request: a cancel takes effect the next time that account refreshes the plaza, with no need to log in again.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-4'>
          {/*
            ★★ 左 ID / 右列表。窄屏（`lg` 以下）自动叠成一列 —— 那时候把 ID 挤在
            左边会让右边只剩一条缝，而"一行放好几个"正是这一版要的。
          */}
          <div className='flex flex-col gap-4 lg:flex-row lg:items-start'>
            <div className='flex shrink-0 flex-col gap-2 lg:w-64'>
              <Label htmlFor='mode-user-id'>{t('Account ID')}</Label>
              <Input
                id='mode-user-id'
                value={userIdText}
                onChange={(e) => setUserIdText(e.target.value)}
                placeholder={t('e.g. 7')}
                className='w-full'
              />
              {/* 填错时**当场**说，而不是点下去之后由服务端说一句"需要 user_id" */}
              {userIdText && isBlockedById ? (
                <p className='text-destructive text-xs'>{parsedId.problem}</p>
              ) : (
                <p className='text-muted-foreground text-xs' data-testid='mode-open-hint'>
                  {t(
                    'Leave it empty to edit how each mode is handed out; fill it in to open or cancel that account’s modes.'
                  )}
                </p>
              )}
            </div>

            <div className='flex min-w-0 flex-1 flex-col gap-3'>
              {actionError ? (
                <Alert variant='destructive' data-testid='mode-action-error'>
                  <AlertDescription>{actionError}</AlertDescription>
                </Alert>
              ) : null}

              {/*
                ★★ 读不到账号的授权时：**列表照样画**（用户看得见有哪几档），
                而每一行那一格写着"状态读不到"、按钮全禁用，服务端那句话原样摆在上面。

                ⚠★ 关键在于"点不动"而不是"不画"：此刻"他有没有这一档"是**未知** ——
                未知不许画成"未开通"（点一下"开通"可能给一个已经有权限的人再插一行）。
              */}
              {hasUserIdText && currentByUserError ? (
                <Alert variant='destructive' data-testid='mode-entitlements-error'>
                  <AlertDescription>
                    {currentByUserError}{' '}
                    {t('Modes cannot be opened or cancelled until this is read.')}
                  </AlertDescription>
                </Alert>
              ) : null}

              {/*
                ★★ **这一块的挂载判据只有 `showsModeRows`**（见上面那段说明）：
                还没读到 / 读失败都**不换掉它**。状态由每一行自己说。
              */}
              {showsModeRows ? (
                /*
                 * ★★ **一行放好几个**（用户 2026-…："不要每个模式或者每个插件单独占一行，
                 * 太浪费空间了"）。每一行只有一行字那么高，所以一屏能看十几档。
                 */
                <div
                  data-testid='mode-account-grid'
                  className='grid grid-cols-1 gap-1.5 xl:grid-cols-2'
                >
                  {buckets.usable.map((row) => (
                    <ModeRow
                      key={row.id}
                      t={t}
                      row={row}
                      selected={row.id === selected}
                      hasUserIdText={hasUserIdText}
                      state={accountModeState({
                        readable: hasFreshUser,
                        active: currentByUser?.active || [],
                        modeId: row.id,
                      })}
                      busy={busyModeId === row.id}
                      disabled={busyModeId === row.id || isBlockedById || !hasFreshUser}
                      onSelect={() => setSelected(row.id)}
                      onEdit={() => {
                        setMetaError('')
                        setEditingId(row.id)
                      }}
                      onToggle={(open) => void toggle(row.id, open)}
                    />
                  ))}
                </div>
              ) : (
                <p className='text-muted-foreground text-sm' data-testid='mode-account-empty'>
                  {emptyModesHint(t, modes?.directory || '', modes?.knownMediums || [])}
                </p>
              )}

              {/*
                ★ 公共模式那几行上"未开通"不是一个问题（谁都能开通）—— 那一句必须说出来，
                否则运营会以为他要把公共模式逐个开一遍。
              */}
              {hasUserIdText && hasFreshUser ? (
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'Public modes need no opening — every account can open them. The state here tells you whether this account was opened for it individually.'
                  )}
                </p>
              ) : null}

              {/* 这个账号手上**现在生效**的那几档（现算，不是从历史行推的） */}
              {hasUserIdText && currentByUser?.active?.length ? (
                <div className='flex flex-wrap items-center gap-2'>
                  <span className='text-muted-foreground text-xs'>
                    {t('Modes opened for this account')}
                  </span>
                  {(currentByUser?.active || []).map((id) => (
                    <Badge key={id} variant='secondary' className='font-mono'>
                      {id}
                    </Badge>
                  ))}
                </div>
              ) : null}

              {/* 历史行（取消过的 / 过期的）—— 运营常问"他什么时候没的" */}
              {hasUserIdText && currentByUser?.items?.length ? (
                <details data-testid='mode-history'>
                  <summary className='text-muted-foreground cursor-pointer text-xs'>
                    {t('History (including cancelled and expired rows)')}
                  </summary>
                  <div className='mt-1 flex flex-col gap-1'>
                    {currentByUser.items.map((row) => {
                      const state = entitlementState(row, nowSeconds)
                      return (
                        <p key={row.id} className='text-muted-foreground text-xs'>
                          <span className='font-mono'>{row.modeId}</span>
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

      {/* ────────────────────────── 二、模式库（有哪几档） ────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle>{t('Mode library')}</CardTitle>
          <CardDescription>
            {modes?.directory
              ? t('Read from {{dir}} — the file name is the plugin id.', {
                  dir: modes.directory,
                })
              : t('Loading…')}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-3'>
          {loading ? <Skeleton className='h-20 w-full' /> : null}

          {!loading && loadError ? (
            <Alert variant='destructive' data-testid='mode-list-error'>
              <AlertDescription>{loadError}</AlertDescription>
            </Alert>
          ) : null}

          {!loading && !loadError ? (
            <>
              {/*
                ★ 坏模式**排在最前面**并带上原因：它是运营唯一需要动手去修的东西，
                而"我放进去的文件不见了"看起来像后台坏了。
              */}
              {buckets.broken.map((row) => (
                <Alert key={row.id} variant='destructive'>
                  <AlertTitle>{row.label || row.id}</AlertTitle>
                  <AlertDescription>
                    <span className='block'>{row.problem}</span>
                    <span className='text-muted-foreground mt-1 block text-xs'>{row.source}</span>
                  </AlertDescription>
                </Alert>
              ))}

              {buckets.usable.length ? (
                /*
                 * ★★ 一行放**好几档**，而且每一档只占一行字的高度。
                 *
                 * ⚠ 这一块的按钮是「选中」（它喂下面"这一档给了谁"那张卡），
                 * 与上面那一块按账号开通/取消**不是同一件事** —— 所以两块都在，
                 * 各自解决一个问题。
                 */
                <div className='grid grid-cols-1 gap-1.5 xl:grid-cols-2'>
                  {buckets.usable.map((row) => {
                    const on = row.id === selected
                    return (
                      <div
                        key={row.id}
                        data-testid={`mode-row-${row.id}`}
                        data-selected={on ? 'true' : 'false'}
                        className={`flex items-center justify-between gap-2 rounded-md border px-2.5 py-1.5 transition-colors ${
                          on ? 'border-primary bg-accent/40' : 'hover:bg-accent/20'
                        }`}
                      >
                        {/*
                          ⚠★ 那一行**不是**整块 button：里面还有「编辑」，
                          而 button 里套 button 是非法结构、点「编辑」还会同时把这一行选中
                          （两件事一起发生）。所以外层是 div，"点它就选中"落在里面那个
                          铺满的 button 上。
                        */}
                        <button
                          type='button'
                          aria-current={on ? 'true' : undefined}
                          onClick={() => setSelected(row.id)}
                          className='flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1 text-left'
                        >
                          <span className='font-medium'>{row.label}</span>
                          <span className='text-muted-foreground font-mono text-xs'>{row.id}</span>
                          <Badge variant='outline'>{row.medium}</Badge>
                          {/*
                            ★★ 可见性必须看得见（它是这一档"发给谁"的那一格）。
                            它只差一个词，后果却完全不同 —— 而文件本身长得一模一样。
                          */}
                          <Badge
                            variant={isPublicMode(row) ? 'default' : 'secondary'}
                            data-testid={`mode-visibility-${row.id}`}
                          >
                            {modeVisibilityLabel(t, row.visibility)}
                          </Badge>
                          <span className='text-muted-foreground text-xs'>
                            {grantedLabel(t, row)}
                          </span>
                        </button>
                        {/*
                          ★★ 「编辑」= 改这一档的**分发策略**（公开 / 私有 + 说明）。
                          它**在行内**（不是藏进一个"⋯"菜单里）：这一格是运营天天要动的
                          那一个，而"藏起来"的代价是他找不到"公开/私有到底在哪儿改"。
                        */}
                        <Button
                          variant='outline'
                          size='xs'
                          data-testid={`mode-edit-library-${row.id}`}
                          onClick={() => {
                            setMetaError('')
                            setEditingId(row.id)
                          }}
                        >
                          <Pencil className='mr-1 size-3' />
                          {t('Edit')}
                        </Button>
                      </div>
                    )
                  })}
                </div>
              ) : (
                <p className='text-muted-foreground text-sm'>
                  {emptyModesHint(t, modes?.directory || '', modes?.knownMediums || [])}
                </p>
              )}

              {modes?.note ? (
                <p className='text-muted-foreground text-xs'>{modes.note}</p>
              ) : null}
            </>
          ) : null}
        </CardContent>
      </Card>

      {/* ────────────────────────── 三、这一档给了谁 ────────────────────────── */}
      <Card>
        <CardHeader>
          <CardTitle>{t('Who holds the selected mode')}</CardTitle>
          <CardDescription>
            {selectedMode
              ? `${selectedMode.label} · ${selectedVisibility}`
              : t('Pick a mode above.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='flex flex-col gap-2'>
          {currentByModeError ? (
            <Alert variant='destructive' data-testid='mode-holders-error'>
              <AlertDescription>{currentByModeError}</AlertDescription>
            </Alert>
          ) : null}

          {/*
            ⚠★ 读不到时说读不到，**不画成"还没有人"**：那个空名单是一个结论
            （"这一档没人有权限"），把它当成一次读取失败的表现会让运营照着它
            去重复授权。与上面 `refreshUser` 那一条同源。
          */}
          {!currentByModeError && !currentByMode && selectedModeId ? (
            <Skeleton className='h-8 w-full' />
          ) : null}

          {showsHolders
            ? holderRows.map((row) => (
                <p key={row.id} className='text-sm'>
                  <span className='font-mono'>{row.userId}</span>
                  {' · '}
                  <span className='text-muted-foreground text-xs'>
                    {entitlementStateLabel(t, entitlementState(row, nowSeconds))}
                  </span>
                </p>
              ))
            : null}

          {showsHolders && holderRows.length === 0 ? (
            <p className='text-muted-foreground text-sm' data-testid='mode-no-holders'>
              {noHoldersLine}
            </p>
          ) : null}
        </CardContent>
      </Card>

          <p className='text-muted-foreground flex items-center gap-1 text-xs'>
            <Layers className='h-3 w-3' />
            {t(
              'The library is a directory on the server: adding or removing a mode is adding or removing a file, never a database migration.'
            )}
          </p>
        </div>

        {/*
          ★★ 「编辑」那个弹窗挂在最外层（`SectionPageLayout` 里面、内容之外都行）：
          它是**覆盖层**，不该受内容的滚动与层级影响（放进 `Content` 里也行，
          但那样它会跟着内容一起被滚走 —— 而一个"跟着滚"的弹窗在长列表上是看得见的坏）。
        */}
        <ModeMetaDialog
          /* ★ `key` = 换一档就换一个实例（草稿跟着重新初始化，见那个组件的说明） */
          key={editingId || 'none'}
          mode={buckets.usable.find((row) => row.id === editingId) || null}
          open={!!editingId}
          saving={savingMeta}
          error={metaError}
          onOpenChange={(next) => {
            if (!next) {
              setEditingId('')
              setMetaError('')
            }
          }}
          onSubmit={(patch) => void saveMeta(patch)}
        />
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

/**
 * 右侧列表里的**一行**：左边那一格填了账号 ID 时，它就是"这个人有没有这一档"；
 * 空着时，它就是"这一档怎么发出去"（带「编辑」）。
 *
 * # ★★ 为什么这一行有两副面孔（用户 2026-… 点名要的）
 *
 * 用户的原话：
 *
 * > "当输入account id的输入框是空的的时候，右侧的模式有编辑功能，只要输入框有内容了，
 * >  右侧模式的 编辑 按钮隐藏，从而显示 给权限 或者 取消权限 的按钮。"
 *
 * 所以那一枚按钮由 `hasUserIdText` 一句决定，**两件事不同时出现** ——
 * 运营一次只做其中一件，而他不必先想清楚"我现在是要编辑还是要开通"再去别处找入口。
 *
 * ⚠★ 这一行不是"通用组件"：它只服务这一屏，而它的形状与下面那一块一一对应 ——
 * 抽到公共目录反而要传一堆 props（`AGENTS.md` 那条"只有一处调用者的辅助函数就地内联"
 * 的同一条理）。
 */
function ModeRow({
  t,
  row,
  selected,
  hasUserIdText,
  state,
  busy,
  disabled,
  onSelect,
  onEdit,
  onToggle,
}: {
  t: TFunction
  row: ModeView
  selected: boolean
  hasUserIdText: boolean
  state: 'unknown' | 'open' | 'closed'
  busy: boolean
  disabled: boolean
  onSelect: () => void
  onEdit: () => void
  onToggle: (open: boolean) => void
}) {
  const open = state === 'open'
  /*
   * ⚠★ 「开通 / 取消开通」那两枚按钮上的字只有两三个字（一行放好几个的前提），
   * 所以它们的**可访问名**要带上这一档的名字 —— 否则读屏用户听到的是一长串
   * 一模一样的"开通"。`aria-label` 不占视觉空间，正是这里要的东西。
   */
  const actionLabel = open
    ? t('Cancel access for {{mode}}', { mode: row.label })
    : t('Open {{mode}} for this account', { mode: row.label })

  return (
    <div
      data-testid={`mode-grant-${row.id}`}
      data-state={hasUserIdText ? state : 'editing'}
      className={`flex items-center justify-between gap-2 rounded-md border px-2.5 py-1.5 ${
        selected && !hasUserIdText ? 'border-primary bg-accent/40' : ''
      }`}
    >
      {/*
        ⚠ 空着 ID 时这一行**可点**（点它就选中，下面"这一档给了谁"跟着它走）；
        填了 ID 之后它是**纯文字** —— 那时运营要的是开通/取消，而不是再选一次。
      */}
      {hasUserIdText ? (
        <span className='flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1'>
          <ModeRowText t={t} row={row} hasUserIdText={hasUserIdText} state={state} />
        </span>
      ) : (
        <button
          type='button'
          aria-current={selected ? 'true' : undefined}
          onClick={onSelect}
          className='flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1 text-left'
        >
          <ModeRowText t={t} row={row} hasUserIdText={hasUserIdText} state={state} />
        </button>
      )}

      {hasUserIdText ? (
        <Button
          variant={open ? 'outline' : 'default'}
          size='xs'
          disabled={disabled}
          aria-label={actionLabel}
          onClick={() => onToggle(open)}
        >
          {busy ? <Loader2 className='mr-1 size-3 animate-spin' /> : null}
          {open ? t('Cancel access') : t('Open')}
        </Button>
      ) : (
        <Button
          variant='outline'
          size='xs'
          data-testid={`mode-edit-${row.id}`}
          onClick={onEdit}
        >
          <Pencil className='mr-1 size-3' />
          {t('Edit')}
        </Button>
      )}
    </div>
  )
}

/**
 * 那一行里的字（两副面孔共用）：名字 + id + 可见性 + 状态。
 *
 * ⚠ 抽出来是因为两副面孔（可点的 button / 纯文字的 span）**必须长得一模一样** ——
 * 各写一份的代价是"填了 ID 之后那一行悄悄变了个样"，而运营会以为换了一块东西。
 */
function ModeRowText({
  t,
  row,
  hasUserIdText,
  state,
}: {
  t: TFunction
  row: ModeView
  hasUserIdText: boolean
  state: 'unknown' | 'open' | 'closed'
}) {
  const open = state === 'open'
  return (
    <>
      <span className='font-medium'>{row.label}</span>
      <span className='text-muted-foreground font-mono text-xs'>{row.id}</span>
      <Badge variant='outline'>{row.medium}</Badge>
      <Badge variant={isPublicMode(row) ? 'default' : 'secondary'}>
        {modeVisibilityLabel(t, row.visibility)}
      </Badge>
      {hasUserIdText ? (
        <Badge
          variant={open ? 'default' : 'secondary'}
          data-testid={`mode-state-${row.id}`}
          className='gap-1'
        >
          {open ? <ShieldCheck className='size-3' /> : <ShieldOff className='size-3' />}
          {accountModeStateLabel(t, state)}
        </Badge>
      ) : (
        <span className='text-muted-foreground text-xs'>{grantedLabel(t, row)}</span>
      )}
    </>
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
