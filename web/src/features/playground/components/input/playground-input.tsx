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
import { AlertCircleIcon, Loader2Icon, XIcon } from 'lucide-react'
import { nanoid } from 'nanoid'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  PromptInput,
  PromptInputFooter,
  PromptInputTextarea,
  type PromptInputMessage,
} from '@/components/ai-elements/prompt-input'

import { uploadPlaygroundImage } from '../../api'
import { IMAGE_UPLOAD } from '../../constants'
import { getSubmittableInputText } from '../../lib'
import type {
  ModelOption,
  GroupOption,
  ParameterEnabled,
  PlaygroundConfig,
} from '../../types'
import { PlaygroundInputControls } from './playground-input-controls'
import { PlaygroundInputTools } from './playground-input-tools'

type PendingImage = {
  id: string
  name: string
  previewUrl: string
  /** Server-stored URL, set once the upload succeeds */
  url?: string
  status: 'uploading' | 'ready' | 'error'
}

interface PlaygroundInputProps {
  config: PlaygroundConfig
  onSubmit: (text: string, images?: string[]) => void
  onStop?: () => void
  disabled?: boolean
  isGenerating?: boolean
  models: ModelOption[]
  modelValue: string
  onModelChange: (value: string) => void
  isModelLoading?: boolean
  groups: GroupOption[]
  groupValue: string
  onGroupChange: (value: string) => void
  hasMessages?: boolean
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
  onClearMessages?: () => void
  onParameterEnabledChange: (
    key: keyof ParameterEnabled,
    value: boolean
  ) => void
  parameterEnabled: ParameterEnabled
}

