import { html, HTMLTemplateResult } from "lit";
import { customElement, state } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";
import { getSkey } from "./shared/skey";
import { ActionVar, Broadcast, getBroadcast, getBroadcastsBySite, privacyLevels } from "./types/broadcast";
import { Device, getDevicesBySite } from "./types/device";
import { getVariablesByDevices, type Variable } from "./types/variable";

@customElement("broadcast-settings")
export class BroadcastSettings extends TailwindElement() {
  static DEFAULT_ERROR_MSG = "Something went wrong, please try again.";
  static VALID_RESOLUTIONS = ["240p", "360p", "480p", "720p", "1080p", "1440p", "2160p"];

  @state() loading: boolean = true;
  @state() failed: boolean = false;
  @state() failureMsg: string = "";
  @state() showAdvanced: boolean = true;
  @state() skey: number = 0; // Current skey.
  @state() broadcasts: Map<string, Broadcast> = new Map(); // Broadcast objects for the current site.
  @state() devices: Device[] = [];
  @state() currUUID = "";

  // TODO: Update to private var, and update when broadcast is changed or loaded.
  @state() curr: Broadcast = this.broadcasts.get(this.currUUID) ?? new Broadcast();

  private controllers: Device[] = [];
  private cameras: Device[] = [];

  // Map of actuators for each controller device.
  private variables: Map<number, Variable[]> = new Map();

  @state() startTimeValue = this.curr.Start.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

  @state() endTimeValue = this.curr.End.toLocaleTimeString("en-GB", {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });

  async connectedCallback() {
    super.connectedCallback();
    this.skey = getSkey();
    if (this.skey === 0) {
      console.error("unable to get skey");
      return;
    }

    try {
      this.broadcasts = await getBroadcastsBySite(this.skey);
    } catch (e) {
      console.error(`failed to fetch broadcasts for site (${this.skey}): ${e}`);
      this.failed = true;
      this.failureMsg = "Failed to fetch broadcasts for this site.";
      this.loading = false;
      return;
    }

    try {
      this.devices = await getDevicesBySite(this.skey);
      this.controllers = this.devices.filter((d) => d.Type.toLowerCase() == "controller");
      this.cameras = this.devices.filter((d) => d.Type.toLowerCase() == "camera");
    } catch (e) {
      console.error(`failed to fetch devices for site (${this.skey}): ${e}`);
      this.failed = true;
      this.failureMsg = "Failed to fetch devices for this site.";
      this.loading = false;
      return;
    }

    try {
      this.variables = await getVariablesByDevices(this.devices);
    } catch (e) {
      console.error("unable to get variables for devices:", e);
      this.failed = true;
      this.failureMsg = "unable to get variables for devices.";
      this.loading = false;
      return;
    }

    this.loading = false;
  }

  async handleBroadcastSelect(e: Event) {
    const target = e.target as HTMLSelectElement;
    this.currUUID = target.value;

    if (this.currUUID == "") {
      return; // We don't need to fetch the empty broadcast.
    }
    // Re-fetch the broadcast to make sure it is up to date.
    const b = await getBroadcast(this.skey, this.currUUID);
    this.broadcasts.set(this.currUUID, b);
  }

