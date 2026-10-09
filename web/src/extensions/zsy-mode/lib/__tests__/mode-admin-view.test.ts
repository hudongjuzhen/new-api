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
import { describe, expect, test } from 'vitest'

import type { ModeEntitlementRow, ModeView } from '../../api'
import {
  accountModeState,
  accountModeStateLabel,
  bucketModes,
  emptyModesHint,
  entitlementState,
  entitlementStateLabel,
  grantedLabel,
  isPublicMode,
  modeVisibilityLabel,
  parseUserId,
  privateModesNote,
  type Translate,
} from '../mode-admin-view'

/**
 * 模式管理那一屏的**判断**（`zsy/mode` 的后台面）。
 *
 * # 这一组守的是**几句话**，不是布局
 *
 * 这一屏上最容易写错的不是排版，而是三个判断 —— 每一个错了都**不报错**，
 * 只是让运营做错下一步：
 *
 * | 判据 | 说错了会怎样 |
 * |---|---|
 * | ★★ **可见性默认是 private** | 运营以为自己设成公有了，而实际正好相反（或反过来：一份付费模式人人可开） |
 * | ★ 坏模式要列出来并说明 | 运营看到的是"我放进去的文件不见了"，以为后台坏了 |
 * | ★ "取消不删本地那份" | 运营以为收回就没了，而用户机器上那份还在、还能用 |
 *
 * # ⚠★ 这里每个函数都要一个 `t`
 *
 * 文案**不再写死在代码里**（用户 2026-… 报的"很多是英文 / 语言对不上"）：
 * 函数拿 `t` 现翻。所以这一组测试里的 `t` 是**恒等函数**（key 就是那句英文），
 * 断言查的是 key —— 而"每种语言下这句 key 有没有翻译"由语言包负责
 * （`i18n:sync` 的 missing/extras 报告 + `find-missing-keys.mjs`）。
 */
const t: Translate = ((key: string, options?: Record<string, unknown>) => {
  if (!options) return key
  let out = key
  for (const [name, value] of Object.entries(options)) {
    out = out.split(`{{${name}}}`).join(String(value))
  }
  return out
}) as Translate

function aMode(overrides: Partial<ModeView> = {}): ModeView {
  const id = overrides.id ?? 'mv'
  return {
    id,
    label: 'MV',
    medium: 'video',
    workScale: 'single',
    summary: '一句话说明',
    bytes: 12000,
    visibility: 'public',
    granted: 0,
    problem: '',
    /* ⚠ source 跟着 id 走（它是"运营该去改哪个文件"那一个答案，不该与 id 分叉） */
    source: `/srv/new-api/modes/${id}.json`,
    ...overrides,
  }
}

describe('模式列表分两堆', () => {
  test('坏的那几份进 broken（**也要显示**），好的进 usable', () => {
    const buckets = bucketModes([
      aMode({ id: 'mv' }),
      aMode({ id: 'bad', problem: '文件名与 id 不一致：文件名说 "bad"，而里面的 id 是 "other"。' }),
    ])

    expect(buckets.usable.map((r) => r.id)).toEqual(['mv'])
    expect(buckets.broken.map((r) => r.id)).toEqual(['bad'])
    /* ⚠ 坏的那一份连着它自己的说明与文件名一起给出去 —— 那是运营唯一能动手的地方 */
    expect(buckets.broken[0].problem).toContain('不一致')
    expect(buckets.broken[0].source).toContain('bad.json')
  })

  test('空列表 / undefined 都不炸', () => {
    expect(bucketModes([]).usable).toEqual([])
    expect(bucketModes([]).broken).toEqual([])
    // @ts-expect-error 服务端没回 items 时界面照样不该崩
    expect(bucketModes(undefined).usable).toEqual([])
  })
})

