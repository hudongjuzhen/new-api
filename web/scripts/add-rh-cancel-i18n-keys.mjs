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

// Adds the RunningHub task-cancel keys (button, confirm dialog, toast) to every
// locale. Run from the web/ package root:
//   node scripts/add-rh-cancel-i18n-keys.mjs
const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const QUEUED_WARNING =
  'This task is still waiting for a free slot. Cancelling it removes it from the queue and refunds the charge.'
const RUNNING_WARNING =
  'The run will be stopped on RunningHub and the charge refunded. This cannot be undone.'

const newKeys = {
  en: {
    'Cancel task': 'Cancel task',
    'Cancel this task?': 'Cancel this task?',
    [QUEUED_WARNING]: QUEUED_WARNING,
    [RUNNING_WARNING]: RUNNING_WARNING,
    'Keep it running': 'Keep it running',
    'Task cancelled, the quota has been refunded':
      'Task cancelled, the quota has been refunded',
  },
  zh: {
    'Cancel task': '取消任务',
    'Cancel this task?': '确定取消该任务？',
    [QUEUED_WARNING]: '该任务仍在排队等待空闲名额。取消后会从队列中移除，并退还本次扣费。',
    [RUNNING_WARNING]: '将在 RunningHub 上停止该任务并退还本次扣费，此操作不可撤销。',
    'Keep it running': '继续运行',
    'Task cancelled, the quota has been refunded': '任务已取消，扣费已退还',
  },
  'zh-TW': {
    'Cancel task': '取消任務',
    'Cancel this task?': '確定取消該任務？',
    [QUEUED_WARNING]: '該任務仍在排隊等待空閒名額。取消後會從佇列中移除，並退還本次扣費。',
    [RUNNING_WARNING]: '將在 RunningHub 上停止該任務並退還本次扣費，此操作無法復原。',
    'Keep it running': '繼續執行',
    'Task cancelled, the quota has been refunded': '任務已取消，扣費已退還',
  },
  fr: {
    'Cancel task': 'Annuler la tâche',
    'Cancel this task?': 'Annuler cette tâche ?',
    [QUEUED_WARNING]:
      'Cette tâche attend encore un créneau libre. L’annuler la retire de la file d’attente et rembourse le montant débité.',
    [RUNNING_WARNING]:
      'L’exécution sera arrêtée sur RunningHub et le montant remboursé. Cette action est irréversible.',
    'Keep it running': 'Continuer l’exécution',
    'Task cancelled, the quota has been refunded':
      'Tâche annulée, le montant a été remboursé',
  },
  ja: {
    'Cancel task': 'タスクをキャンセル',
    'Cancel this task?': 'このタスクをキャンセルしますか？',
    [QUEUED_WARNING]:
      'このタスクは空き枠を待ってキューに入っています。キャンセルするとキューから削除され、課金は返金されます。',
    [RUNNING_WARNING]:
      'RunningHub 上で実行を停止し、課金を返金します。この操作は取り消せません。',
    'Keep it running': '実行を続ける',
    'Task cancelled, the quota has been refunded':
      'タスクをキャンセルしました。課金は返金されました',
  },
  ru: {
    'Cancel task': 'Отменить задачу',
    'Cancel this task?': 'Отменить эту задачу?',
    [QUEUED_WARNING]:
      'Задача всё ещё ждёт свободный слот. Отмена уберёт её из очереди и вернёт списанную сумму.',
    [RUNNING_WARNING]:
      'Выполнение будет остановлено на RunningHub, а списанная сумма возвращена. Действие необратимо.',
    'Keep it running': 'Продолжить выполнение',
    'Task cancelled, the quota has been refunded':
      'Задача отменена, списанная сумма возвращена',
  },
  vi: {
    'Cancel task': 'Hủy tác vụ',
    'Cancel this task?': 'Hủy tác vụ này?',
    [QUEUED_WARNING]:
      'Tác vụ này vẫn đang chờ chỗ trống. Hủy sẽ xóa nó khỏi hàng đợi và hoàn lại khoản đã trừ.',
    [RUNNING_WARNING]:
      'Tác vụ sẽ bị dừng trên RunningHub và khoản đã trừ được hoàn lại. Không thể hoàn tác.',
    'Keep it running': 'Tiếp tục chạy',
    'Task cancelled, the quota has been refunded':
      'Đã hủy tác vụ, khoản đã trừ đã được hoàn lại',
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
