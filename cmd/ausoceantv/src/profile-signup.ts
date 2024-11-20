import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";

@customElement("profile-signup")
export class ProfileSignup extends TailwindElement() {
  @property({ type: Boolean })
  auth = false;

  @property({ type: String })
  name = "";

  constructor() {
    super();
    fetch("/auth/getprofile")
      .then((res) => res.json())
      .then((profile) => {
        console.log("profile:", profile);
        if (profile.er != undefined) {
          this.auth = false;
        } else {
          this.auth = true;
          this.name = profile.GivenName;
        }
      });

    console.log(this.auth);
  }

  render() {
    return this.auth ? html` <a href="home">Hi ${this.name}</a>` : html`<a href="/auth/login">Signup</a>`;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "profile-signup": ProfileSignup;
  }
}
