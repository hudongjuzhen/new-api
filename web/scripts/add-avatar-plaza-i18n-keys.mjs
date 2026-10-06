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

// One-off migration script: adds the Avatar Plaza (zsy-avatar) keys plus the
// "Public Data" group title to all seven locales.
// Run from web/: node scripts/add-avatar-plaza-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Asian': 'Asian',
    'Avatar List API': 'Avatar List API',
    'Avatar Name': 'Avatar Name',
    'Avatar Plaza': 'Avatar Plaza',
    'Avatar created': 'Avatar created',
    'Avatar deleted': 'Avatar deleted',
    'Avatar name is required': 'Avatar name is required',
    'Avatar name is too long (max 191 characters)':
      'Avatar name is too long (max 191 characters)',
    'Avatar updated': 'Avatar updated',
    'Black': 'Black',
    'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.',
    'Customer service girl': 'Customer service girl',
    'Delete Avatar': 'Delete Avatar',
    'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.':
      'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.',
    'Edit Avatar': 'Edit Avatar',
    'Image': 'Image',
    'Image must be 10MB or smaller': 'Image must be 10MB or smaller',
    'Image must be an /uploads path or an http(s) URL':
      'Image must be an /uploads path or an http(s) URL',
    'Image preview': 'Image preview',
    'Image uploaded': 'Image uploaded',
    'Import Avatars': 'Import Avatars',
    'Latino': 'Latino',
    'Middle Eastern': 'Middle Eastern',
    'Mixed': 'Mixed',
    'New Avatar': 'New Avatar',
    'No avatars yet': 'No avatars yet',
    'No image': 'No image',
    'No sample audio for this voice': 'No sample audio for this voice',
    'No voice linked': 'No voice linked',
    'Off-shelf personas stay hidden from the public list.':
      'Off-shelf personas stay hidden from the public list.',
    'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.':
      'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.',
    'Public Data': 'Public Data',
    'Race': 'Race',
    'Remove image': 'Remove image',
    'Search by name or voice id': 'Search by name or voice id',
    'South Asian': 'South Asian',
    'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.':
      'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.',
    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.':
      'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.',
    'This voice id is not in the Voice Plaza, so no sample audio is available.':
      'This voice id is not in the Voice Plaza, so no sample audio is available.',
    'Total {{count}} avatars': 'Total {{count}} avatars',
    'Upload a CSV exported from this page. name and image_url are required.':
      'Upload a CSV exported from this page. name and image_url are required.',
    'Upload image': 'Upload image',
    'Voice ID': 'Voice ID',
    'Voice Sample': 'Voice Sample',
    'Voice id is too long (max 191 characters)':
      'Voice id is too long (max 191 characters)',
    'What this persona looks like and does':
      'What this persona looks like and does',
    'White': 'White',
    'https://… or /uploads/images/…': 'https://… or /uploads/images/…',
  },
  zh: {
    'Asian': '亚洲人',
    'Avatar List API': '形象列表接口',
    'Avatar Name': '形象名称',
    'Avatar Plaza': '形象广场',
    'Avatar created': '形象创建成功',
    'Avatar deleted': '形象已删除',
    'Avatar name is required': '请填写形象名称',
    'Avatar name is too long (max 191 characters)':
      '形象名称过长（最多 191 字符）',
    'Avatar updated': '形象更新成功',
    'Black': '黑人',
    'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name（形象名称）、description（简介）、image_url（图片地址）、gender（性别）、age_range（年龄段）、race（种族）、scenes（适合场景）、voice_id（音色 ID）、enabled（是否上架）、sort_order（排序）。导出当前列表即可获得填好表头的模板。',
    'Customer service girl': '客服小雨',
    'Delete Avatar': '删除形象',
    'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.':
      '确定删除「{{name}}」？图片文件会保留在磁盘上，但该记录无法恢复。',
    'Edit Avatar': '编辑形象',
    'Image': '图片',
    'Image must be 10MB or smaller': '图片不能超过 10MB',
    'Image must be an /uploads path or an http(s) URL':
      '图片地址必须是 /uploads 路径或 http(s) 地址',
    'Image preview': '图片预览',
    'Image uploaded': '图片上传成功',
    'Import Avatars': '导入形象',
    'Latino': '拉丁裔',
    'Middle Eastern': '中东人',
    'Mixed': '混血',
    'New Avatar': '新建形象',
    'No avatars yet': '暂无形象',
    'No image': '暂无图片',
    'No sample audio for this voice': '该音色暂无示例音频',
    'No voice linked': '未关联音色',
    'Off-shelf personas stay hidden from the public list.':
      '已下架的形象不会出现在公开列表中。',
    'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.':
      '已上架的形象以只读方式公开，无需鉴权即可调用；每条数据都会带上所关联音色的示例音频。',
    'Public Data': '公共数据',
    'Race': '种族',
    'Remove image': '移除图片',
    'Search by name or voice id': '按名称或音色 ID 搜索',
    'South Asian': '南亚人',
    'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.':
      '音色 ID 即调用 TTS 时发送的 voice_type，其示例音频取自音色广场。',
    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.':
      '填写音色广场中某个音色的 voice_type，示例音频会自动带出。',
    'This voice id is not in the Voice Plaza, so no sample audio is available.':
      '该音色 ID 不在音色广场中，暂无示例音频。',
    'Total {{count}} avatars': '共 {{count}} 个形象',
    'Upload a CSV exported from this page. name and image_url are required.':
      '上传从本页导出的 CSV 文件；name 与 image_url 为必需列。',
    'Upload image': '上传图片',
    'Voice ID': '音色 ID',
    'Voice Sample': '音色示例',
    'Voice id is too long (max 191 characters)': '音色 ID 过长（最多 191 字符）',
    'What this persona looks like and does': '描述这个形象的外形与人设',
    'White': '白人',
    'https://… or /uploads/images/…': 'https://… 或 /uploads/images/…',
  },
  'zh-TW': {
    'Asian': '亞洲人',
    'Avatar List API': '形象列表介面',
    'Avatar Name': '形象名稱',
    'Avatar Plaza': '形象廣場',
    'Avatar created': '形象建立成功',
    'Avatar deleted': '形象已刪除',
    'Avatar name is required': '請填寫形象名稱',
    'Avatar name is too long (max 191 characters)':
      '形象名稱過長（最多 191 字元）',
    'Avatar updated': '形象更新成功',
    'Black': '黑人',
    'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      '欄位：name（形象名稱）、description（簡介）、image_url（圖片位址）、gender（性別）、age_range（年齡段）、race（種族）、scenes（適合情境）、voice_id（音色 ID）、enabled（是否上架）、sort_order（排序）。匯出目前列表即可取得填好表頭的範本。',
    'Customer service girl': '客服小雨',
    'Delete Avatar': '刪除形象',
    'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.':
      '確定刪除「{{name}}」？圖片檔案會保留在磁碟上，但該記錄無法復原。',
    'Edit Avatar': '編輯形象',
    'Image': '圖片',
    'Image must be 10MB or smaller': '圖片不能超過 10MB',
    'Image must be an /uploads path or an http(s) URL':
      '圖片位址必須是 /uploads 路徑或 http(s) 位址',
    'Image preview': '圖片預覽',
    'Image uploaded': '圖片上傳成功',
    'Import Avatars': '匯入形象',
    'Latino': '拉丁裔',
    'Middle Eastern': '中東人',
    'Mixed': '混血',
    'New Avatar': '新增形象',
    'No avatars yet': '尚無形象',
    'No image': '尚無圖片',
    'No sample audio for this voice': '此音色尚無範例音訊',
    'No voice linked': '未連結音色',
    'Off-shelf personas stay hidden from the public list.':
      '已下架的形象不會出現在公開列表中。',
    'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.':
      '已上架的形象以唯讀方式公開，無需驗證即可呼叫；每筆資料都會帶上所連結音色的範例音訊。',
    'Public Data': '公共資料',
    'Race': '種族',
    'Remove image': '移除圖片',
    'Search by name or voice id': '依名稱或音色 ID 搜尋',
    'South Asian': '南亞人',
    'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.':
      '音色 ID 即呼叫 TTS 時送出的 voice_type，其範例音訊取自音色廣場。',
    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.':
      '填寫音色廣場中某個音色的 voice_type，範例音訊會自動帶出。',
    'This voice id is not in the Voice Plaza, so no sample audio is available.':
      '此音色 ID 不在音色廣場中，尚無範例音訊。',
    'Total {{count}} avatars': '共 {{count}} 個形象',
    'Upload a CSV exported from this page. name and image_url are required.':
      '上傳從本頁匯出的 CSV 檔案；name 與 image_url 為必需欄位。',
    'Upload image': '上傳圖片',
    'Voice ID': '音色 ID',
    'Voice Sample': '音色範例',
    'Voice id is too long (max 191 characters)': '音色 ID 過長（最多 191 字元）',
    'What this persona looks like and does': '描述這個形象的外型與人設',
    'White': '白人',
    'https://… or /uploads/images/…': 'https://… 或 /uploads/images/…',
  },
  fr: {
    'Asian': 'Asiatique',
    'Avatar List API': 'API de liste des avatars',
    'Avatar Name': "Nom de l'avatar",
    'Avatar Plaza': 'Place des avatars',
    'Avatar created': 'Avatar créé',
    'Avatar deleted': 'Avatar supprimé',
    'Avatar name is required': "Le nom de l'avatar est obligatoire",
    'Avatar name is too long (max 191 characters)':
      "Le nom de l'avatar est trop long (191 caractères max)",
    'Avatar updated': 'Avatar mis à jour',
    'Black': 'Noir',
    'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Colonnes : name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Exportez la liste actuelle pour obtenir un modèle prérempli.',
    'Customer service girl': 'Hôtesse de service client',
    'Delete Avatar': "Supprimer l'avatar",
    'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.':
      "Supprimer « {{name}} » ? Le fichier image reste sur le disque, mais l'enregistrement ne peut pas être restauré.",
    'Edit Avatar': "Modifier l'avatar",
    'Image': 'Image',
    'Image must be 10MB or smaller': "L'image doit faire 10 Mo ou moins",
    'Image must be an /uploads path or an http(s) URL':
      "L'image doit être un chemin /uploads ou une URL http(s)",
    'Image preview': "Aperçu de l'image",
    'Image uploaded': 'Image téléversée',
    'Import Avatars': 'Importer des avatars',
    'Latino': 'Latino-américain',
    'Middle Eastern': 'Moyen-Oriental',
    'Mixed': 'Métis',
    'New Avatar': 'Nouvel avatar',
    'No avatars yet': 'Aucun avatar pour le moment',
    'No image': 'Aucune image',
    'No sample audio for this voice': 'Aucun extrait audio pour cette voix',
    'No voice linked': 'Aucune voix associée',
    'Off-shelf personas stay hidden from the public list.':
      'Les avatars retirés restent invisibles dans la liste publique.',
    'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.':
      'Les avatars publiés sont exposés en lecture seule, sans authentification ; chaque entrée inclut l’extrait audio de la voix associée.',
    'Public Data': 'Données publiques',
    'Race': 'Ethnie',
    'Remove image': "Retirer l'image",
    'Search by name or voice id': 'Rechercher par nom ou ID de voix',
    'South Asian': 'Sud-Asiatique',
    'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.':
      "L'ID de voix est le voice_type envoyé à la requête TTS ; son extrait audio provient de la place des voix.",
    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.':
      "Le voice_type d'une voix de la place des voix ; son extrait audio s'affiche automatiquement.",
    'This voice id is not in the Voice Plaza, so no sample audio is available.':
      "Cet ID de voix n'est pas dans la place des voix : aucun extrait audio disponible.",
    'Total {{count}} avatars': '{{count}} avatars au total',
    'Upload a CSV exported from this page. name and image_url are required.':
      'Téléversez un CSV exporté depuis cette page ; name et image_url sont obligatoires.',
    'Upload image': 'Téléverser une image',
    'Voice ID': 'ID de voix',
    'Voice Sample': 'Extrait de voix',
    'Voice id is too long (max 191 characters)':
      "L'ID de voix est trop long (191 caractères max)",
    'What this persona looks like and does':
      'Décrivez le physique et le rôle de cet avatar',
    'White': 'Blanc',
    'https://… or /uploads/images/…': 'https://… ou /uploads/images/…',
  },
  ja: {
    'Asian': 'アジア系',
    'Avatar List API': 'アバターリスト API',
    'Avatar Name': 'アバター名',
    'Avatar Plaza': 'アバター広場',
    'Avatar created': 'アバターを作成しました',
    'Avatar deleted': 'アバターを削除しました',
    'Avatar name is required': 'アバター名を入力してください',
    'Avatar name is too long (max 191 characters)':
      'アバター名が長すぎます（最大191文字）',
    'Avatar updated': 'アバターを更新しました',
    'Black': '黒人',
    'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order。現在のリストをエクスポートすると、見出し入りのテンプレートになります。',
    'Customer service girl': 'カスタマーサポートの女性',
    'Delete Avatar': 'アバターを削除',
    'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.':
      '「{{name}}」を削除しますか？画像ファイルはディスクに残りますが、このレコードは復元できません。',
    'Edit Avatar': 'アバターを編集',
    'Image': '画像',
    'Image must be 10MB or smaller': '画像は10MB以下にしてください',
    'Image must be an /uploads path or an http(s) URL':
      '画像は /uploads パスまたは http(s) URL を指定してください',
    'Image preview': '画像プレビュー',
    'Image uploaded': '画像をアップロードしました',
    'Import Avatars': 'アバターをインポート',
    'Latino': 'ラテン系',
    'Middle Eastern': '中東系',
    'Mixed': '混血',
    'New Avatar': '新規アバター',
    'No avatars yet': 'アバターはまだありません',
    'No image': '画像なし',
    'No sample audio for this voice': 'この音色のサンプル音声はありません',
    'No voice linked': '音色が未設定',
    'Off-shelf personas stay hidden from the public list.':
      '非公開のアバターは公開リストに表示されません。',
    'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.':
      '公開中のアバターは読み取り専用で公開され、認証は不要です。各項目には紐づけた音色のサンプル音声が含まれます。',
    'Public Data': '公開データ',
    'Race': '人種',
    'Remove image': '画像を削除',
    'Search by name or voice id': '名前または音色 ID で検索',
    'South Asian': '南アジア系',
    'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.':
      '音色 ID は TTS リクエストで送信される voice_type です。サンプル音声は音色広場から取得します。',
    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.':
      '音色広場にある音色の voice_type を入力すると、サンプル音声が自動的に表示されます。',
    'This voice id is not in the Voice Plaza, so no sample audio is available.':
      'この音色 ID は音色広場にないため、サンプル音声はありません。',
    'Total {{count}} avatars': 'アバターは合計 {{count}} 件',
    'Upload a CSV exported from this page. name and image_url are required.':
      'このページからエクスポートした CSV をアップロードしてください。name と image_url は必須です。',
    'Upload image': '画像をアップロード',
    'Voice ID': '音色 ID',
    'Voice Sample': '音色サンプル',
    'Voice id is too long (max 191 characters)':
      '音色 ID が長すぎます（最大191文字）',
    'What this persona looks like and does': 'このアバターの外見と設定を入力',
    'White': '白人',
    'https://… or /uploads/images/…': 'https://… または /uploads/images/…',
  },
  ru: {
    'Asian': 'Азиат',
    'Avatar List API': 'API списка образов',
    'Avatar Name': 'Название образа',
    'Avatar Plaza': 'Площадка образов',
    'Avatar created': 'Образ создан',
    'Avatar deleted': 'Образ удалён',
    'Avatar name is required': 'Укажите название образа',
    'Avatar name is too long (max 191 characters)':
      'Название образа слишком длинное (максимум 191 символ)',
    'Avatar updated': 'Образ обновлён',
    'Black': 'Темнокожий',
    'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Столбцы: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Экспортируйте текущий список, чтобы получить готовый шаблон.',
    'Customer service girl': 'Девушка из службы поддержки',
    'Delete Avatar': 'Удалить образ',
    'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.':
      'Удалить «{{name}}»? Файл изображения останется на диске, но запись восстановить нельзя.',
    'Edit Avatar': 'Изменить образ',
    'Image': 'Изображение',
    'Image must be 10MB or smaller':
      'Размер изображения не должен превышать 10 МБ',
    'Image must be an /uploads path or an http(s) URL':
      'Изображение должно быть путём /uploads или URL http(s)',
    'Image preview': 'Предпросмотр изображения',
    'Image uploaded': 'Изображение загружено',
    'Import Avatars': 'Импорт образов',
    'Latino': 'Латиноамериканец',
    'Middle Eastern': 'Ближневосточный',
    'Mixed': 'Метис',
    'New Avatar': 'Новый образ',
    'No avatars yet': 'Образов пока нет',
    'No image': 'Нет изображения',
    'No sample audio for this voice': 'Для этого голоса нет образца аудио',
    'No voice linked': 'Голос не привязан',
    'Off-shelf personas stay hidden from the public list.':
      'Снятые с публикации образы не попадают в публичный список.',
    'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.':
      'Опубликованные образы доступны только для чтения, без аутентификации; каждый элемент содержит образец аудио привязанного голоса.',
    'Public Data': 'Публичные данные',
    'Race': 'Раса',
    'Remove image': 'Убрать изображение',
    'Search by name or voice id': 'Поиск по названию или ID голоса',
    'South Asian': 'Южноазиатский',
    'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.':
      'ID голоса — это voice_type, отправляемый в запросе TTS; образец аудио берётся с площадки голосов.',
    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.':
      'Укажите voice_type голоса с площадки голосов — образец аудио подставится автоматически.',
    'This voice id is not in the Voice Plaza, so no sample audio is available.':
      'Этого ID нет на площадке голосов, поэтому образец аудио недоступен.',
    'Total {{count}} avatars': 'Всего образов: {{count}}',
    'Upload a CSV exported from this page. name and image_url are required.':
      'Загрузите CSV, экспортированный с этой страницы; столбцы name и image_url обязательны.',
    'Upload image': 'Загрузить изображение',
    'Voice ID': 'ID голоса',
    'Voice Sample': 'Образец голоса',
    'Voice id is too long (max 191 characters)':
      'ID голоса слишком длинный (максимум 191 символ)',
    'What this persona looks like and does':
      'Опишите внешность и роль этого образа',
    'White': 'Белый',
    'https://… or /uploads/images/…': 'https://… или /uploads/images/…',
  },
  vi: {
    'Asian': 'Người châu Á',
    'Avatar List API': 'API danh sách hình tượng',
    'Avatar Name': 'Tên hình tượng',
    'Avatar Plaza': 'Quảng trường hình tượng',
    'Avatar created': 'Đã tạo hình tượng',
    'Avatar deleted': 'Đã xóa hình tượng',
    'Avatar name is required': 'Vui lòng nhập tên hình tượng',
    'Avatar name is too long (max 191 characters)':
      'Tên hình tượng quá dài (tối đa 191 ký tự)',
    'Avatar updated': 'Đã cập nhật hình tượng',
    'Black': 'Người da đen',
    'Columns: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Export the current list to get a filled-in template.':
      'Các cột: name, description, image_url, gender, age_range, race, scenes, voice_id, enabled, sort_order. Hãy xuất danh sách hiện tại để có mẫu có sẵn tiêu đề.',
    'Customer service girl': 'Cô gái chăm sóc khách hàng',
    'Delete Avatar': 'Xóa hình tượng',
    'Delete "{{name}}"? Its picture file stays on disk, but the row cannot be restored.':
      'Xóa "{{name}}"? Tệp hình ảnh vẫn còn trên ổ đĩa, nhưng bản ghi này không thể khôi phục.',
    'Edit Avatar': 'Sửa hình tượng',
    'Image': 'Hình ảnh',
    'Image must be 10MB or smaller': 'Hình ảnh phải có dung lượng từ 10MB trở xuống',
    'Image must be an /uploads path or an http(s) URL':
      'Hình ảnh phải là đường dẫn /uploads hoặc URL http(s)',
    'Image preview': 'Xem trước hình ảnh',
    'Image uploaded': 'Đã tải hình ảnh lên',
    'Import Avatars': 'Nhập hình tượng',
    'Latino': 'Người Mỹ Latinh',
    'Middle Eastern': 'Người Trung Đông',
    'Mixed': 'Người lai',
    'New Avatar': 'Hình tượng mới',
    'No avatars yet': 'Chưa có hình tượng nào',
    'No image': 'Chưa có hình ảnh',
    'No sample audio for this voice': 'Giọng đọc này chưa có âm thanh mẫu',
    'No voice linked': 'Chưa liên kết giọng đọc',
    'Off-shelf personas stay hidden from the public list.':
      'Hình tượng đã gỡ sẽ không hiển thị trong danh sách công khai.',
    'On-shelf personas are published read-only; no authentication is required. Each item carries the sample audio of its linked voice.':
      'Hình tượng đang phát hành chỉ cho phép đọc, không cần xác thực; mỗi mục đều kèm âm thanh mẫu của giọng đọc đã liên kết.',
    'Public Data': 'Dữ liệu công khai',
    'Race': 'Chủng tộc',
    'Remove image': 'Xóa hình ảnh',
    'Search by name or voice id': 'Tìm theo tên hoặc ID giọng đọc',
    'South Asian': 'Người Nam Á',
    'The voice id is the voice_type a TTS request sends; its sample audio comes from the Voice Plaza.':
      'ID giọng đọc chính là voice_type gửi trong yêu cầu TTS; âm thanh mẫu lấy từ Quảng trường giọng đọc.',
    'The voice_type of a voice in the Voice Plaza; its sample audio is shown automatically.':
      'Nhập voice_type của một giọng đọc trong Quảng trường giọng đọc; âm thanh mẫu sẽ tự động hiển thị.',
    'This voice id is not in the Voice Plaza, so no sample audio is available.':
      'ID giọng đọc này không có trong Quảng trường giọng đọc nên chưa có âm thanh mẫu.',
    'Total {{count}} avatars': 'Tổng {{count}} hình tượng',
    'Upload a CSV exported from this page. name and image_url are required.':
      'Tải lên tệp CSV xuất từ trang này; name và image_url là bắt buộc.',
    'Upload image': 'Tải hình ảnh lên',
    'Voice ID': 'ID giọng đọc',
    'Voice Sample': 'Âm thanh mẫu',
    'Voice id is too long (max 191 characters)':
      'ID giọng đọc quá dài (tối đa 191 ký tự)',
    'What this persona looks like and does':
      'Mô tả ngoại hình và vai trò của hình tượng này',
    'White': 'Người da trắng',
    'https://… or /uploads/images/…': 'https://… hoặc /uploads/images/…',
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
