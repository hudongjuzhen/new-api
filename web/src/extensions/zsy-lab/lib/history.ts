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
export interface LabHistoryEntry {
  id: string
  model: string
  prompt: string
  status: 'success' | 'error'
  createdAt: number
  durationMs?: number
  error?: string
}

const LAB_HISTORY_KEY = 'zsy_lab_history'
export const LAB_HISTORY_UPDATED_EVENT = 'zsy-lab-history-updated'
const LAB_HISTORY_TTL_MS = 24 * 60 * 60 * 1000
const LAB_HISTORY_MAX_ENTRIES = 50

function pruneEntries(entries: LabHistoryEntry[]): LabHistoryEntry[] {
  const oldestAllowed = Date.now() - LAB_HISTORY_TTL_MS
  return entries
    .filter((entry) => entry.createdAt >= oldestAllowed)
    .slice(0, LAB_HISTORY_MAX_ENTRIES)
}

export function loadLabHistory(): LabHistoryEntry[] {
  try {
    const raw = window.localStorage.getItem(LAB_HISTORY_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return pruneEntries(parsed as LabHistoryEntry[])
  } catch {
    return []
  }
}

export function appendLabHistory(entry: LabHistoryEntry): void {
  const entries = loadLabHistory()
  const next = pruneEntries([entry, ...entries])
  try {
    window.localStorage.setItem(LAB_HISTORY_KEY, JSON.stringify(next))
    window.dispatchEvent(new Event(LAB_HISTORY_UPDATED_EVENT))
  } catch {
    /* storage full or unavailable: history is best-effort */
  }
}
