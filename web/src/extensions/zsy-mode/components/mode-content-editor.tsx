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
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

import type { Translate } from '../lib/mode-admin-view'

/**
 * 一份模式正文的**结构化编辑**（用户 2026-… 选的"表单为主 + 原始 JSON 页签"）。
 *
 * # ★★ 它只覆盖**高频字段**，其余交给原始 JSON 页签
 *
 * 用户的选择是"先做高频字段"：
 *
 *	label / medium / workScale / 一句话说明 / 可见性   ← 运营天天要动的
 *	words 那一节（界面上那些用词）                     ← 「这一类作品该叫什么」全在这一节
 *	libraries 的 id + label                            ← 资料库那几张表叫什么
 *
 * ⚠★ 剩下那些（`planDialog.instruction` 二十几段指令、`templates`、`audio`、
 * `extraButtons`、`appRows`…）**不在这里编** —— 它们是几千字的结构化正文，
 * 给运营一个输入框去改等于让他手写指令。那不是"没做完"，是**刻意**：
 * 原始 JSON 页签里它们一个都在，而且保存前服务端会跑一遍与读盘同源的校验。
 *
 * # ⚠ 它是**受控**的：草稿在调用方手上
 *
 * `draft` 与 `onDraftChange` 都由外面拿着 —— 因为"保存"要对**两份**草稿做决定
 * （表单这一份与原始 JSON 那一份），而状态放在这里的话调用方就拿不到另一份了。
 */
