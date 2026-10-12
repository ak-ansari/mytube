import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({ default: { subscriptionFeed: vi.fn() } }));
vi.mock("@/api/user.service", () => ({ default: { mySubscriptions: vi.fn(), me: vi.fn() } }));

import catalogService from "@/api/catalog.service";
import userService from "@/api/user.service";
import { isAuthenticated } from "@/auth/keycloak";
import Subscriptions from "@/pages/Subscriptions.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { channel, page, videoCard } from "../../helpers/fixtures";

const feed = vi.mocked(catalogService.subscriptionFeed);
const subs = vi.mocked(userService.mySubscriptions);

describe("Subscriptions page", () => {
  it("shows the channel strip and latest videos", async () => {
    isAuthenticated.value = true;
    subs.mockResolvedValue([channel({ id: "c1", channel_name: "Alice Films", subscribers: 1200 }), channel({ id: "c2", channel_name: "Bob TV" })]);
    feed.mockResolvedValue(page([videoCard()]));
    const { wrapper } = await mountWithPlugins(Subscriptions);
    await settle();
    expect(wrapper.text()).toContain("Channels");
    expect(wrapper.text()).toContain("Latest");
    const links = wrapper.findAll("a.channel");
    expect(links.map((l) => l.attributes("href"))).toEqual(["/channel/c1", "/channel/c2"]);
    expect(links[0].text()).toContain("Alice Films");
    expect(links[0].text()).toContain("1.2K subscribers");
    expect(wrapper.findAllComponents(VideoCard)).toHaveLength(1);
  });

  it("empty with no subscriptions invites to subscribe", async () => {
    isAuthenticated.value = true;
    subs.mockResolvedValue([]);
    feed.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Subscriptions);
    await settle();
    expect(wrapper.text()).not.toContain("Channels");
    expect(wrapper.text()).toContain("Don’t miss new videos");
  });

  it("empty with subscriptions says no new videos", async () => {
    isAuthenticated.value = true;
    subs.mockResolvedValue([channel()]);
    feed.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Subscriptions);
    await settle();
    expect(wrapper.text()).toContain("No new videos");
    expect(wrapper.text()).toContain("Channels you subscribe to haven’t posted yet.");
  });

  it("does not fetch while signed out", async () => {
    await mountWithPlugins(Subscriptions);
    await settle();
    expect(subs).not.toHaveBeenCalled();
    expect(feed).not.toHaveBeenCalled();
  });
});
