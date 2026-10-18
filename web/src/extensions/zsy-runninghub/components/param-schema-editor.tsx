/*
Copyright (C) 2023-2026 QuantumNous

RunningHub app parameter-template editor: one card per schema parameter.

Two things live here because they are both properties of the list the admin
arranges:

- Order. The schema array order is the order the app's run form renders its
  fields in, so a reorder is part of the submitted app record rather than a
  view preference.
- Default value. Every field carries the value the run form pre-fills for the
  customer (`defaultValue` in the schema, honoured by ParamField in the app
  center).

The handle is the drag source and also carries the keyboard equivalent
(ArrowUp / ArrowDown), because a drag is not reachable without a pointer. The
card itself is deliberately not draggable: its own text inputs would start a
card drag instead of selecting text.
*/

import { GripVertical, Plus, X } from 'lucide-react'
import {
  useState,
  type DragEvent,
  type KeyboardEvent,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

import type { SchemaParam } from '../api'

const TYPE_CHOICES = [
  'text',
  'textarea',
  'number',
  'image',
  'audio',
  'video',
  'select',
  'switch',
]

/** Types the submit validator treats as numbers, i.e. the ones that honour
 * min/max bounds. Kept in sync with coerceValueByType in controllers_user.go. */
const NUMERIC_TYPES = new Set([
  'number',
  'int',
  'integer',
  'float',
  'duration',
  'seconds',
])

/** Optional numeric bound: an empty input clears the bound. */
function parseBound(raw: string): number | undefined {
  const trimmed = raw.trim()
  if (trimmed === '') return undefined
  const n = Number(trimmed)
  return Number.isFinite(n) ? n : undefined
}

/** Option list from its JSON text; anything unparsable reads as "no options". */
function parseOptions(raw: string): SchemaParam['options'] {
  try {
    const arr = JSON.parse(raw)
    if (!Array.isArray(arr)) return []
    return arr.map((o) =>
      typeof o === 'string'
        ? { label: o, value: o }
        : { label: o?.label ?? o?.value, value: o?.value ?? o?.label }
    )
  } catch {
    return []
  }
}

/** Which side of the hovered card the dragged parameter would land on. */
type DropPosition = 'before' | 'after'

/** The list's in-flight drag, cleared whenever a drop or the drag ends. */
type ParamDragState = {
  from: number | null
  over: number | null
  position: DropPosition
}

const IDLE_DRAG: ParamDragState = { from: null, over: null, position: 'before' }

export function ParamSchemaEditor(props: {
  params: SchemaParam[]
  /** Receives the whole list, so content and order travel together. */
  onChange: (params: SchemaParam[]) => void
}) {
  const { t } = useTranslation()
  const [drag, setDrag] = useState<ParamDragState>(IDLE_DRAG)

  const setParam = (index: number, patch: Partial<SchemaParam>) => {
    props.onChange(
      props.params.map((param, i) =>
        i === index ? { ...param, ...patch } : param
      )
    )
  }

  const removeParam = (index: number) => {
    props.onChange(props.params.filter((_, i) => i !== index))
  }

  const addParam = () => {
    props.onChange([
      ...props.params,
      { nodeId: '', fieldName: '', label: '', type: 'text', required: true },
    ])
  }

  /** Move the card at `from` so that it ends up at index `to`. */
  const moveParam = (from: number, to: number) => {
    if (from === to || to < 0 || to >= props.params.length) return
    const next = [...props.params]
    const [moved] = next.splice(from, 1)
    next.splice(to, 0, moved)
    props.onChange(next)
  }

  const resetDrag = () => setDrag(IDLE_DRAG)

  const handleDragStart = (
    event: DragEvent<HTMLSpanElement>,
    index: number
  ) => {
    setDrag({ from: index, over: null, position: 'before' })
    event.dataTransfer.effectAllowed = 'move'
    event.dataTransfer.setData('text/plain', String(index))
  }

  const handleDragOver = (event: DragEvent<HTMLLIElement>, index: number) => {
    event.preventDefault()
    if (drag.from === null || drag.from === index) return
    // The half of the card the pointer is in decides which side it lands on.
    const rect = event.currentTarget.getBoundingClientRect()
    const position: DropPosition =
      event.clientY - rect.top > rect.height / 2 ? 'after' : 'before'
    setDrag((prev) =>
      prev.over === index && prev.position === position
        ? prev
        : { ...prev, over: index, position }
    )
    event.dataTransfer.dropEffect = 'move'
  }

  const handleDrop = (event: DragEvent<HTMLLIElement>, target: number) => {
    event.preventDefault()
    const from = drag.from
    // A drop can land on a card dragover never reported (a fast release), so
    // the recorded side only counts when it belongs to this card.
    const position = drag.over === target ? drag.position : 'before'
    resetDrag()
    if (from === null || from === target) return
    // Where the card ends up once it has been lifted out of the list.
    let to: number
    if (from < target) {
      to = position === 'after' ? target : target - 1
    } else {
      to = position === 'after' ? target + 1 : target
    }
    moveParam(from, to)
  }

  /** Move one card one position towards the front (up) or the back (down). */
  const handleMoveKey = (
    event: KeyboardEvent<HTMLSpanElement>,
    index: number
  ) => {
    if (event.key !== 'ArrowUp' && event.key !== 'ArrowDown') return
    event.preventDefault()
    moveParam(index, index + (event.key === 'ArrowUp' ? -1 : 1))
  }

  return (
    <>
      {props.params.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t(
            'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.'
          )}
        </p>
      ) : (
        <ol className='space-y-3'>
          {props.params.map((param, index) => {
            // Identity is the row position, never the (nodeId, fieldName) it
            // holds: those are edited in place, and a value-derived key would
            // remount — and blur — the input after every keystroke.
            const cardKey = `rh-app-param-${index}`
            const defaultId = `rh-param-default-${index}`
            // Names the handle for screen readers, so the same words on every row
            // still say which field it moves.
            const name =
              param.label.trim() || param.fieldName.trim() || String(index + 1)
            const isDropTarget = drag.over === index && drag.from !== index

            let defaultControl: ReactNode
            if (param.type === 'switch') {
              defaultControl = (
                <div className='flex items-center gap-2'>
                  <Switch
                    id={defaultId}
                    checked={param.defaultValue === 'true'}
                    onCheckedChange={(v) =>
                      setParam(index, { defaultValue: v ? 'true' : 'false' })
                    }
                  />
                  <span className='text-muted-foreground text-xs'>
                    {param.defaultValue === 'true'
                      ? t('Enabled')
                      : t('Disabled')}
                  </span>
                </div>
              )
            } else if (param.type === 'select') {
              // A dropdown is matched by option value upstream, so the default
              // is chosen from the declared options instead of typed. A stored
              // default no option matches any more stays listed, so rewriting
              // the options cannot silently hide the value that is submitted.
              const options = (param.options ?? []).filter(
                (o) => o.value !== ''
              )
              const stored = param.defaultValue ?? ''
              if (stored !== '' && !options.some((o) => o.value === stored)) {
                options.push({ label: stored, value: stored })
              }
              defaultControl = (
                <Select
                  value={stored}
                  onValueChange={(v) => {
                    if (v != null) setParam(index, { defaultValue: v })
                  }}
                >
                  <SelectTrigger id={defaultId} className='w-full'>
                    <SelectValue placeholder={t('No default')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value=''>{t('No default')}</SelectItem>
                    {options.map((o) => (
                      <SelectItem key={`${o.value}-${o.label}`} value={o.value}>
                        {o.label || o.value}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              )
            } else {
              defaultControl = (
                <Input
                  id={defaultId}
                  value={param.defaultValue ?? ''}
                  placeholder={t('No default')}
                  onChange={(e) =>
                    setParam(index, { defaultValue: e.target.value })
                  }
                />
              )
            }

            return (
              <li
                key={cardKey}
                onDragOver={(event) => handleDragOver(event, index)}
                onDrop={(event) => handleDrop(event, index)}
                className={cn(
                  'flex gap-2 rounded-md border p-2 transition-opacity',
                  drag.from === index && 'opacity-50',
                  isDropTarget &&
                    drag.position === 'before' &&
                    'border-t-primary border-t-2',
                  isDropTarget &&
                    drag.position === 'after' &&
                    'border-b-primary border-b-2'
                )}
              >
                <span
                  role='button'
                  tabIndex={0}
                  draggable
                  title={t('Drag to reorder')}
                  aria-label={t('Drag {{field}} to reorder', { field: name })}
                  onDragStart={(event) => handleDragStart(event, index)}
                  onDragEnd={resetDrag}
                  onKeyDown={(event) => handleMoveKey(event, index)}
                  className={cn(
                    buttonVariants({ variant: 'ghost', size: 'icon-sm' }),
                    'cursor-grab self-start active:cursor-grabbing'
                  )}
                >
                  <GripVertical aria-hidden='true' className='size-3.5' />
                </span>

                <div className='min-w-0 flex-1 space-y-2'>
                  <div className='grid grid-cols-[1fr_140px] gap-2'>
                    <Input
                      value={param.label}
                      placeholder={t('Label')}
                      aria-label={t('Label')}
                      onChange={(e) =>
                        setParam(index, { label: e.target.value })
                      }
                    />
                    <Select
                      value={param.type}
                      onValueChange={(v) => {
                        if (v != null) setParam(index, { type: v })
                      }}
                    >
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {TYPE_CHOICES.map((c) => (
                          <SelectItem key={c} value={c}>
                            {c}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </div>

                  <div className='grid grid-cols-2 gap-2'>
                    <Input
                      value={param.nodeId ?? ''}
                      placeholder={t('Node ID')}
                      aria-label={t('Node ID')}
                      onChange={(e) =>
                        setParam(index, { nodeId: e.target.value })
                      }
                    />
                    <Input
                      value={param.fieldName}
                      placeholder={t('Field name')}
                      aria-label={t('Field name')}
                      onChange={(e) =>
                        setParam(index, { fieldName: e.target.value })
                      }
                    />
                  </div>

                  {NUMERIC_TYPES.has(param.type) && (
                    <div className='grid grid-cols-2 gap-2'>
                      <Input
                        type='number'
                        value={param.min ?? ''}
                        placeholder={t('Min')}
                        aria-label={t('Min')}
                        onChange={(e) =>
                          setParam(index, { min: parseBound(e.target.value) })
                        }
                      />
                      <Input
                        type='number'
                        value={param.max ?? ''}
                        placeholder={t('Max')}
                        aria-label={t('Max')}
                        onChange={(e) =>
                          setParam(index, { max: parseBound(e.target.value) })
                        }
                      />
                    </div>
                  )}

                  {param.type === 'select' && (
                    <textarea
                      className='min-h-16 w-full rounded-md border bg-transparent px-2 py-1 font-mono text-xs'
                      placeholder={t(
                        'Options as JSON, e.g. [{"label":"1:1","value":"1:1"}]'
                      )}
                      aria-label={t('Options')}
                      value={JSON.stringify(param.options ?? [])}
                      onChange={(e) =>
                        setParam(index, {
                          options: parseOptions(e.target.value),
                        })
                      }
                    />
                  )}

                  {/* An empty default is the same state as no default: the run form
                  leaves the field blank until the customer fills it. */}
                  <div className='flex items-center gap-2'>
                    <Label
                      htmlFor={defaultId}
                      className='text-muted-foreground shrink-0 text-xs'
                    >
                      {t('Default value')}
                    </Label>
                    <div className='min-w-0 flex-1'>{defaultControl}</div>
                  </div>

                  <div className='flex items-center justify-between'>
                    <label className='flex items-center gap-2'>
                      <Switch
                        checked={!!param.required}
                        onCheckedChange={(v) =>
                          setParam(index, { required: !!v })
                        }
                      />
                      <span className='text-xs'>{t('Required')}</span>
                    </label>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      title={t('Remove')}
                      aria-label={t('Remove')}
                      onClick={() => removeParam(index)}
                    >
                      <X className='size-3.5' />
                    </Button>
                  </div>
                </div>
              </li>
            )
          })}
        </ol>
      )}
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={addParam}
        className='w-full'
      >
        <Plus className='size-4' />
        {t('Add Parameter')}
      </Button>
    </>
  )
}
