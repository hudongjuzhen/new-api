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
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'

import type { ToneView } from '../api'
import type { ToneExamplePair } from '../lib/tone-fields'

interface ToneSampleDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The row whose pair is being read, or null when nothing is open. */
  tone: ToneView | null
  /** The standard's own note on what a pair is for, when it ships one. */
  examplePair: ToneExamplePair | null
}

/** One half of the pair, in a scrollable box so long prose stays readable. */
function SampleColumn(props: { label: string; body: string }) {
  return (
    <div className='space-y-2'>
      <p className='text-sm font-medium'>{props.label}</p>
      <div className='max-h-80 overflow-y-auto rounded-md border p-3'>
        <p className='text-sm break-words whitespace-pre-wrap'>
          {props.body || '-'}
        </p>
      </div>
    </div>
  )
}

/**
 * The before / after pair, side by side.
 *
 * A tone cannot be listened to or looked at, so the pair is the only thing that
 * shows an operator what a row actually does to a text: this is the tone plaza's
 * equivalent of the voice plaza's audio player. Either half may be empty (the
 * button that opens this dialog is hidden only when *both* are), so each column
 * renders its own placeholder rather than collapsing the layout.
 */
export function ToneSampleDialog(props: ToneSampleDialogProps) {
  const { t } = useTranslation()
  const tone = props.tone
  // The standard's own prose about the pair, not UI strings, so it is rendered
  // as-is instead of through t(): `convention` is the rule (many tones share one
  // sampleInput), `whyItMatters` is why the rule exists at all.
  const convention = props.examplePair?.convention ?? ''
  const whyItMatters = props.examplePair?.whyItMatters ?? ''

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Sample')}
      description={tone?.name ?? ''}
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => props.onOpenChange(false)}
        >
          {t('Close')}
        </Button>
      }
    >
      {convention ? (
        <p className='text-muted-foreground text-sm'>{convention}</p>
      ) : null}
      <div className='grid gap-4 sm:grid-cols-2'>
        <SampleColumn
          label={t('Sample Input')}
          body={tone?.sampleInput ?? ''}
        />
        <SampleColumn
          label={t('Sample Output')}
          body={tone?.sampleOutput ?? ''}
        />
      </div>
      {whyItMatters ? (
        <p className='text-muted-foreground text-xs'>{whyItMatters}</p>
      ) : null}
    </Dialog>
  )
}
