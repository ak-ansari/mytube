import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({ default: { liked: vi.fn() } }));

import catalogService from "@/api/catalog.service";
import { isAuthenticated } from "@/auth/keycloak";
import Liked from "@/pages/Liked.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { page, videoCard } from "../../helpers/fixtures";

const liked = vi.mocked(catalogService.liked);

describe("Liked page", () => {
  it("does not fetch when signed out", async () => {
    liked.mockResolvedValue(page([]));
    await mountWithPlugins(Liked);
    await settle();
    expect(liked).not.toHaveBeenCalled();
  });

  it("lists liked videos", async () => {
    isAuthenticated.value = true;
    liked.mockResolvedValue(page([videoCard({ id: "a" }), videoCard({ id: "b" })]));
    const { wrapper } = await mountWithPlugins(Liked);
    await settle();
    expect(wrapper.find("h1").text()).toBe("Liked videos");
    expect(wrapper.findAllComponents(VideoCard)).toHaveLength(2);
  });

  it("shows the empty state", async () => {
    isAuthenticated.value = true;
    liked.mockResolvedValue(page([]));
    const { wrapper } = await mountWithPlugins(Liked);
    await settle();
    expect(wrapper.text()).toContain("No liked videos yet");
    expect(wrapper.find('a[href="/"]').exists()).toBe(true);
  });
});
