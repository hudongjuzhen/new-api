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
import { MESSAGE_ROLES, MESSAGE_STATUS } from '../../constants'
import type { Message } from '../../types'
import { parseThinkTags } from './message-reasoning-utils'

type MessageContentStateBase = {
  displayContent: string
  hasSources: boolean
  isAssistant: boolean
  showLoader: boolean
  showMessageContent: boolean
  sources: NonNullable<Message['sources']>
}

type MessageContentState = MessageContentStateBase &
  (
    | {
        hasReasoning: true
        reasoningContent: string
      }
    | {
        hasReasoning: false
        reasoningContent: undefined
      }
  )

function shouldShowMessageLoader(
  message: Message,
  isAssistant: boolean,
  versionContent: string
): boolean {
  return (
    isAssistant &&
    !message.isReasoningStreaming &&
    (message.status === MESSAGE_STATUS.LOADING ||
      (message.status === MESSAGE_STATUS.STREAMING && !versionContent))
  )
}

function shouldShowMessageContent(
  message: Message,
  versionContent: string
): boolean {
  return (
    (message.from === MESSAGE_ROLES.USER || !message.isReasoningStreaming) &&
    versionContent.length > 0
  )
}

function getDisplayContent(message: Message, versionContent: string): string {
  if (message.from !== MESSAGE_ROLES.ASSISTANT) {
    return versionContent
  }

  if (!versionContent.includes('<think>')) {
    return versionContent
  }

  return parseThinkTags(versionContent).visibleContent
}

/**
 * Extract the markdown image alternate-text reference used by image models
 * (e.g. gpt-image-2-c emits `![image_1](data:image/png;base64,....)`).
 * The alt text must be non-empty and the destination must be an image data
 * URI, otherwise this message carries no in-chat image.
 */
export function extractMarkdownImageRef(content: string): string | null {
  if (!content || content.length > 30000) return null
  const match = /!\[([^\]]*)\]\((data:image\/[A-Za-z0-9.+-]+;base64,[A-Za-z0-9+/=\s]+)\)/.exec(content)
  if (!match) return null
  const alt = match[1].trim()
  const dataUri = match[2].trim()
  if (!alt || !dataUri) return null
  return dataUri
}

/**
 * Remove markdown image references that have been captured into message.images
 * (e.g. `![image_1](data:image/png;base64,...)`), so the visible message shows
 * the rendered thumbnail instead of a wall of base64 text.
 */
export function stripMarkdownImageRefs(content: string): string {
  if (!content || !content.includes('![')) return content
  return content.replace(
    /!\[[^\]]*\]\(data:image\/[A-Za-z0-9.+-]+;base64,[A-Za-z0-9+/=\s]+\)/g,
    ''
  )
}

export function getMessageContentState(
  message: Message,
  versionContent: string
): MessageContentState {
  const isAssistant = message.from === MESSAGE_ROLES.ASSISTANT
  const sources = message.sources ?? []
  const reasoningContent = isAssistant ? message.reasoning?.content : undefined
  const showLoader = shouldShowMessageLoader(
    message,
    isAssistant,
    versionContent
  )
  const showMessageContent = shouldShowMessageContent(message, versionContent)

  const baseState: MessageContentStateBase = {
    displayContent: getDisplayContent(message, versionContent),
    hasSources: sources.length > 0,
    isAssistant,
    showLoader,
    showMessageContent,
    sources,
  }

  if (reasoningContent) {
    return {
      ...baseState,
      hasReasoning: true,
      reasoningContent,
    }
  }

  return {
    ...baseState,
    hasReasoning: false,
    reasoningContent: undefined,
  }
}
