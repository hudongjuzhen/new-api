/**
 * 修一份**落在客户端本机**、但类型不合契约的模式文件。
 *
 * # 它修的是什么
 *
 * `C:\Users\Administrator\AppData\Roaming\com.aimv.studio\modes\mode-1791873032.json`
 * 是 2026/10/14 用 new-api 的「AI 一键生成」造出来的那一档「短剧」。它读不出来，
 * 因为它把三格**必须是对象 / 字符串数组**的东西写成了别的形状：
 *
 * | 那一格 | 客户端要求 | 生成出来的 |
 * |---|---|---|
 * | `planDialog.instruction` | 对象，23 格 | **一个字符串**（那段「你是资深短剧编剧…」） |
 * | `planDialog.audio` | 对象，15 格 | **一个字符串** |
 * | `planDialog.templates` | 字符串数组 | **对象数组**（`{label, prompt}`） |
 *
 * 客户端对这三格的处置是**逐格回落**（`core/workModes.js` 的 `normalizeInstruction`
 * 与 `MODE_BASELINE`），于是整套规划提示词退回内置 `mv` 那一套 ——
 * 用户看到的是「请你作为顶级MV导演…」，而不是自己写的那一段。
 *
 * # 修法：**形状照内置短剧那一档，内容保留他写的**
 *
 * 它本来就是"短剧"这一档：`instruction` / `audio` / 词表都照
 * `aimv-studio/modes/short-drama.json`（那一份是客户端当前那一版的真身），
 * 再把用户自己写的东西**并**进去：
 *
 *	· 那段人设 → `instruction.ruleOneSubject`（整句原样）与 `skeleton` 的一句摘要
 *	· 五条模板 → 字符串，开场白用短剧那一句，正文是他写的 `prompt`
 *	· 那一句音频说明 → `audio.audioScriptLead`
 *	· `words` → 短剧的 28 格**打底**，他写的 13 个词覆盖同名的那些
 *	· `libraries` / `coreLabels` → 按契约归一（`fields[].key` 而不是 `id`）
 *
 * 用法：
 *   node _scripts/repair-local-custom-mode.mjs            # 干跑（写一个预览文件）
 *   node _scripts/repair-local-custom-mode.mjs --write    # 覆盖那一个文件（先留 .bak）
 */
import fs from 'node:fs'
import path from 'node:path'

const TARGET = 'C:\\Users\\Administrator\\AppData\\Roaming\\com.aimv.studio\\modes\\mode-1791873032.json'
const BASE = 'D:\\MV项目\\aimv-studio\\modes\\short-drama.json'
const PREVIEW = 'D:\\work\\new-api\\_scripts\\out\\mode-1791873032.repaired.json'
const WRITE = process.argv.includes('--write')

const base = JSON.parse(fs.readFileSync(BASE, 'utf8'))
const mine = JSON.parse(fs.readFileSync(TARGET, 'utf8'))

const out = structuredClone(base)

/* ① 身份：id 与 label 是他那一档的 */
out.id = mine.id
out.label = mine.label
out.medium = mine.medium || 'video'
out.workScale = mine.workScale || base.workScale
out.videoKind = 'audio' /* 音频生视频：这一档的驱动是「台词音频」 */
out.planDialog.impl = 'song'

/* ② 规划弹窗那两个名字是他起的（`规划剧本` / `规划你的短剧`） */
out.planDialog.label = mine.planDialog?.label || base.planDialog.label
out.planDialog.title = mine.planDialog?.title || base.planDialog.title

/* ③ 那段人设：整句进 `ruleOneSubject`（那一格讲的就是"写谁、怎么写"），
      并压一句摘要进 `skeleton`（骨架的正文里那一句说的是"这一集"） */
const persona = String(mine.planDialog?.instruction ?? '').trim()
if (persona) {
  out.planDialog.instruction.ruleOneSubject = persona
  out.planDialog.instruction.skeleton = out.planDialog.instruction.skeleton.replace(
    '目标视频是一段 {本片时长，秒，三位小数} 秒的短剧（这一集）：',
    '目标视频是一段 {本片时长，秒，三位小数} 秒的短剧（这一集，竖屏 9:16、每集 1–2 分钟）：'
  )
}

/* ④ 五条模板：对象 → **字符串**（开场白 = 短剧那一句，正文是他写的 prompt） */
const OPENERS = {
  opener: '请你作为短剧导演，结合这一集的台词与人物设定，参考图，来生成如下风格的视频脚本：',
}
const custom = Array.isArray(mine.planDialog?.templates) ? mine.planDialog.templates : []
out.planDialog.templates = [
  OPENERS.opener,
  ...custom
    .map((t) => {
      if (typeof t === 'string') return t.trim()
      const label = String(t?.label ?? '').trim()
      const prompt = String(t?.prompt ?? '').trim()
      if (!prompt) return ''
      return `${OPENERS.opener}\n${label}：${prompt}`
    })
    .filter(Boolean),
]
/* ⚠ 结尾那个全角冒号是形状守卫（`constants.test.js` 按它扫）——开场白那句自带 */

/* ⑤ 他写的那一句音频说明并进 `audioScriptLead` */
const audioNote = String(mine.planDialog?.audio ?? '').trim()
if (audioNote) {
  out.planDialog.audio.audioScriptLead = `${audioNote}\n（照下面的分节把这一集的音频脚本写全：音色表 + 每一句台词 + 音效点位。）`
}

/* ⑥ 词表：短剧那 28 格打底，他写的词覆盖同名格 */
out.words = { ...base.words, ...(mine.words || {}) }

/* ⑦ `requires`：他那一档要音频、要参考、要分段（那是短剧的真实需要） */
out.requires = { ...base.requires, ...(mine.requires || {}), audio: true, refs: true, segments: true }

