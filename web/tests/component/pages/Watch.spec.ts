import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/components/VideoPlayer.vue", async () => {
  const { defineComponent, h } = await import("vue");
  return {
    default: defineComponent({
      name: "VideoPlayer",
      props: { src: String, poster: String },
      emits: ["play"],
      render() {
        return h("div", { class: "player-stub" });
      },
    }),
  };
});
vi.mock("@/api/catalog.service", () => ({
  default: {
    details: vi.fn(),
    related: vi.fn(),
    recordView: vi.fn(),
    react: vi.fn(),
    comments: vi.fn(),
    addComment: vi.fn(),
    deleteComment: vi.fn(),
  },
}));
vi.mock("@/api/user.service", () => ({
  default: { me: vi.fn(), subscribe: vi.fn(), unsubscribe: vi.fn() },
}));

import { push } from "notivue";
import catalogService from "@/api/catalog.service";
import userService from "@/api/user.service";
import { isAuthenticated, login } from "@/auth/keycloak";
import Watch from "@/pages/Watch.vue";
import VideoPlayer from "@/components/VideoPlayer.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import SubscribeButton from "@/components/channel/SubscribeButton.vue";
import { VideoStatus, VideoVisibility } from "@/types";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { channelSummary, page, profile, videoCard, videoDetails } from "../../helpers/fixtures";

const svc = vi.mocked(catalogService);
const users = vi.mocked(userService);

const mountWatch = async () => {
  const res = await mountWithPlugins(Watch, { route: "/watch/vid-1" });
  await settle();
  return res;
};
const btn = (w: Awaited<ReturnType<typeof mountWatch>>["wrapper"], text: string) =>
  w.findAll("button").find((b) => b.text().trim() === text)!;

beforeEach(() => {
  svc.details.mockResolvedValue(videoDetails());
  svc.related.mockResolvedValue(page([videoCard({ id: "r1", title: "Related one" }), videoCard({ id: "r2", title: "Related two" })]));
  svc.comments.mockResolvedValue(page([]));
  svc.recordView.mockResolvedValue(null);
});

