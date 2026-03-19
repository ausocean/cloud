import { html, nothing } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";

// ─── Types ────────────────────────────────────────────────────────────────────

type MediaItem = {
    key_id: number;
    mid: number;
    mac: string;
    pin: string;
    timestamp: number;
    duration_sec: number;
    type: string;
    geohash?: string;
    date: string;
    clip_size_bytes: number;
};

type UIState = "loading" | "idle" | "deleting" | "error";

// ─── Helpers ──────────────────────────────────────────────────────────────────

async function fetchJSON<T>(url: string, options?: RequestInit, ms = 20_000): Promise<T> {
    const ac = new AbortController();
    const id = setTimeout(() => ac.abort(), ms);
    try {
        const r = await fetch(url, { credentials: "same-origin", signal: ac.signal, ...options });
        const body = await r.text();
        if (!r.ok) throw new Error(`${r.status} ${r.statusText} — ${body}`);
        return JSON.parse(body) as T;
    } finally {
        clearTimeout(id);
    }
}

function fmtDate(iso: string): string {
    try {
        return new Date(iso).toLocaleString(undefined, { dateStyle: "short", timeStyle: "medium" });
    } catch {
        return iso;
    }
}

function fmtDuration(sec: number): string {
    if (!sec) return "—";
    if (sec < 60) return `${sec.toFixed(1)} s`;
    const m = Math.floor(sec / 60);
    const s = Math.round(sec % 60);
    return `${m}m ${s}s`;
}

