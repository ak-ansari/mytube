import { expect, type Page } from "@playwright/test";

/** pages/UploadVideo.vue */
export class UploadPage {
  constructor(readonly page: Page) {}

  async goto() {
    await this.page.goto("/upload");
    await expect(this.dropzone()).toBeVisible({ timeout: 20_000 });
  }

  readonly dropzone = () => this.page.getByRole("button", { name: "Select a video file to upload" });
  readonly fileInput = () => this.page.locator('input[type="file"][accept="video/*"]');
  readonly titleField = () => this.page.getByLabel("Title (required)");
  readonly descriptionField = () => this.page.getByLabel("Description");
  readonly publishButton = () => this.page.getByRole("button", { name: "Publish" });
  readonly discardButton = () => this.page.getByRole("button", { name: "Discard" });
  readonly publishedHeading = () => this.page.getByRole("heading", { name: "Video published" });

  /** Picks the file and resolves with the draft video id returned by the presign call. */
  async chooseFile(path: string): Promise<string> {
    const presign = this.page.waitForResponse((r) => r.url().includes("/api/videos/upload/presign") && r.request().method() === "POST");
    await this.fileInput().setInputFiles(path);
    const res = await presign;
    expect(res.ok(), `presign failed: ${res.status()}`).toBeTruthy();
    const body = (await res.json()) as { data: { id: string } };
    return body.data.id;
  }
}