describe('★★ 可见性：认不出的一律按 private 说', () => {
  test('public / private 各有各的字', () => {
    expect(modeVisibilityLabel(t, 'public')).toBe('Public mode — anyone can open it')
    expect(modeVisibilityLabel(t, 'private')).toBe('Private mode — opened per account')
  })

  test('★★ 没写 / 认不出的一律回落到「私有」（与服务端同一个方向）', () => {
    /*
     * ⚠★ 服务端的 `readVisibility` 默认是 `private`（`public` 意味着"任何账号
     * 点一下就装上了"，而一份忘写这一格的模式悄悄变成人人可用**没有任何地方会报错**）。
     * 界面上若把"认不出"回落成「公共」，运营会以为自己设对了 —— 而实际正好相反。
     */
    for (const raw of ['', '   ', undefined, 'Public', 'publik', 'x']) {
      expect(modeVisibilityLabel(t, raw), `${JSON.stringify(raw)} 应当按私有说`).toBe(
        'Private mode — opened per account'
      )
    }
  })

  test('isPublicMode 的判据只有一格（且同样默认私有）', () => {
    expect(isPublicMode({ visibility: 'public' })).toBe(true)
    expect(isPublicMode({ visibility: 'private' })).toBe(false)
    expect(isPublicMode({ visibility: 'yolo' })).toBe(false)
  })
})

describe('★ "给了几个账号"那句话', () => {
  test('私有 + 0 说出的不只是 0，还要说下一步', () => {
    /*
     * ⚠ 一个 private 而**零开通**的模式多半意味着"运营忘了给谁开" ——
     * 用户那边看到的是"广场上没这一档"，运营这边看到的是"我明明放进去了"。
     * 两句话都对，而中间缺的就是这一下。所以那个 0 必须说得出下一步。
     */
    expect(grantedLabel(t, { visibility: 'private', granted: 0 })).toBe(
      'Private — nobody holds it yet'
    )
  })

  test('私有 + N', () => {
    expect(grantedLabel(t, { visibility: 'private', granted: 3 })).toBe(
      'Private — opened for 3 account(s)'
    )
  })

  test('公共的**不摆一个"0 个账号"**：那个数对它没有意义', () => {
    const line = grantedLabel(t, { visibility: 'public', granted: 0 })
    expect(line).toBe('Public — no opening needed')
    expect(line).not.toContain('0')
  })

  test('公共 + 另外几个单独开通（那是有信息量的）', () => {
    expect(grantedLabel(t, { visibility: 'public', granted: 2 })).toContain('2')
  })

  test('granted 缺失 / 是 NaN 时按 0 说（不画出 undefined）', () => {
    /*
     * ⚠ 服务端漏回这一格（或回了一个字符串）时，界面要**照样说人话**：
     * `undefined 个账号` / `NaN 个账号` 都是那种"看起来像坏了"的字。
     * ⚠ 这里的 `as unknown as …` 是**故意的**：类型上 `granted` 是必填的，
     * 而这一条测的正是"服务端没守约时界面怎么办"。
     */
    const missing = { visibility: 'private' } as unknown as Pick<ModeView, 'visibility' | 'granted'>
    expect(grantedLabel(t, missing)).not.toContain('undefined')
    const junk = { visibility: 'private', granted: 'x' } as unknown as Pick<
      ModeView,
      'visibility' | 'granted'
    >
    expect(grantedLabel(t, junk)).not.toContain('NaN')
  })

  test('★ 人数**读不到**（服务端压成负数）时说的是读不到，不是"0 个"', () => {
    /*
     * ⚠★ 那个 0 是一个结论（"没人有权限"），而读不到是"不知道"。
     * 把后者说成前者会让运营以为这一档没人用过 —— 见服务端 `liveGrantCountOf`。
     */
    expect(grantedLabel(t, { visibility: 'private', granted: -1 })).toBe(
      'That count could not be read'
    )
  })
})

describe('★ 一条授权行的状态：取消与过期是两件事', () => {
  const NOW = 1_700_000_000

  function row(overrides: Partial<ModeEntitlementRow> = {}): ModeEntitlementRow {
    return { id: 1, userId: 7, modeId: 'audiobook', source: 'admin', createdAt: 0, expiresAt: null, revokedAt: null, ...overrides }
  }

  test('没取消、没到期 → 生效中', () => {
    expect(entitlementState(row(), NOW)).toBe('active')
    expect(entitlementStateLabel(t, 'active')).toBe('In effect')
  })

  test('取消过 → 已取消（哪怕它本来还没到期）', () => {
    expect(entitlementState(row({ revokedAt: NOW - 10 }), NOW)).toBe('revoked')
    expect(entitlementStateLabel(t, 'revoked')).toBe('Already cancelled')
  })

  test('到期了 → 已过期（而不是"不生效"）', () => {
    expect(entitlementState(row({ expiresAt: NOW - 1 }), NOW)).toBe('expired')
    expect(entitlementState(row({ expiresAt: NOW + 1 }), NOW)).toBe('active')
    expect(entitlementStateLabel(t, 'expired')).toBe('Expired')
  })

  test('★ 三种状态各有各的字 —— 合成一个"不生效"会让运营看不出下一步做什么', () => {
    const labels = (['active', 'revoked', 'expired'] as const).map((s) =>
      entitlementStateLabel(t, s)
    )
    expect(new Set(labels).size).toBe(3)
  })
})

