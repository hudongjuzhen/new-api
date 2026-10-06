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
import { exportTones, type ToneListParams } from '../api'

/**
 * Downloads the tones matching the filters as a CSV file.
 *
 * The browser plumbing (object URL, anchor click, cleanup) has exactly one
 * implementation here rather than in the page component, which keeps the page
 * about the catalog and makes the download testable on its own.
 */
export async function downloadTonesCsv(params: ToneListParams): Promise<void> {
  const { blob, filename } = await exportTones(params)
  const url = URL.createObjectURL(blob)
  try {
    const link = document.createElement('a')
    link.href = url
    link.download = filename
    document.body.append(link)
    link.click()
    link.remove()
  } finally {
    URL.revokeObjectURL(url)
  }
}
