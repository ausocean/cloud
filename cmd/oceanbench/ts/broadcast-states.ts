import { html, PropertyValues } from "lit";
import { customElement, property, state } from "lit/decorators.js";
import { TailwindElement } from "./shared/tailwind.element";
import { type Broadcast } from "./types/broadcast";

enum DirectState {
  IDLE = "main.directIdle",
  STARTING = "main.directStarting",
  LIVE = "main.directLive",
  UNHEALTHY = "main.directLiveUnhealthy",
}

// Maps the state to the progress amount.
const progressMap: Record<DirectState, number> = {
  [DirectState.IDLE]: 0,
  [DirectState.STARTING]: 34,
  [DirectState.LIVE]: 68,
  [DirectState.UNHEALTHY]: 84,
} as const;

// Displays a stepped progress bar showing the current state
// of a broadcast.
@customElement("broadcast-states")
export class BroadcastStates extends TailwindElement() {
  // UUID of the currently selected broadcast.
  @property({ attribute: "broadcast-id", type: "string" })
  UUID = "";

  // Broadcast config of currently selected broadcast.
  @state()
  config: Broadcast | null = null;

  // Current progress %.
  @state()
  progress: number = 0;

  // Interval ID.
  @state()
  intervalID: ReturnType<typeof setInterval> | undefined = undefined;

  // Handle updates to the state.
  protected updated(_changedProperties: PropertyValues): void {
    if (_changedProperties.has("UUID") && this.UUID != "") {
      if (this.intervalID != undefined) {
        clearInterval(this.intervalID);
        this.intervalID = undefined;
      }

      this.poll();
      this.intervalID = setInterval(() => this.poll(), 2000);
    }
  }

  // Poll makes a request to update the current broadcast config.
  async poll(): Promise<void> {
    try {
      const skey = window.location.pathname.split("/")[1]
      const url = `/api/v1/${skey}/broadcasts/${this.UUID}`

      const res = await fetch(url);
      if (!res.ok) {
        throw new Error(`Failed to fetch broadcast config: ${res.statusText}`);
      }
      this.config = await res.json();
    } catch (err) {
      console.error(`Error fetching broadcast config:`, err);
      this.config = null;
      return;
    }

    this.updateProgression();
  }

  // Updates the current progress based on the current broadcast state from the config.
  private updateProgression() {
    if (!this.config) return;

    this.progress = progressMap[this.config.BroadcastState as DirectState] ?? 0;
  }

  render() {
    if (this.config === null) {
      // Don't render when a broadcast isn't selected.
      return;
    }

    return html`
      <div class="flex gap-2 relative mb-8">
        <!--Failure badge-->
        <div class="${!this.config.InFailure ? "hidden" : "flex"} ">
          <div class="bg-red-400 rounded-full px-4 py-2 text-red-900 w-fit">
            Failed
          </div>
        </div>

        <div class="relative flex w-full">
          <!--Progress Bar-->
          <div
            class="flex h-2 w-full content-center overflow-hidden rounded-full mt-1.5"
          >
            <div
              class="h-full bg-green-600 ease-out transition-all"
              style="width: ${this.progress}%"
            ></div>
            <div
              class="h-full ${this.progress == 84
                ? "bg-red-600"
                : "bg-slate-600"} ease-out transition-all"
              style="width: ${100 - this.progress}%"
            ></div>
          </div>

          <!--Idle Milestone-->
          <div class="absolute left-0 flex flex-col size-fit items-start">
            <div class="h-5 w-5 rounded-full bg-green-600"></div>
            <label>Idle</label>
          </div>

          <!--Starting Milestone-->
          <div
            class="absolute left-1/3 -translate-x-1/2 flex flex-col size-fit items-center w-0"
          >
            <div
              class="h-5 w-5 rounded-full ${this.progress >=
              progressMap[DirectState.STARTING]
                ? "bg-green-600"
                : "bg-slate-600"}"
            ></div>
            <label>Starting</label>
          </div>

          <!--Live Milestone-->
          <div
            class="absolute left-2/3 -translate-x-1/2 flex flex-col size-fit items-center"
          >
            <div
              class="h-5 w-5 rounded-full ${this.progress >=
              progressMap[DirectState.LIVE]
                ? "bg-green-600"
                : "bg-slate-600"}"
            ></div>
            <label>Live</label>
          </div>

          <!--Live Unhealthy Milestone-->
          <div class="absolute right-0 flex flex-col size-fit items-end">
            <div
              class="h-5 w-5 rounded-full ${this.progress >=
              progressMap[DirectState.UNHEALTHY]
                ? "bg-red-600"
                : "bg-slate-600"}"
            ></div>
            <label>Unhealthy</label>
          </div>
        </div>
      </div>
    `;
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "broadcast-states": BroadcastStates;
  }
}
