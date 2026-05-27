import { defineConfig } from 'vite'
import RubyPlugin from 'vite-plugin-ruby'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [
    RubyPlugin(),
    react(),
  ],
  server: {
    allowedHosts: true,
    cors: true,
  },
  legacy: {
    skipWebSocketTokenCheck: true,
  },
})
