import react from "@astrojs/react";
import tailwindcss from "@tailwindcss/vite";
import { defineConfig } from "astro/config";

// https://astro.build/config
export default defineConfig({
  // Static-render the dashboard. The Go server embeds web/dist/ via //go:embed.
  output: "static",
  outDir: "./dist",

  integrations: [react()],

  vite: {
    plugins: [tailwindcss()],
    server: {
      // Dev: proxy /api/* → Go server on :8080 so the Astro dev server
      // can call the same origin without CORS noise.
      proxy: {
        "/api": {
          target: "http://localhost:8080",
          changeOrigin: true,
        },
      },
    },
  },
});
