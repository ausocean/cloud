import { defineConfig } from "vite";

export default defineConfig({
  server: {
    proxy: {
      "/stripe": {
        target: "http://localhost:8084",
        changeOrigin: true,
      },
    },
  },
});
