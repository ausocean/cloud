import { defineConfig } from "vite";
import { globSync } from "glob";
import path from "node:path";
import { fileURLToPath } from "node:url";

const input = Object.fromEntries(globSync("*.html").map((file) => [file.slice(0, file.length - path.extname(file).length), fileURLToPath(new URL(file, import.meta.url))]));

export default defineConfig({
  build: {
    rollupOptions: {
      input,
    },
  },
  server: {
    fs: {
      strict: true,
      // Allow reading roadmap.config.json (and any future shared assets)
      // from the parent cmd/roadmap/ directory.
      allow: ["..", "."],
    },
    proxy: {
      "/api": {
        target: "http://localhost:8080",
        changeOrigin: true,
      },
    },
  },
});
