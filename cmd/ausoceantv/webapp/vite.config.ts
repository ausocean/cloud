import { defineConfig } from "vite";
import { globSync } from "glob";
import path from "node:path";
import { fileURLToPath } from "node:url";
import fs from "node:fs";

const input = Object.fromEntries(globSync("{policies/*,*}.html").map((file) => [file.slice(0, file.length - path.extname(file).length), fileURLToPath(new URL(file, import.meta.url))]));

// Read API key from config file pointed to by env var.
let googleMapsApiKey = "";
const configPath = process.env.AUSOCEANTV_MAPS_API_KEY;
if (configPath) {
  try {
    const configJson = fs.readFileSync(configPath, "utf8");
    const parsed = JSON.parse(configJson);
    googleMapsApiKey = parsed.google_maps_api_key || "";
  } catch (err) {
    console.warn(`Could not load config from ${configPath}:`, err);
  }
} else {
  console.warn("AUSOCEANTV_MAPS_API_KEY env variable not set.");
}

export default defineConfig({
  define: {
    __GOOGLE_MAPS_API_KEY__: JSON.stringify(googleMapsApiKey),
  },
  build: {
    rollupOptions: {
      input,
    },
  },
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:8084",
        changeOrigin: true,
      },
    },
  },
});
