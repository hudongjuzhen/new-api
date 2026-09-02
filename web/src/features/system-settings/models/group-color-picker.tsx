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
import { Check, RotateCcw } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { GROUP_COLOR_PRESETS, hexToRgba, stringToColor } from '@/lib/colors'
import { cn } from '@/lib/utils'

type GroupColorPickerProps = {
  /** The group this picker edits (used for the auto-color preview). */
  group: string
  /** Currently selected explicit HEX color, or '' when using auto color. */
  value: string
  /** Called with the new HEX color, or '' to restore auto color. */
  onChange: (value: string) => void
}

/**
 * Color picker for a pricing group. Offers the preset palette, a native color
 * input for arbitrary colors, and a "reset to auto" action that reverts to the
 * stable hash-generated color.
 */
export function GroupColorPicker({
  group,
  value,
  onChange,
}: GroupColorPickerProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  // Keyed by the committed value so the draft input remounts (and resets)
  // whenever the external color changes, avoiding a sync-in-effect.
  const [draft, setDraft] = useState<{ color: string; draft: string }>(() => ({
    color: value,
    draft: value,
  }))
  const synced = draft.color === value
  const hexInput = synced ? draft.draft : value
  const setHexInput = (next: string) => setDraft({ color: value, draft: next })

  // Preview uses the selected color, or the group's hash color when unset.
  const previewColor = useMemo(() => {
    if (value) return value
    return hexToRgba(stringToColor(group) as unknown as string, 1)
  }, [group, value])

  const handleHexInputCommit = () => {
    const normalized = hexInput.trim().toUpperCase()
    if (/^#[0-9A-F]{6}$/.test(normalized)) {
      onChange(normalized)
      setDraft({ color: normalized, draft: normalized })
    } else {
      setDraft({ color: value, draft: value })
    }
  }

  const committedHex = /^#[0-9A-F]{6}$/.test(value) ? value : undefined

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <button
            type='button'
            className='hover:border-foreground/40 inline-flex h-8 w-14 items-center justify-center rounded-lg border bg-transparent transition-colors'
            aria-label={t('Pick a color')}
          >
            <span
              className='inline-block h-4 w-7 rounded-full border'
              style={{
                backgroundColor: committedHex ?? hexToRgba(previewColor, 0.9),
                borderColor: hexToRgba(previewColor, 0.5),
              }}
            />
          </button>
        }
      />
      <PopoverContent align='start' className='w-auto p-2.5'>
        <div className='space-y-2.5'>
          <div className='grid grid-cols-8 gap-1.5'>
            {GROUP_COLOR_PRESETS.map((preset) => {
              const active = value.toUpperCase() === preset
              return (
                <button
                  key={preset}
                  type='button'
                  aria-label={preset}
                  title={preset}
                  className={cn(
                    'flex h-6 w-6 items-center justify-center rounded-full transition-transform hover:scale-110',
                    active && 'ring-foreground/60 ring-2 ring-offset-1'
                  )}
                  style={{ backgroundColor: preset }}
                  onClick={() => onChange(preset)}
                >
                  {active && <Check className='text-background h-3.5 w-3.5' />}
                </button>
              )
            })}
          </div>

          <div className='flex items-center gap-2'>
            <input
              type='color'
              value={committedHex ?? '#3B82F6'}
              onChange={(event) =>
                onChange(event.target.value.toUpperCase())
              }
              className='h-8 w-8 cursor-pointer rounded border bg-transparent p-0.5'
              aria-label={t('Custom color')}
            />
            <input
              type='text'
              value={hexInput}
              onChange={(event) => setHexInput(event.target.value)}
              onBlur={handleHexInputCommit}
              onKeyDown={(event) => {
                if (event.key === 'Enter') {
                  handleHexInputCommit()
                  setOpen(false)
                }
              }}
              placeholder='#3B82F6'
              className='focus-visible:border-ring focus-visible:ring-ring/50 h-8 w-28 rounded-lg border bg-transparent px-2.5 font-mono text-xs uppercase outline-none transition-colors focus-visible:ring-3'
              aria-label={t('Custom color')}
            />
          </div>

          <button
            type='button'
            onClick={() => onChange('')}
            className='text-muted-foreground hover:text-foreground inline-flex items-center gap-1.5 text-xs transition-colors'
          >
            <RotateCcw className='h-3.5 w-3.5' />
            {t('Reset to auto')}
          </button>
        </div>
      </PopoverContent>
    </Popover>
  )
}