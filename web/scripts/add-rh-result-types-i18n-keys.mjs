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

// Adds the RunningHub app-center result-type keys (audio / video / archive /
// text previews) to every locale. Run from the web/ package root:
//   node scripts/add-rh-result-types-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    Archive: 'Archive',
    'Preview video': 'Preview video',
    'Preview audio': 'Preview audio',
    'Preview text': 'Preview text',
    'Download file': 'Download file',
    'Loading preview…': 'Loading preview…',
    'Failed to load preview': 'Failed to load preview',
    'Showing the first part of the file only.':
      'Showing the first part of the file only.',
    'This file type cannot be previewed. Download it to open.':
      'This file type cannot be previewed. Download it to open.',
  },
  zh: {
    Archive: '压缩包',
    'Preview video': '预览视频',
    'Preview audio': '预览音频',
    'Preview text': '预览文本',
    'Download file': '下载文件',
    'Loading preview…': '正在加载预览…',
    'Failed to load preview': '预览加载失败',
    'Showing the first part of the file only.': '仅显示文件的前一部分。',
    'This file type cannot be previewed. Download it to open.':
      '该文件类型不支持在线预览，请下载后打开。',
  },
  'zh-TW': {
    Archive: '壓縮檔',
    'Preview video': '預覽影片',
    'Preview audio': '預覽音訊',
    'Preview text': '預覽文字',
    'Download file': '下載檔案',
    'Loading preview…': '正在載入預覽…',
    'Failed to load preview': '預覽載入失敗',
    'Showing the first part of the file only.': '僅顯示檔案的前一部分。',
    'This file type cannot be previewed. Download it to open.':
      '該檔案類型不支援線上預覽，請下載後開啟。',
  },
  fr: {
    Archive: 'Archive',
    'Preview video': 'Aperçu de la vidéo',
    'Preview audio': "Aperçu de l'audio",
    'Preview text': 'Aperçu du texte',
    'Download file': 'Télécharger le fichier',
    'Loading preview…': "Chargement de l'aperçu…",
    'Failed to load preview': "Échec du chargement de l'aperçu",
    'Showing the first part of the file only.':
      'Seule la première partie du fichier est affichée.',
    'This file type cannot be previewed. Download it to open.':
      'Ce type de fichier ne peut pas être prévisualisé. Téléchargez-le pour l’ouvrir.',
  },
  ja: {
    Archive: 'アーカイブ',
    'Preview video': '動画をプレビュー',
    'Preview audio': '音声をプレビュー',
    'Preview text': 'テキストをプレビュー',
    'Download file': 'ファイルをダウンロード',
    'Loading preview…': 'プレビューを読み込み中…',
    'Failed to load preview': 'プレビューの読み込みに失敗しました',
    'Showing the first part of the file only.':
      'ファイルの先頭部分のみを表示しています。',
    'This file type cannot be previewed. Download it to open.':
      'このファイル形式はプレビューできません。ダウンロードして開いてください。',
  },
  ru: {
    Archive: 'Архив',
    'Preview video': 'Просмотр видео',
    'Preview audio': 'Просмотр аудио',
    'Preview text': 'Просмотр текста',
    'Download file': 'Скачать файл',
    'Loading preview…': 'Загрузка предпросмотра…',
    'Failed to load preview': 'Не удалось загрузить предпросмотр',
    'Showing the first part of the file only.':
      'Показана только первая часть файла.',
    'This file type cannot be previewed. Download it to open.':
      'Этот тип файла нельзя просмотреть. Скачайте его, чтобы открыть.',
  },
  vi: {
    Archive: 'Tệp nén',
    'Preview video': 'Xem trước video',
    'Preview audio': 'Xem trước âm thanh',
    'Preview text': 'Xem trước văn bản',
    'Download file': 'Tải tệp xuống',
    'Loading preview…': 'Đang tải bản xem trước…',
    'Failed to load preview': 'Không tải được bản xem trước',
    'Showing the first part of the file only.':
      'Chỉ hiển thị phần đầu của tệp.',
    'This file type cannot be previewed. Download it to open.':
      'Không thể xem trước loại tệp này. Hãy tải xuống để mở.',
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
