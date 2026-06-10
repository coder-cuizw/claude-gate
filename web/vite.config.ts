import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// claude-gate 管理面板构建配置
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    // 演示模式下前端走内置 mock，无需后端代理。
    // 接入真实后端时，按需为 /api/admin 与 /v1 配置代理到网关（localhost:8080），
    // 注意用正则 '^/api/admin' 等精确前缀，避免与前端路由 /api-keys 冲突。
  },
  preview: {
    host: '0.0.0.0',
    port: 4173,
  },
  build: {
    outDir: 'dist',
    chunkSizeWarningLimit: 1600,
  },
})
