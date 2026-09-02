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
export type SemanticColor =
  | 'blue'
  | 'green'
  | 'cyan'
  | 'purple'
  | 'pink'
  | 'red'
  | 'orange'
  | 'amber'
  | 'yellow'
  | 'lime'
  | 'light-green'
  | 'teal'
  | 'light-blue'
  | 'indigo'
  | 'violet'
  | 'grey'
  | 'slate'

export const colorToBgClass: Record<SemanticColor, string> = {
  blue: 'bg-blue-500',
  green: 'bg-green-500',
  cyan: 'bg-cyan-500',
  purple: 'bg-purple-500',
  pink: 'bg-pink-500',
  red: 'bg-red-500',
  orange: 'bg-orange-500',
  amber: 'bg-amber-500',
  yellow: 'bg-yellow-500',
  lime: 'bg-lime-500',
  'light-green': 'bg-green-400',
  teal: 'bg-teal-500',
  'light-blue': 'bg-sky-400',
  indigo: 'bg-indigo-500',
  violet: 'bg-violet-500',
  grey: 'bg-gray-400',
  slate: 'bg-slate-500',
}

export const avatarColorMap: Record<SemanticColor, string> = {
  blue: 'bg-chart-1/10 text-chart-1',
  green: 'bg-success/10 text-success',
  cyan: 'bg-chart-2/10 text-chart-2',
  purple: 'bg-chart-4/10 text-chart-4',
  pink: 'bg-chart-5/10 text-chart-5',
  red: 'bg-destructive/10 text-destructive',
  orange: 'bg-warning/10 text-warning',
  amber: 'bg-warning/10 text-warning',
  yellow: 'bg-warning/10 text-warning',
  lime: 'bg-chart-3/10 text-chart-3',
  'light-green': 'bg-success/10 text-success',
  teal: 'bg-chart-2/10 text-chart-2',
  'light-blue': 'bg-info/10 text-info',
  indigo: 'bg-chart-1/10 text-chart-1',
  violet: 'bg-chart-4/10 text-chart-4',
  grey: 'bg-muted text-muted-foreground',
  slate: 'bg-muted text-muted-foreground',
}

export function getAvatarColorClass(name: string): string {
  return avatarColorMap[stringToColor(name)]
}

export function getBgColorClass(color?: string): string {
  if (!color) return colorToBgClass.blue
  return (
    (colorToBgClass as Record<string, string>)[color] || colorToBgClass.blue
  )
}

/**
 * Chart color palette - Modern gradient colors compatible with light/dark themes
 * Uses HSL format for better theme adaptation
 */
export const CHART_COLORS = [
  'hsl(217, 91%, 60%)', // blue
  'hsl(142, 76%, 36%)', // green
  'hsl(38, 92%, 50%)', // amber
  'hsl(258, 90%, 66%)', // violet
  'hsl(330, 81%, 60%)', // pink
  'hsl(189, 94%, 43%)', // cyan
  'hsl(25, 95%, 53%)', // orange
  'hsl(239, 84%, 67%)', // indigo
  'hsl(173, 80%, 40%)', // teal
  'hsl(271, 91%, 65%)', // purple
  'hsl(199, 89%, 48%)', // sky
  'hsl(280, 65%, 60%)', // fuchsia
] as const

/**
 * Get a chart color by index (cycles through the palette)
 */
export function getChartColor(index: number): string {
  return CHART_COLORS[index % CHART_COLORS.length]
}

/**
 * Announcement status types
 */
export type AnnouncementType =
  | 'default'
  | 'ongoing'
  | 'success'
  | 'warning'
  | 'error'

/**
 * Announcement status color mapping
 */
export const ANNOUNCEMENT_TYPE_COLORS: Record<AnnouncementType, string> = {
  default: 'bg-neutral',
  ongoing: 'bg-info',
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-destructive',
}

/**
 * Get announcement status color class
 */
export function getAnnouncementColorClass(type?: string): string {
  const validType = (type || 'default') as AnnouncementType
  return ANNOUNCEMENT_TYPE_COLORS[validType] || ANNOUNCEMENT_TYPE_COLORS.default
}

/**
 * Semantic colors for tags and badges
 */
const TAG_COLORS = [
  'amber',
  'blue',
  'cyan',
  'green',
  'grey',
  'indigo',
  'light-blue',
  'lime',
  'orange',
  'pink',
  'purple',
  'red',
  'teal',
  'violet',
  'yellow',
] as const

/**
 * Convert string to a stable semantic color
 * Used for model tags, group badges, user avatars, etc.
 * Same string always returns the same color
 *
 * @param str - Input string (model name, group name, username, etc.)
 * @returns Semantic color name from TAG_COLORS
 *
 * @example
 * stringToColor('gpt-4') // 'blue'
 * stringToColor('claude-3') // 'purple'
 * stringToColor('default') // 'green'
 */
export function stringToColor(str: string): SemanticColor {
  let sum = 0
  for (let i = 0; i < str.length; i++) {
    sum += str.charCodeAt(i)
  }
  const index = sum % TAG_COLORS.length
  return TAG_COLORS[index]
}

// ----------------------------------------------------------------------------
// Group color presets
//
// Admin-picked HEX colors for pricing groups. The presets mirror the semantic
// palette above (blue/green/cyan/purple/... /slate) so a picked color blends
// with the rest of the UI. Explicit colors are rendered via inline styles
// (Tailwind cannot statically scan runtime HEX values).
// ----------------------------------------------------------------------------

