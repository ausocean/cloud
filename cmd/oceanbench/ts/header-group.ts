import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { NavMenu } from './nav-menu.js';
import '../s/lit/nav-menu.js';
import '../s/lit/site-menu.js';

@customElement('header-group')
class HeaderGroup extends LitElement {

    @property({ type: Number, attribute: 'selected-perm' })
    selectedPerm;

    static styles = css`
        :host {
            display: block;
            width: 100%;
        }
    `;

    constructor() {
        super();
        this.selectedPerm = 1;
    }

    override render() {
        return html`
            <slot @permission-change=${this._onPermissionChange}></slot>
        `;
    }

    private _onPermissionChange(event: Event){
        const customEvent = event as CustomEvent;
        this.selectedPerm = customEvent.detail.selectedPerm;
        const slot = this.shadowRoot?.querySelector('slot') as HTMLSlotElement;
        const elements = slot.assignedElements();
        const nav = elements.find(element => element.id === "nav-menu") as NavMenu;
        nav.setPerm(this.selectedPerm);
    }
}

declare global {
    interface HTMLElementTagNameMap {
        'header-group': HeaderGroup;
    }
}