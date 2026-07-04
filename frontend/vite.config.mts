import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'path'

export default defineConfig({
  plugins: [
    react(),
  ],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    chunkSizeWarningLimit: 1200,
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return
          const normalized = id.replace(/\\/g, '/')
          if (
            normalized.includes('/node_modules/antd/') ||
            normalized.includes('/node_modules/@ant-design/') ||
            normalized.includes('/node_modules/rc-')
          ) return 'antd'
          if (
            normalized.includes('/node_modules/@emotion/') ||
            normalized.includes('/node_modules/dayjs/') ||
            normalized.includes('/node_modules/classnames/') ||
            normalized.includes('/node_modules/copy-to-clipboard/')
          ) return 'ui-vendor'
          if (
            normalized.includes('/node_modules/recharts/') ||
            normalized.includes('/node_modules/d3-') ||
            normalized.includes('/node_modules/react-smooth/') ||
            normalized.includes('/node_modules/victory-vendor/')
          ) return 'charts'
          if (
            normalized.includes('/node_modules/react-router/') ||
            normalized.includes('/node_modules/react-router-dom/') ||
            normalized.includes('/node_modules/@remix-run/router/')
          ) return 'router'
          if (
            normalized.includes('/node_modules/react/') ||
            normalized.includes('/node_modules/react-dom/') ||
            normalized.includes('/node_modules/scheduler/')
          ) return 'react-core'
          if (normalized.includes('/node_modules/axios/')) return 'network'
        },
      },
    },
  },
  server: {
    host: '0.0.0.0',
    port: 3000,
    allowedHosts: 'all',
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:8080',
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: './src/test/setup.ts',
  },
})
