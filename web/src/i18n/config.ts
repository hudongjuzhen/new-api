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
import i18n from 'i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import { initReactI18next } from 'react-i18next'

import { EXTENSION_LOCALES } from '@/extensions/locales'
import { convertDetectedLanguage } from './languages'
import en from './locales/en.json'
import fr from './locales/fr.json'
import ja from './locales/ja.json'
import ru from './locales/ru.json'
import vi from './locales/vi.json'
import zhTW from './locales/zh-TW.json'
import zhCN from './locales/zh.json'

function mergeExt<LangCode extends keyof typeof EXTENSION_LOCALES>(
  base: Record<string, unknown>,
  code: LangCode,
): Record<string, unknown> {
  const ext = EXTENSION_LOCALES[code]
  if (!ext || Object.keys(ext).length === 0) return base
  const baseAny = base as Record<string, unknown>
  const baseTx = (baseAny.translation ?? {}) as Record<string, unknown>
  return {
    ...baseAny,
    translation: { ...baseTx, ...(ext as Record<string, unknown>) },
  }
}

export const resources = {
  en: mergeExt(en as Record<string, unknown>, 'en'),
  zhCN: mergeExt(zhCN as Record<string, unknown>, 'zhCN'),
  fr: mergeExt(fr as Record<string, unknown>, 'fr'),
  ru: mergeExt(ru as Record<string, unknown>, 'ru'),
  ja: mergeExt(ja as Record<string, unknown>, 'ja'),
  vi: mergeExt(vi as Record<string, unknown>, 'vi'),
  zhTW: mergeExt(zhTW as Record<string, unknown>, 'zhTW'),
} as const

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources,
    fallbackLng: 'en',
    supportedLngs: ['en', 'zhCN', 'fr', 'ru', 'ja', 'vi', 'zhTW'],
    load: 'currentOnly',
    nsSeparator: false, // Allow literal colons in keys (e.g., URLs, labels)
    debug: import.meta.env.DEV,
    interpolation: {
      escapeValue: false, // not needed for react as it escapes by default
    },
    detection: {
      order: ['localStorage', 'navigator'],
      caches: ['localStorage'],
      // Browsers report `zh-CN`/`zh-TW`/`zh`; map them onto our `zhCN`/`zhTW`
      // codes (non-Chinese codes pass through for normal supportedLngs matching).
      convertDetectedLanguage,
    },
  })

export default i18n
