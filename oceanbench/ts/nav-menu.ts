import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('nav-menu')
export class NavMenu extends LitElement {

    @property({ type: Number, attribute: 'selected-perm' })
    selectedPerm;

    initialised;

    static styles = css`
        #menubars { display: inline-block; cursor: pointer; position: fixed; top: 5px; left: 0; }
        #menu { list-style-type: none; padding: 0; margin: 0; }
        nav { font-family: "Open Sans", sans-serif; font-size: 16px; font-weight: bold; letter-spacing: 1px; background-color: #2F4F7F; position: fixed; top: 0; left: 0; z-index: 1000}
        nav ul li { text-align: left; padding: 0 0 0 32px; }
        nav ul li a { display: none; padding: 10px; text-transform: uppercase; text-decoration: none; }
        nav ul li.indent-1 { padding: 0 0 0 42px; }
        nav ul li.indent-1 a { text-transform: capitalize; }
        nav ul.expanded li a { display: block; color: #99AABB }
        nav ul li a.selected { display: block; color: white }
        nav ul li a:hover { color: #009933 }
        .menubar { width: 30px; height: 4px; background-color: #99AABB; margin: 3px 8px; }
    `;

    constructor() {
        super();
        this.selectedPerm = 0;
        this.initialised = false;
    }

    connectedCallback() {
        super.connectedCallback();
        document.addEventListener('click', (e) => {
            if (!this.contains(e.target as Node)) {
                const menu = this.shadowRoot?.querySelector('#menu') as HTMLUListElement;
                if (menu.className == "") {
                    return;
                }
                this.toggleMenu();
            };
        });
    }

    override render() {
        return html`
            <div style="display: none">
                <slot @slotchange="${this.addItems}"></slot>
            </div>
            <nav id="nav">
                <div id="menubars" @click=${this.toggleMenu}>
                    <div class="menubar"></div>
                    <div class="menubar"></div>
                    <div class="menubar"></div>
                </div>
                <ul id="menu">
                </ul>
            </nav>
        `;
    }

    public setPerm(p: number){
        this.selectedPerm = p;
        const ul = this.shadowRoot?.querySelector('#menu') as HTMLUListElement;
        const items = Array.from(ul.querySelectorAll('li'));
        for (const item of items) {
            const perm = Number(item.dataset && item.dataset['perm']);
            if(perm && (perm & this.selectedPerm) != 0){
                item.hidden = false;
                item.style.display = 'block';
            } else {
                item.hidden = true;
                item.style.display = 'none';
            }
        }
        this.render();
    }

    // addItems copies the li elements from the slot to the ul element.
    // This is done because slot can't be used directly inside of a ul.
    async addItems(e: Event) {
        const slot = e.target as HTMLSlotElement;
        const items = slot.assignedElements() as HTMLLIElement[];
        const ul = this.shadowRoot?.querySelector('#menu') as HTMLUListElement;
        for (const item of items) {
            ul.appendChild(item);
        }
    }

    // toggleMenu toggles the main menu. It looks for a selected class on the first child of each list item.
    toggleMenu() {
        // Get the ul element that nav-menu renders.
        const menu = this.shadowRoot?.querySelector('#menu') as HTMLUListElement;
        if (menu == null){
            return;
        }
        var expand = true;
        if (menu.classList.contains('expanded')) {
            menu.classList.remove('expanded');
            expand = false;
        } else {
            menu.classList.add('expanded');
        }
        var list = menu.getElementsByTagName("li");
        for (var ii = 0; ii < list.length; ii++) {
            var item = list[ii];
            if (expand && !item.hidden) {
                item.style.display = 'block';
            } else {
                if (item.firstChild instanceof Element && item.firstChild?.className == 'selected') {
                    item.style.display = 'block';
                } else {
                    item.style.display = 'none';
                }
            }
        }
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'nav-menu': NavMenu;
    }
}