import { LitElement, html, css, PropertyValues } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';

export const SandboxSkey: Number = 3;

@customElement('site-menu')
class SiteMenu extends LitElement {
    private _clickListener?: (e: MouseEvent) => void;

    @property({ type: String, attribute: 'selected-data' })
    selectedData;

    // This should contain the permission text for the selected permission.
    @property({ type: String, attribute: 'selected-perm' })
    selectedPerm;

    // If custom-handling is set to be true, the page must handle
    // site-change events, otherwise the site menu will force a page refresh.
    @property({ type: Boolean, attribute: 'custom-handling' })
    customHandling = false;

    @property({ type: Boolean })
    reloadConfirmed = false;

    static styles = css`
        select {
            padding: 8px;
            border: 1px solid #ccc;
            border-radius: 4px;
        }
    `;

    constructor() {
        super();
        this.selectedData = "";
        this.selectedPerm = "";
    }

    override render() {
        return html`
            <select id="select" @change=${this.handleSiteChange}>
                <option id="loading">
                  ${this.selectedData
                ? this.selectedData.split(":")[1]
                : "Select Site"
            }
                </option>
                <optgroup style="display: none" id="read" label="Read"></optgroup>
                <optgroup style="display: none" id="write" label="Write"></optgroup>
                <optgroup style="display: none" id="admin" label="Admin"></optgroup>
            </select>
        `;
    }

    firstUpdated() {
        // Handle init logic for tab site selection
        const urlParams = new URLSearchParams(window.location.search);
        let siteParams = urlParams.get('site');

        if (siteParams) {
            sessionStorage.setItem('site', siteParams);
        } else {
            let sessionSite = sessionStorage.getItem('site');
            if (sessionSite) {
                // Apply the session site by redirecting to include the parameter
                let currentUrl = new URL(window.location.href);
                currentUrl.searchParams.set('site', sessionSite);
                window.location.replace(currentUrl.toString());
                return;
            }
        }

        // If no URL or session site is present, set session to the backend fallback.
        if (!sessionStorage.getItem('site') && this.selectedData) {
            let s = this.selectedData.split(":");
            if (s.length > 0 && containsInt(s[0])) {
                sessionStorage.setItem('site', s[0]);
            }
        }

        this.loadSites();
    }

    async loadSites() {
        var optGroups: HTMLOptGroupElement[] = [];
        optGroups.push(this.renderRoot.querySelector("#read")! as HTMLOptGroupElement)
        optGroups.push(this.renderRoot.querySelector("#write")! as HTMLOptGroupElement)
        optGroups.push(this.renderRoot.querySelector("#admin")! as HTMLOptGroupElement)
        var loading = this.renderRoot.querySelector("#loading")! as HTMLOptionElement

        let r = new XMLHttpRequest();
        r.onreadystatechange = () => {
            if (r.readyState == XMLHttpRequest.DONE) {
                if (r.status !== 200) {
                    console.error("Failed to load sites: ", r.status, r.responseText);
                    return;
                }
                let sites;
                try {
                    sites = JSON.parse(r.response);
                } catch (e) {
                    console.error("Failed to parse sites JSON: ", e);
                    return;
                }
                this.dispatchEvent(new CustomEvent('sites-loaded', { bubbles: true, composed: true, detail: { 'len': sites.length } }))
                var opts: HTMLOptionElement[][] = [[], [], []]
                for (let site of sites) {
                    var opt = document.createElement("option");
                    opt.value = site.Skey
                    opt.label = site.Name
                    opt.setAttribute("perm", site.Perm)
                    if (site.Public) {
                        opt.label += " (Public)"
                        opt.getAttribute("perm") == "0" ? opt.setAttribute("perm", "1") : null;
                    }
                    switch (opt.getAttribute("perm")) {
                        case "1":
                            opts[0].push(opt);
                            break;
                        case "3":
                            opts[1].push(opt);
                            break;
                        case "7":
                            opts[2].push(opt);
                            break;
                    }
                }
                for (let i = 0; i < 3; i++) {
                    if (opts[i].length <= 0) {
                        continue;
                    }
                    opts[i].sort((a, b) => a.label.toLowerCase().localeCompare(b.label.toLowerCase()))
                    optGroups[i].style.display = 'block'
                    opts[i].forEach(option => {
                        this.checkSelected(option, optGroups[i], this.selectedData)
                        optGroups[i].appendChild(option);
                    });
                }
                if (this.selectedData != '') {
                    loading.remove()
                }
            }
        }
        r.open("GET", "/api/v1/sites/user")
        r.send();
    }

