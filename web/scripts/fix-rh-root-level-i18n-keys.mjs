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
import fs from 'node:fs/promises'
import path from 'node:path'

/*
 * One-off repair for the RunningHub locale keys that an earlier migration wrote
 * as siblings of the `translation` object instead of inside it (commit
 * 3f247a56). The host reads `resources[lang].translation` (see src/i18n/config.ts)
 * and these locales are the only source for those keys, so the root-level copies
 * were dead data — and one of them, "Dynamic Billing", had no live copy at all,
 * which is why the billing-mode selector read English in every language.
 *
 * The script moves the missing value into `translation`, adds the translations
 * that never existed, then drops the root-level duplicates. Locale files are
 * edited by splicing lines, never by re-serializing the whole file: these files
 * carry an intentionally escaped project-attribution key, and a JSON.stringify
 * rewrite would normalize it (plus the file's own indentation) away.
 *
 * Run from web/: node scripts/fix-rh-root-level-i18n-keys.mjs
 */
const LOCALES_DIR = path.resolve('src/i18n/locales')

/**
 * "Dynamic Billing" had no copy inside `translation` in any locale. The wording
 * follows the sibling billing modes already in each file ("Per-Call Billing",
 * "Per-Second Billing").
 */
const dynamicBilling = {
  en: 'Dynamic Billing',
  zh: '动态计费',
  'zh-TW': '動態計費',
  fr: 'Facturation dynamique',
  ja: 'ダイナミック課金',
  ru: 'Динамическая оплата',
  vi: 'Thanh toán động',
}

/** Indentation of the `translation` block, taken from its first key line. */
function translationIndent(lines) {
  const start = lines.findIndex((line) => line.trim() === '"translation": {')
  if (start < 0) throw new Error('no translation block found')
  for (let i = start + 1; i < lines.length; i++) {
    const match = /^(\s*)"/.exec(lines[i])
    if (match) return match[1]
  }
  throw new Error('translation block is empty')
}

/** Insert `key` at the position the block's own key order puts it. */
function insertKey(lines, key, value, indent) {
  const line = `${indent}${JSON.stringify(key)}: ${JSON.stringify(value)},`
  let at = lines.length - 1
  for (let i = 0; i < lines.length; i++) {
    if (!lines[i].startsWith(`${indent}"`)) continue
    const match = /^\s*"((?:[^"\\]|\\.)*)":/.exec(lines[i])
    if (!match) continue
    if (JSON.parse(`"${match[1]}"`).localeCompare(key) > 0) {
      at = i
      break
    }
  }
  // The line an insertion lands after has to end with a comma; the last key of
  // a block does not carry one.
  const previous = lines[at - 1]?.trimEnd()
  if (previous && previous.endsWith('"')) {
    lines[at - 1] = `${lines[at - 1]},`
  }
  lines.splice(at, 0, line)
}

/** Drop the root-level siblings of `translation` and the comma they leave. */
function removeRootBlock(lines, rootKeys) {
  const closeAt = lines.findIndex((line) => /^ {2}\},?$/.test(line))
  if (closeAt < 0) throw new Error('no closing brace for the translation block')
  // The last element of a trailing-newline split is empty, so the root object's
  // own closing brace is the line before it.
  const endAt = lines.length - 2
  if (lines[endAt] !== '}') throw new Error('unexpected end of file')

  const stray = lines.slice(closeAt + 1, endAt)
  if (stray.length !== rootKeys.length) {
    throw new Error(
      `expected ${rootKeys.length} root-level keys, found ${stray.length} lines`
    )
  }
  lines.splice(closeAt + 1, stray.length)
  lines[closeAt] = '  }'
}

async function main() {
  let total = 0

  for (const locale of Object.keys(dynamicBilling)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const raw = await fs.readFile(filePath, 'utf8')
    const parsed = JSON.parse(raw)
    const rootKeys = Object.keys(parsed).filter((key) => key !== 'translation')
    if (rootKeys.length === 0) {
      console.log(`${locale}: already clean, 0 keys moved`)
      continue
    }

    const lines = raw.split('\n')
    const indent = translationIndent(lines)
    removeRootBlock(lines, rootKeys)

    const moved = []
    for (const key of rootKeys) {
      if (Object.prototype.hasOwnProperty.call(parsed.translation, key)) continue
      const value = dynamicBilling[locale] ?? parsed[key]
      if (typeof value !== 'string' || value === '') {
        throw new Error(`${locale}: no value for ${JSON.stringify(key)}`)
      }
      insertKey(lines, key, value, indent)
      moved.push(key)
    }

    await fs.writeFile(filePath, lines.join('\n'), 'utf8')
    console.log(
      `${locale}: ${rootKeys.length} root keys removed, ${moved.length} moved into translation` +
        (moved.length > 0 ? ` (${moved.join(', ')})` : '')
    )
    total += moved.length
  }

  console.log(`\nTotal: ${total} keys moved into translation`)
}

main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
