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
import { readFileSync, readdirSync } from 'node:fs'
import { join } from 'node:path'
import { describe, expect, test } from 'vitest'

/**
 * 每一个**挂在路由上的**扩展页都必须自己能滚（用户 2026-… 报的那件事）。
 *
 * # 那条链（读一遍就明白为什么"内容一多就看不见下面"）
 *
 * ```text
 * AuthenticatedLayout
 *   SidebarInset      h-[calc(100svh-var(--app-header-height))] + overflow-hidden  ← ★ 它自己不滚
 *     Main            flex min-h-0 flex-1 flex-col overflow-hidden                 ← 也只是个盒子
 *       <div class='min-h-0 flex-1 overflow-auto …'>                               ← ★ 滚动条在这一层
 * ```
 *
 * 而那一层 `overflow-auto` 是 **`SectionPageLayout`** 画的。所以一个扩展页
 * 只要**不包它**（也不自己写一个滚动容器），超过一屏的内容就会被 `overflow-hidden`
 * 直接裁掉 —— **不报错、Web 控制台也没有任何提示**，用户看到的是
 * "这一页好像只有一半"。实测就是这么坏过三屏的（`world-plugins` / `mode-plaza` /
 * `rh-portal`），而另外四屏因为照了 `voice-plaza` 的写法，一直是对的。
 *
 * # 判据为什么从**路由表**来，而不是"扫所有 pages/*.tsx"
 *
 * 因为扩展的 `pages/` 下有一堆**不是页面**的东西：`rh-portal.tsx` 里导出的
 * `ParamField` / `ResultTile` / `ResultPreviewDialog` 都是**页内组件**，
 * 它们当然不该各包一层布局（那会画出三条滚动条）。真正的入口只有一个：
 * **`src/routes/_authenticated/<name>/index.tsx` 里 `component:` 指向的那个**。
 *
 * ⚠ 这条判据一旦成立，下一次新加扩展页时**漏包布局会当场红**，
 * 而不是等用户来报"我这一页下面看不到了"。
 */

const ROOT = process.cwd()
const ROUTES_DIR = join(ROOT, 'src', 'routes', '_authenticated')

/** 路由 → 它挂的那个扩展页文件（相对仓库根） */
function routedExtensionPages(): { route: string; file: string }[] {
  const out: { route: string; file: string }[] = []

  for (const name of readdirSync(ROUTES_DIR)) {
    const file = join(ROUTES_DIR, name, 'index.tsx')
    let code: string
    try {
      code = readFileSync(file, 'utf8')
    } catch {
      continue // 不是目录 / 没有 index.tsx
    }
    /* ⚠ 只认**扩展**的页面：宿主自己的页面由它们各自的布局负责 */
    const m = /from\s+'@\/extensions\/([^']+)'/.exec(code)
    if (!m) continue
    out.push({ route: name, file: `src/extensions/${m[1]}.tsx` })
  }
  return out
}

/**
 * ★★ **把注释摘掉再判**（变异验证逼出来的一条）。
 *
 * ⚠ 第一版直接在被测文件里找 `SectionPageLayout` 这个词，而**它是绿的** ——
 * 因为那三屏的注释里就写着这个名字（"必须包一层 `SectionPageLayout`"）。
 * 于是把真实的布局那一层整个删掉，守卫**照样绿**（变异验证当场抓到了它）。
 *
 * 这正是本工程记过的老教训（`modeEditorWiring.test.js` 那条）：
 * **在源码里找子串会把注释判成代码** —— 所以判据先摘注释。
 *
 * ⚠ 行注释只认"行首的 `//`"：`'http://…'` 那种写在字符串中间的不会被误伤
 * （摘多了只会造成**假红**，而那一眼就能看出；假绿才是要命的）。
 */
function stripComments(code: string): string {
  return code
    .replaceAll(/\/\*[\s\S]*?\*\//g, '')
    .replaceAll(/^[ \t]*\/\/.*$/gm, '')
}

/** 这一份源码里，页面的根有没有一个能滚的容器（**看的是代码，不是注释**） */
function hasScrollHost(rawCode: string): { ok: boolean; how: string } {
  const code = stripComments(rawCode)
  if (/\bSectionPageLayout\b/.test(code)) {
    return { ok: true, how: 'SectionPageLayout（推荐：它画了 overflow-auto 那一层）' }
  }
  if (/<Main[\s>]/.test(code)) {
    /* ⚠ `Main` 自己也**不滚**（它也只是 overflow-hidden 的盒子）—— 用了它的人
       必须自己再给一层 `overflow-auto/h-auto`，所以往下看那一层 */
    if (/overflow-(auto|y-auto|scroll)/.test(code)) {
      return { ok: true, how: '<Main> + 自己的 overflow-auto' }
    }
    return {
      ok: false,
      how: '用了 <Main> 但没有任何 overflow-auto —— 内容一多还是会被裁掉',
    }
  }
  if (/overflow-(auto|y-auto|scroll)/.test(code) && /h-full|min-h-0/.test(code)) {
    return { ok: true, how: '自己写的滚动容器' }
  }
  return {
    ok: false,
    how: '既没有 SectionPageLayout，也没有 <Main> + overflow-auto',
  }
}

describe('★ 扩展页的滚动（内容过多时必须能滚到下面）', () => {
  const pages = routedExtensionPages()

  test('★ 判据本身是活的：至少能找到几个挂路由的扩展页', () => {
    /*
     * ⚠ 这一条是"守卫的守卫"：路由目录改了名、或者 `component:` 换了写法，
     * 上面那个正则就会**一个都匹配不到** —— 而那种情况下所有断言都会
     * **空转通过**（`for` 一个都不跑），看起来一切正常。所以先钉一个下限。
     */
    expect(pages.length, '一个挂路由的扩展页都没找到 —— 判据失效了').toBeGreaterThanOrEqual(5)
  })

  test('★★ 每一个都包了能滚的容器（漏一个 = 那一屏下面看不见）', () => {
    const bad: string[] = []
    for (const { route, file } of pages) {
      const code = readFileSync(join(ROOT, file), 'utf8')
      const got = hasScrollHost(code)
      if (!got.ok) bad.push(`${route} → ${file}：${got.how}`)
    }
    expect(
      bad,
      '这些扩展页**下面会看不到了**（SidebarInset 是 overflow-hidden，页面得自己滚）',
    ).toEqual([])
  })

  test('★ 三个曾经坏过的屏，现在都走的是布局那一层', () => {
    /*
     * 用户报的那三个（它们原来都是一个裸的 `<div className='flex flex-col …'>`）：
     *  · /world-plugins → `zsy-world/pages/world-plugins-page.tsx`
     *  · /mode-plaza    → `zsy-mode/pages/mode-plaza-page.tsx`
     *  · /rh-app-center → `zsy-runninghub/pages/rh-portal.tsx`
     *
     * ⚠ 这一条**不重复**上面那条的判据（那一条是"全部"），它多钉一件事：
     * 这三屏用的是**同一个** `SectionPageLayout`（而不是各写各的滚动容器）——
     * 下一屏照抄时才有唯一一个样板。
     */
    for (const file of [
      'src/extensions/zsy-world/pages/world-plugins-page.tsx',
      'src/extensions/zsy-mode/pages/mode-plaza-page.tsx',
      'src/extensions/zsy-runninghub/pages/rh-portal.tsx',
    ]) {
      const code = readFileSync(join(ROOT, file), 'utf8')
      expect(code, `${file} 又变成裸 div 了`).toContain('SectionPageLayout')
    }
  })
})
