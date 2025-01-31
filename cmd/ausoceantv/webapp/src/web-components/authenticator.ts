import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";
import { provide } from "@lit/context";
import { userContext } from "../utils/context.ts";
import { User } from "../types/user.ts";

@customElement("auth-wrapper")
export class Authenticator extends TailwindElement() {
  @provide({ context: userContext })
  @property({ type: Object })
  user: User | null = null;

  async connectedCallback() {
    super.connectedCallback();

    await fetch("/api/v1/auth/profile")
      .then((resp) => {
        if (resp.status != 200) {
          throw resp.status;
        }
        return resp.json();
      })
      .then((resp) => {
        this.user = new User();
        this.user.name = resp.GivenName;
        console.log(this.user.name);
      })
      .catch((err) => {
        if (err == 401) {
          console.log("No session found");
        } else {
          console.log("Error fetching profile:", err);
        }

        // Send Non-Authenticated users to the index page.
        if (window.location.pathname != "/") {
          window.location.assign("/");
        }
      });
  }

  render() {
    return html` <slot></slot> `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "auth-wrapper": Authenticator;
  }
}
