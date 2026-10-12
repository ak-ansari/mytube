import { expect, test } from "@playwright/test";
import { api } from "./helpers/api";
import { AUTH_FILE } from "./helpers/env";
import { AppShell } from "./pages/AppShell";
import { ChannelPage } from "./pages/ChannelPage";
import { KeycloakLoginPage } from "./pages/KeycloakLoginPage";
import { StudioPage } from "./pages/StudioPage";

test.describe("login / logout via the app bar", () => {
  // a fresh session of its own: signing out must not kill the shared storageState session
  test.use({ storageState: { cookies: [], origins: [] } });

  test("sign in through Keycloak, see the account menu, sign out", async ({ page }) => {
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    await shell.expectSignedOut();

    await shell.signInButton().click();
    await new KeycloakLoginPage(page).login();
    await shell.expectSignedIn();

    const me = await api.me();
    const menu = await shell.openAccountMenu();
    await expect(menu.getByText(`@${me.user_name}`)).toBeVisible();
    await expect(menu.getByText("Your videos")).toBeVisible();
    await expect(menu.getByText("Liked videos")).toBeVisible();

    await menu.getByText("Sign out").click();
    await page.waitForURL("http://localhost:5173/", { timeout: 20_000 });
    await shell.expectSignedOut();

    // the session is really gone: a reload stays anonymous
    await page.reload();
    await shell.ready();
    await shell.expectSignedOut();
  });
});

test.describe("signed in", () => {
  test.use({ storageState: AUTH_FILE });

  test("theme toggle switches theme and persists after reload", async ({ page }) => {
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    const initial = await shell.currentTheme();
    const other = initial === "dark" ? "light" : "dark";

    await expect(shell.themeToggle()).toHaveAccessibleName(initial === "dark" ? "Light theme" : "Dark theme");
    await shell.themeToggle().click();
    await expect.poll(() => shell.currentTheme()).toBe(other);
    expect(await page.evaluate(() => localStorage.getItem("mytube-theme"))).toBe(other === "light" ? "myTubeLight" : "myTubeDark");

    await page.reload();
    await shell.ready();
    expect(await shell.currentTheme()).toBe(other);
    await expect(shell.themeToggle()).toHaveAccessibleName(other === "dark" ? "Light theme" : "Dark theme");

    // toggle back (storage is per-test-context anyway)
    await shell.themeToggle().click();
    await expect.poll(() => shell.currentTheme()).toBe(initial);
  });

  test("desktop nav drawer collapses to a rail and expands again", async ({ page }) => {
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    const drawer = shell.drawer();
    await expect(drawer).toBeVisible();
    await expect(drawer).not.toHaveClass(/v-navigation-drawer--rail/);
    await expect(drawer.getByRole("link", { name: "Your videos" })).toBeVisible();
    expect((await drawer.boundingBox())!.width).toBeGreaterThan(200);

    await shell.menuButton().click();
    await expect(drawer).toHaveClass(/v-navigation-drawer--rail/);
    await expect.poll(async () => (await drawer.boundingBox())!.width).toBeLessThanOrEqual(72);
    await expect(drawer.locator("a.rail-item")).toHaveCount(3);
    await expect(drawer.getByRole("link", { name: "You" })).toBeVisible();

    await shell.menuButton().click();
    await expect(drawer).not.toHaveClass(/v-navigation-drawer--rail/);
    await expect(drawer.getByRole("link", { name: "Your videos" })).toBeVisible();
  });

  test("watch page uses an overlay drawer on desktop", async ({ page }) => {
    const video = (await api.browse()).items[0];
    test.skip(!video, "The feed is empty");
    const shell = new AppShell(page);
    await page.goto(`/watch/${video.id}`);
    await shell.ready();
    await expect(shell.drawer()).not.toHaveClass(/v-navigation-drawer--active/);
    await shell.menuButton().click();
    await expect(shell.drawer()).toHaveClass(/v-navigation-drawer--active/);
    await expect(shell.drawer()).toHaveClass(/v-navigation-drawer--temporary/);
    await page.locator(".v-navigation-drawer__scrim").click();
    await expect(shell.drawer()).not.toHaveClass(/v-navigation-drawer--active/);
  });

  test("protected pages render when signed in", async ({ page }) => {
    const shell = new AppShell(page);
    for (const [path, heading] of [
      ["/studio", "Channel content"],
      ["/upload", "Upload video"],
    ] as const) {
      await page.goto(path);
      await shell.expectSignedIn();
      await expect(page.getByRole("heading", { name: heading })).toBeVisible();
    }
    for (const path of ["/liked", "/subscriptions"]) {
      await page.goto(path);
      await shell.expectSignedIn();
      await expect(page).toHaveURL(new RegExp(`${path}$`));
    }
  });

  test("channel customize dialog opens via ?edit=1 and cancels without saving", async ({ page }) => {
    const me = await api.me();
    const channel = new ChannelPage(page);
    await channel.goto(me.id, { edit: true });
    const dialog = channel.customizeDialog();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel("Channel name")).toHaveValue(me.channel_name || me.user_name);

    await dialog.getByLabel("Channel name").fill("should not be saved");
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
    await expect(page).toHaveURL(new RegExp(`/channel/${me.id}$`));
    await expect(channel.title()).toHaveText(me.channel_name || me.user_name);
    expect((await api.me()).channel_name).toBe(me.channel_name);

    // reopen from the button: fields are reset to the saved profile
    await channel.customizeButton().click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel("Channel name")).toHaveValue(me.channel_name || me.user_name);
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
  });

  test("account menu 'Customize channel' opens the dialog", async ({ page }) => {
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    const menu = await shell.openAccountMenu();
    await menu.getByText("Customize channel").click();
    await expect(page).toHaveURL(/\/channel\/[^/?]+\?edit=1$/);
    const dialog = new ChannelPage(page).customizeDialog();
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
  });

  test("studio lists my videos and the edit dialog opens", async ({ page }) => {
    const mine = (await api.mine()).items;
    const studio = new StudioPage(page);
    await studio.goto();
    test.skip(!mine.length, "The demo user has no videos");
    const editable = mine.find((v) => v.title && v.status === "published");

    for (const v of mine.filter((m) => m.title).slice(0, 5)) await expect(studio.row(v.title.trim()).first()).toBeVisible();

    test.skip(!editable, "No published video to edit");
    const dialog = await studio.openEdit(editable!.title.trim());
    await expect(dialog.getByText("Video details")).toBeVisible();
    await expect(dialog.getByLabel("Title (required)")).toHaveValue(editable!.title);
    await dialog.getByRole("button", { name: "Cancel" }).click();
    await expect(dialog).toBeHidden();
  });
});
