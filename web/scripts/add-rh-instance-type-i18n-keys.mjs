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
 * One-off migration: the RunningHub app form (/rh-apps) gained an instance-type
 * selector, so its label and help text need the seven locales.
 *
 * Run from web/: node scripts/add-rh-instance-type-i18n-keys.mjs
 *
 * The keys are spliced into each `translation` block as text rather than
 * re-serialized with JSON.stringify: these files carry an intentionally escaped
 * project-attribution key plus root-level extras that the sync tool keeps out of
 * `translation`, and a full rewrite would normalize both, turning a two-line
 * change into a whole-file diff. Insertion keeps each file's own key order
 * (localeCompare, the comparator scripts/sync-i18n.mjs sorts with).
 */
const LOCALES_DIR = path.resolve('src/i18n/locales')

// The three values themselves (default / plus / ultra) are upstream API tokens
// and stay untranslated — they are what the RH request body carries.
const newKeys = {
  en: {
    'Instance Type': 'Instance Type',
    'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)':
      'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)',
  },
  zh: {
    'Instance Type': '算力实例',
    'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)':
      '本应用使用的 RH 算力实例（default 24GB / plus 48GB / ultra 80GB 显存）',
  },
  'zh-TW': {
    'Instance Type': '算力執行個體',
    'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)':
      '本應用程式使用的 RH 算力執行個體（default 24GB / plus 48GB / ultra 80GB VRAM）',
  },
  fr: {
    'Instance Type': "Type d'instance",
    'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)':
      "Instance de calcul RH utilisée par cette application (default 24GB / plus 48GB / ultra 80GB de VRAM)",
  },
  ja: {
    'Instance Type': 'インスタンスタイプ',
    'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)':
      'このアプリが使用する RH の計算インスタンス（default 24GB / plus 48GB / ultra 80GB VRAM）',
  },
  ru: {
    'Instance Type': 'Тип инстанса',
    'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)':
      'Вычислительный инстанс RH для этого приложения (default 24GB / plus 48GB / ultra 80GB VRAM)',
  },
  vi: {
    'Instance Type': 'Loại instance',
    'The RH compute instance this app runs on (default 24GB / plus 48GB / ultra 80GB VRAM)':
      'Instance tính toán RH mà ứng dụng này chạy trên đó (default 24GB / plus 48GB / ultra 80GB VRAM)',
  },
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

/** Index the new key belongs at, i.e. before the first key sorting after it. */
function insertionIndex(lines, key, indent) {
  const prefix = `${indent}"`
  for (let i = 0; i < lines.length; i++) {
    if (!lines[i].startsWith(prefix)) continue
    const match = /^\s*"((?:[^"\\]|\\.)*)":/.exec(lines[i])
    if (!match) continue
    const existing = JSON.parse(`"${match[1]}"`)
    if (existing.localeCompare(key) > 0) return i
  }
  return lines.length - 1
}

async function main() {
  let total = 0

  for (const [locale, keys] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const raw = await fs.readFile(filePath, 'utf8')
    const parsed = JSON.parse(raw)
    const missing = Object.entries(keys).filter(
      ([key]) => !Object.prototype.hasOwnProperty.call(parsed.translation, key)
    )
    if (missing.length === 0) {
      console.log(`${locale}: already in place, 0 added`)
      continue
    }

    const lines = raw.split('\n')
    const indent = translationIndent(lines)
    for (const [key, value] of missing) {
      const line = `${indent}${JSON.stringify(key)}: ${JSON.stringify(value)},`
      const at = insertionIndex(lines, key, indent)
      // The row before an insertion has to end with a comma; the last key of
      // the block does not carry one.
      const previous = lines[at - 1]?.trimEnd()
      if (previous && previous.endsWith('"') && at > 0) {
        lines[at - 1] = `${lines[at - 1]},`
      }
      lines.splice(at, 0, line)
    }

    await fs.writeFile(filePath, lines.join('\n'), 'utf8')
    console.log(`${locale}: ${missing.length} translations applied`)
    total += missing.length
  }

  console.log(`\nTotal: ${total} translations applied`)
}

main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
