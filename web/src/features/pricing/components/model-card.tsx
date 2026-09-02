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
import { ChevronRight, Copy, Play, Users } from 'lucide-react'
import { memo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getAvatarColorClass, getGroupDiscountClassName, hexToRgba } from '@/lib/colors'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'
import { useGroupColor } from '@/stores/group-color-store'

import { DEFAULT_TOKEN_UNIT } from '../constants'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
} from '../lib/dynamic-price'
import { parseTags } from '../lib/filters'
import { getConfiguredGroupRatio, isTokenBasedModel } from '../lib/model-helpers'
import { formatPrice, formatRequestPrice } from '../lib/price'
import type { PricingModel, TokenUnit } from '../types'
import { ModelBillingModeBadge } from './model-billing-mode-badge'
import { ModelPerfBadge, type ModelPerfBadgeData } from './model-perf-badge'

export interface ModelCardProps {
  model: PricingModel
  onClick: () => void
  onTry?: () => void
  priceRate?: number
  usdExchangeRate?: number
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  selectedGroup?: string
  perf?: ModelPerfBadgeData
}

export const ModelCard = memo(function ModelCard(props: ModelCardProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  const tokenUnit = props.tokenUnit ?? DEFAULT_TOKEN_UNIT
  const priceRate = props.priceRate ?? 1
  const usdExchangeRate = props.usdExchangeRate ?? 1
  const showRechargePrice = props.showRechargePrice ?? false
  const isTokenBased = isTokenBasedModel(props.model)
  const tokenUnitLabel = tokenUnit === 'K' ? '1K' : '1M'
  const tags = parseTags(props.model.tags)
  const groups = props.model.enable_groups || []
  const endpoints = props.model.supported_endpoint_types || []
  const modelIconKey = props.model.icon || props.model.vendor_icon
  const modelIcon = modelIconKey ? getLobeIcon(modelIconKey, 28) : null
  const initial = props.model.model_name?.charAt(0).toUpperCase() || '?'
  const isDynamicPricing =
    props.model.billing_mode === 'tiered_expr' &&
    Boolean(props.model.billing_expr)
  const hasCachedPrice = isTokenBased && props.model.cache_ratio != null
  const dynamicSummary = isDynamicPricing
    ? getDynamicPricingSummary(props.model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: getDynamicDisplayGroupRatio(
          props.model,
          props.selectedGroup
        ),
      })
    : null

  const primaryGroup = groups[0]
  const groupRatioMap = props.model.group_ratio || {}

  // Convert a group multiplier to a discount label:
  //  0.5 -> "5折", 0.8 -> "8折", 1 -> "原价", 1.5 -> "原价×1.5"
  const formatDiscountLabel = (ratio: number): string => {
    const percent = Math.round(ratio * 100)
    if (percent === 100) return t('Standard price')
    if (ratio < 1) {
      return `${(percent / 10).toFixed(1).replace(/\.0$/, '')}${t('% off')}`
    }
    return `${t('Standard price')}×${(percent / 100)
      .toFixed(2)
      .replace(/\.?0+$/, '')}`
  }

  // Show each group's configured multiplier as an explicit discount label:
  // 0.5 -> "5折" (50% off), 0.8 -> "8折", 1 -> "标准价格", 1.5 -> "标准价格×1.5".
  // Sorted by ratio so the best discount (lowest %) shows first.
  const groupRatios = groups
    .map((group) => ({
      group,
      ratio: getConfiguredGroupRatio(groupRatioMap, group),
      label: formatDiscountLabel(getConfiguredGroupRatio(groupRatioMap, group)),
    }))
    .sort((a, b) => a.ratio - b.ratio)
  const bottomTags = [...endpoints.slice(0, 2), ...tags.slice(0, 2)]
  const hiddenCount =
    Math.max(groups.length - 1, 0) +
    Math.max(endpoints.length - 2, 0) +
    Math.max(tags.length - 2, 0)

  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation()
    copyToClipboard(props.model.model_name || '')
  }

  let priceSummary: ReactNode
  if (dynamicSummary) {
    if (dynamicSummary.isSpecialExpression) {
      priceSummary = (
        <span className='min-w-0'>
          <span className='text-amber-700 dark:text-amber-300'>
            {t('Special billing expression')}
          </span>
          <code className='text-muted-foreground/70 mt-0.5 line-clamp-1 block font-mono text-[11px] break-all'>
            {dynamicSummary.rawExpression}
          </code>
        </span>
      )
    } else if (dynamicSummary.primaryEntries.length > 0) {
      priceSummary = (
        <>
          {dynamicSummary.primaryEntries.map((entry) => (
            <span
              key={entry.key}
              className='text-muted-foreground whitespace-nowrap'
            >
              {t(entry.shortLabel)}{' '}
              <span className='text-foreground font-mono font-semibold'>
                {entry.formatted}
              </span>
            </span>
          ))}
        </>
      )
    } else {
      priceSummary = (
        <span className='text-muted-foreground text-sm'>
          {t('Dynamic Pricing')}
        </span>
      )
    }
  } else if (isTokenBased) {
    priceSummary = (
      <>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Input')}{' '}
          <span className='text-foreground font-mono font-semibold'>
            {formatPrice(
              props.model,
              'input',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
        <span className='text-muted-foreground whitespace-nowrap'>
          {t('Output')}{' '}
          <span className='text-foreground font-mono font-semibold'>
            {formatPrice(
              props.model,
              'output',
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              props.selectedGroup
            )}
          </span>
        </span>
        {hasCachedPrice && (
          <span className='text-muted-foreground whitespace-nowrap'>
            {t('Cached')}{' '}
            <span className='text-foreground font-mono font-semibold'>
              {formatPrice(
                props.model,
                'cache',
                tokenUnit,
                showRechargePrice,
                priceRate,
                usdExchangeRate,
                props.selectedGroup
              )}
            </span>
          </span>
        )}
      </>
    )
  } else {
    priceSummary = (
      <span className='text-muted-foreground whitespace-nowrap'>
        <span className='text-foreground font-mono font-semibold'>
          {formatRequestPrice(
            props.model,
            showRechargePrice,
            priceRate,
            usdExchangeRate,
            props.selectedGroup
          )}
        </span>{' '}
        / {t('request')}
      </span>
    )
  }

  return (
    <div
      className={cn(
        'group relative flex flex-col rounded-xl border p-3 transition-colors sm:p-5',
        'hover:bg-muted/20'
      )}
    >
      {/* Header: icon + name + price + actions */}
      <div className='flex items-start justify-between gap-2.5 sm:gap-3'>
        <div className='flex min-w-0 items-start gap-2.5 sm:gap-3'>
          <div className='bg-muted/40 flex size-9 shrink-0 items-center justify-center rounded-lg sm:size-10 sm:rounded-xl'>
            {modelIcon || (
              <span className='text-muted-foreground text-sm font-bold'>
                {initial}
              </span>
            )}
          </div>
          <div className='min-w-0'>
            <h3 className='text-foreground truncate font-mono text-[15px] leading-tight font-bold'>
              {props.model.model_name}
            </h3>
            <div className='mt-0.5 flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-sm sm:mt-1 sm:gap-x-3'>
              {priceSummary}
            </div>
          </div>
        </div>

        <div className='flex shrink-0 items-center gap-1.5'>
          {props.onTry && (
            <Button
              type='button'
              size='xs'
              className='gap-1 px-2'
              onClick={(event) => {
                event.stopPropagation()
                props.onTry?.()
              }}
            >
              <Play className='size-3' />
              {t('Try Now')}
            </Button>
          )}
          <button
            type='button'
            onClick={props.onClick}
            className='text-muted-foreground hover:text-foreground hover:bg-muted inline-flex items-center gap-1 rounded-md border px-2 py-1 text-xs transition-colors sm:px-2.5 sm:py-1.5'
          >
            {t('Details')}
            <ChevronRight className='size-3.5' />
          </button>
          <button
            type='button'
            onClick={handleCopy}
            className='text-muted-foreground hover:text-foreground hover:bg-muted rounded-md border p-1.5 transition-colors'
            title={t('Copy')}
          >
            <Copy className='size-3.5' />
          </button>
        </div>
      </div>

      {/* Description */}
      <p className='text-muted-foreground mt-2 line-clamp-1 flex-1 text-[13px] leading-relaxed sm:mt-4 sm:line-clamp-2 sm:min-h-[2.5rem]'>
        {props.model.description || t('No description available.')}
      </p>

      {/* Footer: billing + tags + performance on one row, groups full-width below */}
      <div className='mt-2 grid grid-cols-[minmax(0,1fr)_auto] items-start gap-x-2 gap-y-1.5 sm:mt-4'>
        {/* Row 1: billing mode (left) + endpoints/tags & perf summary (right) */}
        <div className='inline-flex min-w-0 items-center gap-1.5'>
          <span className='text-foreground/80 text-xs font-bold uppercase tracking-wide'>
            {t('Billing')}
          </span>
          <ModelBillingModeBadge model={props.model} />
        </div>
        <div className='flex min-w-0 items-center gap-2 justify-self-end'>
          <div className='flex min-w-0 flex-wrap items-center gap-x-2.5 gap-y-0.5 sm:gap-x-3 sm:gap-y-1'>
            {bottomTags.map((item) => (
              <span key={item} className='text-muted-foreground/70 text-xs'>
                {item}
              </span>
            ))}
            <span className='text-muted-foreground/50 text-xs'>
              {tokenUnitLabel}
            </span>
            {hiddenCount > 0 && (
              <span className='text-muted-foreground/40 text-xs'>
                +{hiddenCount}
              </span>
            )}
          </div>
          <ModelPerfBadge perf={props.perf} />
        </div>

        {/* Row 2: groups span the full card width so lines wrap only when full */}
        <div className='col-span-2 flex min-w-0 w-full flex-wrap items-center gap-x-2 gap-y-1'>
          <span className='text-foreground/80 inline-flex items-center gap-1 text-xs font-bold uppercase tracking-wide'>
            <Users className='size-3.5 text-info' />
            {t('Groups')}
          </span>
          {groupRatios.length > 0 ? (
            groupRatios.map(({ group, ratio, label }) => (
              <GroupRatioCapsule
                key={group}
                group={group}
                ratio={ratio}
                label={label}
              />
            ))
          ) : (
            <span className='text-muted-foreground text-sm font-medium'>
              {primaryGroup}
            </span>
          )}
        </div>
      </div>
    </div>
  )
})

