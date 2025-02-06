import { html } from "lit";
import { customElement } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";

@customElement("app-header")
export class AppHeader extends TailwindElement() {
  connectedCallback() {
    super.connectedCallback();
    if (
      !document.querySelector(
        'link[href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css"]',
      )
    ) {
      const link = document.createElement("link");
      link.rel = "stylesheet";
      link.href =
        "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css";
      document.head.appendChild(link);
    }
  }

  render() {
    return html`
      <link
        rel="stylesheet"
        href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css"
      />
      <div
        class="sticky top-0 z-10 flex items-center justify-between bg-neutral-50 px-4 py-2 shadow-lg"
      >
        <a href="home.html" class="flex items-center justify-center">
          <img
            src="src/assets/blue_logo.png"
            class="w-12 pt-1 md:w-20"
            alt="AusOcean logo"
          />
        </a>
        <div class="flex items-center space-x-6">
          <a
            href="home.html"
            title="Home"
            class="text-2xl text-[#0c69ad] transition-all hover:text-[#1f617a]"
          >
            <i class="fas fa-home"></i>
          </a>
          <a
            href="/api/v1/auth/logout"
            title="Logout"
            class="text-2xl text-[#0c69ad] transition-all hover:text-[#1f617a]"
          >
            <i class="fas fa-sign-out-alt"></i>
          </a>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "app-header": AppHeader;
  }
}
