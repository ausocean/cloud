import { html } from "lit";
import { customElement } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";

@customElement("custom-footer")
export class customFooter extends TailwindElement() {
  render() {
    return html`
      <footer class="z-10 w-full bg-[#0c69ad] p-5 text-white">
        <div class="m-auto grid max-w-5xl grid-cols-2 gap-y-5 text-sm">
          <ul class="mx-auto">
            <li class="font-semibold">Contact</li>
            <li><a class="opacity-65" href="mailto:info@ausocean.org">&emsp;info@ausocean.org</a></li>
            <li class="font-semibold">Socials</li>
            <li><a class="opacity-65" href="https://www.facebook.com/ausocean">&emsp;Facebook</a></li>
            <li><a class="opacity-65" href="https://www.instagram.com/ausocean/">&emsp;Instagram</a></li>
          </ul>
          <ul class="mx-auto">
            <li class="font-semibold">Policies</li>
            <li><a class="opacity-65" href="policies/privacy.html">&emsp;Privacy Policy</a></li>
            <li><a class="opacity-65" href="policies/terms-of-service.html">&emsp;Terms of Service</a></li>
          </ul>
          <p class="col-span-2 text-center text-sm">© 2025 AusOcean. All rights reserved.</p>
        </div>
      </footer>
    `;
  }
}
declare global {
  interface HTMLElementTagNameMap {
    "custom-footer ": customFooter;
  }
}