  render() {
    // Show the loading animation.
    // TODO: Make this prettier.
    if (this.loading) {
      return html`
        <p class="animate-pulse">loading</p>
      `;
    }

    // Show the failure message.
    if (this.failed) {
      let msg = BroadcastSettings.DEFAULT_ERROR_MSG;
      if (this.failureMsg != "") {
        msg = this.failureMsg;
      }
      return html`
        <p class="text-red-500 font-bold">${msg}</p>
      `;
    }

    // Show the settings.
    return html`
      <form class="p-2">
        <!--Broadcast Select-->
        <!-- TODO: Reimplement Secondary Toggle -->
        <div class="flex flex-col items-start">
          <label for="broadcast-select" class="shrink-0 text-primary font-bold">Select Broadcast:</label>
          <select id="broadcast-select" name="broadcast-select" class="border rounded-sm border-solid w-full p-2 border-slate-200 shrink font-bold" @change=${(e: Event) => this.handleBroadcastSelect(e)}>
            <option value="">-- New Broadcast --</option>
            ${Array.from(this.broadcasts).map(([uuid, b]) => {
              console.log(uuid);
              console.log(this.currUUID);
              return html`
                <option value="${b.UUID}" ?selected=${uuid == this.currUUID}>${b.Name}</option>
              `;
            })}
          </select>
        </div>

        <!--Stream State-->
        ${this.streamState()}
        <!--Stream Settings-->
        ${this.streamSettings()}
        <!--Device Settings-->
        ${this.deviceSettings()}
        <!--Advanced Settings-->
        ${this.advancedSettings()}
      </form>

      <div class="sticky bottom-0 z-10 flex flex-col-reverse md:flex-row justify-between py-4 gap-4 left-0 bg-white border-t border-slate-200 border-solid mt-2 mx-2 items-baseline">
        <div class="flex gap-2 items-center w-full">
          <button class="rounded-md py-2 bg-primary hover:bg-primary-hover text-white cursor-pointer w-1/2 md:w-20" onclick="buttonClick(this)" value="broadcast-save">Save</button>
          <button class="rounded-md py-2 bg-red-400 hover:bg-red-500 cursor-pointer text-white font-medium w-1/2 md:w-20" onclick="if (confirm('Are you sure you want to delete the broadcast?')) {buttonClick(this)}" value="broadcast-delete">Delete</button>
        </div>
        <div class="flex md:items-center gap-2 w-full md:w-fit flex-col md:flex-row text-sm md:text-base">
          <div class="flex justify-between items-center h-full w-full">
            <label for="advanced-off" class="has-checked:bg-primary has-checked:text-white flex justify-center items-center rounded-l-md bg-slate-200 px-2 py-1 cursor-pointer w-full">
              <input type="radio" name="show-advanced" id="advanced-off" value="false" hidden ?checked=${!this.showAdvanced} />
              Basic
            </label>
            <label for="advanced-on" class="has-checked:bg-primary has-checked:text-white flex justify-center items-center rounded-r-md bg-slate-200 px-2 py-1 cursor-pointer w-full">
              <input type="radio" name="show-advanced" id="advanced-on" value="true" hidden ?checked=${this.showAdvanced} />
              Advanced
            </label>
          </div>
        </div>
      </div>
    `;
  }

  // Renders JSON state data as human readable text.
  renderStateData(data: string): HTMLTemplateResult {
    // Decode base64.
    try {
      data = atob(data);
    } catch (e) {
      console.error("unable to decode base64 encoded state data:", e);
      return html`
        <p class="text-red-600 font-bold">Error decoding State Data</p>
      `;
    }

    // Parse JSON.
    let jsonData: object;
    try {
      jsonData = JSON.parse(data);
    } catch (e) {
      console.error("unable to parse JSON state data:", data);
      return html`
        <p class="text-red-600 font-bold">Error Parsing State Data</p>
      `;
    }

    return html`
      ${Object.entries(jsonData).map(([key, value]) => {
        // Try to parse as a date time.
        let asDate = new Date(value);
        if (asDate instanceof Date && !isNaN(asDate.getTime())) {
          return html`
            <p class="font-bold">${key}:</p>
            <p>${asDate.toLocaleString()}</p>
          `;
        }

        return html`
          <p class="font-bold">${key}:</p>
          <p>${value}</p>
        `;
      })}
    `;
  }

  // Renders JSON state data as human readable text.
  renderActions(actions: ActionVar[]): HTMLTemplateResult {
    const devMap = new Map(this.devices.map((d) => [d.Mac, d.Name]));
    return html`
      ${actions.map((action) => {
        const parts = action.name.split(".");

        // encode mac address.
        const mac = parseInt(parts[0], 16);

        const devName = devMap.get(mac);
        return html`
          <p>${devName}.${parts[1]} = ${action.value}</p>
        `;
      })}
    `;
  }

