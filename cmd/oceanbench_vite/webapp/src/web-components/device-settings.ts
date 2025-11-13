import { html } from "lit";
import { customElement } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.js";

@customElement("device-settings")
export class DeviceSettings extends TailwindElement() {
  connectedCallback() {
    super.connectedCallback();
  }

  render() {
    return html`
      <h1 class="text-red-500">hello</h1>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "device-settings": DeviceSettings;
  }
}
