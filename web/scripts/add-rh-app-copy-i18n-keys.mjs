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

// One-off migration script: adds the RunningHub app-record copy keys to all
// seven locales. Run from web/: node scripts/add-rh-app-copy-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Copy App': 'Copy App',
    'Copied from {{name}}. Adjust the fields and save to create a new app.':
      'Copied from {{name}}. Adjust the fields and save to create a new app.',
  },
  zh: {
    'Copy App': '复制应用',
    'Copied from {{name}}. Adjust the fields and save to create a new app.':
      '已复制 {{name}}，请修改内容后保存为新应用。',
  },
  'zh-TW': {
    'Copy App': '複製應用',
    'Copied from {{name}}. Adjust the fields and save to create a new app.':
      '已複製 {{name}}，請修改內容後儲存為新應用。',
  },
  fr: {
    'Copy App': "Copier l'application",
    'Copied from {{name}}. Adjust the fields and save to create a new app.':
      'Copie de {{name}}. Modifiez les champs puis enregistrez pour créer une nouvelle application.',
  },
  ja: {
    'Copy App': 'アプリをコピー',
    'Copied from {{name}}. Adjust the fields and save to create a new app.':
      '{{name}} をコピーしました。内容を編集して保存すると新しいアプリを作成できます。',
  },
  ru: {
    'Copy App': 'Копировать приложение',
    'Copied from {{name}}. Adjust the fields and save to create a new app.':
      'Скопировано из {{name}}. Измените поля и сохраните, чтобы создать новое приложение.',
  },
  vi: {
    'Copy App': 'Sao chép ứng dụng',
    'Copied from {{name}}. Adjust the fields and save to create a new app.':
      'Đã sao chép từ {{name}}. Chỉnh sửa các trường rồi lưu để tạo ứng dụng mới.',
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
