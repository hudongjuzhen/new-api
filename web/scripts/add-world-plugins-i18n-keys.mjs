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
/*
One-off migration script: adds the World IP · Plugins (zsy-world) keys to all
seven locales. Run from web/: node scripts/add-world-plugins-i18n-keys.mjs

It follows scripts/add-tone-plaza-i18n-keys.mjs, and the same two rules apply:

  · **Only keys that are actually missing get written.** A key that already
    exists belongs to some other page, and overwriting its translation would
    silently change that page's wording — so its current value is left alone.
  · **The table holds only this page's own strings.** Shared interface wording
    it reuses (`Plugin templates`, `Reload`, `Account ID`, `Grant`, `Revoke`,
    `Active`, …) is deliberately absent: if it already exists, reusing it is the
    point — the two admin pages then read identically in every language.

The sentences here are the ones an operator acts on, so the translations say
what to *do* rather than paraphrasing the English. The two that matter most:

  · `Signing is not granting` — 签发只产出一个文件；能不能用由下面的授予决定。
  · `Signed — but this account cannot use it yet` — 文件装得上、可是会被服务端拒绝。

Those two are the page's whole reason for existing: an operator who reads
"生成成功" as "已经开通" will send the file and then get "你给我的东西用不了".
*/
import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return `${JSON.stringify(obj, null, 2)}\n`
}

