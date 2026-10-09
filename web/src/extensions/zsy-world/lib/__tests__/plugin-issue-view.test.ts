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

import type { PluginTemplateView } from '../../api'
import {
  accountCapabilityState,
  accountCapabilityStateLabel,
  bucketTemplates,
  capabilityHintFallback,
  capabilityHints,
  emptyTemplatesHint,
  entitlementState,
  entitlementStateLabel,
  issueSuccessLine,
  parseUserId,
  templatesOfCapabilities,
  visibilityLabel,
  type Translate,
} from '../plugin-issue-view'

/**
 * 这一屏的**判断**（"该说什么"）。
 *
 * # 这一组守的是三件事
 *
 * | 判据 | 说错了会怎样 |
 * |---|---|
 * | ★★ **能力名册来自模板**（不是第二份手写常量） | 新加一份插件之后那一档**没人能开通**（名册里根本没有它） |
 * | ★★ 每个能力说得出**是哪几份插件要的** | 运营不知道开通之后他能打开什么、该签哪一份文件给他 |
 * | ★ 认不出的可见性按 private 说 | 付费插件被读成"人人可装" |
 *
 * # ⚠★ 这里每个函数都要一个 `t`
 *
 * 文案**不再写死在代码里**（用户 2026-… 报的"很多是英文 / 语言对不上"）：
 * 函数拿 `t` 现翻。所以这一组测试里的 `t` 是**恒等函数**（key 就是那句英文），
 * 断言查的是 key —— 而"每种语言下这句 key 有没有翻译"由语言包负责。
 */
const t: Translate = ((key: string, options?: Record<string, unknown>) => {
  /*
   * ⚠★ 那个"顿号"也是一句**要翻译的话**（中文用「、」，英法用「, 」）——
   * 所以它有一个 key（`, `）。恒等 `t` 会把它原样返回，于是这里补上中文那一版：
   * 这一条测试断言的正是"两个名字怎么连起来"。
   */
  if (key === ', ') return '、'
  if (!options) return key
  let out = key
  for (const [name, value] of Object.entries(options)) {
    out = out.split(`{{${name}}}`).join(String(value))
  }
  return out
}) as Translate

function aTemplate(overrides: Partial<PluginTemplateView> = {}): PluginTemplateView {
  const id = overrides.id ?? 'world-ip'
  return {
    id,
    name: '世界 IP 资源管理器',
    capabilities: ['world-ip'],
    visibility: 'private',
    screens: ['世界 IP'],
    problem: '',
    source: `/srv/new-api/plugin-templates/${id}.json`,
    ...overrides,
  }
}

describe('★★ 能力名册（这一屏要列哪几行）', () => {
  test('名册来自**模板**，而且去重 + 排序', () => {
    const rows = [
      aTemplate({ id: 'b', capabilities: ['world-ip', 'world-ip-ai'] }),
      aTemplate({ id: 'a', capabilities: ['world-ip', 'zeta'] }),
    ]

    /* ⚠ 去重：两个模板都要 world-ip，而那一档只该有一行 */
    /* ⚠ 排序：顺序不跟着文件名走 —— 否则两次刷新看到的名册次序可能不同 */
    expect(capabilityHints(rows)).toEqual(['world-ip', 'world-ip-ai', 'zeta'])
  })

  test('坏模板**不算**名册（它的能力一格都不可信）', () => {
    /*
     * ⚠ 这一条与"坏模板照样列出来"不矛盾：坏模板要**露面**（在模板那一块带着原因），
     * 但它的 `capabilities` 是那份读坏了的文件里读出来的东西 —— 拿它去开权限
     * 就是拿一个不确定的名字去授权。
     */
    const buckets = bucketTemplates([
      aTemplate({ id: 'good', capabilities: ['world-ip'] }),
      aTemplate({ id: 'bad', capabilities: ['ghost'], problem: '文件名与 id 不一致' }),
    ])

    expect(capabilityHints(buckets.usable)).toEqual(['world-ip'])
    expect(buckets.broken.map((r) => r.id)).toEqual(['bad'])
  })

  test('空白的 / 缺字段的能力名不进名册（不画出空字符串）', () => {
    const rows = [
      aTemplate({ capabilities: ['', '   ', 'world-ip'] }),
      aTemplate({ id: 'empty', capabilities: [] }),
    ]

    expect(capabilityHints(rows)).toEqual(['world-ip'])
    expect(capabilityHints([])).toEqual([])
  })
})

describe('★★ 每个能力是哪几份插件要的', () => {
  test('一个能力被两份插件要时，两个名字都在', () => {
    const rows = [
      aTemplate({ id: 'a', name: '资源管理器', capabilities: ['world-ip'] }),
      aTemplate({ id: 'b', name: '世界对话', capabilities: ['world-ip', 'world-chat'] }),
    ]

    const hints = templatesOfCapabilities(t, rows)
    expect(hints['world-ip']).toBe('资源管理器、世界对话')
    expect(hints['world-chat']).toBe('世界对话')
  })

  test('同一份插件被重复列两次时名字只出现一次', () => {
    const rows = [
      aTemplate({ id: 'a', name: '资源管理器', capabilities: ['world-ip'] }),
      aTemplate({ id: 'a', name: '资源管理器', capabilities: ['world-ip'] }),
    ]

    expect(templatesOfCapabilities(t, rows)['world-ip']).toBe('资源管理器')
  })

  test('模板没写 name 时回落到 id（绝不空着）', () => {
    const hints = templatesOfCapabilities(t, [aTemplate({ id: 'world-ip', name: '' })])
    expect(hints['world-ip']).toBe('world-ip')
  })

  test('没有模板要它时**没有这一格**，由界面说那句回落的话', () => {
    /*
     * ⚠★ 回落那句话由 `capabilityHintFallback` 给（不是界面现编）：这一屏上
     * 每一句给运营看的话都该有人钉得住，否则它会成为界面上唯一一句没人守的字。
     */
    expect(templatesOfCapabilities(t, [])).toEqual({})
    expect(capabilityHintFallback(t)).toBe('No plugin asks for it')
  })
})

