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
 * Task-status helpers for the RunningHub app center.
 *
 * Two host statuses mean "the run can still be cancelled", and they read very
 * differently to the user:
 *
 * - `QUEUED` — accepted while every channel of the site was at its concurrency
 *   cap; it has not reached RunningHub yet, so cancelling it is immediate and
 *   free.
 * - `IN_PROGRESS` / `SUBMITTED` / `NOT_START` — already running upstream.
 *
 * Everything else is terminal (`SUCCESS` / `FAILURE`) and must not offer a
 * cancel affordance.
 */

export type RhCancelKind = 'queued' | 'running' | null

/** The i18n key shown as the status badge for a raw task status. */
export function rhStatusKey(status: string): string {
  const normalized = (status || '').toLowerCase()
  if (normalized === 'success' || normalized === 'done' || normalized === 'succeeded') {
    return 'Success'
  }
  if (normalized === 'failure' || normalized === 'failed' || normalized === 'canceled') {
    return 'Failed'
  }
  if (normalized === 'queued') {
    return 'Queued'
  }
  return 'In progress'
}

/**
 * How a task can be cancelled, or null when the record is terminal and must not
 * offer the affordance.
 */
export function rhCancelKind(status: string): RhCancelKind {
  const normalized = (status || '').toLowerCase()
  if (normalized === 'queued') {
    return 'queued'
  }
  if (
    normalized === 'in_progress' ||
    normalized === 'submitted' ||
    normalized === 'not_start'
  ) {
    return 'running'
  }
  return null
}
