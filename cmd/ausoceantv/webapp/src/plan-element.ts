import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element.ts";

// Keep this in sync with plans.html
const planDay = "Day Pass";
const planMonth = "Monthly Subscription";
const planYear = "Yearly Subscription";

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

  // Returns a different gradient for each of the plan types to make the
  // month and year plans stand out.
  productColor() {
    switch (this.planType) {
      case planDay:
        return "from-gray-600 to-gray-500";
      case planMonth:
        return "from-[#0c69ad] to-[#108eea]";
      case planYear:
        return "from-[#fbb018] to-[#fcc85d]";
    }
  }

  render() {
    let comparePrice = html``;
    if (this.compCost != "") {
      comparePrice = html`
        <p class="line-through">$${this.compCost}</p>
      `;
    }

    return html`
      <div class="flex flex-col items-center gap-5 overflow-hidden rounded-2xl bg-gradient-to-b from-neutral-50 to-neutral-50/80">
        <img src="src/assets/blue_logo.png" class="logo m-5 mx-auto mb-0 w-20 text-nowrap md:m-12 md:mb-0 md:w-32 lg:m-24 lg:mb-0" alt="AusOcean logo" />
        <p class="${this.productColor()} flex h-16 w-full items-center justify-center bg-gradient-to-r text-center text-2xl font-bold text-white">${this.planType}</p>
        <div id="price" class="flex justify-center text-gray-800">
          <p class="text-3xl font-bold">$${this.planCost}</p>
          ${comparePrice}
        </div>
        <button @click="${this.selectPlan}" class="mb-2 rounded-lg bg-[#0c69ad] px-6 py-3 text-lg font-semibold text-white shadow-md transition-all hover:shadow-lg hover:brightness-110 md:mb-6 lg:mb-12">Select Plan</button>
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
