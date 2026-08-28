/*
Copyright (C) 2023-2026 QuantumNous

Aggregated extension i18n resource contributions.

Each plugin keeps its translations under
`src/extensions/<plugin>/locales/<lang>.json` and registers them by calling
`registerExtensionLocales` from its own `index.ts`. The host's i18n
initialisation (src/i18n/config.ts) deep-merges `EXTENSION_LOCALES` into the
base resources, so plugins can ship strings without touching the core
`src/i18n/locales/*.json` files.

Keys should be the canonical English phrase (flat) so they match the core
convention described in AGENTS.md §3.1 — plugin translations with identical
keys to the core *overwrite* them, which is useful for runtime branding but
otherwise should be avoided.
*/

export type ExtensionLocaleResource = Record<string, string>

export interface ExtensionLocales {
  en?: ExtensionLocaleResource
  zhCN?: ExtensionLocaleResource
  zhTW?: ExtensionLocaleResource
  fr?: ExtensionLocaleResource
  ru?: ExtensionLocaleResource
  ja?: ExtensionLocaleResource
  vi?: ExtensionLocaleResource
}

export const EXTENSION_LOCALES: Required<ExtensionLocales> = {
  en: {},
  zhCN: {},
  zhTW: {},
  fr: {},
  ru: {},
  ja: {},
  vi: {},
}

function deepMerge(
  target: ExtensionLocaleResource,
  source: ExtensionLocaleResource | undefined,
): void {
  if (!source) return
  for (const k of Object.keys(source)) {
    target[k] = source[k]
  }
}

/**
 * Register a plugin's translation dictionaries into the extension registry.
 *
 * Missing locale dicts are ignored so a plugin only has to ship the subset of
 * languages it actually translates. Later registrations overwrite earlier
 * entries for identical keys.
 */
export function registerExtensionLocales(locales: ExtensionLocales): void {
  deepMerge(EXTENSION_LOCALES.en, locales.en)
  deepMerge(EXTENSION_LOCALES.zhCN, locales.zhCN)
  deepMerge(EXTENSION_LOCALES.zhTW, locales.zhTW)
  deepMerge(EXTENSION_LOCALES.fr, locales.fr)
  deepMerge(EXTENSION_LOCALES.ru, locales.ru)
  deepMerge(EXTENSION_LOCALES.ja, locales.ja)
  deepMerge(EXTENSION_LOCALES.vi, locales.vi)
}
