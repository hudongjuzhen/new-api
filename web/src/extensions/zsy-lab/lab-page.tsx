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
import { useNavigate } from '@tanstack/react-router'
import { ArrowLeftRightIcon } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { parseTags } from '@/features/pricing/lib/filters'
import type { PricingModel } from '@/features/pricing/types'
import { PublicLayout } from '@/components/layout'
import { getLobeIcon } from '@/lib/lobe-icon'

import { LabApi } from './lab-api'
import { LabExamples } from './lab-examples'
import { LabHistory } from './lab-history'
import { LabImagePlayground } from './lab-image-playground'
import { LabPlayground } from './lab-playground'
import { isImageGenModel } from './lib/model'

export interface LabProps {
  model?: string
}

function getModelInitials(name: string): string {
  const initials = name
    .split(/[-_.]/)
    .map((segment) => segment.charAt(0))
    .join('')
    .slice(0, 2)
    .toUpperCase()
  return initials || 'AI'
}

export function Lab(props: LabProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { models } = usePricingData()

  const modelName = props.model?.trim() ? props.model.trim() : ''
  const modelInfo = useMemo<PricingModel | null>(() => {
    if (!modelName) return null
    return models.find((model) => model.model_name === modelName) ?? null
  }, [models, modelName])

  const tags = parseTags(modelInfo?.tags)
  const iconKey = modelInfo?.icon || modelInfo?.vendor_icon
  const modelIcon = iconKey ? getLobeIcon(iconKey, 24) : null

  return (
    <PublicLayout>
      <div className='mx-auto max-w-7xl px-0 sm:px-2'>
        <header className='flex flex-wrap items-start gap-3 py-6'>
          <div className='bg-primary/10 text-primary flex size-14 shrink-0 items-center justify-center overflow-hidden rounded-xl text-lg font-bold'>
            {modelIcon ?? getModelInitials(modelName || 'AI')}
          </div>
          <div className='min-w-0 flex-1'>
            <h1 className='truncate font-mono text-xl font-bold tracking-tight sm:text-2xl'>
              {modelName || t('No model selected')}
            </h1>
            {modelInfo?.description && (
              <p className='text-muted-foreground mt-1 line-clamp-2 text-sm leading-relaxed'>
                {modelInfo.description}
              </p>
            )}
            {tags.length > 0 && (
              <div className='mt-2 flex flex-wrap gap-1.5'>
                {tags.map((tag) => (
                  <Badge
                    className='text-xs font-normal'
                    key={tag}
                    variant='secondary'
                  >
                    {tag}
                  </Badge>
                ))}
              </div>
            )}
          </div>
          <Button
            onClick={() => navigate({ to: '/pricing' })}
            size='sm'
            variant='outline'
          >
            <ArrowLeftRightIcon className='size-3.5' />
            {t('Switch model')}
          </Button>
        </header>

        <Tabs className='gap-4' defaultValue='playground'>
          <TabsList
            className='border-border/60 w-full justify-start gap-2 rounded-none border-b pb-1'
            variant='line'
          >
            <TabsTrigger className='px-3' value='playground'>
              {t('Playground')}
            </TabsTrigger>
            <TabsTrigger className='px-3' value='examples'>
              {t('Examples')}
            </TabsTrigger>
            <TabsTrigger className='px-3' value='api'>
              {t('API')}
            </TabsTrigger>
            <TabsTrigger className='px-3' value='history'>
              {t('History')}
            </TabsTrigger>
          </TabsList>

          <TabsContent value='playground'>
            {isImageGenModel(modelInfo) ? (
              <LabImagePlayground model={modelName || undefined} />
            ) : (
              <LabPlayground model={modelName || undefined} />
            )}
          </TabsContent>
          <TabsContent value='examples'>
            <LabExamples model={modelName || undefined} />
          </TabsContent>
          <TabsContent value='api'>
            <LabApi model={modelName || undefined} />
          </TabsContent>
          <TabsContent value='history'>
            <LabHistory />
          </TabsContent>
        </Tabs>
      </div>
    </PublicLayout>
  )
}
