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
import {
  Layers,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  ShieldCheck,
  ShieldOff,
  Users,
} from 'lucide-react'
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
import { SectionPageLayout } from '@/components/layout'

import { ModeCreateDialog } from '../components/mode-create-dialog'
import { ModeEditDialog } from '../components/mode-edit-dialog'
import { ModeHoldersDialog } from '../components/mode-holders-dialog'
import {
  grantMode,
  listEntitlementsByUser,
  listModes,
  revokeMode,
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
 * 开通 / 取消之后一律重新读一遍（`refreshUser` + 模式列表），
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

  /**
   * ★★ **编辑那一档**（用户 2026-…："模式管理 应该是可以编辑的，可以设置
   * 权限是公开还是私有"）。
   *
   * ⚠ `editingId` 存的是 **id**，不是那一行对象：保存之后要重读列表，
   * 而那时手上那份行对象已经过期了 —— 存 id 才能在重读之后拿到**新的**那一行
   * （否则弹窗里显示的还是改之前那个可见性）。
   */
  const [editingId, setEditingId] = useState('')
  /**
   * ★★ **添加模式**（用户 2026-…："右上角增加一个添加模式的功能"）。
   *
   * ⚠ 它只记"弹窗开着没有"：那四格输入与生成那一下全在弹窗自己身上
   * （关掉就该丢，而放在这里会让它们跨次存活 —— 运营会看到上一次那几个字）。
   */
  const [creating, setCreating] = useState(false)

  /**
   * ★★ **正在看"谁有这一档的权限"**（用户 2026-…："再加上一个点击查看拥有权限的
   * 账号列表的按钮，点击出来对应的弹窗可以显示"）。
   *
   * ⚠ 存 **id** 而不是那一行对象：读完名单要回填"这一档现在几个人"，
   * 而那时手上那份行对象已经过期了（与 `editingId` 同一条理由）。
   */
  const [holdersId, setHoldersId] = useState('')
  /**
   * ★ 这一档现在几个账号有权限（名单弹窗读完回填那一行）。
   *
   * ⚠ 它是**一份补丁**而不是"把模式列表重读一遍"：名单那一次请求已经拿到了准确的
   * 生效行数，为了一个数字再读一遍列表会让"点开名单"变成两次往返。
   */
  const [grantedCounts, setGrantedCounts] = useState<Record<string, number>>({})

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
  /** ★ 正在看名单的那一档（弹窗要它的 label 与可见性）。 */
  const holdersMode = buckets.usable.find((row) => row.id === holdersId) || null

  /**
   * ★ 手上那份数据属于谁 —— 一个键。
   *
   * 它是"手上这份还算不算数"的判据：账号一变，旧数据就属于**上一个**账号了 ——
   * 而两个人都有同一档权限时看起来完全正常，那正是最危险的一种错。
   *
   * ⚠ 用"记下它属于谁"而不是"在 effect 里把它清空"，是为了避开
   * `react(set-state-in-effect)`（在 effect 里同步 setState 会引发级联渲染）。
   * 清空的活交给读取方：下面用 `isUserStale` 把"过期的数据"当成"还没有数据"。
   */
  const wantUserKey = targetUserId ? String(targetUserId) : ''
  const isUserStale = byUserOf !== wantUserKey
  const currentByUser = isUserStale ? null : byUser
  const currentByUserError = isUserStale ? '' : byUserError

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
       * ⚠ 这里**不再**"默认选中第一档"（用户 2026-…）：选中这个概念随着
       * 「这一档给了谁」那张卡一起没了 —— 名单现在由每一行上的按钮打开。
       */
      void data
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
   * 账号一变就去读一次。⚠ 这里**不**清空旧数据（那会犯 `set-state-in-effect`）：
   * 清空交给 `isUserStale`。
   *
   * 依赖用那个**字符串**：它由"账号 id"拼成，所以输入没变它就不变，
   * 而 `parsedId` 那种对象引用每次渲染都会变。
   */
  useEffect(() => {
    void refreshUser(targetUserId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [wantUserKey])

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
       * ★★ **两件都要重读**：这个账号手上有什么（右栏每一行的状态由它算出来）、
       * 以及模式列表（每一行上"已给 N 个账号开通"那一个数）。
       *
       * ⚠ 第三件（"这一档给了谁"的名单）**不在这里读** —— 它现在是弹窗的事，
       * 而弹窗每次打开都会自己读一遍（那比"跟着每一次开通去读"省一半请求）。
       */
      await Promise.all([refreshUser(targetUserId), loadAll()])
    } catch (err) {
      setActionError(readError(err, active ? t('Could not cancel') : t('Could not open')))
    } finally {
      setBusyModeId('')
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
        {/*
          ★★ **右上角那颗「添加模式」**（用户 2026-…："右上角增加一个添加模式的功能，
          点击添加模式，可以输入模式名称，模式类型，模式简介，然后能够 AI一键生成，
          选择一个API密钥，然后调用 glm-5.3-flash 这个模型生成"）。

          ⚠ 它排在「重新读取」**左边**：这一屏的主语是"这一档模式"，
          而重读是维护动作（与"在列表里选一个人"同一类，不是这一屏的目的）。
        */}
        <Button size='sm' onClick={() => setCreating(true)} data-testid='mode-create-open'>
          <Plus className='mr-1 h-4 w-4' />
          {t('Add a mode')}
        </Button>
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
            ★★ 左 ID / 右列表。窄屏（`md` 以下）自动叠成一列 —— 那时候把 ID 挤在
            左边会让右边只剩一条缝，而"一行放好几个"正是这一版要的。

            ⚠ 断点用 `md`（48rem）而不是 `lg`（64rem）：左边那一格只有一格输入框 +
            一行说明，它不需要 640px 那么宽 —— 早点并排，右边就能早点多出一列
            （实测：用 `lg` 时右边在常见笔记本上只有一列，看起来像"没变化"）。
          */}
          <div className='flex flex-col gap-4 md:flex-row md:items-start'>
            <div className='flex shrink-0 flex-col gap-2 md:w-56'>
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
                 *
                 * ⚠★★ 列数是**按实际可用宽度算**的（`auto-fill` + 最小列宽），
                 * **不用 `sm:` / `xl:` 那种断点** —— 这一条是实测改的：
                 * 断点看的是**视口**宽度，而这一屏的内容区被侧边栏与内边距吃掉一截，
                 * 于是 `xl:grid-cols-2`（Tailwind 4 里 = 1280px）在这块屏幕上**从来没生效**，
                 * 表现就是"一行一个、跟没改一样"（用户 2026-… 报的原话）。
                 * `minmax(18rem, 1fr)` 里的 **18rem 是唯一要调的那个数**：
                 * 它决定"内容区至少多宽才排得下两列"。⚠★ 第一版给的 22rem 偏大 ——
                 * 内容区要 736px 才两列，而带侧边栏时那大约要 **1472px 视口**，
                 * 于是常见笔记本上仍然是一列、看起来"跟没改一样"（用户 2026-… 第二次报的话）。
                 * 18rem 让两列在约 1248px 视口就出现；宽屏会给到三列。
                 */
                <div
                  data-testid='mode-account-grid'
                  className='grid gap-1.5'
                  style={{ gridTemplateColumns: 'repeat(auto-fill, minmax(18rem, 1fr))' }}
                >
                  {buckets.usable.map((row) => (
                    <ModeRow
                      key={row.id}
                      t={t}
                      row={row}
                      /* ★ 这一档现在几个人（名单弹窗读完回填的那一份补丁优先） */
                      granted={grantedCounts[row.id] ?? row.granted}
                      hasUserIdText={hasUserIdText}
                      state={accountModeState({
                        readable: hasFreshUser,
                        active: currentByUser?.active || [],
                        modeId: row.id,
                      })}
                      busy={busyModeId === row.id}
                      disabled={busyModeId === row.id || isBlockedById || !hasFreshUser}
                      onEdit={() => setEditingId(row.id)}
                      onViewHolders={() => setHoldersId(row.id)}
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

      {/*
        ★★ **「模式库」那一张卡没有了**（用户 2026-…）：

        > "下面的 模式库 就没有必要显示了吧，直接在上面右侧的模式列表就可以了"

        原来它列的就是"磁盘上有哪几档"，而上面那一张卡的右栏**列的是同一批** ——
        同一件事写两遍，正是下一步会分叉的地方。所以现在只有一处：

        	账号卡右栏 = 这一档的**全部**事实（名字 / id / 媒介 / 可见性 /
        	这一档给了几个账号 / 编辑 / 查看拥有权限的账号）

        ⚠★ 但**坏模式必须留着**（下面那一段）：它原来排在模式库那一块的头上，
        跟着卡片一起删掉就变成"我放进去的文件不见了"，而运营唯一能动手修的东西
        正是它 —— 服务端那一面也一直坚持"坏文件照样列出来并说明原因"。

        ⚠ 「这一档给了谁」那张卡也一起撤了：它的入口现在是每一行上的
        「查看拥有权限的账号」按钮（弹窗），比"先选中一行、再往下找名单"少一步。
      */}
      {buckets.broken.length ? (
        <Card>
          <CardHeader>
            <CardTitle>{t('Modes that cannot be shipped')}</CardTitle>
            <CardDescription>
              {t(
                'These files are in the mode directory but cannot be handed to a client. Fix the file, then click Reload.'
              )}
            </CardDescription>
          </CardHeader>
          <CardContent className='flex flex-col gap-3'>
            {buckets.broken.map((row) => (
              <Alert key={row.id} variant='destructive' data-testid={`mode-broken-${row.id}`}>
                <AlertTitle>{row.label || row.id}</AlertTitle>
                <AlertDescription>
                  <span className='block'>{row.problem}</span>
                  <span className='text-muted-foreground mt-1 block text-xs'>{row.source}</span>
                </AlertDescription>
              </Alert>
            ))}
          </CardContent>
        </Card>
      ) : null}

      {/*
        ⚠ 列表读失败时说清楚（它是**上面那一张卡**的数据源，所以那句话留在上面那一张卡里）。
      */}
      {!loading && loadError ? (
        <Alert variant='destructive' data-testid='mode-list-error'>
          <AlertDescription>{loadError}</AlertDescription>
        </Alert>
      ) : null}


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
        <ModeEditDialog
          /* ★ `key` = 换一档就换一个实例（草稿跟着重新初始化，见那个组件的说明） */
          key={editingId || 'none'}
          mode={buckets.usable.find((row) => row.id === editingId) || null}
          open={!!editingId}
          onOpenChange={(next) => {
            if (!next) setEditingId('')
          }}
          /* ★★ 存完**重读列表**：服务端那份才是权威（它会把可见性 trim、把人数读回来） */
          onSaved={() => void loadAll()}
        />

        {/*
          ★★ 「谁有这一档的权限」那个弹窗（用户 2026-…）：
          它由每一行上那一颗按钮打开，而**名单是它自己读的** ——
          页面不再为了"选中那一档"去预先读一份名单（那正是它取代掉的那张卡）。
        */}
        <ModeHoldersDialog
          mode={holdersMode}
          open={!!holdersId}
          onOpenChange={(next) => {
            if (!next) setHoldersId('')
          }}
          onGrantedCount={(modeId, count) =>
            setGrantedCounts((prev) => ({ ...prev, [modeId]: count }))
          }
        />

        {/*
          ★★ 「添加模式」那个弹窗（用户 2026-…）：四格输入 + 一颗「AI 一键生成」。
          ⚠ 生成成功之后**重读列表** —— 新那一档就在磁盘上了，界面得跟着出现它。
        */}
        <ModeCreateDialog
          open={creating}
          onOpenChange={setCreating}
          onCreated={() => void loadAll()}
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
  granted,
  hasUserIdText,
  state,
  busy,
  disabled,
  onEdit,
  onViewHolders,
  onToggle,
}: {
  t: TFunction
  row: ModeView
  /**
   * ★ 这一档现在几个人有权限。
   *
   * ⚠ 它与 `row.granted` **不是同一份**：打开过名单弹窗之后，弹窗读回来的那个数
   * 比列表里那份新（列表是页面加载时读的）。所以这里接的是"手上最新的那个数"。
   */
  granted: number
  hasUserIdText: boolean
  state: 'unknown' | 'open' | 'closed'
  busy: boolean
  disabled: boolean
  onEdit: () => void
  onViewHolders: () => void
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

  /*
   * ★★ 这一行**画成什么颜色**（用户 2026-…）：
   *
   * > "输入了ID之后，已开通和未开通的背景颜色换一下吧，未开通和没有输入ID的颜色
   * >  是一样的，保持现在这样就行，已开通的弄个深色主题，是不是就更好了"
   *
   * 于是三支，而只有一支有颜色：
   *
   *	没填 ID / 未开通  → ★ **一个字节都不改**（用户点名要"保持现在这样"）
   *	已开通            → ★ 反色（`bg-foreground text-background`）
   *	状态读不到        → 反色（它与"已开通"是**同一句"这个人现在能用了"**，
   *	                    而这一屏的纪律是"未知不许画成没有"——见 `accountModeState`）
   *
   * ⚠★ 为什么是 `bg-foreground text-background` 而不是写死的深灰：
   * 这个站有**一整套主题变量 + 十个预设**（`theme-presets.css`），还各有暗色一版。
   * 写死 `bg-slate-800 text-white` 在暗色主题下就是**深色压深色**（那一行会糊掉）。
   * `bg-foreground text-background` 是仓库里现成的反色写法（`public-header.tsx`
   * 的按钮、`tooltip.tsx` 的气泡），亮色下正是"深色卡片"，
   * 而暗色下自动反过来 —— 十套预设里它都是**反色**，读得清。
   */
  const tinted = open || state === 'unknown'
  const rowTone = tinted ? `border-transparent bg-foreground text-background` : ''

  return (
    <div
      data-testid={`mode-grant-${row.id}`}
      data-state={hasUserIdText ? state : 'editing'}
      /* ★ 颜色也放进 `data-*`：测试与排查时不用去猜 class 拼出来的结果 */
      data-tone={tinted ? 'inverted' : 'plain'}
      className={`flex items-center justify-between gap-2 rounded-md border px-2.5 py-1.5 ${rowTone}`}
    >
      <span className='flex min-w-0 flex-1 flex-wrap items-center gap-x-2 gap-y-1'>
        <ModeRowText
          t={t}
          row={row}
          granted={granted}
          hasUserIdText={hasUserIdText}
          state={state}
          tinted={tinted}
        />
      </span>

      <span className='flex shrink-0 items-center gap-1'>
        {/*
          ★★ 空着 ID 时给**两颗**按钮（用户 2026-…）：
          「查看拥有权限的账号列表」+「编辑」。

          ⚠★ 「查看名单」**只在空着 ID 时出现**：填了 ID 之后右边那两枚是
          "开通 / 取消开通"（同一个位置只放得下两件事），而那时运营关心的是
          **这个人**的权限，不是"谁还有这一档"。
        */}
        {hasUserIdText ? null : (
          <Button
            variant='outline'
            size='xs'
            data-testid={`mode-holders-${row.id}`}
            aria-label={t('View the accounts that hold {{mode}}', { mode: row.label })}
            onClick={onViewHolders}
          >
            <Users className='mr-1 size-3' />
            {t('Accounts')}
          </Button>
        )}

        {hasUserIdText ? (
          <Button
            variant={open ? 'outline' : 'default'}
            size='xs'
            disabled={disabled}
            aria-label={actionLabel}
            /* ★ 屏幕阅读器也该知道"处在哪一边"（视觉上那一行已经反色了） */
            aria-pressed={open}
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
      </span>
    </div>
  )
}

/**
 * 那一行里的字：名字 + id + 媒介 + 可见性 + （状态 / 给了几个账号）。
 *
 * ⚠ 抽出来是因为两副面孔（空 ID / 填了 ID）**必须长得一模一样** ——
 * 各写一份的代价是"填了 ID 之后那一行悄悄变了个样"，而运营会以为换了一块东西。
 */
function ModeRowText({
  t,
  row,
  granted,
  hasUserIdText,
  state,
  tinted,
}: {
  t: TFunction
  row: ModeView
  granted: number
  hasUserIdText: boolean
  state: 'unknown' | 'open' | 'closed'
  /** ★ 这一行是反色的（已开通 / 状态读不到）—— 里面那些灰字与徽章要跟着换色。 */
  tinted: boolean
}) {
  const open = state === 'open'
  /*
   * ⚠★ 反色那一行上的**每一处**都要单独给色：
   * `text-muted-foreground` 是"浅灰"，而它压在深色底上几乎看不见；
   * `Badge` 的 `outline` / `secondary` 也各自带着自己的底色与边框色。
   * 少改一处的表现是"那一行上有一个字/一枚签读不出来"，而它**不会报错**。
   */
  const muted = tinted ? 'text-background/70' : 'text-muted-foreground'
  const badgeTone = tinted ? 'border-background/30 text-background' : ''
  /*
   * ⚠ `no-nested-ternary` 在本仓库是开的 —— 所以可见性那一枚签的 variant
   * 用一个**提前返回**的小函数算（串两层三元读着像谜题）。
   */
  const visibilityVariant = visibilityBadgeVariant(row, tinted)
  return (
    <>
      <span className='font-medium'>{row.label}</span>
      <span className={`font-mono text-xs ${muted}`}>{row.id}</span>
      <Badge variant='outline' className={badgeTone}>
        {row.medium}
      </Badge>
      <Badge variant={visibilityVariant} className={badgeTone}>
        {modeVisibilityLabel(t, row.visibility)}
      </Badge>
      {hasUserIdText ? (
        <Badge
          variant={tinted ? 'outline' : 'secondary'}
          data-testid={`mode-state-${row.id}`}
          className={`gap-1 ${badgeTone}`}
        >
          {open ? <ShieldCheck className='size-3' /> : <ShieldOff className='size-3' />}
          {accountModeStateLabel(t, state)}
        </Badge>
      ) : (
        <span className={`text-xs ${muted}`} data-testid={`mode-granted-${row.id}`}>
          {grantedLabel(t, { visibility: row.visibility, granted })}
        </span>
      )}
    </>
  )
}

/**
 * 可见性那一枚签用哪个 variant。
 *
 * ⚠ 单独一个函数是因为 `no-nested-ternary` 在本仓库是开的，而这里的判据是两层
 * （先看这一行反不反色，再看这一档是不是公共的）—— 提前返回比串三元好读。
 *
 * ⚠★ 反色那一行上它必须是 `outline`：`default` 是 `bg-primary`（一块彩色底），
 * 压在深色底上会变成"两块颜色打架"，而 `outline` 只要把边框与字调成背景色就统一了。
 */
function visibilityBadgeVariant(
  row: ModeView,
  tinted: boolean
): 'default' | 'secondary' | 'outline' {
  if (tinted) return 'outline'
  if (isPublicMode(row)) return 'default'
  return 'secondary'
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
