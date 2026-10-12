import { execFileSync } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test } from "@playwright/test";
import { api, deleteVideoQuietly } from "./helpers/api";
import { AUTH_FILE, e2eTag, isWorkerRunning } from "./helpers/env";
import { StudioPage } from "./pages/StudioPage";
import { UploadPage } from "./pages/UploadPage";

/**
 * Full UI upload through the real pipeline: presign → PUT to MinIO → worker validates + thumbnails
 * → Publish → confirm → Studio → delete. Needs ffmpeg (to make a clip) and the pipeline worker.
 * Anything left behind is deleted through the API in afterAll.
 */
test.use({ storageState: AUTH_FILE });
test.describe.configure({ mode: "serial" });

const FFMPEG = process.env.FFMPEG ?? "/usr/local/bin/ffmpeg";
const PIPELINE_TIMEOUT = 3 * 60_000;

// The worker grabs its thumbnail at t=3s (internal/workers/thumbnail.go), so a clip of exactly 3s
// yields no frame and phase 1 never finishes. 5s keeps the clip tiny while clearing that offset.
let dir = "";
let clip = "";
let ffmpegError = "";
const createdIds = new Set<string>();

test.beforeAll(async () => {
  dir = mkdtempSync(join(tmpdir(), "mytube-e2e-"));
  clip = join(dir, `e2e-clip-${Date.now()}.mp4`);
  try {
    execFileSync(
      FFMPEG,
      ["-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc=duration=5:size=320x240:rate=25", "-pix_fmt", "yuv420p", "-c:v", "libx264", clip],
      { stdio: "pipe" }
    );
  } catch (err) {
    ffmpegError = `ffmpeg failed (${FFMPEG}): ${(err as Error).message}`;
  }
});

test.afterAll(async () => {
  for (const id of createdIds) await deleteVideoQuietly(id);
  if (dir) rmSync(dir, { recursive: true, force: true });
});

test.beforeEach(() => test.skip(!!ffmpegError, ffmpegError));

test("discard mid-flow removes the draft", async ({ page }) => {
  const upload = new UploadPage(page);
  await upload.goto();
  const id = await upload.chooseFile(clip);
  createdIds.add(id);

  await expect(upload.titleField()).toHaveValue(/^e2e-clip-\d+$/);
  await expect(upload.publishButton()).toBeDisabled();

  const del = page.waitForResponse((r) => r.url().endsWith(`/api/videos/${id}`) && r.request().method() === "DELETE");
  await upload.discardButton().click();
  expect((await del).ok()).toBeTruthy();
  await expect(page.getByText("Upload discarded")).toBeVisible();
  await expect(upload.dropzone()).toBeVisible();

  const mine = await api.mine();
  expect(mine.items.some((v) => v.id === id)).toBe(false);
  createdIds.delete(id);
});

test("upload, publish, see it in Studio, delete it", async ({ page }) => {
  test.skip(!isWorkerRunning(), "The pipeline worker is not running (start it with `make run-worker` in ../mytube); Publish would never unlock");
  test.setTimeout(PIPELINE_TIMEOUT * 2 + 60_000);

  const upload = new UploadPage(page);
  const studio = new StudioPage(page);
  const title = e2eTag("upload");

  await upload.goto();
  const id = await upload.chooseFile(clip);
  createdIds.add(id);

  await upload.titleField().fill(title);
  await upload.descriptionField().fill("Uploaded by the Playwright e2e suite; safe to delete.");
  const outcome = page.getByText(/Checks complete\. Ready to publish\.|We couldn't process this file/);
  await expect(outcome, "phase 1 (validate + thumbnail) did not finish; check the worker log").toBeVisible({ timeout: PIPELINE_TIMEOUT });
  await expect(outcome, "the worker marked the upload as failed").toHaveText("Checks complete. Ready to publish.");
  await expect(upload.publishButton()).toBeEnabled();

  await upload.publishButton().click();
  await expect(upload.publishedHeading()).toBeVisible({ timeout: 30_000 });
  await expect(page.getByText(`“${title}” is being transcoded now.`)).toBeVisible();

  await page.getByRole("link", { name: "Go to Studio" }).click();
  await expect(studio.heading()).toBeVisible();
  await expect(studio.row(title)).toBeVisible();

  // let phase 2 finish so deleting doesn't race the worker writing segments
  await expect(studio.statusChip(title)).toHaveText(/Published|Failed/, { timeout: PIPELINE_TIMEOUT });

  await studio.deleteVideo(title);
  await expect(page.getByText("Video deleted")).toBeVisible();
  expect((await api.mine()).items.some((v) => v.id === id)).toBe(false);
  createdIds.delete(id);
});
