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
/**
 * The persona attribute contract: the controlled vocabularies, the scene editor
 * rules and the form schema, mirroring the backend contract
 * (zsy/avatar/models.go + store.go). The bounds are the same numbers the API
 * validates, so the dialog refuses exactly what the API would reject.
 */
import { z } from 'zod'

import type { AvatarView } from '../api'

export interface AvatarOption {
  /** Wire value stored by the API; '' means "not specified". */
  value: string
  /** i18n key the UI renders. */
  labelKey: string
}

/** Values accepted by the `gender` field. */
export const GENDER_OPTIONS: AvatarOption[] = [
  { value: '', labelKey: 'Not specified' },
  { value: 'female', labelKey: 'Female' },
  { value: 'male', labelKey: 'Male' },
  { value: 'neutral', labelKey: 'Neutral' },
]

/** Values accepted by the `ageRange` field. */
export const AGE_RANGE_OPTIONS: AvatarOption[] = [
  { value: '', labelKey: 'Not specified' },
  { value: 'child', labelKey: 'Child' },
  { value: 'teen', labelKey: 'Teen' },
  { value: 'young', labelKey: 'Young' },
  { value: 'middle', labelKey: 'Middle-aged' },
  { value: 'senior', labelKey: 'Senior' },
]

/** Values accepted by the `race` field. */
export const RACE_OPTIONS: AvatarOption[] = [
  { value: '', labelKey: 'Not specified' },
  { value: 'asian', labelKey: 'Asian' },
  { value: 'black', labelKey: 'Black' },
  { value: 'white', labelKey: 'White' },
  { value: 'latino', labelKey: 'Latino' },
  { value: 'middle_eastern', labelKey: 'Middle Eastern' },
  { value: 'south_asian', labelKey: 'South Asian' },
  { value: 'mixed', labelKey: 'Mixed' },
]

/** Scene list bounds; the labels below carry the numbers so a change stays in one place. */
export const AVATAR_SCENES_MAX = 8
export const AVATAR_SCENE_MAX_LENGTH = 24
export const AVATAR_SCENES_MESSAGE_TOO_MANY = 'At most 8 scenes'
export const AVATAR_SCENES_MESSAGE_TOO_LONG =
  'Each scene must be 24 characters or fewer'

export const AVATAR_NAME_MAX_LENGTH = 191
export const AVATAR_DESCRIPTION_MAX_LENGTH = 2000
export const AVATAR_VOICE_ID_MAX_LENGTH = 191
export const AVATAR_SORT_ORDER_MAX = 1_000_000

/** Form field name of one picture; identical to the API payload field. */
export type AvatarImageFieldName =
  | 'imageUrl'
  | 'fullBodyUrl'
  | 'fourViewUrl'
  | 'expressionUrl'

export interface AvatarImageFieldSpec {
  name: AvatarImageFieldName
  /** i18n key the UI renders. */
  labelKey: string
  /** Tailwind aspect ratio of the preview box, matching the picture's shape. */
  aspectClassName: string
}

/**
 * The four pictures a persona carries, in the order the form and the table show
 * them: 封面图 (the one an application lists the persona with) followed by the
 * three reference pictures. Mirrors the backend's four `*Url` fields.
 */
export const AVATAR_IMAGE_FIELDS: AvatarImageFieldSpec[] = [
  { name: 'imageUrl', labelKey: 'Cover Image', aspectClassName: 'aspect-3/4' },
  {
    name: 'fullBodyUrl',
    labelKey: 'Full-body Photo',
    aspectClassName: 'aspect-3/4',
  },
  { name: 'fourViewUrl', labelKey: 'Four Views', aspectClassName: 'aspect-4/3' },
  {
    name: 'expressionUrl',
    labelKey: 'Expression Sheet',
    aspectClassName: 'aspect-4/3',
  },
]

/**
 * One picture field. The API accepts the same two shapes for all four: a
 * gateway-served /uploads/... path or an absolute http(s) URL.
 */
function avatarImageField() {
  return z
    .string()
    .trim()
    .refine(
      (value) =>
        value === '' ||
        (value.startsWith('/') && !value.startsWith('//')) ||
        /^https?:\/\//i.test(value),
      'Image must be an /uploads path or an http(s) URL'
    )
}

