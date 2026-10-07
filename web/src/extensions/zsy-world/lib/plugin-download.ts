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
import { issuePluginFile, type IssueResult } from '../api'

/**
 * Signs one plugin file for one account **and** downloads it, answering the signed
 * result so the caller can show what just happened.
 *
 * # 为什么"签发"与"下载"在一次调用里
 *
 * 页面需要两样东西：**文件到了用户手里**（下载），以及**那份文件上写了什么**
 * （`fileName` / `check` / `capabilities` / `granted` —— 提示那一句要用）。
 *
 * 分开写会变成"签两次"：一次为了下载、一次为了拿结果。而**签两次**在这里不是
 * 性能问题，是**正确性**问题 —— 两次签发的 `issuedAt` 不同，于是界面上显示的
 * `check` 与用户手上那份文件的 `check` **对不上**。运营照着界面核对文件时会以为
 * 文件被改过。
 *
 * 所以它只签一次，把结果一起交回来。
 *
 * # 为什么浏览器那套单独放一个文件
 *
 * object URL、造一个 `<a>`、点它、清理 —— 本仓库里**只有这一份实现**
 * （与 `zsy-tone` / `zsy-avatar` 的 CSV 导出同一个形状）。写在页面组件里会让
 * "下载到底有没有发生"只能靠眼睛看，而这一屏最要紧的动作就是它。
 *
 * ⚠ `revokeObjectURL` 放在 `finally` 里：点完就撤销最省内存，而浏览器在点击那一刻
 * 已经把内容取走了，所以不会截断下载。
 */
export async function signAndDownloadPluginFile(
  userId: number,
  pluginId: string,
  capabilities?: string[]
): Promise<IssueResult> {
  const result = await issuePluginFile(userId, pluginId, capabilities)
  saveTextAsFile(result.file, result.fileName, 'application/json;charset=utf-8')
  return result
}

/**
 * Saves one string as a file in the browser.
 *
 * 它被单独拆出来，是为了让"下载"这一步可以被单测直接调用（而不必先跑一遍网络）
 * —— 页面测试里那条"点一下真的存了个文件"用的就是它。
 */
export function saveTextAsFile(text: string, fileName: string, mime: string): void {
  const blob = new Blob([text], { type: mime })
  const url = URL.createObjectURL(blob)
  try {
    const link = document.createElement('a')
    link.href = url
    link.download = fileName
    document.body.append(link)
    link.click()
    link.remove()
  } finally {
    URL.revokeObjectURL(url)
  }
}
