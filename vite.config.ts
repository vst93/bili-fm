import { defineConfig, type Plugin } from 'vite'
import react from '@vitejs/plugin-react'
import tsconfigPaths from 'vite-tsconfig-paths'

// 移除构建产物中的 `crossorigin` 属性 (script/link 标签)。
//
// 背景: webkit2gtk (Linux Tauri) 在加载 `<script type="module" crossorigin>` 时会以
// CORS 模式请求 tauri://localhost 自定义协议上的资源, 而 webkit2gtk 的资源加载器
// 在该模式下存在已知的竞态问题 (多个并发请求可能被混淆, 导致
// "TypeError: 'image/svg+xml' is not a valid JavaScript MIME type" 之类错误,
// 表现为窗口白屏、main.tsx 未执行 — 参见 tauri-apps/tauri#13074)。
//
// 本应用所有资源均为同源 (tauri://localhost), 不需要 crossorigin;
// 移除后模块以 no-cors / same-origin 方式加载, 绕开该问题。
const stripCrossorigin = (): Plugin => ({
  name: 'strip-crossorigin',
  transformIndexHtml(html) {
    return html.replace(/\s*crossorigin(="[^"]*")?/g, '')
  },
})

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tsconfigPaths(), stripCrossorigin()],
  server: {
    host: '127.0.0.1',
    port: 5173,
    strictPort: true,
  },
  build: {
    target: 'es2019',
    minify: 'esbuild',
    // 不生成 <link rel="modulepreload"> 预加载链接:
    // 1) webkit2gtk 对 modulepreload + 自定义协议的处理曾多次出现兼容问题;
    // 2) 模块本来就会在 import 时按需加载, 预加载链接并非必需。
    // 这同时避免了与入口模块并发竞速的额外请求 (白屏问题的一类触发源)。
    modulePreload: false,
    rollupOptions: {
      output: {
        manualChunks: {
          tauri: ['@tauri-apps/api', '@tauri-apps/plugin-shell', '@tauri-apps/plugin-updater', '@tauri-apps/plugin-process'],
          heroui: ['@heroui/react'],
          icons: ['@icon-park/react'],
        },
      },
    },
  },
})
