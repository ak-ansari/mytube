import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/user.service", () => ({ default: { me: vi.fn(), mySubscriptions: vi.fn() } }));

import userService from "@/api/user.service";
import { isAuthenticated, login } from "@/auth/keycloak";
import Sidemenu from "@/components/Sidemenu.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { channel, profile } from "../../helpers/fixtures";

const signIn = (subs = [channel({ id: "c1", channel_name: "Cooking" }), channel({ id: "c2", channel_name: null, user_name: "gamer" })]) => {
  isAuthenticated.value = true;
  vi.mocked(userService.me).mockResolvedValue(profile({ id: "u1" }));
  vi.mocked(userService.mySubscriptions).mockResolvedValue(subs);
};

describe("Sidemenu", () => {
  it("signed out: main links plus a sign-in prompt", async () => {
    const { wrapper } = await mountWithPlugins(Sidemenu);
    await settle();
    expect(wrapper.find("a[href='/']").text()).toContain("Home");
    expect(wrapper.find("a[href='/subscriptions']").text()).toContain("Subscriptions");
    expect(wrapper.text()).toContain("Sign in to like videos, comment, and subscribe.");
    expect(wrapper.text()).not.toContain("Your videos");
    await wrapper.findAll("button").find((b) => b.text().includes("Sign in"))!.trigger("click");
    expect(login).toHaveBeenCalled();
    expect(userService.mySubscriptions).not.toHaveBeenCalled();
  });

  it("signed in: You section and subscribed channels", async () => {
    signIn();
    const { wrapper } = await mountWithPlugins(Sidemenu);
    await settle();
    expect(wrapper.find("a[href='/channel/u1']").text()).toContain("Your channel");
    expect(wrapper.find("a[href='/studio']").text()).toContain("Your videos");
    expect(wrapper.find("a[href='/liked']").text()).toContain("Liked videos");
    expect(wrapper.find("a[href='/channel/c1']").text()).toContain("Cooking");
    expect(wrapper.find("a[href='/channel/c2']").text()).toContain("gamer");
    expect(wrapper.text()).not.toContain("Sign in to like");
  });

  it("signed in without subscriptions shows a hint", async () => {
    signIn([]);
    const { wrapper } = await mountWithPlugins(Sidemenu);
    await settle();
    expect(wrapper.text()).toContain("Channels you subscribe to show up here.");
  });

  it("highlights the active route with the filled icon", async () => {
    const { wrapper } = await mountWithPlugins(Sidemenu, { route: "/subscriptions" });
    await settle();
    const subs = wrapper.find("a[href='/subscriptions']");
    expect(subs.classes()).toContain("v-list-item--active");
    expect(wrapper.find("a[href='/']").classes()).not.toContain("v-list-item--active");
    expect(wrapper.find("a[href='/']").html()).toContain("mdi-home-outline");
  });

  it("matches params for the active channel link", async () => {
    signIn();
    const { wrapper } = await mountWithPlugins(Sidemenu, { route: "/channel/u1" });
    await settle();
    expect(wrapper.find("a[href='/channel/u1'].guide-item").classes()).toContain("v-list-item--active");
  });

  it("rail mode: compact Home / Subscriptions / You items", async () => {
    const { wrapper } = await mountWithPlugins(Sidemenu, { props: { rail: true }, route: "/" });
    await settle();
    const items = wrapper.findAll("a.rail-item");
    expect(items.map((i) => i.text())).toEqual(["Home", "Subscriptions", "You"]);
    expect(items[0].classes()).toContain("active");
    expect(items[0].find('.v-icon').classes()).toContain('mdi-home');
    expect(items[2].attributes("href")).toBe("/studio");
  });
});
