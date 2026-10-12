import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({
  default: {
    browse: vi.fn(),
    mine: vi.fn(),
    liked: vi.fn(),
    subscriptionFeed: vi.fn(),
    details: vi.fn(),
    related: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
    recordView: vi.fn(),
    react: vi.fn(),
    comments: vi.fn(),
    addComment: vi.fn(),
    deleteComment: vi.fn(),
  },
}));

import catalogService from "@/api/catalog.service";
import { isAuthenticated } from "@/auth/keycloak";
import { catalogKeys } from "@/composables/catalog/catalog.keys";
import {
  useAddComment,
  useDeleteComment,
  useDeleteVideo,
  useReactToVideo,
  useUpdateVideo,
} from "@/composables/catalog/catalog.mutation";
import {
  useBrowseVideos,
  useComments,
  useLikedVideos,
  useMyVideos,
  useRelatedVideos,
  useSubscriptionFeed,
  useVideoDetails,
} from "@/composables/catalog/catalog.query";
import { withSetup } from "../../helpers/withSetup";
import { settle } from "../../helpers/mount";
import { page, videoCard, videoDetails } from "../../helpers/fixtures";

const svc = vi.mocked(catalogService);

describe("catalogKeys", () => {
  it("builds keys under the catalog root", () => {
    expect(catalogKeys.all).toEqual(["catalog"]);
    expect(catalogKeys.browse("x")).toEqual(["catalog", "browse", "x"]);
    expect(catalogKeys.mine()).toEqual(["catalog", "mine"]);
    expect(catalogKeys.liked()).toEqual(["catalog", "liked"]);
    expect(catalogKeys.subscriptionFeed()).toEqual(["catalog", "subscription_feed"]);
    expect(catalogKeys.details("1")).toEqual(["catalog", "details", "1"]);
    expect(catalogKeys.related("1")).toEqual(["catalog", "related", "1"]);
    expect(catalogKeys.comments("1")).toEqual(["catalog", "comments", "1"]);
  });
});

describe("catalog mutations invalidate the right queries", () => {
  const setup = <T>(fn: () => T) => {
    const ctx = withSetup(fn);
    const invalidate = vi.spyOn(ctx.queryClient, "invalidateQueries");
    const setData = vi.spyOn(ctx.queryClient, "setQueryData");
    return { ...ctx, invalidate, setData };
  };

  it("useUpdateVideo -> update + invalidate all catalog", async () => {
    svc.update.mockResolvedValue(videoDetails());
    const { result, invalidate } = setup(() => useUpdateVideo());
    await result.mutateAsync({ id: "v1", input: { title: "New" } });
    expect(svc.update).toHaveBeenCalledWith("v1", { title: "New" });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog"] });
  });

  it("useDeleteVideo -> remove + invalidate all catalog", async () => {
    svc.remove.mockResolvedValue(null);
    const { result, invalidate } = setup(() => useDeleteVideo());
    await result.mutateAsync("v1");
    expect(svc.remove).toHaveBeenCalledWith("v1");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog"] });
  });

  it("useReactToVideo -> writes details cache and invalidates liked", async () => {
    const d = videoDetails({ id: "v7", my_reaction: 1, likes: 10 });
    svc.react.mockResolvedValue(d);
    const { result, invalidate, queryClient } = setup(() => useReactToVideo());
    await result.mutateAsync({ id: "v7", value: 1 });
    expect(svc.react).toHaveBeenCalledWith("v7", 1);
    expect(queryClient.getQueryData(["catalog", "details", "v7"])).toEqual(d);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog", "liked"] });
    expect(invalidate).toHaveBeenCalledTimes(1);
  });

  it("useAddComment -> invalidates comments + details of that video", async () => {
    svc.addComment.mockResolvedValue({} as never);
    const { result, invalidate } = setup(() => useAddComment(() => "v3"));
    await result.mutateAsync("hi");
    expect(svc.addComment).toHaveBeenCalledWith("v3", "hi");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog", "comments", "v3"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog", "details", "v3"] });
  });

  it("useDeleteComment -> invalidates comments + details of that video", async () => {
    svc.deleteComment.mockResolvedValue(null);
    const { result, invalidate } = setup(() => useDeleteComment(() => "v3"));
    await result.mutateAsync("c1");
    expect(svc.deleteComment).toHaveBeenCalledWith("v3", "c1");
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog", "comments", "v3"] });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog", "details", "v3"] });
  });

  it("does not invalidate on failure", async () => {
    svc.update.mockRejectedValue({ message: "nope" });
    const { result, invalidate } = setup(() => useUpdateVideo());
    await expect(result.mutateAsync({ id: "v1", input: {} })).rejects.toEqual({ message: "nope" });
    expect(invalidate).not.toHaveBeenCalled();
  });
});

describe("catalog queries", () => {
  beforeEach(() => {
    for (const fn of [svc.browse, svc.mine, svc.liked, svc.subscriptionFeed, svc.related, svc.comments]) fn.mockResolvedValue(page([videoCard()]) as never);
    svc.details.mockResolvedValue(videoDetails());
  });

  it("useBrowseVideos passes the search term", async () => {
    const { result } = withSetup(() => useBrowseVideos("cats"));
    await settle();
    expect(svc.browse).toHaveBeenCalledWith("cats", 1);
    expect(result.items.value).toHaveLength(1);
  });

  it("useBrowseVideos defaults to an empty search (home feed)", async () => {
    withSetup(() => useBrowseVideos());
    await settle();
    expect(svc.browse).toHaveBeenCalledWith("", 1);
  });

  it("auth-only lists wait for a session", async () => {
    withSetup(() => [useMyVideos(), useLikedVideos(), useSubscriptionFeed()]);
    await settle();
    expect(svc.mine).not.toHaveBeenCalled();
    expect(svc.liked).not.toHaveBeenCalled();
    expect(svc.subscriptionFeed).not.toHaveBeenCalled();
    isAuthenticated.value = true;
    await settle();
    expect(svc.mine).toHaveBeenCalledWith(1);
    expect(svc.liked).toHaveBeenCalledWith(1);
    expect(svc.subscriptionFeed).toHaveBeenCalledWith(1);
  });

  it("id-based queries are disabled for an empty id", async () => {
    withSetup(() => [useRelatedVideos(""), useComments(""), useVideoDetails("")]);
    await settle();
    expect(svc.related).not.toHaveBeenCalled();
    expect(svc.comments).not.toHaveBeenCalled();
    expect(svc.details).not.toHaveBeenCalled();
  });

  it("id-based queries fetch with the id", async () => {
    const { result } = withSetup(() => ({ rel: useRelatedVideos("v1"), com: useComments("v1"), det: useVideoDetails("v1") }));
    await settle();
    expect(svc.related).toHaveBeenCalledWith("v1", 1);
    expect(svc.comments).toHaveBeenCalledWith("v1", 1);
    expect(svc.details).toHaveBeenCalledWith("v1");
    expect(result.det.data.value?.id).toBe("vid-1");
  });

  it("useVideoDetails does not retry on error", async () => {
    svc.details.mockRejectedValue({ message: "not found" });
    const { result } = withSetup(() => useVideoDetails("missing"));
    await settle();
    expect(result.isError.value).toBe(true);
    expect(svc.details).toHaveBeenCalledTimes(1);
  });
});