  // Renders all the power variables for the selected controller device in a styled
  // radio button input list.
  renderPowerVars(): HTMLTemplateResult {
    let vars = this.variables.get(this.curr.ControllerMAC);
    if (vars == undefined) {
      return html`
        <p>No variables found for selected controller</p>
      `;
    }

    // Parse current power variable from actions. (Assume that the first Power variable)
    let hexMac = this.curr.ControllerMAC.toString(16);
    let onPowerActs = this.curr.OnActions.filter((act) => {
      console.log(`${hexMac}.Power vs ${act.name}`);
      return act.name.includes(`${hexMac}.Power`);
    });

    let powerVarName = `${hexMac}.Power2`;
    if (onPowerActs.length > 0) {
      // Typically we will only have a single power variable for the controller,
      // assume the camera is powered by the first found.
      powerVarName = onPowerActs[0].name;
    }

    return html`
      <div class="flex justify-between">
        ${vars.map((v, i, key) => {
          let id = `${v.Name}-var`;
          // Only show the variable name, without the MAC scope.
          console.log(i);
          let label = v.Name.split(".")[1];
          return html`
            <label for=${id} class="flex w-full h-9 items-center justify-center has-checked:bg-primary has-checked:text-white bg-slate-200 cursor-pointer last:rounded-r-md first:rounded-l-md">
              <p class="">${label}</p>
              <input type="radio" name="controller-power-var" id=${id} value=${v.Name} hidden ?checked=${v.Name == powerVarName} />
            </label>
          `;
        })}
      </div>
    `;
  }

  // Renders styled radio buttons
  renderResolutions(): HTMLTemplateResult {
    const currRes = this.getResolution();
    console.log("current res:", currRes);

    // To avoid making buttons too small, we will only offer the resolutions that we are likely to ever use.
    const validResolutions = BroadcastSettings.VALID_RESOLUTIONS.slice(3, 6);

    return html`
      <div class="flex justify-between">
        ${validResolutions.map((res, i) => {
          let id = `${res}-res`;
          // Only show the variable name, without the MAC scope.
          console.log("res:", res);
          return html`
            <label for=${id} class="flex w-full h-9 items-center justify-center has-checked:bg-primary has-checked:text-white bg-slate-200 cursor-pointer last:rounded-r-md first:rounded-l-md">
              <p class="">${res}</p>
              <input type="radio" name="resolution" id=${id} value=${res} hidden ?checked=${res == currRes} />
            </label>
          `;
        })}
      </div>
    `;
  }

  // Get resolution returns a valid resolution in order of:
  // - current broadcast resolution
  // - camera height variable converted to resolution
  // - default of 1080p
  // This also validates using isValidResolution.
  getResolution(): string {
    const defaultResolution = "1080p";

    // Use the current resolution if valid.
    if (this.isValidResolution(this.curr.Resolution)) {
      return this.curr.Resolution;
    }

    // Use the camera's resolution variable if it exists.
    // Otherwise use the default resolution.
    let camVars = this.variables.get(this.curr.CameraMac);
    if (camVars == undefined) {
      return defaultResolution;
    }

    let hexMac = this.curr.CameraMac.toString(16);
    let height = camVars.find((v) => v.Name == `${hexMac}.Height`);

    // Convert height to resolution value (ie. 1080 => 1080p)
    let resolution = `${height}p`;
    if (this.isValidResolution(resolution)) {
      return resolution;
    }

    return defaultResolution;
  }

  // isValidResolution validates against the valid youtube live resolution list.
  isValidResolution(resolution: string): boolean {
    return BroadcastSettings.VALID_RESOLUTIONS.includes(resolution);
  }

  // Renders a card with the current state of the broadcast.
  // TODO: Make this update.
  streamState(): HTMLTemplateResult {
    return html`
      <!--Header-->
      <div class="flex justify-between items-center pt-5 border-b border-solid border-slate-200 mb-2">
        <h2 class="text-xl font-bold text-black">Stream Status</h2>
      </div>

      <!--Enabled and Failure status-->
      <div class="bg-slate-200 text-primary rounded-md p-2 mb-2">
        <div class="flex justify-between items-center">
          <!--Enable Toggle-->
          <div class="flex md:flex-row flex-col items-center gap-2">
            <div class="flex gap-2 items-center">
              <input type="checkbox" name="enabled" id="enabled" class="form-check-input m-0" role="switch" ?checked=${this.curr.Enabled} />
              <label for="enabled">Enabled</label>
            </div>

            <!--Failure Msg-->
            ${this.curr.InFailure
              ? html`
                  <p class="text-red-600 px-4 py-1 font-bold bg-red-300 rounded-full">Broadcast Failed</p>
                `
              : html``}
          </div>

          <!--Reset States-->
          <div class="flex flex-col h-full">
            <button class="bg-primary rounded-sm text-white hover:bg-primary-hover px-5 py-1 md:w-fit w-full h-full" onclick="buttonClick(this)" value="broadcast-reset-state">Reset States</button>
          </div>
        </div>
        <div class="${this.showAdvanced ? "grid" : "hidden"} md:grid-cols-2 font-mono border-t border-solid border-slate-400 mt-2 pt-2">
          <!--Broadcast State-->
          <div class="flex flex-col">
            <div class="flex gap-2">
              <h4 class="font-bold">Broadcast State:</h4>
              <p>${this.curr.BroadcastState}</p>
            </div>
            <div class="px-5 text-sm">${this.renderStateData(this.curr.StateData)}</div>
          </div>

          <!--Hardware State-->
          <div class="flex flex-col">
            <div class="flex gap-2">
              <h4 class="font-bold">Hardware State:</h4>
              <p>${this.curr.HardwareState}</p>
            </div>
            ${this.renderStateData(this.curr.HardwareStateData)}
          </div>
        </div>
      </div>
    `;
  }