export const GROUP_COLOR_PRESETS = [
  '#3B82F6', // blue
  '#0EA5E9', // light-blue / sky
  '#06B6D4', // cyan
  '#10B981', // teal
  '#22C55E', // green
  '#84CC16', // lime
  '#F59E0B', // amber
  '#F97316', // orange
  '#EF4444', // red
  '#EC4899', // pink
  '#8B5CF6', // violet
  '#A855F7', // purple
  '#6366F1', // indigo
  '#6B7280', // grey
  '#64748B', // slate
] as const

/**
 * Convert a `#RRGGBB` hex color to an `rgba(r, g, b, a)` string. Falls back to
 * transparent when the input is not a valid 6-digit hex, so callers can safely
 * tint arbitrary user-provided colors without breaking the layout.
 */
export function hexToRgba(hex: string, alpha: number): string {
  const match = /^#?([0-9a-fA-F]{6})$/.exec(hex.trim())
  if (!match) return `rgba(0, 0, 0, 0)`
  const value = Number.parseInt(match[1], 16)
  const r = (value >> 16) & 0xff
  const g = (value >> 8) & 0xff
  const b = value & 0xff
  return `rgba(${r}, ${g}, ${b}, ${alpha})`
}

// ----------------------------------------------------------------------------
// Group tone variants
//
// Each group tag gets a stable hash color (see avatarColorMap above). The
// group's discount capsule must match that color family, using depth/boldness
// only to signal billing state:
//   - discount (< 1):  stronger fill + bold, highlights the deal
//   - normal (= 1):    faint fill + muted text, quietly the standard price
//   - premium (> 1):   deepest fill + bold, highlights the surcharge
// All class names are hard-coded below (no runtime interpolation) so Tailwind
// can statically scan and generate them.
// ----------------------------------------------------------------------------

type GroupToneClasses = {
  discount: string
  normal: string
  premium: string
}

/** Maps a semantic color name to the shared theme variable used by its group. */
const semanticToToneVar: Record<SemanticColor, string> = {
  blue: 'chart-1',
  indigo: 'chart-1',
  'light-blue': 'info',
  green: 'success',
  'light-green': 'success',
  lime: 'chart-3',
  cyan: 'chart-2',
  teal: 'chart-2',
  purple: 'chart-4',
  violet: 'chart-4',
  pink: 'chart-5',
  red: 'destructive',
  orange: 'warning',
  amber: 'warning',
  yellow: 'warning',
  grey: 'grey',
  slate: 'grey',
}

const GROUP_TONE_TABLE: Record<string, GroupToneClasses> = {
  'chart-1': {
    discount: 'bg-chart-1/25 text-chart-1 font-bold',
    normal: 'bg-chart-1/5 text-chart-1/60',
    premium: 'bg-chart-1/30 text-chart-1 font-bold',
  },
  'chart-2': {
    discount: 'bg-chart-2/25 text-chart-2 font-bold',
    normal: 'bg-chart-2/5 text-chart-2/60',
    premium: 'bg-chart-2/30 text-chart-2 font-bold',
  },
  'chart-3': {
    discount: 'bg-chart-3/25 text-chart-3 font-bold',
    normal: 'bg-chart-3/5 text-chart-3/60',
    premium: 'bg-chart-3/30 text-chart-3 font-bold',
  },
  'chart-4': {
    discount: 'bg-chart-4/25 text-chart-4 font-bold',
    normal: 'bg-chart-4/5 text-chart-4/60',
    premium: 'bg-chart-4/30 text-chart-4 font-bold',
  },
  'chart-5': {
    discount: 'bg-chart-5/25 text-chart-5 font-bold',
    normal: 'bg-chart-5/5 text-chart-5/60',
    premium: 'bg-chart-5/30 text-chart-5 font-bold',
  },
  success: {
    discount: 'bg-success/25 text-success font-bold',
    normal: 'bg-success/5 text-success/60',
    premium: 'bg-success/30 text-success font-bold',
  },
  info: {
    discount: 'bg-info/25 text-info font-bold',
    normal: 'bg-info/5 text-info/60',
    premium: 'bg-info/30 text-info font-bold',
  },
  warning: {
    discount: 'bg-warning/25 text-warning font-bold',
    normal: 'bg-warning/5 text-warning/60',
    premium: 'bg-warning/30 text-warning font-bold',
  },
  destructive: {
    discount: 'bg-destructive/25 text-destructive font-bold',
    normal: 'bg-destructive/5 text-destructive/60',
    premium: 'bg-destructive/30 text-destructive font-bold',
  },
  grey: {
    discount: 'bg-foreground/15 text-foreground font-bold',
    normal: 'bg-muted/60 text-muted-foreground/70',
    premium: 'bg-foreground/20 text-foreground font-bold',
  },
}

/**
 * Discount capsule classes for a group, matched to the group's tag color
 * family. Depth/boldness encodes billing state (see GROUP_TONE_TABLE).
 */
export function getGroupDiscountClassName(
  group: string,
  ratio: number
): string {
  const color = stringToColor(group)
  const tone = GROUP_TONE_TABLE[semanticToToneVar[color]]
  if (ratio < 1) return tone.discount
  if (ratio > 1) return tone.premium
  return tone.normal
}
