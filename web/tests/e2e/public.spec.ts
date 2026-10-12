import { expect, test } from "@playwright/test";
import { api, type VideoCard } from "./helpers/api";
import { KEYCLOAK_LOGIN_URL } from "./helpers/env";
import { AppShell } from "./pages/AppShell";
import { KeycloakLoginPage } from "./pages/KeycloakLoginPage";
import { WatchPage } from "./pages/WatchPage";

// anonymous visitor: no storageState
test.use({ storageState: { cookies: [], origins: [] } });

let feed: VideoCard[] = [];
test.beforeAll(async () => {
  feed = (await api.browse()).items;
});

test.describe("home feed", () => {
  test("renders video cards linking to the watch page", async ({ page }) => {
    test.skip(!feed.length, "The feed is empty: publish at least one public video first");
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    await shell.expectSignedOut();

    const cards = page.locator(".video-grid a.video-card");
    await expect(cards.first()).toBeVisible();
    expect(await cards.count()).toBeGreaterThanOrEqual(Math.min(feed.length, 12));
    await expect(cards.first()).toHaveAttribute("href", `/watch/${feed[0].id}`);
    await expect(cards.first().getByRole("heading", { level: 3 })).toHaveText(feed[0].title.trim());

    await cards.first().click();
    await expect(page).toHaveURL(new RegExp(`/watch/${feed[0].id}$`));
  });
});

test.describe("search", () => {
  test("submitting the search bar opens /results?q=", async ({ page }) => {
    test.skip(!feed.length, "The feed is empty");
    const term = feed[0].title.trim().split(/\s+/)[0];
    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    await shell.search(term);
    await expect(page).toHaveURL(`/results?q=${encodeURIComponent(term)}`);
    await expect(page.getByText(`Results for “${term}”`)).toBeVisible();
    await expect(page.locator("a.video-card").first()).toBeVisible();
    await expect(shell.searchInput()).toHaveValue(term);
  });

  test("shows suggestions while typing and a suggestion click searches for it", async ({ page }) => {
    // suggestions come from Elasticsearch; pick a prefix that is known to have some
    let prefix = "";
    let expected: string[] = [];
    for (const v of feed) {
      const candidate = v.title.trim().slice(0, 3).toLowerCase();
      if (candidate.length < 2) continue;
      const s = await api.suggest(candidate).catch(() => []);
      if (s.length) {
        prefix = candidate;
        expected = s;
        break;
      }
    }
    test.skip(!prefix, "No search suggestions available from the API (empty ES index?)");

    const shell = new AppShell(page);
    await page.goto("/");
    await shell.ready();
    await shell.searchInput().click();
    await shell.searchInput().pressSequentially(prefix, { delay: 30 });
    await expect(shell.suggestions().first()).toBeVisible();
    expect(await shell.suggestions().count()).toBe(Math.min(expected.length, 8));

    const first = (await shell.suggestions().first().innerText()).trim();
    await shell.suggestions().first().click();
    await expect(page).toHaveURL(/\/results\?q=/);
    expect(new URL(page.url()).searchParams.get("q")).toBe(first);
  });

  test("an unknown term shows the empty state", async ({ page }) => {
    await page.goto(`/results?q=${encodeURIComponent("qxzjwvkpl")}`);
    await expect(page.getByText("No results found")).toBeVisible({ timeout: 20_000 });
  });
});

test.describe("watch page", () => {
  test("loads the player, title, channel, related videos and comments", async ({ page }) => {
    test.skip(!feed.length, "The feed is empty");
    const video = feed[0];
    const details = await api.details(video.id);
    const watch = new WatchPage(page);
    await watch.goto(video.id);

    await expect(watch.title()).toHaveText(details.title.trim());
    await expect(watch.player()).toBeVisible();
    // channel link next to the avatar
    await expect(page.locator(`.primary-col a[href="/channel/${details.channel.id}"]`).first()).toBeVisible();
    await expect(watch.subscribeButton()).toBeVisible();
    await expect(watch.likeButton()).toBeVisible();
    await expect(watch.commentsHeading()).toBeVisible();
    await expect(watch.commentsHeading()).toHaveText(new RegExp(`^${details.comment_count} Comments?$`));
    if (feed.length > 1) await expect(watch.related().first()).toBeVisible();
  });

  test("an unknown video id shows 'Video unavailable'", async ({ page }) => {
    await page.goto("/watch/00000000-0000-0000-0000-000000000000");
    await expect(page.getByText("Video unavailable")).toBeVisible({ timeout: 20_000 });
  });
});

test.describe("anonymous actions redirect to Keycloak", () => {
  test.beforeEach(() => test.skip(!feed.length, "The feed is empty"));

  test("like", async ({ page }) => {
    const watch = new WatchPage(page);
    await watch.goto(feed[0].id);
    await watch.likeButton().click();
    await new KeycloakLoginPage(page).expectVisible();
    // the redirect_uri brings the user back to this video
    expect(decodeURIComponent(page.url())).toContain(`/watch/${feed[0].id}`);
  });

  test("comment", async ({ page }) => {
    const watch = new WatchPage(page);
    await watch.goto(feed[0].id);
    await watch.commentBox().click();
    await new KeycloakLoginPage(page).expectVisible();
  });

  test("subscribe", async ({ page }) => {
    const watch = new WatchPage(page);
    await watch.goto(feed[0].id);
    await expect(watch.subscribeButton()).toHaveText("Subscribe");
    await watch.subscribeButton().click();
    await new KeycloakLoginPage(page).expectVisible();
  });
});

test.describe("routing", () => {
  test("unknown routes render the 404 page", async ({ page }) => {
    await page.goto("/this/route/does-not-exist");
    await expect(page.getByText("This page isn't available")).toBeVisible({ timeout: 20_000 });
    await page.getByRole("link", { name: "Go to home" }).click();
    await expect(page).toHaveURL("/");
  });

  for (const path of ["/studio", "/liked", "/subscriptions", "/upload"]) {
    test(`protected route ${path} redirects to the Keycloak login`, async ({ page }) => {
      await page.goto(path);
      await page.waitForURL(KEYCLOAK_LOGIN_URL);
      expect(decodeURIComponent(page.url())).toContain(`redirect_uri=http://localhost:5173${path}`);
      await new KeycloakLoginPage(page).expectVisible();
    });
  }

  test("logging in from a protected route lands back on it", async ({ page }) => {
    await page.goto("/liked");
    await new KeycloakLoginPage(page).login();
    await expect(page).toHaveURL(/\/liked/);
    await new AppShell(page).expectSignedIn();
  });
});
