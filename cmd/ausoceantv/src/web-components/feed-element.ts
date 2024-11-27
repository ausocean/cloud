import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";

const placeholderImgPath = "src/assets/thumbnail.jpg";

@customElement("feed-element")
export class FeedElement extends TailwindElement() {
  @property({ attribute: "name", type: String })
  name = "";

  @property({ attribute: "stream-id", type: String })
  streamID = "";

  @property({ attribute: "live", type: Boolean })
  isLive = false;

  @property({ attribute: "img", type: String })
  imgURL = placeholderImgPath;

  render() {
    let liveIcon;
    if (this.isLive) {
      liveIcon = "🔴 Live";
    }

    return html`
      <div
        @click="${this.ClickHandler}"
        class="inline-flex cursor-pointer flex-col rounded-lg bg-gray-300 p-5"
      >
        <img src=${this.imgURL} class="w-80 rounded-md" />
        <div class="flex items-center justify-between gap-2 px-2 pt-2">
          <h1 class="w-fit text-lg font-bold">${this.name}</h1>
          ${liveIcon}
        </div>
      </div>
    `;
  }

  ClickHandler() {
    window.location.assign("/watch" + this.streamID);
  }
}
declare global {
  interface HTMLElementTagNameMap {
    "feed-element": FeedElement;
  }
}
