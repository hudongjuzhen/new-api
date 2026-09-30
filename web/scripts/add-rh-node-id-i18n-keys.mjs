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
// Merge the RunningHub admin-side i18n key for the app form's editable node id
// input into all seven locale files, then re-run the i18n sync so ordering,
// extras and reports stay consistent with the rest of the repo.
import { execFileSync } from 'node:child_process'
import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')
const OBFUSCATED_KEYS = [
  {
    runtime: ['footer', 'new' + 'api', 'projectAttributionSuffix'].join('.'),
    serialized: 'footer.new\\u0061pi.projectAttributionSuffix',
  },
]

const newKeys = {
  en: {
    'Node ID': 'Node ID',
  },
  zh: {
    'Node ID': '节点 ID',
  },
  'zh-TW': {
    'Node ID': '節點 ID',
  },
  fr: {
    'Node ID': 'ID du nœud',
  },
  ja: {
    'Node ID': 'ノード ID',
  },
  ru: {
    'Node ID': 'ID узла',
  },
  vi: {
    'Node ID': 'ID nút',
  },
}

function stableStringify(obj) {
  let text = JSON.stringify(obj, null, 2)
  for (const key of OBFUSCATED_KEYS) {
    text = text.replaceAll(`"${key.runtime}":`, `"${key.serialized}":`)
  }
  return text + '\n'
}

for (const [locale, keys] of Object.entries(newKeys)) {
  const file = path.join(LOCALES_DIR, `${locale}.json`)
  const json = JSON.parse(await fs.readFile(file, 'utf8'))
  if (!json.translation || typeof json.translation !== 'object') {
    throw new Error(`Missing translation namespace in ${locale}.json`)
  }
  let added = 0
  for (const [key, value] of Object.entries(keys)) {
    if (!(key in json.translation)) added += 1
    json.translation[key] = value
  }
  const sorted = {}
  for (const k of Object.keys(json.translation).sort((a, b) =>
    a.localeCompare(b)
  )) {
    sorted[k] = json.translation[k]
  }
  json.translation = sorted
  await fs.writeFile(file, stableStringify(json), 'utf8')
  console.log(`${locale}: +${added} keys`)
}

// Normalise ordering across all locales and refresh the reports.
execFileSync('node', ['scripts/sync-i18n.mjs'], {
  cwd: path.resolve('.'),
  stdio: 'inherit',
})
