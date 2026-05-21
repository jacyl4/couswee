import adapter from '@sveltejs/adapter-static';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter({
      pages: 'web/dist',
      assets: 'web/dist',
      fallback: 'fallback.html',
      strict: false
    }),
    files: {
      assets: 'web/static',
      lib: 'web/src/lib',
      routes: 'web/src/routes',
      appTemplate: 'web/src/app.html'
    }
  }
};

export default config;
