import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';

@customElement('nav-menu')
export class NavMenu extends LitElement {

    @property({ type: Number, attribute: 'selected-perm' })
    selectedPerm;

    initialised;

    static styles = css`
        #menu { list-style-type: none; padding: 0; margin: 0; }
        nav { font-family: "Open Sans", sans-serif; font-size: 16px; font-weight: bold; letter-spacing: 1px; background-color: var(--ao-blue); position: fixed; top: -468px; left: 0; z-index: 1000; transition-duration: 300ms; animation-timing-function: ease-in-out}
        nav ul li { text-align: left; padding: 0 0 0 12px; z-index: 998}
        nav ul li a { display: none; padding: 10px; text-transform: uppercase; text-decoration: none; }
        nav ul li.indent-1 { padding: 0 0 0 24px; }
        nav ul li.indent-1 a { text-transform: capitalize; }
        nav ul.expanded li a { display: block; color: white }
        nav ul li a.selected { display: block; color: #fbae16 }
        nav ul li a:hover { color: #fbae16 }
        #menubars { display: inline-block; cursor: pointer; position: fixed; top: 18px; left: 12px; z-index: 1002}
        .menubar { width: 30px; height: 4px; background-color: white; margin: 3px 8px; }
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
            <div>
                <slot @slotchange="${this.addItems}"></slot>
            </div>
            <div id="menubars" @click=${this.toggleMenu}>
                <div class="menubar"></div>
                <div class="menubar"></div>
                <div class="menubar"></div>
            </div>
            <nav id="nav">
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
            if(perm & this.selectedPerm){
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
            const perm = Number(item.dataset && item.dataset['perm']);
            if(perm & this.selectedPerm){
                item.hidden = false;
                item.style.display = 'block';
            } else {
                item.hidden = true;
                item.style.display = 'none';
            }
        }
    }

    // toggleMenu toggles the main menu. It looks for a selected class on the first child of each list item.
    toggleMenu() {
        let nav = this.shadowRoot?.querySelector('nav') as HTMLElement;
        // Get the ul element that nav-menu renders.
        const menu = this.shadowRoot?.querySelector('#menu') as HTMLUListElement;
        if (menu == null){
            return;
        }
        var expand = true;
        if (menu.classList.contains('expanded')) {
            menu.classList.remove('expanded');
            nav.style.top = '-468px';
            expand = false;
        } else {
            menu.classList.add('expanded');
            nav.style.top = '60px';
        }
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'nav-menu': NavMenu;
    }
}