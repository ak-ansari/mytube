import { expect, type Locator, type Page } from "@playwright/test";

/** pages/Watch.vue + components/video/CommentsSection.vue */
export class WatchPage {
  constructor(readonly page: Page) {}

  async goto(videoId: string) {
    await this.page.goto(`/watch/${videoId}`);
    await expect(this.title()).toBeVisible({ timeout: 20_000 });
  }

  readonly title = () => this.page.getByRole("heading", { level: 1 });
  readonly player = () => this.page.locator(".player-shell .video-js");
  readonly related = () => this.page.locator("aside.secondary-col a.video-card");
  readonly likeButton = () => this.page.locator(".reactions").getByRole("button").first();
  readonly dislikeButton = () => this.page.getByRole("button", { name: "Dislike" });
  readonly subscribeButton = () => this.page.getByRole("button", { name: /^(Subscribe|Subscribed)$/ });
  readonly commentsHeading = () => this.page.getByRole("heading", { level: 2, name: /^\d+ Comments?$/ });
  readonly commentBox = () => this.page.getByPlaceholder("Add a comment...");
  readonly commentSubmit = () => this.page.getByRole("button", { name: "Comment", exact: true });
  readonly comment = (text: string): Locator => this.page.locator(".comments .comment").filter({ hasText: text });

  async isLiked() {
    return (await this.likeButton().locator(".mdi-thumb-up").count()) > 0;
  }

  async likeCount() {
    return (await this.likeButton().innerText()).trim();
  }

  async addComment(text: string) {
    await this.commentBox().click();
    await this.commentBox().fill(text);
    await this.commentSubmit().click();
    await expect(this.comment(text)).toBeVisible();
  }

  async deleteComment(text: string) {
    const row = this.comment(text);
    await row.hover();
    await row.getByRole("button", { name: "Comment actions" }).click();
    await this.page.locator(".v-overlay--active").getByText("Delete", { exact: true }).click();
    await expect(row).toHaveCount(0);
  }
}
