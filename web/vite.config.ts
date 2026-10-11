import { defineConfig } from "vite";
import { svelte } from "@sveltejs/vite-plugin-svelte";
import tailwindcss from "@tailwindcss/vite";

// Built into internal/ui/dist/app and embedded in the Go binary (ADR 0006).
// `npm run dev` proxies the API to a quipu started with `quipu ui --port 47801`.
export default defineConfig({
  plugins: [svelte(), tailwindcss()],
  build: {
    outDir: "../internal/ui/dist/app",
    emptyOutDir: true,
    assetsInlineLimit: 0,
  },
  server: {
    proxy: { "/api": "http://127.0.0.1:47801" },
  },
});
