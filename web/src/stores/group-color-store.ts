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
import { create } from 'zustand'

interface GroupColorState {
  /** Explicit group → HEX color map configured in Group Pricing. */
  colors: Record<string, string>
  setColors: (colors: Record<string, string>) => void
}

/**
 * Global store for admin-configured group colors, populated from the public
 * `/api/status` payload. All group badges read from here and fall back to the
 * stable hash color when a group has no explicit color.
 */
export const useGroupColorStore = create<GroupColorState>((set) => ({
  colors: {},
  setColors: (colors) => set({ colors }),
}))

/** Returns the explicit HEX color for a group, or undefined when unset. */
export function useGroupColor(group?: string | null): string | undefined {
  const key = group?.trim()
  // Hook is always called; an empty key selects undefined so components with
  // no group name (auto/empty) keep their existing neutral styling.
  return useGroupColorStore((state) =>
    key ? state.colors[key] : undefined
  )
}
