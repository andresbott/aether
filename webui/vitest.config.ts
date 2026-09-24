import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { fileURLToPath, URL } from 'node:url'
import { materialSymbols } from './build/material-symbols'

export default defineConfig({
  plugins: [vue(), materialSymbols({ srcDir: fileURLToPath(new URL('./src', import.meta.url)) })],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    }
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test-setup.ts'],
    poolOptions: {
      forks: {
        maxForks: 4
      }
    }
  }
})
