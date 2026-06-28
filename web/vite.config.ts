import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// 构建产物直接输出到 Go 内嵌目录，由 go:embed 打进单一二进制。
export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: {
    outDir: '../internal/api/admin/webui/dist',
    // 不清空目录：保留被 git 跟踪的 .gitkeep（go:embed 的锚点），
    // 构建产物自身被 .gitignore 忽略。
    emptyOutDir: false,
  },
  server: {
    proxy: {
      // 开发期把 /api 代理到本机 admin 监听端口（默认 3001）。
      '/api': 'http://127.0.0.1:3001',
    },
  },
})
