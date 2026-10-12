import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({ default: { browse: vi.fn() } }));

import catalogService from "@/api/catalog.service";
import Results from "@/pages/Results.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { page, videoCard } from "../../helpers/fixtures";

const browse = vi.mocked(catalogService.browse);

describe("Results page", () => {
  it("searches with the trimmed ?q= and renders list cards", async () => {
    browse.mockResolvedValue(page([videoCard()]));
    const { wrapper } = await mountWithPlugins(Results, { route: "/results?q=%20%20cats%20" });
    await settle();
    expect(browse).toHaveBeenCalledWith("cats", 1);
    expect(wrapper.text()).toContain("Results for “cats”");
    expect(wrapper.findComponent(VideoCard).props("variant")).toBe("list");
  });

  it("shows the no-results state", async () => {
    browse.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Results, { route: "/results?q=zzz" });
    await settle();
    expect(wrapper.text()).toContain("No results found");
  });

  it("re-searches when the query changes", async () => {
    browse.mockResolvedValue(page([]));
    const { wrapper, router } = await mountWithPlugins(Results, { route: "/results?q=a" });
    await settle();
    await router.push("/results?q=b");
    await settle();
    expect(browse).toHaveBeenLastCalledWith("b", 1);
    expect(wrapper.text()).toContain("Results for “b”");
  });

  it("without q it shows no heading", async () => {
    browse.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Results, { route: "/results" });
    await settle();
    expect(wrapper.text()).not.toContain("Results for");
    expect(browse).toHaveBeenCalledWith("", 1);
  });
});
