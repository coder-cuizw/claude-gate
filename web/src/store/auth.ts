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
 * 鉴权状态。token 来自管理后台 /api/admin/login 签发的 JWT，持久化到 localStorage。
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
