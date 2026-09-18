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
// Merge the RunningHub admin-side i18n keys for the numeric parameter bounds
// (Min/Max) and the per-second billing "seconds field" expression into all
// seven locale files, then re-run the i18n sync so ordering, extras and reports
// stay consistent with the rest of the repo.
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
    Max: 'Max',
    Min: 'Min',
    'Seconds Field': 'Seconds Field',
    'Seconds field expression over node ids, e.g. "212" or "229-212".':
      'Seconds field expression over node ids, e.g. "212" or "229-212".',
    'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.':
      'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.',
  },
  zh: {
    Max: '最大值',
    Min: '最小值',
    'Seconds Field': '秒数字段',
    'Seconds field expression over node ids, e.g. "212" or "229-212".':
      '秒数字段表达式，基于 nodeId，例如 "212" 或 "229-212"。',
    'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.':
      '留空则使用 seconds/duration 类型的参数；结果会被限制在 1-3600 秒之间。',
  },
  'zh-TW': {
    Max: '最大值',
    Min: '最小值',
    'Seconds Field': '秒數欄位',
    'Seconds field expression over node ids, e.g. "212" or "229-212".':
      '秒數欄位表達式，以 nodeId 為基礎，例如 "212" 或 "229-212"。',
    'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.':
      '留空則使用 seconds/duration 類型的參數；結果會限制在 1-3600 秒之間。',
  },
  fr: {
    Max: 'Maximum',
    Min: 'Minimum',
    'Seconds Field': 'Champ des secondes',
    'Seconds field expression over node ids, e.g. "212" or "229-212".':
      'Expression du champ des secondes sur les identifiants de nœud, par ex. « 212 » ou « 229-212 ».',
    'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.':
      'Laisser vide pour utiliser un paramètre de type seconds/duration ; le résultat est limité à 1-3600 secondes.',
  },
  ja: {
    Max: '最大',
    Min: '最小',
    'Seconds Field': '秒数フィールド',
    'Seconds field expression over node ids, e.g. "212" or "229-212".':
      'nodeId を使った秒数フィールドの式（例: "212" または "229-212"）。',
    'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.':
      '空欄の場合は seconds/duration タイプのパラメータを使用します。結果は 1〜3600 秒に制限されます。',
  },
  ru: {
    Max: 'Максимум',
    Min: 'Минимум',
    'Seconds Field': 'Поле секунд',
    'Seconds field expression over node ids, e.g. "212" or "229-212".':
      'Выражение для поля секунд по nodeId, например «212» или «229-212».',
    'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.':
      'Оставьте пустым, чтобы использовать параметр типа seconds/duration; результат ограничен 1–3600 секундами.',
  },
  vi: {
    Max: 'Tối đa',
    Min: 'Tối thiểu',
    'Seconds Field': 'Trường số giây',
    'Seconds field expression over node ids, e.g. "212" or "229-212".':
      'Biểu thức trường số giây theo nodeId, ví dụ "212" hoặc "229-212".',
    'Leave empty to use a seconds/duration parameter; the result is clamped to 1-3600 seconds.':
      'Để trống để dùng tham số kiểu seconds/duration; kết quả được giới hạn trong 1-3600 giây.',
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
