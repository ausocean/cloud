import { LitElement, html, css, HTMLTemplateResult } from "lit";
import { customElement, property, state } from "lit/decorators.js";

// tvOverviewConfig holds the saved configuration for a user.
interface tvOverviewConfig {
  broadcasts: { UUID: string }[]; // List of broadcasts UUIDs.
}

// broadcast is a stripped down version of the broadcast config with the relevant fields to the
// overview page.
interface broadcast {
  UUID: string; // The immutable unique key of the broadcast.
  SKey: number; // The key of the site this broadcast belongs to.
  Name: string; // The name of the broadcast.
  BID: string; // Broadcast identification.
  Description: string; // The broadcast description shown below viewing window.
  Start: Date; // Start time in native go format for easy operations.
  End: Date; // End time in native go format for easy operations.
  CameraMac: number; // Camera hardware's MAC address.
  ControllerMAC: number; // Controller hardware's MAC adress (controller used to power camera).
  Active: boolean; // This is true if the broadcast is currently active i.e. waiting for data or currently streaming.
  Issues: number; // The number of successive stream issues currently experienced. Reset when good health seen.
  AttemptingToStart: boolean; // Indicates if we're currently attempting to start the broadcast.
  Enabled: boolean; // Is the broadcast enabled? If not, it will not be started.
  Unhealthy: boolean; // True if the broadcast is unhealthy.
  BroadcastState: string; // Holds the current state of the broadcast.
  HardwareState: string; // Holds the current state of the hardware.
  StartFailures: number; // The number of times the broadcast has failed to start.
  // StateData:                []byte        // States will be marshalled and their data stored here.
  // HardwareStateData:        []byte        // Hardware states will be marshalled and their data stored here.
  Account: string; // The YouTube account email that this broadcast is associated with.
  InFailure: boolean; // True if the broadcast is in a failure state.
  RecoveringVoltage: boolean; // True if the broadcast is currently recovering voltage.
}

// TVOverview is an element designed to list all of the broadcasts that are important
// to a superadmin. It shows their current state all in one place.
@customElement("tv-overview")
export class TVOverview extends LitElement {
  // tvOverviewConfig holds the users configuration of the page. This consists
  // of an array of broadcast UUIDs that have been added by the superadmin.
  @state() cfg: tvOverviewConfig | null = null;

  // broadcasts holds an array of broadcast objects (which are stripped down
  // versions of the broadcast configuration). These are used to render the current state
  // of the broadcasts.
  @state() broadcasts: broadcast[] = [];

  // Styles for the element.
  static styles = css`
    /* The card class is the background of each broadcast element. */
    .card {
      background-color: white;
      border-radius: 0.375rem;
      padding: 1.5rem;
      border: 1px solid var(--bs-border-color);
      display: flex;
      flex-direction: column;
      justify-content: center;
    }

    /* 1. Remove the default spacing between paragraphs */
    .card p {
      margin: 0;
      line-height: 1.2; /* 2. Tighten the space between wrapped text lines */
    }

    /* 3. Add a small gap between the Title and the first paragraph */
    .card h2 {
      margin: 0 0 0.5rem 0;
    }

    .card-content {
      display: flex;
      gap: 20px;
    }

    iframe {
      border-radius: 0.25rem;
    }
  `;

  async firstUpdated() {
    // Make a request to get the configuration for the user.
    await this.fetchConfig();
  }

  override render() {
    if (this.broadcasts.length == 0) {
      console.log("no broadcasts to render");
      return html``;
    } else {
      console.log("rendering with broadcasts:");
      console.log(this.broadcasts);
    }
    return html`
      <div style="display: flex; flex-direction:column; gap: 1.5rem;">${this.broadcasts.map((b) => this.renderBroadcast(b))}</div>
    `;
  }

  // renderBroadcast renders an individual broadcast state element.
  private renderBroadcast(b: broadcast): HTMLTemplateResult {
    if (!b.UUID) {
      console.log("no UUID on broadcast");
      return html``;
    }

    return html`
      <div class="card">
        <h2>${b.Name}</h2>
        <div class="card-content">
          <div>
            <iframe src="https://www.youtube.com/embed/${b.BID}" title="YouTube video player" frameborder="0" allowfullscreen></iframe>
          </div>
          <div>
            <p>${new Date(b.Start).toLocaleTimeString()} - ${new Date(b.End).toLocaleTimeString()}</p>
            <p>
              Active:
              <strong style="color: ${b.Active ? "green" : "red"};">${b.Active ? "Active" : "Inactive"}</strong>
            </p>
            <p>
              Healthy:
              <strong style="color: ${b.Unhealthy ? "red" : "green"};">${b.Unhealthy ? "Unhealthy" : "Healthy"}</strong>
            </p>
            <p>Broadcast State: ${b.BroadcastState}</p>
            <p>Hardware State: ${b.HardwareState}</p>
          </div>
        </div>
      </div>
    `;
  }

  // fetchConfig makes an API request to get the config for the signed in user.
  private async fetchConfig() {
    console.log("getting tv overview config");
    fetch("/api/get/profile/tv-overview-config", { method: "POST" })
      .then(async (resp) => {
        if (!resp.ok) {
          throw await resp.text();
        }
        return resp.json();
      })
      .then((resp) => {
        this.cfg = resp;
        console.log("got tv overview config");
        return;
      })
      .then(() => {
        this.fetchBroadcasts();
      })
      .catch((err) => {
        console.log("error fetching config:", err);
        return;
      });
  }

  // fetchBroadcasts fetches the broadcast configurations for each of the broadcasts
  // in the users configuration.
  private async fetchBroadcasts() {
    if (!this.cfg?.broadcasts) {
      // Return early if there are no broadcasts to fetch.
      return;
    }

    // Make the fetch requests.
    const fetchPromises = this.cfg.broadcasts.map((broadcast) => this.getBroadcastWithUUID(broadcast.UUID));

    // Await the requests to return.
    const resolvedBroadcasts = await Promise.all(fetchPromises);

    // Filter out any failed requests.
    this.broadcasts = resolvedBroadcasts.filter((b): b is broadcast => b !== undefined && b !== null);

    console.log("finished fetching all broadcasts");

    // This seems to be required, as otherwise a re-render isn't triggered.
    this.requestUpdate();
  }

  // getBroadcastWithUUID makes requests to the api endpoint to get the broadcast configuration
  // for a broadcast with the passed uuid, returning a promise for the fetch.
  private async getBroadcastWithUUID(uuid: string): Promise<broadcast | undefined> {
    console.log("getting broadcast with UUID:", uuid);
    return fetch(`/api/get/broadcast/config?id=${uuid}`)
      .then(async (resp) => {
        if (!resp.ok) {
          throw await resp.text();
        }
        return resp.json();
      })
      .catch((err) => {
        console.log("error getting broadcast config:", err);
      });
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "tv-overview": TVOverview;
  }
}
