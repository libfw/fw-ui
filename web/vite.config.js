import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

// Build into the Go binary's embed dir; proxy /api to the daemon during dev.
export default defineConfig({
  plugins: [svelte()],
  build: { outDir: '../cmd/fw-ui/static', emptyOutDir: true },
  server: { proxy: { '/api': 'http://localhost:8849' } },
})
