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
// Merge the RunningHub apps-form curl-import i18n keys into all seven locale
// files, then re-run the i18n sync so ordering, extras and reports stay
// consistent with the rest of the repo.
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
    'Failed to parse the request example': 'Failed to parse the request example',
    'Fill with an example': 'Fill with an example',
    'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.':
      'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.',
    'Paste a RunningHub curl request example…':
      'Paste a RunningHub curl request example…',
    'Please paste a RunningHub request example first':
      'Please paste a RunningHub request example first',
    'Request Example': 'Request Example',
    'Request example parsed': 'Request example parsed',
    'Request example parsed, but some fields were skipped':
      'Request example parsed, but some fields were skipped',
    'Use example': 'Use example',
  },
  zh: {
    'Failed to parse the request example': '解析请求示例失败',
    'Fill with an example': '填入示例',
    'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.':
      '暂无参数。请在上方粘贴请求示例并点击"一键获取模板"，或手动添加。',
    'Paste a RunningHub curl request example…':
      '粘贴 RunningHub curl 请求示例…',
    'Please paste a RunningHub request example first':
      '请先粘贴 RunningHub 请求示例',
    'Request Example': '请求示例',
    'Request example parsed': '请求示例解析成功',
    'Request example parsed, but some fields were skipped':
      '请求示例已解析，但部分字段被跳过',
    'Use example': '填入示例',
  },
  'zh-TW': {
    'Failed to parse the request example': '解析請求示例失敗',
    'Fill with an example': '填入範例',
    'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.':
      '暫無參數。請在上方貼上請求示例並點擊「一鍵取得模板」，或手動新增。',
    'Paste a RunningHub curl request example…':
      '貼上 RunningHub curl 請求示例…',
    'Please paste a RunningHub request example first':
      '請先貼上 RunningHub 請求示例',
    'Request Example': '請求示例',
    'Request example parsed': '請求示例解析成功',
    'Request example parsed, but some fields were skipped':
      '請求示例已解析，但部分欄位被略過',
    'Use example': '填入範例',
  },
  fr: {
    'Failed to parse the request example': "Échec de l'analyse de l'exemple de requête",
    'Fill with an example': "Remplir avec un exemple",
    'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.':
      'Aucun paramètre pour l\'instant. Collez un exemple de requête ci-dessus puis cliquez sur « Récupérer le modèle », ou ajoutez manuellement.',
    'Paste a RunningHub curl request example…':
      'Collez un exemple de requête curl RunningHub…',
    'Please paste a RunningHub request example first':
      "Collez d'abord un exemple de requête RunningHub",
    'Request Example': "Exemple de requête",
    'Request example parsed': "Exemple de requête analysé",
    'Request example parsed, but some fields were skipped':
      'Exemple de requête analysé, mais certains champs ont été ignorés',
    'Use example': "Utiliser l'exemple",
  },
  ja: {
    'Failed to parse the request example': 'リクエスト例の解析に失敗しました',
    'Fill with an example': '例を入力',
    'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.':
      'パラメータはまだありません。上にリクエスト例を貼り付けて「テンプレートを取得」をクリックするか、手動で追加してください。',
    'Paste a RunningHub curl request example…':
      'RunningHub の curl リクエスト例を貼り付け…',
    'Please paste a RunningHub request example first':
      '先に RunningHub のリクエスト例を貼り付けてください',
    'Request Example': 'リクエスト例',
    'Request example parsed': 'リクエスト例を解析しました',
    'Request example parsed, but some fields were skipped':
      'リクエスト例を解析しましたが、一部のフィールドはスキップされました',
    'Use example': '例を使用',
  },
  ru: {
    'Failed to parse the request example': 'Не удалось разобрать пример запроса',
    'Fill with an example': 'Заполнить примером',
    'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.':
      'Параметров пока нет. Вставьте пример запроса выше и нажмите «Получить шаблон» либо добавьте вручную.',
    'Paste a RunningHub curl request example…':
      'Вставьте пример curl-запроса RunningHub…',
    'Please paste a RunningHub request example first':
      'Сначала вставьте пример запроса RunningHub',
    'Request Example': 'Пример запроса',
    'Request example parsed': 'Пример запроса разобран',
    'Request example parsed, but some fields were skipped':
      'Пример запроса разобран, но некоторые поля пропущены',
    'Use example': 'Использовать пример',
  },
  vi: {
    'Failed to parse the request example': 'Không thể phân tích ví dụ yêu cầu',
    'Fill with an example': 'Điền ví dụ',
    'No parameters yet. Paste a request example above and click "Fetch Template", or add manually.':
      'Chưa có tham số nào. Hãy dán một ví dụ yêu cầu ở trên và nhấp "Lấy mẫu", hoặc thêm thủ công.',
    'Paste a RunningHub curl request example…':
      'Dán một ví dụ yêu cầu curl RunningHub…',
    'Please paste a RunningHub request example first':
      'Vui lòng dán một ví dụ yêu cầu RunningHub trước',
    'Request Example': 'Ví dụ yêu cầu',
    'Request example parsed': 'Đã phân tích ví dụ yêu cầu',
    'Request example parsed, but some fields were skipped':
      'Đã phân tích ví dụ yêu cầu, nhưng một số trường bị bỏ qua',
    'Use example': 'Dùng ví dụ',
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
