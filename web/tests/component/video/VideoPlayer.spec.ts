import { describe, expect, it, vi } from "vitest";

const player = vi.hoisted(() => ({
  one: vi.fn(),
  poster: vi.fn(),
  src: vi.fn(),
  dispose: vi.fn(),
}));
vi.mock("video.js", () => ({ default: vi.fn(() => player) }));

import videojs from "video.js";
import VideoPlayer from "@/components/VideoPlayer.vue";
import NotFound from "@/pages/404.vue";
import { mountWithPlugins } from "../../helpers/mount";

describe("VideoPlayer (video.js mocked)", () => {
  it("creates an HLS player with the poster and emits play once video.js reports it", async () => {
    const { wrapper } = await mountWithPlugins(VideoPlayer, { props: { src: "http://hls.test/master.m3u8", poster: "http://img.test/p.jpg" } });
    expect(videojs).toHaveBeenCalledWith(
      wrapper.find("video").element,
      expect.objectContaining({
        controls: true,
        poster: "http://img.test/p.jpg",
        sources: [{ src: "http://hls.test/master.m3u8", type: "application/x-mpegURL" }],
      })
    );
    const [event, cb] = player.one.mock.calls[0];
    expect(event).toBe("play");
    cb();
    expect(wrapper.emitted("play")).toHaveLength(1);
  });

  it("switches source (mp4 type for non-HLS) and disposes on unmount", async () => {
    const { wrapper } = await mountWithPlugins(VideoPlayer, { props: { src: "a.m3u8" } });
    await wrapper.setProps({ src: "http://cdn.test/b.mp4" });
    expect(player.poster).toHaveBeenCalledWith("");
    expect(player.src).toHaveBeenCalledWith({ src: "http://cdn.test/b.mp4", type: "video/mp4" });
    expect(player.one).toHaveBeenCalledTimes(2);
    wrapper.unmount();
    expect(player.dispose).toHaveBeenCalled();
  });
});

describe("404 page", () => {
  it("explains and links home", async () => {
    const { wrapper } = await mountWithPlugins(NotFound);
    expect(wrapper.text()).toContain("This page isn't available");
    expect(wrapper.find("a[href='/']").text()).toContain("Go to home");
  });
});
