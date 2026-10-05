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

// One-off migration script: adds the Voice Plaza avatar + language keys to all
// seven locales (language *names* need no keys: they come from Intl.DisplayNames).
// Run from web/: node scripts/add-voice-plaza-avatar-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'All languages': 'All languages',
    Avatar: 'Avatar',
    'Avatar must be an /uploads path or an http(s) URL':
      'Avatar must be an /uploads path or an http(s) URL',
    'Avatar preview': 'Avatar preview',
    'Invalid language code': 'Invalid language code',
    Language: 'Language',
    'Portrait shown next to the voice; optional.':
      'Portrait shown next to the voice; optional.',
    'Short lower-case language tag (zh, en, ja…).':
      'Short lower-case language tag (zh, en, ja…).',
    'https://… or /uploads/voices/…': 'https://… or /uploads/voices/…',
  },
  zh: {
    'All languages': '全部语言',
    Avatar: '头像',
    'Avatar must be an /uploads path or an http(s) URL':
      '头像必须是 /uploads 路径或 http(s) 地址',
    'Avatar preview': '头像预览',
    'Invalid language code': '语言代码格式不正确',
    Language: '语言',
    'Portrait shown next to the voice; optional.': '显示在音色旁的头像，可选。',
    'Short lower-case language tag (zh, en, ja…).':
      '小写语言代码（zh、en、ja…）。',
    'https://… or /uploads/voices/…': 'https://… 或 /uploads/voices/…',
  },
  'zh-TW': {
    'All languages': '全部語言',
    Avatar: '頭像',
    'Avatar must be an /uploads path or an http(s) URL':
      '頭像必須是 /uploads 路徑或 http(s) 網址',
    'Avatar preview': '頭像預覽',
    'Invalid language code': '語言代碼格式不正確',
    Language: '語言',
    'Portrait shown next to the voice; optional.': '顯示在音色旁的頭像，選填。',
    'Short lower-case language tag (zh, en, ja…).':
      '小寫語言代碼（zh、en、ja…）。',
    'https://… or /uploads/voices/…': 'https://… 或 /uploads/voices/…',
  },
  fr: {
    'All languages': 'Toutes les langues',
    Avatar: 'Avatar',
    'Avatar must be an /uploads path or an http(s) URL':
      "L'avatar doit être un chemin /uploads ou une URL http(s)",
    'Avatar preview': "Aperçu de l'avatar",
    'Invalid language code': 'Code de langue invalide',
    Language: 'Langue',
    'Portrait shown next to the voice; optional.':
      'Portrait affiché à côté de la voix ; facultatif.',
    'Short lower-case language tag (zh, en, ja…).':
      'Code de langue court en minuscules (zh, en, ja…).',
    'https://… or /uploads/voices/…': 'https://… ou /uploads/voices/…',
  },
  ja: {
    'All languages': 'すべての言語',
    Avatar: 'アバター',
    'Avatar must be an /uploads path or an http(s) URL':
      'アバターは /uploads パスまたは http(s) URL を指定してください',
    'Avatar preview': 'アバターのプレビュー',
    'Invalid language code': '言語コードが正しくありません',
    Language: '言語',
    'Portrait shown next to the voice; optional.':
      '音色の横に表示されるアバター（任意）。',
    'Short lower-case language tag (zh, en, ja…).':
      '小文字の言語コード（zh、en、ja…）。',
    'https://… or /uploads/voices/…': 'https://… または /uploads/voices/…',
  },
  ru: {
    'All languages': 'Все языки',
    Avatar: 'Аватар',
    'Avatar must be an /uploads path or an http(s) URL':
      'Аватар должен быть путём /uploads или URL http(s)',
    'Avatar preview': 'Предпросмотр аватара',
    'Invalid language code': 'Неверный код языка',
    Language: 'Язык',
    'Portrait shown next to the voice; optional.':
      'Портрет рядом с голосом; необязательно.',
    'Short lower-case language tag (zh, en, ja…).':
      'Короткий код языка в нижнем регистре (zh, en, ja…).',
    'https://… or /uploads/voices/…': 'https://… или /uploads/voices/…',
  },
  vi: {
    'All languages': 'Tất cả ngôn ngữ',
    Avatar: 'Ảnh đại diện',
    'Avatar must be an /uploads path or an http(s) URL':
      'Ảnh đại diện phải là đường dẫn /uploads hoặc URL http(s)',
    'Avatar preview': 'Xem trước ảnh đại diện',
    'Invalid language code': 'Mã ngôn ngữ không hợp lệ',
    Language: 'Ngôn ngữ',
    'Portrait shown next to the voice; optional.':
      'Ảnh hiển thị cạnh giọng đọc; không bắt buộc.',
    'Short lower-case language tag (zh, en, ja…).':
      'Mã ngôn ngữ viết thường, ngắn (zh, en, ja…).',
    'https://… or /uploads/voices/…': 'https://… hoặc /uploads/voices/…',
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
