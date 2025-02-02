import { LitElement, html, css, HTMLTemplateResult, PropertyValues } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { consume } from "@lit/context";
import { devicesContext } from "./utils/devices-context";
import { Devices } from "./types/device";

@customElement("action-input")
export class ActionInput extends LitElement {
  static formAssociated = true;
  internals: ElementInternals;
  @state()
  value = "";
  @property({ attribute: "name" })
  name = "";

  @property({ attribute: "device-mac" })
  mac = "";
  @property({ attribute: "variable" })
  variable = "";
  @property({ attribute: "var-value" })
  varValue = "";

  constructor(mac: string = "", variable: string = "", varValue: string = "") {
    super();
    this.mac = mac;
    this.variable = variable;
    this.varValue = varValue;

    this.internals = this.attachInternals();
  }

  firstUpdated() {
    this.updateValue();
    console.log("Devices:", this.devices);
  }

  // Get the devices provided by the action-list parent element.
  @consume({ context: devicesContext, subscribe: true })
  devices: Devices | null = null;

  connectedCalllback() {
    console.log("connected!!!");
  }

  // Update the value to be read by the form.
  updateValue() {
    const action = this.shadowRoot?.getElementById("action");
    if (!action) {
      return;
    }

    // format action for form submission.
    const parts = action.children;
    const deviceSelect = parts[0] as HTMLSelectElement;
    const device = deviceSelect.options[deviceSelect.selectedIndex].value;
    const variable = (parts[1] as HTMLInputElement).value;
    const value = (parts[2] as HTMLInputElement).value;

    this.value = device + "." + variable + "=" + value;

    this.internals.setFormValue(this.value);
    this.dispatchEvent(new Event("action-value", { bubbles: true, composed: true }));
    console.log("[child] fired action-value event");
  }

  deviceSelect(): HTMLTemplateResult {
    if (!this.devices) {
      console.warn("devices is null for some reason.");
      return html``;
    }

    return html`
      <select @change=${this.updateValue}>
        <option>-- Select Device --</option>
        <optgroup label="Controllers">
          ${this.devices.Controllers.map(
            (dev) => html`
              <option value=${dev.MAC} ?selected=${this.mac == dev.MAC}>${dev.Name}</option>
            `,
          )}
        </optgroup>
        <optgroup label="Cameras">
          ${this.devices.Cameras.map(
            (dev) => html`
              <option value=${dev.MAC} ?selected=${this.mac == dev.MAC}>${dev.Name}</option>
            `,
          )}
        </optgroup>
      </select>
    `;
  }

  static styles = css`
    .row {
      display: flex;
      flex-direction: row;
    }
  `;

  render() {
    return html`
      <div class="row" id="action">
        ${this.deviceSelect()}
        <input @change=${this.updateValue} value=${this.variable} />
        <input @change=${this.updateValue} value=${this.varValue} />
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "action-input": ActionInput;
  }
}
