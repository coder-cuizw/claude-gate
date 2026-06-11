// 真实后端 HTTP 客户端：统一注入 JWT、处理 401 与错误信封。
//
// 同源部署（网关托管 dist）时 BASE 为空；开发时由 Vite proxy 把 /api 与 /v1
// 转发到网关（见 vite.config.ts）。可用 VITE_API_BASE 覆盖。

import { useAuthStore } from '../store/auth'

// 同源部署：网关托管 dist 时接口同源；开发时 Vite proxy 把 /api、/v1 转发到网关。
const BASE = ''

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const token = useAuthStore.getState().token
  const res = await fetch(BASE + path, {
    method,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })
  if (res.status === 401) {
    useAuthStore.getState().logout()
    throw new Error('未授权，请重新登录')
  }
  if (!res.ok) {
    let msg = `HTTP ${res.status}`
    try {
      const e = await res.json()
      msg = (e && (e.error || e.message)) || msg
    } catch {
      /* 忽略非 JSON 错误体 */
    }
    throw new Error(msg)
  }
  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}

export const http = {
  get: <T>(p: string) => req<T>('GET', p),
  post: <T>(p: string, b?: unknown) => req<T>('POST', p, b),
  put: <T>(p: string, b?: unknown) => req<T>('PUT', p, b),
  del: <T>(p: string) => req<T>('DELETE', p),
}

// 登录无需 token，单独封装。
export async function apiLogin(email: string, password: string) {
  const res = await fetch(BASE + '/api/admin/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  })
  if (!res.ok) throw new Error('邮箱或密码错误')
  return (await res.json()) as { token: string; user: { email: string; role: string } }
}
