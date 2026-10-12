import { expect, test, type Page } from "@playwright/test";
import { api } from "./helpers/api";
import { AUTH_FILE } from "./helpers/env";
import { AppShell } from "./pages/AppShell";

/** Runs only in the "mobile" project (Pixel 7 viewport). */

const noHorizontalScroll = async (page: Page) => {
  const { scrollWidth, clientWidth } = await page.evaluate(() => ({
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
  }));
  expect(scrollWidth, `page is ${scrollWidth}px wide in a ${clientWidth}px viewport`).toBeLessThanOrEqual(clientWidth + 1);
};

test.describe("anonymous, mobile width", () => {
  test.use({ storageState: { cookies: [], origins: [] } });

  const pages: { name: string; path: () => Promise<string | undefined> }[] = [
    { name: "home", path: async () => "/" },
    { name: "results", path: async () => "/results?q=video" },
    { name: "watch", path: async () => { const v = (await api.browse()).items[0]; return v && `/watch/${v.id}`; } },
    { name: "channel", path: async () => { const v = (await api.browse()).items[0]; return v && `/channel/${v.channel.id}`; } },
    { name: "404", path: async () => "/no-such-page" },
  ];
  for (const p of pages) {
    test(`${p.name} has no horizontal scroll`, async ({ page }) => {
      const path = await p.path();
      test.skip(!path, "No data");
      await page.goto(path!);
      await new AppShell(page).ready();
      await page.waitForLoadState("networkidle").catch(() => undefined);
      await noHorizontalScroll(page);
    });
  }

  test("search collapses to an icon and expands on tap", async ({ page }) => {
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    await expect(shell.searchInput()).toBeHidden();
    await page.getByRole("banner").getByRole("button", { name: "Search", exact: true }).click();
    await expect(shell.searchInput()).toBeVisible();
    await shell.search("video");
    await expect(page).toHaveURL(/\/results\?q=video/);
    await page.getByRole("button", { name: "Close search" }).click();
    await expect(shell.searchInput()).toBeHidden();
  });
});

test.describe("signed in, mobile width", () => {
  test.use({ storageState: AUTH_FILE });

  test("nav drawer is an overlay: hidden, opens on Menu, closes on scrim and on navigation", async ({ page }) => {
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.expectSignedIn();
    const drawer = shell.drawer();
    await expect(drawer).not.toHaveClass(/v-navigation-drawer--active/);

    await shell.menuButton().click();
    await expect(drawer).toHaveClass(/v-navigation-drawer--active/);
    await expect(drawer).toHaveClass(/v-navigation-drawer--temporary/);
    await expect(drawer.getByRole("link", { name: "Your videos" })).toBeVisible();
    await page.locator(".v-navigation-drawer__scrim").click({ position: { x: 380, y: 400 } });
    await expect(drawer).not.toHaveClass(/v-navigation-drawer--active/);

    await shell.menuButton().click();
    await drawer.getByRole("link", { name: "Your videos" }).click();
    await expect(page).toHaveURL(/\/studio$/);
    await expect(drawer).not.toHaveClass(/v-navigation-drawer--active/);
  });

  for (const path of ["/studio", "/upload"]) {
    test(`${path} has no horizontal scroll`, async ({ page }) => {
      await page.goto(path);
      await new AppShell(page).expectSignedIn();
      await page.waitForLoadState("networkidle").catch(() => undefined);
      await noHorizontalScroll(page);
    });
  }
});
