import { create } from 'zustand'
import { persist } from 'zustand/middleware'

interface User {
  email: string
  role: string
}

interface AuthState {
  token: string | null
  user: User | null
  login: (token: string, user: User) => void
  logout: () => void
}

/**
 * 鉴权状态（演示环境）。
 *
 * 真实环境下 token 来自管理后台登录接口签发的 JWT；这里持久化到 localStorage，
 * 演示模式下任意账号即可登录。
 */
export const useAuthStore = create<AuthState>()(
  persist(
    (set) => ({
      token: null,
      user: null,
      login: (token, user) => set({ token, user }),
      logout: () => set({ token: null, user: null }),
    }),
    { name: 'cg-auth' },
  ),
)
