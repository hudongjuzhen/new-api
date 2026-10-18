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

// One-off migration script: the RunningHub app form (/rh-apps) now carries one
// upstream-id / identifier pair PER SITE instead of a single pair, so its labels
// are composed from the site name ("{{site}} Upstream ID"). Run from web/:
// node scripts/add-rh-site-upstream-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    '{{site}} Upstream ID': '{{site}} Upstream ID',
    '{{site}} Identifier': '{{site}} Identifier',
    'Upstream IDs by site': 'Upstream IDs by site',
    'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).':
      'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).',
    'Which site handles this app. The run submits on that site with the upstream ID configured for it below.':
      'Which site handles this app. The run submits on that site with the upstream ID configured for it below.',
  },
  zh: {
    '{{site}} Upstream ID': '{{site}}上游ID',
    '{{site}} Identifier': '{{site}}标识符',
    'Upstream IDs by site': '各站点的上游ID',
    'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).':
      '每个站点对应各自的上游应用ID。一次运行使用其所属站点的上游ID与标识符（见上方「站点」）。',
    'Which site handles this app. The run submits on that site with the upstream ID configured for it below.':
      '该应用由哪个站点处理。运行时会使用该站点、以及下方为它配置的上游ID提交。',
    'RunningHub Intl': 'RunningHub国际站',
  },
  'zh-TW': {
    '{{site}} Upstream ID': '{{site}}上游ID',
    '{{site}} Identifier': '{{site}}標識符',
    'Upstream IDs by site': '各站點的上游ID',
    'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).':
      '每個站點對應各自的上游應用ID。一次執行使用其所在站點的上游ID與標識符（見上方「站點」）。',
    'Which site handles this app. The run submits on that site with the upstream ID configured for it below.':
      '該應用由哪個站點處理。執行時會使用該站點、以及下方為它配置的上游ID提交。',
    'RunningHub Intl': 'RunningHub國際站',
  },
  fr: {
    '{{site}} Upstream ID': 'ID amont {{site}}',
    '{{site}} Identifier': 'Identifiant {{site}}',
    'Upstream IDs by site': 'ID amont par site',
    'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).':
      "Chaque site répond à son propre ID d'application amont. Une exécution utilise les identifiants du site sur lequel elle s'exécute (voir Site ci-dessus).",
    'Which site handles this app. The run submits on that site with the upstream ID configured for it below.':
      "Site qui traite cette application. L'exécution est envoyée sur ce site avec l'ID amont configuré ci-dessous.",
  },
  ja: {
    '{{site}} Upstream ID': '{{site}} の上流ID',
    '{{site}} Identifier': '{{site}} の識別子',
    'Upstream IDs by site': 'サイト別の上流ID',
    'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).':
      '各サイトはそれぞれ独自の上流アプリIDを持ちます。実行時は実行先サイトの上流IDと識別子が使われます（上部の「サイト」を参照）。',
    'Which site handles this app. The run submits on that site with the upstream ID configured for it below.':
      'このアプリを処理するサイトです。実行はこのサイトに、下で設定した上流IDを使って送信されます。',
  },
  ru: {
    '{{site}} Upstream ID': 'ID вышестоящего сервиса {{site}}',
    '{{site}} Identifier': 'Идентификатор {{site}}',
    'Upstream IDs by site': 'ID вышестоящего сервиса по сайтам',
    'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).':
      'У каждого сайта свой ID приложения в вышестоящем сервисе. Запуск использует идентификаторы того сайта, на котором выполняется (см. «Сайт» выше).',
    'Which site handles this app. The run submits on that site with the upstream ID configured for it below.':
      'Какой сайт обслуживает это приложение. Запуск отправляется на этот сайт с указанным ниже ID вышестоящего сервиса.',
  },
  vi: {
    '{{site}} Upstream ID': 'ID thượng nguồn {{site}}',
    '{{site}} Identifier': 'Mã định danh {{site}}',
    'Upstream IDs by site': 'ID thượng nguồn theo trang web',
    'Each site answers to its own upstream app ID. A run uses the identifiers of the site it runs on (see Site above).':
      'Mỗi trang web có ID ứng dụng thượng nguồn riêng. Một lần chạy dùng ID và mã định danh của trang web mà nó chạy trên đó (xem «Trang web» ở trên).',
    'Which site handles this app. The run submits on that site with the upstream ID configured for it below.':
      'Trang web xử lý ứng dụng này. Lần chạy được gửi tới trang web đó với ID thượng nguồn cấu hình bên dưới.',
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
