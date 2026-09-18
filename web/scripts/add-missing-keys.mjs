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

// One-off migration script: adds the RunningHub app-center API-example keys to
// all seven locales. Run from web/: node scripts/add-missing-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'API Examples': 'API Examples',
    'An API key can only query and cancel the runs it submitted.':
      'An API key can only query and cancel the runs it submitted.',
    'Call this app from your own service with an API key.':
      'Call this app from your own service with an API key.',
    'Query task': 'Query task',
    'Run app': 'Run app',
    'Values are keyed by nodeId.fieldName.':
      'Values are keyed by nodeId.fieldName.',
  },
  zh: {
    'API Examples': 'API 调用示例',
    'An API key can only query and cancel the runs it submitted.':
      'API Key 只能查询和取消自己提交的任务。',
    'Call this app from your own service with an API key.':
      '在自己的服务中用 API Key 调用该应用。',
    'Query task': '查询任务',
    'Run app': '运行应用',
    'Values are keyed by nodeId.fieldName.': '参数以 nodeId.fieldName 作为键名。',
  },
  'zh-TW': {
    'API Examples': 'API 呼叫範例',
    'An API key can only query and cancel the runs it submitted.':
      'API Key 只能查詢與取消自己提交的任務。',
    'Call this app from your own service with an API key.':
      '在自己的服務中以 API Key 呼叫此應用。',
    'Query task': '查詢任務',
    'Run app': '執行應用',
    'Values are keyed by nodeId.fieldName.': '參數以 nodeId.fieldName 作為鍵名。',
  },
  fr: {
    'API Examples': "Exemples d'appel API",
    'An API key can only query and cancel the runs it submitted.':
      "Une clé API ne peut consulter et annuler que les exécutions qu'elle a lancées.",
    'Call this app from your own service with an API key.':
      'Appelez cette application depuis votre service avec une clé API.',
    'Query task': 'Consulter la tâche',
    'Run app': "Lancer l'application",
    'Values are keyed by nodeId.fieldName.':
      'Les valeurs sont indexées par nodeId.fieldName.',
  },
  ja: {
    'API Examples': 'API 呼び出し例',
    'An API key can only query and cancel the runs it submitted.':
      'API キーは自身が送信した実行のみ照会・キャンセルできます。',
    'Call this app from your own service with an API key.':
      'ご自身のサービスから API キーでこのアプリを呼び出せます。',
    'Query task': 'タスクを照会',
    'Run app': 'アプリを実行',
    'Values are keyed by nodeId.fieldName.': '値のキーは nodeId.fieldName です。',
  },
  ru: {
    'API Examples': 'Примеры вызовов API',
    'An API key can only query and cancel the runs it submitted.':
      'Ключ API может запрашивать и отменять только запущенные им задачи.',
    'Call this app from your own service with an API key.':
      'Вызывайте это приложение из своего сервиса с помощью ключа API.',
    'Query task': 'Запросить задачу',
    'Run app': 'Запустить приложение',
    'Values are keyed by nodeId.fieldName.':
      'Значения индексируются по nodeId.fieldName.',
  },
  vi: {
    'API Examples': 'Ví dụ gọi API',
    'An API key can only query and cancel the runs it submitted.':
      'API Key chỉ có thể truy vấn và hủy các tác vụ do chính nó gửi.',
    'Call this app from your own service with an API key.':
      'Gọi ứng dụng này từ dịch vụ của bạn bằng API Key.',
    'Query task': 'Truy vấn tác vụ',
    'Run app': 'Chạy ứng dụng',
    'Values are keyed by nodeId.fieldName.': 'Khóa giá trị là nodeId.fieldName.',
  },
}

async function main() {
  let totalAdded = 0

  for (const [locale, trans] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const json = JSON.parse(await fs.readFile(filePath, 'utf8'))

    let count = 0
    for (const [key, value] of Object.entries(trans)) {
      if (!Object.prototype.hasOwnProperty.call(json.translation, key)) {
        json.translation[key] = value
        count++
      } else if (json.translation[key] !== value) {
        json.translation[key] = value
        count++
      }
    }

    if (count > 0) {
      json.translation = Object.fromEntries(
        Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b))
      )
      await fs.writeFile(filePath, stableStringify(json), 'utf8')
    }

    console.log(`${locale}: ${count} translations applied`)
    totalAdded += count
  }

  console.log(`\nTotal: ${totalAdded} translations applied`)
}

main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
