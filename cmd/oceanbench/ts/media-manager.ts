import { html, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";

type UIState = "loading" | "idle" | "deleting" | "error";

interface Device {
  Mac: number;
  Name: string;
  Inputs: string;
}

async function fetchJSON<T>(url: string, options?: RequestInit, ms = 300_000): Promise<T> {
  const ac = new AbortController();
  const id = setTimeout(() => ac.abort(), ms);
  try {
    const r = await fetch(url, { credentials: "same-origin", signal: ac.signal, ...options });
    const body = await r.text();
    if (!r.ok) throw new Error(`${r.status} ${r.statusText} — ${body}`);
    return JSON.parse(body) as T;
  } catch (e: any) {
    if (e.name === "AbortError" || String(e).includes("aborted")) {
      throw new Error(`The request timed out after ${Math.round(ms / 1000)} seconds. This usually means the server was too busy processing the data. Adjust chunk sizes or try again later.`);
    }
    throw e;
  } finally {
    clearTimeout(id);
  }
}

@customElement("media-manager")
export class MediaManager extends TailwindElement() {
  @state() private devices: Device[] = [];
  @state() private selectedDeviceMac: string = "";
  @state() private selectedPin: string = "";
  @state() private keysByDate: Record<string, string[]> = {};
  @state() private selectedDates: Set<string> = new Set();
  @state() private summary: Record<string, number> = {};
  @state() private uiState: UIState = "loading";
  @state() private errorMsg = "";
  @state() private deleteProgress = { current: 0, total: 0 };
  @state() private selectedMonth: string;
  @state() private selectedDay: string = "";
  @state() private selectedLimit = "50000";
  @state() private showDeleteModal = false;

  constructor() {
    super();
    const now = new Date();
    this.selectedMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
  }

  connectedCallback() {
    super.connectedCallback();
    queueMicrotask(() => this.fetchDevices().then(() => this.load()));
  }

  private async fetchDevices() {
    try {
      this.devices = await fetchJSON<Device[]>("/api/get/devices/site");
    } catch (e) {
      console.error("Failed to load devices", e);
    }
  }

  private async load() {
    this.uiState = "loading";
    this.errorMsg = "";
    try {
      const params = new URLSearchParams();
      try {
        params.set("tz", Intl.DateTimeFormat().resolvedOptions().timeZone);
      } catch (e) {
        // Fallback for older browsers
      }
      if (this.selectedLimit) params.set("limit", this.selectedLimit);

      if (this.selectedDeviceMac) params.set("device", this.selectedDeviceMac);
      if (this.selectedPin) params.set("pin", this.selectedPin);

      if (this.selectedMonth) {
        const [year, month] = this.selectedMonth.split('-').map(Number);
        
        if (this.selectedDay) {
          // local time start and end of specific day
          const day = Number(this.selectedDay);
          params.set("from", String(new Date(year, month - 1, day).getTime() / 1000));
          params.set("to", String(new Date(year, month - 1, day + 1).getTime() / 1000));
        } else {
          // local time start of month
          params.set("from", String(new Date(year, month - 1, 1).getTime() / 1000));
          // local time start of next month
          params.set("to", String(new Date(year, month, 1).getTime() / 1000));
        }
      }

      const url = `/api/v1/media?${params.toString()}`;
      const data = await fetchJSON<{keysByDate: Record<string, string[]>, summary: Record<string, number>}>(url);
      this.keysByDate = data?.keysByDate ?? {};
      this.summary = data?.summary ?? {};
      this.selectedDates = new Set();
      this.uiState = "idle";
    } catch (e: any) {
      this.errorMsg = e?.message ?? String(e);
      this.uiState = "error";
    }
  }

  private promptDelete() {
    this.showDeleteModal = true;
  }

  private async confirmDelete() {
    this.showDeleteModal = false;
    this.uiState = "deleting";
    
    const pendingKeys: string[] = [];
    for (const date of this.selectedDates) {
      if (this.keysByDate[date]) {
        pendingKeys.push(...this.keysByDate[date]);
      }
    }

    this.deleteProgress = { current: 0, total: pendingKeys.length };
    this.errorMsg = "";

    try {

      while (pendingKeys.length > 0) {
        // Pop up to 500 keys (datastore limit)
        const batch = pendingKeys.splice(0, 500);

        await fetchJSON<{ deleted: number }>("/api/v1/media", {
          method: "DELETE",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ key_ids: batch }),
        });

        this.deleteProgress = {
          current: this.deleteProgress.total - pendingKeys.length,
          total: this.deleteProgress.total
        };

        // Brief pause between batches to avoid overwhelming the datastore.
        if (pendingKeys.length > 0) {
          await new Promise(r => setTimeout(r, 1000));
        }
      }

      // Reload when done
      await this.load();
    } catch (e: any) {
      this.errorMsg = `Delete stopped safely at ${this.deleteProgress.current}/${this.deleteProgress.total}: ${e?.message ?? e}`;
      this.uiState = "error";
    }
  }

  private onMonthChange(e: Event) {
    this.selectedMonth = (e.target as HTMLInputElement).value;
    this.selectedDay = ""; // Reset day when month changes
  }

  private onDayChange(e: Event) {
    this.selectedDay = (e.target as HTMLSelectElement).value;
  }

  private onLimitChange(e: Event) {
    this.selectedLimit = (e.target as HTMLSelectElement).value;
  }

  private getAvailablePins(): string[] {
    const pins = new Set<string>();
    for (const dev of this.devices) {
      if (this.selectedDeviceMac && String(dev.Mac) !== this.selectedDeviceMac) continue;
      if (!dev.Inputs) continue;
      for (const p of dev.Inputs.split(",")) {
        const trimmed = p.trim();
        if (trimmed) pins.add(trimmed);
      }
    }
    return Array.from(pins).sort();
  }

  render() {
    const [year, month] = this.selectedMonth ? this.selectedMonth.split('-').map(Number) : [new Date().getFullYear(), new Date().getMonth() + 1];
    const daysInMonth = new Date(year, month, 0).getDate();

    return html`
      <div class="max-w-screen-md mx-auto px-4 py-8 space-y-6">
        <div class="flex flex-wrap items-center gap-6">
          <div class="flex items-center gap-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Select Month:</label>
            <input 
              type="month" 
              .value=${this.selectedMonth}
              @change=${this.onMonthChange}
              class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2.5 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-sm"
            />
          </div>

          <div class="flex items-center gap-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Day:</label>
            <select
              .value=${this.selectedDay}
              @change=${this.onDayChange}
              class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2.5 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-sm"
            >
              <option value="">All Days</option>
              ${Array.from({length: daysInMonth}, (_, i) => i + 1).map(d => html`<option value="${d}">${d}</option>`)}
            </select>
          </div>

          <div class="flex items-center gap-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Load Limit:</label>
            <select
              .value=${this.selectedLimit}
              @change=${this.onLimitChange}
              class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2.5 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-sm"
            >
              <option value="15000">15,000 clips</option>
              <option value="50000" selected>50,000 clips</option>
              <option value="100000">100,000 clips</option>
              <option value="250000">250,000 clips (May time out)</option>
              <option value="0">All clips (Likely to time out)</option>
            </select>
          </div>

          <div class="flex items-center gap-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Device:</label>
            <select
              .value=${this.selectedDeviceMac}
              @change=${(e: Event) => {
                this.selectedDeviceMac = (e.target as HTMLSelectElement).value;
                this.selectedPin = ""; // reset pin when device changes
              }}
              class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2.5 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-sm"
            >
              <option value="">All Devices</option>
              ${this.devices.map(d => html`<option value="${d.Mac}">${d.Name}</option>`)}
            </select>
          </div>

          <div class="flex items-center gap-2">
            <label class="text-sm font-medium text-gray-700 dark:text-gray-300">Pin:</label>
            <select
              .value=${this.selectedPin}
              @change=${(e: Event) => this.selectedPin = (e.target as HTMLSelectElement).value}
              class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2.5 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500 shadow-sm"
            >
              <option value="">All Pins</option>
              ${this.getAvailablePins().map(p => html`<option value="${p}">${p}</option>`)}
            </select>
          </div>
          
          <button
            @click=${this.load}
            ?disabled=${this.uiState === "loading"}
            class="rounded bg-blue-600 hover:bg-blue-700 text-white px-4 py-1.5 text-sm font-medium transition-colors shadow-sm disabled:opacity-50 disabled:cursor-not-allowed"
          >
            Load
          </button>
        </div>

        ${this.renderBanner()}

        <div class="bg-white dark:bg-gray-800 rounded-xl shadow-sm border border-gray-200 dark:border-gray-700 p-6 flex flex-col items-center justify-center space-y-4">
          ${this.renderContent()}
        </div>
      </div>
      
      ${this.showDeleteModal ? this.renderModal() : nothing}
    `;
  }

  private renderBanner() {
    if (this.uiState === "error") {
      return html`
        <div class="flex items-center gap-3 rounded-lg border border-red-300 bg-red-50 dark:bg-red-900/20 dark:border-red-700 px-4 py-3 text-sm text-red-700 dark:text-red-300">
          <svg class="w-5 h-5 shrink-0" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm-.75-11.25a.75.75 0 011.5 0v4a.75.75 0 01-1.5 0v-4zm.75 7a1 1 0 110-2 1 1 0 010 2z" clip-rule="evenodd"/></svg>
          <span class="font-medium">${this.errorMsg}</span>
          <button @click=${() => this.load()} class="ml-auto text-xs font-semibold hover:underline">Retry</button>
        </div>
      `;
    }
    return nothing;
  }

  private renderContent() {
    if (this.uiState === "loading") {
      return html`
        <div class="animate-pulse flex items-center gap-3 text-gray-500 dark:text-gray-400">
          <svg class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
          Loading keys...
        </div>
      `;
    }

    if (this.uiState === "deleting") {
      const { current, total } = this.deleteProgress;
      const pct = total > 0 ? Math.round((current / total) * 100) : 0;
      return html`
        <div class="text-center w-full max-w-sm">
          <div class="text-sm font-medium text-gray-600 dark:text-gray-300 mb-2">Deleting ${current} / ${total}</div>
          <div class="w-full bg-gray-200 dark:bg-gray-700 rounded-full h-2.5 overflow-hidden">
            <div class="bg-red-600 h-2.5 rounded-full transition-all duration-300" style="width: ${pct}%"></div>
          </div>
          <div class="text-xs text-red-500 mt-2 font-medium animate-pulse">Please do not close this window.</div>
        </div>
      `;
    }

    if (this.uiState === "error" && Object.keys(this.keysByDate).length === 0) {
      return html`
        <div class="text-sm text-gray-500 dark:text-gray-400 font-medium">
          No data loaded. Change limit and try again.
        </div>
      `;
    }

    const count = Object.values(this.keysByDate).reduce((acc, keys) => acc + keys.length, 0);

    const dates = Object.keys(this.summary).sort((a, b) => b.localeCompare(a));
    const allSelected = dates.length > 0 && dates.every(d => this.selectedDates.has(d));

    const toggleAll = (e: Event) => {
      const checked = (e.target as HTMLInputElement).checked;
      const newSelections = new Set(this.selectedDates);
      if (checked) {
        dates.forEach(d => newSelections.add(d));
      } else {
        newSelections.clear();
      }
      this.selectedDates = newSelections;
    };

    const toggleDate = (date: string, e: Event) => {
      const checked = (e.target as HTMLInputElement).checked;
      const newSelections = new Set(this.selectedDates);
      if (checked) {
        newSelections.add(date);
      } else {
        newSelections.delete(date);
      }
      this.selectedDates = newSelections;
    };

    return html`
      <div class="text-center w-full">
        <div class="text-4xl font-bold text-gray-900 dark:text-gray-100 mb-1">${count}</div>
        <div class="text-sm font-medium text-gray-500 dark:text-gray-400 uppercase tracking-wide">Clips found</div>
      </div>
      
      ${count > 0 ? html`
        <div class="w-full max-w-sm mt-4 text-left">
          <div class="text-xs font-semibold text-gray-400 uppercase tracking-wider mb-2 pb-1 border-b border-gray-200 dark:border-gray-700 flex justify-between items-center">
            <span>Clips by Day</span>
            <label class="flex items-center gap-1.5 cursor-pointer text-gray-600 dark:text-gray-300">
              <input type="checkbox" .checked=${allSelected} @change=${toggleAll} class="rounded text-blue-600 focus:ring-blue-500 bg-gray-100 border-gray-300 dark:bg-gray-700 dark:border-gray-600">
              <span class="normal-case">Select All</span>
            </label>
          </div>
          <div class="max-h-48 overflow-y-auto pr-2 space-y-1">
            ${dates.map(d => html`
              <div class="flex justify-between items-center text-sm py-1">
                <label class="flex items-center gap-2 cursor-pointer">
                  <input type="checkbox" .checked=${this.selectedDates.has(d)} @change=${(e: Event) => toggleDate(d, e)} class="rounded text-blue-600 focus:ring-blue-500 bg-gray-100 border-gray-300 dark:bg-gray-700 dark:border-gray-600">
                  <span class="text-gray-700 dark:text-gray-300 font-medium">${d}</span>
                </label>
                <span class="text-gray-500 dark:text-gray-400 bg-gray-100 dark:bg-gray-700/50 px-2 py-0.5 rounded-full text-xs">${this.summary[d]} clips</span>
              </div>
            `)}
          </div>
        </div>

        <button
          @click=${this.promptDelete}
          ?disabled=${this.selectedDates.size === 0}
          class="mt-4 flex items-center justify-center gap-2 rounded-lg bg-red-600 hover:bg-red-700 text-white px-6 py-2.5 font-semibold transition-colors shadow-sm disabled:opacity-50 disabled:cursor-not-allowed w-full max-w-sm"
        >
          <svg class="w-5 h-5" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022.841 10.518A2.75 2.75 0 007.596 19h4.807a2.75 2.75 0 002.742-2.53l.841-10.52.149.023a.75.75 0 00.23-1.482A41.03 41.03 0 0014 4.193V3.75A2.75 2.75 0 0011.25 1h-2.5zM10 4c.84 0 1.673.025 2.5.075V3.75c0-.69-.56-1.25-1.25-1.25h-2.5c-.69 0-1.25.56-1.25 1.25v.325C8.327 4.025 9.16 4 10 4zM8.58 7.72a.75.75 0 00-1.5.06l.3 7.5a.75.75 0 101.5-.06l-.3-7.5zm4.34.06a.75.75 0 10-1.5-.06l-.3 7.5a.75.75 0 101.5.06l.3-7.5z" clip-rule="evenodd"/></svg>
          ${this.selectedDates.size === 0 ? "Select days to delete" : "Delete Selected"}
        </button>
        ${Number(this.selectedLimit) > 0 && count >= Number(this.selectedLimit) ? html`
        <div class="mt-4 text-xs text-gray-500 dark:text-gray-400 max-w-sm">
          Note: To preserve performance a load limit is applied, only ${this.selectedLimit} clips are loaded at once. Delete this batch to reveal older clips.
        </div>
        ` : nothing}
      ` : html`
        <div class="mt-4 text-sm text-green-600 dark:text-green-400 font-medium">
          No media left to clean up for this time period.
        </div>
      `}
    `;
  }

  private renderModal() {
    let count = 0;
    for (const date of this.selectedDates) {
      if (this.keysByDate[date]) {
        count += this.keysByDate[date].length;
      }
    }

    return html`
      <div 
        class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm"
        @click=${() => { this.showDeleteModal = false; }}
      ></div>

      <div class="fixed inset-0 z-50 flex items-center justify-center p-4 pointer-events-none">
        <div class="w-full max-w-md rounded-xl bg-white dark:bg-gray-800 shadow-2xl border border-gray-200 dark:border-gray-700 overflow-hidden pointer-events-auto">
          <div class="flex items-center gap-3 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <span class="flex-shrink-0 rounded-full bg-red-100 dark:bg-red-900/30 p-2">
              <svg class="w-5 h-5 text-red-600 dark:text-red-400" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd"/>
              </svg>
            </span>
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Confirm deletion</h2>
            </div>
          </div>

          <div class="px-6 py-4 space-y-3">
            <p class="text-sm text-gray-700 dark:text-gray-300">
              You are about to permanently delete
              <span class="font-semibold text-red-600 dark:text-red-400">${count} media clip${count === 1 ? "" : "s"}</span>
              from ${this.selectedDates.size} selected day${this.selectedDates.size === 1 ? "" : "s"}.
            </p>
            <p class="text-xs text-gray-500 dark:text-gray-400">This action cannot be undone.</p>
          </div>

          <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700 bg-gray-50 dark:bg-gray-900/40">
            <button
              @click=${() => { this.showDeleteModal = false; }}
              class="rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-300 text-sm font-medium px-4 py-2 hover:bg-gray-100 dark:hover:bg-gray-700 transition-colors"
            >
              Cancel
            </button>
            <button
              @click=${this.confirmDelete}
              class="rounded-lg bg-red-600 hover:bg-red-700 text-white text-sm font-semibold px-4 py-2 transition-colors"
            >
              Delete
            </button>
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "media-manager": MediaManager;
  }
}
