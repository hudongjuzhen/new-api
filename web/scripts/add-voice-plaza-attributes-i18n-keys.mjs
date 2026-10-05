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

// One-off migration script: adds the Voice Plaza attribute + CSV import/export
// keys to all seven locales.
// Run from web/: node scripts/add-voice-plaza-attributes-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Age Range': 'Age Range',
    Attributes: 'Attributes',
    'At most 8 scenes': 'At most 8 scenes',
    'CSV File': 'CSV File',
    Child: 'Child',
    'Choose a CSV file': 'Choose a CSV file',
    'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.':
      'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.',
    'Create and update by name': 'Create and update by name',
    'Create only': 'Create only',
    'Created: {{count}}': 'Created: {{count}}',
    'Customer service, audiobook, live stream':
      'Customer service, audiobook, live stream',
    'Each scene must be 24 characters or fewer':
      'Each scene must be 24 characters or fewer',
    Export: 'Export',
    'Export failed': 'Export failed',
    'Export started': 'Export started',
    'Failed rows': 'Failed rows',
    'Failed: {{count}}': 'Failed: {{count}}',
    Female: 'Female',
    Gender: 'Gender',
    Import: 'Import',
    'Import Mode': 'Import Mode',
    'Import Result': 'Import Result',
    'Import Voices': 'Import Voices',
    'Import failed': 'Import failed',
    'Importing...': 'Importing...',
    'Imported: {{created}} created, {{updated}} updated, {{failed}} failed':
      'Imported: {{created}} created, {{updated}} updated, {{failed}} failed',
    Male: 'Male',
    'Middle-aged': 'Middle-aged',
    Neutral: 'Neutral',
    'Not specified': 'Not specified',
    'Row {{row}}': 'Row {{row}}',
    Senior: 'Senior',
    'Separate scenes with a comma; at most 8 scenes.':
      'Separate scenes with a comma; at most 8 scenes.',
    'Suitable Scenes': 'Suitable Scenes',
    Teen: 'Teen',
    'Updated: {{count}}': 'Updated: {{count}}',
    'Upload a CSV exported from this page. name and voice_type are required.':
      'Upload a CSV exported from this page. name and voice_type are required.',
    Young: 'Young',
  },
  zh: {
    'Age Range': '年龄段',
    Attributes: '属性',
    'At most 8 scenes': '最多 8 个场景',
    'CSV File': 'CSV 文件',
    Child: '儿童',
    'Choose a CSV file': '请选择 CSV 文件',
    'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name、description、voice_type、gender、age_range、scenes、audio_url、audio_name、audio_size、enabled、sort_order。可先导出当前列表获得带示例的模板。',
    'Create and update by name': '按名称新增或更新',
    'Create only': '仅新增',
    'Created: {{count}}': '新增 {{count}} 条',
    'Customer service, audiobook, live stream': '客服播报, 有声书, 直播',
    'Each scene must be 24 characters or fewer': '单个场景最多 24 个字符',
    Export: '导出',
    'Export failed': '导出失败',
    'Export started': '已开始导出',
    'Failed rows': '失败行',
    'Failed: {{count}}': '失败 {{count}} 条',
    Female: '女声',
    Gender: '性别',
    Import: '导入',
    'Import Mode': '导入模式',
    'Import Result': '导入结果',
    'Import Voices': '导入音色',
    'Import failed': '导入失败',
    'Importing...': '导入中…',
    'Imported: {{created}} created, {{updated}} updated, {{failed}} failed':
      '导入完成：新增 {{created}} 条，更新 {{updated}} 条，失败 {{failed}} 条',
    Male: '男声',
    'Middle-aged': '中年',
    Neutral: '中性',
    'Not specified': '未标注',
    'Row {{row}}': '第 {{row}} 行',
    Senior: '老年',
    'Separate scenes with a comma; at most 8 scenes.':
      '多个场景用逗号分隔，最多 8 个。',
    'Suitable Scenes': '适合场景',
    Teen: '少年',
    'Updated: {{count}}': '更新 {{count}} 条',
    'Upload a CSV exported from this page. name and voice_type are required.':
      '上传从本页导出的 CSV 文件；name 与 voice_type 为必填列。',
    Young: '青年',
  },
  'zh-TW': {
    'Age Range': '年齡段',
    Attributes: '屬性',
    'At most 8 scenes': '最多 8 個場景',
    'CSV File': 'CSV 檔案',
    Child: '兒童',
    'Choose a CSV file': '請選擇 CSV 檔案',
    'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.':
      '欄位：name、description、voice_type、gender、age_range、scenes、audio_url、audio_name、audio_size、enabled、sort_order。可先匯出目前列表取得含範例的範本。',
    'Create and update by name': '依名稱新增或更新',
    'Create only': '僅新增',
    'Created: {{count}}': '新增 {{count}} 筆',
    'Customer service, audiobook, live stream': '客服播報, 有聲書, 直播',
    'Each scene must be 24 characters or fewer': '單個場景最多 24 個字元',
    Export: '匯出',
    'Export failed': '匯出失敗',
    'Export started': '已開始匯出',
    'Failed rows': '失敗列',
    'Failed: {{count}}': '失敗 {{count}} 筆',
    Female: '女聲',
    Gender: '性別',
    Import: '匯入',
    'Import Mode': '匯入模式',
    'Import Result': '匯入結果',
    'Import Voices': '匯入音色',
    'Import failed': '匯入失敗',
    'Importing...': '匯入中…',
    'Imported: {{created}} created, {{updated}} updated, {{failed}} failed':
      '匯入完成：新增 {{created}} 筆，更新 {{updated}} 筆，失敗 {{failed}} 筆',
    Male: '男聲',
    'Middle-aged': '中年',
    Neutral: '中性',
    'Not specified': '未標註',
    'Row {{row}}': '第 {{row}} 行',
    Senior: '老年',
    'Separate scenes with a comma; at most 8 scenes.':
      '多個場景以逗號分隔，最多 8 個。',
    'Suitable Scenes': '適合場景',
    Teen: '少年',
    'Updated: {{count}}': '更新 {{count}} 筆',
    'Upload a CSV exported from this page. name and voice_type are required.':
      '上傳從本頁匯出的 CSV 檔案；name 與 voice_type 為必填欄位。',
    Young: '青年',
  },
  fr: {
    'Age Range': "Tranche d'âge",
    Attributes: 'Attributs',
    'At most 8 scenes': '8 scènes au maximum',
    'CSV File': 'Fichier CSV',
    Child: 'Enfant',
    'Choose a CSV file': 'Choisissez un fichier CSV',
    'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.':
      'Colonnes : name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Exportez la liste actuelle pour obtenir un modèle prérempli.',
    'Create and update by name': 'Créer et mettre à jour par nom',
    'Create only': 'Créer uniquement',
    'Created: {{count}}': 'Créés : {{count}}',
    'Customer service, audiobook, live stream':
      'Service client, livre audio, direct',
    'Each scene must be 24 characters or fewer':
      'Chaque scène doit faire 24 caractères maximum',
    Export: 'Exporter',
    'Export failed': "Échec de l'export",
    'Export started': 'Export lancé',
    'Failed rows': 'Lignes en échec',
    'Failed: {{count}}': 'Échecs : {{count}}',
    Female: 'Voix féminine',
    Gender: 'Genre',
    Import: 'Importer',
    'Import Mode': "Mode d'import",
    'Import Result': "Résultat de l'import",
    'Import Voices': 'Importer des voix',
    'Import failed': "Échec de l'import",
    'Importing...': 'Import en cours…',
    'Imported: {{created}} created, {{updated}} updated, {{failed}} failed':
      'Import terminé : {{created}} créés, {{updated}} mis à jour, {{failed}} en échec',
    Male: 'Voix masculine',
    'Middle-aged': 'Adulte',
    Neutral: 'Neutre',
    'Not specified': 'Non spécifié',
    'Row {{row}}': 'Ligne {{row}}',
    Senior: 'Senior',
    'Separate scenes with a comma; at most 8 scenes.':
      'Séparez les scènes par une virgule ; 8 scènes au maximum.',
    'Suitable Scenes': 'Scènes adaptées',
    Teen: 'Adolescent',
    'Updated: {{count}}': 'Mis à jour : {{count}}',
    'Upload a CSV exported from this page. name and voice_type are required.':
      'Importez un CSV exporté depuis cette page. name et voice_type sont obligatoires.',
    Young: 'Jeune',
  },
  ja: {
    'Age Range': '年齢層',
    Attributes: '属性',
    'At most 8 scenes': 'シーンは最大8件',
    'CSV File': 'CSV ファイル',
    Child: '児童',
    'Choose a CSV file': 'CSV ファイルを選択してください',
    'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name、description、voice_type、gender、age_range、scenes、audio_url、audio_name、audio_size、enabled、sort_order。現在の一覧をエクスポートすると記入済みのテンプレートが得られます。',
    'Create and update by name': '名前で新規作成・更新',
    'Create only': '新規作成のみ',
    'Created: {{count}}': '作成: {{count}}',
    'Customer service, audiobook, live stream':
      '客服、オーディオブック、ライブ配信',
    'Each scene must be 24 characters or fewer':
      '各シーンは24文字以内にしてください',
    Export: 'エクスポート',
    'Export failed': 'エクスポートに失敗しました',
    'Export started': 'エクスポートを開始しました',
    'Failed rows': '失敗した行',
    'Failed: {{count}}': '失敗: {{count}}',
    Female: '女性',
    Gender: '性別',
    Import: 'インポート',
    'Import Mode': 'インポートモード',
    'Import Result': 'インポート結果',
    'Import Voices': '音色のインポート',
    'Import failed': 'インポートに失敗しました',
    'Importing...': 'インポート中…',
    'Imported: {{created}} created, {{updated}} updated, {{failed}} failed':
      'インポート完了：作成 {{created}} 件、更新 {{updated}} 件、失敗 {{failed}} 件',
    Male: '男性',
    'Middle-aged': '中年',
    Neutral: '中性',
    'Not specified': '未設定',
    'Row {{row}}': '{{row}} 行目',
    Senior: '高齢',
    'Separate scenes with a comma; at most 8 scenes.':
      '複数のシーンはカンマで区切ってください（最大8件）。',
    'Suitable Scenes': '適合シーン',
    Teen: '少年',
    'Updated: {{count}}': '更新: {{count}}',
    'Upload a CSV exported from this page. name and voice_type are required.':
      'このページからエクスポートした CSV をアップロードしてください。name と voice_type は必須です。',
    Young: '青年',
  },
  ru: {
    'Age Range': 'Возрастная группа',
    Attributes: 'Атрибуты',
    'At most 8 scenes': 'Не более 8 сцен',
    'CSV File': 'Файл CSV',
    Child: 'Ребёнок',
    'Choose a CSV file': 'Выберите файл CSV',
    'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.':
      'Столбцы: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Выгрузите текущий список, чтобы получить заполненный шаблон.',
    'Create and update by name': 'Создавать и обновлять по названию',
    'Create only': 'Только создание',
    'Created: {{count}}': 'Создано: {{count}}',
    'Customer service, audiobook, live stream': 'Поддержка, аудиокнига, стрим',
    'Each scene must be 24 characters or fewer':
      'Каждая сцена — не более 24 символов',
    Export: 'Экспорт',
    'Export failed': 'Не удалось экспортировать',
    'Export started': 'Экспорт запущен',
    'Failed rows': 'Отклонённые строки',
    'Failed: {{count}}': 'Ошибок: {{count}}',
    Female: 'Женский',
    Gender: 'Пол',
    Import: 'Импорт',
    'Import Mode': 'Режим импорта',
    'Import Result': 'Результат импорта',
    'Import Voices': 'Импорт голосов',
    'Import failed': 'Не удалось импортировать',
    'Importing...': 'Импорт…',
    'Imported: {{created}} created, {{updated}} updated, {{failed}} failed':
      'Импорт завершён: создано {{created}}, обновлено {{updated}}, ошибок {{failed}}',
    Male: 'Мужской',
    'Middle-aged': 'Средний возраст',
    Neutral: 'Нейтральный',
    'Not specified': 'Не указано',
    'Row {{row}}': 'Строка {{row}}',
    Senior: 'Пожилой',
    'Separate scenes with a comma; at most 8 scenes.':
      'Разделяйте сцены запятой; не более 8 сцен.',
    'Suitable Scenes': 'Подходящие сцены',
    Teen: 'Подросток',
    'Updated: {{count}}': 'Обновлено: {{count}}',
    'Upload a CSV exported from this page. name and voice_type are required.':
      'Загрузите CSV, выгруженный с этой страницы. Поля name и voice_type обязательны.',
    Young: 'Молодой',
  },
  vi: {
    'Age Range': 'Nhóm tuổi',
    Attributes: 'Thuộc tính',
    'At most 8 scenes': 'Tối đa 8 cảnh',
    'CSV File': 'Tệp CSV',
    Child: 'Trẻ em',
    'Choose a CSV file': 'Vui lòng chọn tệp CSV',
    'Columns: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Export the current list to get a filled-in template.':
      'Các cột: name, description, voice_type, gender, age_range, scenes, audio_url, audio_name, audio_size, enabled, sort_order. Hãy xuất danh sách hiện tại để có mẫu đã điền sẵn.',
    'Create and update by name': 'Tạo mới và cập nhật theo tên',
    'Create only': 'Chỉ tạo mới',
    'Created: {{count}}': 'Đã tạo: {{count}}',
    'Customer service, audiobook, live stream':
      'Chăm sóc khách hàng, sách nói, livestream',
    'Each scene must be 24 characters or fewer': 'Mỗi cảnh tối đa 24 ký tự',
    Export: 'Xuất',
    'Export failed': 'Xuất thất bại',
    'Export started': 'Đã bắt đầu xuất',
    'Failed rows': 'Các dòng lỗi',
    'Failed: {{count}}': 'Lỗi: {{count}}',
    Female: 'Nữ',
    Gender: 'Giới tính',
    Import: 'Nhập',
    'Import Mode': 'Chế độ nhập',
    'Import Result': 'Kết quả nhập',
    'Import Voices': 'Nhập giọng đọc',
    'Import failed': 'Nhập thất bại',
    'Importing...': 'Đang nhập…',
    'Imported: {{created}} created, {{updated}} updated, {{failed}} failed':
      'Nhập xong: tạo {{created}}, cập nhật {{updated}}, lỗi {{failed}}',
    Male: 'Nam',
    'Middle-aged': 'Trung niên',
    Neutral: 'Trung tính',
    'Not specified': 'Chưa xác định',
    'Row {{row}}': 'Dòng {{row}}',
    Senior: 'Cao tuổi',
    'Separate scenes with a comma; at most 8 scenes.':
      'Phân tách các cảnh bằng dấu phẩy; tối đa 8 cảnh.',
    'Suitable Scenes': 'Cảnh phù hợp',
    Teen: 'Thiếu niên',
    'Updated: {{count}}': 'Đã cập nhật: {{count}}',
    'Upload a CSV exported from this page. name and voice_type are required.':
      'Tải lên tệp CSV xuất từ trang này. name và voice_type là bắt buộc.',
    Young: 'Thanh niên',
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
