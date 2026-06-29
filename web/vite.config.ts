import path from 'node:path'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

// 构建产物直接输出到 Go 内嵌目录，由 go:embed 打进单一二进制。
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    // 与 tsconfig 的 paths 对齐：@ 指向 src，shadcn 组件依赖该别名。
    alias: { '@': path.resolve(__dirname, './src') },
  },
  build: {
    outDir: '../internal/api/webui/dist',
    // 不清空目录：保留被 git 跟踪的 .gitkeep（go:embed 的锚点），
    // 构建产物自身被 .gitignore 忽略。
    emptyOutDir: false,
    // 固定产物文件名（不带 hash）：产物内嵌进二进制，无需浏览器缓存失效。
    rollupOptions: {
      output: {
        entryFileNames: 'main.js',
        chunkFileNames: '[name].js',
        assetFileNames: 'main.[ext]',
      },
    },
  },
  server: {
    proxy: {
      // 开发期把 /api 代理到本机服务端口（默认 5678）。
      '/api': 'http://127.0.0.1:5678',
    },
  },
})
