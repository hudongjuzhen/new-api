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

// Adds the channel "Maximum Concurrency" keys (label, description, validation
// message) to every locale. Run from the web/ package root:
//   node scripts/add-channel-concurrency-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const DESCRIPTION =
  'Maximum number of tasks this channel may run at the same time. 0 means unlimited; when the limit is reached further requests wait in a queue.'

const newKeys = {
  en: {
    'Maximum Concurrency': 'Maximum Concurrency',
    [DESCRIPTION]: DESCRIPTION,
    'Maximum concurrency must be between 0 and 100':
      'Maximum concurrency must be between 0 and 100',
  },
  zh: {
    'Maximum Concurrency': '最大并发数',
    [DESCRIPTION]:
      '该渠道同时运行的任务数上限。0 表示不限制；达到上限后新的请求会进入排队等待。',
    'Maximum concurrency must be between 0 and 100':
      '最大并发数必须在 0 到 100 之间',
  },
  'zh-TW': {
    'Maximum Concurrency': '最大並發數',
    [DESCRIPTION]:
      '該渠道同時執行的任務數上限。0 表示不限制；達到上限後新的請求會進入排隊等待。',
    'Maximum concurrency must be between 0 and 100':
      '最大並發數必須介於 0 到 100 之間',
  },
  fr: {
    'Maximum Concurrency': 'Concurrence maximale',
    [DESCRIPTION]:
      'Nombre maximal de tâches que ce canal peut exécuter simultanément. 0 signifie illimité ; une fois la limite atteinte, les nouvelles requêtes sont mises en file d’attente.',
    'Maximum concurrency must be between 0 and 100':
      'La concurrence maximale doit être comprise entre 0 et 100',
  },
  ja: {
    'Maximum Concurrency': '最大同時実行数',
    [DESCRIPTION]:
      'このチャネルが同時に実行できるタスク数の上限です。0 は無制限を意味し、上限に達したリクエストはキューで待機します。',
    'Maximum concurrency must be between 0 and 100':
      '最大同時実行数は 0 から 100 の間で指定してください',
  },
  ru: {
    'Maximum Concurrency': 'Максимальная параллельность',
    [DESCRIPTION]:
      'Максимальное число задач, которые канал может выполнять одновременно. 0 — без ограничений; при достижении лимита новые запросы встают в очередь.',
    'Maximum concurrency must be between 0 and 100':
      'Максимальная параллельность должна быть от 0 до 100',
  },
  vi: {
    'Maximum Concurrency': 'Số luồng đồng thời tối đa',
    [DESCRIPTION]:
      'Số tác vụ tối đa kênh này có thể chạy đồng thời. 0 nghĩa là không giới hạn; khi đạt giới hạn, các yêu cầu mới sẽ xếp hàng chờ.',
    'Maximum concurrency must be between 0 and 100':
      'Số luồng đồng thời tối đa phải nằm trong khoảng 0 đến 100',
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
