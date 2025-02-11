import { html } from "lit";
import { customElement } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";

@customElement("free-banner")
export class FreeBanner extends TailwindElement() {
  render() {
    return html`
      <div id="promo-banner" class="fixed left-4 top-16 z-50 max-w-64 rounded-lg bg-blue-700 p-4 shadow-lg transition-transform md:top-24">
        <p class="text-sm font-semibold text-white md:text-base">☀️ Free for a Limited Time!</p>
        <button class="mt-2 text-sm text-gray-300 hover:underline" @click="${this.toggleInfo}">Learn More</button>

        <div id="promo-info" class="mt-2 hidden">
          <p class="text-sm text-white">
            AusOcean is a non-profit dedicated to developing innovative tech to help our oceans.
            <br />
            AusOceanTV is an effort to combat ocean blindness and collect useful ocean data. To help sustain AusOcean's mission, AusOceanTV will soon be moving towards a paid subscription model.
          </p>
          <a href="https://www.ausocean.org/support" target="_blank" class="mt-2 block max-w-64 rounded bg-blue-600 py-2 text-center text-sm text-white hover:brightness-110">Support Our Mission</a>
        </div>
      </div>
    `;
  }

  toggleInfo() {
    const promoInfo = this.shadowRoot?.querySelector("#promo-info");
    promoInfo!.classList.toggle("hidden");
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "free-banner": FreeBanner;
  }
}
