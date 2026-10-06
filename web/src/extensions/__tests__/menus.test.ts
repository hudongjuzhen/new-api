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
import { describe, expect, test } from 'vitest'

// Side-effect import: this is what runs each plugin's registration, exactly as
// the app does at startup.
import '@/extensions'
import { EXT_MENU_GROUPS } from '@/extensions/menus'
import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { ROLE } from '@/lib/roles'

const LOCALES: Record<string, Record<string, string>> = {
  en: en.translation,
  zh: zh.translation,
  'zh-TW': zhTW.translation,
  fr: fr.translation,
  ja: ja.translation,
  ru: ru.translation,
  vi: vi.translation,
}

/**
 * Every title an operator can read in the sidebar, at any depth.
 *
 * The walk is recursive on purpose: the host translates a group's title and the
 * titles of that group's own items, so a title nested one level deeper than that
 * would render as its raw English key. Collecting every depth is what makes the
 * locale assertion below catch that.
 */
function menuTitles(): string[] {
  const titles: string[] = []

  const walk = (nodes: { title: string; items?: unknown }[]) => {
    for (const node of nodes) {
      titles.push(node.title)
      if (Array.isArray(node.items)) {
        walk(node.items as { title: string; items?: unknown }[])
      }
    }
  }

  for (const group of EXT_MENU_GROUPS) {
    titles.push(group.title)
    walk(group.items as { title: string; items?: unknown }[])
  }
  return titles
}

describe('extension sidebar menus', () => {
  test('register the shared catalog group with one flat entry per catalog', () => {
    const group = EXT_MENU_GROUPS.find((entry) => entry.id === 'zsy-voice')

    expect(group).toBeDefined()
    expect(group?.title).toBe('Public Data')
    expect(
      group?.items.map((item) => ({ title: item.title, url: item.url }))
    ).toEqual([
      { title: 'Voice Plaza', url: '/voice-plaza' },
      { title: 'Avatar Plaza', url: '/avatar-plaza' },
      // zsy-tone registers itself into this same group rather than opening a
      // second "Public Data" entry (see zsy-tone/index.ts), so a third catalog
      // lands here. The list stays literal on purpose: it is what makes a
      // silently dropped or reordered entry fail instead of just looking fine.
      { title: 'Tone Plaza', url: '/tone-plaza' },
    ])
  })

  test('keep the plugin pages admin-only', () => {
    const group = EXT_MENU_GROUPS.find((entry) => entry.id === 'zsy-voice')

    for (const item of group?.items ?? []) {
      // The pages manage unpublished rows too, so they must not be offered to a
      // regular user: the route guard would only redirect them to /403.
      expect(item.requiredRole).toBe(ROLE.ADMIN)
    }
  })

  test('translate every menu title in all supported locales', () => {
    // The host only renders a group's title and its own items through t(), so
    // every registered title must exist as a locale key — a title missing from
    // the locale files would show up as its raw English key in the sidebar.
    for (const [locale, translation] of Object.entries(LOCALES)) {
      for (const title of menuTitles()) {
        expect(
          translation[title],
          `menu title ${JSON.stringify(title)} is missing from ${locale}.json`
        ).toBeDefined()
      }
    }
  })
})
