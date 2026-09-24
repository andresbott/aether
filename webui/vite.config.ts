import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { materialSymbols } from './build/material-symbols'

export default defineConfig({
  plugins: [vue(), materialSymbols({ srcDir: fileURLToPath(new URL('./src', import.meta.url)) })],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  base: '/',
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8075',
        changeOrigin: true,
        secure: false,
        cookieDomainRewrite: { '*': '' }
      },
      '/rest': {
        target: 'http://localhost:8075',
        changeOrigin: true,
        secure: false
      }
    }
  }
})
