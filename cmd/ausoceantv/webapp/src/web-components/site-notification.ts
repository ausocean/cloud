import { html } from 'lit';
import { customElement, state } from 'lit/decorators.js';
import { TailwindElement } from '../shared/tailwind.element.ts';

@customElement('site-notification')
export class SiteNotification extends TailwindElement() {
  @state() private notification: any = null;
  @state() private visible = false;

  connectedCallback() {
    super.connectedCallback();
    this.loadNotification();
  }

  async loadNotification() {
    try {
      const res = await fetch('/temp/notification-001.json');
      const data = await res.json();

      const dismissed = localStorage.getItem('dismissedNotification');
      const parsed = dismissed ? JSON.parse(dismissed) : null;

      if (!parsed || parsed.id !== data.id || parsed.version !== data.version) {
        const now = new Date();
        const expiry = new Date(data.expiresAt);
        if (now < expiry) {
          this.notification = data;
          this.visible = true;
        }
      }
    } catch (err) {
      console.error('Failed to load notification:', err);
    }
  }

  dismiss() {
    if (this.notification) {
      localStorage.setItem(
        'dismissedNotification',
        JSON.stringify({
          id: this.notification.id,
          version: this.notification.version,
        })
      );
    }
    this.visible = false;
  }

  render() {
    if (!this.visible || !this.notification) return null;

    return html`
      <div
        class="relative mx-auto mt-4 max-w-3xl rounded-xl border-l-4 border-yellow-400 bg-yellow-900 bg-opacity-80 p-4 text-yellow-100 shadow-xl"
      >
        <button
          class="absolute right-3 top-2 cursor-pointer text-lg text-yellow-300 hover:text-white"
          @click=${this.dismiss}
        >
          ×
        </button>
        <h2 class="text-lg font-semibold">${this.notification.title}</h2>
        <p class="mt-1 text-sm">${this.notification.message}</p>
      </div>
    `;
  }
}
