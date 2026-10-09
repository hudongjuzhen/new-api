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
import { Loader2, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Textarea } from '@/components/ui/textarea'

import { createMode, listModeAiKeys, type ModeAiKey, type ModeView } from '../api'
import { modeVisibilityLabel } from '../lib/mode-admin-view'

/** 这一份模式属于哪一家（与客户端 `MODE_MEDIUM_IDS` 同一个白名单）。 */
const MEDIUMS = ['video', 'audio', 'text'] as const

/**
 * ★★ **添加模式**（用户 2026-… 点名要的那一颗按钮）
 *
 * 用户的原话：
 *
 * > "右上角增加一个添加模式的功能，点击添加模式，可以输入模式名称，模式类型，模式简介，
 * >  然后能够 AI一键生成，选择一个API密钥，然后调用 glm-5.3-flash 这个模型生成"
 *
 * 所以这一屏就是**四格输入 + 一颗生成按钮**：
 *
 *	模式名称 / 模式类型 / 模式简介 / （给模型的需求）
 *	+ 选择 API 密钥            ← 这一趟请求用它走站内中继（花的也是它的额度）
 *	+ [AI 一键生成]            ← 生成 + 落盘，一次做完
 *
 * # ⚠★ 为什么"生成"与"建模式"是**同一个动作**
 *
 * 用户说的是"AI 一键生成"—— 他的期望是点一下就**有一档新模式**。
 * 拆成"先生成看看、再点一次保存"多一步，而多出来的那一步要显示一份几万字的 JSON
 * 让人过目（没有人会读它）。所以这里一次做完：生成 → 服务端校验 → 落盘 →
 * 关弹窗、列表里多一档 —— 而**改它**走的是每一行上那颗「编辑」。
 *
 * # ⚠★ 密钥那一格给的是**本站的令牌**，不是"再填一把上游密钥"
 *
 * 见服务端 `ai_generate.go` 文件头那段：不新增第二套密钥存法、运营看得见花了多少、
 * 换模型换渠道不用改代码。⚠ 而这一格**必填**：没有它这一趟请求没有身份，
 * 也就落不到任何渠道上。
 */
