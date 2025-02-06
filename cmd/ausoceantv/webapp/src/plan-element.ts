import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element.ts";

@customElement("plan-element")
export class planElement extends TailwindElement() {
  // Type of plan: eg 'Day Pass' or 'Monthly Subscription'.
  @property({ type: String, attribute: "plan-type" })
  planType = "";

  // URL to redirect to when plan is selected.
  @property({ type: String, attribute: "href" })
  href = "";

  // Cost of the plan in dollars.
  @property({ type: Number, attribute: "plan-cost" })
  planCost = "";

  // Comparison cost of paying for a different plan in dollars.
  @property({ type: Number, attribute: "comp-cost" })
  compCost = "";

  render() {
    let comparePrice = html``;
    if (this.compCost != "") {
      comparePrice = html`
        <p class="line-through">$${this.compCost}</p>
      `;
    }

    return html`
      <div class="flex h-fit flex-col items-center gap-5 overflow-hidden rounded bg-gray-300">
        <p class="w-full bg-gray-800 p-5 text-center text-lg font-bold text-white">${this.planType}</p>
        <div id="price" class="flex justify-center">
          <p class="text-3xl font-bold">$${this.planCost}</p>
          ${comparePrice}
        </div>
        <button @click="${this.selectPlan}" class="mb-5 w-fit rounded bg-gray-800 px-10 py-2 text-white">Select Plan</button>
      </div>
    `;
  }

  selectPlan() {
    window.location.href = this.href;
  }
}
declare global {
  interface HTMLElementTagNameMap {
    "plan-element": planElement;
  }
}
