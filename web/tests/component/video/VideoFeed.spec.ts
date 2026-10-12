import { describe, expect, it } from "vitest";
import VideoFeed from "@/components/video/VideoFeed.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import LoadMore from "@/components/common/LoadMore.vue";
import { mountWithPlugins } from "../../helpers/mount";
import { videoCard } from "../../helpers/fixtures";
import { intersectAll } from "../../helpers/intersection";

const base = { items: [], loading: false, hasMore: false, loadingMore: false };

describe("VideoFeed", () => {
  it("shows 12 grid skeletons while loading", async () => {
    const { wrapper } = await mountWithPlugins(VideoFeed, { props: { ...base, loading: true } });
    expect(wrapper.findAll(".skeleton-grid")).toHaveLength(12);
    expect(wrapper.findComponent(VideoCard).exists()).toBe(false);
  });

  it("shows 5 list skeletons while loading in list mode", async () => {
    const { wrapper } = await mountWithPlugins(VideoFeed, { props: { ...base, loading: true, variant: "list" } });
    expect(wrapper.findAll(".skeleton-list")).toHaveLength(5);
  });

  it("renders the empty slot when there are no items", async () => {
    const { wrapper } = await mountWithPlugins(VideoFeed, { props: base, slots: { empty: "<p class='nothing'>Nothing here</p>" } });
    expect(wrapper.find(".nothing").exists()).toBe(true);
    expect(wrapper.findComponent(LoadMore).exists()).toBe(false);
  });

  it("renders a card per item with the variant and showChannel passed through", async () => {
    const items = [videoCard({ id: "a" }), videoCard({ id: "b" })];
    const { wrapper } = await mountWithPlugins(VideoFeed, {
      props: { ...base, items, variant: "list", showChannel: false },
      slots: { empty: "<p class='nothing'/>" },
    });
    const cards = wrapper.findAllComponents(VideoCard);
    expect(cards).toHaveLength(2);
    expect(cards[0].props()).toMatchObject({ variant: "list", showChannel: false });
    expect(wrapper.find(".nothing").exists()).toBe(false);
    expect(wrapper.find(".video-grid").exists()).toBe(false);
  });

  it("forwards hasMore/loadingMore to LoadMore and re-emits loadMore", async () => {
    const { wrapper } = await mountWithPlugins(VideoFeed, { props: { ...base, items: [videoCard()], hasMore: true, loadingMore: false } });
    expect(wrapper.findComponent(LoadMore).props()).toEqual({ hasMore: true, loading: false });
    intersectAll(true);
    expect(wrapper.emitted("loadMore")).toHaveLength(1);
  });
});