export function ModeContentEditor({
  t,
  draft,
  onChange,
  disabled,
}: {
  t: Translate
  /** 模式正文（一整份 JSON 对象，含 `x-visibility` / `x-summary`）。 */
  draft: Record<string, unknown>
  onChange: (next: Record<string, unknown>) => void
  disabled?: boolean
}) {
  const words = asObject(draft.words)
  const libraries = Array.isArray(draft.libraries) ? draft.libraries : []
  const requires = asObject(draft.requires)

  /** 改一格（`patch` 是浅合并，调用方拿到的是新对象 —— 不许就地改）。 */
  function patch(next: Record<string, unknown>) {
    onChange({ ...draft, ...next })
  }

  /** 改 `words` 里的一格（**嵌套那一层也照做**：`planSelfCheck` 是对象）。 */
  function patchWord(key: string, value: string) {
    patch({ words: { ...words, [key]: value } })
  }

  /** 改 `planSelfCheck` 里的一格。 */
  function patchSelfCheck(key: string, value: string) {
    const selfCheck = asObject(words.planSelfCheck)
    patch({ words: { ...words, planSelfCheck: { ...selfCheck, [key]: value } } })
  }

  /** 改第 N 张资料库的名字（`id` 与 `label` 两格）。 */
  function patchLibrary(index: number, key: 'id' | 'label', value: string) {
    const next = libraries.map((row, i) => {
      if (i !== index) return row
      if (!isPlainObject(row)) return row
      return { ...row, [key]: value }
    })
    patch({ libraries: next })
  }

  /** 改一项必填开关。 */
  function patchRequire(key: string, value: boolean) {
    patch({ requires: { ...requires, [key]: value } })
  }

  const wordKeys = Object.keys(words).filter((key) => key !== 'planSelfCheck')
  const selfCheck = asObject(words.planSelfCheck)
  const selfCheckKeys = Object.keys(selfCheck)

  return (
    <div className='flex flex-col gap-5'>
      <Alert>
        <AlertDescription>
          {t(
            'These fields are the ones operators change most. Everything else (the planning instruction, the templates, the extra buttons…) is edited on the JSON tab — the server checks the whole file again before it is saved.'
          )}
        </AlertDescription>
      </Alert>

      {/* ────────────────────── 一、This mode's own identity ────────────────────── */}
      <section className='flex flex-col gap-3' data-testid='mode-editor-identity'>
        <h4 className='text-sm font-medium'>{t('This mode')}</h4>
        <div className='grid gap-3 sm:grid-cols-3'>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='mode-content-label'>{t('Name')}</Label>
            <Input
              id='mode-content-label'
              value={String(draft.label ?? '')}
              disabled={disabled}
              onChange={(event) => patch({ label: event.target.value })}
            />
          </div>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='mode-content-medium'>{t('Type')}</Label>
            <Input
              id='mode-content-medium'
              value={String(draft.medium ?? '')}
              disabled={disabled}
              onChange={(event) => patch({ medium: event.target.value.trim() })}
            />
            <p className='text-muted-foreground text-xs'>
              {t('One of: {{values}}', { values: 'video / audio / text' })}
            </p>
          </div>
          <div className='flex flex-col gap-2'>
            <Label htmlFor='mode-content-scale'>{t('Work scale')}</Label>
            <Input
              id='mode-content-scale'
              value={String(draft.workScale ?? '')}
              disabled={disabled}
              onChange={(event) => patch({ workScale: event.target.value.trim() })}
            />
            <p className='text-muted-foreground text-xs'>
              {t('One of: {{values}}', { values: 'single / serial' })}
            </p>
          </div>
        </div>
      </section>

      {/* ────────────────────── 二、Which parts of it are required ────────────────────── */}
      <section className='flex flex-col gap-3' data-testid='mode-editor-requires'>
        <h4 className='text-sm font-medium'>{t('What this mode requires')}</h4>
        <p className='text-muted-foreground text-xs'>
          {t(
            'These decide which inputs the planning dialog insists on. Audio and segments are usually required; lyrics and references usually are not.'
          )}
        </p>
        <div className='flex flex-wrap gap-x-6 gap-y-2'>
          {['audio', 'lyrics', 'refs', 'segments'].map((key) => (
            <label key={key} className='flex items-center gap-2 text-sm'>
              <input
                type='checkbox'
                className='size-4'
                disabled={disabled}
                data-testid={`mode-requires-${key}`}
                checked={requires[key] === true}
                onChange={(event) => patchRequire(key, event.target.checked)}
              />
              <span className='font-mono text-xs'>{key}</span>
            </label>
          ))}
        </div>
      </section>

      {/* ────────────────────── 三、The words the UI uses ────────────────────── */}
      <section className='flex flex-col gap-3' data-testid='mode-editor-words'>
        <h4 className='text-sm font-medium'>
          {t('The words the interface uses')}
          <span className='text-muted-foreground ml-2 text-xs font-normal'>
            {t('{{count}} fields', { count: wordKeys.length + selfCheckKeys.length })}
          </span>
        </h4>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Every one of them decides a label the user reads. An empty field falls back to what the app ships with, so leaving one blank is safe.'
          )}
        </p>
        <div className='grid gap-2 lg:grid-cols-2'>
          {wordKeys.map((key) => (
            <div key={key} className='flex items-center gap-2'>
              <span className='text-muted-foreground w-40 shrink-0 truncate font-mono text-xs' title={key}>
                {key}
              </span>
              <Input
                className='h-8'
                disabled={disabled}
                data-testid={`mode-word-${key}`}
                value={String(words[key] ?? '')}
                onChange={(event) => patchWord(key, event.target.value)}
              />
            </div>
          ))}
        </div>
        {selfCheckKeys.length ? (
          <div className='flex flex-col gap-2'>
            <p className='text-muted-foreground text-xs'>
              {t('The pre-flight checklist shown in the planning dialog:')}
            </p>
            <div className='grid gap-2 lg:grid-cols-2'>
              {selfCheckKeys.map((key) => (
                <div key={key} className='flex items-center gap-2'>
                  <span
                    className='text-muted-foreground w-40 shrink-0 truncate font-mono text-xs'
                    title={`planSelfCheck.${key}`}
                  >
                    {key}
                  </span>
                  <Input
                    className='h-8'
                    disabled={disabled}
                    data-testid={`mode-selfcheck-${key}`}
                    value={String(selfCheck[key] ?? '')}
                    onChange={(event) => patchSelfCheck(key, event.target.value)}
                  />
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </section>

      {/* ────────────────────── 四、The library tables ────────────────────── */}
      <section className='flex flex-col gap-3' data-testid='mode-editor-libraries'>
        <h4 className='text-sm font-medium'>
          {t('Library tables')}
          <span className='text-muted-foreground ml-2 text-xs font-normal'>
            {t('{{count}} tables', { count: libraries.length })}
          </span>
        </h4>
        <p className='text-muted-foreground text-xs'>
          {t(
            'Each table is one kind of thing this mode collects (characters, locations, products…). Only the id and the label are editable here; the fields of a table are on the JSON tab.'
          )}
        </p>
        <div className='flex flex-col gap-2'>
          {libraries.map((row, index) => {
            const item = asObject(row)
            return (
              <div key={String(item.id ?? index)} className='flex flex-wrap items-center gap-2'>
                <Input
                  className='h-8 w-40'
                  disabled={disabled}
                  data-testid={`mode-library-id-${index}`}
                  value={String(item.id ?? '')}
                  onChange={(event) => patchLibrary(index, 'id', event.target.value.trim())}
                />
                <Input
                  className='h-8 w-56'
                  disabled={disabled}
                  data-testid={`mode-library-label-${index}`}
                  value={String(item.label ?? '')}
                  onChange={(event) => patchLibrary(index, 'label', event.target.value)}
                />
                <span className='text-muted-foreground text-xs'>
                  {t('{{count}} fields', {
                    count: Array.isArray(item.fields) ? item.fields.length : 0,
                  })}
                </span>
              </div>
            )
          })}
        </div>
      </section>
    </div>
  )
}

/** Reads one value as a plain object (anything else answers `{}`). */
function asObject(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>
  }
  return {}
}

/** Answers whether a value is a plain JSON object. */
function isPlainObject(value: unknown): boolean {
  return !!value && typeof value === 'object' && !Array.isArray(value)
}
