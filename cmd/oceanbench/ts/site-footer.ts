// ts/site-footer.ts
import { LitElement, html } from "lit";
import { customElement, property } from "lit/decorators.js";

@customElement("site-footer")
export class SiteFooter extends LitElement {
  @property({ type: Number }) startYear = 2019;
  @property({ type: String }) orgName = "Australian Ocean Laboratory Limited (AusOcean)";
  @property({ type: String }) licenseHref = "https://www.ausocean.org/license";

  // Render into light DOM so global Tailwind classes apply.
  protected createRenderRoot() {
    return this;
  }

  private yearRange(): string {
    const y = new Date().getFullYear();
    return this.startYear === y ? `${y}` : `${this.startYear}–${y}`;
  }

  override render() {
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
