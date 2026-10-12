import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/user.service", () => ({
  default: { me: vi.fn(), channel: vi.fn(), channelVideos: vi.fn(), subscribe: vi.fn(), unsubscribe: vi.fn(), updateMe: vi.fn() },
}));
vi.mock("@/api/file.service", () => ({ default: { getPresignedUrl: vi.fn(), uploadToS3: vi.fn() } }));

import userService from "@/api/user.service";
import { isAuthenticated } from "@/auth/keycloak";
import Channel from "@/pages/Channel.vue";
import SubscribeButton from "@/components/channel/SubscribeButton.vue";
import ChannelEditDialog from "@/components/channel/ChannelEditDialog.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { channel, page, profile, videoCard } from "../../helpers/fixtures";

const svc = vi.mocked(userService);

const setup = async (route = "/channel/chan-1", opts: { ownerId?: string } = {}) => {
  svc.channel.mockResolvedValue(channel({ id: "chan-1", subscribers: 1200, video_count: 3, description: "Channel about films" }));
  svc.channelVideos.mockResolvedValue(page([videoCard({ id: "a" }), videoCard({ id: "b" })]));
  if (opts.ownerId) {
    isAuthenticated.value = true;
    svc.me.mockResolvedValue(profile({ id: opts.ownerId }));
  }
  const r = await mountWithPlugins(Channel, { route });
  await settle();
  return r;
};

const btn = (w: Awaited<ReturnType<typeof setup>>["wrapper"], text: string) => w.findAll(".v-btn").find((b) => b.text() === text);

describe("Channel page", () => {
  it("shows a spinner while loading", async () => {
    svc.channel.mockReturnValue(new Promise(() => {}));
    svc.channelVideos.mockReturnValue(new Promise(() => {}));
    const { wrapper } = await mountWithPlugins(Channel, { route: "/channel/x" });
    expect(wrapper.find(".v-progress-circular").exists()).toBe(true);
  });

  it("renders the header with counts and videos without channel info", async () => {
    const { wrapper } = await setup();
    expect(svc.channel).toHaveBeenCalledWith("chan-1");
    expect(svc.channelVideos).toHaveBeenCalledWith("chan-1", 1);
    expect(wrapper.find("h1").text()).toBe("Alice Films");
    expect(wrapper.text()).toContain("@alice");
    expect(wrapper.text()).toContain("1.2K subscribers · 3 videos");
    const cards = wrapper.findAllComponents(VideoCard);
    expect(cards).toHaveLength(2);
    expect(cards[0].props("showChannel")).toBe(false);
  });

  it("singular video count", async () => {
    svc.channel.mockResolvedValue(channel({ video_count: 1, subscribers: 1 }));
    svc.channelVideos.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Channel, { route: "/channel/chan-1" });
    await settle();
    expect(wrapper.text()).toContain("1 subscriber · 1 video");
    expect(wrapper.text()).toContain("This channel has no videos yet");
  });

  it("others' channel shows SubscribeButton, not owner actions", async () => {
    const { wrapper } = await setup();
    expect(wrapper.findComponent(SubscribeButton).props()).toEqual({ channelId: "chan-1", subscribed: false });
    expect(btn(wrapper, "Customize channel")).toBeUndefined();
  });

  it("own channel shows Customize + Manage videos and opens the edit dialog", async () => {
    const { wrapper } = await setup("/channel/chan-1", { ownerId: "chan-1" });
    expect(wrapper.findComponent(SubscribeButton).exists()).toBe(false);
    expect(btn(wrapper, "Manage videos")!.attributes("href")).toBe("/studio");
    await btn(wrapper, "Customize channel")!.trigger("click");
    await settle();
    expect(wrapper.findComponent(ChannelEditDialog).props("modelValue")).toBe(true);
    expect(wrapper.text()).toContain("Change picture");
  });

  it("own empty channel suggests uploading", async () => {
    svc.channel.mockResolvedValue(channel());
    svc.channelVideos.mockResolvedValue(page([]));
    isAuthenticated.value = true;
    svc.me.mockResolvedValue(profile({ id: "chan-1" }));
    const { wrapper } = await mountWithPlugins(Channel, { route: "/channel/chan-1" });
    await settle();
    expect(wrapper.text()).toContain("Create content on any device");
    expect(wrapper.find('a[href="/upload"]').exists()).toBe(true);
  });

  it("About tab shows the description and channel details", async () => {
    const { wrapper } = await setup();
    const about = wrapper.findAll(".v-tab").find((t) => t.text() === "About")!;
    await about.trigger("click");
    await settle();
    expect(wrapper.findComponent(VideoCard).exists()).toBe(false);
    expect(wrapper.find(".about").text()).toContain("Channel about films");
    expect(wrapper.find(".about").text()).toContain("Joined Jan 15, 2024");
    expect(wrapper.find(".about").text()).toContain("3 videos");
  });

  it("clicking the description snippet opens About", async () => {
    const { wrapper } = await setup();
    await wrapper.find(".about-link").trigger("click");
    await settle();
    expect(wrapper.find(".about").exists()).toBe(true);
  });

  it("?edit=1 opens the customize dialog and closing clears the query", async () => {
    const { wrapper, router } = await setup("/channel/chan-1?edit=1", { ownerId: "chan-1" });
    const dialog = wrapper.findComponent(ChannelEditDialog);
    expect(dialog.props("modelValue")).toBe(true);
    await wrapper.findAll(".v-btn").find((b) => b.text() === "Cancel")!.trigger("click");
    await settle();
    expect(router.currentRoute.value.query).toEqual({});
    expect(router.currentRoute.value.params.channelId).toBe("chan-1");
  });

  it("unknown channel shows the not-found state", async () => {
    svc.channel.mockRejectedValue({ message: "not found" });
    svc.channelVideos.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Channel, { route: "/channel/nope" });
    await settle();
    expect(wrapper.text()).toContain("Channel not found");
    expect(svc.channel).toHaveBeenCalledTimes(1);
  });
});
