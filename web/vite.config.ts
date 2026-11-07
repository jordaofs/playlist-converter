import { defineConfig, type Plugin } from 'vitest/config';
import react from '@vitejs/plugin-react';
import tailwindcss from '@tailwindcss/vite';
import { robots, seoHead, siteOrigin, sitemap } from './src/seo';

// Injects canonical metadata and writes robots.txt/sitemap.xml at build time.
function seo(): Plugin {
  const origin = siteOrigin(process.env.VITE_SITE_URL);
  return {
    name: 'playlist-converter-seo',
    transformIndexHtml: (html) =>
      html.replace('<!--seo-head-->', seoHead(origin)),
    generateBundle() {
      this.emitFile({
        type: 'asset',
        fileName: 'robots.txt',
        source: robots(origin),
      });
      const map = sitemap(origin);
      if (map)
        this.emitFile({ type: 'asset', fileName: 'sitemap.xml', source: map });
    },
  };
}

export default defineConfig({
  plugins: [react(), tailwindcss(), seo()],
  server: {
    proxy: { '/api': process.env.API_PROXY_TARGET ?? 'http://127.0.0.1:8080' },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}'],
    coverage: {
      provider: 'v8',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/main.tsx', 'src/lib/types.ts', 'src/test/**'],
      thresholds: { lines: 80, statements: 80, functions: 80, branches: 80 },
    },
  },
});
