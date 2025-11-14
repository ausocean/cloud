import { defineConfig } from "vite";
import { resolve } from "path";

export default defineConfig({
  build: {
    manifest: true,
    rollupOptions: {
      input: {
        "web-components/cron-settings": resolve(__dirname, "s/web-components/cron-settings.ts"),
        "web-components/header-group": resolve(__dirname, "s/web-components/header-group.ts"),
        "web-components/nav-menu": resolve(__dirname, "s/web-components/nav-menu.ts"),
        "web-components/site-menu": resolve(__dirname, "s/web-components/site-menu.ts"),
      },
      output: {
        entryFileNames: "[name].js",
        dir: "dist",
      },
    },
    emptyOutDir: true,
  },

  server: {
    // Proxy all Go endpoints
    proxy: {
      "/search": { target: "http://localhost:8080", changeOrigin: true },
      "/play": { target: "http://localhost:8080", changeOrigin: true },
      "/learn/mooring": { target: "http://localhost:8080", changeOrigin: true },
      "/upload": { target: "http://localhost:8080", changeOrigin: true },
      "/set/devices": { target: "http://localhost:8080", changeOrigin: true },
      "/set/crons": { target: "http://localhost:8080", changeOrigin: true },
      "/get": { target: "http://localhost:8080", changeOrigin: true },
      "/test": { target: "http://localhost:8080", changeOrigin: true },
      "/login": { target: "http://localhost:8080", changeOrigin: true },
      "/logout": { target: "http://localhost:8080", changeOrigin: true },
      "/oauth2callback": { target: "http://localhost:8080", changeOrigin: true },
      "/live": { target: "http://localhost:8080", changeOrigin: true },
      "/monitor": { target: "http://localhost:8080", changeOrigin: true },
      "/play/audiorequest": { target: "http://localhost:8080", changeOrigin: true },
      "/admin": { target: "http://localhost:8080", changeOrigin: true },
      "/data": { target: "http://localhost:8080", changeOrigin: true },
      "/throughputs": { target: "http://localhost:8080", changeOrigin: true },
    },
  },
});
