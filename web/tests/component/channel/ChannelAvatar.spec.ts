import { describe, expect, it } from "vitest";
import { VAvatar, VImg } from "vuetify/components";
import ChannelAvatar from "@/components/channel/ChannelAvatar.vue";
import { mountWithPlugins } from "../../helpers/mount";

describe("ChannelAvatar", () => {
  it("renders the avatar image when there is a url", async () => {
    const { wrapper } = await mountWithPlugins(ChannelAvatar, {
      props: { channel: { id: "1", channel_name: "Alice", avatar_url: "http://img.test/a.png" }, size: 48 },
    });
    expect(wrapper.findComponent(VImg).props("src")).toBe("http://img.test/a.png");
    expect(wrapper.findComponent(VImg).props("alt")).toBe("Alice");
    expect(wrapper.findComponent(VAvatar).props("size")).toBe(48);
    expect(wrapper.find(".initial").exists()).toBe(false);
  });

  it("renders the uppercase initial with a deterministic colour otherwise", async () => {
    const ch = { id: "abc", channel_name: "  zed tv", avatar_url: "" };
    const a = (await mountWithPlugins(ChannelAvatar, { props: { channel: ch } })).wrapper;
    const b = (await mountWithPlugins(ChannelAvatar, { props: { channel: ch } })).wrapper;
    expect(a.find(".initial").text()).toBe("Z");
    expect(a.findComponent(VImg).exists()).toBe(false);
    const style = a.find(".channel-avatar").attributes("style");
    expect(style).toMatch(/background/);
    expect(style).toBe(b.find(".channel-avatar").attributes("style"));
    // default size 36 -> font 16px
    expect(a.find(".initial").attributes("style")).toContain("font-size: 16px");
  });

  it("falls back to user_name, then '?'", async () => {
    expect((await mountWithPlugins(ChannelAvatar, { props: { channel: { id: "1", channel_name: null, user_name: "bob" } } })).wrapper.find(".initial").text()).toBe("B");
    expect((await mountWithPlugins(ChannelAvatar, { props: { channel: null } })).wrapper.find(".initial").text()).toBe("?");
    expect((await mountWithPlugins(ChannelAvatar, { props: {} })).wrapper.find(".initial").text()).toBe("?");
  });

  it("different ids can get different colours", async () => {
    const colours = new Set<string>();
    for (const id of ["a", "b", "c", "d", "e", "f"]) {
      const { wrapper } = await mountWithPlugins(ChannelAvatar, { props: { channel: { id, channel_name: "x" } } });
      colours.add(wrapper.find(".channel-avatar").attributes("style")!);
    }
    expect(colours.size).toBeGreaterThan(1);
  });
});
