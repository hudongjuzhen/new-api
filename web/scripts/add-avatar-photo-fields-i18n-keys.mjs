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

// One-off migration for the Avatar Plaza pictures: the cover (封面图, the
// pre-existing imageUrl) plus the three reference pictures added beside it
// (全身照 / 四视图 / 表情图). 'Cover Image' already exists in every locale, so only
// its three siblings and the reworded CSV column hint are written here.
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Full-body Photo': 'Full-body Photo',
    'Four Views': 'Four Views',
    'Expression Sheet': 'Expression Sheet',
    'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.',
  },
  zh: {
    'Full-body Photo': '全身照',
    'Four Views': '四视图',
    'Expression Sheet': '表情图',
    'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name（形象名称）、description（简介）、image_url（封面图）、full_body_url（全身照）、four_view_url（四视图）、expression_url（表情图）、gender（性别）、age_range（年龄段）、race（种族）、scenes（适合场景）、voice_id（音色 ID）、enabled（是否上架）、sort_order（排序）。导出当前列表即可获得填好表头的模板。',
  },
  'zh-TW': {
    'Full-body Photo': '全身照',
    'Four Views': '四視圖',
    'Expression Sheet': '表情圖',
    'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      '欄位：name（形象名稱）、description（簡介）、image_url（封面圖）、full_body_url（全身照）、four_view_url（四視圖）、expression_url（表情圖）、gender（性別）、age_range（年齡段）、race（種族）、scenes（適合情境）、voice_id（音色 ID）、enabled（是否上架）、sort_order（排序）。匯出目前列表即可取得填好表頭的範本。',
  },
  fr: {
    'Full-body Photo': 'Photo en pied',
    'Four Views': 'Quatre vues',
    'Expression Sheet': "Planche d'expressions",
    'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Colonnes : name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Exportez la liste actuelle pour obtenir un modèle prérempli.',
  },
  ja: {
    'Full-body Photo': '全身写真',
    'Four Views': '四方向ビュー',
    'Expression Sheet': '表情シート',
    'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order。現在の一覧をエクスポートすると、見出し入りのテンプレートになります。',
  },
  ru: {
    'Full-body Photo': 'Фото в полный рост',
    'Four Views': 'Четыре вида',
    'Expression Sheet': 'Лист эмоций',
    'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Столбцы: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Экспортируйте текущий список, чтобы получить готовый шаблон.',
  },
  vi: {
    'Full-body Photo': 'Ảnh toàn thân',
    'Four Views': 'Bốn góc nhìn',
    'Expression Sheet': 'Bảng biểu cảm',
    'Columns: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Các cột: name, description, image_url, full_body_url, four_view_url, expression_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Hãy xuất danh sách hiện tại để có mẫu có sẵn tiêu đề.',
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
