import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/file.service", () => ({ default: { getPresignedUrl: vi.fn(), uploadToS3: vi.fn() } }));

import { push } from "notivue";
import { VImg } from "vuetify/components";
import fileService from "@/api/file.service";
import ThumbnailPicker from "@/components/studio/ThumbnailPicker.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { makeFile, pickFiles } from "../../helpers/files";

const fs = vi.mocked(fileService);

describe("ThumbnailPicker", () => {
  it("shows the current thumbnail, or the empty text", async () => {
    const a = (await mountWithPlugins(ThumbnailPicker, { props: { prefix: "thumbnails/v1", currentUrl: "http://img.test/cur.jpg" } })).wrapper;
    expect(a.findComponent(VImg).props("src")).toBe("http://img.test/cur.jpg");
    expect(a.text()).toContain("Upload thumbnail");
    const b = (await mountWithPlugins(ThumbnailPicker, { props: { prefix: "p", emptyText: "After upload", hint: "Pick one" } })).wrapper;
    expect(b.text()).toContain("After upload");
    expect(b.text()).toContain("Pick one");
  });

  it("uploads a picked image under the prefix and emits key + uploading state", async () => {
    fs.getPresignedUrl.mockResolvedValue({ key: "", url: "http://minio.test/put" });
    let finish!: () => void;
    fs.uploadToS3.mockImplementation((_u, _f, onProgress) => {
      onProgress?.(40);
      return new Promise((r) => (finish = () => r({} as never)));
    });
    const { wrapper } = await mountWithPlugins(ThumbnailPicker, { props: { prefix: "thumbnails/v1" } });
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("pic.jpg", "image/jpeg")]);
    await settle();
    expect(wrapper.emitted("update:uploading")?.[0]).toEqual([true]);
    expect(wrapper.find(".overlay .v-progress-circular").exists()).toBe(true);
    finish();
    await settle();
    const key = wrapper.emitted("update:thumbnailKey")!.at(-1)![0] as string;
    expect(key).toMatch(/^thumbnails\/v1\/[0-9a-f-]{36}\.jpg$/);
    expect(fs.getPresignedUrl).toHaveBeenCalledWith(key, 1024);
    expect(wrapper.emitted("update:uploading")?.at(-1)).toEqual([false]);
    expect(wrapper.text()).toContain("Change");
    expect(wrapper.text()).toContain("Use default");
  });

  it("shows a progress overlay while the parent reports uploading", async () => {
    const { wrapper } = await mountWithPlugins(ThumbnailPicker, { props: { prefix: "p", uploading: true } });
    expect(wrapper.find(".overlay .v-progress-circular").exists()).toBe(true);
  });

  it("'Use default' clears the custom key", async () => {
    fs.getPresignedUrl.mockResolvedValue({ key: "", url: "u" });
    fs.uploadToS3.mockResolvedValue({} as never);
    const { wrapper } = await mountWithPlugins(ThumbnailPicker, { props: { prefix: "p" } });
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("pic.png", "image/png")]);
    await settle();
    await wrapper.findAll("button").find((b) => b.text() === "Use default")!.trigger("click");
    expect(wrapper.emitted("update:thumbnailKey")?.at(-1)).toEqual([""]);
    expect(wrapper.text()).toContain("Upload thumbnail");
  });

  it("only offers and accepts JPEG, PNG, WebP and GIF", async () => {
    const { wrapper } = await mountWithPlugins(ThumbnailPicker, { props: { prefix: "p" } });
    expect(wrapper.find("input[type=file]").attributes("accept")).toBe("image/jpeg,image/png,image/webp,image/gif");
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("pic.svg", "image/svg+xml")]);
    expect(push.error).toHaveBeenCalledWith("Thumbnails must be JPEG, PNG, WebP or GIF images");
    expect(fs.getPresignedUrl).not.toHaveBeenCalled();
  });

  it("rejects files over 5 MB without uploading", async () => {
    const { wrapper } = await mountWithPlugins(ThumbnailPicker, { props: { prefix: "p" } });
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("big.png", "image/png", 6 * 1024 * 1024)]);
    expect(push.error).toHaveBeenCalledWith("Thumbnails must be under 5 MB");
    expect(fs.getPresignedUrl).not.toHaveBeenCalled();
  });

  it("toasts and resets when the upload fails", async () => {
    fs.getPresignedUrl.mockRejectedValue({ message: "denied" });
    const { wrapper } = await mountWithPlugins(ThumbnailPicker, { props: { prefix: "p" } });
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("pic.png", "image/png")]);
    await settle();
    expect(push.error).toHaveBeenCalledWith("denied");
    // key never left "" (no emit) or was reset to ""
    expect((wrapper.emitted("update:thumbnailKey") ?? [[""]]).at(-1)).toEqual([""]);
    expect(wrapper.emitted("update:uploading")?.at(-1)).toEqual([false]);
    expect(wrapper.text()).toContain("Upload thumbnail");
  });

  it("ignores an empty selection", async () => {
    const { wrapper } = await mountWithPlugins(ThumbnailPicker, { props: { prefix: "p" } });
    await pickFiles(wrapper.find("input[type=file]"), []);
    expect(fs.getPresignedUrl).not.toHaveBeenCalled();
    expect(wrapper.emitted("update:uploading")).toBeUndefined();
  });
});
