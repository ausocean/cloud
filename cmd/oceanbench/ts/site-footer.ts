// ts/site-footer.ts
import { LitElement, html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";

@customElement("site-footer")
export class SiteFooter extends TailwindElement() {
  @property({ type: Number }) startYear = 2019;
  @property({ type: String }) orgName = "Australian Ocean Laboratory Limited (AusOcean)";
  @property({ type: String }) licenseHref = "https://www.ausocean.org/license";

  @property({ type: String }) version = "";
  @property({ type: String }) commit = "";
  @property({ type: Boolean }) superadmin = false;

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
          ${this.version ? html`
          <p class="opacity-50 text-xs mt-1">
            ${this.version}${this.commit ? ` (${this.commit})` : ""}
          </p>
          ` : ""}
          ${this.superadmin ? html`
          <div class="mt-4">
            <a href="/admin/tv-overview" class="inline-flex items-center px-3 py-1 bg-neutral-200 hover:bg-neutral-300 rounded text-xs font-medium transition-colors text-neutral-700 hover:text-neutral-900 no-underline">
              TV Overview
            </a>
          </div>
          ` : ""}
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
