import path from "path"
import tailwindcss from "@tailwindcss/vite"
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react-swc'

// https://vite.dev/config/
export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(import.meta.dirname, "./src"),
    },
  },
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/api/, '/api'),
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (/node_modules\/(echarts|zrender)\//.test(id)) return "charts"
          if (/node_modules\/(react|react-dom|react-router|@tanstack\/react-query)\//.test(id)) return "ui-vendor"
          if (/node_modules\/(react-hook-form|zod|@hookform\/resolvers)\//.test(id)) return "forms"
        },
      },
    },
  },
})
