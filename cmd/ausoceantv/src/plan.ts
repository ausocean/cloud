import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element.ts";

@customElement("plan-element")
export class PlanElement extends TailwindElement() {
    // Payment period of the subscription.
    // ie. year, month...
    @property({ type: String, attribute: "period" })
    period = ""

    // Name of the plan (title).
    // ie. Basic, Premium...
    @property({ type: String, attribute: "plan-name" })
    planName = ""

    // Price of the plan over period.
    // ie. 3.99.
    @property({ type: Number , attribute: "price"})
    price = null

    constructor() {
        super();
    }

  render() {
    return html`
      <div class="border-2 p-10 text-white bg-white dark:border-none text-neutral-800">
        <h2 class="text-2xl font-bold">${this.planName}</h2>
        <p>$${this.price}/${this.period}</p>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "plan-element": PlanElement;
  }
}