describe('★ 目录为空时那句能照做的话', () => {
  test('带上目录与白名单', () => {
    const line = emptyModesHint(t, '/srv/new-api/modes', ['video', 'audio', 'text'])
    expect(line).toContain('/srv/new-api/modes')
    expect(line).toContain('video / audio / text')
    expect(line).toContain('mv.json')
  })

  test('服务端没报目录/白名单时也不画出 undefined', () => {
    const line = emptyModesHint(t, '', [])
    expect(line).not.toContain('undefined')
  })
})

describe('★ 账号 ID 那一格', () => {
  test('正整数通过', () => {
    expect(parseUserId(t, '7')).toEqual({ value: 7, problem: '' })
    expect(parseUserId(t, '  12 ')).toEqual({ value: 12, problem: '' })
  })

  test('空的 / 非数字 / 0 / 负数 都要说清为什么（并给 0）', () => {
    for (const raw of ['', '   ', 'abc', '7a', '0', '-3', '1e3']) {
      const parsed = parseUserId(t, raw)
      expect(parsed.value, `${JSON.stringify(raw)} 不该算出一个账号`).toBe(0)
      expect(parsed.problem.length, `${JSON.stringify(raw)} 应当有话`).toBeGreaterThan(0)
    }
  })

  test('★ 那几句话是**英文 key**，不是写死的中文（否则换语言还是中文）', () => {
    expect(parseUserId(t, 'abc').problem).toBe(
      'The account ID is a run of digits, but this says “abc”.'
    )
    expect(parseUserId(t, '').problem).toBe(
      'Fill in an account ID (it is in the users list of the site’s admin panel).'
    )
  })
})

describe('★★ 卡片上"这一档开通了没有"那一格', () => {
  const ID = 'audiobook'

  test('没读到 / 账号 ID 还没填对 → unknown（**不许**画成"未开通"）', () => {
    /*
     * ⚠★ 这一支是整块最要紧的：一份**没读到**的数据与一份"他确实没有"的数据
     * 在这一格上长得一样，而前者会被运营读成"他没开通" → 再点一次开通
     * （服务端会多插一行，哪里都不报错）。
     */
    expect(accountModeState({ readable: false, active: [], modeId: ID })).toBe('unknown')
    /* ⚠ 哪怕服务端那份旧数据里写着他有，只要**这一份不算数**就得说读不到 */
    expect(accountModeState({ readable: false, active: [ID], modeId: ID })).toBe('unknown')
  })

  test('读回来了：在 active 里 → open，不在 → closed', () => {
    expect(accountModeState({ readable: true, active: [ID], modeId: ID })).toBe('open')
    expect(accountModeState({ readable: true, active: [], modeId: ID })).toBe('closed')
    /* ⚠ 判据是**那一档自己**：别人有、他没有，不能算成他开通了 */
    expect(accountModeState({ readable: true, active: ['mv'], modeId: ID })).toBe('closed')
  })

  test('三种状态各有各的字 —— 合成一个"未开通"会让运营重复开通', () => {
    const labels = (['unknown', 'open', 'closed'] as const).map((s) =>
      accountModeStateLabel(t, s)
    )
    expect(new Set(labels).size).toBe(3)
    expect(accountModeStateLabel(t, 'unknown')).toBe('Status unreadable')
    expect(accountModeStateLabel(t, 'open')).toBe('Opened')
    expect(accountModeStateLabel(t, 'closed')).toBe('Not opened')
  })
})

describe('★★ 那句与直觉相反的说明', () => {
  test('它同时说了"看不到"与"取消不删本地"', () => {
    /*
     * ⚠★ 两件都反直觉，所以两句都要有：
     *   ① 没开通的私有模式在广场上**根本看不到**（"我在哪儿预览一下"的答案是"看不到"）；
     *   ② 取消**不会**删掉用户机器上那一份（运营的第一反应是"收回了就没了"）。
     */
    const note = privateModesNote(t)
    expect(note).toContain('hidden, not greyed out')
    expect(note).toContain('is not deleted')
  })
})
