import { html } from "lit";
import { customElement, state } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";

@customElement("share-link")
export class ShareLink extends TailwindElement() {
  @state()
  copied = false;

  connectedCallback() {
    super.connectedCallback();
    if (!document.querySelector('link[href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css"]')) {
      const link = document.createElement("link");
      link.rel = "stylesheet";
      link.href = "https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css";
      document.head.appendChild(link);
    }
  }

  render() {
    let copyMsg = html``;
    if (this.copied) {
      copyMsg = html`
        <p class="text-gray-500">Link copied to clipboard</p>
      `;
    }
    return html`
      <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0-beta3/css/all.min.css" />
      ${copyMsg}
      <div @click="${this.Share}" class="m-auto flex w-3/4 cursor-pointer items-center overflow-hidden whitespace-nowrap rounded bg-[#0c69ad] md:w-1/2">
        <div class="bg-[#108eea] p-5 py-2"><i class="fas fa-copy"></i></div>
        <div class="text-l h-full w-full cursor-pointer bg-inherit text-center font-semibold">ausocean.tv</div>
      </div>
    `;
  }

  async Share() {
    const shareData = {
      title: "AusOcean TV",
      text: "Check out AusOceanTV for live underwater video streams:",
      url: "ausocean.tv",
    };

    if (navigator.share && navigator.canShare(shareData)) {
      navigator.share(shareData).then(() => (this.copied = true));
    } else {
      navigator.clipboard
        .writeText("ausocean.tv")
        .then(() => {
          this.copied = true;
        })
        .catch(() => alert("Failed to copy the text"));
    }
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "share-link": ShareLink;
  }
}
