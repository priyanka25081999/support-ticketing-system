import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// Dev server proxies /api to the Go API, so the browser sees a single origin
// (no CORS issues and the refresh cookie works out of the box).
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5180,
    strictPort: true,
    proxy: { '/api': 'http://localhost:8080' },
  },
});
