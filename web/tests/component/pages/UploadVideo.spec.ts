import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/video.service", () => ({ default: { presignedUrl: vi.fn(), confirmVideo: vi.fn() } }));
vi.mock("@/api/file.service", () => ({ default: { uploadToS3: vi.fn(), getPresignedUrl: vi.fn() } }));
vi.mock("@/api/catalog.service", () => ({ default: { details: vi.fn(), remove: vi.fn() } }));

import { defineComponent, h } from "vue";
import { RouterView } from "vue-router";
import { push } from "notivue";
import videoService from "@/api/video.service";
import fileService from "@/api/file.service";
import catalogService from "@/api/catalog.service";
import UploadVideo from "@/pages/UploadVideo.vue";
import { VideoStatus } from "@/types";
import { createTestRouter, mountWithPlugins, settle, testRoutes } from "../../helpers/mount";
import { videoDetails } from "../../helpers/fixtures";
import { makeFile, pickFiles } from "../../helpers/files";

const presign = vi.mocked(videoService.presignedUrl);
const confirmVideo = vi.mocked(videoService.confirmVideo);
const put = vi.mocked(fileService.uploadToS3);
const details = vi.mocked(catalogService.details);
const remove = vi.mocked(catalogService.remove);

/** A PUT the test drives: report progress, then finish or fail; rejects like axios when aborted. */
const controllablePut = () => {
  const ctl = {} as { progress: (p: number) => void; finish: () => void; fail: (e: unknown) => void; signal?: AbortSignal };
  put.mockImplementation((_url, _file, onProgress, signal) => {
    ctl.signal = signal;
    ctl.progress = (p) => onProgress?.(p);
    return new Promise((resolve, reject) => {
      ctl.finish = () => resolve({} as never);
      ctl.fail = reject;
      signal?.addEventListener("abort", () => reject(new Error("canceled")));
    });
  });
  return ctl;
};

const draft = (over: Parameters<typeof videoDetails>[0] = {}) =>
  videoDetails({ id: "v1", title: null, status: VideoStatus.Uploaded, thumbnail: null, thumbnail_url: "", manifest_url: "", ...over });

const mountUpload = async () => {
  const router = createTestRouter(testRoutes.map((r) => (r.name === "Upload" ? { ...r, component: UploadVideo } : r)) as never);
  const r = await mountWithPlugins(defineComponent({ render: () => h(RouterView) }), { router, route: "/upload" });
  await settle();
  return r;
};

type W = Awaited<ReturnType<typeof mountUpload>>["wrapper"];
const publishBtn = (w: W) => w.findAll("button").find((b) => b.text() === "Publish")!;
const pickVideo = async (w: W, file = makeFile("holiday.mp4", "video/mp4", 10 * 1024 * 1024)) => {
  await pickFiles(w.find("input[type=file]"), [file]);
  await settle();
  return file;
};

