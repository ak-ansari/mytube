import { expect, test, type Page } from "@playwright/test";
import { api } from "./helpers/api";
import { AUTH_FILE } from "./helpers/env";
import { AppShell } from "./pages/AppShell";

/**
 * Visual / UI smoke checks on the desktop project: console errors and a basic accessibility pass.
 * Kept deliberately tolerant: third-party noise (fonts, media decode, Keycloak iframe) is ignored.
 */
const IGNORED = [
  /fonts\.(googleapis|gstatic)\.com/i,
  /favicon/i,
  /Failed to load resource.*(\.m3u8|\.ts|9002|:80\/videos)/i, // media / presigned assets can 403/410 when stale
  /VIDEOJS/i,
  /MEDIA_ERR/i,
  /silent-check-sso/i,
];

const collectErrors = (page: Page) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(`pageerror: ${e.message}`));
  page.on("console", (m) => {
    if (m.type() === "error" && !IGNORED.some((r) => r.test(m.text()))) errors.push(`console: ${m.text()}`);
  });
  return errors;
};

type Target = { name: string; path: () => Promise<string | undefined>; heading?: boolean };

const anonymousPages: Target[] = [
  { name: "home", path: async () => "/" },
  { name: "results", path: async () => "/results?q=video" },
  { name: "watch", path: async () => (await api.browse()).items[0] && `/watch/${(await api.browse()).items[0].id}`, heading: true },
  { name: "channel", path: async () => (await api.browse()).items[0] && `/channel/${(await api.browse()).items[0].channel.id}`, heading: true },
  { name: "404", path: async () => "/no-such-page" },
];

const signedInPages: Target[] = [
  { name: "studio", path: async () => "/studio", heading: true },
  { name: "upload", path: async () => "/upload", heading: true },
  { name: "liked", path: async () => "/liked" },
  { name: "subscriptions", path: async () => "/subscriptions" },
];

const check = async (page: Page, t: Target) => {
  const path = await t.path();
  test.skip(!path, "No data for this page");
  const errors = collectErrors(page);
  await page.goto(path!);
  await new AppShell(page).ready();
  await page.waitForLoadState("networkidle").catch(() => undefined);

  expect(errors, `console errors on ${path}`).toEqual([]);

  // accessibility smoke: one main landmark, a banner, a navigation landmark, a document title and lang
  await expect(page.getByRole("main")).toHaveCount(1);
  await expect(page.getByRole("banner")).toBeVisible();
  await expect(page.getByRole("navigation").first()).toBeAttached();
  await expect(page).toHaveTitle(/myTube/);
  expect(await page.locator("html").getAttribute("lang")).toBeTruthy();
  if (t.heading) await expect(page.getByRole("heading", { level: 1 }).first()).toBeVisible();

  // every icon-only button in the app bar has an accessible name
  for (const btn of await page.getByRole("banner").getByRole("button").all()) {
    expect((await btn.getAttribute("aria-label")) || (await btn.innerText()).trim(), "unnamed app-bar button").toBeTruthy();
  }
};

test.describe("anonymous pages", () => {
  test.use({ storageState: { cookies: [], origins: [] } });
  for (const t of anonymousPages) test(`${t.name}: no console errors, landmarks present`, async ({ page }) => check(page, t));
});

test.describe("signed-in pages", () => {
  test.use({ storageState: AUTH_FILE });
  for (const t of signedInPages) test(`${t.name}: no console errors, landmarks present`, async ({ page }) => check(page, t));
});

test.describe("images", () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  // thumbnails are decorative (alt=""): the card link already carries the title
  test("thumbnails have an alt attribute", async ({ page }) => {
    await page.goto("/");
    await new AppShell(page).ready();
    await expect(page.locator("main img").first()).toBeVisible();
    expect(await page.locator("main img:not([alt])").count(), "images without alt").toBe(0);
  });
});
