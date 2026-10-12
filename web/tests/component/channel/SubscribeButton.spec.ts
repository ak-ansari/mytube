import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/user.service", () => ({ default: { subscribe: vi.fn(), unsubscribe: vi.fn() } }));

import { push } from "notivue";
import userService from "@/api/user.service";
import { isAuthenticated, login } from "@/auth/keycloak";
import SubscribeButton from "@/components/channel/SubscribeButton.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { channel } from "../../helpers/fixtures";

describe("SubscribeButton", () => {
  it("shows Subscribe / Subscribed", async () => {
    expect((await mountWithPlugins(SubscribeButton, { props: { channelId: "c1", subscribed: false } })).wrapper.text()).toBe("Subscribe");
    expect((await mountWithPlugins(SubscribeButton, { props: { channelId: "c1", subscribed: true } })).wrapper.text()).toBe("Subscribed");
  });

  it("anonymous click calls login() and makes no request", async () => {
    const { wrapper } = await mountWithPlugins(SubscribeButton, { props: { channelId: "c1", subscribed: false } });
    await wrapper.find("button").trigger("click");
    expect(login).toHaveBeenCalledTimes(1);
    expect(userService.subscribe).not.toHaveBeenCalled();
  });

  it("signed-in click subscribes and toasts", async () => {
    isAuthenticated.value = true;
    vi.mocked(userService.subscribe).mockResolvedValue(channel({ id: "c1", is_subscribed: true }));
    const { wrapper, queryClient } = await mountWithPlugins(SubscribeButton, { props: { channelId: "c1", subscribed: false } });
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    await wrapper.find("button").trigger("click");
    await settle();
    expect(userService.subscribe).toHaveBeenCalledWith("c1");
    expect(push.success).toHaveBeenCalledWith("Subscription added");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["user", "subscriptions"] });
    expect(login).not.toHaveBeenCalled();
  });

  it("signed-in click on Subscribed unsubscribes", async () => {
    isAuthenticated.value = true;
    vi.mocked(userService.unsubscribe).mockResolvedValue(channel({ id: "c1" }));
    const { wrapper } = await mountWithPlugins(SubscribeButton, { props: { channelId: "c1", subscribed: true } });
    await wrapper.find("button").trigger("click");
    await settle();
    expect(userService.unsubscribe).toHaveBeenCalledWith("c1");
    expect(push.success).toHaveBeenCalledWith("Subscription removed");
  });

  it("no success toast when the request fails", async () => {
    isAuthenticated.value = true;
    vi.mocked(userService.subscribe).mockRejectedValue({ message: "nope" });
    const { wrapper } = await mountWithPlugins(SubscribeButton, { props: { channelId: "c1", subscribed: false } });
    await wrapper.find("button").trigger("click");
    await settle();
    expect(push.success).not.toHaveBeenCalled();
  });
});
