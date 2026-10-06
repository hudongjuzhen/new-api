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
// Merge the RunningHub admin cover-image picker i18n keys into all seven
// locale files, then re-run the i18n sync so ordering, extras and reports stay
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
    'Cover Image': 'Cover Image',
    'Cover image must be 10MB or smaller':
      'Cover image must be 10MB or smaller',
    'Cover image uploaded': 'Cover image uploaded',
    'No cover image': 'No cover image',
    'PNG / JPG / GIF / WebP / BMP, up to 10MB':
      'PNG / JPG / GIF / WebP / BMP, up to 10MB',
    'Paste an image URL': 'Paste an image URL',
    'Upload an image or paste an image URL':
      'Upload an image or paste an image URL',
    'Upload image': 'Upload image',
  },
  zh: {
    'Cover Image': '封面图',
    'Cover image must be 10MB or smaller': '封面图不能超过 10MB',
    'Cover image uploaded': '封面图已上传',
    'No cover image': '暂无封面图',
    'PNG / JPG / GIF / WebP / BMP, up to 10MB':
      '支持 PNG / JPG / GIF / WebP / BMP，单张不超过 10MB',
    'Paste an image URL': '粘贴图片地址',
    'Upload an image or paste an image URL': '上传图片，或粘贴图片地址',
    'Upload image': '上传图片',
  },
  'zh-TW': {
    'Cover Image': '封面圖',
    'Cover image must be 10MB or smaller': '封面圖不能超過 10MB',
    'Cover image uploaded': '封面圖已上傳',
    'No cover image': '尚無封面圖',
    'PNG / JPG / GIF / WebP / BMP, up to 10MB':
      '支援 PNG / JPG / GIF / WebP / BMP，單張不超過 10MB',
    'Paste an image URL': '貼上圖片網址',
    'Upload an image or paste an image URL': '上傳圖片，或貼上圖片網址',
    'Upload image': '上傳圖片',
  },
  fr: {
    'Cover Image': 'Image de couverture',
    'Cover image must be 10MB or smaller':
      "L'image de couverture ne doit pas dépasser 10 Mo",
    'Cover image uploaded': 'Image de couverture importée',
    'No cover image': "Aucune image de couverture",
    'PNG / JPG / GIF / WebP / BMP, up to 10MB':
      'PNG / JPG / GIF / WebP / BMP, 10 Mo maximum',
    'Paste an image URL': "Collez l'URL d'une image",
    'Upload an image or paste an image URL':
      "Importez une image ou collez son URL",
    'Upload image': 'Importer une image',
  },
  ja: {
    'Cover Image': 'カバー画像',
    'Cover image must be 10MB or smaller':
      'カバー画像は 10MB 以下にしてください',
    'Cover image uploaded': 'カバー画像をアップロードしました',
    'No cover image': 'カバー画像なし',
    'PNG / JPG / GIF / WebP / BMP, up to 10MB':
      'PNG / JPG / GIF / WebP / BMP、10MB まで',
    'Paste an image URL': '画像 URL を貼り付け',
    'Upload an image or paste an image URL':
      '画像をアップロードするか、画像 URL を貼り付けてください',
    'Upload image': '画像をアップロード',
  },
  ru: {
    'Cover Image': 'Обложка',
    'Cover image must be 10MB or smaller':
      'Размер обложки не должен превышать 10 МБ',
    'Cover image uploaded': 'Обложка загружена',
    'No cover image': 'Обложка не задана',
    'PNG / JPG / GIF / WebP / BMP, up to 10MB':
      'PNG / JPG / GIF / WebP / BMP, до 10 МБ',
    'Paste an image URL': 'Вставьте URL изображения',
    'Upload an image or paste an image URL':
      'Загрузите изображение или вставьте его URL',
    'Upload image': 'Загрузить изображение',
  },
  vi: {
    'Cover Image': 'Ảnh bìa',
    'Cover image must be 10MB or smaller': 'Ảnh bìa không được vượt quá 10MB',
    'Cover image uploaded': 'Đã tải ảnh bìa lên',
    'No cover image': 'Chưa có ảnh bìa',
    'PNG / JPG / GIF / WebP / BMP, up to 10MB':
      'PNG / JPG / GIF / WebP / BMP, tối đa 10MB',
    'Paste an image URL': 'Dán URL hình ảnh',
    'Upload an image or paste an image URL':
      'Tải ảnh lên hoặc dán URL hình ảnh',
    'Upload image': 'Tải ảnh lên',
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