  // Renders the settings for a YouTube streaming connection.
  streamSettings(): HTMLTemplateResult {
    return html`
      <!--Header-->
      <div
        class="flex justify-between items-center border-b border-solid border-slate-200 mb-2"
      >
        <h2 class="text-xl font-bold text-black">Stream Settings</h2>
        <a
          href="https://docs.ausocean.org/oceanbench/broadcast/broadcast.html"
          class="hover:slate-900 text-slate-700"
          >Help↗</a
        >
      </div>

      <fieldset>

      <div class="md:grid gap-x-2 md:grid-cols-4">
        <!--Broadcast Name-->
        <div class="flex flex-col col-span-3">
          <label for="broadcast-name" class="text-primary font-bold"
            >Broadcast Name:</label
          >
          <input
            type="input"
            name="broadcast-name"
            class="border border-slate-200 border-solid rounded-sm px-2 py-1"
            value=${this.curr.Name}
          />
        </div>

        <div class="flex flex-col">
          <label for="check-health" class="text-primary font-bold"
            >Health Check:</label
          >
          <div class="flex justify-between items-center h-full">
            <label for="check-health-on" class="has-checked:bg-primary has-checked:text-white flex justify-center items-center rounded-l-md bg-slate-200 w-full h-full">
            <input type="radio" name="check-health" id="check-health-on" value="true" hidden ?checked=${this.curr.CheckingHealth}>
            On
            </label>
            <label for="check-health-off" class="has-checked:bg-primary has-checked:text-white flex justify-center items-center rounded-r-md bg-slate-200 w-full h-full">
            <input type="radio" name="check-health" id="check-health-off" value="false" hidden ?checked=${!this.curr.CheckingHealth}>
            Off
            </label>
        </div>
        </div>
        </div>

        <!--Broadcast UUID-->
        <div class="${this.showAdvanced ? "flex" : "hidden"}  gap-2 font-mono text-sm w-full text-slate-700">
          <label for="broadcast-uuid" class="font-bold">UUID:</label>
          <input type="input" name="broadcast-uuid" class="w-full" value=${this.curr.UUID} readonly disabled />
        </div>


        <!--In failure checkbox (hidden from user)-->
        <div hidden>
          <div class="w-25"></div>
          <div
            class="form-check form-switch d-flex align-items-center gap-1 p-0"
          >
            <input
              type="checkbox"
              name="in-failure"
              class="form-check-input m-0"
              role="switch"
              value="in-failure"
            />
            <small>Failure Mode</small>
          </div>
        </div>

        <!--Description-->
        <div class="flex flex-col mb-2">
          <label for="description" class="text-primary font-bold"
            >Description:</label
          >
          <textarea
            id="description"
            class="border border-slate-200 border-solid rounded-sm px-2 py-1"
            type="input"
            name="description"
          >${this.curr.Description}</textarea>
        </div>

        <!--Privacy-->
        <div class="grid md:grid-cols-2 grid-cols-1 gap-2 mb-2">
          <!--Live Privacy-->
          <div class="flex flex-col">
            <label for="account" class="text-primary font-bold">Live Privacy:</label>
            <div class="flex justify-between h-9">
              ${privacyLevels.map((p) => {
                return html`
                  <label class="capitalize has-checked:bg-primary has-checked:text-white bg-slate-200 last:rounded-r-md first:rounded-l-md flex items-center justify-center w-full h-full" for="${p}-radio">
                    ${p}
                    <input type="radio" name="live-privacy" value=${p} id="${p}-radio" hidden ?checked=${this.curr.LivePrivacy == p} />
                  </label>
                `;
              })}
            </div>
          </div>

          <!--Post Live Privacy-->
          <div class="flex flex-col mb-2">
            <label for="account" class="text-primary font-bold">Post Live Privacy:</label>
            <div class="flex justify-between h-9">
              ${privacyLevels.map((p) => {
                return html`
                  <label class="capitalize has-checked:bg-primary has-checked:text-white bg-slate-200 last:rounded-r-md first:rounded-l-md flex items-center justify-center w-full h-full" for="${p}-radio">
                    ${p}
                    <input type="radio" name="post-live-privacy" value=${p} id="${p}-post-radio" hidden ?checked=${this.curr.PostLivePrivacy == p} />
                  </label>
                `;
              })}
            </div>
          </div>
        </div

        <!--YouTube Authentication-->
        <div class="flex flex-col bg-slate-200 p-2 rounded-md mb-2">
          <h3 class="font-bold text-black">YouTube Channel Authentication</h3>
          <small class="text-primary pb-1"
            >ⓘ Authenticate with YouTube to allow CloudBlue to stream to your
            channel</small
          ><button
            class="bg-primary rounded-sm text-white hover:bg-primary-hover px-5 py-1 md:w-fit w-full"
            onclick="buttonClick(this)"
            value="broadcast-token"
          >
            Authenticate
          </button>
          ${
            this.curr.Account != ""
              ? html`
                  <p class="text-sm text-primary mt-1 md:text-start text-center">
                    Previously authenticated by:
                    <span class="font-mono">${this.curr.Account}</span>
                  </p>
                `
              : ""
          }
        </div>

        <!--Stream Name and Broadcast Host-->
        <div class="grid md:grid-cols-2 grid-cols-1 gap-2 mb-2">
          <!--Stream Name-->
          <div class="flex flex-col">
            <label for="stream-name" class="text-primary font-bold">Stream Name:</label>
            <input
              type="input"
              class="border border-slate-200 border-solid rounded-sm px-2 py-1 h-9"
              name="stream-name"
              value=${this.curr.StreamName}
            />
          </div>

          <!--Broadcast Host-->
          <div class="flex flex-col">
            <label for="broadcast-host" class="text-primary font-bold">Broadcast Host:</label>
            <select
              id="broadcast-host"
              name="broadcast-host"
              class="border border-slate-200 border-solid rounded-sm px-2 py-1 h-9"
            >
              <option value="youtube" ?selected=${this.curr.BroadcastHost == "youtube" || this.curr.BroadcastHost == ""}>YouTube</option>
              <option value="oceanmedia"  ?selected=${this.curr.BroadcastHost == "oceanmedia"}>OceanMedia</option>
            </select>
          </div>
        </div>

        <!--Time Settings-->
        <div class="grid md:grid-cols-2 grid-cols-1 gap-2 mb-2">
          <!--Start Time-->
          <div class="flex flex-col w-full">
            <label for="start-time" class="text-primary font-bold">Start Time:</label>

              <input
                name="start-time"
                class="border border-slate-200 border-solid rounded-sm px-2 py-1 h-9 w-full"
                type="time"
                id="start-time"
                size="14"
                value=${this.startTimeValue}
              />
              <input
                name="start-timestamp"
                type="input"
                id="start-timestamp"
                size="15"
                hidden
              />
          </div>

          <!--End Time-->
          <div class="flex flex-col w-full">
            <label for="end-time" class="text-primary font-bold">End Time:</label>

              <input
                name="end-time"
                class="border border-slate-200 border-solid rounded-sm px-2 py-1 h-9 w-full"
                type="time"
                id="end-time"
                size="14"
                value=${this.endTimeValue}
              />
              <input
                name="end-timestamp"
                type="input"
                id="end-timestamp"
                value=""
                size="15"
                hidden
              />
            </div>
        </div>

      </fieldset>
    `;
  }

