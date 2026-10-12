import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/video.service", () => ({ default: { getSuggestions: vi.fn() } }));

import videoService from "@/api/video.service";
import Searchbar from "@/components/searchbar/Searchbar.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";

const suggest = vi.mocked(videoService.getSuggestions);

const typeAndWait = async (wrapper: Awaited<ReturnType<typeof mountWithPlugins>>["wrapper"], text: string) => {
  const input = wrapper.find("input");
  await input.trigger("focus");
  await input.setValue(text);
  await vi.waitFor(() => expect(suggest).toHaveBeenCalledWith(text.trim()), { timeout: 2000 });
  await settle();
};

describe("Searchbar", () => {
  it("debounces suggestions (one request for a burst of typing)", async () => {
    suggest.mockResolvedValue([]);
    const { wrapper } = await mountWithPlugins(Searchbar);
    const input = wrapper.find("input");
    await input.setValue("c");
    await input.setValue("ca");
    await input.setValue("cat");
    expect(suggest).not.toHaveBeenCalled();
    await vi.waitFor(() => expect(suggest).toHaveBeenCalled(), { timeout: 2000 });
    await settle();
    expect(suggest).toHaveBeenCalledTimes(1);
    expect(suggest).toHaveBeenCalledWith("cat");
  });

  it("does not request suggestions for blank input", async () => {
    const { wrapper } = await mountWithPlugins(Searchbar);
    await wrapper.find("input").setValue("   ");
    await new Promise((r) => setTimeout(r, 300));
    await settle();
    expect(suggest).not.toHaveBeenCalled();
  });

  it("shows up to 8 suggestions while focused, hides them on blur", async () => {
    suggest.mockResolvedValue(Array.from({ length: 10 }, (_, i) => `cat ${i}`));
    const { wrapper } = await mountWithPlugins(Searchbar);
    await typeAndWait(wrapper, "cat");
    expect(wrapper.findAll(".suggestion")).toHaveLength(8);
    await wrapper.find("input").trigger("blur");
    expect(wrapper.findAll(".suggestion")).toHaveLength(0);
  });

  it("submitting navigates to /results?q= with the trimmed query", async () => {
    suggest.mockResolvedValue([]);
    const { wrapper, router } = await mountWithPlugins(Searchbar);
    await wrapper.find("input").setValue("  funny cats ");
    await wrapper.find("form").trigger("submit");
    await settle();
    expect(router.currentRoute.value.name).toBe("Results");
    expect(router.currentRoute.value.query).toEqual({ q: "funny cats" });
  });

  it("the search button submits too", async () => {
    const { wrapper, router } = await mountWithPlugins(Searchbar);
    await wrapper.find("input").setValue("dogs");
    await wrapper.find("button[type=submit]").trigger("submit");
    await settle();
    expect(router.currentRoute.value.fullPath).toBe("/results?q=dogs");
  });

  it("empty submit does nothing", async () => {
    const { wrapper, router } = await mountWithPlugins(Searchbar);
    await wrapper.find("form").trigger("submit");
    await settle();
    expect(router.currentRoute.value.fullPath).toBe("/");
  });

  it("clicking a suggestion searches for it", async () => {
    suggest.mockResolvedValue(["cat videos", "cat memes"]);
    const { wrapper, router } = await mountWithPlugins(Searchbar);
    await typeAndWait(wrapper, "cat");
    await wrapper.findAll(".suggestion")[1].trigger("mousedown");
    await settle();
    expect(router.currentRoute.value.query.q).toBe("cat memes");
    expect((wrapper.find("input").element as HTMLInputElement).value).toBe("cat memes");
  });

  it("arrow keys highlight a suggestion (wrapping) and Enter submits it", async () => {
    suggest.mockResolvedValue(["one", "two", "three"]);
    const { wrapper, router } = await mountWithPlugins(Searchbar);
    await typeAndWait(wrapper, "t");
    const input = wrapper.find("input");
    await input.trigger("keydown", { key: "ArrowDown" });
    await input.trigger("keydown", { key: "ArrowDown" });
    expect(wrapper.findAll(".suggestion")[1].classes()).toContain("active");
    await input.trigger("keydown", { key: "ArrowUp" });
    await input.trigger("keydown", { key: "ArrowUp" });
    expect(wrapper.findAll(".suggestion")[2].classes()).toContain("active");
    await wrapper.find("form").trigger("submit");
    await settle();
    expect(router.currentRoute.value.query.q).toBe("three");
  });

  it("Escape closes the suggestions", async () => {
    suggest.mockResolvedValue(["one"]);
    const { wrapper } = await mountWithPlugins(Searchbar);
    await typeAndWait(wrapper, "o");
    expect(wrapper.findAll(".suggestion")).toHaveLength(1);
    await wrapper.find("input").trigger("keydown", { key: "Escape" });
    expect(wrapper.findAll(".suggestion")).toHaveLength(0);
  });

  it("is prefilled from ?q= and follows route changes", async () => {
    const { wrapper, router } = await mountWithPlugins(Searchbar, { route: "/results?q=dogs" });
    expect((wrapper.find("input").element as HTMLInputElement).value).toBe("dogs");
    await router.push("/results?q=birds");
    await settle();
    expect((wrapper.find("input").element as HTMLInputElement).value).toBe("birds");
    await router.push("/");
    await settle();
    expect((wrapper.find("input").element as HTMLInputElement).value).toBe("");
  });
});
