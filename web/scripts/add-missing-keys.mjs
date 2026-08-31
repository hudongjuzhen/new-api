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

// One-off migration script: adds zsy-lab and zsy-runninghub portal i18n keys
// to all seven locales. Run from web/: node scripts/add-missing-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

const newKeys = {
  en: {
    '(empty prompt)': '(empty prompt)',
    'Anthropic compatible': 'Anthropic compatible',
    'API Format': 'API Format',
    'API reference': 'API reference',
    'Both gateway-compatible call styles are shown below. Current model:':
      'Both gateway-compatible call styles are shown below. Current model:',
    'cached locally for 24h': 'cached locally for 24h',
    'Chat Completions': 'Chat Completions',
    'compatible endpoint': 'compatible endpoint',
    'Conversation content': 'Conversation content',
    'Conversation messages (system / user / assistant roles)':
      'Conversation messages (system / user / assistant roles)',
    'Encourages new topics, between -2 and 2':
      'Encourages new topics, between -2 and 2',
    'Endpoints below accept an API key. Current model:':
      'Endpoints below accept an API key. Current model:',
    Examples: 'Examples',
    'Explain quantum entanglement in plain language':
      'Explain quantum entanglement in plain language',
    Form: 'Form',
    'Frequency penalty': 'Frequency penalty',
    'Generating…': 'Generating…',
    'Invalid JSON request body': 'Invalid JSON request body',
    'Max output tokens': 'Max output tokens',
    'Maximum number of tokens to generate':
      'Maximum number of tokens to generate',
    Messages: 'Messages',
    'Model identifier to call': 'Model identifier to call',
    'More parameters': 'More parameters',
    'Nucleus sampling, between 0 and 1': 'Nucleus sampling, between 0 and 1',
    'No model selected. Pick one from the model marketplace first.':
      'No model selected. Pick one from the model marketplace first.',
    'No records yet. They will be logged automatically after your first request.':
      'No records yet. They will be logged automatically after your first request.',
    'OpenAI Chat': 'OpenAI Chat',
    'OpenAI compatible': 'OpenAI compatible',
    'Option 1: OpenAI Chat compatible': 'Option 1: OpenAI Chat compatible',
    'Option 2: Anthropic compatible': 'Option 2: Anthropic compatible',
    'Optional: set the model role, background or behavior constraints…':
      'Optional: set the model role, background or behavior constraints…',
    'Penalizes repeated tokens, between -2 and 2':
      'Penalizes repeated tokens, between -2 and 2',
    'Please enter the conversation content':
      'Please enter the conversation content',
    'Presence penalty': 'Presence penalty',
    'Raw stream events will appear here.':
      'Raw stream events will appear here.',
    'Recommended for most SDKs': 'Recommended for most SDKs',
    Run: 'Run',
    'Run a request to see the model response here.':
      'Run a request to see the model response here.',
    'Sampling temperature, between 0 and 2':
      'Sampling temperature, between 0 and 2',
    'Session channel': 'Session channel',
    'Show less': 'Show less',
    'Sign in to try': 'Sign in to try',
    'Stream the response as server-sent events':
      'Stream the response as server-sent events',
    'Stream tokens as they are generated; disable to wait for the full response.':
      'Stream tokens as they are generated; disable to wait for the full response.',
    'Streaming output': 'Streaming output',
    'Switch model': 'Switch model',
    'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.':
      'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.',
    'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.':
      'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.',
    'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).':
      'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).',
    'Upload images': 'Upload images',
    'Usage history': 'Usage history',
    'Uses x-api-key and anthropic-version headers':
      'Uses x-api-key and anthropic-version headers',
    'View all': 'View all',
    'About this app': 'About this app',
    'Browse RunningHub applications, fill in the parameters and generate.':
      'Browse RunningHub applications, fill in the parameters and generate.',
    'Generation Records': 'Generation Records',
    'In progress': 'In progress',
    'No apps found': 'No apps found',
    'No description': 'No description',
    'No generation records yet': 'No generation records yet',
    'Paste a public URL': 'Paste a public URL',
    'Please fix the highlighted fields': 'Please fix the highlighted fields',
    'RunningHub App Center': 'RunningHub App Center',
    'Search apps...': 'Search apps...',
    'Select an application from the left to start.':
      'Select an application from the left to start.',
    'Select...': 'Select...',
    'This application has no configurable parameters yet.':
      'This application has no configurable parameters yet.',
    'This field is required': 'This field is required',
  },
  zh: {
    '(empty prompt)': '（空提示词）',
    'Anthropic compatible': 'Anthropic 兼容',
    'API Format': 'API 格式',
    'API reference': 'API 参考',
    'Both gateway-compatible call styles are shown below. Current model:':
      '以下展示两种网关兼容的调用方式。当前模型：',
    'cached locally for 24h': '本地缓存 24 小时',
    'Chat Completions': 'Chat Completions',
    'compatible endpoint': '兼容端点',
    'Conversation content': '对话内容',
    'Conversation messages (system / user / assistant roles)':
      '对话消息（system / user / assistant 角色）',
    'Encourages new topics, between -2 and 2': '鼓励引入新话题，取值范围 -2 到 2',
    'Endpoints below accept an API key. Current model:':
      '以下端点接受 API 密钥。当前模型：',
    Examples: '示例',
    'Explain quantum entanglement in plain language': '用通俗的语言解释量子纠缠',
    Form: '表单',
    'Frequency penalty': '频率惩罚',
    'Generating…': '生成中…',
    'Invalid JSON request body': 'JSON 请求体无效',
    'Max output tokens': '最大输出 Token 数',
    'Maximum number of tokens to generate': '生成的最大 Token 数',
    Messages: 'Messages',
    'Model identifier to call': '要调用的模型标识',
    'More parameters': '更多参数',
    'Nucleus sampling, between 0 and 1': '核采样，取值范围 0 到 1',
    'No model selected. Pick one from the model marketplace first.':
      '未选择模型。请先从模型广场选择一个模型。',
    'No records yet. They will be logged automatically after your first request.':
      '暂无记录。首次请求后将自动记录。',
    'OpenAI Chat': 'OpenAI Chat',
    'OpenAI compatible': 'OpenAI 兼容',
    'Option 1: OpenAI Chat compatible': '方式一：OpenAI Chat 兼容',
    'Option 2: Anthropic compatible': '方式二：Anthropic 兼容',
    'Optional: set the model role, background or behavior constraints…':
      '可选：设定模型的角色、背景或行为约束…',
    'Penalizes repeated tokens, between -2 and 2':
      '惩罚重复的 Token，取值范围 -2 到 2',
    'Please enter the conversation content': '请输入对话内容',
    'Presence penalty': '存在惩罚',
    'Raw stream events will appear here.': '原始流事件将显示在这里。',
    'Recommended for most SDKs': '适用于大多数 SDK',
    Run: '运行',
    'Run a request to see the model response here.':
      '运行一次请求后，模型回复将显示在这里。',
    'Sampling temperature, between 0 and 2': '采样温度，取值范围 0 到 2',
    'Session channel': '会话通道',
    'Show less': '收起',
    'Sign in to try': '登录后试用',
    'Stream the response as server-sent events':
      '以 SSE（服务器发送事件）形式流式返回响应',
    'Stream tokens as they are generated; disable to wait for the full response.':
      'Token 生成时即时流式输出；关闭后将等待完整响应。',
    'Streaming output': '流式输出',
    'Switch model': '切换模型',
    'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.':
      '「试玩」标签页使用你已登录的会话请求 /pg/chat/completions，无需 API 密钥。用量将像普通 API 调用一样计入你的账户。',
    'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.':
      '「试玩」标签页使用你已登录的会话（无需 API 密钥）。上方示例用于生产环境调用，需使用在「令牌」页面创建的 API 密钥。',
    'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).':
      '会话通道仅支持 OpenAI Chat 格式；如需 Anthropic 格式，请使用 API 密钥调用对应端点（见「API」标签页）。',
    'Upload images': '上传图片',
    'Usage history': '使用记录',
    'Uses x-api-key and anthropic-version headers':
      '使用 x-api-key 与 anthropic-version 请求头',
    'View all': '查看全部',
    'About this app': '应用介绍',
    'Browse RunningHub applications, fill in the parameters and generate.':
      '浏览 RunningHub 应用，填写参数并生成。',
    'Generation Records': '生成记录',
    'In progress': '进行中',
    'No apps found': '未找到应用',
    'No description': '暂无描述',
    'No generation records yet': '暂无生成记录',
    'Paste a public URL': '粘贴公开链接',
    'Please fix the highlighted fields': '请修正标红的字段',
    'RunningHub App Center': 'RunningHub 应用中心',
    'Search apps...': '搜索应用...',
    'Select an application from the left to start.':
      '从左侧选择一个应用开始使用。',
    'Select...': '请选择...',
    'This application has no configurable parameters yet.':
      '该应用暂无可配置的参数。',
    'This field is required': '该字段为必填项',
  },
  'zh-TW': {
    '(empty prompt)': '（空提示詞）',
    'Anthropic compatible': 'Anthropic 相容',
    'API Format': 'API 格式',
    'API reference': 'API 參考',
    'Both gateway-compatible call styles are shown below. Current model:':
      '以下展示兩種閘道器相容的呼叫方式。目前模型：',
    'cached locally for 24h': '本機快取 24 小時',
    'Chat Completions': 'Chat Completions',
    'compatible endpoint': '相容端點',
    'Conversation content': '對話內容',
    'Conversation messages (system / user / assistant roles)':
      '對話訊息（system / user / assistant 角色）',
    'Encourages new topics, between -2 and 2': '鼓勵引入新話題，取值範圍 -2 到 2',
    'Endpoints below accept an API key. Current model:':
      '以下端點接受 API 金鑰。目前模型：',
    Examples: '範例',
    'Explain quantum entanglement in plain language': '用通俗的語言解釋量子糾纏',
    Form: '表單',
    'Frequency penalty': '頻率懲罰',
    'Generating…': '產生中…',
    'Invalid JSON request body': 'JSON 請求內容無效',
    'Max output tokens': '最大輸出 Token 數',
    'Maximum number of tokens to generate': '產生的最大 Token 數',
    Messages: 'Messages',
    'Model identifier to call': '要呼叫的模型標識',
    'More parameters': '更多參數',
    'Nucleus sampling, between 0 and 1': '核取樣，取值範圍 0 到 1',
    'No model selected. Pick one from the model marketplace first.':
      '未選擇模型。請先從模型廣場選擇一個模型。',
    'No records yet. They will be logged automatically after your first request.':
      '暫無記錄。首次請求後將自動記錄。',
    'OpenAI Chat': 'OpenAI Chat',
    'OpenAI compatible': 'OpenAI 相容',
    'Option 1: OpenAI Chat compatible': '方式一：OpenAI Chat 相容',
    'Option 2: Anthropic compatible': '方式二：Anthropic 相容',
    'Optional: set the model role, background or behavior constraints…':
      '可選：設定模型的角色、背景或行為約束…',
    'Penalizes repeated tokens, between -2 and 2':
      '懲罰重複的 Token，取值範圍 -2 到 2',
    'Please enter the conversation content': '請輸入對話內容',
    'Presence penalty': '存在懲罰',
    'Raw stream events will appear here.': '原始串流事件將顯示在這裡。',
    'Recommended for most SDKs': '適用於大多數 SDK',
    Run: '執行',
    'Run a request to see the model response here.':
      '執行一次請求後，模型回覆將顯示在這裡。',
    'Sampling temperature, between 0 and 2': '取樣溫度，取值範圍 0 到 2',
    'Session channel': '會話通道',
    'Show less': '收合',
    'Sign in to try': '登入後試用',
    'Stream the response as server-sent events':
      '以 SSE（伺服器傳送事件）形式串流回應',
    'Stream tokens as they are generated; disable to wait for the full response.':
      'Token 產生時即時串流輸出；關閉後將等待完整回應。',
    'Streaming output': '串流輸出',
    'Switch model': '切換模型',
    'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.':
      '「試玩」分頁使用你已登入的會話請求 /pg/chat/completions，無需 API 金鑰。用量將像一般 API 呼叫一樣計入你的帳戶。',
    'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.':
      '「試玩」分頁使用你已登入的會話（無需 API 金鑰）。上方範例用於生產環境呼叫，需使用在「權杖」頁面建立的 API 金鑰。',
    'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).':
      '會話通道僅支援 OpenAI Chat 格式；如需 Anthropic 格式，請使用 API 金鑰呼叫對應端點（見「API」分頁）。',
    'Upload images': '上傳圖片',
    'Usage history': '使用記錄',
    'Uses x-api-key and anthropic-version headers':
      '使用 x-api-key 與 anthropic-version 標頭',
    'View all': '查看全部',
    'About this app': '應用介紹',
    'Browse RunningHub applications, fill in the parameters and generate.':
      '瀏覽 RunningHub 應用，填寫參數並產生。',
    'Generation Records': '產生記錄',
    'In progress': '進行中',
    'No apps found': '未找到應用',
    'No description': '暫無描述',
    'No generation records yet': '暫無產生記錄',
    'Paste a public URL': '貼上公開連結',
    'Please fix the highlighted fields': '請修正標示的欄位',
    'RunningHub App Center': 'RunningHub 應用中心',
    'Search apps...': '搜尋應用...',
    'Select an application from the left to start.':
      '從左側選擇一個應用開始使用。',
    'Select...': '請選擇...',
    'This application has no configurable parameters yet.':
      '該應用暫無可設定的參數。',
    'This field is required': '該欄位為必填項',
  },
  fr: {
    '(empty prompt)': '(invite vide)',
    'Anthropic compatible': 'Compatible Anthropic',
    'API Format': "Format d'API",
    'API reference': 'Référence API',
    'Both gateway-compatible call styles are shown below. Current model:':
      "Les deux styles d'appel compatibles avec la passerelle sont présentés ci-dessous. Modèle actuel :",
    'cached locally for 24h': 'mis en cache localement pendant 24 h',
    'Chat Completions': 'Chat Completions',
    'compatible endpoint': 'endpoint compatible',
    'Conversation content': 'Contenu de la conversation',
    'Conversation messages (system / user / assistant roles)':
      'Messages de la conversation (rôles system / user / assistant)',
    'Encourages new topics, between -2 and 2':
      'Encourage les nouveaux sujets, entre -2 et 2',
    'Endpoints below accept an API key. Current model:':
      'Les points de terminaison ci-dessous acceptent une clé API. Modèle actuel :',
    Examples: 'Exemples',
    'Explain quantum entanglement in plain language':
      "Expliquez l'intrication quantique en langage simple",
    Form: 'Formulaire',
    'Frequency penalty': 'Pénalité de fréquence',
    'Generating…': 'Génération…',
    'Invalid JSON request body': 'Corps de requête JSON invalide',
    'Max output tokens': 'Tokens de sortie max',
    'Maximum number of tokens to generate':
      'Nombre maximum de tokens à générer',
    Messages: 'Messages',
    'Model identifier to call': 'Identifiant du modèle à appeler',
    'More parameters': 'Plus de paramètres',
    'Nucleus sampling, between 0 and 1':
      'Échantillonnage nucleus, entre 0 et 1',
    'No model selected. Pick one from the model marketplace first.':
      "Aucun modèle sélectionné. Choisissez d'abord un modèle dans la place de marché.",
    'No records yet. They will be logged automatically after your first request.':
      'Aucun enregistrement pour le moment. Ils seront consignés automatiquement après votre première requête.',
    'OpenAI Chat': 'OpenAI Chat',
    'OpenAI compatible': 'Compatible OpenAI',
    'Option 1: OpenAI Chat compatible': 'Option 1 : compatible OpenAI Chat',
    'Option 2: Anthropic compatible': 'Option 2 : compatible Anthropic',
    'Optional: set the model role, background or behavior constraints…':
      'Facultatif : définissez le rôle, le contexte ou les contraintes de comportement du modèle…',
    'Penalizes repeated tokens, between -2 and 2':
      'Pénalise les tokens répétés, entre -2 et 2',
    'Please enter the conversation content':
      'Veuillez saisir le contenu de la conversation',
    'Presence penalty': 'Pénalité de présence',
    'Raw stream events will appear here.':
      'Les événements de flux bruts apparaîtront ici.',
    'Recommended for most SDKs': 'Recommandé pour la plupart des SDK',
    Run: 'Exécuter',
    'Run a request to see the model response here.':
      'Exécutez une requête pour voir la réponse du modèle ici.',
    'Sampling temperature, between 0 and 2':
      "Température d'échantillonnage, entre 0 et 2",
    'Session channel': 'Canal de session',
    'Show less': 'Réduire',
    'Sign in to try': 'Connectez-vous pour essayer',
    'Stream the response as server-sent events':
      'Diffuser la réponse en événements envoyés par le serveur',
    'Stream tokens as they are generated; disable to wait for the full response.':
      'Diffuse les tokens au fur et à mesure de leur génération ; désactivez pour attendre la réponse complète.',
    'Streaming output': 'Sortie en flux',
    'Switch model': 'Changer de modèle',
    'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.':
      "L'onglet Playground envoie ses requêtes à /pg/chat/completions avec votre session connectée — aucune clé API requise. L'utilisation est facturée sur votre compte exactement comme un appel API normal.",
    'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.':
      "L'onglet Playground utilise votre session connectée (aucune clé API requise). Les exemples ci-dessus concernent les appels en production avec une clé API créée sur la page Jetons.",
    'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).':
      "Le canal de session prend uniquement en charge le format OpenAI Chat ; utilisez le point de terminaison Anthropic avec une clé API (voir l'onglet API).",
    'Upload images': 'Téléverser des images',
    'Usage history': "Historique d'utilisation",
    'Uses x-api-key and anthropic-version headers':
      'Utilise les en-têtes x-api-key et anthropic-version',
    'View all': 'Tout afficher',
    'About this app': 'À propos de cette application',
    'Browse RunningHub applications, fill in the parameters and generate.':
      'Parcourez les applications RunningHub, remplissez les paramètres et générez.',
    'Generation Records': 'Enregistrements de génération',
    'In progress': 'En cours',
    'No apps found': 'Aucune application trouvée',
    'No description': 'Aucune description',
    'No generation records yet': 'Aucun enregistrement de génération',
    'Paste a public URL': 'Collez une URL publique',
    'Please fix the highlighted fields':
      'Veuillez corriger les champs surlignés',
    'RunningHub App Center': "Centre d'applications RunningHub",
    'Search apps...': 'Rechercher des applications...',
    'Select an application from the left to start.':
      'Sélectionnez une application à gauche pour commencer.',
    'Select...': 'Sélectionner...',
    'This application has no configurable parameters yet.':
      "Cette application n'a pas encore de paramètres configurables.",
    'This field is required': 'Ce champ est requis',
  },
  ja: {
    '(empty prompt)': '（空のプロンプト）',
    'Anthropic compatible': 'Anthropic 互換',
    'API Format': 'API 形式',
    'API reference': 'API リファレンス',
    'Both gateway-compatible call styles are shown below. Current model:':
      '以下にゲートウェイ互換の 2 つの呼び出し方式を示します。現在のモデル：',
    'cached locally for 24h': '24 時間ローカルにキャッシュ',
    'Chat Completions': 'Chat Completions',
    'compatible endpoint': '互換エンドポイント',
    'Conversation content': '会話内容',
    'Conversation messages (system / user / assistant roles)':
      '会話メッセージ（system / user / assistant ロール）',
    'Encourages new topics, between -2 and 2':
      '新しい話題を奨励します（-2 ～ 2）',
    'Endpoints below accept an API key. Current model:':
      '以下のエンドポイントは API キーを受け付けます。現在のモデル：',
    Examples: '使用例',
    'Explain quantum entanglement in plain language':
      '量子もつれを平易な言葉で説明してください',
    Form: 'フォーム',
    'Frequency penalty': '頻度ペナルティ',
    'Generating…': '生成中…',
    'Invalid JSON request body': 'JSON リクエストボディが無効です',
    'Max output tokens': '最大出力トークン数',
    'Maximum number of tokens to generate': '生成する最大トークン数',
    Messages: 'Messages',
    'Model identifier to call': '呼び出すモデルの識別子',
    'More parameters': '詳細パラメータ',
    'Nucleus sampling, between 0 and 1': '核サンプリング（0 ～ 1）',
    'No model selected. Pick one from the model marketplace first.':
      'モデルが選択されていません。先にモデルマーケットから選択してください。',
    'No records yet. They will be logged automatically after your first request.':
      '記録はまだありません。最初のリクエスト後に自動的に記録されます。',
    'OpenAI Chat': 'OpenAI Chat',
    'OpenAI compatible': 'OpenAI 互換',
    'Option 1: OpenAI Chat compatible': '方法 1: OpenAI Chat 互換',
    'Option 2: Anthropic compatible': '方法 2: Anthropic 互換',
    'Optional: set the model role, background or behavior constraints…':
      '任意：モデルの役割、背景、動作の制約を設定…',
    'Penalizes repeated tokens, between -2 and 2':
      '繰り返しトークンにペナルティを課します（-2 ～ 2）',
    'Please enter the conversation content': '会話内容を入力してください',
    'Presence penalty': '存在ペナルティ',
    'Raw stream events will appear here.':
      '生のストリームイベントがここに表示されます。',
    'Recommended for most SDKs': 'ほとんどの SDK に推奨',
    Run: '実行',
    'Run a request to see the model response here.':
      'リクエストを実行すると、モデルの応答がここに表示されます。',
    'Sampling temperature, between 0 and 2': 'サンプリング温度（0 ～ 2）',
    'Session channel': 'セッションチャネル',
    'Show less': '折りたたむ',
    'Sign in to try': 'ログインして試用',
    'Stream the response as server-sent events':
      'サーバー送信イベント（SSE）としてレスポンスをストリーミング',
    'Stream tokens as they are generated; disable to wait for the full response.':
      'トークンを生成時に逐次ストリーミングします。オフにすると完全な応答を待ちます。',
    'Streaming output': 'ストリーミング出力',
    'Switch model': 'モデルを切り替え',
    'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.':
      '「Playground」タブはログイン中のセッションで /pg/chat/completions にリクエストを送信します。API キーは不要です。使用量は通常の API 呼び出しと同様にアカウントに請求されます。',
    'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.':
      '「Playground」タブはログイン中のセッションを使用します（API キー不要）。上記のサンプルは「トークン」ページで作成した API キーを使った本番呼び出し用です。',
    'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).':
      'セッションチャネルは OpenAI Chat 形式のみ対応しています。Anthropic 形式は API キーで対応エンドポイントをご利用ください（「API」タブを参照）。',
    'Upload images': '画像をアップロード',
    'Usage history': '利用履歴',
    'Uses x-api-key and anthropic-version headers':
      'x-api-key と anthropic-version ヘッダーを使用',
    'View all': 'すべて表示',
    'About this app': 'このアプリについて',
    'Browse RunningHub applications, fill in the parameters and generate.':
      'RunningHub アプリを閲覧し、パラメータを入力して生成します。',
    'Generation Records': '生成履歴',
    'In progress': '処理中',
    'No apps found': 'アプリが見つかりません',
    'No description': '説明なし',
    'No generation records yet': '生成履歴はまだありません',
    'Paste a public URL': '公開 URL を貼り付け',
    'Please fix the highlighted fields':
      '強調表示されたフィールドを修正してください',
    'RunningHub App Center': 'RunningHub アプリセンター',
    'Search apps...': 'アプリを検索...',
    'Select an application from the left to start.':
      '左側からアプリを選択して開始します。',
    'Select...': '選択...',
    'This application has no configurable parameters yet.':
      'このアプリには設定可能なパラメータがまだありません。',
    'This field is required': 'このフィールドは必須です',
  },
  ru: {
    '(empty prompt)': '(пустой запрос)',
    'Anthropic compatible': 'Совместимо с Anthropic',
    'API Format': 'Формат API',
    'API reference': 'Справочник API',
    'Both gateway-compatible call styles are shown below. Current model:':
      'Ниже показаны оба совместимых с шлюзом стиля вызова. Текущая модель:',
    'cached locally for 24h': 'кэшируется локально 24 ч',
    'Chat Completions': 'Chat Completions',
    'compatible endpoint': 'совместимая конечная точка',
    'Conversation content': 'Содержание беседы',
    'Conversation messages (system / user / assistant roles)':
      'Сообщения беседы (роли system / user / assistant)',
    'Encourages new topics, between -2 and 2':
      'Поощряет новые темы, от -2 до 2',
    'Endpoints below accept an API key. Current model:':
      'Указанные ниже конечные точки принимают API-ключ. Текущая модель:',
    Examples: 'Примеры',
    'Explain quantum entanglement in plain language':
      'Объясните квантовую запутанность простым языком',
    Form: 'Форма',
    'Frequency penalty': 'Штраф за частоту',
    'Generating…': 'Генерация…',
    'Invalid JSON request body': 'Недопустимое тело запроса JSON',
    'Max output tokens': 'Макс. токенов вывода',
    'Maximum number of tokens to generate':
      'Максимальное количество генерируемых токенов',
    Messages: 'Messages',
    'Model identifier to call': 'Идентификатор вызываемой модели',
    'More parameters': 'Дополнительные параметры',
    'Nucleus sampling, between 0 and 1':
      'Nucleus-сэмплирование, от 0 до 1',
    'No model selected. Pick one from the model marketplace first.':
      'Модель не выбрана. Сначала выберите модель в маркетплейсе.',
    'No records yet. They will be logged automatically after your first request.':
      'Записей пока нет. Они будут сохраняться автоматически после первого запроса.',
    'OpenAI Chat': 'OpenAI Chat',
    'OpenAI compatible': 'Совместимо с OpenAI',
    'Option 1: OpenAI Chat compatible':
      'Вариант 1: совместимый с OpenAI Chat',
    'Option 2: Anthropic compatible': 'Вариант 2: совместимый с Anthropic',
    'Optional: set the model role, background or behavior constraints…':
      'Необязательно: задайте роль модели, контекст или ограничения поведения…',
    'Penalizes repeated tokens, between -2 and 2':
      'Штрафует за повторяющиеся токены, от -2 до 2',
    'Please enter the conversation content': 'Введите содержание беседы',
    'Presence penalty': 'Штраф за присутствие',
    'Raw stream events will appear here.':
      'Необработанные события потока появятся здесь.',
    'Recommended for most SDKs': 'Рекомендуется для большинства SDK',
    Run: 'Запустить',
    'Run a request to see the model response here.':
      'Выполните запрос, чтобы увидеть ответ модели здесь.',
    'Sampling temperature, between 0 and 2':
      'Температура сэмплирования, от 0 до 2',
    'Session channel': 'Сеансовый канал',
    'Show less': 'Свернуть',
    'Sign in to try': 'Войдите, чтобы попробовать',
    'Stream the response as server-sent events':
      'Потоковая передача ответа в виде серверных событий (SSE)',
    'Stream tokens as they are generated; disable to wait for the full response.':
      'Токены передаются потоком по мере генерации; отключите, чтобы дождаться полного ответа.',
    'Streaming output': 'Потоковый вывод',
    'Switch model': 'Сменить модель',
    'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.':
      'Вкладка «Песочница» отправляет запросы на /pg/chat/completions с вашим сеансом входа — ключ API не требуется. Использование тарифицируется на ваш аккаунт так же, как обычный вызов API.',
    'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.':
      'Вкладка «Песочница» использует ваш сеанс входа (ключ API не нужен). Примеры выше предназначены для производственных вызовов с ключом API, созданным на странице «Токены».',
    'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).':
      'Сеансовый канал поддерживает только формат OpenAI Chat; используйте конечную точку Anthropic с ключом API (см. вкладку «API»).',
    'Upload images': 'Загрузить изображения',
    'Usage history': 'История использования',
    'Uses x-api-key and anthropic-version headers':
      'Использует заголовки x-api-key и anthropic-version',
    'View all': 'Показать все',
    'About this app': 'Об этом приложении',
    'Browse RunningHub applications, fill in the parameters and generate.':
      'Просматривайте приложения RunningHub, заполняйте параметры и создавайте.',
    'Generation Records': 'Записи генерации',
    'In progress': 'Выполняется',
    'No apps found': 'Приложения не найдены',
    'No description': 'Нет описания',
    'No generation records yet': 'Записей генерации пока нет',
    'Paste a public URL': 'Вставьте общедоступную ссылку',
    'Please fix the highlighted fields': 'Исправьте выделенные поля',
    'RunningHub App Center': 'Центр приложений RunningHub',
    'Search apps...': 'Поиск приложений...',
    'Select an application from the left to start.':
      'Выберите приложение слева, чтобы начать.',
    'Select...': 'Выберите...',
    'This application has no configurable parameters yet.':
      'У этого приложения пока нет настраиваемых параметров.',
    'This field is required': 'Это поле обязательно',
  },
  vi: {
    '(empty prompt)': '(prompt trống)',
    'Anthropic compatible': 'Tương thích Anthropic',
    'API Format': 'Định dạng API',
    'API reference': 'Tài liệu API',
    'Both gateway-compatible call styles are shown below. Current model:':
      'Cả hai kiểu gọi tương thích với gateway đều được hiển thị bên dưới. Mô hình hiện tại:',
    'cached locally for 24h': 'lưu cục bộ trong 24 giờ',
    'Chat Completions': 'Chat Completions',
    'compatible endpoint': 'endpoint tương thích',
    'Conversation content': 'Nội dung hội thoại',
    'Conversation messages (system / user / assistant roles)':
      'Thông điệp hội thoại (vai trò system / user / assistant)',
    'Encourages new topics, between -2 and 2':
      'Khuyến khích chủ đề mới, từ -2 đến 2',
    'Endpoints below accept an API key. Current model:':
      'Các endpoint bên dưới chấp nhận API key. Mô hình hiện tại:',
    Examples: 'Ví dụ',
    'Explain quantum entanglement in plain language':
      'Giải thích sự vướng víu lượng tử bằng ngôn ngữ đơn giản',
    Form: 'Biểu mẫu',
    'Frequency penalty': 'Hình phạt tần suất',
    'Generating…': 'Đang tạo…',
    'Invalid JSON request body': 'Nội dung yêu cầu JSON không hợp lệ',
    'Max output tokens': 'Số token đầu ra tối đa',
    'Maximum number of tokens to generate': 'Số token tối đa cần tạo',
    Messages: 'Messages',
    'Model identifier to call': 'Định danh mô hình cần gọi',
    'More parameters': 'Tham số khác',
    'Nucleus sampling, between 0 and 1': 'Lấy mẫu nucleus, từ 0 đến 1',
    'No model selected. Pick one from the model marketplace first.':
      'Chưa chọn mô hình. Hãy chọn một mô hình từ chợ mô hình trước.',
    'No records yet. They will be logged automatically after your first request.':
      'Chưa có bản ghi nào. Chúng sẽ được ghi lại tự động sau yêu cầu đầu tiên của bạn.',
    'OpenAI Chat': 'OpenAI Chat',
    'OpenAI compatible': 'Tương thích OpenAI',
    'Option 1: OpenAI Chat compatible': 'Cách 1: Tương thích OpenAI Chat',
    'Option 2: Anthropic compatible': 'Cách 2: Tương thích Anthropic',
    'Optional: set the model role, background or behavior constraints…':
      'Tùy chọn: đặt vai trò, bối cảnh hoặc ràng buộc hành vi của mô hình…',
    'Penalizes repeated tokens, between -2 and 2':
      'Phạt token lặp lại, từ -2 đến 2',
    'Please enter the conversation content':
      'Vui lòng nhập nội dung hội thoại',
    'Presence penalty': 'Hình phạt hiện diện',
    'Raw stream events will appear here.':
      'Các sự kiện stream thô sẽ hiển thị tại đây.',
    'Recommended for most SDKs': 'Khuyến nghị cho hầu hết SDK',
    Run: 'Chạy',
    'Run a request to see the model response here.':
      'Chạy một yêu cầu để xem phản hồi của mô hình tại đây.',
    'Sampling temperature, between 0 and 2': 'Nhiệt độ lấy mẫu, từ 0 đến 2',
    'Session channel': 'Kênh phiên',
    'Show less': 'Thu gọn',
    'Sign in to try': 'Đăng nhập để dùng thử',
    'Stream the response as server-sent events':
      'Trả về phản hồi dạng luồng sự kiện máy chủ (SSE)',
    'Stream tokens as they are generated; disable to wait for the full response.':
      'Phát trực tiếp token khi được tạo; tắt để chờ phản hồi đầy đủ.',
    'Streaming output': 'Đầu ra phát trực tuyến',
    'Switch model': 'Chuyển mô hình',
    'The Playground tab posts to /pg/chat/completions with your signed-in session — no API key required. Usage is billed to your account exactly like a regular API call.':
      'Tab Playground gửi yêu cầu đến /pg/chat/completions bằng phiên đăng nhập của bạn — không cần API key. Mức sử dụng được tính vào tài khoản của bạn giống như một lệnh gọi API thông thường.',
    'The Playground tab uses your signed-in session (no API key needed). The samples above are for production calls with an API key created on the Tokens page.':
      'Tab Playground sử dụng phiên đăng nhập của bạn (không cần API key). Các mẫu trên dùng cho lệnh gọi sản xuất với API key được tạo tại trang Tokens.',
    'The session channel only supports OpenAI Chat format; use the Anthropic endpoint with an API key (see the API tab).':
      'Kênh phiên chỉ hỗ trợ định dạng OpenAI Chat; hãy dùng endpoint Anthropic với API key (xem tab API).',
    'Upload images': 'Tải ảnh lên',
    'Usage history': 'Lịch sử sử dụng',
    'Uses x-api-key and anthropic-version headers':
      'Dùng tiêu đề x-api-key và anthropic-version',
    'View all': 'Xem tất cả',
    'About this app': 'Giới thiệu ứng dụng này',
    'Browse RunningHub applications, fill in the parameters and generate.':
      'Duyệt các ứng dụng RunningHub, điền tham số và tạo nội dung.',
    'Generation Records': 'Bản ghi tạo nội dung',
    'In progress': 'Đang xử lý',
    'No apps found': 'Không tìm thấy ứng dụng',
    'No description': 'Chưa có mô tả',
    'No generation records yet': 'Chưa có bản ghi tạo nội dung',
    'Paste a public URL': 'Dán URL công khai',
    'Please fix the highlighted fields':
      'Vui lòng sửa các trường được đánh dấu',
    'RunningHub App Center': 'Trung tâm ứng dụng RunningHub',
    'Search apps...': 'Tìm kiếm ứng dụng...',
    'Select an application from the left to start.':
      'Chọn một ứng dụng từ bên trái để bắt đầu.',
    'Select...': 'Chọn...',
    'This application has no configurable parameters yet.':
      'Ứng dụng này chưa có tham số nào có thể cấu hình.',
    'This field is required': 'Trường này là bắt buộc',
  },
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
  await fs.writeFile(file, `${JSON.stringify(json, null, 2)}\n`, 'utf8')
  console.log(`${locale}: +${added} keys`)
}
console.log('Done. Run `npm run i18n:sync` next.')
