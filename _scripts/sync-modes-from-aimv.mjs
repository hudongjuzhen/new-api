/**
 * 把 `D:\MV项目\aimv-studio\modes\*.json` 同步进 `D:\work\new-api\modes\`。
 *
 * 两份的差异**只允许**是那两格宿主字段（`x-visibility` / `x-summary`）——
 * 这是 `zsy/mode/catalog.go` 文件头那条立足点（"服务端这一份就是用户在本地会拿到的那一份"），
 * 也是 `TestShippedModes_ServedTextIsTheFileMinusHostKeys` 盯的东西。
 *
 * 用法：
 *   node _scripts/sync-modes-from-aimv.mjs            # 干跑（只报告差异）
 *   node _scripts/sync-modes-from-aimv.mjs --write    # 真写
 *
 * ⚠ 写之前会**读一遍现有文件**，把那两格的**现值**保留下来（可见性是运营在后台按商业
 * 节奏改的活数据，不许被一次同步改掉）；新加的档默认 private。
 */
import fs from 'node:fs'
import path from 'node:path'

const SRC = 'D:\\MV项目\\aimv-studio\\modes'
const DST = 'D:\\work\\new-api\\modes'
const HOST_KEYS = ['x-visibility', 'x-summary']
const WRITE = process.argv.includes('--write')

/** 读一份模式文件里的两格宿主字段（没有就给兜底） */
function hostFieldsOf(file, fallbackSummary) {
  if (!fs.existsSync(file)) return { 'x-visibility': 'private', 'x-summary': fallbackSummary }
  const raw = fs.readFileSync(file, 'utf8')
  const out = {}
  for (const key of HOST_KEYS) {
    const hit = new RegExp(`"${key}"\\s*:\\s*"((?:[^"\\\\]|\\\\.)*)"`).exec(raw)
    out[key] = hit ? hit[1] : ''
  }
  if (!out['x-visibility']) out['x-visibility'] = 'private'
  if (!out['x-summary']) out['x-summary'] = fallbackSummary
  return out
}

/** 把两格宿主字段插在 `formatVersion` 之后，其余键序原样保留 */
function injectHostKeys(raw, host) {
  const lines = []
  let injected = false
  for (const line of raw.split('\n')) {
    lines.push(line)
    if (!injected && /^\s*"formatVersion"\s*:/.test(line)) {
      for (const key of HOST_KEYS) {
        lines.push(`  ${JSON.stringify(key)}: ${JSON.stringify(host[key])},`)
      }
      injected = true
    }
  }
  if (!injected) throw new Error('这一份里没有 formatVersion 那一行，插不进宿主字段')
  /* `formatVersion` 那一行自己带着逗号，插进去之后仍然合法 —— 但要确认它真有 */
  const text = lines.join('\n')
  JSON.parse(text) // 不合法就在这里当场炸，而不是写出去之后
  return text
}

const files = fs.readdirSync(SRC).filter((f) => f.endsWith('.json')).sort()
let changed = 0
let added = 0
for (const name of files) {
  const src = path.join(SRC, name)
  const dst = path.join(DST, name)
  const raw = fs.readFileSync(src, 'utf8')
  const manifest = JSON.parse(raw)
  const host = hostFieldsOf(dst, `${manifest.label}：${manifest.id}`)
  const next = injectHostKeys(raw, host)

  if (!fs.existsSync(dst)) {
    added++
    console.log(`+ ${name}  （新加，${host['x-visibility']}）`)
    if (WRITE) fs.writeFileSync(dst, next, 'utf8')
    continue
  }
  const current = fs.readFileSync(dst, 'utf8')
  if (current === next) {
    console.log(`= ${name}  已经一致`)
    continue
  }
  changed++
  console.log(`~ ${name}  ${current.length} -> ${next.length} 字节（${host['x-visibility']}）`)
  if (WRITE) fs.writeFileSync(dst, next, 'utf8')
}

/* 反向：目标目录里多出来的（源里已经没有的）也要报出来，不静默 */
for (const name of fs.readdirSync(DST).sort()) {
  if (!name.endsWith('.json') || name.endsWith('.bak')) continue
  if (!files.includes(name)) console.log(`! ${name}  只在 new-api 里有（源里没有这一档）`)
}

console.log(`\n${WRITE ? '已写入' : '干跑'}：改动 ${changed} 份、新增 ${added} 份、源共 ${files.length} 份`)
if (!WRITE) console.log('加 --write 才真的写。')
