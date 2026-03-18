// This follows https://github.com/butopen/web-components-tailwind-starter-kit —
// the compiled tailwind.global.css is imported as a plain string at build time by
// rollup-plugin-string (equivalent to ?inline syntax), then passed into
// Lit's static styles so the browser uses efficient Constructable Stylesheets.
// The build:css step must run before Rollup so the compiled CSS is available.

import { LitElement, unsafeCSS } from "lit";
import globalStyles from "../../s/dist/tailwind.global.css";

const tailwindStyles = unsafeCSS(globalStyles);

// A common workaround is to link the stylesheet inside the shadow DOM:
export const TailwindElement = (style?: string) =>
  class extends LitElement {
    static styles = style
      ? [tailwindStyles, unsafeCSS(style)]
      : [tailwindStyles];
  };
