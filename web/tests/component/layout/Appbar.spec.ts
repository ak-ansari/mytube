import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/user.service", () => ({ default: { me: vi.fn() } }));
vi.mock("@/api/video.service", () => ({ default: { getSuggestions: vi.fn() } }));

import userService from "@/api/user.service";
import { isAuthenticated, login, logout } from "@/auth/keycloak";
import Appbar from "@/components/Appbar.vue";
import Searchbar from "@/components/searchbar/Searchbar.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { profile } from "../../helpers/fixtures";

const mountBar = async (signedIn: boolean) => {
  if (signedIn) {
    isAuthenticated.value = true;
    vi.mocked(userService.me).mockResolvedValue(profile({ id: "u1", channel_name: "Alice Films", user_name: "alice" }));
  }
  const r = await mountWithPlugins(Appbar, { withApp: true });
  await settle();
  return r;
};

describe("Appbar", () => {
  it("has a logo link home and a search bar", async () => {
    const { wrapper } = await mountBar(false);
    expect(wrapper.find("a[aria-label='myTube home']").attributes("href")).toBe("/");
    expect(wrapper.findComponent(Searchbar).exists()).toBe(true);
  });

  it("menu button emits toggle-drawer", async () => {
    const { wrapper } = await mountBar(false);
    await wrapper.find("[aria-label=Menu]").trigger("click");
    expect(wrapper.findComponent(Appbar).emitted("toggle-drawer")).toHaveLength(1);
  });

  it("signed out: Sign in calls login(), no Create / account menu", async () => {
    const { wrapper } = await mountBar(false);
    expect(wrapper.find("[aria-label='Account menu']").exists()).toBe(false);
    expect(wrapper.find("a[href='/upload']").exists()).toBe(false);
    await wrapper.findAll("button").find((b) => b.text().includes("Sign in"))!.trigger("click");
    expect(login).toHaveBeenCalledTimes(1);
    expect(userService.me).not.toHaveBeenCalled();
  });

  it("signed in: Create links to upload and the account menu shows the profile and sign out", async () => {
    const { wrapper } = await mountBar(true);
    expect(wrapper.findAll("a[href='/upload']").length).toBeGreaterThan(0);
    expect(wrapper.text()).not.toContain("Sign in");
    await wrapper.find("[aria-label='Account menu']").trigger("click");
    await settle();
    expect(wrapper.text()).toContain("Alice Films");
    expect(wrapper.text()).toContain("@alice");
    expect(wrapper.find("a[href='/channel/u1']").exists()).toBe(true);
    expect(wrapper.find("a[href='/channel/u1?edit=1']").exists()).toBe(true);
    expect(wrapper.find("a[href='/studio']").exists()).toBe(true);
    expect(wrapper.find("a[href='/liked']").exists()).toBe(true);
    expect(wrapper.find("a[href='http://keycloak.test/account']").attributes("target")).toBe("_blank");
    await wrapper.findAll(".v-list-item").find((i) => i.text() === "Sign out")!.trigger("click");
    expect(logout).toHaveBeenCalledTimes(1);
  });

  it("theme button toggles and persists the theme", async () => {
    const { wrapper, vuetify } = await mountBar(false);
    const themeBtn = () => wrapper.find("[aria-label='Light theme'], [aria-label='Dark theme']");
    expect(themeBtn().attributes("aria-label")).toBe("Light theme");
    await themeBtn().trigger("click");
    expect(vuetify.theme.global.name.value).toBe("myTubeLight");
    expect(themeBtn().attributes("aria-label")).toBe("Dark theme");
    expect(localStorage.getItem("mytube-theme")).toBe("myTubeLight");
  });

  it("mobile search button swaps in the full-width search with a back button", async () => {
    const { wrapper } = await mountBar(false);
    await wrapper.find("[aria-label=Search].d-sm-none").trigger("click");
    expect(wrapper.find("[aria-label='Close search']").exists()).toBe(true);
    await wrapper.find("[aria-label='Close search']").trigger("click");
    expect(wrapper.find("[aria-label='Close search']").exists()).toBe(false);
  });
});
