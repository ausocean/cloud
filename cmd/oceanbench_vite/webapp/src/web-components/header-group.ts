import { LitElement, html, css } from "lit";
import { customElement, property } from "lit/decorators.js";
import { NavMenu } from "./nav-menu.ts";
import "./nav-menu.ts";
import "./site-menu.ts";
@customElement("header-group")
class HeaderGroup extends LitElement {
  @property({ type: Number, attribute: "selected-perm" })
  selectedPerm;
  @property({ type: String, attribute: "version" })
  version;
  @property({ type: String, attribute: "auth" })
  auth;
  @property({ type: String })
  logoutURL;

  static styles = css`
    :host {
      display: block;
      width: 100%;
    }

    #top-bar {
      background-color: var(--primary-blue);
      position: fixed;
      top: 0px;
      left: 0px;
      box-sizing: border-box;
      display: flex;
      @media (max-width: 900px) {
        flex-direction: column;
        padding-top: 5px;
        padding-bottom: 10px;
      }
      @media (min-width: 900px) {
        gap: 50px;
        height: 60px;
        padding-inline: 100px;
      }
      width: 100%;
      align-items: center;
      z-index: 1001;
    }

    #logout {
      position: fixed;
      height: 60px;
      right: 12px;
      top: 0px;
      z-index: 1002;
      line-height: 60px;
    }

    a {
      text-decoration: none;
      color: white;
    }

    #title {
      color: white;
      margin: 0px;
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
            <a href="/"><h1 id="title">CloudBlue</h1></a>
            <slot @permission-change=${this._onPermissionChange} id="site-menu" name="site-menu"></slot>
          </div>
          <a id="logout" href="${this.logoutURL}">Log out</a>
        `
      : html`
          <div id="top-bar">
            <a href="/"><h1 id="title">CloudBlue</h1></a>
          </div>
        `;
  }

  override firstUpdated() {
    this._applyPermission(this.selectedPerm);
  }

  private _onPermissionChange(event: Event) {
    const customEvent = event as CustomEvent;
    this.selectedPerm = customEvent.detail.selectedPerm;
    this._applyPermission(this.selectedPerm);
  }

  private _applyPermission(permission: number) {
    const slot = this.shadowRoot?.querySelector('slot[name="nav-menu"]') as HTMLSlotElement;
    const elements = slot.assignedElements();
    const nav = elements.find((element) => element.id === "nav-menu") as NavMenu;
    if (nav) {
      nav.setPerm(permission); // Apply permission to nav menu
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "header-group": HeaderGroup;
  }
}
