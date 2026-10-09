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
import { useEffect, useMemo, useState } from 'react'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { Textarea } from '@/components/ui/textarea'

import { getModeContent, saveModeContent, updateModeMeta, type ModeView } from '../api'
import { modeVisibilityLabel } from '../lib/mode-admin-view'
import { ModeContentEditor } from './mode-content-editor'

/**
 * 编辑一档模式（`zsy/mode` 后台那一屏的弹窗）。
 *
 * # ★★ 这一版的形状是用户 2026-… 点名要的
 *
 * 用户的原话：
 *
 * > "再在编辑弹窗里，现有的这个**开通权限和一句话说明放在最左侧**，
 * >  整个弹窗变的要更大，里面有这个**模式的具体内容编辑**，根据 json 中的内容做编辑"
 *
 * 于是它是一张**宽弹窗**（`sm:max-w-5xl`，而不是默认那个 `sm:max-w-2xl`），
 * 里面分左右两栏：
 *
 *	┌─ 左（窄）─┬─ 右（宽，可滚动）──────────────────────┐
 *	│ 谁能开通   │  这个模式（名称 / 类型 / 连载）        │
 *	│ 一句话说明 │  必填项（四个开关）                    │
 *	│ [保存]     │  界面用词（二十几格）                  │
 *	│            │  资料库表（id + 名字）                 │
 *	│            │  ── 或者 ──「原始 JSON」页签 ──       │
 *	└────────────┴───────────────────────────────────────┘
 *
 * # ⚠★ 保存分**两条路**（而这不是省事，是两个动作的代价不同）
 *
 * | 改了哪儿 | 走哪条接口 | 服务端怎么保存 |
 * |---|---|---|
 * | 只有左边那两格（可见性 / 说明） | `POST …/:id/meta` | **逐键搬运**，改完只差那两行 |
 * | 碰过右边（表单或 JSON） | `POST …/:id/content` | 校验 + 原子替换 + 留 `.bak` |
 *
 * 合成一条的代价是**每次改一个说明都要整篇重写那份文件** —— 而"服务端那一份 =
 * 客户端那一份、一眼可验"是这套方案的全部立足点（见 `catalog.go` 文件头）。
 *
 * # ⚠★ 两份草稿不许各说各的
 *
 * 表单与 JSON 是**同一份数据的两种画法**，所以它们是**同一个 state**：
 * 切页签不丢改动、也不各存一份。JSON 页签解析失败时**不许保存**
 * （一份坏 JSON 存下去就是"这一档客户端读不出来"），而那句话要说清错在第几行。
 */
