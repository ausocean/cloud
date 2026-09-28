import { LitElement, html, css } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";

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
export class CronSettings extends TailwindElement() {
  @property({ type: String, attribute: "id" }) ID = "";
  @property({ type: String, attribute: "time" }) Time = "";
  @property({ type: String, attribute: "action" }) Action = "";
  @property({ type: String, attribute: "var" }) Variable = "";
  @property({ type: String, attribute: "value" }) Value = "";
  @property({ type: Boolean, attribute: "enabled" }) Enabled = false;
  @property({ type: Boolean, attribute: "new-cron" }) newCron = false;
  @property({ type: Number }) skey = 0;

  @state() saveButtonText = "Save";
  @state() deleteButtonText = "Delete";
  @state() dropdownOption = "";

  siteVars: SiteVar[] = [];
  devMap: Map<string, SiteDevice> = new Map();

  async connectedCallback() {
    super.connectedCallback();
    this.getVars();
    this.getDevices();
  }

  getVars() {
    fetch(`/api/get/${this.skey}/vars/site`)
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
    fetch(`/api/get/${this.skey}/devices/site`)
      .then(async (resp) => {
        if (resp.ok) {
          return resp.json();
        }
        throw await resp.text();
      })
      .then((data) => {
        this.devMap = new Map(
          data.map((d: SiteDevice) => [d.Mac.toString(16), d]),
        );
        this.requestUpdate();
      })
      .catch((err) => {
        console.error(err);
      });
  }

  override render() {
    return html`
      <div class="flex gap-1 flex-col mb-8">
        <div
          class="flex-col md:grid md:grid-cols-5 min-h-7 md:gap-x-2 md:gap-y-1 gap-1 flex"
        >
          <div class="flex gap-2 w-full">
            <input
              @change="${this.updateEnabled}"
              type="checkbox"
              ?checked=${this.Enabled}
              class="accent-primary"
            />
            <input
              @change="${this.updateID}"
              type="text"
              value="${this.ID}"
              class="font-mono font-black text-lg hover:bg-slate-200"
              placeholder="Cron Name"
            />
          </div>

          <div class="flex gap-2 col-span-2">
            <label class="w-20 shrink-0">Time:</label>
            <input
              @change="${this.updateTime}"
              type="text"
              value="${this.Time}"
              class="w-full border-solid border rounded-md border-slate-400 px-2"
            />
          </div>

          <div class="flex gap-2 col-span-2">
            <label class="w-20 shrink-0">Action:</label>
            <select
              @input="${this.updateAction}"
              class="border-solid border rounded-md border-slate-400 px-2 w-full"
            >
              <option ?selected="${this.Action == "set"}">set</option>
              <option ?selected="${this.Action == "del"}">del</option>
              <option ?selected="${this.Action == "call"}">call</option>
              <option ?selected="${this.Action == "rpc"}">rpc</option>
              <option ?selected="${this.Action == "email"}">email</option>
            </select>
            <button
              @click="${this.submitCron}"
              class="w-fit h-7 px-3 whitespace-nowrap bg-primary hover:bg-primary-hover text-white rounded-md hidden md:flex"
            >
              ${this.saveButtonText}
            </button>
            <button
              @click="${this.deleteCron}"
              class="w-fit h-7 px-3 whitespace-nowrap bg-red-700 hover:bg-red-800 text-white rounded-md hidden md:flex"
            >
              ${this.deleteButtonText}
            </button>
          </div>

          <div></div>

          <div class="flex gap-2 col-span-2">
            <label class="w-20 shrink-0">Variable:</label>
            ${this.varDropdown()}
          </div>

          <div class="flex gap-2 col-span-2">
            <label class="w-20 shrink-0">Value:</label>
            <input
              @change="${this.updateValue}"
              type="text"
              value="${this.Value}"
              class="w-full border-solid border rounded-md border-slate-400 px-2"
            />
          </div>
        </div>
        <div class="flex gap-2 justify-end w-full">
          <button
            @click="${this.submitCron}"
            class="w-fit h-7 px-3 whitespace-nowrap bg-primary hover:bg-primary-hover text-white rounded-md md:hidden flex"
          >
            ${this.saveButtonText}
          </button>
          <button
            @click="${this.deleteCron}"
            class="w-fit h-7 px-3 whitespace-nowrap bg-red-700 hover:bg-red-800 text-white rounded-md md:hidden flex"
          >
            ${this.deleteButtonText}
          </button>
        </div>
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

    fetch(`/${this.skey}/set/crons/edit`, { method: "POST", body: formData })
      .then((resp) => {
        if (resp.ok) {
          this.saveButtonText = "Saved!";
          this.requestUpdate();
          setTimeout(() => {
            this.saveButtonText = "Save";
            this.requestUpdate();
          }, 1000);
        }
      })
      .catch((err) => {
        console.log("Got error:", err);
      });
  }

  deleteCron() {
    let formData = new FormData();
    formData.append("ci", this.ID);
    formData.append("task", "Delete");

    this.deleteButtonText = "Deleting...";
    fetch(`/${this.skey}/set/crons/edit`, { method: "POST", body: formData })
      .then((resp) => {
        if (resp.ok) {
          this.remove();
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
          <div class="flex items-center gap-1 w-full min-h-7">
            <select
              @change="${this.updateEndpoint}"
              class="h-7 border-solid border rounded-md border-slate-400 px-1"
            >
              <option value="other">other</option>
              <option
                value="https://oceantv.appspot.com/checkbroadcasts"
                ?selected="${this.Variable == prodEndpoint}"
              >
                PROD
              </option>
              <option
                value="https://dev-dot-oceantv.ts.r.appspot.com/checkbroadcasts"
                ?selected="${this.Variable == devEndpoint}"
              >
                DEV
              </option>
            </select>
            <input
              @change="${this.updateEndpoint}"
              type="text"
              id="endpoint-input"
              value="${this.Variable}"
              class="h-7 border-solid border rounded-md border-slate-400 px-2 w-full"
            />
          </div>
        `;
      case "set":
        if (this.devMap.size == 0 || this.siteVars.length == 0) {
          return html`
            <input
              type="text"
              readonly
              value="loading..."
              class="w-full h-7 border-solid border rounded-md border-slate-400 animate-pulse px-2"
            />
          `;
        }
        console.log("devmap:", this.devMap);
        return html`
          <select
            @change="${this.updateVariable}"
            class="w-full h-7 border-solid border rounded-md border-slate-400 px-2"
          >
            <option value="">-- Select a Variable --</option>
            ${this.siteVars.map((v) => {
              let parts = v.Name.split(".");
              let dev = this.devMap.get(parts[0]);

              return html`
                <option
                  value="${v.Name}"
                  ?selected="${this.Variable === v.Name}"
                >
                  ${dev?.Name + "." + parts[1]}
                </option>
              `;
            })}
          </select>
        `;
      default:
        return html`
          <input
            type="text"
            .value="${this.Variable}"
            class="w-full min-w-0 h-7 basis-0 grow border-solid border rounded-md border-slate-400 px-2"
          />
        `;
    }
  }

  updateEndpoint(e: Event) {
    const select = e.target as HTMLSelectElement;
    this.Variable = select.value;
    if (this.Variable === "other") {
      let input = this.shadowRoot?.querySelector(
        "#other-input",
      ) as HTMLInputElement;
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
