import { html, PropertyValueMap } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { isConstructSignatureDeclaration } from "typescript";
import { TailwindElement } from "./shared/tailwind.element";
import { getLogsByDevice, putNewLog, type Log } from "./types/log";

// Displays the logs for a given device MAC, as well as providing an
// interface to add new logs.
@customElement("device-logs")
export class DeviceLogs extends TailwindElement() {
  // Encoded MAC of the device to show logs for.
  @property({ type: Number }) MAC: number;

  // Array of logs to display.
  @state() private logs: Log[] = [];

  // Format spec for printing dates.
  private dateFmt: Intl.DateTimeFormat = new Intl.DateTimeFormat("en-AU", {
    year: "2-digit",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
  });

  constructor() {
    super();

    this.MAC = 0;
  }

  async connectedCallback(): Promise<void> {
    super.connectedCallback();
    try {
      let logs = await getLogsByDevice(this.MAC);
      logs.forEach((l) => {
        // Ensure date is parsed correctly.
        l.Created = new Date(l.Created);
      });
      logs.sort((a, b) => {
        // Sort the logs by their created date.
        return a.Created.valueOf() - b.Created.valueOf();
      });
      this.logs = logs;
    } catch {
      this.logs = [];
    }
  }

  render() {
    if (this.MAC == 0) {
      // We don't have a MAC, so don't render anything yet.
      return;
    }
    return html`
      <h2 class="font-bold text-2xl mt-8">Logs</h2>
      <div
        class="w-full border-slate-200 rounded-md bg-white border border-solid p-6 max-h-96 overflow-y-auto"
      >
        ${this.logs.map((log) => this.renderLog(log))} ${this.newLogForm()}
      </div>
    `;
  }

  // Renders a log in a row.
  renderLog(log: Log) {
    return html`
      <div class="grid gap-4 grid-cols-6 border-b border-slate-200 py-2">
        <span class="text-slate-700 col-span-1 text-right font-mono"
          >${this.dateFmt.format(new Date(log.Created))}</span
        >
        <span class="text-slate-900 col-span-5 font-mono">${log.Note}</span>
      </div>
    `;
  }

  // Renders a form to type a new log and set its level.
  newLogForm() {
    return html`
      <form class="grid gap-4 grid-cols-6 border-b border-slate-200 py-2" @submit=${this.handleNewLog}>
        <span class="text-slate-700 col-span-1 text-right font-mono"
          >Now</span
        >
        <input name="note" class="text-slate-900 col-span-3 font-mono" placeholder="New log note"></input>
        <select name="level" class="text-end font-mono text-slate-900 bg-slate-200 rounded-full field-sizing-content">
          <option>Low</option>
          <option>Medium</option>
          <option>High</option>
          <option>Critical</option>
        </select>
        <button class="bg-primary text-white hover:bg-primary-hover rounded-full cursor-pointer">Save</button>
      </form>
    `;
  }

  // Puts a new log and clears the form.
  async handleNewLog(e: SubmitEvent) {
    e.preventDefault();
    e.stopPropagation();

    let form = e.target as HTMLFormElement;
    let formData = new FormData(form);

    let newLog = await putNewLog(this.MAC, formData);
    this.logs = [...this.logs, newLog];

    form.reset();
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "device-logs": DeviceLogs;
  }
}