export function ModeEditDialog({
  mode,
  open,
  onOpenChange,
  onSaved,
}: {
  mode: ModeView | null
  open: boolean
  onOpenChange: (next: boolean) => void
  /** 保存成功之后回给页面（页面据此重读列表 —— 服务端那份才是权威）。 */
  onSaved?: () => void
}) {
  const { t } = useTranslation()

  const modeId = mode?.id || ''
  /** 左边那两格（分发策略）—— 它们是**分开**的两格，不在正文那一份草稿里。 */
  const [visibility, setVisibility] = useState(String(mode?.visibility ?? 'private'))
  const [summary, setSummary] = useState(String(mode?.summary ?? ''))

  /** 正文那一份草稿（表单与 JSON 共用的**同一个** state）。 */
  const [draft, setDraft] = useState<Record<string, unknown> | null>(null)
  const [loadError, setLoadError] = useState('')
  const [saveError, setSaveError] = useState('')
  const [saving, setSaving] = useState(false)
  const [tab, setTab] = useState('form')

  /** JSON 页签那一格**自己**的文本（用户正在敲的东西，可能暂时还不是合法 JSON）。 */
  const [jsonText, setJsonText] = useState('')

  const visibilityOptions = useMemo(
    () => [
      { value: 'public', label: modeVisibilityLabel(t, 'public') },
      { value: 'private', label: modeVisibilityLabel(t, 'private') },
    ],
    [t]
  )

  /**
   * 打开时读一次正文。
   *
   * ⚠ 依赖是 `modeId`（字符串）而不是 `mode` 那个对象：对象引用每次渲染都会变，
   * 写进去会让这个 effect 每一轮都跑一次（而它要发请求）。
   */
  useEffect(() => {
    if (!open || !modeId) return
    let cancelled = false
    setLoadError('')
    void getModeContent(modeId)
      .then((content) => {
        if (cancelled) return
        setDraft(content)
        setJsonText(JSON.stringify(content, null, 2))
      })
      .catch((err) => {
        if (cancelled) return
        setDraft(null)
        setJsonText('')
        setLoadError(readError(err, t('Could not read this mode’s content')))
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, modeId])

  if (!mode) return null

  /** 切到 JSON 页签：把当前草稿渲染成文本（**每次切换都重渲染**，不丢表单的改动）。 */
  function onTabChange(next: string) {
    if (next === 'json' && draft) setJsonText(JSON.stringify(draft, null, 2))
    setTab(next)
  }

  /** JSON 页签里敲字：当场解析，合法就同步进草稿（不合法就只留着文本）。 */
  function onJsonChange(text: string) {
    setJsonText(text)
    setSaveError('')
    const parsed = parseJsonObject(text)
    if (parsed.ok) setDraft(parsed.value)
  }

  const jsonProblem = parseJsonObject(jsonText)

  async function save() {
    if (!modeId) return
    /* ⚠★ JSON 坏着的时候**不许保存**：一份坏 JSON 存下去就是"这一档客户端读不出来" */
    if (tab === 'json' && !jsonProblem.ok) {
      setSaveError(t('The JSON is not valid yet: {{message}}', { message: jsonProblem.message }))
      return
    }
    setSaving(true)
    setSaveError('')
    try {
      /*
       * ⚠ 先存**分发策略**（它走的是"逐键搬运"那条路，改完只差两行），
       * 再存正文（那条路要整篇重写）。次序是刻意的：
       * 正文那一步失败时，可见性那一次已经生效了 —— 而它是**独立的一格**，
       * 运营再点一次保存就能把正文补上（反过来则会把一整份正文白写一遍）。
       */
      await updateModeMeta(modeId, { visibility, summary })
      if (draft) await saveModeContent(modeId, draft)
      onSaved?.()
      onOpenChange(false)
    } catch (err) {
      setSaveError(readError(err, t('Could not save')))
    } finally {
      setSaving(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Edit mode')}
      description={t(
        'Left: who can open it and the one-line summary. Right: the mode itself — the words the interface uses, the tables it collects, and the raw JSON.'
      )}
      /* ★★ 宽弹窗（`.Dialog` 默认是 `sm:max-w-2xl`）：这一屏的右边要放二十几格表单 */
      contentClassName='sm:max-w-5xl'
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
            disabled={saving || !!loadError}
            onClick={() => void save()}
            data-testid='mode-meta-save'
          >
            {saving ? <Loader2 className='mr-1 size-4 animate-spin' /> : null}
            {t('Save')}
          </Button>
        </>
      }
    >
      <div className='flex flex-col gap-4 lg:flex-row lg:items-start'>
        {/* ───────────────────── 左：分发策略（最左侧，用户点名要的位置） ───────────────────── */}
        <div className='flex shrink-0 flex-col gap-4 lg:w-64'>
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
              onValueChange={(next) => setVisibility(next ?? 'private')}
            >
              <SelectTrigger id='mode-meta-visibility' data-testid='mode-meta-visibility'>
                <SelectValue>
                  <span>
                    {visibilityOptions.find((option) => option.value === visibility)?.label ||
                      visibility}
                  </span>
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
            <p className='text-muted-foreground text-xs' data-testid='mode-meta-visibility-hint'>
              {visibility === 'public'
                ? t(
                    'Public: every account sees it in the plaza and can open it with one click — including accounts that are not signed in.'
                  )
                : t(
                    'Private: only the accounts you grant it to can see it in the plaza at all. It does not appear greyed out for anyone else.'
                  )}
            </p>
          </div>

          <div className='flex flex-col gap-2'>
            <Label htmlFor='mode-meta-summary'>{t('One-line summary')}</Label>
            <Input
              id='mode-meta-summary'
              value={summary}
              disabled={saving}
              onChange={(event) => setSummary(event.target.value)}
              placeholder={t('Shown on the plaza card.')}
            />
            <p className='text-muted-foreground text-xs'>
              {t('Leave it empty to show no summary on the card.')}
            </p>
          </div>
        </div>

        {/* ───────────────────── 右：模式本身（表单 / 原始 JSON） ───────────────────── */}
        <div className='flex min-w-0 flex-1 flex-col gap-3'>
          {loadError ? (
            <Alert variant='destructive' data-testid='mode-content-error'>
              <AlertDescription>{loadError}</AlertDescription>
            </Alert>
          ) : null}

          {!draft && !loadError ? (
            <p className='text-muted-foreground text-sm'>{t('Loading…')}</p>
          ) : null}

          {draft ? (
            <Tabs value={tab} onValueChange={onTabChange}>
              <TabsList className='h-7 p-0.5'>
                <TabsTrigger className='h-6 px-2.5 text-xs' value='form' data-testid='mode-tab-form'>
                  {t('Form')}
                </TabsTrigger>
                <TabsTrigger className='h-6 px-2.5 text-xs' value='json' data-testid='mode-tab-json'>
                  {t('Raw JSON')}
                </TabsTrigger>
              </TabsList>

              <TabsContent value='form' className='min-h-0'>
                <ModeContentEditor
                  t={t}
                  draft={draft}
                  disabled={saving}
                  onChange={(next) => setDraft(next)}
                />
              </TabsContent>

              <TabsContent value='json' className='min-h-0'>
                <div className='flex flex-col gap-2'>
                  <Textarea
                    className='min-h-[24rem] font-mono text-xs'
                    data-testid='mode-json-editor'
                    spellCheck={false}
                    value={jsonText}
                    disabled={saving}
                    onChange={(event) => onJsonChange(event.target.value)}
                  />
                  {jsonProblem.ok ? (
                    <p className='text-muted-foreground text-xs'>
                      {t(
                        'This is the whole file. Editing it and saving writes every field, including the ones the form above does not show.'
                      )}
                    </p>
                  ) : (
                    <p className='text-destructive text-xs' data-testid='mode-json-problem'>
                      {t('The JSON is not valid yet: {{message}}', {
                        message: jsonProblem.message,
                      })}
                    </p>
                  )}
                </div>
              </TabsContent>
            </Tabs>
          ) : null}

          {saveError ? (
            <Alert variant='destructive' data-testid='mode-meta-error'>
              <AlertDescription>{saveError}</AlertDescription>
            </Alert>
          ) : null}
        </div>
      </div>
    </Dialog>
  )
}

/** Parses one JSON object out of a text area's content. */
function parseJsonObject(text: string): { ok: true; value: Record<string, unknown> } | { ok: false; message: string } {
  const trimmed = text.trim()
  if (!trimmed) return { ok: false, message: 'empty' }
  try {
    const parsed: unknown = JSON.parse(trimmed)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return { ok: false, message: 'not an object' }
    }
    return { ok: true, value: parsed as Record<string, unknown> }
  } catch (err) {
    return { ok: false, message: err instanceof Error ? err.message : String(err) }
  }
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
