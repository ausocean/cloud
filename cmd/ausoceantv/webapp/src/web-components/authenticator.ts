import { html } from "lit";
import { customElement, state } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";
import { provide } from "@lit/context";
import { userContext } from "../utils/context.ts";
import { User } from "../types/user.ts";

@customElement("auth-wrapper")
export class Authenticator extends TailwindElement() {
  @provide({ context: userContext })
  @state()
  user: User = new User();

  async connectedCallback() {
    super.connectedCallback();

    await fetch("/api/v1/auth/profile")
      .then(async (resp) => {
        if (!resp.ok) {
          const error = await resp.json();
          throw resp.statusText + ": " + error.message;
        }
        return resp.json();
      })
      .then((resp) => {
        this.user = new User();
        this.user.name = resp.GivenName + " " + resp.FamilyName;
        this.user.email = resp.Email;
      })
      .catch((err) => {
        console.warn("Error fetching profile:", err);

        // Send Non-Authenticated users to the index page.
        if (window.location.pathname != "/") {
          window.location.assign("/");
        }
      });
  }

  render() {
    return html`
      <slot></slot>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "auth-wrapper": Authenticator;
  }
}
