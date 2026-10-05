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

// One-off migration script: adds the Aliyun OSS object storage settings keys to
// all seven locales. Run from web/: node scripts/add-oss-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Access OSS over HTTPS': 'Access OSS over HTTPS',
    'AccessKey ID': 'AccessKey ID',
    'AccessKey Secret': 'AccessKey Secret',
    'Alibaba Cloud AccessKey ID used to sign uploads':
      'Alibaba Cloud AccessKey ID used to sign uploads',
    'Aliyun OSS': 'Aliyun OSS',
    'Aliyun OSS Object Storage': 'Aliyun OSS Object Storage',
    Bucket: 'Bucket',
    'Custom Domain': 'Custom Domain',
    'Directory prefix inside the bucket. Leave blank to write to the bucket root.':
      'Directory prefix inside the bucket. Leave blank to write to the bucket root.',
    'Enable Aliyun OSS': 'Enable Aliyun OSS',
    'Enter AccessKey ID': 'Enter AccessKey ID',
    'Enter new secret to update': 'Enter new secret to update',
    'Name of the OSS bucket that stores uploaded files':
      'Name of the OSS bucket that stores uploaded files',
    'OSS Endpoint': 'OSS Endpoint',
    'Object Key Prefix': 'Object Key Prefix',
    'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.':
      'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.',
    'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.':
      'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.',
    'Required when Aliyun OSS is enabled':
      'Required when Aliyun OSS is enabled',
    'Save OSS settings': 'Save OSS settings',
    'The configured bucket accepts uploads and deletes.':
      'The configured bucket accepts uploads and deletes.',
    'Uploads and removes a probe object using the saved configuration, so save your changes first.':
      'Uploads and removes a probe object using the saved configuration, so save your changes first.',
    'Use HTTPS': 'Use HTTPS',
    'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.':
      'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.',
    'https://cdn.example.com': 'https://cdn.example.com',
    'my-bucket': 'my-bucket',
    uploads: 'uploads',
  },
  zh: {
    'Access OSS over HTTPS': '通过 HTTPS 访问 OSS',
    'AccessKey ID': 'AccessKey ID',
    'AccessKey Secret': 'AccessKey Secret',
    'Alibaba Cloud AccessKey ID used to sign uploads':
      '用于签名上传请求的阿里云 AccessKey ID',
    'Aliyun OSS': '阿里云 OSS',
    'Aliyun OSS Object Storage': '阿里云对象存储 OSS',
    Bucket: 'Bucket',
    'Custom Domain': '自定义域名',
    'Directory prefix inside the bucket. Leave blank to write to the bucket root.':
      '文件在 Bucket 中的目录前缀，留空表示写入 Bucket 根目录',
    'Enable Aliyun OSS': '启用阿里云 OSS',
    'Enter AccessKey ID': '请输入 AccessKey ID',
    'Enter new secret to update': '输入新的密钥以更新',
    'Name of the OSS bucket that stores uploaded files':
      '存放上传文件的 OSS Bucket 名称',
    'OSS Endpoint': 'OSS 访问域名',
    'Object Key Prefix': '对象键前缀',
    'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.':
      '可选：用于拼接返回地址的 CDN 或自定义绑定域名，留空则使用 OSS 默认域名',
    'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.':
      'Bucket 所在地域的访问域名，可省略 https:// 与 Bucket 前缀',
    'Required when Aliyun OSS is enabled': '启用阿里云 OSS 后必填',
    'Save OSS settings': '保存 OSS 设置',
    'The configured bucket accepts uploads and deletes.':
      '已配置的 Bucket 可以正常上传与删除对象',
    'Uploads and removes a probe object using the saved configuration, so save your changes first.':
      '使用已保存的配置写入并删除一个探测对象，请先保存修改',
    'Use HTTPS': '使用 HTTPS',
    'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.':
      '启用后，新上传的文件将保存到阿里云 OSS，而不再写入服务器本地磁盘',
    'https://cdn.example.com': 'https://cdn.example.com',
    'my-bucket': 'my-bucket',
    uploads: 'uploads',
  },
  'zh-TW': {
    'Access OSS over HTTPS': '透過 HTTPS 存取 OSS',
    'AccessKey ID': 'AccessKey ID',
    'AccessKey Secret': 'AccessKey Secret',
    'Alibaba Cloud AccessKey ID used to sign uploads':
      '用於簽署上傳請求的阿里雲 AccessKey ID',
    'Aliyun OSS': '阿里雲 OSS',
    'Aliyun OSS Object Storage': '阿里雲物件儲存 OSS',
    Bucket: 'Bucket',
    'Custom Domain': '自訂網域',
    'Directory prefix inside the bucket. Leave blank to write to the bucket root.':
      '檔案在 Bucket 中的目錄前綴，留空表示寫入 Bucket 根目錄',
    'Enable Aliyun OSS': '啟用阿里雲 OSS',
    'Enter AccessKey ID': '請輸入 AccessKey ID',
    'Enter new secret to update': '輸入新的密鑰以更新',
    'Name of the OSS bucket that stores uploaded files':
      '存放上傳檔案的 OSS Bucket 名稱',
    'OSS Endpoint': 'OSS 存取網域',
    'Object Key Prefix': '物件鍵前綴',
    'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.':
      '選填：用於拼接回傳網址的 CDN 或自訂綁定網域，留空則使用 OSS 預設網域',
    'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.':
      'Bucket 所在地區的存取網域，可省略 https:// 與 Bucket 前綴',
    'Required when Aliyun OSS is enabled': '啟用阿里雲 OSS 後必填',
    'Save OSS settings': '儲存 OSS 設定',
    'The configured bucket accepts uploads and deletes.':
      '已設定的 Bucket 可以正常上傳與刪除物件',
    'Uploads and removes a probe object using the saved configuration, so save your changes first.':
      '使用已儲存的設定寫入並刪除一個探測物件，請先儲存變更',
    'Use HTTPS': '使用 HTTPS',
    'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.':
      '啟用後，新上傳的檔案將儲存到阿里雲 OSS，而不再寫入伺服器本機磁碟',
    'https://cdn.example.com': 'https://cdn.example.com',
    'my-bucket': 'my-bucket',
    uploads: 'uploads',
  },
  fr: {
    'Access OSS over HTTPS': 'Accéder à OSS via HTTPS',
    'AccessKey ID': 'AccessKey ID',
    'AccessKey Secret': 'AccessKey Secret',
    'Alibaba Cloud AccessKey ID used to sign uploads':
      'AccessKey ID Alibaba Cloud utilisé pour signer les envois',
    'Aliyun OSS': 'Aliyun OSS',
    'Aliyun OSS Object Storage': 'Stockage objet Aliyun OSS',
    Bucket: 'Bucket',
    'Custom Domain': 'Domaine personnalisé',
    'Directory prefix inside the bucket. Leave blank to write to the bucket root.':
      'Préfixe du répertoire dans le bucket. Laissez vide pour écrire à la racine du bucket.',
    'Enable Aliyun OSS': 'Activer Aliyun OSS',
    'Enter AccessKey ID': "Saisissez l'AccessKey ID",
    'Enter new secret to update':
      'Saisissez un nouveau secret pour mettre à jour',
    'Name of the OSS bucket that stores uploaded files':
      'Nom du bucket OSS qui stocke les fichiers envoyés',
    'OSS Endpoint': 'Point de terminaison OSS',
    'Object Key Prefix': "Préfixe de clé d'objet",
    'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.':
      'CDN ou domaine lié facultatif utilisé dans les URL renvoyées. Laissez vide pour utiliser le domaine OSS par défaut.',
    'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.':
      'Point de terminaison de la région du bucket. Le préfixe du bucket et https:// peuvent être omis.',
    'Required when Aliyun OSS is enabled':
      'Obligatoire lorsque Aliyun OSS est activé',
    'Save OSS settings': 'Enregistrer les paramètres OSS',
    'The configured bucket accepts uploads and deletes.':
      'Le bucket configuré accepte les envois et les suppressions.',
    'Uploads and removes a probe object using the saved configuration, so save your changes first.':
      "Envoie puis supprime un objet de test avec la configuration enregistrée ; enregistrez d'abord vos modifications.",
    'Use HTTPS': 'Utiliser HTTPS',
    'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.':
      'Lorsque cette option est activée, les nouveaux fichiers envoyés sont stockés dans Aliyun OSS au lieu du disque local.',
    'https://cdn.example.com': 'https://cdn.example.com',
    'my-bucket': 'my-bucket',
    uploads: 'uploads',
  },
  ja: {
    'Access OSS over HTTPS': 'HTTPS で OSS にアクセス',
    'AccessKey ID': 'AccessKey ID',
    'AccessKey Secret': 'AccessKey Secret',
    'Alibaba Cloud AccessKey ID used to sign uploads':
      'アップロードの署名に使用する Alibaba Cloud AccessKey ID',
    'Aliyun OSS': 'Aliyun OSS',
    'Aliyun OSS Object Storage': 'Aliyun OSS オブジェクトストレージ',
    Bucket: 'Bucket',
    'Custom Domain': 'カスタムドメイン',
    'Directory prefix inside the bucket. Leave blank to write to the bucket root.':
      'Bucket 内のディレクトリ接頭辞。空欄の場合は Bucket のルートに書き込みます。',
    'Enable Aliyun OSS': 'Aliyun OSS を有効にする',
    'Enter AccessKey ID': 'AccessKey ID を入力',
    'Enter new secret to update': '更新する場合は新しいシークレットを入力',
    'Name of the OSS bucket that stores uploaded files':
      'アップロードされたファイルを保存する OSS Bucket 名',
    'OSS Endpoint': 'OSS エンドポイント',
    'Object Key Prefix': 'オブジェクトキーの接頭辞',
    'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.':
      '返却 URL に使用する CDN またはバインド済みドメイン（任意）。空欄の場合は OSS の既定ドメインを使用します。',
    'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.':
      'Bucket のリージョンエンドポイント。https:// と Bucket プレフィックスは省略できます。',
    'Required when Aliyun OSS is enabled': 'Aliyun OSS を有効にした場合は必須',
    'Save OSS settings': 'OSS設定を保存',
    'The configured bucket accepts uploads and deletes.':
      '設定済みの Bucket でアップロードと削除が可能です。',
    'Uploads and removes a probe object using the saved configuration, so save your changes first.':
      '保存済みの設定でプローブオブジェクトを書き込み・削除します。先に変更を保存してください。',
    'Use HTTPS': 'HTTPS を使用',
    'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.':
      '有効にすると、新しくアップロードされたファイルはローカルディスクではなく Aliyun OSS に保存されます。',
    'https://cdn.example.com': 'https://cdn.example.com',
    'my-bucket': 'my-bucket',
    uploads: 'uploads',
  },
  ru: {
    'Access OSS over HTTPS': 'Доступ к OSS по HTTPS',
    'AccessKey ID': 'AccessKey ID',
    'AccessKey Secret': 'AccessKey Secret',
    'Alibaba Cloud AccessKey ID used to sign uploads':
      'AccessKey ID Alibaba Cloud для подписи загрузок',
    'Aliyun OSS': 'Aliyun OSS',
    'Aliyun OSS Object Storage': 'Объектное хранилище Aliyun OSS',
    Bucket: 'Bucket',
    'Custom Domain': 'Собственный домен',
    'Directory prefix inside the bucket. Leave blank to write to the bucket root.':
      'Префикс каталога внутри бакета. Оставьте пустым для записи в корень бакета.',
    'Enable Aliyun OSS': 'Включить Aliyun OSS',
    'Enter AccessKey ID': 'Введите AccessKey ID',
    'Enter new secret to update': 'Введите новый секрет для обновления',
    'Name of the OSS bucket that stores uploaded files':
      'Имя бакета OSS для хранения загруженных файлов',
    'OSS Endpoint': 'Endpoint OSS',
    'Object Key Prefix': 'Префикс ключа объекта',
    'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.':
      'Необязательный CDN или привязанный домен для возвращаемых URL. Оставьте пустым, чтобы использовать домен OSS по умолчанию.',
    'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.':
      'Endpoint региона бакета. Префикс бакета и https:// можно опустить.',
    'Required when Aliyun OSS is enabled':
      'Обязательно при включённом Aliyun OSS',
    'Save OSS settings': 'Сохранить настройки OSS',
    'The configured bucket accepts uploads and deletes.':
      'Настроенный бакет принимает загрузку и удаление объектов.',
    'Uploads and removes a probe object using the saved configuration, so save your changes first.':
      'Загружает и удаляет пробный объект с сохранённой конфигурацией, поэтому сначала сохраните изменения.',
    'Use HTTPS': 'Использовать HTTPS',
    'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.':
      'Если включено, новые загруженные файлы сохраняются в Aliyun OSS, а не на локальный диск.',
    'https://cdn.example.com': 'https://cdn.example.com',
    'my-bucket': 'my-bucket',
    uploads: 'uploads',
  },
  vi: {
    'Access OSS over HTTPS': 'Truy cập OSS qua HTTPS',
    'AccessKey ID': 'AccessKey ID',
    'AccessKey Secret': 'AccessKey Secret',
    'Alibaba Cloud AccessKey ID used to sign uploads':
      'AccessKey ID của Alibaba Cloud dùng để ký yêu cầu tải lên',
    'Aliyun OSS': 'Aliyun OSS',
    'Aliyun OSS Object Storage': 'Lưu trữ đối tượng Aliyun OSS',
    Bucket: 'Bucket',
    'Custom Domain': 'Tên miền tùy chỉnh',
    'Directory prefix inside the bucket. Leave blank to write to the bucket root.':
      'Tiền tố thư mục trong bucket. Để trống để ghi vào thư mục gốc của bucket.',
    'Enable Aliyun OSS': 'Bật Aliyun OSS',
    'Enter AccessKey ID': 'Nhập AccessKey ID',
    'Enter new secret to update': 'Nhập secret mới để cập nhật',
    'Name of the OSS bucket that stores uploaded files':
      'Tên bucket OSS lưu trữ các tệp đã tải lên',
    'OSS Endpoint': 'Endpoint OSS',
    'Object Key Prefix': 'Tiền tố khóa đối tượng',
    'Optional CDN or bound domain used in returned URLs. Leave blank to use the default OSS domain.':
      'CDN hoặc tên miền đã liên kết tùy chọn dùng trong URL trả về. Để trống để dùng tên miền OSS mặc định.',
    'Region endpoint of the bucket. The bucket prefix and https:// may be omitted.':
      'Endpoint khu vực của bucket. Có thể bỏ qua tiền tố bucket và https://.',
    'Required when Aliyun OSS is enabled': 'Bắt buộc khi bật Aliyun OSS',
    'Save OSS settings': 'Lưu cài đặt OSS',
    'The configured bucket accepts uploads and deletes.':
      'Bucket đã cấu hình chấp nhận tải lên và xóa.',
    'Uploads and removes a probe object using the saved configuration, so save your changes first.':
      'Tải lên và xóa một đối tượng thử bằng cấu hình đã lưu, hãy lưu thay đổi trước.',
    'Use HTTPS': 'Dùng HTTPS',
    'When enabled, newly uploaded files are stored in Aliyun OSS instead of the local disk.':
      'Khi bật, các tệp mới tải lên sẽ được lưu vào Aliyun OSS thay vì đĩa cục bộ.',
    'https://cdn.example.com': 'https://cdn.example.com',
    'my-bucket': 'my-bucket',
    uploads: 'uploads',
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
