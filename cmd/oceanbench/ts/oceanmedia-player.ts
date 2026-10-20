import "@videojs/html/live-video/player";
import "@videojs/html/media/hlsjs-video";
import "./videojs/live-video/skin";
import skinHtml from "./videojs/live-video/skin.html?raw";

const MEDIA_PLACEHOLDER = "<!-- Add a compatible media component here. -->";

class OceanMediaPlayer extends HTMLElement {
  static get observedAttributes(): string[] {
    return ["src"];
  }

  connectedCallback(): void {
    if (!this.hasChildNodes()) {
      const media = this.hasAttribute("autoplay") ? "<hlsjs-video playsinline autoplay></hlsjs-video>" : "<hlsjs-video playsinline></hlsjs-video>";
      this.innerHTML = `<live-video-player>${skinHtml.replace(MEDIA_PLACEHOLDER, media)}</live-video-player>`;
    }

    this.style.display = "block";
    if (!this.style.aspectRatio) {
      this.style.aspectRatio = "16 / 9";
    }

    this.updateSrc();
  }

  attributeChangedCallback(): void {
    this.updateSrc();
  }

  get src(): string {
    return this.getAttribute("src") ?? "";
  }

  set src(value: string) {
    this.setAttribute("src", value);
  }

  private updateSrc(): void {
    const media = this.querySelector("hlsjs-video") as HlsjsVideoElement | null;
    if (!media || !this.src) {
      return;
    }
    media.source = {
      src: this.src,
      engine: {
        hlsJs: {
          liveSyncDurationCount: 4, // stay ~4 target durations behind the edge
          maxBufferLength: 60, // buffer up to 60s ahead
          backBufferLength: 300, // keep 5 minutes for DVR seeking
        },
      },
    };
  }
}

type HlsjsVideoElement = HTMLElement & {
  source:
    | {
        src: string;
        engine?: { hlsJs?: Record<string, unknown> };
      }
    | null
    | undefined;
};

customElements.define("oceanmedia-player", OceanMediaPlayer);

declare global {
  interface HTMLElementTagNameMap {
    "oceanmedia-player": OceanMediaPlayer;
  }
}
