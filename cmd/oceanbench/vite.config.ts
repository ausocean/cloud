import { defineConfig } from "vite";
import fs from "node:fs";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig(() => {
  const input = fs
    .readdirSync("ts")
    .filter((file) => file.endsWith(".ts"))
    .map((file) => `ts/${file}`);
  return {
    root: "./",
    base: "/s/",
    plugins: [tailwindcss()],
    build: {
      outDir: "s/lit",
      rollupOptions: {
        input: input,
        output: {
          entryFileNames: "[name].js",
          chunkFileNames: "[name].js",
          assetFileNames: "[name].[ext]",
        },
      },
    },
  };
});
