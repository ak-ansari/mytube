import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/user.service", () => ({ default: { me: vi.fn(), mySubscriptions: vi.fn() } }));
vi.mock("@/api/video.service", () => ({ default: { getSuggestions: vi.fn() } }));

import { defineComponent, h } from "vue";
import { VNavigationDrawer } from "vuetify/components";
import MainLayout from "@/layouts/Mainlayout.vue";
import Appbar from "@/components/Appbar.vue";
import Sidemenu from "@/components/Sidemenu.vue";
import { createTestRouter, mountWithPlugins, settle, testRoutes } from "../../helpers/mount";

const Page = defineComponent({ render: () => h("div", { class: "page-content" }, "page") });

const router = () =>
  createTestRouter([
    {
      path: "/",
      component: MainLayout,
      children: testRoutes.map((r) => ({ ...r, path: r.path.replace(/^\//, ""), component: Page })) as never,
    },
  ]);

describe("Mainlayout", () => {
  it("renders app bar, side menu and the routed page", async () => {
    const { wrapper } = await mountWithPlugins(defineComponent({ render: () => h(MainLayout) }), { router: router() });
    await settle();
    expect(wrapper.findComponent(Appbar).exists()).toBe(true);
    expect(wrapper.findComponent(Sidemenu).exists()).toBe(true);
  });

  it("on narrow screens the drawer is a closed overlay that the menu button toggles and navigation closes", async () => {
    const r = router();
    const { wrapper } = await mountWithPlugins(defineComponent({ render: () => h(MainLayout) }), { router: r });
    await settle();
    const drawer = () => wrapper.findComponent(VNavigationDrawer);
    expect(drawer().props("temporary")).toBe(true);
    expect(drawer().props("modelValue")).toBe(false);
    await wrapper.find("[aria-label=Menu]").trigger("click");
    expect(drawer().props("modelValue")).toBe(true);
    await r.push("/liked");
    await settle();
    expect(drawer().props("modelValue")).toBe(false);
  });
});
