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
import { z } from 'zod'

import type { VoiceView } from '../api'
import {
  formatSceneList,
  parseSceneList,
  VOICE_LANGUAGE_PATTERN,
  VOICE_SCENES_MAX,
  VOICE_SCENES_MESSAGE_TOO_LONG,
  VOICE_SCENES_MESSAGE_TOO_MANY,
  VOICE_SCENE_MAX_LENGTH,
} from './voice-fields'

/**
 * Voice form contract. The bounds mirror the backend validator
 * (zsy/voice/store.go validateVoice) so the dialog refuses what the API would
 * reject, with the message attached to the offending field.
 */
export const VOICE_NAME_MAX_LENGTH = 191
export const VOICE_TYPE_MAX_LENGTH = 191
export const VOICE_DESCRIPTION_MAX_LENGTH = 2000
export const VOICE_SORT_ORDER_MAX = 1_000_000

export const voiceFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Voice name is required')
    .max(VOICE_NAME_MAX_LENGTH, 'Voice name is too long (max 191 characters)'),
  voiceType: z
    .string()
    .trim()
    .min(1, 'voice_type is required')
    .max(VOICE_TYPE_MAX_LENGTH, 'voice_type is too long (max 191 characters)'),
  description: z
    .string()
    .max(
      VOICE_DESCRIPTION_MAX_LENGTH,
      'Introduction is too long (max 2000 characters)'
    ),
  // The controlled vocabularies are validated by the API against their own
  // list; the dialog only offers values from it, so the form just carries them.
  gender: z.string(),
  ageRange: z.string(),
  // Edited as one comma-separated line, submitted as the API's array.
  scenes: z
    .string()
    .refine(
      (value) => parseSceneList(value).length <= VOICE_SCENES_MAX,
      VOICE_SCENES_MESSAGE_TOO_MANY
    )
    .refine(
      (value) =>
        parseSceneList(value).every(
          (scene) => [...scene].length <= VOICE_SCENE_MAX_LENGTH
        ),
      VOICE_SCENES_MESSAGE_TOO_LONG
    ),
  // Optional short tag; the backend normalizes it to lower case and accepts any
  // tag a provider uses, so the form only checks the shape.
  language: z
    .string()
    .trim()
    .refine(
      (value) =>
        value === '' || VOICE_LANGUAGE_PATTERN.test(value.toLowerCase()),
      'Invalid language code'
    ),
  // Optional portrait: either the gateway's own path or an absolute http(s) URL.
  avatarUrl: z
    .string()
    .trim()
    .refine(
      (value) =>
        value === '' ||
        (value.startsWith('/') && !value.startsWith('//')) ||
        /^https?:\/\//i.test(value),
      'Avatar must be an /uploads path or an http(s) URL'
    ),
  audioUrl: z.string(),
  audioName: z.string(),
  audioSize: z.number(),
  enabled: z.boolean(),
  sortOrder: z
    .number()
    .int('Sort order must be a whole number')
    .min(-VOICE_SORT_ORDER_MAX, 'Sort order is out of range')
    .max(VOICE_SORT_ORDER_MAX, 'Sort order is out of range'),
})

export type VoiceFormValues = z.infer<typeof voiceFormSchema>

/** Empty state of the create dialog: a new voice is published to the plaza. */
export const EMPTY_VOICE_FORM: VoiceFormValues = {
  name: '',
  voiceType: '',
  description: '',
  gender: '',
  ageRange: '',
  language: '',
  scenes: '',
  avatarUrl: '',
  audioUrl: '',
  audioName: '',
  audioSize: 0,
  enabled: true,
  sortOrder: 0,
}

/** Form state pre-filled from a stored row. */
export function voiceToFormValues(voice: VoiceView): VoiceFormValues {
  return {
    name: voice.name,
    voiceType: voice.voiceType,
    description: voice.description,
    gender: voice.gender,
    ageRange: voice.ageRange,
    language: voice.language,
    scenes: formatSceneList(voice.scenes),
    avatarUrl: voice.avatarUrl,
    audioUrl: voice.audioUrl,
    audioName: voice.audioName,
    audioSize: voice.audioSize,
    enabled: voice.enabled,
    sortOrder: voice.sortOrder,
  }
}
