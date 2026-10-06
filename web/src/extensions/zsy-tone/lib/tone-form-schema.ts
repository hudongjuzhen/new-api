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

import type { ToneView } from '../api'
import {
  formatSceneList,
  parseSceneList,
  TONE_LANGUAGE_PATTERN,
  TONE_LIMITS_FALLBACK,
  type ToneLimits,
} from './tone-fields'

/**
 * Tone form contract. The bounds come from the published standard's `limits`
 * (zsy/tone/standard.go), so the dialog refuses exactly what the API would
 * reject without keeping a second copy of every cap.
 *
 * The schema is a factory rather than a constant because `limits` arrives with
 * the standard request: a caller that has the response passes its numbers, one
 * that does not gets the mirror. Message wording still carries the number, so a
 * raised cap degrades to an informative English sentence instead of a stale one
 * that quotes a limit the backend no longer enforces.
 *
 * Two fields are required, and both are required by the backend rather than by
 * taste: `name` (the unique key an upsert matches on) and `prompt` — the standard
 * is explicit that a tone without an instruction is an empty style for every
 * consumer, and the validator rejects it (`文风提示词不能为空`).
 */
export function createToneFormSchema(limits: ToneLimits = TONE_LIMITS_FALLBACK) {
  return z.object({
    name: z
      .string()
      .trim()
      .min(1, 'Tone name is required')
      .max(
        limits.name,
        `Tone name is too long (max ${limits.name} characters)`
      ),
    description: z
      .string()
      .max(
        limits.description,
        `Introduction is too long (max ${limits.description} characters)`
      ),
    // The instruction downstream models follow; the largest field on the form.
    prompt: z
      .string()
      .trim()
      .min(1, 'Tone prompt is required')
      .max(
        limits.prompt,
        `Tone prompt is too long (max ${limits.prompt} characters)`
      ),
    // The controlled vocabularies are validated by the API against its own list;
    // the dialog only offers values from it, so the form just carries them.
    category: z.string(),
    tone: z.string(),
    // Edited as one comma-separated line, submitted as the API's array.
    scenes: z
      .string()
      .refine(
        (value) => parseSceneList(value).length <= limits.scenes,
        `At most ${limits.scenes} scenes`
      )
      .refine(
        (value) =>
          parseSceneList(value).every(
            (scene) => [...scene].length <= limits.sceneLength
          ),
        `Each scene must be ${limits.sceneLength} characters or fewer`
      ),
    // Optional short tag; the backend normalizes it to lower case and accepts any
    // tag a consumer uses, so the form checks the shape and the cap.
    language: z
      .string()
      .trim()
      .max(
        limits.language,
        `Language tag is too long (max ${limits.language} characters)`
      )
      .refine(
        (value) =>
          value === '' || TONE_LANGUAGE_PATTERN.test(value.toLowerCase()),
        'Invalid language code'
      ),
    sampleInput: z
      .string()
      .max(
        limits.sample,
        `Sample input is too long (max ${limits.sample} characters)`
      ),
    sampleOutput: z
      .string()
      .max(
        limits.sample,
        `Sample output is too long (max ${limits.sample} characters)`
      ),
    enabled: z.boolean(),
    sortOrder: z
      .number()
      .int('Sort order must be a whole number')
      .min(-limits.sortOrderAbs, 'Sort order is out of range')
      .max(limits.sortOrderAbs, 'Sort order is out of range'),
  })
}

export type ToneFormValues = z.infer<ReturnType<typeof createToneFormSchema>>

/**
 * Empty state of the create dialog: a new tone is published to the plaza.
 *
 * `tone` starts at `plain` rather than empty. The standard draws a distinction
 * between the two — empty means "not classified yet", while `plain` means "it is
 * meant to be plain" — and says the latter is the more useful answer for a tone
 * whose flavour is undecided. `plain` is a published value, which the standard
 * promises never to rename or remove, so using it as a default cannot rot.
 */
export const EMPTY_TONE_FORM: ToneFormValues = {
  name: '',
  description: '',
  prompt: '',
  category: '',
  tone: 'plain',
  language: '',
  scenes: '',
  sampleInput: '',
  sampleOutput: '',
  enabled: true,
  sortOrder: 0,
}

/** Form state pre-filled from a stored row. */
export function toneToFormValues(tone: ToneView): ToneFormValues {
  return {
    name: tone.name,
    description: tone.description,
    prompt: tone.prompt,
    category: tone.category,
    tone: tone.tone,
    language: tone.language,
    scenes: formatSceneList(tone.scenes),
    sampleInput: tone.sampleInput,
    sampleOutput: tone.sampleOutput,
    enabled: tone.enabled,
    sortOrder: tone.sortOrder,
  }
}

/**
 * Whether a row carries anything worth opening the sample dialog for. A tone
 * with both halves empty shows no 「示例」 button at all — the same rule the voice
 * plaza applies to a voice with no audio.
 */
export function hasToneSample(tone: {
  sampleInput: string
  sampleOutput: string
}): boolean {
  return tone.sampleInput.trim() !== '' || tone.sampleOutput.trim() !== ''
}
