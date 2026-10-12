import { describe, expect, it } from "vitest";
import VideoCard from "@/components/video/VideoCard.vue";
import ChannelAvatar from "@/components/channel/ChannelAvatar.vue";
import VideoThumbnail from "@/components/video/VideoThumbnail.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { videoCard } from "../../helpers/fixtures";

const video = videoCard({ id: "v42", title: "Learning Vue", views: 1_234_567, description: "All about Vue" });

describe("VideoCard", () => {
  it("grid: links to watch, shows avatar, channel name and formatted counts", async () => {
    const { wrapper } = await mountWithPlugins(VideoCard, { props: { video } });
    const link = wrapper.find("a.video-card");
    expect(link.attributes("href")).toBe("/watch/v42");
    expect(link.classes()).toContain("variant-grid");
    expect(wrapper.find("h3.title").text()).toBe("Learning Vue");
    expect(wrapper.findComponent(ChannelAvatar).exists()).toBe(true);
    expect(wrapper.find(".channel-name").text()).toBe("Alice Films");
    expect(wrapper.text()).toContain("1.2M views • 2 days ago");
    expect(wrapper.findComponent(VideoThumbnail).props()).toMatchObject({ url: video.thumbnail_url, durationSeconds: 75 });
    // only one link per card (no nested anchors)
    expect(wrapper.findAll("a")).toHaveLength(1);
  });

  it("list: shows description and a small channel row", async () => {
    const { wrapper } = await mountWithPlugins(VideoCard, { props: { video, variant: "list" } });
    expect(wrapper.find("a").classes()).toContain("variant-list");
    expect(wrapper.text()).toContain("All about Vue");
    expect(wrapper.text()).toContain("Alice Films");
    const avatars = wrapper.findAllComponents(ChannelAvatar);
    expect(avatars).toHaveLength(1);
    expect(avatars[0].props("size")).toBe(24);
  });

  it("compact: no avatar, still shows channel name and counts", async () => {
    const { wrapper } = await mountWithPlugins(VideoCard, { props: { video, variant: "compact" } });
    expect(wrapper.find("a").classes()).toContain("variant-compact");
    expect(wrapper.findComponent(ChannelAvatar).exists()).toBe(false);
    expect(wrapper.find(".channel-name").text()).toBe("Alice Films");
    expect(wrapper.text()).toContain("1.2M views");
    expect(wrapper.text()).not.toContain("All about Vue");
  });

  it("showChannel=false hides channel info in every variant", async () => {
    for (const variant of ["grid", "list", "compact"] as const) {
      const { wrapper } = await mountWithPlugins(VideoCard, { props: { video, variant, showChannel: false } });
      expect(wrapper.findComponent(ChannelAvatar).exists()).toBe(false);
      expect(wrapper.text()).not.toContain("Alice Films");
    }
  });

  it("uses user_name when the channel has no name, and singular view", async () => {
    const v = videoCard({ views: 1, channel: { ...video.channel, channel_name: null, user_name: "bob" } });
    const { wrapper } = await mountWithPlugins(VideoCard, { props: { video: v } });
    expect(wrapper.find(".channel-name").text()).toBe("bob");
    expect(wrapper.text()).toContain("1 view •");
  });

  it("clicking the channel goes to the channel instead of the video", async () => {
    const { wrapper, router } = await mountWithPlugins(VideoCard, { props: { video } });
    await wrapper.find(".channel-name").trigger("click");
    await settle();
    expect(router.currentRoute.value.name).toBe("Channel");
    expect(router.currentRoute.value.params.channelId).toBe("chan-1");
  });

  it("clicking the card navigates to watch", async () => {
    const { wrapper, router } = await mountWithPlugins(VideoCard, { props: { video } });
    await wrapper.find("h3").trigger("click");
    await settle();
    expect(router.currentRoute.value.fullPath).toBe("/watch/v42");
  });
});
