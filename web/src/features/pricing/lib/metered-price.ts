/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { PricingModel } from '../types'

/** Billing mode the backend sets for usage-metered models. */
const METERED_BILLING_MODE = 'metered'

/**
 * Check whether a model bills by the amount of output it produced rather than by
 * a flat per-request fee.
 *
 * Such a model still carries a `model_price`, but that price is quoted per unit
 * of output (`metered_unit`), not per call — the actual charge follows the
 * duration the upstream reported. Showing it as "per request" would misstate the
 * price by the full output length.
 */
export function isMeteredPricingModel(model: PricingModel): boolean {
  return model.billing_mode === METERED_BILLING_MODE && Boolean(model.metered_unit)
}

/**
 * Translation key naming the unit on its own ("Minute"), for price summaries that
 * render a "/ unit" suffix: "$0.375 / Minute".
 *
 * Returns undefined for models that are not metered, letting callers fall back to
 * their existing per-request wording.
 */
export function getMeteredUnitNameKey(model: PricingModel): string | undefined {
  if (!isMeteredPricingModel(model)) return undefined
  if (model.metered_unit === 'minute') return 'Minute'
  if (model.metered_unit === 'second') return 'Second'
  return undefined
}

/**
 * Translation key for the unit as a rate ("Per minute"), for places that render a
 * bare noun phrase rather than a "/ unit" suffix: "$0.375" beside "Per minute".
 */
export function getMeteredRateLabelKey(model: PricingModel): string | undefined {
  if (!isMeteredPricingModel(model)) return undefined
  if (model.metered_unit === 'minute') return 'Per minute'
  if (model.metered_unit === 'second') return 'Per second'
  return undefined
}
