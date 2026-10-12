import { describe, expect, it, vi } from "vitest";
import { ref } from "vue";

vi.mock("@/api/video.service", () => ({ default: { getSuggestions: vi.fn() } }));
vi.mock("@/api/file.service", () => ({ default: { getPresignedUrl: vi.fn(), uploadToS3: vi.fn() } }));

import videoService from "@/api/video.service";
import fileService from "@/api/file.service";
import { useSuggestSearch } from "@/composables/video/video.query";
import { videoKeys } from "@/composables/video/video.keys";
import { IMAGE_ACCEPT, isAcceptedImage, uploadObject } from "@/composables/file/uploadObject";
import { withSetup } from "../../helpers/withSetup";
import { settle } from "../../helpers/mount";

describe("useSuggestSearch", () => {
  it("key embeds the search ref", () => {
    const s = ref("x");
    expect(videoKeys.suggest(s)).toEqual(["search_suggestions", s]);
  });

  it("only fetches for a non-empty term and refetches when it changes", async () => {
    vi.mocked(videoService.getSuggestions).mockImplementation(async (s) => [`${s} one`, `${s} two`]);
    const term = ref("");
    const { result } = withSetup(() => useSuggestSearch(term));
    await settle();
    expect(videoService.getSuggestions).not.toHaveBeenCalled();
    term.value = "cat";
    await settle();
    expect(videoService.getSuggestions).toHaveBeenCalledWith("cat");
    expect(result.data.value).toEqual(["cat one", "cat two"]);
  });
});

describe("uploadObject", () => {
  it("presigns, PUTs the file and returns the object key", async () => {
    vi.mocked(fileService.getPresignedUrl).mockResolvedValue({ key: "ignored", url: "http://minio.test/put" });
    vi.mocked(fileService.uploadToS3).mockResolvedValue({} as never);
    const file = new File(["png"], "Me.PNG", { type: "image/png" });
    const onProgress = vi.fn();

    const key = await uploadObject("avatars/u1", file, onProgress);

    expect(key).toMatch(/^avatars\/u1\/[0-9a-f-]{36}\.png$/); // lowercased: the API checks the extension
    expect(fileService.getPresignedUrl).toHaveBeenCalledWith(key, file.size);
    expect(fileService.uploadToS3).toHaveBeenCalledWith("http://minio.test/put", file, onProgress);
    const presignOrder = vi.mocked(fileService.getPresignedUrl).mock.invocationCallOrder[0];
    const putOrder = vi.mocked(fileService.uploadToS3).mock.invocationCallOrder[0];
    expect(presignOrder).toBeLessThan(putOrder);
  });

  it("uses a unique key per upload", async () => {
    vi.mocked(fileService.getPresignedUrl).mockResolvedValue({ key: "", url: "u" });
    vi.mocked(fileService.uploadToS3).mockResolvedValue({} as never);
    const f = new File(["x"], "a.jpg");
    expect(await uploadObject("p", f)).not.toBe(await uploadObject("p", f));
  });

  it("falls back to .bin for names ending in a dot", async () => {
    vi.mocked(fileService.getPresignedUrl).mockResolvedValue({ key: "", url: "u" });
    vi.mocked(fileService.uploadToS3).mockResolvedValue({} as never);
    expect(await uploadObject("p", new File(["x"], "weird."))).toMatch(/\.bin$/);
  });

  it("takes the extension from the MIME type when the name has none", async () => {
    vi.mocked(fileService.getPresignedUrl).mockResolvedValue({ key: "", url: "u" });
    vi.mocked(fileService.uploadToS3).mockResolvedValue({} as never);
    expect(await uploadObject("p", new File(["x"], "avatar", { type: "image/webp" }))).toMatch(/^p\/[0-9a-f-]{36}\.webp$/);
    expect(await uploadObject("p", new File(["x"], "avatar"))).toMatch(/\.bin$/);
  });
});

describe("isAcceptedImage", () => {
  it("accepts the formats the API allows, by MIME type or extension", () => {
    expect(IMAGE_ACCEPT).toBe("image/jpeg,image/png,image/webp,image/gif");
    for (const type of ["image/jpeg", "image/png", "image/webp", "image/gif"]) {
      expect(isAcceptedImage(new File(["x"], "a", { type }))).toBe(true);
    }
    expect(isAcceptedImage(new File(["x"], "a.heic", { type: "image/heic" }))).toBe(false);
    expect(isAcceptedImage(new File(["x"], "a.svg", { type: "image/svg+xml" }))).toBe(false);
    expect(isAcceptedImage(new File(["x"], "doc.pdf", { type: "application/pdf" }))).toBe(false);
    // no MIME type (some OS/browser combos): fall back to the extension
    expect(isAcceptedImage(new File(["x"], "pic.JPG"))).toBe(true);
    expect(isAcceptedImage(new File(["x"], "pic.bmp"))).toBe(false);
    expect(isAcceptedImage(new File(["x"], "pic"))).toBe(false);
  });

  it("propagates presign failures without uploading", async () => {
    vi.mocked(fileService.getPresignedUrl).mockRejectedValue({ message: "forbidden" });
    await expect(uploadObject("p", new File(["x"], "a.jpg"))).rejects.toEqual({ message: "forbidden" });
    expect(fileService.uploadToS3).not.toHaveBeenCalled();
  });

  it("propagates PUT failures", async () => {
    vi.mocked(fileService.getPresignedUrl).mockResolvedValue({ key: "", url: "u" });
    vi.mocked(fileService.uploadToS3).mockRejectedValue(new Error("network"));
    await expect(uploadObject("p", new File(["x"], "a.jpg"))).rejects.toThrow("network");
  });
});
