import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tsconfigPaths from 'vite-tsconfig-paths'

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react(), tsconfigPaths()],
  build: {
    target: 'es2019',
    minify: 'esbuild',
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
