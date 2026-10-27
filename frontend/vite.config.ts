import {defineConfig} from 'vite'
import {svelte} from '@sveltejs/vite-plugin-svelte'
import {readFileSync} from 'node:fs'

// La versión sale de wails.json (info.productVersion) para tener una sola fuente.
const version = JSON.parse(readFileSync(new URL('../wails.json', import.meta.url), 'utf8')).info.productVersion

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [svelte()],
  define: {__APP_VERSION__: JSON.stringify(version)}
})