function fmtBytes(n: number): string {
    if (n === 0) return "0 B";
    if (n < 1024) return `${n} B`;
    if (n < 1048576) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1048576).toFixed(2)} MB`;
}

function playURL(item: MediaItem): string {
    return `/play?id=${item.mid}&ts=${item.timestamp}`;
}

// ─── Component ────────────────────────────────────────────────────────────────

@customElement("media-manager")
export class MediaManager extends TailwindElement() {
    @state() private items: MediaItem[] = [];
    @state() private uiState: UIState = "loading";
    @state() private errorMsg = "";
    @state() private selected = new Set<number>(); // key_ids
    @state() private showModal = false;
    @state() private filterMac = "";
    @state() private filterPin = "";
    @state() private filterFrom = "";
    @state() private filterTo = "";
    @state() private deleteCount = 0;
    @state() private deleteOk = false;

    connectedCallback() {
        super.connectedCallback();
        queueMicrotask(() => this.load());
    }

    // ── Data loading ────────────────────────────────────────────────────────────

    private async load() {
        this.uiState = "loading";
        this.errorMsg = "";
        try {
            const params = new URLSearchParams();
            if (this.filterFrom) params.set("from", String(Math.floor(new Date(this.filterFrom).getTime() / 1000)));
            if (this.filterTo) params.set("to", String(Math.floor(new Date(this.filterTo).getTime() / 1000)));
            const url = "/api/v1/media" + (params.toString() ? "?" + params : "");
            const data = await fetchJSON<MediaItem[]>(url);
            this.items = data ?? [];
            this.selected = new Set();
            this.uiState = "idle";
        } catch (e: any) {
            this.errorMsg = e?.message ?? String(e);
            this.uiState = "error";
        }
    }

    // ── Filtering ────────────────────────────────────────────────────────────────

    private get visibleItems(): MediaItem[] {
        return this.items.filter((item) => {
            if (this.filterMac && !item.mac.toLowerCase().includes(this.filterMac.toLowerCase())) return false;
            if (this.filterPin && item.pin.toLowerCase() !== this.filterPin.toLowerCase()) return false;
            return true;
        });
    }

    private get uniqueMacs(): string[] {
        return [...new Set(this.items.map((i) => i.mac))].sort();
    }

    private get uniquePins(): string[] {
        return [...new Set(this.items.map((i) => i.pin))].sort();
    }

    // ── Selection ────────────────────────────────────────────────────────────────

    private toggleSelect(keyId: number) {
        const s = new Set(this.selected);
        if (s.has(keyId)) s.delete(keyId);
        else s.add(keyId);
        this.selected = s;
    }

    private toggleAll() {
        const visible = this.visibleItems;
        const allSelected = visible.every((i) => this.selected.has(i.key_id));
        const s = new Set(this.selected);
        if (allSelected) {
            visible.forEach((i) => s.delete(i.key_id));
        } else {
            visible.forEach((i) => s.add(i.key_id));
        }
        this.selected = s;
    }

    // ── Delete flow ──────────────────────────────────────────────────────────────

    private openDeleteModal() {
        this.showModal = true;
        this.deleteOk = false;
    }

    private closeModal() {
        this.showModal = false;
    }

    private async confirmDelete() {
        this.uiState = "deleting";
        this.showModal = false;
        try {
            const keyIds = [...this.selected];
            await fetchJSON<{ deleted: number }>("/api/v1/media", {
                method: "DELETE",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ key_ids: keyIds }),
            });
            this.deleteCount = keyIds.length;
            this.deleteOk = true;
            await this.load();
        } catch (e: any) {
            this.errorMsg = `Delete failed: ${e?.message ?? e}`;
            this.uiState = "error";
        }
    }

    // ── Render ───────────────────────────────────────────────────────────────────

    render() {
        return html`
      <div class="max-w-screen-xl mx-auto px-4 py-6 space-y-4">
        ${this.renderBanner()}
        ${this.renderFilters()}
        ${this.renderTable()}
        ${this.renderDeleteBar()}
        ${this.showModal ? this.renderModal() : nothing}
      </div>
    `;
    }

    private renderBanner() {
        if (this.uiState === "error") {
            return html`
        <div class="flex items-center gap-2 rounded-lg border border-red-300 bg-red-50 dark:bg-red-900/20 dark:border-red-700 px-4 py-3 text-sm text-red-700 dark:text-red-300">
          <svg class="w-4 h-4 shrink-0" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm-.75-11.25a.75.75 0 011.5 0v4a.75.75 0 01-1.5 0v-4zm.75 7a1 1 0 110-2 1 1 0 010 2z" clip-rule="evenodd"/></svg>
          <span>${this.errorMsg}</span>
          <button @click=${() => this.load()} class="ml-auto text-xs underline">Retry</button>
        </div>
      `;
        }
        if (this.deleteOk) {
            return html`
        <div class="flex items-center gap-2 rounded-lg border border-green-300 bg-green-50 dark:bg-green-900/20 dark:border-green-700 px-4 py-3 text-sm text-green-700 dark:text-green-300">
          <svg class="w-4 h-4 shrink-0" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
          <span>Successfully deleted ${this.deleteCount} clip${this.deleteCount === 1 ? "" : "s"}.</span>
          <button @click=${() => { this.deleteOk = false; }} class="ml-auto text-xs underline">Dismiss</button>
        </div>
      `;
        }
        return nothing;
    }

    private renderFilters() {
        return html`
      <div class="flex flex-wrap gap-3 items-end">
        <div class="flex flex-col gap-1">
          <label class="text-xs font-medium text-gray-500 dark:text-gray-400">MAC</label>
          <select
            class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change=${(e: Event) => { this.filterMac = (e.target as HTMLSelectElement).value; }}
          >
            <option value="">All MACs</option>
            ${this.uniqueMacs.map((m) => html`<option value=${m}>${m}</option>`)}
          </select>
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-xs font-medium text-gray-500 dark:text-gray-400">Pin</label>
          <select
            class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change=${(e: Event) => { this.filterPin = (e.target as HTMLSelectElement).value; }}
          >
            <option value="">All Pins</option>
            ${this.uniquePins.map((p) => html`<option value=${p}>${p}</option>`)}
          </select>
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-xs font-medium text-gray-500 dark:text-gray-400">From</label>
          <input
            type="datetime-local"
            class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change=${(e: Event) => { this.filterFrom = (e.target as HTMLInputElement).value; }}
          />
        </div>

        <div class="flex flex-col gap-1">
          <label class="text-xs font-medium text-gray-500 dark:text-gray-400">To</label>
          <input
            type="datetime-local"
            class="text-sm rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-800 text-gray-800 dark:text-gray-100 px-2 py-1.5 focus:outline-none focus:ring-2 focus:ring-blue-500"
            @change=${(e: Event) => { this.filterTo = (e.target as HTMLInputElement).value; }}
          />
        </div>

        <button
          @click=${() => this.load()}
          class="text-sm rounded bg-blue-600 hover:bg-blue-700 text-white px-4 py-1.5 font-medium transition-colors"
        >
          Apply
        </button>

        <span class="ml-auto text-xs text-gray-400 self-end pb-0.5">
          ${this.uiState === "loading" ? "Loading…" : `${this.visibleItems.length} of ${this.items.length} clips`}
        </span>
      </div>
    `;
    }

    private renderTable() {
        const visible = this.visibleItems;
        const allSelected = visible.length > 0 && visible.every((i) => this.selected.has(i.key_id));

        if (this.uiState === "loading") {
            return html`
        <div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div class="px-4 py-10 text-center text-sm text-gray-400">Loading clips…</div>
        </div>
      `;
        }

        if (this.uiState === "deleting") {
            return html`
        <div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div class="px-4 py-10 text-center text-sm text-gray-400">Deleting…</div>
        </div>
      `;
        }

        if (visible.length === 0 && this.uiState === "idle") {
            return html`
        <div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-hidden">
          <div class="px-4 py-10 text-center text-sm text-gray-400">No clips found for the current site and filters.</div>
        </div>
      `;
        }

        const thCls = "px-3 py-2 text-left text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wide whitespace-nowrap";
        const tdCls = "px-3 py-1.5 text-sm text-gray-800 dark:text-gray-200 whitespace-nowrap";
        const tdMkd = "px-3 py-1.5 text-xs font-mono text-gray-500 dark:text-gray-400 whitespace-nowrap";

        return html`
      <div class="rounded-lg border border-gray-200 dark:border-gray-700 overflow-x-auto bg-white dark:bg-gray-800">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700 text-left">
          <thead class="bg-gray-50 dark:bg-gray-900/40">
            <tr>
              <th class="${thCls} w-8">
                <input
                  type="checkbox"
                  .checked=${allSelected}
                  @change=${() => this.toggleAll()}
                  class="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                  title="${allSelected ? "Deselect all" : "Select all visible"}"
                />
              </th>
              <th class="${thCls}">Date</th>
              <th class="${thCls}">MAC</th>
              <th class="${thCls}">Pin</th>
              <th class="${thCls}">Duration</th>
              <th class="${thCls}">Type</th>
              <th class="${thCls}">Size</th>
              <th class="${thCls}">Key ID</th>
              <th class="${thCls}">Actions</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-gray-700">
            ${visible.map((item) => {
            const sel = this.selected.has(item.key_id);
            return html`
                <tr class="${sel ? "bg-blue-50 dark:bg-blue-900/20" : "hover:bg-gray-50 dark:hover:bg-gray-700/30"} transition-colors">
                  <td class="px-3 py-1.5">
                    <input
                      type="checkbox"
                      .checked=${sel}
                      @change=${() => this.toggleSelect(item.key_id)}
                      class="rounded border-gray-300 dark:border-gray-600 text-blue-600 focus:ring-blue-500"
                    />
                  </td>
                  <td class="${tdCls}">${fmtDate(item.date)}</td>
                  <td class="${tdMkd}">${item.mac}</td>
                  <td class="${tdCls}">${item.pin}</td>
                  <td class="${tdCls}">${fmtDuration(item.duration_sec)}</td>
                  <td class="${tdMkd}">${item.type || "—"}</td>
                  <td class="${tdCls}">${fmtBytes(item.clip_size_bytes)}</td>
                  <td class="${tdMkd}">${item.key_id}</td>
                  <td class="px-3 py-1.5">
                    <a
                      href=${playURL(item)}
                      title="Play clip"
                      class="inline-flex items-center gap-1 text-xs text-blue-600 dark:text-blue-400 hover:underline"
                    >
                      <svg class="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20"><path d="M6.3 2.84A1.5 1.5 0 004 4.11v11.78a1.5 1.5 0 002.3 1.27l9.344-5.891a1.5 1.5 0 000-2.538L6.3 2.84z"/></svg>
                      Play
                    </a>
                  </td>
                </tr>
              `;
        })}
          </tbody>
        </table>
      </div>
    `;
    }

    private renderDeleteBar() {
        const count = this.selected.size;
        if (count === 0) return nothing;
        return html`
      <div class="sticky bottom-4 flex items-center justify-between rounded-lg border border-red-200 dark:border-red-700 bg-white dark:bg-gray-800 shadow-lg px-4 py-3">
        <span class="text-sm text-gray-700 dark:text-gray-200 font-medium">
          ${count} clip${count === 1 ? "" : "s"} selected
        </span>
        <button
          @click=${() => this.openDeleteModal()}
          class="flex items-center gap-1.5 rounded bg-red-600 hover:bg-red-700 text-white text-sm font-medium px-4 py-1.5 transition-colors"
        >
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 006 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 10.23 1.482l.149-.022.841 10.518A2.75 2.75 0 007.596 19h4.807a2.75 2.75 0 002.742-2.53l.841-10.52.149.023a.75.75 0 00.23-1.482A41.03 41.03 0 0014 4.193V3.75A2.75 2.75 0 0011.25 1h-2.5zM10 4c.84 0 1.673.025 2.5.075V3.75c0-.69-.56-1.25-1.25-1.25h-2.5c-.69 0-1.25.56-1.25 1.25v.325C8.327 4.025 9.16 4 10 4zM8.58 7.72a.75.75 0 00-1.5.06l.3 7.5a.75.75 0 101.5-.06l-.3-7.5zm4.34.06a.75.75 0 10-1.5-.06l-.3 7.5a.75.75 0 101.5.06l.3-7.5z" clip-rule="evenodd"/></svg>
          Delete selected (${count})
        </button>
      </div>
    `;
    }

    private renderModal() {
        const count = this.selected.size;
        return html`
      <!-- Backdrop -->
      <div
        class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm"
        @click=${() => this.closeModal()}
      ></div>

      <!-- Dialog -->
      <div class="fixed inset-0 z-50 flex items-center justify-center p-4">
        <div class="w-full max-w-md rounded-xl bg-white dark:bg-gray-800 shadow-2xl border border-gray-200 dark:border-gray-700 overflow-hidden">
          <!-- Header -->
          <div class="flex items-center gap-3 px-6 py-4 border-b border-gray-200 dark:border-gray-700">
            <span class="flex-shrink-0 rounded-full bg-red-100 dark:bg-red-900/30 p-2">
              <svg class="w-5 h-5 text-red-600 dark:text-red-400" fill="currentColor" viewBox="0 0 20 20">
                <path fill-rule="evenodd" d="M8.485 2.495c.673-1.167 2.357-1.167 3.03 0l6.28 10.875c.673 1.167-.17 2.625-1.516 2.625H3.72c-1.347 0-2.189-1.458-1.515-2.625L8.485 2.495zM10 5a.75.75 0 01.75.75v3.5a.75.75 0 01-1.5 0v-3.5A.75.75 0 0110 5zm0 9a1 1 0 100-2 1 1 0 000 2z" clip-rule="evenodd"/>
              </svg>
            </span>
            <div>
              <h2 class="text-base font-semibold text-gray-900 dark:text-gray-100">Confirm deletion</h2>
              <p class="text-xs text-gray-500 dark:text-gray-400">This action cannot be undone.</p>
            </div>
          </div>

          <!-- Body -->
          <div class="px-6 py-4 space-y-3">
            <p class="text-sm text-gray-700 dark:text-gray-300">
              You are about to permanently delete
              <span class="font-semibold text-red-600 dark:text-red-400">${count} media clip${count === 1 ? "" : "s"}</span>
              from the datastore. This will free up storage but the data cannot be recovered.
            </p>
            <div class="rounded-lg bg-red-50 dark:bg-red-900/20 border border-red-200 dark:border-red-700 px-3 py-2 text-xs text-red-700 dark:text-red-300 max-h-32 overflow-y-auto font-mono">
              ${[...this.selected].map((k) => html`<div>key_id: ${k}</div>`)}
            </div>
          </div>

          <!-- Footer -->
          <div class="flex items-center justify-end gap-3 px-6 py-4 border-t border-gray-200 dark:border-gray-700">
            <button
              @click=${() => this.closeModal()}
              class="rounded-lg border border-gray-300 dark:border-gray-600 text-gray-700 dark:text-gray-200 text-sm font-medium px-4 py-2 hover:bg-gray-50 dark:hover:bg-gray-700 transition-colors"
            >
              Cancel
            </button>
            <button
              @click=${() => this.confirmDelete()}
              class="rounded-lg bg-red-600 hover:bg-red-700 text-white text-sm font-semibold px-4 py-2 transition-colors"
            >
              Delete ${count} clip${count === 1 ? "" : "s"}
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
