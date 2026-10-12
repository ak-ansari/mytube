import { describe, expect, it } from "vitest";
import { VIcon } from "vuetify/components";
import EmptyState from "@/components/common/EmptyState.vue";
import { mountWithPlugins } from "../../helpers/mount";

describe("EmptyState", () => {
  it("renders title, text, icon and slot", async () => {
    const { wrapper } = await mountWithPlugins(EmptyState, {
      props: { title: "No videos", text: "Upload one", icon: "mdi-magnify" },
      slots: { default: "<button class='cta'>Go</button>" },
    });
    expect(wrapper.text()).toContain("No videos");
    expect(wrapper.text()).toContain("Upload one");
    expect(wrapper.findComponent(VIcon).props("icon")).toBe("mdi-magnify");
    expect(wrapper.find(".cta").exists()).toBe(true);
  });

  it("uses the default icon and omits text when not given", async () => {
    const { wrapper } = await mountWithPlugins(EmptyState, { props: { title: "Empty" } });
    expect(wrapper.findComponent(VIcon).props("icon")).toBe("mdi-video-off-outline");
    expect(wrapper.find(".text-body-2").exists()).toBe(false);
  });
});
