import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({ default: { update: vi.fn() } }));
vi.mock("@/api/video.service", () => ({ default: { confirmVideo: vi.fn() } }));
vi.mock("@/api/file.service", () => ({ default: { getPresignedUrl: vi.fn(), uploadToS3: vi.fn() } }));

import { push } from "notivue";
import catalogService from "@/api/catalog.service";
import videoService from "@/api/video.service";
import EditVideoDialog from "@/components/studio/EditVideoDialog.vue";
import ThumbnailPicker from "@/components/studio/ThumbnailPicker.vue";
import VisibilityPicker from "@/components/studio/VisibilityPicker.vue";
import { VideoStatus, VideoVisibility, type VideoCard } from "@/types";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { videoCard, videoDetails } from "../../helpers/fixtures";

const openDialog = async (video: VideoCard) => {
  const r = await mountWithPlugins(EditVideoDialog, { props: { modelValue: false, video } });
  await r.wrapper.setProps({ modelValue: true });
  await settle();
  return r;
};
const btn = (w: Awaited<ReturnType<typeof openDialog>>["wrapper"], text: string) => w.findAll("button").find((b) => b.text() === text)!;

describe("EditVideoDialog", () => {
  it("prefills fields from the video when opened", async () => {
    const { wrapper } = await openDialog(videoCard({ title: "Hello", description: "World", visibility: VideoVisibility.Unlisted }));
    expect(wrapper.text()).toContain("Video details");
    expect((wrapper.find("input[type=text]").element as HTMLInputElement).value).toBe("Hello");
    expect((wrapper.find("textarea").element as HTMLTextAreaElement).value).toBe("World");
    expect(wrapper.findComponent(VisibilityPicker).props("modelValue")).toBe("unlisted");
    expect(wrapper.findComponent(ThumbnailPicker).props()).toMatchObject({ prefix: "thumbnails/vid-1", currentUrl: videoCard().thumbnail_url });
    expect(btn(wrapper, "Save").exists()).toBe(true);
  });

  it("Save updates title/description/visibility (no thumbnail unless changed), toasts and closes", async () => {
    vi.mocked(catalogService.update).mockResolvedValue(videoDetails());
    const { wrapper } = await openDialog(videoCard({ id: "v9", title: "Old" }));
    await wrapper.find("input[type=text]").setValue("  New title  ");
    await btn(wrapper, "Save").trigger("click");
    await settle();
    expect(catalogService.update).toHaveBeenCalledWith("v9", { title: "New title", description: "A description", visibility: "public" });
    expect(videoService.confirmVideo).not.toHaveBeenCalled();
    expect(push.success).toHaveBeenCalledWith("Changes saved");
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual([false]);
  });

  it("includes a newly uploaded thumbnail key", async () => {
    vi.mocked(catalogService.update).mockResolvedValue(videoDetails());
    const { wrapper } = await openDialog(videoCard({ id: "v9" }));
    wrapper.findComponent(ThumbnailPicker).vm.$emit("update:thumbnailKey", "thumbnails/v9/new.jpg");
    await settle();
    await btn(wrapper, "Save").trigger("click");
    await settle();
    expect(vi.mocked(catalogService.update).mock.calls[0][1]).toMatchObject({ thumbnail: "thumbnails/v9/new.jpg" });
  });

  it("does not save an invalid title", async () => {
    const { wrapper } = await openDialog(videoCard());
    await wrapper.find("input[type=text]").setValue("ab");
    await btn(wrapper, "Save").trigger("click");
    await settle();
    expect(catalogService.update).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("Use 3–100 characters");
  });

  it("Save is disabled while a thumbnail uploads", async () => {
    const { wrapper } = await openDialog(videoCard());
    wrapper.findComponent(ThumbnailPicker).vm.$emit("update:uploading", true);
    await settle();
    expect(btn(wrapper, "Save").attributes("disabled")).toBeDefined();
  });

  it("drafts are finished through confirm (Publish), then the catalog is invalidated", async () => {
    vi.mocked(videoService.confirmVideo).mockResolvedValue({} as never);
    const draft = videoCard({ id: "d1", title: null, description: null, visibility: null, status: VideoStatus.Valid });
    const { wrapper, queryClient } = await openDialog(draft);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");
    expect(wrapper.text()).toContain("Finish your upload");
    expect(wrapper.findComponent(VisibilityPicker).props("modelValue")).toBe("public");
    await btn(wrapper, "Publish").trigger("click");
    await settle();
    expect(videoService.confirmVideo).not.toHaveBeenCalled(); // empty title is invalid
    await wrapper.find("input[type=text]").setValue("Finally");
    await btn(wrapper, "Publish").trigger("click");
    await settle();
    expect(videoService.confirmVideo).toHaveBeenCalledWith({
      id: "d1",
      video: { title: "Finally", description: "", visibility: "public", thumbnail: "" },
    });
    expect(catalogService.update).not.toHaveBeenCalled();
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["catalog"] });
    expect(push.success).toHaveBeenCalledWith("Publishing — your video is processing");
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual([false]);
  });

  it("toasts a failed confirm and stays open", async () => {
    vi.mocked(videoService.confirmVideo).mockRejectedValue({ message: "video is past phase 1" });
    const { wrapper } = await openDialog(videoCard({ title: null, status: VideoStatus.ThumbnailGenerated }));
    await wrapper.find("input[type=text]").setValue("Title ok");
    await btn(wrapper, "Publish").trigger("click");
    await settle();
    expect(push.error).toHaveBeenCalledWith("video is past phase 1");
    expect(wrapper.emitted("update:modelValue")).toBeUndefined();
  });

  it("Cancel and the close icon close the dialog", async () => {
    const { wrapper } = await openDialog(videoCard());
    await btn(wrapper, "Cancel").trigger("click");
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual([false]);
    const second = (await openDialog(videoCard())).wrapper;
    await second.find("[aria-label=Close]").trigger("click");
    expect(second.emitted("update:modelValue")?.at(-1)).toEqual([false]);
  });

  it("renders nothing without a video", async () => {
    const { wrapper } = await mountWithPlugins(EditVideoDialog, { props: { modelValue: true, video: null } });
    await settle();
    expect(wrapper.find("input").exists()).toBe(false);
  });
});
