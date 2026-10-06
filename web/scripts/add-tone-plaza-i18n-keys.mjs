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

// One-off migration script: adds the Tone Plaza (zsy-tone) keys to all seven
// locales. Run from web/: node scripts/add-tone-plaza-i18n-keys.mjs
//
// Note what is deliberately *absent* from the table below. The tone plaza shares
// a large vocabulary of interface wording with the voice plaza — `Introduction`,
// `Sort Order`, `Shelf Status`, every CSV-import string, and the shared form
// validators ("Introduction is too long (max 2000 characters)", "At most 8
// scenes", "Sort order is out of range", …). Those keys already exist in all
// seven files because the voice plaza shipped them, and reusing them for the
// same sentences is the point: the two catalogs then read identically in every
// language, and fixing a wording means fixing one key rather than two. Adding
// them here would only risk overwriting a translation another page relies on.
//
// So this table holds exactly the tone plaza's own strings: the categories and
// tones of the published standard, the tone-specific labels (prompt, sample
// pair), the standard-version wording, and the tone-specific validation
// messages. Every key with a number in it quotes the fallback limit from
// lib/tone-fields.ts; if the backend raises a cap, the message degrades to the
// English sentence carrying the new number instead of quoting a stale one.
//
// The category and tone labels follow docs/zsy-tone-standard.md §3/§4 verbatim
// (口播 for `spoken`, 冷静 for `calm`): the standard is the definition of those
// words, and a label invented here would make the plaza disagree with the
// document it publishes.
const LOCALES_DIR = path.resolve('src/i18n/locales')

/**
 * Keys an earlier draft of this extension shipped and no longer uses.
 *
 * They are listed rather than left behind so the script is re-runnable: a second
 * run would otherwise add the corrected keys while the superseded ones lingered
 * in all seven files, unused and slowly wrong. Each was renamed after reading the
 * published contract — the catalog's `keyword` search covers the prompt and the
 * sample pair as well as the name, and the CSV importer requires `prompt` in
 * addition to `name`.
 */
