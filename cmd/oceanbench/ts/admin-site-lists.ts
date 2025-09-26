// ts/admin-site-lists.ts
import { LitElement, html, nothing } from "lit";
import { customElement, state } from "lit/decorators.js";

type Site = {
  Skey: number;
  Name: string;
  Public?: boolean;
  Perm?: number;
};

function fetchJSONWithTimeout<T>(url: string, ms = 10000): Promise<T> {
  const ac = new AbortController();
  const id = setTimeout(() => ac.abort(), ms);
  return fetch(url, { credentials: "same-origin", signal: ac.signal })
    .then(async (r) => {
      const ct = r.headers.get("content-type") || "";
      const bodyText = await r
        .clone()
        .text()
        .catch(() => "");
      console.info("[admin-site-lists] GET", url, r.status, r.statusText, ct, bodyText.slice(0, 200));
      if (!r.ok) throw new Error(`${r.status} ${r.statusText} — ${bodyText}`);
      return JSON.parse(bodyText) as T;
    })
    .finally(() => clearTimeout(id));
}

@customElement("admin-site-lists")
export class AdminSiteLists extends LitElement {
  // Light DOM so global CSS applies.
  protected createRenderRoot() {
    return this;
  }

  @state() private allSites: Site[] | null = null;
  @state() private publicSites: Site[] | null = null;
  @state() private userSites: Site[] | null = null;

  @state() private errorAll = "";
  @state() private errorPublic = "";
  @state() private errorUser = "";

  @state() private loadingAll = true;
  @state() private loadingPublic = true;
  @state() private loadingUser = true;

  connectedCallback() {
    super.connectedCallback();
    // Defer to microtask to ensure element is fully upgraded before async work.
    queueMicrotask(() => this.loadAll());
  }

  private async loadAll() {
    // Kick off all three in parallel.
    this.loadOne("/api/v1/sites/all", "all");
    this.loadOne("/api/v1/sites/public", "public");
    this.loadOne("/api/v1/sites/user", "user");
  }

  private async loadOne(url: string, kind: "all" | "public" | "user") {
    try {
      const data = await fetchJSONWithTimeout<Site[]>(url, 15000);
      switch (kind) {
        case "all":
          this.allSites = data ?? [];
          this.loadingAll = false;
          this.requestUpdate("allSites");
          this.requestUpdate("loadingAll");
          console.log("[admin-site-lists] allSites loaded:", this.allSites.length);
          break;
        case "public":
          this.publicSites = data ?? [];
          this.loadingPublic = false;
          this.requestUpdate("publicSites");
          this.requestUpdate("loadingPublic");
          console.log("[admin-site-lists] publicSites loaded:", this.publicSites.length);
          break;
        case "user":
          this.userSites = data ?? [];
          this.loadingUser = false;
          this.requestUpdate("userSites");
          this.requestUpdate("loadingUser");
          console.log("[admin-site-lists] userSites loaded:", this.userSites.length);
          break;
      }
    } catch (e: any) {
      const msg = e?.message ?? String(e);
      switch (kind) {
        case "all":
          this.errorAll = msg;
          this.loadingAll = false;
          this.requestUpdate("errorAll");
          this.requestUpdate("loadingAll");
          console.warn("[admin-site-lists] allSites error:", msg);
          break;
        case "public":
          this.errorPublic = msg;
          this.loadingPublic = false;
          this.requestUpdate("errorPublic");
          this.requestUpdate("loadingPublic");
          console.warn("[admin-site-lists] publicSites error:", msg);
          break;
        case "user":
          this.errorUser = msg;
          this.loadingUser = false;
          this.requestUpdate("errorUser");
          this.requestUpdate("loadingUser");
          console.warn("[admin-site-lists] userSites error:", msg);
          break;
      }
    }
  }

  private renderList(title: string, items: Site[] | null, loading: boolean, err: string) {
    if (loading) {
      return html`
        <div class="card mb-3">
          <div class="card-header">${title}</div>
          <div class="card-body">
            <div class="text-muted">Loading…</div>
          </div>
        </div>
      `;
    }
    if (err) {
      return html`
        <div class="card mb-3 border-danger">
          <div class="card-header">${title}</div>
          <div class="card-body text-danger">${err}</div>
        </div>
      `;
    }
    if (!items || items.length === 0) {
      return html`
        <div class="card mb-3">
          <div class="card-header">${title}</div>
          <div class="card-body text-muted">No sites found.</div>
        </div>
      `;
    }
    const sorted = [...items].sort((a, b) => (a.Name || "").toLowerCase().localeCompare((b.Name || "").toLowerCase()));
    return html`
      <div class="card mb-3">
        <div class="card-header d-flex justify-content-between align-items-center">
          <span>${title}</span>
          <span class="badge bg-secondary">${sorted.length}</span>
        </div>
        <div class="list-group list-group-flush">
          ${sorted.map(
            (s) => html`
              <div class="list-group-item d-flex flex-column flex-sm-row justify-content-between align-items-start align-items-sm-center">
                <div>
                  <div class="fw-semibold">${s.Name}</div>
                  <div class="text-muted small">Skey: ${s.Skey}</div>
                </div>
                <div class="mt-2 mt-sm-0">
                  ${s.Public
                    ? html`
                        <span class="badge bg-success">Public</span>
                      `
                    : nothing}
                  ${s.Perm !== undefined
                    ? html`
                        <span class="badge bg-info ms-2">Perm: ${s.Perm}</span>
                      `
                    : nothing}
                </div>
              </div>
            `,
          )}
        </div>
      </div>
    `;
  }

  render() {
    return html`
      <div class="container-md my-3">
        <div class="row">
          <div class="col-12 col-lg-4">${this.renderList("All Sites", this.allSites, this.loadingAll, this.errorAll)}</div>
          <div class="col-12 col-lg-4">${this.renderList("Public Sites", this.publicSites, this.loadingPublic, this.errorPublic)}</div>
          <div class="col-12 col-lg-4">${this.renderList("Your Sites", this.userSites, this.loadingUser, this.errorUser)}</div>
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
