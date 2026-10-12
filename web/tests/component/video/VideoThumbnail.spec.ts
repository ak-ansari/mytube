import { describe, expect, it } from "vitest";
import { VImg } from "vuetify/components";
import VideoThumbnail from "@/components/video/VideoThumbnail.vue";
import { mountWithPlugins } from "../../helpers/mount";

describe("VideoThumbnail", () => {
  it("renders the image url and a formatted duration badge", async () => {
    const { wrapper } = await mountWithPlugins(VideoThumbnail, { props: { url: "http://img.test/a.jpg", durationSeconds: 3725 } });
    expect(wrapper.findComponent(VImg).props("src")).toBe("http://img.test/a.jpg");
    expect(wrapper.find(".duration").text()).toBe("1:02:05");
    expect(wrapper.find(".thumb").classes()).toContain("rounded");
  });

  it("hides the duration when unknown and supports square corners", async () => {
    const { wrapper } = await mountWithPlugins(VideoThumbnail, { props: { url: "u", durationSeconds: null, square: true } });
    expect(wrapper.find(".duration").exists()).toBe(false);
    expect(wrapper.find(".thumb").classes()).not.toContain("rounded");
  });

  it("marks the image decorative with an empty alt (the card link carries the title)", async () => {
    const { wrapper } = await mountWithPlugins(VideoThumbnail, { props: { url: "http://img.test/a.jpg" } });
    expect(wrapper.findComponent(VImg).props("alt")).toBe("");
  });

  it("falls back to the placeholder when there is no url", async () => {
    const { wrapper } = await mountWithPlugins(VideoThumbnail, { props: {} });
    expect(String(wrapper.findComponent(VImg).props("src"))).toContain("no-image-placeholder");
  });

  it("falls back to the placeholder after a load error and recovers for a new url", async () => {
    const { wrapper } = await mountWithPlugins(VideoThumbnail, { props: { url: "http://img.test/broken.jpg" } });
    wrapper.findComponent(VImg).vm.$emit("error");
    await wrapper.vm.$nextTick();
    expect(String(wrapper.findComponent(VImg).props("src"))).toContain("no-image-placeholder");
    await wrapper.setProps({ url: "http://img.test/ok.jpg" });
    expect(wrapper.findComponent(VImg).props("src")).toBe("http://img.test/ok.jpg");
  });

  it("renders slot content (e.g. overlays)", async () => {
    const { wrapper } = await mountWithPlugins(VideoThumbnail, { props: { url: "u" }, slots: { default: "<b class='badge'>LIVE</b>" } });
    expect(wrapper.find(".badge").text()).toBe("LIVE");
  });
});