const retiredKeys = [
  'Search by name or prompt',
  'Upload a CSV exported from this page. name is required.',
]

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    // --- the published standard's controlled vocabularies ---
    Academic: 'Academic',
    Business: 'Business',
    Calm: 'Calm',
    Humorous: 'Humorous',
    Literary: 'Literary',
    Lively: 'Lively',
    Media: 'Media',
    Plain: 'Plain',
    Sharp: 'Sharp',
    Solemn: 'Solemn',
    Spoken: 'Spoken',
    Technical: 'Technical',
    Warm: 'Warm',

    // --- page chrome ---
    'Tone Plaza': 'Tone Plaza',
    'Tone Standard': 'Tone Standard',
    'Tone Name': 'Tone Name',
    Tone: 'Tone',
    'Tone Prompt': 'Tone Prompt',
    'New Tone': 'New Tone',
    'Edit Tone': 'Edit Tone',
    'Delete Tone': 'Delete Tone',
    'Delete "{{name}}"? This cannot be undone.':
      'Delete "{{name}}"? This cannot be undone.',
    'No tones yet': 'No tones yet',
    'Total {{count}} tones': 'Total {{count}} tones',
    'Search by name, prompt or sample': 'Search by name, prompt or sample',
    'Tone created': 'Tone created',
    'Tone updated': 'Tone updated',
    'Tone deleted': 'Tone deleted',

    // --- the sample pair (this plaza's stand-in for an audio player) ---
    Sample: 'Sample',
    'Sample Input': 'Sample Input',
    'Sample Output': 'Sample Output',
    'A sentence written plainly': 'A sentence written plainly',
    'The same sentence in this tone': 'The same sentence in this tone',
    'Optional before / after pair showing what this tone does.':
      'Optional before / after pair showing what this tone does.',

    // --- form hints ---
    'The tone prompt is the instruction downstream models follow.':
      'The tone prompt is the instruction downstream models follow.',
    'Warm and composed': 'Warm and composed',
    'What this tone reads like': 'What this tone reads like',
    'The instruction downstream models follow':
      'The instruction downstream models follow',
    'Paste the wording a model should follow. This is the field consumers actually use.':
      'Paste the wording a model should follow. This is the field consumers actually use.',
    'Separate scenes with a comma.': 'Separate scenes with a comma.',
    'Newsletter, product copy, documentation':
      'Newsletter, product copy, documentation',
    'Off-shelf tones stay hidden from the public list.':
      'Off-shelf tones stay hidden from the public list.',

    // --- endpoints ---
    'Tone List API': 'Tone List API',
    'Tone Standard API': 'Tone Standard API',
    'On-shelf tones are published read-only; no authentication is required.':
      'On-shelf tones are published read-only; no authentication is required.',
    'Vocabulary, field caps and compatibility promises every consumer can rely on.':
      'Vocabulary, field caps and compatibility promises every consumer can rely on.',
    'Loaded from the published standard endpoint.':
      'Loaded from the published standard endpoint.',
    'Built-in mirror: the standard endpoint is unreachable.':
      'Built-in mirror: the standard endpoint is unreachable.',

    // --- CSV import ---
    'Import Tones': 'Import Tones',
    'Upload a CSV exported from this page. name and prompt are required.':
      'Upload a CSV exported from this page. name and prompt are required.',
    'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.':
      'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.',

    // --- validation (numbers mirror limits.name / prompt / sample / language) ---
    'Tone name is required': 'Tone name is required',
    'Tone prompt is required': 'Tone prompt is required',
    'Tone name is too long (max 191 characters)':
      'Tone name is too long (max 191 characters)',
    'Tone prompt is too long (max 8000 characters)':
      'Tone prompt is too long (max 8000 characters)',
    'Sample input is too long (max 4000 characters)':
      'Sample input is too long (max 4000 characters)',
    'Sample output is too long (max 4000 characters)':
      'Sample output is too long (max 4000 characters)',
    'Language tag is too long (max 16 characters)':
      'Language tag is too long (max 16 characters)',
  },
  zh: {
    Academic: '学术',
    Business: '商务',
    Calm: '冷静',
    Humorous: '幽默',
    Literary: '文学',
    Lively: '活泼',
    Media: '媒体',
    Plain: '平实',
    Sharp: '犀利',
    Solemn: '庄重',
    Spoken: '口播',
    Technical: '技术',
    Warm: '温暖',

    'Tone Plaza': '文风广场',
    'Tone Standard': '文风标准',
    'Tone Name': '文风名称',
    Tone: '语气',
    'Tone Prompt': '文风提示词',
    'New Tone': '新建文风',
    'Edit Tone': '编辑文风',
    'Delete Tone': '删除文风',
    'Delete "{{name}}"? This cannot be undone.':
      '确定删除「{{name}}」？此操作无法撤销。',
    'No tones yet': '暂无文风',
    'Total {{count}} tones': '共 {{count}} 条文风',
    'Search by name, prompt or sample': '按名称、提示词或示例搜索',
    'Tone created': '文风已创建',
    'Tone updated': '文风已更新',
    'Tone deleted': '文风已删除',

    Sample: '示例',
    'Sample Input': '示例原文',
    'Sample Output': '示例改写',
    'A sentence written plainly': '一句平铺直叙的话',
    'The same sentence in this tone': '同一句话用这条文风改写',
    'Optional before / after pair showing what this tone does.':
      '可选的原文／改写对照，用来说明这条文风做了什么。',

    'The tone prompt is the instruction downstream models follow.':
      '文风提示词是下游模型遵循的指令。',
    'Warm and composed': '温暖而从容',
    'What this tone reads like': '这条文风读起来是什么感觉',
    'The instruction downstream models follow': '下游模型遵循的指令内容',
    'Paste the wording a model should follow. This is the field consumers actually use.':
      '粘贴希望模型遵循的措辞。这是消费方真正使用的那一格。',
    'Separate scenes with a comma.': '多个场景用逗号分隔。',
    'Newsletter, product copy, documentation': '公众号长文、产品文案、技术文档',
    'Off-shelf tones stay hidden from the public list.':
      '已下架的文风不会出现在公开列表中。',

    'Tone List API': '文风列表接口',
    'Tone Standard API': '文风标准接口',
    'On-shelf tones are published read-only; no authentication is required.':
      '已上架的文风以只读方式公开，无需鉴权即可调用。',
    'Vocabulary, field caps and compatibility promises every consumer can rely on.':
      '取值词表、字段上限与兼容性承诺，供所有消费方依赖。',
    'Loaded from the published standard endpoint.':
      '已从公开的标准接口加载。',
    'Built-in mirror: the standard endpoint is unreachable.':
      '使用内置镜像：标准接口当前不可达。',

    'Import Tones': '导入文风',
    'Upload a CSV exported from this page. name and prompt are required.':
      '请上传从本页导出的 CSV。name 与 prompt 为必填。',
    'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name、description、prompt、category、tone、language、scenes、sample_input、sample_output、enabled、sort_order。可先导出当前列表获得带示例的模板。',

    'Tone name is required': '请填写文风名称',
    'Tone prompt is required': '请填写文风提示词',
    'Tone name is too long (max 191 characters)': '文风名称过长（最多 191 字符）',
    'Tone prompt is too long (max 8000 characters)':
      '文风提示词过长（最多 8000 字符）',
    'Sample input is too long (max 4000 characters)':
      '示例原文过长（最多 4000 字符）',
    'Sample output is too long (max 4000 characters)':
      '示例改写过长（最多 4000 字符）',
    'Language tag is too long (max 16 characters)': '语言标签过长（最多 16 字符）',
  },
  'zh-TW': {
    Academic: '學術',
    Business: '商務',
    Calm: '冷靜',
    Humorous: '幽默',
    Literary: '文學',
    Lively: '活潑',
    Media: '媒體',
    Plain: '平實',
    Sharp: '犀利',
    Solemn: '莊重',
    Spoken: '口播',
    Technical: '技術',
    Warm: '溫暖',

    'Tone Plaza': '文風廣場',
    'Tone Standard': '文風標準',
    'Tone Name': '文風名稱',
    Tone: '語氣',
    'Tone Prompt': '文風提示詞',
    'New Tone': '新建文風',
    'Edit Tone': '編輯文風',
    'Delete Tone': '刪除文風',
    'Delete "{{name}}"? This cannot be undone.':
      '確定刪除「{{name}}」？此操作無法復原。',
    'No tones yet': '暫無文風',
    'Total {{count}} tones': '共 {{count}} 條文風',
    'Search by name, prompt or sample': '按名稱、提示詞或示例搜尋',
    'Tone created': '文風已建立',
    'Tone updated': '文風已更新',
    'Tone deleted': '文風已刪除',

    Sample: '示例',
    'Sample Input': '示例原文',
    'Sample Output': '示例改寫',
    'A sentence written plainly': '一句平鋪直敘的話',
    'The same sentence in this tone': '同一句話用這條文風改寫',
    'Optional before / after pair showing what this tone does.':
      '可選的原文／改寫對照，用來說明這條文風做了什麼。',

    'The tone prompt is the instruction downstream models follow.':
      '文風提示詞是下游模型遵循的指令。',
    'Warm and composed': '溫暖而從容',
    'What this tone reads like': '這條文風讀起來是什麼感覺',
    'The instruction downstream models follow': '下游模型遵循的指令內容',
    'Paste the wording a model should follow. This is the field consumers actually use.':
      '貼上希望模型遵循的措辭。這是消費方真正使用的那一格。',
    'Separate scenes with a comma.': '多個場景用逗號分隔。',
    'Newsletter, product copy, documentation': '電子報、產品文案、技術文件',
    'Off-shelf tones stay hidden from the public list.':
      '已下架的文風不會出現在公開列表中。',

    'Tone List API': '文風列表介面',
    'Tone Standard API': '文風標準介面',
    'On-shelf tones are published read-only; no authentication is required.':
      '已上架的文風以唯讀方式公開，無需驗證即可呼叫。',
    'Vocabulary, field caps and compatibility promises every consumer can rely on.':
      '取值詞表、欄位上限與相容性承諾，供所有消費方依賴。',
    'Loaded from the published standard endpoint.':
      '已從公開的標準介面載入。',
    'Built-in mirror: the standard endpoint is unreachable.':
      '使用內建鏡像：標準介面目前無法連線。',

    'Import Tones': '匯入文風',
    'Upload a CSV exported from this page. name and prompt are required.':
      '請上傳從本頁匯出的 CSV。name 與 prompt 為必填。',
    'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.':
      '欄位：name、description、prompt、category、tone、language、scenes、sample_input、sample_output、enabled、sort_order。可先匯出目前列表取得範本。',

    'Tone name is required': '請填寫文風名稱',
    'Tone prompt is required': '請填寫文風提示詞',
    'Tone name is too long (max 191 characters)': '文風名稱過長（最多 191 字元）',
    'Tone prompt is too long (max 8000 characters)':
      '文風提示詞過長（最多 8000 字元）',
    'Sample input is too long (max 4000 characters)':
      '示例原文過長（最多 4000 字元）',
    'Sample output is too long (max 4000 characters)':
      '示例改寫過長（最多 4000 字元）',
    'Language tag is too long (max 16 characters)': '語言標籤過長（最多 16 字元）',
  },
  fr: {
    Academic: 'Académique',
    Business: 'Affaires',
    Calm: 'Calme',
    Humorous: 'Humoristique',
    Literary: 'Littéraire',
    Lively: 'Vivant',
    Media: 'Médias',
    Plain: 'Simple',
    Sharp: 'Incisif',
    Solemn: 'Solennel',
    Spoken: 'Oral',
    Technical: 'Technique',
    Warm: 'Chaleureux',

    'Tone Plaza': 'Place des styles',
    'Tone Standard': 'Standard de style',
    'Tone Name': 'Nom du style',
    Tone: 'Ton',
    'Tone Prompt': 'Invite de style',
    'New Tone': 'Nouveau style',
    'Edit Tone': 'Modifier le style',
    'Delete Tone': 'Supprimer le style',
    'Delete "{{name}}"? This cannot be undone.':
      'Supprimer « {{name}} » ? Cette action est irréversible.',
    'No tones yet': 'Aucun style pour le moment',
    'Total {{count}} tones': 'Total {{count}} styles',
    'Search by name, prompt or sample':
      'Rechercher par nom, invite ou exemple',
    'Tone created': 'Style créé',
    'Tone updated': 'Style mis à jour',
    'Tone deleted': 'Style supprimé',

    Sample: 'Exemple',
    'Sample Input': 'Texte source',
    'Sample Output': 'Texte réécrit',
    'A sentence written plainly': 'Une phrase écrite simplement',
    'The same sentence in this tone': 'La même phrase dans ce style',
    'Optional before / after pair showing what this tone does.':
      'Paire avant / après facultative montrant l’effet de ce style.',

    'The tone prompt is the instruction downstream models follow.':
      'L’invite de style est l’instruction que suivent les modèles en aval.',
    'Warm and composed': 'Chaleureux et posé',
    'What this tone reads like': 'Ce que ce style donne à lire',
    'The instruction downstream models follow':
      'L’instruction que suivent les modèles en aval',
    'Paste the wording a model should follow. This is the field consumers actually use.':
      'Collez la formulation que le modèle doit suivre. C’est le champ réellement utilisé par les consommateurs.',
    'Separate scenes with a comma.': 'Séparez les scènes par une virgule.',
    'Newsletter, product copy, documentation':
      'Newsletter, texte produit, documentation',
    'Off-shelf tones stay hidden from the public list.':
      'Les styles retirés restent absents de la liste publique.',

    'Tone List API': 'API de la liste des styles',
    'Tone Standard API': 'API du standard de style',
    'On-shelf tones are published read-only; no authentication is required.':
      'Les styles publiés sont en lecture seule ; aucune authentification n’est requise.',
    'Vocabulary, field caps and compatibility promises every consumer can rely on.':
      'Vocabulaire, limites de champs et engagements de compatibilité sur lesquels tout consommateur peut compter.',
    'Loaded from the published standard endpoint.':
      'Chargé depuis l’interface publique du standard.',
    'Built-in mirror: the standard endpoint is unreachable.':
      'Miroir intégré : l’interface du standard est injoignable.',

    'Import Tones': 'Importer des styles',
    'Upload a CSV exported from this page. name and prompt are required.':
      'Importez un CSV exporté depuis cette page. name et prompt sont obligatoires.',
    'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.':
      'Colonnes : name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Exportez la liste actuelle pour obtenir un modèle rempli.',

    'Tone name is required': 'Le nom du style est obligatoire',
    'Tone prompt is required': 'L’invite de style est obligatoire',
    'Tone name is too long (max 191 characters)':
      'Nom du style trop long (191 caractères maximum)',
    'Tone prompt is too long (max 8000 characters)':
      'Invite de style trop longue (8000 caractères maximum)',
    'Sample input is too long (max 4000 characters)':
      'Texte source trop long (4000 caractères maximum)',
    'Sample output is too long (max 4000 characters)':
      'Texte réécrit trop long (4000 caractères maximum)',
    'Language tag is too long (max 16 characters)':
      'Code de langue trop long (16 caractères maximum)',
  },
  ja: {
    Academic: '学術',
    Business: 'ビジネス',
    Calm: '落ち着いた',
    Humorous: 'ユーモラス',
    Literary: '文学',
    Lively: '活発',
    Media: 'メディア',
    Plain: '平易',
    Sharp: '鋭い',
    Solemn: '厳粛',
    Spoken: '口語',
    Technical: 'テクニカル',
    Warm: '温かい',

    'Tone Plaza': '文体広場',
    'Tone Standard': '文体標準',
    'Tone Name': '文体名',
    Tone: 'トーン',
    'Tone Prompt': '文体プロンプト',
    'New Tone': '文体を作成',
    'Edit Tone': '文体を編集',
    'Delete Tone': '文体を削除',
    'Delete "{{name}}"? This cannot be undone.':
      '「{{name}}」を削除しますか？この操作は取り消せません。',
    'No tones yet': '文体がまだありません',
    'Total {{count}} tones': '全 {{count}} 件の文体',
    'Search by name, prompt or sample': '名前・プロンプト・サンプルで検索',
    'Tone created': '文体を作成しました',
    'Tone updated': '文体を更新しました',
    'Tone deleted': '文体を削除しました',

    Sample: 'サンプル',
    'Sample Input': '元の文例',
    'Sample Output': '改写例',
    'A sentence written plainly': '平易に書かれた一文',
    'The same sentence in this tone': '同じ文をこの文体で',
    'Optional before / after pair showing what this tone does.':
      'この文体の効果を示す任意の「元の文／改写例」の組です。',

    'The tone prompt is the instruction downstream models follow.':
      '文体プロンプトは、下流のモデルが従う指示です。',
    'Warm and composed': '温かく落ち着いた',
    'What this tone reads like': 'この文体がどう読めるか',
    'The instruction downstream models follow': '下流モデルが従う指示',
    'Paste the wording a model should follow. This is the field consumers actually use.':
      'モデルに従わせたい文言を貼り付けてください。利用者が実際に使う項目です。',
    'Separate scenes with a comma.': 'シーンはカンマで区切ってください。',
    'Newsletter, product copy, documentation':
      'ニュースレター、製品コピー、ドキュメント',
    'Off-shelf tones stay hidden from the public list.':
      '非公開の文体は公開リストに表示されません。',

    'Tone List API': '文体一覧 API',
    'Tone Standard API': '文体標準 API',
    'On-shelf tones are published read-only; no authentication is required.':
      '公開中の文体は読み取り専用で、認証は不要です。',
    'Vocabulary, field caps and compatibility promises every consumer can rely on.':
      '語彙・文字数上限・互換性の約束をまとめた、すべての利用者が依存できる定義です。',
    'Loaded from the published standard endpoint.':
      '公開されている標準エンドポイントから読み込みました。',
    'Built-in mirror: the standard endpoint is unreachable.':
      '内蔵ミラーを使用中：標準エンドポイントに接続できません。',

    'Import Tones': '文体をインポート',
    'Upload a CSV exported from this page. name and prompt are required.':
      'このページからエクスポートした CSV をアップロードしてください。name と prompt は必須です。',
    'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.':
      '列：name、description、prompt、category、tone、language、scenes、sample_input、sample_output、enabled、sort_order。現在の一覧をエクスポートすると記入済みの雛形が得られます。',

    'Tone name is required': '文体名を入力してください',
    'Tone prompt is required': '文体プロンプトを入力してください',
    'Tone name is too long (max 191 characters)':
      '文体名が長すぎます（最大 191 文字）',
    'Tone prompt is too long (max 8000 characters)':
      '文体プロンプトが長すぎます（最大 8000 文字）',
    'Sample input is too long (max 4000 characters)':
      '元の文例が長すぎます（最大 4000 文字）',
    'Sample output is too long (max 4000 characters)':
      '改写例が長すぎます（最大 4000 文字）',
    'Language tag is too long (max 16 characters)':
      '言語タグが長すぎます（最大 16 文字）',
  },
  ru: {
    Academic: 'Научный',
    Business: 'Деловой',
    Calm: 'Спокойный',
    Humorous: 'Юмористический',
    Literary: 'Литературный',
    Lively: 'Живой',
    Media: 'Медиа',
    Plain: 'Простой',
    Sharp: 'Резкий',
    Solemn: 'Торжественный',
    Spoken: 'Разговорный',
    Technical: 'Технический',
    Warm: 'Тёплый',

    'Tone Plaza': 'Площадь стилей',
    'Tone Standard': 'Стандарт стиля',
    'Tone Name': 'Название стиля',
    Tone: 'Тон',
    'Tone Prompt': 'Промпт стиля',
    'New Tone': 'Новый стиль',
    'Edit Tone': 'Редактировать стиль',
    'Delete Tone': 'Удалить стиль',
    'Delete "{{name}}"? This cannot be undone.':
      'Удалить «{{name}}»? Это действие необратимо.',
    'No tones yet': 'Стилей пока нет',
    'Total {{count}} tones': 'Всего стилей: {{count}}',
    'Search by name, prompt or sample':
      'Поиск по названию, промпту или примеру',
    'Tone created': 'Стиль создан',
    'Tone updated': 'Стиль обновлён',
    'Tone deleted': 'Стиль удалён',

    Sample: 'Пример',
    'Sample Input': 'Исходный текст',
    'Sample Output': 'Переписанный текст',
    'A sentence written plainly': 'Простое предложение',
    'The same sentence in this tone': 'То же предложение в этом стиле',
    'Optional before / after pair showing what this tone does.':
      'Необязательная пара «до / после», показывающая, что делает стиль.',

    'The tone prompt is the instruction downstream models follow.':
      'Промпт стиля — это инструкция, которой следуют модели.',
    'Warm and composed': 'Тёплый и собранный',
    'What this tone reads like': 'Как читается этот стиль',
    'The instruction downstream models follow':
      'Инструкция, которой следуют модели',
    'Paste the wording a model should follow. This is the field consumers actually use.':
      'Вставьте формулировку, которой должна следовать модель. Именно это поле используют потребители.',
    'Separate scenes with a comma.': 'Разделяйте сцены запятой.',
    'Newsletter, product copy, documentation':
      'Рассылка, текст о продукте, документация',
    'Off-shelf tones stay hidden from the public list.':
      'Снятые с публикации стили скрыты из публичного списка.',

    'Tone List API': 'API списка стилей',
    'Tone Standard API': 'API стандарта стиля',
    'On-shelf tones are published read-only; no authentication is required.':
      'Опубликованные стили доступны только для чтения; аутентификация не требуется.',
    'Vocabulary, field caps and compatibility promises every consumer can rely on.':
      'Словарь значений, ограничения полей и гарантии совместимости, на которые может опираться любой потребитель.',
    'Loaded from the published standard endpoint.':
      'Загружено из опубликованного эндпоинта стандарта.',
    'Built-in mirror: the standard endpoint is unreachable.':
      'Встроенное зеркало: эндпоинт стандарта недоступен.',

    'Import Tones': 'Импорт стилей',
    'Upload a CSV exported from this page. name and prompt are required.':
      'Загрузите CSV, экспортированный с этой страницы. Поля name и prompt обязательны.',
    'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.':
      'Столбцы: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Экспортируйте текущий список, чтобы получить заполненный шаблон.',

    'Tone name is required': 'Укажите название стиля',
    'Tone prompt is required': 'Укажите промпт стиля',
    'Tone name is too long (max 191 characters)':
      'Название стиля слишком длинное (не более 191 символа)',
    'Tone prompt is too long (max 8000 characters)':
      'Промпт стиля слишком длинный (не более 8000 символов)',
    'Sample input is too long (max 4000 characters)':
      'Исходный текст слишком длинный (не более 4000 символов)',
    'Sample output is too long (max 4000 characters)':
      'Переписанный текст слишком длинный (не более 4000 символов)',
    'Language tag is too long (max 16 characters)':
      'Языковой тег слишком длинный (не более 16 символов)',
  },
  vi: {
    Academic: 'Học thuật',
    Business: 'Kinh doanh',
    Calm: 'Điềm tĩnh',
    Humorous: 'Hài hước',
    Literary: 'Văn học',
    Lively: 'Sinh động',
    Media: 'Truyền thông',
    Plain: 'Giản dị',
    Sharp: 'Sắc bén',
    Solemn: 'Trang trọng',
    Spoken: 'Khẩu ngữ',
    Technical: 'Kỹ thuật',
    Warm: 'Ấm áp',

    'Tone Plaza': 'Quảng trường văn phong',
    'Tone Standard': 'Tiêu chuẩn văn phong',
    'Tone Name': 'Tên văn phong',
    Tone: 'Giọng điệu',
    'Tone Prompt': 'Lời nhắc văn phong',
    'New Tone': 'Tạo văn phong',
    'Edit Tone': 'Sửa văn phong',
    'Delete Tone': 'Xóa văn phong',
    'Delete "{{name}}"? This cannot be undone.':
      'Xóa "{{name}}"? Thao tác này không thể hoàn tác.',
    'No tones yet': 'Chưa có văn phong',
    'Total {{count}} tones': 'Tổng {{count}} văn phong',
    'Search by name, prompt or sample':
      'Tìm theo tên, lời nhắc hoặc ví dụ',
    'Tone created': 'Đã tạo văn phong',
    'Tone updated': 'Đã cập nhật văn phong',
    'Tone deleted': 'Đã xóa văn phong',

    Sample: 'Ví dụ',
    'Sample Input': 'Văn bản gốc',
    'Sample Output': 'Văn bản viết lại',
    'A sentence written plainly': 'Một câu viết đơn giản',
    'The same sentence in this tone': 'Cùng câu đó theo văn phong này',
    'Optional before / after pair showing what this tone does.':
      'Cặp trước / sau tùy chọn cho thấy văn phong này làm gì.',

    'The tone prompt is the instruction downstream models follow.':
      'Lời nhắc văn phong là chỉ dẫn mà các mô hình phía sau tuân theo.',
    'Warm and composed': 'Ấm áp và điềm đạm',
    'What this tone reads like': 'Văn phong này đọc lên như thế nào',
    'The instruction downstream models follow':
      'Chỉ dẫn mà các mô hình phía sau tuân theo',
    'Paste the wording a model should follow. This is the field consumers actually use.':
      'Dán nội dung mà mô hình cần tuân theo. Đây là trường mà bên tiêu thụ thực sự dùng.',
    'Separate scenes with a comma.': 'Phân tách các cảnh bằng dấu phẩy.',
    'Newsletter, product copy, documentation':
      'Bản tin, nội dung sản phẩm, tài liệu',
    'Off-shelf tones stay hidden from the public list.':
      'Văn phong đã gỡ kệ sẽ ẩn khỏi danh sách công khai.',

    'Tone List API': 'API danh sách văn phong',
    'Tone Standard API': 'API tiêu chuẩn văn phong',
    'On-shelf tones are published read-only; no authentication is required.':
      'Văn phong đã lên kệ được công bố ở chế độ chỉ đọc, không cần xác thực.',
    'Vocabulary, field caps and compatibility promises every consumer can rely on.':
      'Từ vựng, giới hạn trường và cam kết tương thích mà mọi bên tiêu thụ có thể dựa vào.',
    'Loaded from the published standard endpoint.':
      'Đã tải từ API tiêu chuẩn công khai.',
    'Built-in mirror: the standard endpoint is unreachable.':
      'Bản sao tích hợp: không kết nối được API tiêu chuẩn.',

    'Import Tones': 'Nhập văn phong',
    'Upload a CSV exported from this page. name and prompt are required.':
      'Tải lên CSV xuất từ trang này. Trường name và prompt là bắt buộc.',
    'Columns: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Export the current list to get a filled-in template.':
      'Cột: name, description, prompt, category, tone, language, scenes, sample_input, sample_output, enabled, sort_order. Hãy xuất danh sách hiện tại để có mẫu đã điền.',

    'Tone name is required': 'Vui lòng nhập tên văn phong',
    'Tone prompt is required': 'Vui lòng nhập lời nhắc văn phong',
    'Tone name is too long (max 191 characters)':
      'Tên văn phong quá dài (tối đa 191 ký tự)',
    'Tone prompt is too long (max 8000 characters)':
      'Lời nhắc văn phong quá dài (tối đa 8000 ký tự)',
    'Sample input is too long (max 4000 characters)':
      'Văn bản gốc quá dài (tối đa 4000 ký tự)',
    'Sample output is too long (max 4000 characters)':
      'Văn bản viết lại quá dài (tối đa 4000 ký tự)',
    'Language tag is too long (max 16 characters)':
      'Thẻ ngôn ngữ quá dài (tối đa 16 ký tự)',
  },
}

async function main() {
  let totalAdded = 0
  let totalRetired = 0

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

    let retired = 0
    for (const key of retiredKeys) {
      if (Object.prototype.hasOwnProperty.call(json.translation, key)) {
        delete json.translation[key]
        retired++
      }
    }

    if (count > 0 || retired > 0) {
      json.translation = Object.fromEntries(
        Object.entries(json.translation).sort(([a], [b]) => a.localeCompare(b))
      )
      await fs.writeFile(filePath, stableStringify(json), 'utf8')
    }

    console.log(
      `${locale}: ${count} translations applied, ${retired} retired`
    )
    totalAdded += count
    totalRetired += retired
  }

  console.log(
    `\nTotal: ${totalAdded} translations applied, ${totalRetired} retired`
  )
}

main().catch((err) => {
  console.error(err)
  process.exitCode = 1
})