  // Renders stylised radio inputs for the analog pins of the given device.
  renderAnalogPins(mac: number, fieldName: string, curr: string, defaultPin?: string): HTMLTemplateResult {
    // Get the device.
    const controller = this.devices.find((d) => d.Mac == mac);
    if (controller == undefined) {
      return html`
        Unable to get device pins.
      `;
    }

    // Filter the analog pins from the inputs.
    const analogInputs = controller.Inputs.split(",").filter((input) => input.startsWith("A"));

    let selected = curr;
    if (selected == "" && defaultPin != undefined) {
      selected = defaultPin;
    }
    return html`
      <div class="flex justify-between">
        ${analogInputs.map((input) => {
          return html`
            <label class="first:rounded-l-sm last:rounded-r-sm has-checked:bg-primary bg-white has-checked:text-white items-center justify-center w-full p-1 flex">
              ${input}
              <input type="radio" hidden name=${fieldName} id="${input}-pin" ?checked=${input == selected} />
            </label>
          `;
        })}
      </div>
    `;
  }

  // Renders the variables of the currently selected camera in a drop down with
  // the given label and field name. The selected variable is taken from the
  // current broadcast config from the passed configField. If the that value
  // is not set, the defaultVarName is selected if it exists.
  cameraVarSelect(fieldName: string, label: string, configField: string, defaultVarName?: string): HTMLTemplateResult {
    const mac = this.curr.CameraMac;

    // No camera selected.
    if (mac == 0) {
      return html`
        <div class="flex flex-col">
          <label for=${fieldName} class="text-primary font-bold">${label}:</label>
          <p>Select a Camera first.</p>
        </div>
      `;
    }

    // Find camera to pretty print variable names.
    const cam = this.cameras.find((c) => (c.Mac = mac));
    if (cam == undefined) {
      return html`
        <p>Camera not found on site.</p>
      `;
    }

    // Get variables of the currently selected camera.
    const camVars = this.variables.get(mac);
    if (camVars == undefined || camVars.length <= 0) {
      return html`
        <div class="flex flex-col">
          <label for=${fieldName} class="text-primary font-bold">${label}:</label>
          <p>No variables found for selected Camera.</p>
        </div>
      `;
    }

    // Use the currently defined variable, otherwise, fallback to the default.
    let selected = Object(this.curr)[configField].split(".")[1];
    console.log("first selected:", selected);
    if (selected == "" || selected == undefined) {
      selected = defaultVarName;
      console.log("second selected:", selected);
    }

    return html`
      <div class="flex flex-col">
        <label for=${fieldName} class="text-primary font-bold">${label}:</label>
        <select name=${fieldName} class="px-2 py-1 border border-solid border-slate-200 bg-white rounded-sm">
          <option>-- ${label} --</option>
          ${camVars.map((v) => {
            const parts = v.Name.split(".");

            return html`
              <option value=${v.Name} ?selected=${parts[1] == selected}>${cam.Name} - ${parts[1]}</option>
            `;
          })}
        </select>
      </div>
    `;
  }