    handleSiteChange(event: Event) {
        const target = event.target as HTMLSelectElement;
        const selectedOpt = target.options[target.selectedIndex];
        const selectedName = selectedOpt.label;
        const selectedKey = target.value;
        if (!containsInt(selectedKey)) {
            console.log("invalid key, no site selected");
            return;
        }
        if (this.customHandling) {
            this.dispatchEvent(new CustomEvent('site-change', { bubbles: true, detail: { previousSite: this.selectedData, newSite: selectedKey + ":" + selectedName } }));
            this.selectedData = selectedKey + ":" + selectedName;
            sessionStorage.setItem('site', selectedKey);
            let targetUrl = new URL(window.location.href);
            if (Number(selectedKey) == SandboxSkey) {
                window.location.assign("/admin/sandbox");
                return;
            }
            targetUrl.searchParams.set("site", selectedKey);
            window.location.assign(targetUrl.toString());
        }

        if (selectedOpt.slot != this.selectedPerm) {
            this.selectedPerm = selectedOpt.hasAttribute("perm") ? selectedOpt.getAttribute("perm")! : "0";
            console.log(this.selectedPerm)
            this.dispatchEvent(new CustomEvent('permission-change', { bubbles: true, composed: true, detail: { selectedPerm: this.selectedPerm } }));
        }
    }

    // checkSelected compares the option's key to the profile's selected site key.
    // If it's a match the option is selected.
    checkSelected(option: HTMLOptionElement, optGroup: HTMLOptGroupElement, data: string) {
        let key = Number(option.value);
        let s = data.split(":");
        option.selected = Number(s[0]) == key;
        if (option.selected) {
            this.selectedPerm = option.hasAttribute("perm") ? option.getAttribute("perm")! : "0";
            console.log(this.selectedPerm)
            this.dispatchEvent(new CustomEvent('permission-change', { bubbles: true, composed: true, detail: { selectedPerm: this.selectedPerm } })); //TODO: only trigger if changed.
            option.innerText = s[1];
        }
    }

    connectedCallback() {
        super.connectedCallback();

        // Intercept clicks on links to ensure they carry over the site parameter if present.
        this._clickListener = (e: MouseEvent) => {
            const anchor = (e.target as Element).closest('a');
            if (anchor && anchor.href && anchor.origin === window.location.origin) {
                // Some links shouldn't have site appended
                if (anchor.getAttribute('href')?.startsWith('mailto:') || anchor.getAttribute('href')?.startsWith('tel:')) return;
                
                const siteKey = sessionStorage.getItem('site');
                if (siteKey) {
                    try {
                        let url = new URL(anchor.href);
                        if (!url.searchParams.has('site') && !url.pathname.startsWith('/api/')) {
                            url.searchParams.set('site', siteKey);
                            anchor.href = url.toString();
                        }
                    } catch (err) {}
                }
            }
        };
        document.addEventListener('click', this._clickListener);
    }

    disconnectedCallback() {
        super.disconnectedCallback();

        if (this._clickListener) {
            document.removeEventListener('click', this._clickListener);
        }
    }
}

// containsInt uses a regular expression to match unsigned integers, returning a boolean.
// This is used to check if a valid site key is being set.
function containsInt(input: string): boolean {
    const integerRegex = /^\d+$/;
    return integerRegex.test(input);
}

declare global {
    interface HTMLElementTagNameMap {
        'site-menu': SiteMenu;
    }
}
