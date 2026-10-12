import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/user.service", () => ({
  default: {
    me: vi.fn(),
    updateMe: vi.fn(),
    mySubscriptions: vi.fn(),
    channel: vi.fn(),
    channelVideos: vi.fn(),
    subscribe: vi.fn(),
    unsubscribe: vi.fn(),
  },
}));

import userService from "@/api/user.service";
import { isAuthenticated } from "@/auth/keycloak";
import { userKeys } from "@/composables/user/user.keys";
import { useToggleSubscription, useUpdateProfile } from "@/composables/user/user.mutation";
import { useChannel, useChannelVideos, useMe, useMySubscriptions } from "@/composables/user/user.query";
import { withSetup } from "../../helpers/withSetup";
import { settle } from "../../helpers/mount";
import { channel, page, profile, videoCard } from "../../helpers/fixtures";

const svc = vi.mocked(userService);

describe("userKeys", () => {
  it("builds keys", () => {
    expect(userKeys.me()).toEqual(["user", "me"]);
    expect(userKeys.subscriptions()).toEqual(["user", "subscriptions"]);
    expect(userKeys.channel("c")).toEqual(["user", "channel", "c"]);
    expect(userKeys.channelVideos("c")).toEqual(["user", "channel_videos", "c"]);
  });
});

describe("user mutations", () => {
  it("useUpdateProfile writes me, invalidates the channel and the whole catalog", async () => {
    const p = profile({ id: "u1", channel_name: "Renamed" });
    svc.updateMe.mockResolvedValue(p);
    const { result, queryClient } = withSetup(() => useUpdateProfile());
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    await result.mutateAsync({ channel_name: "Renamed" });
    expect(svc.updateMe).toHaveBeenCalledWith({ channel_name: "Renamed" });
    expect(queryClient.getQueryData(["user", "me"])).toEqual(p);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["user", "channel", "u1"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog"] });
  });

  it.each([
    [true, "subscribe"],
    [false, "unsubscribe"],
  ] as const)("useToggleSubscription(subscribe=%s) calls %s and refreshes subscription data", async (subscribe, method) => {
    const c = channel({ id: "c9", is_subscribed: subscribe });
    svc.subscribe.mockResolvedValue(c);
    svc.unsubscribe.mockResolvedValue(c);
    const { result, queryClient } = withSetup(() => useToggleSubscription());
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    await result.mutateAsync({ channelId: "c9", subscribe });
    expect(svc[method]).toHaveBeenCalledWith("c9");
    expect(svc[method === "subscribe" ? "unsubscribe" : "subscribe"]).not.toHaveBeenCalled();
    expect(queryClient.getQueryData(["user", "channel", "c9"])).toEqual(c);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["user", "subscriptions"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog", "subscription_feed"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog", "details"] });
  });
});

describe("user queries", () => {
  it("useMe / useMySubscriptions only run when signed in", async () => {
    svc.me.mockResolvedValue(profile());
    svc.mySubscriptions.mockResolvedValue([channel()]);
    const { result } = withSetup(() => ({ me: useMe(), subs: useMySubscriptions() }));
    await settle();
    expect(svc.me).not.toHaveBeenCalled();
    expect(svc.mySubscriptions).not.toHaveBeenCalled();
    isAuthenticated.value = true;
    await settle();
    expect(result.me.data.value?.user_name).toBe("alice");
    expect(result.subs.data.value).toHaveLength(1);
  });

  it("useChannel / useChannelVideos use the id and skip empty ids", async () => {
    svc.channel.mockResolvedValue(channel());
    svc.channelVideos.mockResolvedValue(page([videoCard()]));
    withSetup(() => [useChannel(""), useChannelVideos("")]);
    await settle();
    expect(svc.channel).not.toHaveBeenCalled();
    const { result } = withSetup(() => ({ c: useChannel("c1"), v: useChannelVideos("c1") }));
    await settle();
    expect(svc.channel).toHaveBeenCalledWith("c1");
    expect(svc.channelVideos).toHaveBeenCalledWith("c1", 1);
    expect(result.v.items.value).toHaveLength(1);
  });
});