const newKeys = {
  en: {
    'World IP': 'World IP',
    'World IP · Plugins': 'World IP · Plugins',
    'Signing is not granting': 'Signing is not granting',
    'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.':
      'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.',
    'Sign a file for an account': 'Sign a file for an account',
    'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.':
      'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.',
    'Sign and download': 'Sign and download',
    'The account that will import this file.':
      'The account that will import this file.',
    'Capabilities of this account': 'Capabilities of this account',
    'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.':
      'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.',
    'Not held': 'Not held',
    'Signed and ready to send': 'Signed and ready to send',
    'Signed — but this account cannot use it yet':
      'Signed — but this account cannot use it yet',
    'History (including revoked and expired rows)':
      'History (including revoked and expired rows)',
    'Fill in an account ID above to see and change its capabilities.':
      'Fill in an account ID above to see and change its capabilities.',
    'Fix the account ID above to grant or revoke.':
      'Fix the account ID above to grant or revoke.',
  },
  zh: {
    'World IP': '世界 IP',
    'World IP · Plugins': '世界 IP · 插件管理',
    'Signing is not granting': '签发 ≠ 授予',
    'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.':
      '签发只产出一个文件。这个账号能不能真的用，由下面的「授予」决定，而且服务端每一次请求都会重判。没有授予的文件装得上，打开那一屏时会被拒绝。',
    'Sign a file for an account': '给某个账号签发一份文件',
    'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.':
      '文件上写着它是给哪个账号的。换一个账号导入会被拒绝，并把两个账号都说出来 —— 所以发错人会在导入那一刻暴露。',
    'Sign and download': '签发并下载',
    'The account that will import this file.': '将来导入这份文件的账号。',
    'Capabilities of this account': '这个账号的能力',
    'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.':
      '在这里决定，而且每一次请求都会重判 —— 撤销立刻生效，用户不需要重新登录。',
    'Not held': '未持有',
    'Signed and ready to send': '已签好，可以发了',
    'Signed — but this account cannot use it yet': '已签好 —— 但这个账号还不能用',
    'History (including revoked and expired rows)':
      '历史（含已撤销与已过期的记录）',
    'Fill in an account ID above to see and change its capabilities.':
      '在上面填一个账号 ID，就能看到并修改它的能力。',
    'Fix the account ID above to grant or revoke.':
      '先把上面的账号 ID 改对，再授予或撤销。',
  },
  'zh-TW': {
    'World IP': '世界 IP',
    'World IP · Plugins': '世界 IP · 外掛管理',
    'Signing is not granting': '簽發 ≠ 授予',
    'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.':
      '簽發只產出一個檔案。這個帳號能不能真的用，由下面的「授予」決定，而且伺服器每一次請求都會重判。沒有授予的檔案裝得上，打開那一屏時會被拒絕。',
    'Sign a file for an account': '給某個帳號簽發一份檔案',
    'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.':
      '檔案上寫著它是給哪個帳號的。換一個帳號匯入會被拒絕，並把兩個帳號都說出來 —— 所以發錯人會在匯入那一刻暴露。',
    'Sign and download': '簽發並下載',
    'The account that will import this file.': '將來匯入這份檔案的帳號。',
    'Capabilities of this account': '這個帳號的能力',
    'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.':
      '在這裡決定，而且每一次請求都會重判 —— 撤銷立刻生效，使用者不需要重新登入。',
    'Not held': '未持有',
    'Signed and ready to send': '已簽好，可以發了',
    'Signed — but this account cannot use it yet': '已簽好 —— 但這個帳號還不能用',
    'History (including revoked and expired rows)':
      '歷史（含已撤銷與已過期的紀錄）',
    'Fill in an account ID above to see and change its capabilities.':
      '在上面填一個帳號 ID，就能看到並修改它的能力。',
    'Fix the account ID above to grant or revoke.':
      '先把上面的帳號 ID 改對，再授予或撤銷。',
  },
  fr: {
    'World IP': 'World IP',
    'World IP · Plugins': 'World IP · Extensions',
    'Signing is not granting': "Signer n'est pas accorder",
    'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.':
      "Signer ne produit qu'un fichier. Le fait que le compte puisse réellement l'utiliser dépend des autorisations ci-dessous, revérifiées à chaque requête par le serveur. Un fichier signé sans autorisation s'installe, puis est refusé à l'ouverture.",
    'Sign a file for an account': 'Signer un fichier pour un compte',
    'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.':
      "Le fichier indique le compte pour lequel il a été signé. L'importer avec un autre compte est refusé, avec les deux noms affichés : une erreur d'envoi se voit donc dès l'import.",
    'Sign and download': 'Signer et télécharger',
    'The account that will import this file.':
      'Le compte qui importera ce fichier.',
    'Capabilities of this account': 'Autorisations de ce compte',
    'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.':
      "Décidé ici, et revérifié à chaque requête — une révocation prend effet immédiatement, sans que l'utilisateur ait à se reconnecter.",
    'Not held': 'Non accordée',
    'Signed and ready to send': 'Signé, prêt à envoyer',
    'Signed — but this account cannot use it yet':
      "Signé — mais ce compte ne peut pas encore l'utiliser",
    'History (including revoked and expired rows)':
      'Historique (y compris révoquées et expirées)',
    'Fill in an account ID above to see and change its capabilities.':
      'Saisissez un identifiant de compte ci-dessus pour voir et modifier ses autorisations.',
    'Fix the account ID above to grant or revoke.':
      "Corrigez d'abord l'identifiant de compte ci-dessus, puis accordez ou révoquez.",
  },
  ja: {
    'World IP': 'ワールド IP',
    'World IP · Plugins': 'ワールド IP · プラグイン管理',
    'Signing is not granting': '発行と付与は別です',
    'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.':
      '発行はファイルを作るだけです。そのアカウントが実際に使えるかは下の「付与」で決まり、サーバーが毎回のリクエストで判定します。付与なしのファイルはインストールできますが、開いた時点で拒否されます。',
    'Sign a file for an account': 'アカウント向けにファイルを発行',
    'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.':
      'ファイルには発行先のアカウントが書かれています。別のアカウントで取り込むと拒否され、両方のアカウント名が表示されます。送り間違いは取り込み時に分かります。',
    'Sign and download': '発行してダウンロード',
    'The account that will import this file.':
      'このファイルを取り込むアカウント。',
    'Capabilities of this account': 'このアカウントの権限',
    'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.':
      'ここで決まり、毎回のリクエストで再判定されます。取り消しは即時有効で、ユーザーの再ログインは不要です。',
    'Not held': '未付与',
    'Signed and ready to send': '発行済み、送信できます',
    'Signed — but this account cannot use it yet':
      '発行済み —— ただしこのアカウントはまだ使えません',
    'History (including revoked and expired rows)':
      '履歴（取り消し済み・期限切れを含む）',
    'Fill in an account ID above to see and change its capabilities.':
      '上にアカウント ID を入力すると、その権限を確認・変更できます。',
    'Fix the account ID above to grant or revoke.':
      '先に上のアカウント ID を直してから付与・取り消しを行ってください。',
  },
  ru: {
    'World IP': 'World IP',
    'World IP · Plugins': 'World IP · Плагины',
    'Signing is not granting': 'Выдача файла — это не выдача прав',
    'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.':
      'Подпись создаёт только файл. Сможет ли аккаунт им пользоваться, решают разрешения ниже, и сервер проверяет это при каждом запросе. Файл без разрешения установится, но при открытии будет отклонён.',
    'Sign a file for an account': 'Подписать файл для аккаунта',
    'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.':
      'В файле указан аккаунт, для которого он подписан. Импорт под другим аккаунтом отклоняется с показом обоих имён — ошибка отправки видна сразу при импорте.',
    'Sign and download': 'Подписать и скачать',
    'The account that will import this file.':
      'Аккаунт, который импортирует этот файл.',
    'Capabilities of this account': 'Разрешения этого аккаунта',
    'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.':
      'Решается здесь и перепроверяется при каждом запросе — отзыв действует сразу, повторный вход не нужен.',
    'Not held': 'Не выдано',
    'Signed and ready to send': 'Подписано, можно отправлять',
    'Signed — but this account cannot use it yet':
      'Подписано — но этот аккаунт пока не может пользоваться',
    'History (including revoked and expired rows)':
      'История (включая отозванные и истёкшие)',
    'Fill in an account ID above to see and change its capabilities.':
      'Укажите ID аккаунта выше, чтобы увидеть и изменить его разрешения.',
    'Fix the account ID above to grant or revoke.':
      'Сначала исправьте ID аккаунта выше, затем выдавайте или отзывайте.',
  },
  vi: {
    'World IP': 'World IP',
    'World IP · Plugins': 'World IP · Tiện ích',
    'Signing is not granting': 'Phát hành không phải là cấp quyền',
    'Signing only produces a file. Whether the account can actually use the capability is decided by the grants below, and checked on every request by the server. A signed file with no grant installs fine and then gets refused when opened.':
      'Phát hành chỉ tạo ra một tệp. Tài khoản có dùng được hay không do phần cấp quyền bên dưới quyết định, và máy chủ kiểm tra lại ở mỗi yêu cầu. Tệp không có quyền vẫn cài được, nhưng sẽ bị từ chối khi mở.',
    'Sign a file for an account': 'Phát hành tệp cho một tài khoản',
    'The file carries the account it was signed for. Importing it under a different account is refused with both account names, so a mix-up surfaces at import time.':
      'Tệp ghi rõ tài khoản được phát hành. Nhập bằng tài khoản khác sẽ bị từ chối và hiện cả hai tên tài khoản — gửi nhầm sẽ lộ ra ngay khi nhập.',
    'Sign and download': 'Phát hành và tải về',
    'The account that will import this file.':
      'Tài khoản sẽ nhập tệp này.',
    'Capabilities of this account': 'Quyền của tài khoản này',
    'Decided here, and re-checked on every request — revoking takes effect immediately, with no need for the user to log in again.':
      'Quyết định ở đây và được kiểm tra lại ở mỗi yêu cầu — thu hồi có hiệu lực ngay, người dùng không cần đăng nhập lại.',
    'Not held': 'Chưa có',
    'Signed and ready to send': 'Đã phát hành, sẵn sàng gửi',
    'Signed — but this account cannot use it yet':
      'Đã phát hành — nhưng tài khoản này chưa dùng được',
    'History (including revoked and expired rows)':
      'Lịch sử (gồm cả đã thu hồi và đã hết hạn)',
    'Fill in an account ID above to see and change its capabilities.':
      'Nhập ID tài khoản ở trên để xem và thay đổi quyền của tài khoản đó.',
    'Fix the account ID above to grant or revoke.':
      'Sửa ID tài khoản ở trên trước, rồi mới cấp hoặc thu hồi.',
  },
}

