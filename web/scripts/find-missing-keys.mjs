import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')
const SRC_DIR = path.resolve('src')

const en = JSON.parse(await fs.readFile(path.join(LOCALES_DIR, 'en.json'), 'utf8'))
const enKeys = new Set(Object.keys(en.translation))

const tCallRegex = /\bt\(\s*['"`]([^'"`\n]+?)['"`]\s*[,)]/g

async function* walk(dir) {
  const entries = await fs.readdir(dir, { withFileTypes: true })
  for (const entry of entries) {
    const full = path.join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === 'locales' || entry.name === 'node_modules') continue
      yield* walk(full)
    } else if (/\.(tsx?|jsx?)$/.test(entry.name)) {
      yield full
    }
  }
}

const missing = new Map()
for await (const file of walk(SRC_DIR)) {
  const content = await fs.readFile(file, 'utf8')
  for (const match of content.matchAll(tCallRegex)) {
    const key = match[1]
    // Skip namespaced dotted keys (e.g. footer.columns.xxx) but keep English
    // sentence keys that merely contain periods.
    if (!/^[a-z0-9_-]+(\.[a-z0-9_-]+)+$/.test(key) && !enKeys.has(key)) {
      if (!missing.has(key)) missing.set(key, [])
      missing.get(key).push(path.relative(SRC_DIR, file))
    }
  }
}

const sorted = [...missing.entries()].sort((a, b) => a[0].localeCompare(b[0]))
for (const [key, files] of sorted) {
  console.log(JSON.stringify({ key, files: [...new Set(files)] }))
}
console.error(`Total missing: ${sorted.length}`)
