import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/http", () => ({
  http: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
  errorMessage: vi.fn(),
}));
vi.mock("axios", () => ({ default: { put: vi.fn() } }));

import axios from "axios";
import { http } from "@/api/http";
import catalogService from "@/api/catalog.service";
import userService from "@/api/user.service";
import videoService from "@/api/video.service";
import fileService from "@/api/file.service";
import { VideoVisibility } from "@/types";

const h = vi.mocked(http);

beforeEach(() => {
  for (const m of [h.get, h.post, h.put, h.delete] as unknown as ReturnType<typeof vi.fn>[]) m.mockResolvedValue("RESULT");
});

describe("catalogService", () => {
  it("browse", async () => {
    await expect(catalogService.browse("cats", 2)).resolves.toBe("RESULT");
    expect(h.get).toHaveBeenCalledWith("/api/videos/browse", { params: { search: "cats", page: 2, size: 20 } });
  });
  it("mine / liked / subscriptionFeed", async () => {
    await catalogService.mine(1);
    expect(h.get).toHaveBeenLastCalledWith("/api/videos/mine", { params: { page: 1, size: 20 } });
    await catalogService.liked(3);
    expect(h.get).toHaveBeenLastCalledWith("/api/videos/liked", { params: { page: 3, size: 20 } });
    await catalogService.subscriptionFeed(1);
    expect(h.get).toHaveBeenLastCalledWith("/api/videos/subscriptions", { params: { page: 1, size: 20 } });
  });
  it("details / related", async () => {
    await catalogService.details("v1");
    expect(h.get).toHaveBeenLastCalledWith("/api/videos/v1/details");
    await catalogService.related("v1", 2);
    expect(h.get).toHaveBeenLastCalledWith("/api/videos/v1/related", { params: { page: 2, size: 12 } });
  });
  it("update / remove", async () => {
    const input = { title: "t", visibility: VideoVisibility.Private };
    await catalogService.update("v1", input);
    expect(h.put).toHaveBeenCalledWith("/api/videos/v1", input);
    await catalogService.remove("v1");
    expect(h.delete).toHaveBeenCalledWith("/api/videos/v1");
  });
  it("recordView / react", async () => {
    await catalogService.recordView("v1");
    expect(h.post).toHaveBeenCalledWith("/api/videos/v1/view");
    await catalogService.react("v1", -1);
    expect(h.put).toHaveBeenCalledWith("/api/videos/v1/reaction", { value: -1 });
    await catalogService.react("v1", 0);
    expect(h.put).toHaveBeenLastCalledWith("/api/videos/v1/reaction", { value: 0 });
  });
  it("comments", async () => {
    await catalogService.comments("v1", 1);
    expect(h.get).toHaveBeenCalledWith("/api/videos/v1/comments", { params: { page: 1, size: 20 } });
    await catalogService.addComment("v1", "hello");
    expect(h.post).toHaveBeenCalledWith("/api/videos/v1/comments", { content: "hello" });
    await catalogService.deleteComment("v1", "c9");
    expect(h.delete).toHaveBeenCalledWith("/api/videos/v1/comments/c9");
  });
});

describe("userService", () => {
  it("profile", async () => {
    await userService.me();
    expect(h.get).toHaveBeenCalledWith("/api/users/me");
    const input = { channel_name: "New", description: "d", avatar: "avatars/u/x.png" };
    await userService.updateMe(input);
    expect(h.put).toHaveBeenCalledWith("/api/users/me", input);
    await userService.mySubscriptions();
    expect(h.get).toHaveBeenLastCalledWith("/api/users/me/subscriptions");
  });
  it("channels", async () => {
    await userService.channel("c1");
    expect(h.get).toHaveBeenCalledWith("/api/channels/c1");
    await userService.channelVideos("c1", 4);
    expect(h.get).toHaveBeenLastCalledWith("/api/channels/c1/videos", { params: { page: 4, size: 20 } });
    await userService.subscribe("c1");
    expect(h.post).toHaveBeenCalledWith("/api/channels/c1/subscribe");
    await userService.unsubscribe("c1");
    expect(h.delete).toHaveBeenCalledWith("/api/channels/c1/subscribe");
  });
});

describe("videoService", () => {
  it("presignedUrl", async () => {
    await videoService.presignedUrl({ file_name: "a.mp4", size: 10 });
    expect(h.post).toHaveBeenCalledWith("/api/videos/upload/presign", { file_name: "a.mp4", size: 10 });
  });
  it("getSuggestions", async () => {
    await videoService.getSuggestions("ca");
    expect(h.get).toHaveBeenCalledWith("/api/videos/suggest", { params: { search: "ca" } });
  });
  it("confirmVideo", async () => {
    const video = { title: "T", description: "D", visibility: VideoVisibility.Unlisted, thumbnail: "" };
    await videoService.confirmVideo({ id: "v1", video });
    expect(h.post).toHaveBeenCalledWith("/api/videos/v1/confirm", video);
  });
});

describe("fileService", () => {
  it("getPresignedUrl", async () => {
    await fileService.getPresignedUrl("thumbnails/v1/x.png", 99);
    expect(h.get).toHaveBeenCalledWith("/api/files/presign", { params: { key: "thumbnails/v1/x.png", size: 99 } });
  });

  it("uploadToS3 PUTs the raw file with its content type via plain axios (no API auth)", async () => {
    vi.mocked(axios.put).mockResolvedValue({ status: 200 });
    const file = new File(["abc"], "a.png", { type: "image/png" });
    const signal = new AbortController().signal;
    await fileService.uploadToS3("http://minio.test/put?sig", file, undefined, signal);
    expect(axios.put).toHaveBeenCalledWith(
      "http://minio.test/put?sig",
      file,
      expect.objectContaining({ headers: { "Content-Type": "image/png" }, signal })
    );
    expect(h.put).not.toHaveBeenCalled();
  });

  it("reports rounded progress and ignores events without a total", async () => {
    vi.mocked(axios.put).mockResolvedValue({ status: 200 });
    const onProgress = vi.fn();
    await fileService.uploadToS3("u", new File(["x"], "v.mp4", { type: "video/mp4" }), onProgress);
    const { onUploadProgress } = vi.mocked(axios.put).mock.calls[0][2] as { onUploadProgress: (e: object) => void };
    onUploadProgress({ loaded: 1, total: 3 });
    onUploadProgress({ loaded: 50 });
    onUploadProgress({ loaded: 3, total: 3 });
    expect(onProgress.mock.calls).toEqual([[33], [100]]);
    // and without a callback it does not throw
    vi.mocked(axios.put).mockClear();
    await fileService.uploadToS3("u", new File(["x"], "v.mp4"));
    const cfg = vi.mocked(axios.put).mock.calls[0][2] as { onUploadProgress: (e: object) => void };
    expect(() => cfg.onUploadProgress({ loaded: 1, total: 2 })).not.toThrow();
  });
});
