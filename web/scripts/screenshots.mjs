// 用 Playwright 对所有页面截图（明亮 + 暗黑两套），输出到 docs/screenshots。
// 通过预置 localStorage 注入登录态与主题偏好，免去交互。
import { chromium } from 'playwright'
import { mkdirSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

const __dirname = dirname(fileURLToPath(import.meta.url))
const OUT = resolve(__dirname, '../../docs/screenshots')
mkdirSync(OUT, { recursive: true })

const BASE = process.env.CG_BASE || 'http://localhost:4173'

// 页面清单：path 路由、name 文件名、auth 是否需要登录态、full 是否整页
const pages = [
  { path: '/login', name: 'login', auth: false, full: false },
  { path: '/dashboard', name: 'dashboard', auth: true, full: true },
  { path: '/traces', name: 'traces', auth: true, full: true },
  { path: '/traces/sample', name: 'trace-detail', auth: true, full: true },
  { path: '/groups', name: 'groups', auth: true, full: true },
  { path: '/groups/104', name: 'group-edit', auth: true, full: true },
  { path: '/channels', name: 'channels', auth: true, full: true },
  { path: '/api-keys', name: 'api-keys', auth: true, full: true },
  { path: '/settings', name: 'settings', auth: true, full: true },
]

const themes = ['light', 'dark']

function initScript(theme, auth) {
  // 预置 zustand persist 的 localStorage 结构
  const themeVal = JSON.stringify({ state: { preference: theme }, version: 0 })
  const authVal = auth
    ? JSON.stringify({ state: { token: 'demo-token', user: { email: 'admin@claude-gate.io', role: 'admin' } }, version: 0 })
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
    // 等字体与图表渲染
    await page.evaluate(() => document.fonts && document.fonts.ready)
    await page.waitForTimeout(1600)
    const file = resolve(OUT, `${p.name}-${theme}.png`)
    await page.screenshot({ path: file, fullPage: p.full })
    console.log('✓', `${p.name}-${theme}.png`)
    count++
    await context.close()
  }
}
await browser.close()
console.log(`完成，共 ${count} 张截图 → ${OUT}`)
