// Tells TypeScript that .css files can be imported as string modules.
// At build time, rollup-plugin-string replaces these imports with the file's raw text.
declare module "*.css" {
  const content: string;
  export default content;
}

// Tells TypeScript that .html files can be imported as raw string modules
// using Vite's ?raw suffix.
declare module "*.html?raw" {
  const content: string;
  export default content;
}