describe("Watch page", () => {
  it("renders title, channel, subscribers and formatted likes", async () => {
    const { wrapper } = await mountWatch();
    expect(svc.details).toHaveBeenCalledWith("vid-1");
    expect(wrapper.find("h1.video-title").text()).toBe("My first video");
    expect(wrapper.text()).toContain("Alice Films");
    expect(wrapper.text()).toContain("42 subscribers");
    expect(btn(wrapper, "1.5K")).toBeTruthy();
    expect(wrapper.text()).toContain("1.2K views");
    expect(wrapper.findComponent(SubscribeButton).props()).toMatchObject({ channelId: "chan-1", subscribed: false });
  });

  it("passes manifest_url and poster to the player", async () => {
    const { wrapper } = await mountWatch();
    const player = wrapper.findComponent(VideoPlayer);
    expect(player.props("src")).toBe("http://hls.test/videos/tok/1/vid-1/master.m3u8");
    expect(player.props("poster")).toBe("http://minio.test/thumb.jpg");
  });

  it("records the view on first play only", async () => {
    const { wrapper } = await mountWatch();
    const player = wrapper.findComponent(VideoPlayer);
    player.vm.$emit("play");
    player.vm.$emit("play");
    await settle();
    expect(svc.recordView).toHaveBeenCalledTimes(1);
    expect(svc.recordView).toHaveBeenCalledWith("vid-1");
  });

  it("anonymous like calls login() and sends nothing", async () => {
    const { wrapper } = await mountWatch();
    await btn(wrapper, "1.5K").trigger("click");
    await wrapper.find("button[aria-label=Dislike]").trigger("click");
    expect(login).toHaveBeenCalledTimes(2);
    expect(svc.react).not.toHaveBeenCalled();
  });

  it("like sends 1, dislike sends -1", async () => {
    isAuthenticated.value = true;
    users.me.mockResolvedValue(profile({ id: "someone-else" }));
    svc.react.mockImplementation(async (id, value) => videoDetails({ id, my_reaction: value }));
    const { wrapper } = await mountWatch();
    await btn(wrapper, "1.5K").trigger("click");
    await settle();
    expect(svc.react).toHaveBeenLastCalledWith("vid-1", 1);
    await wrapper.find("button[aria-label=Dislike]").trigger("click");
    await settle();
    expect(svc.react).toHaveBeenLastCalledWith("vid-1", -1);
  });

  it("clicking the active reaction clears it (0)", async () => {
    isAuthenticated.value = true;
    users.me.mockResolvedValue(profile({ id: "someone-else" }));
    svc.details.mockResolvedValue(videoDetails({ my_reaction: 1 }));
    svc.react.mockResolvedValue(videoDetails({ my_reaction: 0 }));
    const { wrapper } = await mountWatch();
    expect(wrapper.html()).toContain("mdi-thumb-up");
    await btn(wrapper, "1.5K").trigger("click");
    await settle();
    expect(svc.react).toHaveBeenCalledWith("vid-1", 0);
  });

  it("owner sees 'Edit video' instead of SubscribeButton", async () => {
    isAuthenticated.value = true;
    users.me.mockResolvedValue(profile({ id: "chan-1" }));
    const { wrapper } = await mountWatch();
    expect(wrapper.findComponent(SubscribeButton).exists()).toBe(false);
    expect(wrapper.text()).toContain("Edit video");
  });

  it("unpublished video shows a status placeholder instead of the player", async () => {
    svc.details.mockResolvedValue(
      videoDetails({ manifest_url: "", status: VideoStatus.Transcoded, visibility: VideoVisibility.Private })
    );
    const { wrapper } = await mountWatch();
    expect(wrapper.findComponent(VideoPlayer).exists()).toBe(false);
    expect(wrapper.text()).toContain("This video is processing.");
    expect(wrapper.text()).toContain("Only you can see it");
    // non-public visibility chip
    expect(wrapper.find(".v-chip").text()).toContain("private");
  });

  it("shows 'Video unavailable' when details fail", async () => {
    svc.details.mockRejectedValue({ message: "not found" });
    const { wrapper } = await mountWatch();
    expect(wrapper.text()).toContain("Video unavailable");
    expect(wrapper.findComponent(VideoPlayer).exists()).toBe(false);
  });

  it("renders related videos as VideoCards", async () => {
    const { wrapper } = await mountWatch();
    expect(svc.related).toHaveBeenCalledWith("vid-1", 1);
    const cards = wrapper.findAllComponents(VideoCard);
    expect(cards.map((c) => c.props("video").id)).toEqual(["r1", "r2"]);
  });

  it("expands and collapses the description", async () => {
    svc.details.mockResolvedValue(videoDetails({ description: "Long text", created_at: "2024-01-15T12:00:00Z" }));
    const { wrapper } = await mountWatch();
    const desc = wrapper.find(".description");
    expect(desc.classes()).not.toContain("expanded");
    expect(desc.text()).toContain("...more");
    await desc.trigger("click");
    expect(desc.classes()).toContain("expanded");
    expect(desc.text()).toContain("Jan 15, 2024");
    await btn(wrapper, "Show less").trigger("click");
    expect(wrapper.find(".description").classes()).not.toContain("expanded");
  });

  it("shows 'No description.' when empty", async () => {
    svc.details.mockResolvedValue(videoDetails({ description: null }));
    const { wrapper } = await mountWatch();
    expect(wrapper.find(".desc-text").text()).toBe("No description.");
  });

  it("share copies the link and toasts", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    const { wrapper } = await mountWatch();
    await btn(wrapper, "Share").trigger("click");
    await settle();
    expect(writeText).toHaveBeenCalledWith(window.location.href);
    expect(push.success).toHaveBeenCalledWith("Link copied to clipboard");
  });

  it("share falls back to an info toast when clipboard is unavailable", async () => {
    Object.defineProperty(navigator, "clipboard", { value: { writeText: vi.fn().mockRejectedValue(new Error("denied")) }, configurable: true });
    const { wrapper } = await mountWatch();
    await btn(wrapper, "Share").trigger("click");
    await settle();
    expect(push.info).toHaveBeenCalledWith(window.location.href);
  });

  it("renders the comments section for the video", async () => {
    const { wrapper } = await mountWatch();
    expect(svc.comments).toHaveBeenCalledWith("vid-1", 1);
    expect(wrapper.text()).toContain("2 Comments");
    expect(channelSummary().id).toBe("chan-1");
  });
});
