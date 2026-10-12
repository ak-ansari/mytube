import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({ default: { browse: vi.fn() } }));

import catalogService from "@/api/catalog.service";
import Home from "@/pages/Home.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { page, videoCard } from "../../helpers/fixtures";

const browse = vi.mocked(catalogService.browse);

describe("Home page", () => {
  it("shows skeletons while loading", async () => {
    browse.mockReturnValue(new Promise(() => {}));
    const { wrapper } = await mountWithPlugins(Home);
    await settle(1);
    expect(wrapper.findAll(".skeleton-grid").length).toBe(12);
  });

  it("renders the feed from browse('')", async () => {
    browse.mockResolvedValue(page([videoCard({ id: "a" }), videoCard({ id: "b" })]));
    const { wrapper } = await mountWithPlugins(Home);
    await settle();
    expect(browse).toHaveBeenCalledWith("", 1);
    expect(wrapper.findAllComponents(VideoCard)).toHaveLength(2);
  });

  it("shows the empty state with an upload link", async () => {
    browse.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Home);
    await settle();
    expect(wrapper.text()).toContain("No videos yet");
    expect(wrapper.find('a[href="/upload"]').exists()).toBe(true);
  });

  it("shows an error state whose Retry refetches", async () => {
    browse.mockRejectedValueOnce({ message: "down" });
    const { wrapper } = await mountWithPlugins(Home);
    await settle();
    expect(wrapper.text()).toContain("Couldn't load videos");
    browse.mockResolvedValue(page([videoCard()]));
    const retry = wrapper.findAll("button").find((b) => b.text() === "Retry")!;
    await retry.trigger("click");
    await settle();
    expect(browse).toHaveBeenCalledTimes(2);
    expect(wrapper.findAllComponents(VideoCard)).toHaveLength(1);
  });
});
