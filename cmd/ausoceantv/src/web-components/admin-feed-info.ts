import { html } from "lit";
import { customElement, property } from "lit/decorators.js";
import { TailwindElement } from "../shared/tailwind.element.ts";
import style from "./admin-feed-info.css?inline"; // # See NOTE

enum action {
  new = "new",
  save = "save",
}

@customElement("admin-feed-info")
export class AdminFeedInfo extends TailwindElement(style) {
  @property({ type: String })
  msg = "";

  @property({ type: Array })
  feeds = [];

  constructor() {
    super();
    this.getFeeds();
  }

  render() {
    let warning;
    if (this.msg != "") {
      // prettier-ignore
      warning = html`
        <p class="text-red-600 font-bold">${this.msg}</p>
          `
    }

    // prettier-ignore
    return html`
      ${warning}
      <div class="grid-col-6 grid gap-1">
        <div class="col-span-6 grid grid-cols-subgrid">
          <h3 class="font-bold">ID</h3>
          <h3 class="font-bold">Feed Name</h3>
          <h3 class="font-bold">Area</h3>
          <h3 class="font-bold">Class</h3>
          <h3 class="font-bold">Bundle</h3>
        </div>
        ${this.feeds.map(
          (feed, i) => html`
            <form class="col-span-6 grid grid-cols-subgrid" @submit="${(e: Event) => this.handleSubmit(e, action.save, i)}">
              <input name="id" value="${feed.id}" readonly class="rounded-md bg-gray-200 px-2 py-1" />
              <input name="name" type="text" value="${feed.name}" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300" />
              <select name="area" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300">
                <option value="SA" ?selected=${feed.area === "SA"}>South Australia</option>
                <option value="FNQ" ?selected=${feed.area === "FNQ"}>Far North Queensland</option>
                <option value="WA" ?selected=${feed.area === "WA"}>Western Australia</option>
                <option value="VIC" ?selected=${feed.area === "VIC"}>Victoria</option>
                <option value="EAC" ?selected=${feed.area === "EAC"}>East Coast</option>
              </select>
              <select name="class" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300">
                <option value="video" ?selected=${feed.dataClass === "video"}>Video</option>
                <option value="data.mtw" ?selected=${feed.dataClass === "data.mtw"}>Water Temperature</option>
                <option value="data.mwh" ?selected=${feed.dataClass === "data.mwh"}>Air Temperature</option>
              </select>
              <input name="bundle" type="text" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300" value="${feed.bundle}" />
              <input type="submit" value="Save" class="rounded-md border-2 border-blue-500 text-blue-500 hover:cursor-pointer hover:bg-blue-600 hover:text-white hover:border-blue-600 px-2" />
            </form>
          `,
        )}
        <form class="col-span-6 grid grid-cols-subgrid" @submit="${(e: Event) => this.handleSubmit(e, action.new, -1)}">
          <input name="id" disabled/>
          <input name="name" type="text" placeholder="Feed Name" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300" />
          <select name="area" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300">
            <option value="SA">South Australia</option>
            <option value="FNQ">Far North Queensland</option>
            <option value="WA">Western Australia</option>
            <option value="VIC">Victoria</option>
            <option value="EAC">East Coast</option>
          </select>
          <select name="class" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300">
            <option value="video">Video</option>
            <option value="data.mtw">Water Temperature</option>
            <option value="data.mwh">Air Temperature</option>
          </select>
          <input name="bundle" type="text" placeholder="123456, 098765" class="rounded-md bg-white px-2 py-1 outline outline-1 -outline-offset-1 outline-gray-300"/>
          <input type="submit" value="Save" class="rounded-md bg-blue-500 text-white hover:cursor-pointer hover:bg-blue-600 px-2" />
        </form>
      </div>
    `;
  }

  async getFeeds() {
    fetch("/api/v1/get/feeds/all")
      .then((resp) => {
        return resp.json();
      })
      .then((resp) => {
        resp.forEach((a) => {
          const feed = {
            id: a.ID,
            name: a.Name,
            area: a.Area,
            dataClass: a.Class,
            bundle: a.Bundle,
          };
          this.feeds = [...this.feeds, feed];
        });
        console.log(this.feeds); // Moved outside of the loop for better readability
      });
  }

  async handleSubmit(e: Event, endpoint: action, index: number) {
    e.preventDefault();
    const form = e.target as HTMLFormElement;
    const formData = new FormData(form);

    let resp = await fetch("/api/v1/admin/feed/" + endpoint, {
      method: "POST",
      body: formData,
    })
      .then((resp) => {
        if (!resp.ok) {
          throw new Error("bad response");
        }
        return resp.json();
      })
      .then((resp) => {
        return resp;
      })
      .catch((err) => {
        alert(err);
      });

    if (endpoint === action.save) {
      if (index < 0 || index >= this.feeds.length || this.feeds[index] === undefined) {
        this.msg = "An error occurred, please try again";
        return;
      }

      const feed = this.feeds[index];
      feed.name = resp.Name;
      feed.area = resp.Area;
      feed.dataClass = resp.Class;
      feed.bundle = resp.Bundle;
      return;
    }

    const feed = {
      id: resp.ID,
      name: resp.Name,
      area: resp.Area,
      dataClass: resp.Class,
      bundle: resp.Bundle,
    };

    form.reset();

    this.feeds = [...this.feeds, feed];
  }
}

declare global {
  interface HTMLElementTagNameMap {
    "admin-feed-info": AdminFeedInfo;
  }
}
