// 用 Playwright 对所有页面截图（明亮 + 暗黑两套），输出到 docs/screenshots。
// 真实对接：先登录拿 JWT 注入 localStorage，并动态取一个真实 trace/group id。
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const OUT = resolve(__dirname, '../../docs/screenshots')
mkdirSync(OUT, { recursive: true })

const BASE = process.env.CG_BASE || 'http://localhost:8791'

// 真实登录拿 token
async function api(path, opts) {
  const res = await fetch(BASE + path, opts)
  if (!res.ok) throw new Error(`${path} -> ${res.status}`)
  return res.json()
}
const { token } = await api('/api/admin/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: 'admin@claude-gate.io', password: 'admin123' }),
})
const auth = { Authorization: `Bearer ${token}` }

// 动态取一个真实 trace id 与一个 formula/任意分组 id，保证详情/编辑页有数据
const traces = await api('/api/admin/traces?page_size=1', { headers: auth })
const sampleTrace = traces.items?.[0]?.trace_id || 'none'
const groups = await api('/api/admin/groups', { headers: auth })
const formulaGroup = groups.find((g) => g.cache_strategy?.type === 'formula') || groups[0]
const groupId = formulaGroup?.id || 1

const pages = [
  { path: '/login', name: 'login', auth: false, full: false },
  { path: '/dashboard', name: 'dashboard', auth: true, full: true },
  { path: '/traces', name: 'traces', auth: true, full: true },
  { path: `/traces/${sampleTrace}`, name: 'trace-detail', auth: true, full: true },
  { path: '/groups', name: 'groups', auth: true, full: true },
  { path: `/groups/${groupId}`, name: 'group-edit', auth: true, full: true },
  { path: '/channels', name: 'channels', auth: true, full: true },
  { path: '/api-keys', name: 'api-keys', auth: true, full: true },
  { path: '/settings', name: 'settings', auth: true, full: true },
]

const themes = ['light', 'dark']

function initScript(theme, needAuth) {
  const themeVal = JSON.stringify({ state: { preference: theme }, version: 0 })
  const authVal = needAuth
    ? JSON.stringify({ state: { token, user: { email: 'admin@claude-gate.io', role: 'admin' } }, version: 0 })
    : JSON.stringify({ state: { token: null, user: null }, version: 0 })
  return `
    try {
      localStorage.setItem('cg-theme', ${JSON.stringify(themeVal)});
      localStorage.setItem('cg-auth', ${JSON.stringify(authVal)});
    } catch (e) {}
  `
}

const browser = await chromium.launch()
let count = 0
for (const theme of themes) {
  for (const p of pages) {
    const context = await browser.newContext({
      viewport: { width: 1440, height: 900 },
      deviceScaleFactor: 2,
      colorScheme: theme,
    })
    await context.addInitScript(initScript(theme, p.auth))
    const page = await context.newPage()
    await page.goto(BASE + p.path, { waitUntil: 'networkidle' })
    await page.evaluate(() => document.fonts && document.fonts.ready)
    await page.waitForTimeout(1800)
    const file = resolve(OUT, `${p.name}-${theme}.png`)
    await page.screenshot({ path: file, fullPage: p.full })
    console.log('✓', `${p.name}-${theme}.png`)
    count++
    await context.close()
  }
}
await browser.close()
console.log(`完成，共 ${count} 张截图 → ${OUT}`)
