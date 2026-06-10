import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import type { ThemeMode } from '../theme/tokens'

/** 用户可选的主题偏好：明亮 / 暗黑 / 跟随系统。 */
export type ThemePreference = 'light' | 'dark' | 'system'

interface ThemeState {
  /** 用户偏好（持久化）。 */
  preference: ThemePreference
  /** 设置偏好。 */
  setPreference: (p: ThemePreference) => void
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      preference: 'system',
      setPreference: (p) => set({ preference: p }),
    }),
    { name: 'cg-theme' },
  ),
)

/** 读取系统是否处于暗色。 */
export function systemPrefersDark(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-color-scheme: dark)').matches
}

/** 把偏好解析为实际生效的明暗模式。 */
export function resolveMode(pref: ThemePreference, systemDark: boolean): ThemeMode {
  if (pref === 'system') return systemDark ? 'dark' : 'light'
  return pref
}
