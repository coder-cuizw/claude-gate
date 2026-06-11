import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// claude-gate 管理面板构建配置
export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 5173,
    // 开发时把后端接口代理到网关（默认 :8791）。用精确前缀 /api/admin 与 /v1，
    // 避免与前端路由 /api-keys 冲突。可用 CG_GATEWAY 覆盖目标地址。
    proxy: {
      '/api/admin': { target: process.env.CG_GATEWAY || 'http://localhost:8791', changeOrigin: true },
      '/v1': { target: process.env.CG_GATEWAY || 'http://localhost:8791', changeOrigin: true },
    },
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
