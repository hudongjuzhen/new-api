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

// One-off migration script: adds the Voice Plaza (zsy-voice) keys to all seven
// locales. Run from web/: node scripts/add-voice-plaza-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Audio file must be 20MB or smaller': 'Audio file must be 20MB or smaller',
    'Audio Sample': 'Audio Sample',
    'Audio uploaded': 'Audio uploaded',
    'Copy endpoint': 'Copy endpoint',
    'Delete Voice': 'Delete Voice',
    'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.':
      'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.',
    'Edit Voice': 'Edit Voice',
    Introduction: 'Introduction',
    'Introduction is too long (max 2000 characters)':
      'Introduction is too long (max 2000 characters)',
    'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB':
      'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB',
    'New Voice': 'New Voice',
    'No audio sample': 'No audio sample',
    'No voices yet': 'No voices yet',
    'Off Shelf': 'Off Shelf',
    'Off-shelf voices stay hidden from the public list.':
      'Off-shelf voices stay hidden from the public list.',
    'On Shelf': 'On Shelf',
    'On-shelf voices are published read-only; no authentication is required.':
      'On-shelf voices are published read-only; no authentication is required.',
    'Page size': 'Page size',
    'Page {{page}} / {{total}}': 'Page {{page}} / {{total}}',
    Pagination: 'Pagination',
    'Remove Audio': 'Remove Audio',
    'Sample audio is optional; a voice without one still works.':
      'Sample audio is optional; a voice without one still works.',
    'Search by name or voice_type': 'Search by name or voice_type',
    'Shelf Status': 'Shelf Status',
    'Smaller numbers come first.': 'Smaller numbers come first.',
    'Sort order is out of range': 'Sort order is out of range',
    'Sort order must be a whole number': 'Sort order must be a whole number',
    'Sweet Female Voice': 'Sweet Female Voice',
    'The voice_type is the value a TTS request sends to the upstream provider.':
      'The voice_type is the value a TTS request sends to the upstream provider.',
    'Total {{count}} voices': 'Total {{count}} voices',
    'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)':
      'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)',
    'Voice created': 'Voice created',
    'Voice deleted': 'Voice deleted',
    'Voice List API': 'Voice List API',
    'Voice Name': 'Voice Name',
    'Voice name is required': 'Voice name is required',
    'Voice name is too long (max 191 characters)':
      'Voice name is too long (max 191 characters)',
    'Voice Plaza': 'Voice Plaza',
    'Voice updated': 'Voice updated',
    'What this voice sounds like': 'What this voice sounds like',
    '{{count}} per page': '{{count}} per page',
    'voice_type is required': 'voice_type is required',
    'voice_type is too long (max 191 characters)':
      'voice_type is too long (max 191 characters)',
  },
  zh: {
    'Audio file must be 20MB or smaller': '音频文件不能超过 20MB',
    'Audio Sample': '示例音频',
    'Audio uploaded': '音频上传成功',
    'Copy endpoint': '复制接口地址',
    'Delete Voice': '删除音色',
    'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.':
      '确定删除「{{name}}」？示例音频文件会保留在磁盘上，但该记录无法恢复。',
    'Edit Voice': '编辑音色',
    Introduction: '简介',
    'Introduction is too long (max 2000 characters)':
      '简介过长（最多 2000 字符）',
    'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB':
      '支持 MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM，最大 20MB',
    'New Voice': '新建音色',
    'No audio sample': '暂无示例音频',
    'No voices yet': '暂无音色',
    'Off Shelf': '已下架',
    'Off-shelf voices stay hidden from the public list.':
      '已下架的音色不会出现在公开列表中。',
    'On Shelf': '已上架',
    'On-shelf voices are published read-only; no authentication is required.':
      '已上架的音色以只读方式公开，无需鉴权即可调用。',
    'Page size': '每页数量',
    'Page {{page}} / {{total}}': '第 {{page}} / {{total}} 页',
    Pagination: '分页',
    'Remove Audio': '移除音频',
    'Sample audio is optional; a voice without one still works.':
      '示例音频可选，没有音频的音色同样可以正常使用。',
    'Search by name or voice_type': '按名称或 voice_type 搜索',
    'Shelf Status': '上架状态',
    'Smaller numbers come first.': '数字越小越靠前。',
    'Sort order is out of range': '排序值超出范围',
    'Sort order must be a whole number': '排序值必须是整数',
    'Sweet Female Voice': '温柔女声',
    'The voice_type is the value a TTS request sends to the upstream provider.':
      'voice_type 是调用上游 TTS 时实际发送的音色标识。',
    'Total {{count}} voices': '共 {{count}} 个音色',
    'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)':
      '不支持的音频格式（MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM）',
    'Voice created': '音色创建成功',
    'Voice deleted': '音色已删除',
    'Voice List API': '音色列表接口',
    'Voice Name': '音色名称',
    'Voice name is required': '请填写音色名称',
    'Voice name is too long (max 191 characters)':
      '音色名称过长（最多 191 字符）',
    'Voice Plaza': '音色广场',
    'Voice updated': '音色更新成功',
    'What this voice sounds like': '描述这个音色的音质与适用场景',
    '{{count}} per page': '每页 {{count}} 条',
    'voice_type is required': '请填写 voice_type',
    'voice_type is too long (max 191 characters)':
      'voice_type 过长（最多 191 字符）',
  },
  'zh-TW': {
    'Audio file must be 20MB or smaller': '音訊檔案不能超過 20MB',
    'Audio Sample': '範例音訊',
    'Audio uploaded': '音訊上傳成功',
    'Copy endpoint': '複製介面位址',
    'Delete Voice': '刪除音色',
    'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.':
      '確定刪除「{{name}}」？範例音訊檔案會保留在磁碟上，但該記錄無法復原。',
    'Edit Voice': '編輯音色',
    Introduction: '簡介',
    'Introduction is too long (max 2000 characters)':
      '簡介過長（最多 2000 字元）',
    'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB':
      '支援 MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM，最大 20MB',
    'New Voice': '新增音色',
    'No audio sample': '尚無範例音訊',
    'No voices yet': '尚無音色',
    'Off Shelf': '已下架',
    'Off-shelf voices stay hidden from the public list.':
      '已下架的音色不會出現在公開列表中。',
    'On Shelf': '已上架',
    'On-shelf voices are published read-only; no authentication is required.':
      '已上架的音色以唯讀方式公開，無需驗證即可呼叫。',
    'Page size': '每頁數量',
    'Page {{page}} / {{total}}': '第 {{page}} / {{total}} 頁',
    Pagination: '分頁',
    'Remove Audio': '移除音訊',
    'Sample audio is optional; a voice without one still works.':
      '範例音訊為選填，沒有音訊的音色仍可正常使用。',
    'Search by name or voice_type': '依名稱或 voice_type 搜尋',
    'Shelf Status': '上架狀態',
    'Smaller numbers come first.': '數字越小越前面。',
    'Sort order is out of range': '排序值超出範圍',
    'Sort order must be a whole number': '排序值必須是整數',
    'Sweet Female Voice': '溫柔女聲',
    'The voice_type is the value a TTS request sends to the upstream provider.':
      'voice_type 是呼叫上游 TTS 時實際送出的音色識別碼。',
    'Total {{count}} voices': '共 {{count}} 個音色',
    'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)':
      '不支援的音訊格式（MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM）',
    'Voice created': '音色建立成功',
    'Voice deleted': '音色已刪除',
    'Voice List API': '音色列表介面',
    'Voice Name': '音色名稱',
    'Voice name is required': '請填寫音色名稱',
    'Voice name is too long (max 191 characters)':
      '音色名稱過長（最多 191 字元）',
    'Voice Plaza': '音色廣場',
    'Voice updated': '音色更新成功',
    'What this voice sounds like': '描述這個音色的音質與適用情境',
    '{{count}} per page': '每頁 {{count}} 筆',
    'voice_type is required': '請填寫 voice_type',
    'voice_type is too long (max 191 characters)':
      'voice_type 過長（最多 191 字元）',
  },
  fr: {
    'Audio file must be 20MB or smaller':
      'Le fichier audio doit faire 20 Mo ou moins',
    'Audio Sample': 'Extrait audio',
    'Audio uploaded': 'Audio téléversé',
    'Copy endpoint': "Copier l'adresse",
    'Delete Voice': 'Supprimer la voix',
    'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.':
      "Supprimer « {{name}} » ? Le fichier audio d'exemple reste sur le disque, mais l'enregistrement ne peut pas être restauré.",
    'Edit Voice': 'Modifier la voix',
    Introduction: 'Présentation',
    'Introduction is too long (max 2000 characters)':
      'La présentation est trop longue (2000 caractères max)',
    'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB':
      "MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, jusqu'à 20 Mo",
    'New Voice': 'Nouvelle voix',
    'No audio sample': 'Aucun extrait audio',
    'No voices yet': 'Aucune voix pour le moment',
    'Off Shelf': 'Retirée',
    'Off-shelf voices stay hidden from the public list.':
      'Les voix retirées restent invisibles dans la liste publique.',
    'On Shelf': 'Publiée',
    'On-shelf voices are published read-only; no authentication is required.':
      'Les voix publiées sont exposées en lecture seule, sans authentification.',
    'Page size': 'Taille de page',
    'Page {{page}} / {{total}}': 'Page {{page}} / {{total}}',
    Pagination: 'Pagination',
    'Remove Audio': "Retirer l'audio",
    'Sample audio is optional; a voice without one still works.':
      "L'extrait audio est facultatif ; une voix sans extrait fonctionne quand même.",
    'Search by name or voice_type': 'Rechercher par nom ou voice_type',
    'Shelf Status': 'État de publication',
    'Smaller numbers come first.':
      'Les nombres plus petits passent en premier.',
    'Sort order is out of range': "L'ordre de tri est hors limites",
    'Sort order must be a whole number':
      "L'ordre de tri doit être un nombre entier",
    'Sweet Female Voice': 'Voix féminine douce',
    'The voice_type is the value a TTS request sends to the upstream provider.':
      'Le voice_type est la valeur envoyée au fournisseur TTS en amont.',
    'Total {{count}} voices': '{{count}} voix au total',
    'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)':
      'Format audio non pris en charge (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)',
    'Voice created': 'Voix créée',
    'Voice deleted': 'Voix supprimée',
    'Voice List API': 'API de liste des voix',
    'Voice Name': 'Nom de la voix',
    'Voice name is required': 'Le nom de la voix est obligatoire',
    'Voice name is too long (max 191 characters)':
      'Le nom de la voix est trop long (191 caractères max)',
    'Voice Plaza': 'Place des voix',
    'Voice updated': 'Voix mise à jour',
    'What this voice sounds like': 'Décrivez le timbre de cette voix',
    '{{count}} per page': '{{count}} par page',
    'voice_type is required': 'Le voice_type est obligatoire',
    'voice_type is too long (max 191 characters)':
      'Le voice_type est trop long (191 caractères max)',
  },
  ja: {
    'Audio file must be 20MB or smaller':
      '音声ファイルは20MB以下にしてください',
    'Audio Sample': 'サンプル音声',
    'Audio uploaded': '音声をアップロードしました',
    'Copy endpoint': 'エンドポイントをコピー',
    'Delete Voice': '音色を削除',
    'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.':
      '「{{name}}」を削除しますか？サンプル音声ファイルはディスクに残りますが、このレコードは復元できません。',
    'Edit Voice': '音色を編集',
    Introduction: '紹介文',
    'Introduction is too long (max 2000 characters)':
      '紹介文が長すぎます（最大2000文字）',
    'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB':
      'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM、最大20MB',
    'New Voice': '新規音色',
    'No audio sample': 'サンプル音声なし',
    'No voices yet': '音色はまだありません',
    'Off Shelf': '非公開',
    'Off-shelf voices stay hidden from the public list.':
      '非公開の音色は公開リストに表示されません。',
    'On Shelf': '公開中',
    'On-shelf voices are published read-only; no authentication is required.':
      '公開中の音色は読み取り専用で公開され、認証は不要です。',
    'Page size': '表示件数',
    'Page {{page}} / {{total}}': '{{page}} / {{total}} ページ',
    Pagination: 'ページネーション',
    'Remove Audio': '音声を削除',
    'Sample audio is optional; a voice without one still works.':
      'サンプル音声は任意です。音声がなくても音色は利用できます。',
    'Search by name or voice_type': '名前または voice_type で検索',
    'Shelf Status': '公開状態',
    'Smaller numbers come first.': '数値が小さいほど前に表示されます。',
    'Sort order is out of range': '並び順が範囲外です',
    'Sort order must be a whole number': '並び順は整数で入力してください',
    'Sweet Female Voice': 'やさしい女性の声',
    'The voice_type is the value a TTS request sends to the upstream provider.':
      'voice_type は上流の TTS に送信される音色識別子です。',
    'Total {{count}} voices': '音色は合計 {{count}} 件',
    'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)':
      '対応していない音声形式です（MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM）',
    'Voice created': '音色を作成しました',
    'Voice deleted': '音色を削除しました',
    'Voice List API': '音色リスト API',
    'Voice Name': '音色名',
    'Voice name is required': '音色名を入力してください',
    'Voice name is too long (max 191 characters)':
      '音色名が長すぎます（最大191文字）',
    'Voice Plaza': '音色プラザ',
    'Voice updated': '音色を更新しました',
    'What this voice sounds like': 'この音色の特徴を入力',
    '{{count}} per page': '1ページ {{count}} 件',
    'voice_type is required': 'voice_type を入力してください',
    'voice_type is too long (max 191 characters)':
      'voice_type が長すぎます（最大191文字）',
  },
  ru: {
    'Audio file must be 20MB or smaller':
      'Размер аудиофайла не должен превышать 20 МБ',
    'Audio Sample': 'Образец аудио',
    'Audio uploaded': 'Аудио загружено',
    'Copy endpoint': 'Скопировать адрес',
    'Delete Voice': 'Удалить голос',
    'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.':
      'Удалить «{{name}}»? Файл образца аудио останется на диске, но запись восстановить нельзя.',
    'Edit Voice': 'Изменить голос',
    Introduction: 'Описание',
    'Introduction is too long (max 2000 characters)':
      'Описание слишком длинное (максимум 2000 символов)',
    'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB':
      'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, до 20 МБ',
    'New Voice': 'Новый голос',
    'No audio sample': 'Нет образца аудио',
    'No voices yet': 'Голосов пока нет',
    'Off Shelf': 'Снят с публикации',
    'Off-shelf voices stay hidden from the public list.':
      'Снятые с публикации голоса не попадают в публичный список.',
    'On Shelf': 'Опубликован',
    'On-shelf voices are published read-only; no authentication is required.':
      'Опубликованные голоса доступны только для чтения, без аутентификации.',
    'Page size': 'Записей на странице',
    'Page {{page}} / {{total}}': 'Страница {{page}} / {{total}}',
    Pagination: 'Пагинация',
    'Remove Audio': 'Убрать аудио',
    'Sample audio is optional; a voice without one still works.':
      'Образец аудио необязателен: голос работает и без него.',
    'Search by name or voice_type': 'Поиск по названию или voice_type',
    'Shelf Status': 'Статус публикации',
    'Smaller numbers come first.': 'Меньшее значение — выше в списке.',
    'Sort order is out of range': 'Порядок сортировки вне диапазона',
    'Sort order must be a whole number':
      'Порядок сортировки должен быть целым числом',
    'Sweet Female Voice': 'Мягкий женский голос',
    'The voice_type is the value a TTS request sends to the upstream provider.':
      'voice_type — значение, которое отправляется вышестоящему TTS-провайдеру.',
    'Total {{count}} voices': 'Всего голосов: {{count}}',
    'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)':
      'Неподдерживаемый формат аудио (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)',
    'Voice created': 'Голос создан',
    'Voice deleted': 'Голос удалён',
    'Voice List API': 'API списка голосов',
    'Voice Name': 'Название голоса',
    'Voice name is required': 'Укажите название голоса',
    'Voice name is too long (max 191 characters)':
      'Название голоса слишком длинное (максимум 191 символ)',
    'Voice Plaza': 'Площадка голосов',
    'Voice updated': 'Голос обновлён',
    'What this voice sounds like': 'Опишите, как звучит этот голос',
    '{{count}} per page': 'По {{count}} на странице',
    'voice_type is required': 'Укажите voice_type',
    'voice_type is too long (max 191 characters)':
      'voice_type слишком длинный (максимум 191 символ)',
  },
  vi: {
    'Audio file must be 20MB or smaller':
      'Tệp âm thanh phải có dung lượng từ 20MB trở xuống',
    'Audio Sample': 'Âm thanh mẫu',
    'Audio uploaded': 'Đã tải âm thanh lên',
    'Copy endpoint': 'Sao chép địa chỉ API',
    'Delete Voice': 'Xóa giọng đọc',
    'Delete "{{name}}"? Its sample audio file stays on disk, but the row cannot be restored.':
      'Xóa "{{name}}"? Tệp âm thanh mẫu vẫn còn trên ổ đĩa, nhưng bản ghi này không thể khôi phục.',
    'Edit Voice': 'Sửa giọng đọc',
    Introduction: 'Giới thiệu',
    'Introduction is too long (max 2000 characters)':
      'Phần giới thiệu quá dài (tối đa 2000 ký tự)',
    'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, up to 20MB':
      'MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM, tối đa 20MB',
    'New Voice': 'Giọng đọc mới',
    'No audio sample': 'Chưa có âm thanh mẫu',
    'No voices yet': 'Chưa có giọng đọc nào',
    'Off Shelf': 'Đã gỡ',
    'Off-shelf voices stay hidden from the public list.':
      'Giọng đọc đã gỡ sẽ không hiển thị trong danh sách công khai.',
    'On Shelf': 'Đang phát hành',
    'On-shelf voices are published read-only; no authentication is required.':
      'Giọng đọc đang phát hành chỉ cho phép đọc, không cần xác thực.',
    'Page size': 'Số mục mỗi trang',
    'Page {{page}} / {{total}}': 'Trang {{page}} / {{total}}',
    Pagination: 'Phân trang',
    'Remove Audio': 'Xóa âm thanh',
    'Sample audio is optional; a voice without one still works.':
      'Âm thanh mẫu là tùy chọn; giọng đọc vẫn dùng được khi không có.',
    'Search by name or voice_type': 'Tìm theo tên hoặc voice_type',
    'Shelf Status': 'Trạng thái phát hành',
    'Smaller numbers come first.': 'Số nhỏ hơn sẽ hiển thị trước.',
    'Sort order is out of range': 'Thứ tự sắp xếp ngoài phạm vi',
    'Sort order must be a whole number': 'Thứ tự sắp xếp phải là số nguyên',
    'Sweet Female Voice': 'Giọng nữ dịu dàng',
    'The voice_type is the value a TTS request sends to the upstream provider.':
      'voice_type là giá trị được gửi tới nhà cung cấp TTS thượng nguồn.',
    'Total {{count}} voices': 'Tổng {{count}} giọng đọc',
    'Unsupported audio format (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)':
      'Định dạng âm thanh không được hỗ trợ (MP3 / WAV / M4A / AAC / OGG / OPUS / FLAC / WEBM)',
    'Voice created': 'Đã tạo giọng đọc',
    'Voice deleted': 'Đã xóa giọng đọc',
    'Voice List API': 'API danh sách giọng đọc',
    'Voice Name': 'Tên giọng đọc',
    'Voice name is required': 'Vui lòng nhập tên giọng đọc',
    'Voice name is too long (max 191 characters)':
      'Tên giọng đọc quá dài (tối đa 191 ký tự)',
    'Voice Plaza': 'Quảng trường giọng đọc',
    'Voice updated': 'Đã cập nhật giọng đọc',
    'What this voice sounds like': 'Mô tả chất giọng của giọng đọc này',
    '{{count}} per page': '{{count}} mỗi trang',
    'voice_type is required': 'Vui lòng nhập voice_type',
    'voice_type is too long (max 191 characters)':
      'voice_type quá dài (tối đa 191 ký tự)',
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
