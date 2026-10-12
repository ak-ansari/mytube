import { describe, expect, it } from "vitest";
import LoadMore from "@/components/common/LoadMore.vue";
import { mountWithPlugins } from "../../helpers/mount";
import { MockIntersectionObserver, intersectAll } from "../../helpers/intersection";

describe("LoadMore", () => {
  it("observes its sentinel with a 400px root margin", async () => {
    const { wrapper } = await mountWithPlugins(LoadMore, { props: { hasMore: true, loading: false } });
    const obs = MockIntersectionObserver.instances.at(-1)!;
    expect(obs.rootMargin).toBe("400px");
    expect(obs.elements[0]).toBe(wrapper.element);
  });

  it("emits load when the sentinel becomes visible and there is more", async () => {
    const { wrapper } = await mountWithPlugins(LoadMore, { props: { hasMore: true, loading: false } });
    intersectAll(false);
    expect(wrapper.emitted("load")).toBeUndefined();
    intersectAll(true);
    expect(wrapper.emitted("load")).toHaveLength(1);
  });

  it("does not emit when there is nothing more or while loading", async () => {
    const a = (await mountWithPlugins(LoadMore, { props: { hasMore: false, loading: false } })).wrapper;
    const b = (await mountWithPlugins(LoadMore, { props: { hasMore: true, loading: true } })).wrapper;
    intersectAll(true);
    expect(a.emitted("load")).toBeUndefined();
    expect(b.emitted("load")).toBeUndefined();
  });

  it("asks again when a page finishes loading while still visible", async () => {
    const { wrapper } = await mountWithPlugins(LoadMore, { props: { hasMore: true, loading: true } });
    intersectAll(true);
    expect(wrapper.emitted("load")).toBeUndefined();
    await wrapper.setProps({ loading: false });
    expect(wrapper.emitted("load")).toHaveLength(1);
  });

  it("asks when hasMore flips to true while visible", async () => {
    const { wrapper } = await mountWithPlugins(LoadMore, { props: { hasMore: false, loading: false } });
    intersectAll(true);
    await wrapper.setProps({ hasMore: true });
    expect(wrapper.emitted("load")).toHaveLength(1);
  });

  it("does not ask again after loading if it scrolled out of view", async () => {
    const { wrapper } = await mountWithPlugins(LoadMore, { props: { hasMore: true, loading: true } });
    intersectAll(false);
    await wrapper.setProps({ loading: false });
    expect(wrapper.emitted("load")).toBeUndefined();
  });

  it("shows a spinner while loading and disconnects on unmount", async () => {
    const { wrapper } = await mountWithPlugins(LoadMore, { props: { hasMore: true, loading: true } });
    expect(wrapper.find(".v-progress-circular").exists()).toBe(true);
    const obs = MockIntersectionObserver.instances.at(-1)!;
    wrapper.unmount();
    expect(obs.disconnected).toBe(true);
  });
});
