// /s/lit/site-footer.ts
import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "shared/tailwind.element.ts";

/**
 * Site footer component.
 * Uses Tailwind for styling. Keep comments ending with full stops.
 */
@customElement("site-footer")
export class SiteFooter extends TailwindElement() {
  /** First year to display in the © range. */
  @property({ type: Number }) startYear = 2019;

  /** Organisation display name. */
  @property({ type: String }) orgName = "Australian Ocean Laboratory Limited (AusOcean)";

  /** License URL. */
  @property({ type: String }) licenseHref = "https://www.ausocean.org/license";

  private yearRange(): string {
    const thisYear = new Date().getFullYear();
    return this.startYear === thisYear ? `${thisYear}` : `${this.startYear}–${thisYear}`;
  }

  render() {
    return html`
      <footer class="w-full border-t border-neutral-200 bg-neutral-50/80 backdrop-blur-sm">
        <div class="mx-auto max-w-6xl px-4 py-6 text-center text-sm text-neutral-700">
          <p>
            &copy;${this.yearRange()} ${this.orgName} (
            <a rel="license" class="underline hover:no-underline" href="${this.licenseHref}">License</a>
            )
          </p>
          <slot name="extra"></slot>
        </div>
      </footer>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "site-footer": SiteFooter;
  }
}
