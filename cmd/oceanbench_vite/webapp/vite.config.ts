import { defineConfig } from "vite";
import tailwindcss from "@tailwindcss/vite";
import { resolve } from "path";
import { globSync } from "glob";

const root = resolve(__dirname);

// Get all HTML files in the root, excluding 'footer.html'
const entryPoints = globSync(resolve(root, "*.html"))
  .filter((filePath) => !filePath.endsWith("footer.html"))
  .reduce((acc, filePath) => {
    const fileName = filePath.substring(filePath.lastIndexOf("/") + 1);
    // Use the filename without extension as the entry point name
    acc[fileName.replace(".html", "")] = filePath;
    return acc;
  }, {});

export default defineConfig({
  plugins: [tailwindcss()],
  build: {
    rollupOptions: {
      input: entryPoints,
    },
  },
});
