import { expect, test } from "@playwright/test";
import { api, findForeignVideo, type VideoCard } from "./helpers/api";
import { AUTH_FILE, e2eTag } from "./helpers/env";
import { ChannelPage } from "./pages/ChannelPage";
import { WatchPage } from "./pages/WatchPage";

/**
 * Likes, comments and subscriptions against the real dev database. Every test records the
 * starting state through the API and afterAll puts it back, even if the UI part failed.
 */
test.use({ storageState: AUTH_FILE });
test.describe.configure({ mode: "serial" });

let video: VideoCard | undefined;
let initialReaction = 0;
let initialSubscribed = false;
const createdComments: { videoId: string; content: string }[] = [];

test.beforeAll(async () => {
  video = await findForeignVideo();
  if (!video) return;
  const d = await api.details(video.id);
  initialReaction = d.my_reaction;
  initialSubscribed = d.is_subscribed;
});

test.afterAll(async () => {
  if (!video) return;
  const d = await api.details(video.id).catch(() => undefined);
  if (d && d.my_reaction !== initialReaction) await api.react(video.id, initialReaction as -1 | 0 | 1);
  if (d && d.is_subscribed !== initialSubscribed) {
    await (initialSubscribed ? api.subscribe(video.channel.id) : api.unsubscribe(video.channel.id));
  }
  if (createdComments.length) {
    const me = await api.me();
    for (const { videoId, content } of createdComments) {
      const left = (await api.comments(videoId)).items.filter((c) => c.content === content && c.author.id === me.id);
      for (const c of left) await api.deleteComment(videoId, c.id);
    }
  }
});

test.beforeEach(() => test.skip(!video, "Needs a published public video owned by someone other than the demo user"));

test("like / unlike toggles the icon and the count", async ({ page }) => {
  const watch = new WatchPage(page);
  await watch.goto(video!.id);
  const before = await api.details(video!.id);
  const wasLiked = before.my_reaction === 1;
  expect(await watch.isLiked()).toBe(wasLiked);
  await expect(watch.likeButton()).toHaveText(String(before.likes));

  // toggle once
  await watch.likeButton().click();
  const expected1 = wasLiked ? before.likes - 1 : before.likes + 1;
  await expect(watch.likeButton()).toHaveText(String(expected1));
  await expect.poll(() => watch.isLiked()).toBe(!wasLiked);
  expect((await api.details(video!.id)).my_reaction).toBe(wasLiked ? 0 : 1);

  // and back
  await watch.likeButton().click();
  await expect(watch.likeButton()).toHaveText(String(before.likes));
  await expect.poll(() => watch.isLiked()).toBe(wasLiked);
  expect((await api.details(video!.id)).my_reaction).toBe(before.my_reaction === -1 ? 0 : before.my_reaction);
});

test("dislike then clear", async ({ page }) => {
  const watch = new WatchPage(page);
  await watch.goto(video!.id);
  const before = await api.details(video!.id);
  test.skip(before.my_reaction === -1, "Already disliked; not touching it");

  await watch.dislikeButton().click();
  await expect(watch.dislikeButton().locator(".mdi-thumb-down")).toBeVisible();
  if (before.my_reaction === 1) await expect(watch.likeButton()).toHaveText(String(before.likes - 1));
  await watch.dislikeButton().click();
  await expect(watch.dislikeButton().locator(".mdi-thumb-down-outline")).toBeVisible();
  expect((await api.details(video!.id)).my_reaction).toBe(0);
  // afterAll restores a previous like
});

test("add and delete a comment", async ({ page }) => {
  const watch = new WatchPage(page);
  await watch.goto(video!.id);
  const before = await api.details(video!.id);
  const text = e2eTag("comment");
  createdComments.push({ videoId: video!.id, content: text });

  // composing shows Cancel / Comment; Comment is disabled while empty
  await watch.commentBox().click();
  await expect(watch.commentSubmit()).toBeDisabled();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(watch.commentSubmit()).toBeHidden();

  await watch.addComment(text);
  await expect(watch.commentBox()).toHaveValue("");
  await expect(watch.comment(text)).toContainText("@demo");
  expect((await api.comments(video!.id)).items.some((c) => c.content === text)).toBe(true);

  await watch.deleteComment(text);
  await expect(page.getByText("Comment deleted")).toBeVisible();
  expect((await api.comments(video!.id)).items.some((c) => c.content === text)).toBe(false);
  expect((await api.details(video!.id)).comment_count).toBe(before.comment_count);
});

test("subscribe / unsubscribe on the channel page", async ({ page }) => {
  const channel = new ChannelPage(page);
  await channel.goto(video!.channel.id);
  const start = initialSubscribed ? "Subscribed" : "Subscribe";
  const flipped = initialSubscribed ? "Subscribe" : "Subscribed";
  await expect(channel.subscribeButton()).toHaveText(start);

  await channel.subscribeButton().click();
  await expect(channel.subscribeButton()).toHaveText(flipped);
  await expect(page.getByText(initialSubscribed ? "Subscription removed" : "Subscription added")).toBeVisible();
  expect((await api.channel(video!.channel.id)).is_subscribed).toBe(!initialSubscribed);

  // the sidebar's Subscriptions list follows
  const sidebarLink = page.locator("nav.v-navigation-drawer").locator(`a[href="/channel/${video!.channel.id}"]`);
  if (initialSubscribed) await expect(sidebarLink).toHaveCount(0);
  else await expect(sidebarLink).toBeVisible();

  await channel.subscribeButton().click();
  await expect(channel.subscribeButton()).toHaveText(start);
  expect((await api.channel(video!.channel.id)).is_subscribed).toBe(initialSubscribed);
});

test("subscribe button on the watch page reflects and toggles state", async ({ page }) => {
  const watch = new WatchPage(page);
  await watch.goto(video!.id);
  const start = initialSubscribed ? "Subscribed" : "Subscribe";
  await expect(watch.subscribeButton()).toHaveText(start);
  await watch.subscribeButton().click();
  await expect(watch.subscribeButton()).toHaveText(initialSubscribed ? "Subscribe" : "Subscribed");
  await watch.subscribeButton().click();
  await expect(watch.subscribeButton()).toHaveText(start);
});
