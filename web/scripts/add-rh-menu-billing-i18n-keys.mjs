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
// Merge the RunningHub menu + per-run billing i18n keys into all seven locale
// files, then re-run the i18n sync so ordering, extras and reports stay
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
    'RH App Center': 'RH App Center',
    'Billing API Key': 'Billing API Key',
    'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.':
      'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.',
    'Per-Second Billing': 'Per-Second Billing',
    'Please wait for the upload to finish':
      'Please wait for the upload to finish',
    'Quota Per Second': 'Quota Per Second',
    'Select an API key to bill this run':
      'Select an API key to bill this run',
    'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.':
      'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.',
  },
  zh: {
    'RH App Center': 'RH 应用中心',
    'Billing API Key': '计费 API 密钥',
    'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.':
      '按秒计费的应用按 seconds/duration 参数值收费；按次计费的应用每次运行收取固定额度。',
    'Per-Second Billing': '按秒计费',
    'Please wait for the upload to finish': '请等待上传完成',
    'Quota Per Second': '每秒额度',
    'Select an API key to bill this run': '选择用于本次运行计费的 API 密钥',
    'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.':
      '本次运行将按所选 API 密钥计费。请选择有额度的启用密钥，或一个无限制的密钥。',
  },
  'zh-TW': {
    'RH App Center': 'RH 應用中心',
    'Billing API Key': '計費 API 金鑰',
    'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.':
      '按秒計費的應用依 seconds/duration 參數值收費；按次計費的應用每次執行收取固定額度。',
    'Per-Second Billing': '按秒計費',
    'Please wait for the upload to finish': '請等待上傳完成',
    'Quota Per Second': '每秒額度',
    'Select an API key to bill this run': '選擇用於本次執行計費的 API 金鑰',
    'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.':
      '本次執行將按所選 API 金鑰計費。請選擇有額度的啟用金鑰，或一個無限制的金鑰。',
  },
  fr: {
    'RH App Center': 'Centre d\'applications RH',
    'Billing API Key': 'Clé API de facturation',
    'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.':
      'Les applications à facturation à la seconde sont facturées selon la valeur du paramètre seconds/duration ; les applications au forfait sont facturées à un quota fixe par exécution.',
    'Per-Second Billing': 'Facturation à la seconde',
    'Please wait for the upload to finish':
      'Veuillez attendre la fin du téléversement',
    'Quota Per Second': 'Quota par seconde',
    'Select an API key to bill this run':
      'Sélectionnez une clé API pour facturer cette exécution',
    'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.':
      'Cette exécution est facturée sur la clé API sélectionnée. Choisissez une clé activée avec un quota, ou une clé illimitée.',
  },
  ja: {
    'RH App Center': 'RH アプリセンター',
    'Billing API Key': '課金 API キー',
    'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.':
      '秒単位課金のアプリは seconds/duration パラメータの値に応じて課金され、従量課金（回数）のアプリは実行ごとに固定クォータを課金します。',
    'Per-Second Billing': '秒単位課金',
    'Please wait for the upload to finish': 'アップロードが完了するまでお待ちください',
    'Quota Per Second': '毎秒クォータ',
    'Select an API key to bill this run':
      'この実行の課金に使用する API キーを選択',
    'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.':
      'この実行は選択した API キーに対して課金されます。クォータのある有効なキー、または無制限のキーを選択してください。',
  },
  ru: {
    'RH App Center': 'Центр приложений RH',
    'Billing API Key': 'Ключ API для выставления счетов',
    'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.':
      'Приложения с оплатой за секунду тарифицируются по значению параметра seconds/duration; приложения с оплатой за вызов берут фиксированную квоту за каждый запуск.',
    'Per-Second Billing': 'Оплата за секунду',
    'Please wait for the upload to finish': 'Дождитесь завершения загрузки',
    'Quota Per Second': 'Квота в секунду',
    'Select an API key to bill this run':
      'Выберите ключ API для оплаты этого запуска',
    'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.':
      'Этот запуск оплачивается по выбранному ключу API. Выберите включенный ключ с квотой или безлимитный.',
  },
  vi: {
    'RH App Center': 'Trung tâm ứng dụng RH',
    'Billing API Key': 'Khóa API thanh toán',
    'Per-second apps charge by the seconds/duration parameter value; per-call apps charge a flat quota per run.':
      'Ứng dụng thanh toán theo giây được tính phí theo giá trị tham số seconds/duration; ứng dụng thanh toán theo lượt gọi tính một hạn mức cố định cho mỗi lần chạy.',
    'Per-Second Billing': 'Thanh toán theo giây',
    'Please wait for the upload to finish': 'Vui lòng đợi quá trình tải lên hoàn tất',
    'Quota Per Second': 'Hạn mức mỗi giây',
    'Select an API key to bill this run':
      'Chọn khóa API để thanh toán cho lần chạy này',
    'This run is charged against the selected API key. Choose an enabled key with quota, or an unlimited one.':
      'Lần chạy này được tính phí theo khóa API đã chọn. Hãy chọn một khóa đã bật có hạn mức, hoặc một khóa không giới hạn.',
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