/** Separators accepted in the scene editor, matching the backend parser. */
const SCENE_SEPARATORS = /[,，、;；|]/

/**
 * Label of a stored vocabulary value, or null when nothing is set / the value is
 * unknown (the table then shows a placeholder instead of a raw identifier).
 */
export function avatarOptionLabelKey(
  options: AvatarOption[],
  value: string
): string | null {
  if (value === '') return null
  const option = options.find((item) => item.value === value)
  return option ? option.labelKey : value
}

/** Scenes as the editor shows them: one comma-separated line. */
export function formatSceneList(scenes: string[]): string {
  return scenes.join(', ')
}

/**
 * Parses the editor's line into the API array, dropping blanks and duplicates in
 * the order the operator typed them.
 */
export function parseSceneList(text: string): string[] {
  const seen = new Set<string>()
  const scenes: string[] = []
  for (const part of text.split(SCENE_SEPARATORS)) {
    const scene = part.trim()
    if (scene === '' || seen.has(scene)) continue
    seen.add(scene)
    scenes.push(scene)
  }
  return scenes
}

/**
 * Persona form contract. Each of the four pictures accepts the same two shapes
 * the API does: a gateway-served /uploads/... path or an absolute http(s) URL.
 */
export const avatarFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Avatar name is required')
    .max(
      AVATAR_NAME_MAX_LENGTH,
      'Avatar name is too long (max 191 characters)'
    ),
  description: z
    .string()
    .max(
      AVATAR_DESCRIPTION_MAX_LENGTH,
      'Introduction is too long (max 2000 characters)'
    ),
  imageUrl: avatarImageField(),
  fullBodyUrl: avatarImageField(),
  fourViewUrl: avatarImageField(),
  expressionUrl: avatarImageField(),
  // The controlled vocabularies are validated by the API against their own list;
  // the dialog only offers values from it, so the form just carries them.
  gender: z.string(),
  ageRange: z.string(),
  race: z.string(),
  // Edited as one comma-separated line, submitted as the API's array.
  scenes: z
    .string()
    .refine(
      (value) => parseSceneList(value).length <= AVATAR_SCENES_MAX,
      AVATAR_SCENES_MESSAGE_TOO_MANY
    )
    .refine(
      (value) =>
        parseSceneList(value).every(
          (scene) => [...scene].length <= AVATAR_SCENE_MAX_LENGTH
        ),
      AVATAR_SCENES_MESSAGE_TOO_LONG
    ),
  // A voice identifier is free text: a persona may name a provider voice that is
  // not in the 音色广场 catalog yet, and the API accepts it.
  voiceId: z
    .string()
    .trim()
    .max(
      AVATAR_VOICE_ID_MAX_LENGTH,
      'Voice id is too long (max 191 characters)'
    ),
  enabled: z.boolean(),
  sortOrder: z
    .number()
    .int('Sort order must be a whole number')
    .min(-AVATAR_SORT_ORDER_MAX, 'Sort order is out of range')
    .max(AVATAR_SORT_ORDER_MAX, 'Sort order is out of range'),
})

export type AvatarFormValues = z.infer<typeof avatarFormSchema>

/** Empty state of the create dialog: a new persona is published to the plaza. */
export const EMPTY_AVATAR_FORM: AvatarFormValues = {
  name: '',
  description: '',
  imageUrl: '',
  fullBodyUrl: '',
  fourViewUrl: '',
  expressionUrl: '',
  gender: '',
  ageRange: '',
  race: '',
  scenes: '',
  voiceId: '',
  enabled: true,
  sortOrder: 0,
}

/** Form state pre-filled from a stored row. */
export function avatarToFormValues(avatar: AvatarView): AvatarFormValues {
  return {
    name: avatar.name,
    description: avatar.description,
    imageUrl: avatar.imageUrl,
    fullBodyUrl: avatar.fullBodyUrl,
    fourViewUrl: avatar.fourViewUrl,
    expressionUrl: avatar.expressionUrl,
    gender: avatar.gender,
    ageRange: avatar.ageRange,
    race: avatar.race,
    scenes: formatSceneList(avatar.scenes),
    voiceId: avatar.voiceId,
    enabled: avatar.enabled,
    sortOrder: avatar.sortOrder,
  }
}
