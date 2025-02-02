import { provide } from "@lit/context";
import { HTMLTemplateResult, LitElement, html } from "lit";

import { customElement, property, state } from "lit/decorators.js";
import "./action-input"; // Ensure the component is imported

import { Devices } from "./types/device";
import { devicesContext } from "./utils/devices-context";
import { ActionInput } from "./action-input";

type action = {
  MAC: String;
  variable: String;
  value: String;
};

@customElement("action-list")
export class ActionList extends LitElement {
  static formAssociated = true;
  internals: ElementInternals;
  @property({ attribute: "value" })
  value = "";
  @property({ attribute: "name" })
  name = "";

  actionsDiv: HTMLDivElement | null = null;

  @state()
  actions: action[] = [];

  constructor() {
    super();
    this.internals = this.attachInternals();
  }

  firstUpdated() {
    this.actionsDiv = this.shadowRoot?.getElementById("actions") as HTMLDivElement;
    this.actions = this.parseActions();
    console.log("existing actions:", this.actions);
  }

  parseActions() {
    if (this.value == "") {
      console.log("got no actions");
      return [];
    }
    const arr = this.value.split(",");
    return arr.map((act) => ({
      MAC: act.substring(0, act.indexOf(".")),
      variable: act.substring(act.indexOf(".") + 1, act.indexOf("=")),
      value: act.substring(act.indexOf("=") + 1, act.length),
    }));
  }

  updateValue() {
    if (!this.actionsDiv) {
      return;
    }

    const result = (Array.from(this.actionsDiv.children) as ActionInput[]).map((input: ActionInput) => input.value);
    this.value = result.join(",");

    this.internals.setFormValue(this.value);
  }

  @provide({ context: devicesContext })
  devices: Devices = {
    Controllers: [
      {
        Name: "name1",
        MAC: "00:00:00:00:00:01",
        Type: "controller",
      },
    ],
    Cameras: [
      {
        Name: "name2",
        MAC: "00:00:00:00:00:02",
        Type: "camera",
      },
    ],
  };

  newBlankAction() {
    console.log("adding blank action");
    this.actions = [...this.actions, { MAC: "", variable: "", value: "" }];
  }

  createAction(mac: string, variable: string, varValue: string) {
    if (!this.actionsDiv) {
      return;
    }

    const element = new ActionInput(mac, variable, varValue);
    element.addEventListener("action-value", this.updateValue.bind(this));
    this.actionsDiv.append(element);
  }

  render() {
    console.log("rendering with action:", this.actions);
    return html`
      <div id="actions">
        ${this.actions.map(
          (act) => html`
            <action-input mac=${act.MAC} variable=${act.variable} value=${act.value}></action-input>
          `,
        )}
      </div>
      <button @click=${this.newBlankAction}>New Action</button>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "action-list": ActionList;
  }
}
