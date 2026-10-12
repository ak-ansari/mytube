import { describe, expect, it } from "vitest";
import { VRadio } from "vuetify/components";
import VisibilityPicker from "@/components/studio/VisibilityPicker.vue";
import { VideoVisibility } from "@/types";
import { mountWithPlugins } from "../../helpers/mount";

describe("VisibilityPicker", () => {
  it("lists the three visibilities with icons and descriptions", async () => {
    const { wrapper } = await mountWithPlugins(VisibilityPicker, { props: { modelValue: VideoVisibility.Public } });
    const radios = wrapper.findAllComponents(VRadio);
    expect(radios.map((r) => r.props("value"))).toEqual(["public", "unlisted", "private"]);
    expect(wrapper.text()).toContain("Everyone can watch and find your video.");
    expect(wrapper.text()).toContain("Anyone with the link can watch.");
    expect(wrapper.text()).toContain("Only you can watch.");
    expect(wrapper.html()).toContain("mdi-earth");
    expect(wrapper.html()).toContain("mdi-link-variant");
    expect(wrapper.html()).toContain("mdi-lock-outline");
  });

  it("checks the current value", async () => {
    const { wrapper } = await mountWithPlugins(VisibilityPicker, { props: { modelValue: VideoVisibility.Unlisted } });
    const checked = wrapper.findAll("input[type=radio]").filter((i) => (i.element as HTMLInputElement).checked);
    expect(checked).toHaveLength(1);
    expect((checked[0].element as HTMLInputElement).value).toBe("unlisted");
  });

  it("emits update:modelValue when another option is picked", async () => {
    const { wrapper } = await mountWithPlugins(VisibilityPicker, { props: { modelValue: VideoVisibility.Public } });
    const priv = wrapper.findAll("input[type=radio]").find((i) => (i.element as HTMLInputElement).value === "private")!;
    await priv.setValue(true);
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual(["private"]);
  });
});
