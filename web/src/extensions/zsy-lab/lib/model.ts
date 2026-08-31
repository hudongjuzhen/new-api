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
import type { PricingModel } from '@/features/pricing/types'

/**
 * Models whose primary interaction is image generation/editing get the
 * dedicated image playground in the lab. Detection follows the channel
 * registry first (`image-generation` endpoint capability) and falls back to
 * the well-known gpt-image family naming, since endpoint metadata can be
 * absent on aggregated channels.
 */
export function isImageGenModel(
  model: Pick<PricingModel, 'model_name' | 'supported_endpoint_types'> | null
    | undefined
): boolean {
  if (!model) return false
  if (model.supported_endpoint_types?.includes('image-generation')) {
    return true
  }
  return /gpt-image/i.test(model.model_name)
}
