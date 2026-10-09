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
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import { listEntitlementsByMode, type ModeEntitlementByMode, type ModeView } from '../api'
import {
  entitlementState,
  entitlementStateLabel,
  isPublicMode,
  modeVisibilityLabel,
} from '../lib/mode-admin-view'

/**
 * 「**谁有这一档的权限**」—— 那一颗按钮点开就是它（用户 2026-… 点名要的）。
 *
 * 用户的原话：
 *
 * > "当没有输入用户ID的时候，除了编辑之外，再加上一个 **点击查看拥有权限的账号列表** 的按钮，
 * >  点击出来对应的弹窗可以显示。"
 *
 * # ⚠★ 与服务端那条"不许全表列举"的纪律的关系
 *
 * 这个名单是**按一档模式**问的（`?mode_id=`），不是"把所有授权都拉出来" ——
 * 服务端 `listAdminEntitlements` 明确拒绝了后者（授权表会随账号数线性长大）。
 * 所以这里一次只问一档，而且只在**用户点开**的那一刻问。
 *
 * # ⚠★ 三件事都不许画错（每一件都有一种具体的坏法）
 *
 * | 那一格 | 画错了会怎样 |
 * |---|---|
 * | 读不到 | 画成"还没有人"→ ★ 运营以为这一档没人有权限，**照着它去再授权一遍**（服务端会多插一行，看不出错） |
 * | 空名单（私有） | 画成"暂无"→ 把"我忘了给谁开"这件**要动手的事**说成一件平常事 |
 * | 空名单（公共） | 画成"还没有人"→ ★ 公共模式本来就"谁都能开通"，说"没有人有权限"是**假话** |
 *
 * 所以三句话分开：读不到说读不到、私有说"还没有账号被开通"、公共说"谁都能开通，所以没有名单"。
 *
 * # ⚠★ 数据"属不属于这一档"由 `holdersOf` 记着，不在 effect 里清空
 *
 * `react(set-state-in-effect)` 在本仓库是开的：在 effect 里同步 `setState(null)`
 * 会多一轮渲染。所以读取失败 / 换一档时**不清空**，而是把"这份数据属于哪一档"记下来
 * （`holdersOf`），由读取方把"过期的数据"当成"还没有数据" —— 与页面上那两处同源。
 */
export function ModeHoldersDialog({
  mode,
  open,
  onOpenChange,
  onGrantedCount,
}: {
  /** 要看的那一档（null = 没打开）。 */
  mode: ModeView | null
  open: boolean
  onOpenChange: (next: boolean) => void
  /** ★ 读完这一档的人数之后回填给列表那一行（"已给 N 个账号开通"就会跟着变）。 */
  onGrantedCount?: (modeId: string, count: number) => void
}) {
  const { t } = useTranslation()

  const modeId = mode?.id || ''
  const [data, setData] = useState<ModeEntitlementByMode | null>(null)
  const [error, setError] = useState('')
  /** 手上这份属于哪一档；空串 = 没有任何一份数据算数。 */
  const [of, setOf] = useState('')
  /** ★ 手动重读中（它只让按钮转圈，**不**把列表换回骨架 —— 那会闪一下）。 */
  const [refreshing, setRefreshing] = useState(false)

  const isStale = of !== modeId
  const current = isStale ? null : data
  const currentError = isStale ? '' : error
  const rows = current?.items || []

  /**
   * 读这一档的名单。
   *
   * ⚠ 它是**普通函数**（不是 memo 的回调）：两条路（effect 与「重新读取」按钮）都要调，
   * 包成 memo 只会让 `react(preserve-manual-memoization)` 有意见，收益为零。
   */
  async function loadHolders(id: string, manual = false) {
    if (!id) return
    if (manual) setRefreshing(true)
    try {
      const result = await listEntitlementsByMode(id)
      setData(result)
      setError('')
      setOf(id)
      /* ★ 生效中的那几个才是"有权限的账号"——人数回填给列表那一行 */
      onGrantedCount?.(id, (result.items || []).length)
    } catch (err) {
      setData(null)
      setError(readError(err, t('Could not read who holds this mode')))
      setOf(id)
    } finally {
      if (manual) setRefreshing(false)
    }
  }

  useEffect(() => {
    /*
     * ⚠ 只在**打开**且**换了一档**时读一次。
     * ⚠ 依赖是 `modeId`（字符串）而不是 `mode` 那个对象：对象引用每次渲染都会变，
     * 写进去会让这个 effect 每一轮都跑一次。
     */
    if (open && modeId) void loadHolders(modeId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, modeId])

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Who holds “{{mode}}”', { mode: mode?.label || modeId })}
      description={
        mode
          ? `${modeId} · ${modeVisibilityLabel(t, mode.visibility)}`
          : t('No mode selected.')
      }
      contentHeight='auto'
      bodyClassName='space-y-2'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={refreshing}
            onClick={() => void loadHolders(modeId, true)}
          >
            <RefreshCw className='mr-1 h-4 w-4' />
            {t('Reload')}
          </Button>
          <Button type='button' onClick={() => onOpenChange(false)}>
            {t('Close')}
          </Button>
        </>
      }
    >
      {/*
        ⚠★ 读不到就说读不到（**不画成"还没有人"**）：那个空名单是一个**结论**
        （"这一档没人有权限"），而运营会照着它去重复授权。
      */}
      {currentError ? (
        <Alert variant='destructive' data-testid='mode-holders-error'>
          <AlertDescription>{currentError}</AlertDescription>
        </Alert>
      ) : null}

      {/* 还没读到就画骨架，不画"还没有人" */}
      {!currentError && !current ? (
        <p className='text-muted-foreground text-sm'>{t('Loading…')}</p>
      ) : null}

      {current && rows.length ? (
        <ul className='flex flex-col divide-y rounded-md border' data-testid='mode-holders-list'>
          {rows.map((row) => (
            <li key={row.id} className='flex items-center justify-between gap-2 px-3 py-2'>
              <span className='flex flex-wrap items-center gap-2'>
                <span className='font-mono text-sm'>{row.userId}</span>
                <Badge variant='secondary'>
                  {entitlementStateLabel(t, entitlementState(row, Date.now() / 1000))}
                </Badge>
              </span>
              <span className='text-muted-foreground text-xs'>{row.source}</span>
            </li>
          ))}
        </ul>
      ) : null}

      {/*
        ★★ 空名单**不是同一句话**，取决于这一档是公共的还是私有的 ——
        两句合成一句会把"我忘了给谁开"说成一件平常事，或对公共模式说一句假话。
      */}
      {current && !rows.length ? (
        <p className='text-muted-foreground text-sm' data-testid='mode-no-holders'>
          {isPublicMode(mode || { visibility: '' })
            ? t('This mode is public — every account can open it, so there is nobody to list.')
            : t('No account holds this mode yet.')}
        </p>
      ) : null}
    </Dialog>
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