/* ⑧ 资料库：契约要 `fields[].key`（他写的是 `id`），其余照他的 */
out.libraries = (Array.isArray(mine.libraries) ? mine.libraries : base.libraries).map((lib) => ({
  id: String(lib.id ?? '').trim(),
  label: String(lib.label ?? '').trim() || String(lib.id ?? '').trim(),
  dir: String(lib.dir ?? lib.id ?? '').trim(),
  itemFile: String(lib.itemFile ?? 'item.json').trim() || 'item.json',
  fields: (Array.isArray(lib.fields) ? lib.fields : [])
    .map((f) => ({
      key: String(f?.key ?? f?.id ?? '').trim(),
      label: String(f?.label ?? '').trim(),
      type: String(f?.type ?? 'text').trim(),
    }))
    .filter((f) => f.key && f.label),
}))

/* ⑨ `coreLabels` 归一：客户端只认 `audio` / `prompt` 两格 */
out.coreLabels = { ...base.coreLabels }
/* ⑩ `appRows` 那一格客户端已经退休（老文件里的按未识别字段忽略），不带出去 */

const text = JSON.stringify(out, null, 2) + '\n'

/* ---------------------------------------------------------------- 自查 */
const problems = []
const pd = out.planDialog
if (!pd.instruction || typeof pd.instruction !== 'object' || Array.isArray(pd.instruction)) {
  problems.push('instruction 不是对象')
} else {
  const missing = Object.keys(base.planDialog.instruction).filter((k) => !(k in pd.instruction))
  if (missing.length) problems.push(`instruction 少了 ${missing.join(', ')}`)
  for (const key of ['framing', 'songFacts']) {
    if (pd.instruction[key] && (typeof pd.instruction[key] !== 'object' || Array.isArray(pd.instruction[key]))) {
      problems.push(`instruction.${key} 不是对象`)
    }
  }
}
if (!Array.isArray(pd.templates) || !pd.templates.every((t) => typeof t === 'string' && t.trim())) {
  problems.push('templates 里有非字符串')
}
if (!pd.templates[0] || !pd.templates[0].endsWith('：')) problems.push('templates[0] 不是开场白（结尾没有全角冒号）')
if (!pd.audio || typeof pd.audio !== 'object' || Array.isArray(pd.audio)) problems.push('audio 不是对象')
else {
  const missing = Object.keys(base.planDialog.audio).filter((k) => !(k in pd.audio))
  if (missing.length) problems.push(`audio 少了 ${missing.join(', ')}`)
}
const wordKeys = Object.keys(base.words)
const missingWords = wordKeys.filter((k) => !(k in out.words))
if (missingWords.length) problems.push(`words 少了 ${missingWords.join(', ')}`)
for (const lib of out.libraries) {
  if (!lib.fields.length) problems.push(`资料库 ${lib.id} 一个字段都没有`)
  for (const f of lib.fields) if (!f.key) problems.push(`资料库 ${lib.id} 有字段没有 key`)
}
if (!['audio', 'frame'].includes(out.videoKind)) problems.push('videoKind 不是 audio / frame')
if (out.appRows) problems.push('appRows 没被去掉（客户端已退休这一格）')

fs.mkdirSync(path.dirname(PREVIEW), { recursive: true })
fs.writeFileSync(PREVIEW, text, 'utf8')
console.log(`预览写到 ${PREVIEW}（${Buffer.byteLength(text)} 字节）`)

console.log('\n=== 改了什么 ===')
console.log(`  label            ${JSON.stringify(mine.label)} → ${JSON.stringify(out.label)}`)
console.log(`  planDialog.label ${JSON.stringify(mine.planDialog?.label)} → ${JSON.stringify(out.planDialog.label)}`)
console.log(`  instruction      ${typeof mine.planDialog?.instruction} → 对象（${Object.keys(out.planDialog.instruction).length} 格）`)
console.log(`  audio            ${typeof mine.planDialog?.audio} → 对象（${Object.keys(out.planDialog.audio).length} 格）`)
console.log(`  templates        ${(mine.planDialog?.templates || []).length} 个对象 → ${out.planDialog.templates.length} 条字符串`)
console.log(`  words            ${Object.keys(mine.words || {}).length} 格 → ${Object.keys(out.words).length} 格`)
console.log(`  requires         ${JSON.stringify(mine.requires)} → ${JSON.stringify(out.requires)}`)
console.log(`  videoKind        ${JSON.stringify(mine.videoKind)} → ${JSON.stringify(out.videoKind)}`)
console.log(`  coreLabels       ${JSON.stringify(mine.coreLabels)} → ${JSON.stringify(out.coreLabels)}`)
console.log(`  core             ${JSON.stringify(mine.core)} → ${JSON.stringify(out.core)}（短剧那一档的口径）`)
console.log(`  appRows          ${Array.isArray(mine.appRows) ? `数组(${mine.appRows.length})` : typeof mine.appRows} → 去掉（客户端已退休）`)

if (problems.length) {
  console.log('\n✗ 自查没过：')
  for (const p of problems) console.log(`  · ${p}`)
  process.exit(1)
}
console.log('\n✓ 自查全过（形状与内置短剧那一档同形）')

if (WRITE) {
  const bak = `${TARGET}.bak-before-contract-fix`
  if (!fs.existsSync(bak)) fs.copyFileSync(TARGET, bak)
  fs.writeFileSync(TARGET, text, 'utf8')
  console.log(`\n已写入 ${TARGET}（原文件备份在 ${path.basename(bak)}）`)
} else {
  console.log('\n（干跑）加 --write 才覆盖那一个文件。')
}