  // Renders all settings related to the hardware, and its configuration.
  deviceSettings(): HTMLTemplateResult {
    return html`
      <!--Heading-->
      <div class="flex justify-between items-center pt-5 border-b border-solid border-slate-200 mb-2">
        <h2 class="text-xl font-bold text-black">Device Settings</h2>
      </div>

      <fieldset>
        <!--Camera Selection-->
        <div class="grid md:grid-cols-2 grid-cols-1 gap-2 mb-2">
          <!--Camera Select-->
          <div class="flex flex-col">
            <label for="camera-mac" class="text-primary font-bold">Camera:</label>
            <select id="camera-select" name="camera-mac" class="border rounded-sm border-solid w-full p-2 border-slate-200 shrink">
              <option ?selected=${this.curr.CameraMac == 0}>Select</option>
              ${this.cameras.map((c) => {
                return html`
                  <option value=${c.Mac} ?selected=${this.curr.CameraMac == c.Mac}>${c.Name}</option>
                `;
              })}
            </select>
            <p class="${this.showAdvanced ? "block" : "hidden"} text-sm font-mono text-primary">MAC Address: ${this.curr.CameraMac.toString(16)}</p>
          </div>

          <!--Resolution Select-->
          <div class="flex flex-col">
            <label for="resolution" class="text-primary font-bold">Resolution:</label>
            ${this.renderResolutions()}
          </div>
        </div>

        <!--Camera Variables-->
        <div class="${this.showAdvanced ? "grid" : "hidden"} bg-slate-200 rounded-md font-mono text-sm p-2 md:grid-cols-2 grid-cols-1 gap-2 mb-2">
          <!--RTMPURL-->
          ${this.cameraVarSelect("rtmp-key-var", "RTMP URL Variable", "RTMPVar", "RTMPURL")}
          <!--AuthKey-->
          ${this.cameraVarSelect("auth-key-var", "Auth Key Variable", "AuthKeyVar", "AuthKey")}
          <!--StorageConfig-->
          ${this.cameraVarSelect("storage-config-var", "Storage Config Variable", "StorageConfigVar", "StorageConfig")}
          <!--Output-->
          ${this.cameraVarSelect("camera-output-var", "Camera Output Variable", "CameraOutputVar", "Output")}
        </div>

        <!--Controller Selection-->
        <div class="grid md:grid-cols-2 grid-cols-1 gap-2 mb-2">
          <!--Controller Select-->
          <div class="flex flex-col">
            <label for="controller-mac" class="text-primary font-bold">Controller:</label>
            <select id="controller-select" name="controller-mac" class="border rounded-sm border-solid w-full p-2 border-slate-200 shrink">
              <option ?selected=${this.curr.ControllerMAC == 0}>Select</option>
              ${this.controllers.map((c) => {
                return html`
                  <option value=${c.Mac} ?selected=${this.curr.ControllerMAC == c.Mac}>${c.Name}</option>
                `;
              })}
            </select>
            <p class="${this.showAdvanced ? "block" : "hidden"} text-sm font-mono text-primary">MAC Address: ${this.curr.ControllerMAC.toString(16)}</p>
          </div>

          <!--Actuator Select-->
          <div class="flex flex-col">
            <label for="controller-mac" class="text-primary font-bold">Actuator:</label>
            <div>${this.renderPowerVars()}</div>
          </div>
        </div>

        <!--Voltage Timeout-->
        <div class="bg-slate-200 rounded-md p-2 mb-2 font-mono text-sm">
          <h3 class="font-bold text-base text-primary">Voltage Recovery Settings</h3>
          <div class="grid md:grid-cols-2 grid-cols-1 gap-2">
            <div class="flex flex-col">
              <label for="battery-voltage-pin" class="text-primary font-bold">Battery Voltage Pin:</label>
              ${this.renderAnalogPins(this.curr.ControllerMAC, "battery-voltage-pin", this.curr.BatteryVoltagePin, "A4")}
            </div>
            <div class="flex flex-col">
              <label for="required-streaming-voltage" class="text-primary font-bold">Required Streaming Voltage:</label>
              <input class="w-full border border-solid border-slate-200 rounded-md px-2 py-1 bg-white invalid:bg-red-200" type="number" name="required-streaming-voltage" value=${this.curr.RequiredStreamingVoltage} />
            </div>
            <div class="flex flex-col">
              <label for="voltage-recovery-timeout" class="text-primary font-bold">Voltage Recovery Timeout (hr):</label>
              <input class="w-full border border-solid border-slate-200 rounded-md px-2 py-1 bg-white invalid:bg-red-200" type="number" step="1" name="voltage-recovery-timeout" value=${this.curr.VoltageRecoveryTimeout} />
            </div>
          </div>
        </div>

        <!--Actions-->
        <div class="${this.showAdvanced ? "block" : "hidden"} bg-slate-200 rounded-md font-mono text-sm p-2">
          <!--On Actions-->
          <div class="flex flex-col">
            <label for="on-actions" class="font-bold text-base text-primary">On Actions:</label>
            <textarea id="on-actions" name="on-actions" class="w-full h-fit bg-white rounded-sm px-2 py-1" placeholder='[{"name":"ESP.Power2","value":"true"}]'>${JSON.stringify(this.curr.OnActions)}</textarea>
          </div>

          <!--Shutdown Actions-->
          <div class="flex flex-col">
            <label for="shutdown-actions" class="font-bold text-base text-primary">Shutdown Actions:</label>
            <textarea id="shutdown-actions" name="shutdown-actions" class="w-full h-fit bg-white rounded-sm px-2 py-1" placeholder='[{"name":"ESP.Power2","value":"true"}]'>${JSON.stringify(this.curr.ShutdownActions)}</textarea>
          </div>

          <!--Off Actions-->
          <div class="flex flex-col">
            <label for="off-actions" class="font-bold text-base text-primary">Off Actions:</label>
            <textarea id="off-actions" name="off-actions" class="w-full h-fit bg-white rounded-sm px-2 py-1" placeholder='[{"name":"ESP.Power2","value":"true"}]'>${JSON.stringify(this.curr.OffActions)}</textarea>
          </div>
        </div>
      </fieldset>
    `;
  }

