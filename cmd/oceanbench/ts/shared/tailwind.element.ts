// TailwindElement is a Lit mixin that injects the compiled Tailwind CSS into each
// component's shadow root as an adopted stylesheet.
//
// This follows https://github.com/butopen/web-components-tailwind-starter-kit —
// the compiled tailwind.global.css is imported as a plain string at build time by
// rollup-plugin-string (equivalent to Vite's ?inline syntax), then passed into
// Lit's static styles so the browser uses efficient Constructable Stylesheets.
// The build:css step must run before Rollup so the compiled CSS is available.

import { LitElement, unsafeCSS } from "lit";
import globalStyles from "./tailwind.global.css";

const tailwindStyles = unsafeCSS(globalStyles);

export const TailwindElement = (style?: string) =>
  class extends LitElement {
    static styles = style
      ? [tailwindStyles, unsafeCSS(style)]
      : [tailwindStyles];
  };
