import { defineConfig } from 'vite';
import { configDefaults } from 'vitest/config';
import react from '@vitejs/plugin-react';
import fs from 'fs';
import path from 'path';
import type { ServerOptions } from 'https';

// Vite dev server will use these files if present. You can override via env vars.
const certDir = process.env.DEV_CERT_DIR || path.resolve(__dirname, 'certs');
const certFile = process.env.DEV_CERT_FILE || path.join(certDir, 'home.sensor-hub.pem');
const keyFile = process.env.DEV_KEY_FILE || path.join(certDir, 'home.sensor-hub-key.pem');

// Use ServerOptions | undefined so the option type lines up with Vite's expected ServerOptions | undefined.
let httpsOption: ServerOptions | undefined = undefined;
try {
  const cert = fs.readFileSync(certFile);
  const key = fs.readFileSync(keyFile);
  httpsOption = { key, cert } as ServerOptions;
  console.log(`Vite: using HTTPS certs ${certFile} and ${keyFile}`);
} catch {
  console.warn('Vite HTTPS: cert/key not found, falling back to HTTP');
}

// Proxy target for /docs (Go backend serves embedded Docusaurus).
// In Docker: http://sensor-hub:8080, locally: http://localhost:8080
const backendTarget = process.env.BACKEND_URL || 'http://localhost:8080';

export default defineConfig({
  plugins: [react()],
  server: {
    // cast to any here to satisfy TypeScript while preserving runtime behavior
    https: httpsOption as unknown as ServerOptions | undefined,
    host: true,
    port: Number(process.env.PORT) || 5173,
    proxy: {
      '/docs': {
        target: backendTarget,
        changeOrigin: true,
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/test/setup.ts',
    exclude: [...configDefaults.exclude, 'tests/**'],
  },
});
