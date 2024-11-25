import { html } from "lit";
import { customElement } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";
import { consume } from "@lit/context";
import { userContext } from "../utils/context.ts";
import type { User } from "../types/user.ts";

// Since this relies on the authenticator, make sure it is imported first.
import "./authenticator.ts";

@customElement("profile-login")
export class ProfileLogin extends TailwindElement() {
  @consume({ context: userContext, subscribe: true })
  user: User | null = null;

  render() {
    let link = "/api/v1/auth/login";
    let buttonText = "Sign Up";

    if (this.user != null) {
      link = "/home";
      buttonText = "Dive In ";
    }

    return html`
      <a class="rounded bg-gray-300 px-5 py-3 text-center" href="${link}">
        ${buttonText}
      </a>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "profile-login": ProfileLogin;
  }
}
