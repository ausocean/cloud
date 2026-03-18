// TailwindElement is a Lit mixin that injects the compiled Tailwind CSS into each
// component's shadow root via a <link> tag.
//
// The original approach from https://github.com/butopen/web-components-tailwind-starter-kit
// inlines the CSS text at build time using Vite's CSS import feature. We use a link
// injection instead because our build system uses Rollup, which doesn't support
// CSS-as-string imports without a plugin.

import { LitElement, unsafeCSS } from "lit";

export const TailwindElement = (style?: string) =>
  class extends LitElement {
    static styles = style ? [unsafeCSS(style)] : [];

    connectedCallback() {
      super.connectedCallback();
      if (!this.shadowRoot?.querySelector('link[href="/s/dist/tailwind.global.css"]')) {
        const link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = "/s/dist/tailwind.global.css";
        this.shadowRoot?.appendChild(link);
      }
    }
  };
