import { describe, expect, it } from "vitest";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";

const buttons = (w: { findAll: (s: string) => { text: () => string; trigger: (e: string) => Promise<void> }[] }) => w.findAll("button");

describe("ConfirmDialog", () => {
  it("renders nothing while closed", async () => {
    const { wrapper } = await mountWithPlugins(ConfirmDialog, { props: { modelValue: false, title: "Delete?" } });
    expect(wrapper.text()).not.toContain("Delete?");
  });

  it("shows title, text and default confirm label when open", async () => {
    const { wrapper } = await mountWithPlugins(ConfirmDialog, { props: { modelValue: true, title: "Delete?", text: "Gone forever" } });
    await settle();
    expect(wrapper.text()).toContain("Delete?");
    expect(wrapper.text()).toContain("Gone forever");
    expect(buttons(wrapper).map((b) => b.text())).toEqual(["Cancel", "Confirm"]);
  });

  it("slot overrides text; confirm emits confirm; danger colours the button", async () => {
    const { wrapper } = await mountWithPlugins(ConfirmDialog, {
      props: { modelValue: true, title: "Delete?", confirmText: "Delete forever", danger: true },
      slots: { default: "Custom <b>body</b>" },
    });
    await settle();
    expect(wrapper.text()).toContain("Custom body");
    const confirm = buttons(wrapper).find((b) => b.text() === "Delete forever")!;
    expect(wrapper.find(".bg-error").exists()).toBe(true);
    await confirm.trigger("click");
    expect(wrapper.emitted("confirm")).toHaveLength(1);
    expect(wrapper.emitted("update:modelValue")).toBeUndefined();
  });

  it("cancel closes via v-model without confirming", async () => {
    const { wrapper } = await mountWithPlugins(ConfirmDialog, { props: { modelValue: true, title: "Sure?" } });
    await settle();
    await buttons(wrapper).find((b) => b.text() === "Cancel")!.trigger("click");
    expect(wrapper.emitted("update:modelValue")?.[0]).toEqual([false]);
    expect(wrapper.emitted("confirm")).toBeUndefined();
  });

  it("shows a loading confirm button", async () => {
    const { wrapper } = await mountWithPlugins(ConfirmDialog, { props: { modelValue: true, title: "Sure?", loading: true } });
    await settle();
    expect(wrapper.find(".v-btn--loading").exists()).toBe(true);
  });
});