describe("UploadVideo page", () => {
  it("starts on the select step", async () => {
    const { wrapper } = await mountUpload();
    expect(wrapper.text()).toContain("Upload video");
    expect(wrapper.text()).toContain("Drag and drop a video file to upload");
    expect(wrapper.find("input[type=file]").attributes("accept")).toBe("video/*");
  });

  it("rejects non-video files", async () => {
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper, makeFile("notes.txt", "text/plain"));
    expect(push.error).toHaveBeenCalledWith("Please choose a video file");
    expect(presign).not.toHaveBeenCalled();
  });

  it("rejects videos over 5 GB", async () => {
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper, makeFile("huge.mp4", "video/mp4", 6 * 1024 ** 3));
    expect(push.error).toHaveBeenCalledWith("Videos must be under 5 GB");
    expect(presign).not.toHaveBeenCalled();
  });

  it("accepts a known extension when the browser reports no MIME type", async () => {
    presign.mockResolvedValue({ id: "v1", key: "originals/v1/original.mkv", url: "http://minio.test/put" });
    controllablePut();
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper, makeFile("clip.MKV", ""));
    expect(presign).toHaveBeenCalledWith({ file_name: "clip.MKV", size: 1024 });
  });

  it("presign -> PUT with progress (capped at 99%) -> poll details -> Publish only when valid + thumbnail -> confirm", async () => {
    presign.mockResolvedValue({ id: "v1", key: "originals/v1/original.mp4", url: "http://minio.test/put" });
    const ctl = controllablePut();
    details
      .mockResolvedValueOnce(draft())
      .mockResolvedValueOnce(draft({ status: VideoStatus.Valid, thumbnail: null }))
      .mockResolvedValue(draft({ status: VideoStatus.Valid, thumbnail: "thumbnails/v1/v1.jpeg", thumbnail_url: "http://img.test/t.jpg" }));
    confirmVideo.mockResolvedValue({} as never);

    const { wrapper, queryClient } = await mountUpload();
    const file = await pickVideo(wrapper);

    expect(presign).toHaveBeenCalledWith({ file_name: "holiday.mp4", size: file.size });
    expect(put).toHaveBeenCalledWith("http://minio.test/put", file, expect.any(Function), expect.any(AbortSignal));
    expect(wrapper.text()).toContain("Uploading 0%");
    expect((wrapper.find("input[type=text]").element as HTMLInputElement).value).toBe("holiday");
    expect(wrapper.text()).toContain("10.0 MB");

    ctl.progress(50);
    await settle();
    expect(wrapper.text()).toContain("Uploading 50%");
    ctl.progress(100);
    await settle();
    expect(wrapper.text()).toContain("Uploading 99%");
    expect(details).not.toHaveBeenCalled(); // no polling before the PUT finishes
    expect(publishBtn(wrapper).attributes("disabled")).toBeDefined();

    ctl.finish();
    await settle();
    expect(details).toHaveBeenCalledWith("v1");
    expect(wrapper.text()).toContain("Upload complete. Checking your video…");
    expect(publishBtn(wrapper).attributes("disabled")).toBeDefined();

    // valid but no thumbnail yet: still locked
    await vi.waitFor(() => expect(details).toHaveBeenCalledTimes(2), { timeout: 4000 });
    await settle();
    expect(publishBtn(wrapper).attributes("disabled")).toBeDefined();

    await vi.waitFor(() => expect(details).toHaveBeenCalledTimes(3), { timeout: 4000 });
    await settle();
    expect(wrapper.text()).toContain("Checks complete. Ready to publish.");
    expect(publishBtn(wrapper).attributes("disabled")).toBeUndefined();

    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    await wrapper.find("textarea").setValue("Trip to the hills");
    await publishBtn(wrapper).trigger("click");
    await settle();
    expect(confirmVideo).toHaveBeenCalledWith({
      id: "v1",
      video: { title: "holiday", description: "Trip to the hills", visibility: "public", thumbnail: "" },
    });
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog"] });
    expect(wrapper.text()).toContain("Video published");

    // polling stopped once ready
    const calls = details.mock.calls.length;
    await new Promise((r) => setTimeout(r, 2200));
    expect(details.mock.calls.length).toBe(calls);
  }, 15_000);

  it("Publish shows an error and stays when confirm fails", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    put.mockResolvedValue({} as never);
    details.mockResolvedValue(draft({ status: VideoStatus.Valid, thumbnail: "thumbnails/v1/v1.jpeg" }));
    confirmVideo.mockRejectedValue({ message: "video is past phase 1" });
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    await vi.waitFor(() => expect(publishBtn(wrapper).attributes("disabled")).toBeUndefined());
    await publishBtn(wrapper).trigger("click");
    await settle();
    expect(push.error).toHaveBeenCalledWith("video is past phase 1");
    expect(wrapper.text()).not.toContain("Video published");
  });

  it("does not publish with an invalid title", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    put.mockResolvedValue({} as never);
    details.mockResolvedValue(draft({ status: VideoStatus.Valid, thumbnail: "t" }));
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    await vi.waitFor(() => expect(publishBtn(wrapper).attributes("disabled")).toBeUndefined());
    await wrapper.find("input[type=text]").setValue("ab");
    await publishBtn(wrapper).trigger("click");
    await settle();
    expect(confirmVideo).not.toHaveBeenCalled();
  });

  it("phase_1_failed shows the failure and keeps Publish locked", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    put.mockResolvedValue({} as never);
    details.mockResolvedValue(draft({ status: VideoStatus.Phase1Failed }));
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    await vi.waitFor(() => expect(wrapper.text()).toContain("We couldn't process this file. Try a different video."));
    expect(publishBtn(wrapper).attributes("disabled")).toBeDefined();
  });

  it("Discard mid-upload aborts the PUT, deletes the draft and returns to select", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    const ctl = controllablePut();
    remove.mockResolvedValue(null);
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    await wrapper.findAll("button").find((b) => b.text() === "Discard")!.trigger("click");
    await settle();
    expect(ctl.signal?.aborted).toBe(true);
    expect(remove).toHaveBeenCalledWith("v1");
    expect(remove).toHaveBeenCalledTimes(1);
    expect(push.info).toHaveBeenCalledWith("Upload discarded");
    expect(push.error).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("Drag and drop a video file to upload");
  });

  it("Discard after upload deletes the draft", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    put.mockResolvedValue({} as never);
    details.mockResolvedValue(draft());
    remove.mockResolvedValue(null);
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    await wrapper.findAll("button").find((b) => b.text() === "Discard")!.trigger("click");
    await settle();
    expect(remove).toHaveBeenCalledWith("v1");
    expect(wrapper.text()).toContain("Drag and drop a video file to upload");
  });

  it("a failed delete on discard after upload keeps the details step and toasts", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    put.mockResolvedValue({} as never);
    details.mockResolvedValue(draft());
    remove.mockRejectedValue({ message: "forbidden" });
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    await wrapper.findAll("button").find((b) => b.text() === "Discard")!.trigger("click");
    await settle();
    expect(push.error).toHaveBeenCalledWith("forbidden");
    expect(wrapper.text()).toContain("Discard");
  });

  it("a failed PUT toasts, deletes the draft and resets", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    put.mockRejectedValue({ message: "network down" });
    remove.mockResolvedValue(null);
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    expect(push.error).toHaveBeenCalledWith("network down");
    expect(remove).toHaveBeenCalledWith("v1");
    expect(wrapper.text()).toContain("Drag and drop a video file to upload");
  });

  it("a failed presign toasts and resets without deleting anything", async () => {
    presign.mockRejectedValue({ message: "quota" });
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    expect(push.error).toHaveBeenCalledWith("quota");
    expect(remove).not.toHaveBeenCalled();
    expect(put).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("Drag and drop a video file to upload");
  });

  it("guards leaving mid-upload: cancel stays, confirm aborts and deletes the draft", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    const ctl = controllablePut();
    remove.mockResolvedValue(null);
    const confirmSpy = vi.spyOn(window, "confirm").mockReturnValueOnce(false).mockReturnValueOnce(true);
    const { wrapper, router } = await mountUpload();
    await pickVideo(wrapper);

    await router.push("/studio");
    await settle();
    expect(confirmSpy).toHaveBeenCalledTimes(1);
    expect(router.currentRoute.value.name).toBe("Upload");
    expect(remove).not.toHaveBeenCalled();

    await router.push("/studio");
    await settle();
    expect(router.currentRoute.value.name).toBe("Studio");
    expect(ctl.signal?.aborted).toBe(true);
    expect(remove).toHaveBeenCalledWith("v1");
    confirmSpy.mockRestore();
  });

  it("registers a beforeunload prompt only while uploading", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    const ctl = controllablePut();
    details.mockResolvedValue(draft());
    const add = vi.spyOn(window, "addEventListener");
    const rem = vi.spyOn(window, "removeEventListener");
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    const handler = add.mock.calls.find(([t]) => t === "beforeunload")?.[1] as EventListener;
    expect(handler).toBeTypeOf("function");
    const evt = new Event("beforeunload", { cancelable: true });
    handler(evt);
    expect(evt.defaultPrevented).toBe(true);
    ctl.finish();
    await settle();
    expect(rem).toHaveBeenCalledWith("beforeunload", handler);
    add.mockRestore();
    rem.mockRestore();
  });

  it("'Upload another' returns to the select step after publishing", async () => {
    presign.mockResolvedValue({ id: "v1", key: "k", url: "u" });
    put.mockResolvedValue({} as never);
    details.mockResolvedValue(draft({ status: VideoStatus.Valid, thumbnail: "t" }));
    confirmVideo.mockResolvedValue({} as never);
    const { wrapper } = await mountUpload();
    await pickVideo(wrapper);
    await vi.waitFor(() => expect(publishBtn(wrapper).attributes("disabled")).toBeUndefined());
    await publishBtn(wrapper).trigger("click");
    await settle();
    expect(wrapper.find("a[href='/studio']").exists()).toBe(true);
    await wrapper.findAll("button").find((b) => b.text() === "Upload another")!.trigger("click");
    expect(wrapper.text()).toContain("Drag and drop a video file to upload");
  });
});
