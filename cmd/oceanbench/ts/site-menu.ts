import { LitElement, html, css } from 'lit';
import { customElement, property, query } from 'lit/decorators.js';

@customElement('site-menu')
class SiteMenu extends LitElement {

    @property({ type: String, attribute: 'selected-data' })
    selectedData;

    // This should contain the permission text for the selected permission.
    @property({ type: String, attribute: 'selected-perm' })
    selectedPerm;

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
            <div style="display: none">
                <slot @slotchange="${this.addOptions}" name="Read Only"></slot>
                <slot @slotchange="${this.addOptions}" name="Read Write"></slot>
                <slot @slotchange="${this.addOptions}" name="Admin"></slot>
            </div>
            <select id="select" @change=${this.handleSiteChange}>
                <option>select a site</option>
                <optgroup style="display: none" title="Read Only" label="Read Only - loading..."></optgroup>
                <optgroup style="display: none" title="Read Write" label="Read Write - loading..."></optgroup>
                <optgroup style="display: none" title="Admin" label="Admin - loading..."></optgroup>
            </select>
        `;
    }

    // addOptions copies the option elements from their slots to the optgroups.
    // This is done because slot can't be used directly inside of an optgroup.
    async addOptions(e: Event) {

        // Get option elements from HTML document.
        const slot = e.target as HTMLSlotElement;
        const options = slot.assignedNodes() as HTMLOptionElement[];

        // Get the select element that site-menu renders.
        const select = this.shadowRoot?.querySelector('#select') as HTMLSelectElement;

        // Find the optgroup that matches the slot we're adding from.
        const optgroup = select?.querySelector(`[title="${slot.name}"]`) as HTMLOptGroupElement;

        // Load site options into their optgroup.
        this.loadSites(options, optgroup);
    }

    async loadSites(options: HTMLOptionElement[], optgroup: HTMLOptGroupElement) {

        // Show optgroup since it's not going to be empty.
        optgroup.style.display = "block";

        // For each option, check if it is selected and load its site name.
        var loaded = 0;
        for (const option of options) {
            this.checkSelected(option, optgroup, this.selectedData);
            const r = new XMLHttpRequest();
            r.open("GET", "/api/get/site/" + option.value.toString(), true);
            r.onreadystatechange = function () {
                if (this.readyState == 4 && this.status == 200) {
                    const site = JSON.parse(this.responseText);
                    option.innerText = site.Name;
                    if (site.Public) {
                        option.innerText += ' (public)';
                    }
                    loaded++;

                    // When we've loaded all the options for this group, sort and show the options.
                    if (loaded == options.length) {
                        options.sort((a, b) => a.innerHTML.toLowerCase().localeCompare(b.innerHTML.toLowerCase()));

                        // Clear existing selected option from menu to be replaced with updated selected option.
                        optgroup.innerHTML = '';
                        options.forEach(opt => {
                            opt.style.display = "block";
                            optgroup.appendChild(opt);
                        });
                        optgroup.label = optgroup.title;
                    }
                }
            };
            r.send();
        }
    }

    handleSiteChange(event: Event) {
        const target = event.target as HTMLSelectElement;
        const selectedOpt = target.options[target.selectedIndex]
        const selectedName = selectedOpt.label;
        const selectedKey = target.value;
        if (!containsInt(selectedKey)) {
            console.log("invalid key, no site selected");
            return;
        }
        let r = new XMLHttpRequest();
        r.onreadystatechange = function () {
            if (r.readyState == XMLHttpRequest.DONE) {
                console.log("response from set site request: ", r.responseText);
                window.location.reload();
            }
        }
        r.open("GET", "/api/set/site/" + selectedKey + ":" + selectedName, true);
        r.send();

        if (selectedOpt.slot != this.selectedPerm){
            this.selectedPerm = selectedOpt.slot;
            this.dispatchEvent(new CustomEvent('permission-change', {bubbles: true, composed: true, detail: {selectedPerm: permNumber.get(this.selectedPerm)}}));
        }
    }

    // checkSelected compares the option's key to the profile's selected site key.
    // If it's a match the option is selected.
    checkSelected(option: HTMLOptionElement, optGroup: HTMLOptGroupElement, data: string) {
        let key = Number(option.value);
        let s = data.split(":");
        option.selected = Number(s[0]) == key;
        if (option.selected) {
            this.selectedPerm = option.slot;
            this.dispatchEvent(new CustomEvent('permission-change', {bubbles: true, composed: true, detail: {selectedPerm: permNumber.get(this.selectedPerm)}})); //TODO: only trigger if changed.
            option.innerText = s[1];
            option.style.display = "block";

            // Clone and append the option so we don't trigger a slotchange event.
            const clonedOption = option.cloneNode(true) as HTMLOptionElement;
            optGroup.appendChild(clonedOption);
            clonedOption.selected = true;
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
                        // If there's not a match, ask the user if they want to reload.
                        // If the user clicks 'OK' in the confirmation dialog, reload the page.
                        if (Number(s1[0]) != Number(s2[0])) {
                            if (!this.reloadConfirmed && window.confirm("The selected site has changed. Do you want to reload the page?")) {
                                this.reloadConfirmed = true;
                                window.location.reload();
                            }
                        }
                    } else {
                        console.log("bad response from 'get profile data' request: ", r.responseText, r.readyState, r.status);
                    }
                }
            }
            r.open("GET", "/api/get/profile/data", true);
            r.send();
        }
    }
}

const permNumber = new Map<string, number>([
    ["Read Only", 1],
    ["Read Write", 3],
    ["Admin", 7],
]);

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