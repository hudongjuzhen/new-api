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

import { hexToRgba } from '@/lib/colors'
import { cn } from '@/lib/utils'
import { useGroupColor } from '@/stores/group-color-store'

import { StatusBadge, type StatusBadgeProps } from './status-badge'

type GroupBadgeProps = Omit<
  StatusBadgeProps,
  'autoColor' | 'label' | 'variant'
> & {
  group?: string | null
  label?: string
  ratio?: number | null
  /** Overrides the default `${ratio}x` pill text (e.g. a percentage label). */
  ratioLabel?: string
}

function getGroupRatioClassName(ratio: number): string {
  if (ratio > 1) {
    return 'bg-warning/10 text-warning'
  }
  if (ratio < 1) {
    return 'bg-info/10 text-info'
  }
  return 'bg-muted text-muted-foreground'
}

function getGroupLabel(params: {
  labelOverride?: string
  groupName?: string
  isAutoGroup: boolean
  isEmptyGroup: boolean
  t: (key: string) => string
}): string {
  if (params.labelOverride) return params.labelOverride
  if (params.isEmptyGroup) return params.t('User Group')
  if (params.isAutoGroup) return params.t('Auto')
  return params.groupName ?? ''
}

/** Formats a group ratio multiplier as an explicit percentage (0.5 -> "50%"). */
// eslint-disable-next-line react-refresh/only-export-components
export { formatGroupRatioPercent } from '@/lib/group-ratio-format'

export function GroupBadge(props: GroupBadgeProps) {
  const { t } = useTranslation()
  const {
    group,
    label: labelOverride,
    ratio,
    ratioLabel,
    copyable = false,
    showDot,
    className,
    ...badgeProps
  } = props
  const groupName = group?.trim()
  const isAutoGroup = groupName === 'auto'
  const isEmptyGroup = !groupName
  const isSpecialGroup = isAutoGroup || isEmptyGroup
  const groupColor = useGroupColor(isSpecialGroup ? null : groupName)
  const label = getGroupLabel({
    labelOverride,
    groupName,
    isAutoGroup,
    isEmptyGroup,
    t,
  })

  const badge = (
    <StatusBadge
      {...badgeProps}
      copyable={copyable}
      label={label}
      showDot={showDot ?? (isSpecialGroup ? false : undefined)}
      variant={isSpecialGroup ? 'neutral' : undefined}
      autoColor={isSpecialGroup ? undefined : groupName}
      color={isSpecialGroup ? undefined : groupColor}
      className={cn('min-w-0 shrink overflow-hidden', className)}
    />
  )

  if (ratio == null) {
    return badge
  }

  return (
    <span className='inline-flex max-w-full min-w-0 items-center gap-2 text-xs'>
      <span className='max-w-full min-w-0 overflow-hidden'>{badge}</span>
      <span
        className={cn(
          'inline-flex h-5 shrink-0 items-center rounded-full px-1.5 font-mono text-xs leading-none font-medium tabular-nums',
          !groupColor && getGroupRatioClassName(ratio)
        )}
        style={
          groupColor
            ? {
                color: groupColor,
                backgroundColor: hexToRgba(groupColor, 0.1),
              }
            : undefined
        }
      >
        <span>{ratioLabel ?? `${ratio}x`}</span>
      </span>
    </span>
  )
}
