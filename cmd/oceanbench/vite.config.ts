import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  root: "./",
  base: "/s/",
  plugins: [tailwindcss()],
  build: {
    outDir: "s/lit",
    rollupOptions: {
      input: [
        "ts/broadcast-states.ts",
        "ts/site-menu.ts",
        "ts/nav-menu.ts",
        "ts/header-group.ts",
        "ts/cron-settings.ts",
        "ts/tv-overview.ts",
        "ts/site-footer.ts",
        "ts/admin-site-lists.ts",
        "ts/media-manager.ts",
        "ts/broadcast-states.ts",
      ],
      output: {
        entryFileNames: "[name].js",
        chunkFileNames: "[name].js",
        assetFileNames: "[name].[ext]",
      },
    },
  },
});