export function ModeCreateDialog({
  open,
  onOpenChange,
  onCreated,
}: {
  open: boolean
  onOpenChange: (next: boolean) => void
  onCreated?: (mode: ModeView) => void
}) {
  const { t } = useTranslation()

  const [label, setLabel] = useState('')
  const [medium, setMedium] = useState<string>('video')
  const [summary, setSummary] = useState('')
  const [request, setRequest] = useState('')
  const [visibility, setVisibility] = useState('private')
  const [tokenId, setTokenId] = useState('')

  const [keys, setKeys] = useState<ModeAiKey[]>([])
  const [keysError, setKeysError] = useState('')
  /**
   * 服务端告诉界面"这一趟用哪个模型"（它由 `ZSY_MODE_AI_MODEL` 决定，默认
   * `glm-5.3-flash`）。
   *
   * ⚠★ 它**不是**给运营改的 —— 模型名由渠道配置决定，界面上再放一格只会多一个
   * 会与渠道分叉的地方。这里只把它**念出来**（"这一趟走的是 glm-5.3-flash"），
   * 因为运营一定会问"它到底调了哪个模型"。
   */
  const [model, setModel] = useState('')
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')

  const mediumOptions = useMemo(() => {
    const labels: Record<string, string> = {
      video: t('Video'),
      audio: t('Audio'),
      text: t('Text'),
    }
    return MEDIUMS.map((value) => ({ value, label: labels[value] || value }))
  }, [t])

  /** 打开时读一次密钥列表（那是这一刻的账号状态，不该跟着页面加载去读）。 */
  useEffect(() => {
    if (!open) return
    let cancelled = false
    setKeysError('')
    void listModeAiKeys()
      .then((setup) => {
        if (cancelled) return
        setKeys(setup.items || [])
        setModel(setup.model || '')
        /* ★ 默认选第一个**能用的**：默认选一个被禁用的会让运营一进来就撞一次墙 */
        const first = (setup.items || []).find((row) => row.usable)
        setTokenId((prev) => prev || (first ? String(first.id) : ''))
      })
      .catch((err) => {
        if (cancelled) return
        setKeys([])
        setKeysError(readError(err, t('Could not read your API keys')))
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const canGenerate =
    !busy && label.trim().length > 0 && !!tokenId && keys.some((row) => String(row.id) === tokenId && row.usable)

  async function generate() {
    if (!canGenerate) return
    setBusy(true)
    setError('')
    try {
      const created = await createMode({
        label: label.trim(),
        medium,
        summary: summary.trim(),
        request: request.trim(),
        visibility,
        token_id: Number(tokenId),
      })
      onCreated?.(created)
      /* ⚠ 成功之后**清空**：不然下一次打开还留着上一次那几个字，而运营会以为它没生成 */
      setLabel('')
      setSummary('')
      setRequest('')
      onOpenChange(false)
    } catch (err) {
      setError(readError(err, t('Could not generate this mode')))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Add a mode')}
      description={t(
        'Describe the kind of work this mode is for, pick one of your API keys, and the model writes the whole mode file. You can refine it afterwards with Edit.'
      )}
      contentClassName='sm:max-w-2xl'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => onOpenChange(false)}
            disabled={busy}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={!canGenerate}
            onClick={() => void generate()}
            data-testid='mode-create-generate'
          >
            {busy ? (
              <Loader2 className='mr-1 size-4 animate-spin' />
            ) : (
              <Sparkles className='mr-1 size-4' />
            )}
            {busy ? t('Generating…') : t('Generate with AI')}
          </Button>
        </>
      }
    >
      <div className='grid gap-3 sm:grid-cols-3'>
        <div className='flex flex-col gap-2 sm:col-span-2'>
          <Label htmlFor='mode-create-label'>{t('Name')}</Label>
          <Input
            id='mode-create-label'
            value={label}
            disabled={busy}
            data-testid='mode-create-label'
            placeholder={t('e.g. Food tour shorts')}
            onChange={(event) => setLabel(event.target.value)}
          />
        </div>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='mode-create-medium'>{t('Type')}</Label>
          <Select value={medium} onValueChange={(next) => setMedium(next ?? 'video')}>
            <SelectTrigger id='mode-create-medium' data-testid='mode-create-medium'>
              <SelectValue>
                <span>
                  {mediumOptions.find((option) => option.value === medium)?.label || medium}
                </span>
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {mediumOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      </div>

      <div className='flex flex-col gap-2'>
        <Label htmlFor='mode-create-summary'>{t('One-line summary')}</Label>
        <Input
          id='mode-create-summary'
          value={summary}
          disabled={busy}
          data-testid='mode-create-summary'
          placeholder={t('Shown on the plaza card.')}
          onChange={(event) => setSummary(event.target.value)}
        />
      </div>

      <div className='flex flex-col gap-2'>
        <Label htmlFor='mode-create-request'>{t('What is this mode for?')}</Label>
        <Textarea
          id='mode-create-request'
          className='min-h-24'
          value={request}
          disabled={busy}
          data-testid='mode-create-request'
          placeholder={t(
            'Describe the work: what it is for, who it is for, and anything the model must get right. The more concrete this is, the better the generated mode fits.'
          )}
          onChange={(event) => setRequest(event.target.value)}
        />
      </div>

      <div className='grid gap-3 sm:grid-cols-2'>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='mode-create-key'>{t('API key')}</Label>
          <Select value={tokenId} onValueChange={(next) => setTokenId(next ?? '')}>
            <SelectTrigger id='mode-create-key' data-testid='mode-create-key'>
              <SelectValue>
                <span>
                  {keys.find((row) => String(row.id) === tokenId)?.name || t('Choose a key')}
                </span>
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              {keys.map((row) => (
                <SelectItem key={row.id} value={String(row.id)} disabled={!row.usable}>
                  {row.usable ? row.name : `${row.name} — ${row.problem}`}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className='text-muted-foreground text-xs'>
            {model
              ? t('This request goes through your own relay with model {{model}}.', { model })
              : t('This request goes through your own relay.')}
          </p>
        </div>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='mode-create-visibility'>{t('Who can open it')}</Label>
          <Select value={visibility} onValueChange={(next) => setVisibility(next ?? 'private')}>
            <SelectTrigger id='mode-create-visibility' data-testid='mode-create-visibility'>
              <SelectValue>
                <span>{modeVisibilityLabel(t, visibility)}</span>
              </SelectValue>
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='private'>{modeVisibilityLabel(t, 'private')}</SelectItem>
              <SelectItem value='public'>{modeVisibilityLabel(t, 'public')}</SelectItem>
            </SelectContent>
          </Select>
          {/* ⚠ 默认私有：与服务端那条"忘写这一格按 private 算"同一个方向 */}
          <p className='text-muted-foreground text-xs'>
            {t('New modes start private — open them per account, or make them public here.')}
          </p>
        </div>
      </div>

      {/* ⚠ 读不到密钥列表时说清楚（不然运营会对着一个空下拉发呆） */}
      {keysError ? (
        <Alert variant='destructive' data-testid='mode-keys-error'>
          <AlertDescription>{keysError}</AlertDescription>
        </Alert>
      ) : null}

      {/* ⚠ 生成失败时把服务端那句话**原样**摆出来（哪一格不对、该去哪儿改都在里面） */}
      {error ? (
        <Alert variant='destructive' data-testid='mode-create-error'>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
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
