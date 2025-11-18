import { html, TemplateResult } from "lit";
import { customElement, state } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";
import "./shared/tailwind.global.css";

type Device = {
  Skey: Number;
  Dkey: Number;
  Mac: Number;
  Name: string;
  Inputs: string;
  Outputs: string;
  Wifi: string;
  MonitorPeriod: Number;
  ActPeriod: Number;
  Type: string;
  Version: string;
  Protocol: string;
  Status: Number;
  Latitude: Number;
  Longitude: Number;
  Enabled: boolean;
  Updated: Date;
  Other: Map<string, string>;
};

@customElement("device-settings")
export class DeviceSettings extends TailwindElement() {
  @state() devices: Map<string, Device> = new Map();
  @state() selectedMAC: string = "0";

  async connectedCallback() {
    super.connectedCallback();
    this.getDevices();
  }

  render() {
    console.log("rendering");
    return html`
      <div class="flex bg-white rounded-sm border border-solid border-gray-300">${this.deviceSelect()}${this.deviceConfiguration()}</div>
    `;
  }

  deviceSelect(): TemplateResult {
    return html`
      <div class="flex">
        <div class="flex flex-col border-solid border-r border-gray-300 gap-1 p-4 text-end">
          <a @click=${this.selectDevice} data-mac=${0} class="${this.selectedMAC == "0" ? "text-slate-800" : "text-slate-600"} text-nowrap font-bold cursor-pointer">New Device</a>
          ${[...this.devices.values()].map(
            (d) => html`
              <a @click=${this.selectDevice} data-mac=${d.Mac} class="${this.selectedMAC == d.Mac.toString() ? "text-slate-800" : "text-slate-600"} text-nowrap font-bold cursor-pointer">${d.Name}</a>
            `,
          )}
        </div>
      </div>
    `;
  }

  selectDevice(e: Event) {
    const mac = (e.target as HTMLLinkElement).getAttribute("data-mac");
    if (mac == null) {
      this.selectedMAC = "0";
      return;
    }
    this.selectedMAC = mac;
    this.requestUpdate();
  }

  deviceConfiguration(): TemplateResult {
    const d: Device = this.devices.get(this.selectedMAC) ?? {
      Skey: 0,
      Dkey: 0,
      Mac: 0,
      Name: "",
      Inputs: "",
      Outputs: "",
      Wifi: "",
      MonitorPeriod: 0,
      ActPeriod: 0,
      Type: "",
      Version: "",
      Protocol: "",
      Status: 0,
      Latitude: 0,
      Longitude: 0,
      Enabled: false,
      Updated: new Date(0),
      Other: new Map<string, string>(),
    };

    return html`
      <div class="p-4">
        <h2 class="text-blue-900 text-lg">Configuration</h2>
        <hr class="border-gray-300 my-2" />
        <form class="flex flex-col gap-2 p-4" @submit=${this.submitConfig}>
          <div class="flex gap-2 justify-center">
            <div>
              <p class="text-green-700">●</p>
              <img src="/s/update.png" class="h-1/3" />
            </div>
            <input .value="${d.Name}" class="w-3/5 border-b border-gray-200 text-center px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">MAC:</label>
            <input .value="${d.Mac}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Type:</label>
            <select class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-2 text-gray-800">
              <option>...</option>
            </select>
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Inputs:</label>
            <input .value="${d.Inputs}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Outputs:</label>
            <input .value="${d.Outputs}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">WiFi:</label>
            <input .value="${d.Wifi}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Mon Period:</label>
            <input .value="${d.MonitorPeriod}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Act Period:</label>
            <input .value="${d.ActPeriod}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Client Version:</label>
            <input .value="${d.Version}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Client Protocol:</label>
            <input .value="${d.Protocol}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Latitude:</label>
            <input .value="${d.Latitude}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Longitude:</label>
            <input .value="${d.Longitude}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Device Key:</label>
            <input .value="${d.Dkey}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Local Address:</label>
            <input .value="${d.Other?.get("localaddr")}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Uptime:</label>
            <input .value="${d.Name}" class="w-3/5 border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          <div class="flex gap-2 items-center">
            <label class="w-1/5 text-end">Enabled:</label>
            <input ?checked=${d.Enabled}" type="checkbox" class="border border-solid border-gray-200 rounded-sm px-4 py-1 text-gray-800" />
          </div>
          ${this.deviceConfigActions(d)}
        </form>
      </div>
    `;
  }

  deviceConfigActions(d: Device) {
    if (d.Mac == 0) {
      // New Device.
      return html`
        <div class="flex gap-2 items-center">
          <label class="w-1/5"></label>
          <input type="submit" value="Add" class="bg-blue-900 text-white rounded-sm py-2 w-3/5" />
        </div>
      `;
    }

    return html`
      <div class="d-flex gap-1">
        <input class="btn btn-primary" type="submit" name="task" value="Update" />
        <input class="btn btn-primary" type="submit" name="task" value="Shutdown" />
        <input class="btn btn-primary" type="submit" name="task" value="Reboot" />
        <input class="advanced btn btn-outline-primary" type="submit" name="task" value="Debug" />
        <input class="advanced btn btn-outline-primary" type="submit" name="task" value="Upgrade" />
        <input class="advanced btn btn-outline-primary" type="submit" name="task" value="Alarm" />
        <input class="advanced btn btn-outline-primary" type="submit" name="task" value="Test" />
        <input class="advanced btn btn-outline-primary" type="submit" name="task" value="Delete" onclick="return confirm('Are you sure?')" />
      </div>
    `;
  }

  submitConfig(e: SubmitEvent) {
    e.stopPropagation();
    e.preventDefault();

    console.log((e.submitter as HTMLInputElement).value);

    const form = new FormData();

    fetch("/set/devices/edit", {
      method: "POST",
      credentials: "include",
      body: form,
    });
  }

  async getDevices() {
    const devs = await fetch("/api/get/devices/site").then((resp) => {
      return resp.json();
    });
    if (typeof devs != "object") {
      return;
    }

    devs.map((dev: Device) => {
      this.devices.set(dev.Mac.toString(), dev);
    });
    this.requestUpdate();
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "device-settings": DeviceSettings;
  }
}
