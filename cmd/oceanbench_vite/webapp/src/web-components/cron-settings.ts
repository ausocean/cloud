import { LitElement, html, css } from "lit";
import { customElement, property, state } from "lit/decorators.js";

interface SiteVar {
  Skey: number;
  Scope: string;
  Name: string;
  Value: string;
  Updated: string;
}

interface SiteDevice {
  Skey: number;
  Dkey: number;
  Mac: number;
  Name: string;
  Inputs: string;
  Outputs: string;
  Wifi: string;
  MonitorPeriod: number;
  ActPeriod: number;
  Type: string;
  Version: string;
  Protocol: string;
  Status: number;
  Latitude: number;
  Longitude: number;
  Enabled: boolean;
  Updated: string;
}

const prodEndpoint = "https://oceantv.appspot.com/checkbroadcasts";
const devEndpoint = "https://dev-dot-oceantv.ts.r.appspot.com/checkbroadcasts";

@customElement("cron-settings")
export class CronSettings extends LitElement {
  @property({ type: String, attribute: "id" }) ID = "";
  @property({ type: String, attribute: "time" }) Time = "";
  @property({ type: String, attribute: "action" }) Action = "";
  @property({ type: String, attribute: "var" }) Variable = "";
  @property({ type: String, attribute: "value" }) Value = "";
  @property({ type: Boolean, attribute: "enabled" }) Enabled = false;
  @property({ type: Boolean, attribute: "new-cron" }) newCron = false;

  @state() buttonText = "Save";
  @state() dropdownOption = "";

  siteVars: SiteVar[] = [];
  devMap: Map<string, SiteDevice> = new Map();

  static styles = css`
    .row {
        width: 100%;
      display: inline-grid;
      grid-template-columns: 8% 20% 8% 8% 20% 16% 10%;
      gap: 10px;
    }

    @keyframes pulse {
          0% {
            opacity: 1;
          }
          50% {
            opacity: 0.5;
          }
          100% {
            opacity: 1;
          }
  `;

  async connectedCallback() {
    super.connectedCallback();
    this.getVars();
    this.getDevices();
  }

  getVars() {
    fetch("/api/get/vars/site")
      .then(async (resp) => {
        if (resp.ok) {
          return resp.json();
        }
        throw await resp.text();
      })
      .then((data) => {
        this.siteVars = data;
        this.requestUpdate();
      })
      .catch((err) => {
        console.error(err);
      });
  }

  getDevices() {
    fetch("/api/get/devices/site")
      .then(async (resp) => {
        if (resp.ok) {
          return resp.json();
        }
        throw await resp.text();
      })
      .then((data) => {
        this.devMap = new Map(data.map((d: SiteDevice) => [d.Mac.toString(16), d]));
        this.requestUpdate();
      })
      .catch((err) => {
        console.error(err);
      });
  }

  override render() {
    return html`
      <div class="row">
        <button @click="${this.submitCron}">${this.buttonText}</button>
        <input @change="${this.updateID}" type="text" value="${this.ID}" />
        <input @change="${this.updateTime}" type="text" value="${this.Time}" />
        <select @input="${this.updateAction}">
          <option ?selected="${this.Action == "set"}">set</option>
          <option ?selected="${this.Action == "del"}">del</option>
          <option ?selected="${this.Action == "call"}">call</option>
          <option ?selected="${this.Action == "rpc"}">rpc</option>
          <option ?selected="${this.Action == "email"}">email</option>
        </select>
        ${this.varDropdown()}
        <input @change="${this.updateValue}" type="text" value="${this.Value}" />
        <input @change="${this.updateEnabled}" type="checkbox" ?checked=${this.Enabled} style="max-height: 16px;" />
      </div>
    `;
  }

  submitCron() {
    let formData = new FormData();
    formData.append("ci", this.ID);
    formData.append("ct", this.Time);
    formData.append("ca", this.Action);
    formData.append("cv", this.Variable);
    formData.append("cd", this.Value);
    formData.append("ce", this.Enabled ? "true" : "");

    // Create a new cron input.
    if (this.newCron) {
      let nextCron = new CronSettings();
      nextCron.newCron = true;
      this.parentElement?.appendChild(nextCron);
      this.newCron = false;
    }

    fetch("/set/crons/edit", { method: "POST", body: formData })
      .then((resp) => {
        if (resp.ok) {
          this.buttonText = "Saved!";
          this.requestUpdate();
          setTimeout(() => {
            this.buttonText = "Save";
            this.requestUpdate();
          }, 1000);
        }
      })
      .catch((err) => {
        console.log("Got error:", err);
      });
  }

  updateID(e: Event) {
    let idInput = e.target as HTMLInputElement;
    this.ID = idInput.value;
    this.requestUpdate();
  }

  updateAction(e: Event) {
    let actionSelect = e.target as HTMLSelectElement;
    this.Action = actionSelect.options[actionSelect.selectedIndex].text;
    this.requestUpdate();
  }

  updateTime(e: Event) {
    let timeInput = e.target as HTMLInputElement;
    this.Time = timeInput.value;
    this.requestUpdate();
  }

  updateValue(e: Event) {
    let valueInput = e.target as HTMLInputElement;
    this.Value = valueInput.value;
    this.requestUpdate();
  }

  updateEnabled(e: Event) {
    let enabledInput = e.target as HTMLInputElement;
    this.Enabled = enabledInput.checked;
    this.requestUpdate();
  }

  varDropdown() {
    switch (this.Action) {
      case "rpc":
        console.log("variable:", this.Variable);
        return html`
          <div>
            <select @change="${this.updateEndpoint}" style="width: 100%">
              <option value="other">other</option>
              <option value="https://oceantv.appspot.com/checkbroadcasts" ?selected="${this.Variable == prodEndpoint}">Production</option>
              <option value="https://dev-dot-oceantv.ts.r.appspot.com/checkbroadcasts" ?selected="${this.Variable == devEndpoint}">Testing (Dev)</option>
            </select>
            <input @change="${this.updateEndpoint}" type="text" id="endpoint-input" value="${this.Variable}" />
          </div>
        `;
      case "set":
        if (this.devMap.size == 0 || this.siteVars.length == 0) {
          return html`
            <input type="text" style="animation: pulse 1s infinite;" readonly value="loading..." />
          `;
        }
        console.log("devmap:", this.devMap);
        return html`
          <select @change="${this.updateVariable}">
            <option value="">-- Select a Variable --</option>
            ${this.siteVars.map((v) => {
              let parts = v.Name.split(".");
              let dev = this.devMap.get(parts[0]);

              return html`
                <option value="${v.Name}" ?selected="${this.Variable === v.Name}">${dev?.Name + "." + parts[1]}</option>
              `;
            })}
          </select>
        `;
      default:
        return html`
          <input type="text" .value="${this.Variable}" />
        `;
    }
  }

  updateEndpoint(e: Event) {
    const select = e.target as HTMLSelectElement;
    this.Variable = select.value;
    if (this.Variable === "other") {
      let input = this.shadowRoot?.querySelector("#other-input") as HTMLInputElement;
      if (!input) {
        return;
      }
      this.Variable = input.value;
    }
    this.requestUpdate();
  }

  updateVariable(e: Event) {
    const select = e.target as HTMLSelectElement;
    this.Variable = select.value;
    this.requestUpdate();
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "cron-settings": CronSettings;
  }
}
