import { ConfigProvider, App as AntApp } from 'antd'
import zhCN from 'antd/locale/zh_CN'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { buildTheme, type ThemeMode } from './tokens'
import { resolveMode, systemPrefersDark, useThemeStore } from '../store/theme'

/**
 * 全局主题提供者。
 *
 * 职责：
 *  1. 把用户偏好（明亮/暗黑/跟随系统）解析为实际模式；
 *  2. 监听系统配色变化，实现"跟随系统"的自适应；
 *  3. 把模式写到 <html data-theme> 上，驱动 CSS 变量；
 *  4. 用对应令牌配置 antd ConfigProvider。
 */
export function ThemeProvider({ children }: { children: ReactNode }) {
  const preference = useThemeStore((s) => s.preference)
  const [systemDark, setSystemDark] = useState(systemPrefersDark())

  // 监听系统配色变化
  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)')
    const handler = (e: MediaQueryListEvent) => setSystemDark(e.matches)
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [])

  const mode: ThemeMode = resolveMode(preference, systemDark)

  // 把模式同步到 <html>，驱动全局 CSS 变量与滚动条等
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', mode)
    document.documentElement.style.colorScheme = mode
  }, [mode])

  const themeConfig = useMemo(() => buildTheme(mode), [mode])

  return (
    <ConfigProvider locale={zhCN} theme={themeConfig}>
      <AntApp>{children}</AntApp>
    </ConfigProvider>
  )
}
