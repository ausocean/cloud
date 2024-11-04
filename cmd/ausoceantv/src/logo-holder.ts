import { LitElement, css, html } from 'lit'
import { customElement } from 'lit/decorators.js'

@customElement('logo-holder')
export class LogoHolder extends LitElement {
  render() {
    return html`
      <div>
        <a href="https://ausocean.org" target="_blank">
          <img src="src/assets/ausocean_logo.png" class="logo" alt="AusOcean logo" />
        </a>
      </div>
      <slot></slot>
    `
  }

  static styles = css`
    :host {
      max-width: 1280px;
      margin: 0 auto;
      padding: 2rem;
      text-align: center;
    }

    .logo {
      height: 6em;
      padding: 1.5em;
      will-change: filter;
      transition: filter 300ms;
    }
    .logo:hover {
      filter: drop-shadow(0 0 2em #646cffaa);
    }

    ::slotted(h1) {
      font-size: 3.2em;
      line-height: 1.1;
    }

    a {
      font-weight: 500;
      color: #646cff;
      text-decoration: inherit;
    }
    a:hover {
      color: #535bf2;
    }

    @media (prefers-color-scheme: light) {
      a:hover {
        color: #747bff;
      }
      button {
        background-color: #f9f9f9;
      }
    }
  `
}

declare global {
  interface HTMLElementTagNameMap {
    'logo-holder': LogoHolder
  }
}
