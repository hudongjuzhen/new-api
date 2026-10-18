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

// One-off migration script: the mode plaza (/mode-plaza) gained a "delete this
// mode" action, which needs its confirm dialog copy. Run from web/:
// node scripts/add-mode-delete-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Delete {{mode}}': 'Delete {{mode}}',
    'Delete this mode?': 'Delete this mode?',
    'Could not delete this mode': 'Could not delete this mode',
    'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.':
      'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.',
  },
  zh: {
    'Delete {{mode}}': '删除「{{mode}}」',
    'Delete this mode?': '删除这一档模式？',
    'Could not delete this mode': '删除这一档模式失败',
    'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.':
      '删除「{{name}}」会从模式目录里移除 {{source}}，于是这一档对任何账号都不再出现在广场上。用户机器上已经下载的那一份不会被删掉，已开通它的账号也继续保留权限 —— 把同名文件放回去，这一档就回来了。',
  },
  'zh-TW': {
    'Delete {{mode}}': '刪除「{{mode}}」',
    'Delete this mode?': '刪除這一檔模式？',
    'Could not delete this mode': '刪除這一檔模式失敗',
    'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.':
      '刪除「{{name}}」會從模式目錄移除 {{source}}，於是這一檔對任何帳號都不再出現在廣場上。使用者機器上已下載的那一份不會被刪除，已開通它的帳號也繼續保留權限 —— 把同名檔案放回去，這一檔就回來了。',
  },
  fr: {
    'Delete {{mode}}': 'Supprimer « {{mode}} »',
    'Delete this mode?': 'Supprimer ce mode ?',
    'Could not delete this mode': 'Impossible de supprimer ce mode',
    'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.':
      'Supprimer « {{name}} » retire {{source}} du répertoire des modes : il disparaît de la place publique pour tous les comptes. La copie déjà téléchargée sur la machine d’un utilisateur n’est pas supprimée, et les comptes qui y ont accès gardent leurs droits — remettre un fichier du même nom le rétablit.',
  },
  ja: {
    'Delete {{mode}}': '「{{mode}}」を削除',
    'Delete this mode?': 'このモードを削除しますか？',
    'Could not delete this mode': 'このモードを削除できませんでした',
    'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.':
      '「{{name}}」を削除すると、モードディレクトリから {{source}} が取り除かれ、このモードはどのアカウントのプラザにも表示されなくなります。ユーザーの端末にダウンロード済みのコピーは削除されず、開通済みのアカウントも権限を保持します —— 同じ名前のファイルを戻せば元どおりになります。',
  },
  ru: {
    'Delete {{mode}}': 'Удалить «{{mode}}»',
    'Delete this mode?': 'Удалить этот режим?',
    'Could not delete this mode': 'Не удалось удалить этот режим',
    'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.':
      'Удаление «{{name}}» убирает {{source}} из каталога режимов, поэтому режим пропадает с площади для всех аккаунтов. Копия, уже загруженная на машину пользователя, не удаляется, а открытые для него аккаунты сохраняют свои права — если вернуть файл с тем же именем, режим восстановится.',
  },
  vi: {
    'Delete {{mode}}': 'Xóa “{{mode}}”',
    'Delete this mode?': 'Xóa chế độ này?',
    'Could not delete this mode': 'Không xóa được chế độ này',
    'Deleting “{{name}}” removes {{source}} from the mode directory, so the mode leaves the plaza for every account. A copy a user already downloaded stays on that machine, and the accounts opened for it keep their grants — putting a file with the same name back restores the mode.':
      'Xóa “{{name}}” sẽ gỡ {{source}} khỏi thư mục chế độ, nên chế độ này không còn xuất hiện trên quảng trường với bất kỳ tài khoản nào. Bản đã tải về máy người dùng không bị xóa, và các tài khoản đã mở nó vẫn giữ quyền — đặt lại tệp cùng tên là chế độ trở lại.',
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
