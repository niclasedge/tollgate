import { defineConfig } from 'vitest/config';
import { svelte } from '@sveltejs/vite-plugin-svelte';

// Static build into web/dist, which is embedded by web/embed.go. The Makefile
// empties dist before the build so that the committed .gitkeep survives.
export default defineConfig({
  plugins: [svelte()],
  base: './',
  build: { outDir: 'dist', emptyOutDir: false, target: 'es2022' },
  // ../config/schema: tests load the schema the server embeds (filters.test.ts).
  server: { proxy: { '/api': 'http://127.0.0.1:8080' }, fs: { allow: ['.', '../config/schema'] } },
  resolve: { conditions: ['browser'] },
  test: {
    environment: 'jsdom',
    setupFiles: ['src/test-setup.ts'],
    include: ['src/**/*.test.ts'],
    // Its ESM build uses extensionless imports that only Vite's resolver handles.
    server: { deps: { inline: ['codemirror-json-schema'] } },
  },
});
