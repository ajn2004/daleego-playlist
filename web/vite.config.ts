import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 8091,
    proxy: {
      '/api': 'http://localhost:8090',
      '/healthz': 'http://localhost:8090',
    }
  }
})