export function PlaygroundInput({
  config,
  onSubmit,
  onStop,
  disabled,
  isGenerating,
  models,
  modelValue,
  onModelChange,
  isModelLoading = false,
  groups,
  groupValue,
  onGroupChange,
  hasMessages = false,
  onConfigChange,
  onClearMessages,
  onParameterEnabledChange,
  parameterEnabled,
}: PlaygroundInputProps) {
  const { t } = useTranslation()
  const [text, setText] = useState('')
  const [pendingImages, setPendingImages] = useState<PendingImage[]>([])

  const readyImageUrls = pendingImages
    .filter((image) => image.status === 'ready' && image.url)
    .map((image) => image.url as string)
  const isUploadingImages = pendingImages.some(
    (image) => image.status === 'uploading'
  )
  const hasReadyImages = readyImageUrls.length > 0

  const removeImage = (id: string) => {
    setPendingImages((previous) => {
      const target = previous.find((image) => image.id === id)
      if (target) {
        URL.revokeObjectURL(target.previewUrl)
      }
      return previous.filter((image) => image.id !== id)
    })
  }

  const clearImages = () => {
    setPendingImages((previous) => {
      previous.forEach((image) => URL.revokeObjectURL(image.previewUrl))
      return []
    })
  }

  const uploadImage = async (file: File) => {
    const id = nanoid()
    const previewUrl = URL.createObjectURL(file)
    setPendingImages((previous) => [
      ...previous,
      { id, name: file.name, previewUrl, status: 'uploading' },
    ])

    try {
      const url = await uploadPlaygroundImage(file)
      setPendingImages((previous) =>
        previous.map((image) =>
          image.id === id ? { ...image, url, status: 'ready' } : image
        )
      )
    } catch (error) {
      setPendingImages((previous) =>
        previous.map((image) =>
          image.id === id ? { ...image, status: 'error' } : image
        )
      )
      toast.error(
        error instanceof Error ? error.message : t('Image upload failed')
      )
    }
  }

  const handleImagesPicked = (files: FileList | File[]) => {
    const imageFiles = [...files].filter((file) => file.type.startsWith('image/'))
    if (imageFiles.length === 0) {
      toast.error(t('Please select image files'))
      return
    }

    const remainingSlots = IMAGE_UPLOAD.MAX_FILES - pendingImages.length
    if (remainingSlots <= 0) {
      toast.error(
        t('You can attach up to {{count}} images', {
          count: IMAGE_UPLOAD.MAX_FILES,
        })
      )
      return
    }

    const accepted = imageFiles.slice(0, remainingSlots)
    if (accepted.length < imageFiles.length) {
      toast.warning(
        t('Only the first {{count}} images were added', {
          count: accepted.length,
        })
      )
    }

    accepted.forEach((file) => {
      void uploadImage(file)
    })
  }

  const handleSubmit = (message: PromptInputMessage) => {
    if (isUploadingImages) {
      toast.info(t('Please wait for the image to finish uploading'))
      return
    }

    const submittableText = getSubmittableInputText(
      message,
      disabled,
      hasReadyImages
    )
    if (submittableText === null) return

    onSubmit(submittableText, readyImageUrls)
    setText('')
    clearImages()
  }

  return (
    <div className='grid shrink-0 gap-4 px-1 md:pb-4'>
      <PromptInput
        className='relative'
        groupClassName='bg-background/95 dark:bg-background/80 border-border/70 shadow-[0_18px_60px_-32px_rgba(0,0,0,0.65)] ring-1 ring-foreground/5 rounded-xl overflow-hidden transition-all duration-200 focus-within:border-primary/45 focus-within:ring-primary/15 focus-within:shadow-[0_22px_70px_-34px_rgba(0,0,0,0.75)]'
        onSubmit={handleSubmit}
      >
        {pendingImages.length > 0 && (
          <div className='flex flex-wrap gap-2 px-5 pt-4'>
            {pendingImages.map((image) => (
              <div
                className='border-border/60 relative size-16 overflow-hidden rounded-md border'
                key={image.id}
              >
                <img
                  alt={image.name}
                  className='size-full object-cover'
                  src={image.previewUrl}
                />
                {image.status === 'uploading' && (
                  <div className='absolute inset-0 flex items-center justify-center bg-black/40'>
                    <Loader2Icon
                      className='size-4 animate-spin text-white'
                      size={16}
                    />
                  </div>
                )}
                {image.status === 'error' && (
                  <div className='bg-destructive/70 absolute inset-0 flex items-center justify-center'>
                    <AlertCircleIcon className='size-4 text-white' size={16} />
                  </div>
                )}
                <button
                  aria-label={t('Remove image')}
                  className='absolute top-0.5 right-0.5 flex size-4 items-center justify-center rounded-full bg-black/60 text-white hover:bg-black/80'
                  onClick={() => removeImage(image.id)}
                  type='button'
                >
                  <XIcon size={10} />
                </button>
              </div>
            ))}
          </div>
        )}

        <PromptInputTextarea
          autoComplete='off'
          autoCorrect='off'
          autoCapitalize='off'
          spellCheck={false}
          className='min-h-20 px-5 pt-4 pb-3 leading-7 md:min-h-24 md:text-base'
          disabled={disabled}
          onChange={(event) => setText(event.target.value)}
          placeholder={t('Ask anything')}
          value={text}
        />

        <PromptInputFooter className='border-border/60 bg-muted/20 dark:bg-muted/10 border-t px-3 py-2.5 backdrop-blur'>
          <PlaygroundInputControls
            disabled={disabled}
            groups={groups}
            groupValue={groupValue}
            hasImages={hasReadyImages}
            isGenerating={isGenerating}
            isModelLoading={isModelLoading}
            models={models}
            modelValue={modelValue}
            onGroupChange={onGroupChange}
            onModelChange={onModelChange}
            onStop={onStop}
            text={text}
            tools={
              <PlaygroundInputTools
                config={config}
                disabled={disabled}
                hasMessages={hasMessages}
                isUploadingImages={isUploadingImages}
                onConfigChange={onConfigChange}
                onClearMessages={onClearMessages}
                onImagesPicked={handleImagesPicked}
                onParameterEnabledChange={onParameterEnabledChange}
                parameterEnabled={parameterEnabled}
              />
            }
          />
        </PromptInputFooter>
      </PromptInput>
    </div>
  )
}
