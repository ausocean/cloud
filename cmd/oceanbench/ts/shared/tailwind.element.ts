// See Also: https://github.com/butopen/web-components-tailwind-starter-kit

import { LitElement, unsafeCSS } from "lit";

// Since Rollup isn't configured for CSS modules, we'll fetch the global stylesheet
// and inject it to all TailwindElements if needed, or rely on a standard approach.
// However, the best approach for standalone components without a bundler plugin
// is to define a constructable stylesheet from a shared string.
// As a temporary fix until the build system is updated, we inject an empty string
// and expect users to provide styles directly or rely on a global link tag 
// if rendering to light DOM. But for shadow DOM, we actually need the CSS text.

// We will fetch the CSS file locally during build if we use a plugin, but since we can't
// rely on plugins right now, let's use a dynamic fetch or just keep it simple.

// A common workaround is to link the stylesheet inside the shadow DOM:
export const TailwindElement = (style?: string) =>
  class extends LitElement {
    static styles = style ? [unsafeCSS(style)] : [];

    connectedCallback() {
      super.connectedCallback();
      // Inject global tailwind CSS into shadow DOM
      if (!this.shadowRoot?.querySelector('link[href="/s/dist/tailwind.global.css"]')) {
        const link = document.createElement("link");
        link.rel = "stylesheet";
        link.href = "/s/dist/tailwind.global.css";
        this.shadowRoot?.appendChild(link);
      }
    }
  };
