import { html, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";

type Site = {
  Skey: number;
  Name: string;
  Public?: boolean;
  Perm?: number;
};

/** Metadata for each site list category. */
const listMeta = {
  all: {
    title: "All Sites",
    info: "Every site that exists, including ones you don't have permissions for. Visible only because you are a super admin.",
    url: "/api/v1/sites/all",
  },
  public: {
    title: "Public Sites",
    info: "Sites that are publicly visible to everyone, regardless of whether you have specific permissions for them.",
    url: "/api/v1/sites/public",
  },
  user: {
    title: "Your Sites",
    info: "Sites you have been specifically granted permissions for. May include some public sites, but only the ones you have explicit permissions on.",
    url: "/api/v1/sites/user",
  },
} as const;

type ListKind = keyof typeof listMeta;

type ListState = {
  items: Site[] | null;
  error: string;
  loading: boolean;
};

async function fetchJSON<T>(url: string, ms = 15_000): Promise<T> {
  const ac = new AbortController();
  const id = setTimeout(() => ac.abort(), ms);
  try {
    const r = await fetch(url, { credentials: "same-origin", signal: ac.signal });
    const body = await r.text();
    if (!r.ok) throw new Error(`${r.status} ${r.statusText} — ${body}`);
    return JSON.parse(body) as T;
  } finally {
    clearTimeout(id);
  }
}

@customElement("admin-site-lists")
export class AdminSiteLists extends TailwindElement() {
  @state() private lists: Record<ListKind, ListState> = {
    all: { items: null, error: "", loading: true },
    public: { items: null, error: "", loading: true },
    user: { items: null, error: "", loading: true },
  };

  connectedCallback() {
    super.connectedCallback();
    queueMicrotask(() => this.loadAll());
  }

  private loadAll() {
    for (const kind of Object.keys(listMeta) as ListKind[]) {
      this.loadOne(kind);
    }
  }

  private async loadOne(kind: ListKind) {
    try {
      const data = await fetchJSON<Site[]>(listMeta[kind].url);
      this.lists = {
        ...this.lists,
        [kind]: { items: data ?? [], error: "", loading: false },
      };
    } catch (e: any) {
      this.lists = {
        ...this.lists,
        [kind]: { items: null, error: e?.message ?? String(e), loading: false },
      };
    }
  }

  private renderList(kind: ListKind) {
    const meta = listMeta[kind];
    const { items, error, loading } = this.lists[kind];

    const header = html`
      <div class="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700">
        <div class="flex items-center gap-2">
          <span class="font-semibold text-gray-800 dark:text-gray-100">${meta.title}</span>
          <span class="relative group">
            <svg xmlns="http://www.w3.org/2000/svg" class="w-4 h-4 text-gray-400 cursor-default" fill="currentColor" viewBox="0 0 24 24">
              <path fill-rule="evenodd" clip-rule="evenodd" d="M12 2C6.477 2 2 6.477 2 12s4.477 10 10 10 10-4.477 10-10S17.523 2 12 2zm0 4a1.25 1.25 0 1 1 0 2.5A1.25 1.25 0 0 1 12 6zm-1 4h2v8h-2v-8z" />
            </svg>
            <span class="absolute left-1/2 -translate-x-1/2 bottom-full mb-2 w-56 rounded bg-gray-900 dark:bg-gray-700 px-3 py-2 text-xs text-white opacity-0 pointer-events-none group-hover:opacity-100 transition-opacity z-10 shadow-lg">
              ${meta.info}
            </span>
          </span>
        </div>
        ${items && !loading
        ? html`<span class="text-xs font-medium px-2 py-0.5 rounded-full bg-gray-200 dark:bg-gray-600 text-gray-700 dark:text-gray-200">${items.length}</span>`
        : nothing}
      </div>
    `;

    let body;
    if (loading) {
      body = html`<div class="px-4 py-6 text-sm text-gray-400">Loading…</div>`;
    } else if (error) {
      body = html`<div class="px-4 py-6 text-sm text-red-500">${error}</div>`;
    } else if (!items || items.length === 0) {
      body = html`<div class="px-4 py-6 text-sm text-gray-400">No sites found.</div>`;
    } else {
      const sorted = [...items].sort((a, b) =>
        (a.Name ?? "").localeCompare(b.Name ?? "", undefined, { sensitivity: "base" }),
      );
      body = html`
        <ul class="divide-y divide-gray-100 dark:divide-gray-700">
          ${sorted.map(
        (s) => html`
              <li class="flex items-center justify-between px-4 py-2.5">
                <div>
                  <div class="text-sm font-medium text-gray-800 dark:text-gray-100">${s.Name}</div>
                  <div class="text-xs text-gray-400">Skey: ${s.Skey}</div>
                </div>
                <div class="flex items-center gap-1.5">
                  ${s.Public
            ? html`<span class="text-xs font-medium px-2 py-0.5 rounded-full bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300">Public</span>`
            : nothing}
                  ${s.Perm !== undefined && s.Perm !== 0
            ? html`<span class="text-xs font-medium px-2 py-0.5 rounded-full bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-300">Perm: ${s.Perm}</span>`
            : nothing}
                </div>
              </li>
            `,
      )}
        </ul>
      `;
    }

    return html`
      <div class="rounded-lg border border-gray-200 dark:border-gray-700 bg-white dark:bg-gray-800">
        ${header}
        <div class="overflow-hidden">${body}</div>
      </div>
    `;
  }

  render() {
    return html`
      <div class="max-w-7xl mx-auto px-4 py-6">
        <div class="grid grid-cols-1 lg:grid-cols-3 gap-4">
          ${(Object.keys(listMeta) as ListKind[]).map((k) => this.renderList(k))}
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "admin-site-lists": AdminSiteLists;
  }
}