  advancedSettings(): HTMLTemplateResult {
    return html`
      <div class="${this.showAdvanced ? "grid" : "hidden"} w-full">
        <!--Heading-->
        <div class="flex justify-between items-center pt-5 border-b border-solid border-slate-200 mb-2">
          <h2 class="text-xl font-bold text-black">Advanced Settings</h2>
        </div>

        <!--Vidforward Settings-->
        <div class="grid md:grid-cols-4 grid-cols-1 gap-x-2 mb-2">
          <div class="flex flex-col w-full">
            <label class="text-primary font-bold">Use Vidforward:</label>
            <div class="flex justify-between items-center h-full w-full">
              <label for="use-vidforward-on" class="has-checked:bg-primary has-checked:text-white flex justify-center items-center rounded-l-md bg-slate-200 w-full h-full">
                <input type="radio" name="use-vidforward" id="use-vidforward-on" value="true" hidden ?checked=${this.curr.UsingVidforward} />
                On
              </label>
              <label for="use-vidforward-off" class="has-checked:bg-primary has-checked:text-white flex justify-center items-center rounded-r-md bg-slate-200 w-full h-full">
                <input type="radio" name="use-vidforward" id="use-vidforward-off" value="false" hidden ?checked=${!this.curr.UsingVidforward} />
                Off
              </label>
            </div>
          </div>
          <div class="flex flex-col col-span-3">
            <label for="vidforward-host" class="text-primary font-bold">Vidforward Host:</label>
            <input class="border border-solid border-slate-200 rounded-md px-2 py-1 w-full" type="input" name="vidforward-host" value="" />
          </div>
        </div>

        <!--Vidforward Slate-->
        <div class="flex flex-col mb-2">
          <label for="slate-file" class="text-primary font-bold">Slate File:</label>
          <div class="grid md:grid-cols-3 grid-cols-1 gap-2 items-center">
            <input class="bg-slate-200 font-mono rounded-md px-2 py-1 cursor-pointer md:col-span-2" type="file" name="slate-file" />
            <button class="rounded-md bg-primary hover:bg-primary-hover px-2 py-1 text-white cursor-pointer shrink-0" onclick="buttonClick(this)" value="vidforward-slate-update">Upload Slate</button>
          </div>
        </div>

        <!--Notification Suppression-->
        <div class="flex flex-col mb-2">
          <label for="notify-suppress-rules" class="text-primary font-bold">Notification Suppression Rules:</label>
          <input class="border border-solid border-slate-200 rounded-md px-2 py-1 w-full" type="input" name="notify-suppress-rules" value="" />
        </div>

        <!--Storage Provider-->
        <div class="flex flex-col mb-2">
          <label for="storage-provider" class="text-primary font-bold">Storage Provider:</label>
          <select id="storage-provider" name="storage-provider" class="border border-solid border-slate-200 rounded-md px-2 py-1 w-full h-9">
            <option value="cloudflare">Cloudflare</option>
          </select>
        </div>

        <!--Bucket and Prefix-->
        <div class="grid md:grid-cols-2 grid-cols-1 gap-2">
          <div class="flex flex-col">
            <label for="storage-prefix" class="text-primary font-bold">Storage Prefix:</label>
            <input id="storage-prefix" class="border border-solid border-slate-200 rounded-md px-2 py-1 w-full" type="input" name="storage-prefix" value="" readonly />
          </div>
          <div class="flex flex-col">
            <label for="storage-bucket" class="text-primary font-bold">Storage Bucket:</label>
            <input id="storage-bucket" class="border border-solid border-slate-200 rounded-md px-2 py-1 w-full" type="input" name="storage-bucket" value="" />
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "broadcast-settings": BroadcastSettings;
  }
}