/**
 * A model card group pill with its discount capsule. Uses the admin-configured
 * group color when present (inline styles — HEX cannot be statically scanned by
 * Tailwind) and falls back to the stable hash color classes otherwise.
 */
function GroupRatioCapsule({
  group,
  ratio,
  label,
}: {
  group: string
  ratio: number
  label: string
}) {
  const groupColor = useGroupColor(group)

  if (!groupColor) {
    return (
      <span
        className={cn(
          'inline-flex h-6 items-center gap-1.5 rounded-full border px-2 text-xs font-semibold whitespace-nowrap',
          getAvatarColorClass(group)
        )}
      >
        <span>{group}</span>
        <span
          className={cn(
            'rounded-full px-1.5 py-px font-mono tabular-nums',
            getGroupDiscountClassName(group, ratio)
          )}
        >
          {label}
        </span>
      </span>
    )
  }

  // Depth/boldness encodes billing state, mirroring GROUP_TONE_TABLE:
  // discount (< 1) and premium (> 1) are emphasized, normal (= 1) stays faint.
  const emphasized = ratio !== 1
  const discountStyle: React.CSSProperties = {
    backgroundColor: hexToRgba(groupColor, emphasized ? 0.22 : 0.08),
    fontWeight: emphasized ? 700 : 500,
  }

  return (
    <span
      className='inline-flex h-6 items-center gap-1.5 rounded-full border px-2 text-xs font-semibold whitespace-nowrap'
      style={{
        color: groupColor,
        borderColor: hexToRgba(groupColor, 0.4),
        backgroundColor: hexToRgba(groupColor, 0.12),
      }}
    >
      <span>{group}</span>
      <span
        className='rounded-full px-1.5 py-px font-mono text-xs leading-none tabular-nums'
        style={discountStyle}
      >
        {label}
      </span>
    </span>
  )
}
