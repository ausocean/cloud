import { LitElement, html, css } from 'lit';
import { customElement, property } from 'lit/decorators.js';
import { NavMenu } from './nav-menu.js';
import '../s/lit/nav-menu.js';
import '../s/lit/site-menu.js';
@customElement('header-group')
class HeaderGroup extends LitElement {

    @property({ type: Number, attribute: 'selected-perm' })
    selectedPerm;
    @property({ type: String, attribute: 'version' })
    version;
    @property({ type: String, attribute: 'auth' })
    auth;
    @property({ type: String })
    logoutURL;

    static styles = css`
        :host {
            display: block;
            width: 100%;
        }

        #top-bar {
            padding-inline: 100px;
            background-color: var(--primary-blue);
            position: fixed;
            top: 0px;
            left: 0px;
            box-sizing: border-box;
            width: 100%;
            display: flex;
            gap: 50px;
            align-items: center;
            height: 60px;
            z-index: 1001;
        }

        #logout {
            position: fixed;
            height: 60px;
            right: 12px;
            top: 0px;
            z-index: 1002;
            line-height:60px;
        }

        a {
            text-decoration: none;
            color: white;
        }

        #title {
            color: white;
        }
    `;

    constructor() {
        super();
        this.selectedPerm = 1;
        this.version = "0";
        this.auth = false;
        this.logoutURL = "/logout?redirect=/";
    }

    override render() {
        return this.auth
            ? html`
                <slot name="nav-menu"></slot>
                <div id="top-bar">
                    <h1 id="title">CloudBlue</h1>
                    <slot @permission-change=${this._onPermissionChange} id="site-menu" name="site-menu"></slot>
                </div>
                <a id="logout" href="${this.logoutURL}">Log out</a>
                
            `
            : html`
                <slot name="nav-menu"></slot>
                <div id="top-bar">
                    <h1 id="title">CloudBlue</h1>
                    <slot id="site-menu" name="site-menu"></slot>
                </div>
            `
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