async function main() {
  const entries = await fs.readdir(LOCALES_DIR, { withFileTypes: true })
  const files = entries
    .filter((e) => e.isFile() && e.name.endsWith('.json'))
    .map((e) => e.name)

  let added = 0
  let skipped = 0

  for (const file of files) {
    const locale = file.replace(/\.json$/, '')
    const table = newKeys[locale]
    if (!table) {
      console.log(`· ${file}: 没有这一份的表，跳过`)
      continue
    }

    const full = path.join(LOCALES_DIR, file)
    const json = JSON.parse(await fs.readFile(full, 'utf8'))
    const translation = json.translation ?? {}

    let wrote = 0
    for (const [key, value] of Object.entries(table)) {
      /*
       * ★ 已有的一律不碰：那个 key 属于别的页面，覆盖它的译文会**静默改掉那一页**。
       * 这也是这个脚本"可以反复跑"的原因。
       *
       * ⚠ 新 key 一律**追加在末尾**（不在这里排序）：`sync-i18n.mjs` 会把每一份
       * 语言的次序对齐到 en，那是它的活。这里再排一遍就有了两处规则，而两处规则
       * 迟早会不一致 —— 不一致的表现是每跑一次 sync 都产生一大片无意义的 diff。
       */
      if (Object.hasOwn(translation, key)) {
        skipped += 1
        continue
      }
      translation[key] = value
      wrote += 1
      added += 1
    }

    json.translation = translation
    await fs.writeFile(full, stableStringify(json), 'utf8')
    console.log(`✓ ${file}: 新增 ${wrote} 个，跳过已存在 ${Object.keys(table).length - wrote} 个`)
  }

  console.log(`\n共新增 ${added} 个 key，跳过 ${skipped} 个（已存在于别的页面）`)
  console.log('建议接着跑：node scripts/sync-i18n.mjs  （把次序与 en 对齐）')
}

await main()
