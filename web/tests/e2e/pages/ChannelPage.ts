import { expect, type Page } from "@playwright/test";

/** pages/Channel.vue */
export class ChannelPage {
  constructor(readonly page: Page) {}

  async goto(channelId: string, opts: { edit?: boolean } = {}) {
    await this.page.goto(`/channel/${channelId}${opts.edit ? "?edit=1" : ""}`);
    await expect(this.title()).toBeVisible({ timeout: 20_000 });
  }

  readonly title = () => this.page.getByRole("heading", { level: 1 });
  readonly subscribeButton = () => this.page.getByRole("button", { name: /^(Subscribe|Subscribed)$/ });
  readonly customizeButton = () => this.page.getByRole("button", { name: "Customize channel" });
  readonly customizeDialog = () => this.page.locator(".v-overlay--active .v-card").filter({ hasText: "Customize channel" });
}
