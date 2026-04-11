import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig({
  plugins: [vue()],
  base: './',
  build: {
    outDir: 'dist',
    assetsInlineLimit: 100000,
  },
  server: {
    proxy: {
      '/api': 'http://10.44.96.142:9090',
      '/healthz': 'http://10.44.96.142:9090',
      '/metrics': 'http://10.44.96.142:9090',
    }
  }
})
