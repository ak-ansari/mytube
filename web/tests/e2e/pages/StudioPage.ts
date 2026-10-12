import { expect, type Locator, type Page } from "@playwright/test";

/** pages/Studio.vue ("Channel content") */
export class StudioPage {
  constructor(readonly page: Page) {}

  async goto() {
    await this.page.goto("/studio");
    await expect(this.heading()).toBeVisible({ timeout: 20_000 });
  }

  readonly heading = () => this.page.getByRole("heading", { name: "Channel content" });
  readonly table = () => this.page.locator(".studio-table");
  readonly row = (title: string): Locator => this.page.locator(".studio-table tbody tr").filter({ hasText: title });
  readonly statusChip = (title: string) => this.row(title).locator(".v-chip");
  readonly editDialog = () => this.page.locator(".v-overlay--active .v-card").filter({ hasText: /Video details|Finish your upload/ });
  readonly deleteDialog = () => this.page.locator(".v-overlay--active .v-card").filter({ hasText: "Permanently delete this video?" });

  async openEdit(title: string) {
    await this.row(title).getByRole("button", { name: /^(Edit|Finish)$/ }).click();
    await expect(this.editDialog()).toBeVisible();
    return this.editDialog();
  }

  async deleteVideo(title: string) {
    await this.row(title).getByRole("button", { name: "Delete" }).click();
    await expect(this.deleteDialog()).toBeVisible();
    await this.deleteDialog().getByRole("button", { name: "Delete forever" }).click();
    await expect(this.row(title)).toHaveCount(0, { timeout: 20_000 });
  }
}
