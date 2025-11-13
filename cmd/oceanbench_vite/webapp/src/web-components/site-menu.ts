import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";

export const SandboxSkey: Number = 3;

@customElement("site-menu")
class SiteMenu extends LitElement {
  @property({ type: String, attribute: "selected-data" })
  selectedData;

  // This should contain the permission text for the selected permission.
  @property({ type: String, attribute: "selected-perm" })
  selectedPerm;

  // If custom-handling is set to be true, the page must handle
  // site-change events, otherwise the site menu will force a page refresh.
  @property({ type: Boolean, attribute: "custom-handling" })
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
        <option id="loading">${this.selectedData ? this.selectedData.split(":")[1] : "Select Site"}</option>
        <optgroup style="display: none" id="read" label="Read"></optgroup>
        <optgroup style="display: none" id="write" label="Write"></optgroup>
        <optgroup style="display: none" id="admin" label="Admin"></optgroup>
      </select>
    `;
  }

  firstUpdated() {
    this.loadSites();
  }

  async loadSites() {
    var optGroups: HTMLOptGroupElement[] = [];
    optGroups.push(this.renderRoot.querySelector("#read")! as HTMLOptGroupElement);
    optGroups.push(this.renderRoot.querySelector("#write")! as HTMLOptGroupElement);
    optGroups.push(this.renderRoot.querySelector("#admin")! as HTMLOptGroupElement);
    var loading = this.renderRoot.querySelector("#loading")! as HTMLOptionElement;

    // Make a request to /api/get/sites/user
    let r = new XMLHttpRequest();
    r.onreadystatechange = () => {
      if (r.readyState == XMLHttpRequest.DONE) {
        let sites = JSON.parse(r.response);
        this.dispatchEvent(new CustomEvent("sites-loaded", { bubbles: true, composed: true, detail: { len: sites.length } }));
        var opts: HTMLOptionElement[][] = [[], [], []];
        for (let site of sites) {
          var opt = document.createElement("option");
          opt.value = site.Skey;
          opt.label = site.Name;
          opt.setAttribute("perm", site.Perm);
          if (site.Public) {
            opt.label += " (Public)";
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
          opts[i].sort((a, b) => a.label.toLowerCase().localeCompare(b.label.toLowerCase()));
          optGroups[i].style.display = "block";
          opts[i].forEach((option) => {
            this.checkSelected(option, this.selectedData);
            optGroups[i].appendChild(option);
          });
        }
        if (this.selectedData != "") {
          loading.remove();
        }
      }
    };
    r.open("GET", "/api/get/sites/user");
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
      this.dispatchEvent(new CustomEvent("site-change", { bubbles: true, detail: { previousSite: this.selectedData, newSite: selectedKey + ":" + selectedName } }));
      this.selectedData = selectedKey + ":" + selectedName;
    } else {
      let r = new XMLHttpRequest();
      r.onreadystatechange = () => {
        if (r.readyState == XMLHttpRequest.DONE) {
          console.log("response from set site request: ", r.responseText);
          if (Number(selectedKey) == SandboxSkey) {
            window.location.assign("/admin/sandbox");
            return;
          }
          window.location.reload();
        }
      };
      r.open("GET", "/api/set/site/" + selectedKey + ":" + selectedName, true);
      r.send();
    }

    if (selectedOpt.slot != this.selectedPerm) {
      this.selectedPerm = selectedOpt.hasAttribute("perm") ? selectedOpt.getAttribute("perm")! : "0";
      console.log(this.selectedPerm);
      this.dispatchEvent(new CustomEvent("permission-change", { bubbles: true, composed: true, detail: { selectedPerm: this.selectedPerm } }));
    }
  }

  // checkSelected compares the option's key to the profile's selected site key.
  // If it's a match the option is selected.
  checkSelected(option: HTMLOptionElement, data: string) {
    let key = Number(option.value);
    let s = data.split(":");
    option.selected = Number(s[0]) == key;
    if (option.selected) {
      this.selectedPerm = option.hasAttribute("perm") ? option.getAttribute("perm")! : "0";
      console.log(this.selectedPerm);
      this.dispatchEvent(new CustomEvent("permission-change", { bubbles: true, composed: true, detail: { selectedPerm: this.selectedPerm } })); //TODO: only trigger if changed.
      option.innerText = s[1];
    }
  }

  connectedCallback() {
    super.connectedCallback();

    // Add event listener for tab change.
    document.addEventListener("visibilitychange", this.checkSiteChange.bind(this));
    window.addEventListener("focus", this.checkSiteChange.bind(this));
  }

  disconnectedCallback() {
    super.disconnectedCallback();

    // Remove tab event listener when the element is disconnected from the DOM.
    document.removeEventListener("visibilitychange", this.checkSiteChange.bind(this));
    window.removeEventListener("focus", this.checkSiteChange.bind(this));
  }

  // Check if the user's selected site is different compared to the menu.
  // This can happen if the site is changed in another tab.
  checkSiteChange() {
    // Check if the tab is visible.
    if (!document.hidden) {
      console.log("Checking if the site has changed...");
      let r = new XMLHttpRequest();
      r.onreadystatechange = () => {
        if (r.readyState == XMLHttpRequest.DONE) {
          if (r.status == 200) {
            // Compare the newly fetched selected site key with the menu's selected site key.
            const currentData = r.responseText;
            let s1 = currentData.split(":");
            let s2 = this.selectedData.split(":");
            // If there's not a match, and no custom handling, ask the user if they want to reload.
            // If the user clicks 'OK' in the confirmation dialog, reload the page.
            if (Number(s1[0]) != Number(s2[0])) {
              let prevSite = this.selectedData;
              this.selectedData = currentData;
              if (window.confirm("The selected site has changed from " + s2[1] + " to " + s1[1] + ". Do you want to load the new site page? Unsaved changes may be lost.")) {
                if (this.customHandling) {
                  this.dispatchEvent(new CustomEvent("site-change", { bubbles: true, detail: { previousSite: prevSite, newSite: this.selectedData } }));
                } else {
                  window.location.reload();
                }
              } else {
                let r = new XMLHttpRequest();
                r.onreadystatechange = () => {
                  if (r.readyState == XMLHttpRequest.DONE) {
                    console.log("response from set site request: ", r.responseText);
                  }
                };
                r.open("GET", "/api/set/site/" + prevSite, true);
                r.send();
              }
            }
          } else {
            console.log("bad response from 'get profile data' request: ", r.responseText, r.readyState, r.status);
          }
        }
      };
      r.open("GET", "/api/get/profile/data", true);
      r.send();
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
    "site-menu": SiteMenu;
  }
}
