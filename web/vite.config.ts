import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  envDir: '..',
  plugins: [react()],
  server: {
    port: 8091,
    strictPort: true,
    proxy: {
      '/api': 'http://localhost:8090',
      '/healthz': 'http://localhost:8090',
    }
  }
})
