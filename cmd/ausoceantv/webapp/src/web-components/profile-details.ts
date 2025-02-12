import { html } from "lit";
import { customElement, state } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element";
import { userContext } from "../utils/context.ts";
import { consume } from "@lit/context";
import { User } from "../types/user.ts";
import { Subscription } from "../types/subscription.ts";

const textRed = "text-red-600";
const textGreen = "text-green-600";

@customElement("profile-details")
export class ProfileDetails extends TailwindElement() {
  @consume({ context: userContext, subscribe: true })
  @state()
  user: User = { name: "loading...", email: "loading..." };

  @state()
  subscription = new Subscription();

  @state()
  msg = "";

  @state()
  msgColour = textGreen;

  async connectedCallback() {
    super.connectedCallback();

    await this.getSubscription();
  }

  async getSubscription() {
    console.log("getting subscription");
    await fetch("/api/v1/get/subscription")
      .then(async (resp) => {
        if (!resp.ok) {
          const error = await resp.json();
          throw resp.statusText + ": " + error.message;
        }
        return resp.json();
      })
      .then((resp) => {
        this.subscription = resp;
      })
      .catch((err) => {
        console.error("Error fetching subscription:", err);
      });
  }

  userDetails() {
    return html`
      <div class="flex w-full flex-col items-center rounded-xl bg-white px-8 py-6 text-left shadow-md">
        <h1 class="text-xl font-bold">Details</h1>
        <table>
          <tr>
            <td class="p-2 pb-0"><strong>Name:</strong></td>
            <td class="p-2 pb-0">${this.user.name}</td>
          </tr>
          <tr>
            <td class="p-2 pb-0"><strong>Email:</strong></td>
            <td class="p-2 pb-0">${this.user.email}</td>
          </tr>
        </table>
      </div>
    `;
  }

  subscriptionErrorMsg() {
    if (this.msg == "") {
      return html``;
    }

    return html`
      <p id="err" class="${this.msgColour} py-2">${this.msg}</p>
    `;
  }

  subscriptionDetails() {
    if (import.meta.env.VITE_LITE == "true") {
      return html``;
    }
    if (!this.subscription) {
      return html`
        <div class="flex w-full flex-col items-center rounded-xl bg-white px-8 py-6 text-left shadow-md">
          <a href="plans.html" class="w-1/3"><button class="w-full rounded bg-gray-600 font-bold text-white">Subscribe Now</button></a>
        </div>
      `;
    }
    let cancelButton;
    if (this.subscription.Renew) {
      cancelButton = html`
        <button @click=${this.handleCancel} class="w-1/3 rounded bg-gray-600 font-bold text-white">Cancel</button>
      `;
    }
    return html`
      <div class="flex w-full flex-col items-center rounded-xl bg-white px-8 py-6 text-left shadow-md">
        <h1 class="text-xl font-bold">Subscription</h1>
        <table>
          <tr>
            <td class="p-2 pb-0"><strong>Subscription Type:</strong></td>
            <td class="p-2 pb-0">${this.subscription.Class}</td>
          </tr>
          <tr>
            <td class="p-2 pb-0"><strong>${this.subscription.Renew ? "Next Billing Date:" : "Subscription Ends:"}</strong></td>
            <td class="p-2 pb-0">${new Date(this.subscription.Finish).toDateString()}</td>
          </tr>
        </table>
        ${this.subscriptionErrorMsg()} ${cancelButton}
      </div>
    `;
  }

  async handleCancel() {
    await fetch("api/v1/stripe/cancel", { method: "POST" }).then(async (resp) => {
      this.msg = await resp.text();
      if (resp.ok) {
        this.msgColour = textGreen;
      } else {
        this.msgColour = textRed;
      }
    });

    this.getSubscription();
  }

  render() {
    return html`
      <div class="flex flex-col gap-2">${this.userDetails()} ${this.subscriptionDetails()}</div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "profile-details": ProfileDetails;
  }
}