describe('★ 可见性：认不出的一律按 private 说', () => {
  test('public / private 各有各的字，认不出的一律回落到「私有」', () => {
    expect(visibilityLabel(t, 'public')).toBe('Public plugin — one-click install')
    expect(visibilityLabel(t, 'private')).toBe('Private plugin — signed per account')
    for (const raw of ['', '   ', undefined, 'Public', 'publik']) {
      expect(visibilityLabel(t, raw), `${JSON.stringify(raw)} 应当按私有说`).toBe(
        'Private plugin — signed per account'
      )
    }
  })
})

describe('★★ 一行上"这个账号开通了没有"那一格', () => {
  test('没读到 → unknown（**不许**画成"未开通"）', () => {
    expect(accountCapabilityState({ readable: false, active: [], capability: 'world-ip' })).toBe(
      'unknown'
    )
    expect(
      accountCapabilityState({ readable: false, active: ['world-ip'], capability: 'world-ip' })
    ).toBe('unknown')
    expect(accountCapabilityStateLabel(t, 'unknown')).toBe('Status unreadable')
  })

  test('读回来了：在 active 里 → open，不在 → closed', () => {
    expect(
      accountCapabilityState({ readable: true, active: ['world-ip'], capability: 'world-ip' })
    ).toBe('open')
    expect(accountCapabilityState({ readable: true, active: [], capability: 'world-ip' })).toBe(
      'closed'
    )
    expect(accountCapabilityStateLabel(t, 'open')).toBe('Opened')
    expect(accountCapabilityStateLabel(t, 'closed')).toBe('Not opened')
  })
})

describe('★ 一条权限行的状态：取消与过期是两件事', () => {
  const NOW = 1_700_000_000

  test('三种状态各有各的字', () => {
    expect(entitlementState({ expiresAt: null, revokedAt: null }, NOW)).toBe('active')
    expect(entitlementState({ expiresAt: null, revokedAt: 1 }, NOW)).toBe('revoked')
    expect(entitlementState({ expiresAt: NOW - 1, revokedAt: null }, NOW)).toBe('expired')
    const labels = (['active', 'revoked', 'expired'] as const).map((s) =>
      entitlementStateLabel(t, s)
    )
    expect(new Set(labels).size).toBe(3)
  })
})

describe('★ 目录为空时那句能照做的话', () => {
  test('带上目录与能写的 source', () => {
    const line = emptyTemplatesHint(t, '/srv/plugin-templates', ['worldOps'])
    expect(line).toContain('/srv/plugin-templates')
    expect(line).toContain('worldOps')
    expect(line).toContain('world-ip.json')
  })

  test('服务端没报目录/白名单时也不画出 undefined', () => {
    expect(emptyTemplatesHint(t, '', [])).not.toContain('undefined')
  })
})

describe('★ 账号 ID 那一格', () => {
  test('正整数通过；其余都说清为什么（并给 0）', () => {
    expect(parseUserId(t, '7')).toEqual({ value: 7, problem: '' })
    for (const raw of ['', '   ', 'abc', '7a', '0', '-3']) {
      const parsed = parseUserId(t, raw)
      expect(parsed.value, `${JSON.stringify(raw)} 不该算出一个账号`).toBe(0)
      expect(parsed.problem.length, `${JSON.stringify(raw)} 应当有话`).toBeGreaterThan(0)
    }
  })
})

describe('★ 签发成功那句话', () => {
  function issued(overrides: Record<string, unknown> = {}) {
    return {
      fileName: 'world-ip-v0.1.0-zsy-user7-20261007.aimv-plugin.json',
      file: '{}',
      pluginId: 'world-ip',
      pluginVersion: '0.1.0',
      userId: 7,
      username: 'zsy',
      site: 'https://example.com',
      check: 'fnv1a64:0',
      capabilities: ['world-ip', 'world-ip-ai'],
      granted: [],
      ...overrides,
    }
  }

  test('★★ 还差能力时，说的是"下面把这些能力开通给他"（不是"开通某个插件"）', () => {
    /*
     * ⚠★ 这一版把开通动作搬到了**账号**那一边（用户 2026-…）：
     * 那一句若还指着"某一个插件"，运营会回到旧路（先选插件再点）——
     * 而那条路已经被拿掉了。
     */
    const line = issueSuccessLine(t, issued())
    expect(line).toContain('world-ip')
    expect(line).toContain('world-ip-ai')
    expect(line).toContain('below')
  })

  test('能力都持有 → 那句警告不见了', () => {
    const line = issueSuccessLine(t, issued({ granted: ['world-ip', 'world-ip-ai'] }))
    expect(line).toContain('send the file over for import')
    expect(line).not.toContain('still missing')
  })

  test('版本一起说出来（运营手上有好几份看着一样的文件）', () => {
    expect(issueSuccessLine(t, issued())).toContain('v0.1.0')
  })

  test('一个都没持有时，那一格是"（一个都没有）"而不是空串', () => {
    expect(issueSuccessLine(t, issued())).toContain('(none at all)')
  })
})
