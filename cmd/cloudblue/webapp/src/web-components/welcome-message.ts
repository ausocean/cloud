import { html } from "lit";
import { customElement } from "lit/decorators.js";
import { consume } from "@lit/context";
import { TailwindElement } from "../shared/tailwind.element";
import { userContext } from "../utils/context";
import { User } from "../types/user";

@customElement("welcome-message")
export class WelcomeMessage extends TailwindElement() {
    @consume({ context: userContext, subscribe: true })
  user!: User;

  render() {
    if (!this.user?.email) {
      return html`
        <p class="text-gray-500">Loading user info...</p>
      `;
    }

    return html`
      <p>
        You are logged in as
        <strong>${this.user.email}</strong>
        .
      </p>
    `;
  }
}
