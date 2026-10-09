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
import { Loader2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { ModeView } from '../api'
import { modeVisibilityLabel } from '../lib/mode-admin-view'

/**
 * ★★ **改一档模式的分发策略**（用户 2026-…："模式管理 应该是可以编辑的，
 * 可以设置 权限是公开还是私有"）。
 *
 * # 它只开放两格，而且这是**刻意**的
 *
 * | 那一格 | 改它会怎样 |
 * |---|---|
 * | `x-visibility` | ★ 决定**谁能拿到它**：`public` = 所有账号（连没登录的都算）在广场上看得见、点一下就下到本机；`private` = 只有后台授权过的账号才看得见 |
 * | `x-summary` | 广场卡片上那句说明（给人看的一句话） |
 *
 * ⚠★ 模式的正文（`words` / `planDialog` / `libraries` / 规划指令…）**不在这一面改**：
 * 那是几千行结构化数据，给运营一个输入框去改它等于让他手写 JSON ——
 * 而"模式文件"本来就是**用编辑器或直接改文件**的东西（服务端那一份就是磁盘上的
 * 一个 `.json`）。这一面**不假装**能改（`api.ts` 的 `updateModeMeta` 注释同源）。
 *
 * # ⚠★ 可见性两个选项的**后果**必须画出来
 *
 * 运营看到的若只是 `public` / `private` 两个英文词，他会当成一个无关紧要的开关
 * —— 而这一格是"一份付费模式会不会变成人人可开通"的唯一一道闸（服务端的
 * `readVisibility` 默认就是 `private`，正是因为这个方向更安全）。
 * 所以下面那一行说明写的是**后果**，不是词义。
 */
export function ModeMetaDialog({
  mode,
  open,
  saving,
  error,
  onOpenChange,
  onSubmit,
}: {
  mode: ModeView | null
  open: boolean
  saving: boolean
  error: string
  onOpenChange: (next: boolean) => void
  onSubmit: (patch: { visibility: string; summary: string }) => void
}) {
  const { t } = useTranslation()

  /*
   * ⚠★ 草稿是**本地两格**（不是"直接改 mode"）：取消时什么都不该留下。
   *
   * ⚠★ 初值**直接从 props 取**，没有 `useEffect` 去同步 —— 因为 `react(set-state-in-effect)`
   * 在本仓库是开的（在 effect 里同步 setState 会多一轮渲染），而这里根本不需要
   * 那个 effect：调用方给了 `key={mode.id}`，**换一档就是换一个组件实例**
   * （状态重新初始化，旧的草稿自然丢掉）。那比"手动同步"少一处会忘的地方。
   */
  const [visibility, setVisibility] = useState(String(mode?.visibility ?? 'private'))
  const [summary, setSummary] = useState(String(mode?.summary ?? ''))

  /*
   * ★★ 那两个选项的字**跟着语言走**（用户 2026-… 报的"很多是英文"）：
   * 它们以前取自一个写死中文的常量表 —— 于是中文界面上正常、换任何语言都还是中文。
   * 现在两处（触发器上那一句、下拉里那两项）**同一个来源**，不会各说各的。
   */
  const visibilityOptions = useMemo(
    () => [
      { value: 'public', label: modeVisibilityLabel(t, 'public') },
      { value: 'private', label: modeVisibilityLabel(t, 'private') },
    ],
    [t]
  )

  if (!mode) return null

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Edit mode')}
      description={t(
        'Only how this mode is handed out can be changed here. The body of the mode (its wording, its planning instruction, its tables) stays in the file on the server — edit that file directly.'
      )}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={saving}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={saving}
            onClick={() => onSubmit({ visibility, summary })}
            data-testid='mode-meta-save'
          >
            {saving ? <Loader2 className='size-4 animate-spin' /> : null}
            {t('Save')}
          </Button>
        </>
      }
    >
      <div className='flex flex-wrap items-center gap-2'>
        <span className='font-medium'>{mode.label}</span>
        <span className='text-muted-foreground font-mono text-xs'>{mode.id}</span>
        <Badge variant={mode.visibility === 'public' ? 'default' : 'secondary'}>
          {modeVisibilityLabel(t, mode.visibility)}
        </Badge>
      </div>

      <div className='flex flex-col gap-2'>
        <Label htmlFor='mode-meta-visibility'>{t('Who can open it')}</Label>
        <Select
          value={visibility}
          /*
           * ⚠ `@base-ui/react` 的 `onValueChange` 给的是 `string | null`（清空时是 null）
           * —— 直接把它塞进 `useState<string>` 在那个类型上是错的。这里落回
           * `private`（**不是** `''`）：空值保存下去会被服务端拒（"可见性只能写…"），
           * 而 `private` 是服务端的默认、也是更安全的那个方向。
           */
          onValueChange={(next) => setVisibility(next ?? 'private')}
        >
          <SelectTrigger id='mode-meta-visibility' data-testid='mode-meta-visibility'>
            {/*
              ★★ 触发按钮上要显示**那一句话**（"私有 · 按账号开通"），不是 `private`。
              ⚠★ `SelectValue` 在没有把 `items` 交给 `Root` 时只会画出**原始值**
              （`@base-ui/react` 的行为；那会让运营看见一个英文词，而这一格
              正是"一份付费模式会不会变成人人可开通"的那道闸 —— 见本文件头）。
              所以这里**自己给那一句话**，取值只有一处（`visibilityOptions`）。
            */}
            <SelectValue>
              {visibilityOptions.find((option) => option.value === visibility)?.label ||
                visibility}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            {visibilityOptions.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {/*
          ★ 两句**后果**（不是词义）—— 运营要据此判断该选哪一个。
        */}
        <p className='text-muted-foreground text-xs' data-testid='mode-meta-visibility-hint'>
          {visibility === 'public'
            ? t(
                'Public: every account sees it in the plaza and can open it with one click — including accounts that are not signed in.'
              )
            : t(
                'Private: only the accounts you grant it to below can see it in the plaza at all. It does not appear greyed out for anyone else.'
              )}
        </p>
      </div>

      <div className='flex flex-col gap-2'>
        <Label htmlFor='mode-meta-summary'>{t('One-line summary')}</Label>
        <Input
          id='mode-meta-summary'
          value={summary}
          onChange={(event) => setSummary(event.target.value)}
          placeholder={t('Shown on the plaza card.')}
        />
        <p className='text-muted-foreground text-xs'>
          {t('Leave it empty to show no summary on the card.')}
        </p>
      </div>

      {error ? (
        <p className='text-destructive text-sm' data-testid='mode-meta-error'>
          {error}
        </p>
      ) : null}
    </Dialog>
  )
}
