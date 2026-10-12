import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/user.service", () => ({ default: { updateMe: vi.fn() } }));
vi.mock("@/api/file.service", () => ({ default: { getPresignedUrl: vi.fn(), uploadToS3: vi.fn() } }));

import { push } from "notivue";
import userService from "@/api/user.service";
import fileService from "@/api/file.service";
import ChannelEditDialog from "@/components/channel/ChannelEditDialog.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { profile } from "../../helpers/fixtures";
import { makeFile, pickFiles } from "../../helpers/files";

const me = profile({ id: "u1", channel_name: "Alice Films", user_name: "alice", description: "About me" });
const mountOpen = async (p = me) => {
  const r = await mountWithPlugins(ChannelEditDialog, { props: { modelValue: true, profile: p } });
  await settle();
  return r;
};
const btn = (w: Awaited<ReturnType<typeof mountOpen>>["wrapper"], text: string) => w.findAll("button").find((b) => b.text() === text)!;

describe("ChannelEditDialog", () => {
  it("prefills from the profile (falls back to user_name)", async () => {
    const { wrapper } = await mountOpen();
    expect((wrapper.find("input[type=text]").element as HTMLInputElement).value).toBe("Alice Films");
    expect((wrapper.find("textarea").element as HTMLTextAreaElement).value).toBe("About me");
    const w2 = (await mountOpen(profile({ channel_name: null, description: null }))).wrapper;
    expect((w2.find("input[type=text]").element as HTMLInputElement).value).toBe("alice");
    expect((w2.find("textarea").element as HTMLTextAreaElement).value).toBe("");
  });

  it("saves name + description, toasts and closes", async () => {
    vi.mocked(userService.updateMe).mockResolvedValue(me);
    const { wrapper } = await mountOpen();
    await wrapper.find("input[type=text]").setValue("  Alice Studio ");
    await btn(wrapper, "Save").trigger("click");
    await settle();
    expect(userService.updateMe).toHaveBeenCalledWith({ channel_name: "Alice Studio", description: "About me", avatar: undefined });
    expect(push.success).toHaveBeenCalledWith("Channel updated");
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual([false]);
  });

  it("rejects too-short names", async () => {
    const { wrapper } = await mountOpen();
    await wrapper.find("input[type=text]").setValue("ab");
    await btn(wrapper, "Save").trigger("click");
    await settle();
    expect(userService.updateMe).not.toHaveBeenCalled();
    expect(wrapper.text()).toContain("Use 3–60 characters");
  });

  it("uploads a new avatar under avatars/<userId>/ and saves its key", async () => {
    vi.mocked(fileService.getPresignedUrl).mockResolvedValue({ key: "", url: "u" });
    vi.mocked(fileService.uploadToS3).mockResolvedValue({} as never);
    vi.mocked(userService.updateMe).mockResolvedValue(me);
    const { wrapper } = await mountOpen();
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("face.png", "image/png")]);
    await settle();
    await btn(wrapper, "Save").trigger("click");
    await settle();
    const { avatar } = vi.mocked(userService.updateMe).mock.calls[0][0];
    expect(avatar).toMatch(/^avatars\/u1\/[0-9a-f-]{36}\.png$/);
  });

  it("rejects non-images, unsupported image formats and files over 5 MB", async () => {
    const { wrapper } = await mountOpen();
    expect(wrapper.find("input[type=file]").attributes("accept")).toBe("image/jpeg,image/png,image/webp,image/gif");
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("doc.pdf", "application/pdf")]);
    expect(push.error).toHaveBeenCalledWith("Please choose a JPEG, PNG, WebP or GIF image");
    vi.mocked(push.error).mockClear();
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("photo.heic", "image/heic")]);
    expect(push.error).toHaveBeenCalledWith("Please choose a JPEG, PNG, WebP or GIF image");
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("big.png", "image/png", 6 * 1024 * 1024)]);
    expect(push.error).toHaveBeenCalledWith("Pictures must be under 5 MB");
    expect(fileService.getPresignedUrl).not.toHaveBeenCalled();
  });

  it("toasts a failed avatar upload and saves without an avatar", async () => {
    vi.mocked(fileService.getPresignedUrl).mockRejectedValue({});
    vi.mocked(userService.updateMe).mockResolvedValue(me);
    const { wrapper } = await mountOpen();
    await pickFiles(wrapper.find("input[type=file]"), [makeFile("face.png", "image/png")]);
    await settle();
    expect(push.error).toHaveBeenCalledWith("Avatar upload failed");
    await btn(wrapper, "Save").trigger("click");
    await settle();
    expect(vi.mocked(userService.updateMe).mock.calls[0][0].avatar).toBeUndefined();
  });

  it("Cancel closes without saving", async () => {
    const { wrapper } = await mountOpen();
    await btn(wrapper, "Cancel").trigger("click");
    expect(wrapper.emitted("update:modelValue")?.at(-1)).toEqual([false]);
    expect(userService.updateMe).not.toHaveBeenCalled();
  });
});
