import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({ default: { mine: vi.fn(), remove: vi.fn(), update: vi.fn() } }));
vi.mock("@/api/video.service", () => ({ default: { confirmVideo: vi.fn() } }));
vi.mock("@/api/file.service", () => ({ default: { getPresignedUrl: vi.fn(), uploadToS3: vi.fn() } }));

import { push } from "notivue";
import catalogService from "@/api/catalog.service";
import { isAuthenticated } from "@/auth/keycloak";
import Studio from "@/pages/Studio.vue";
import EditVideoDialog from "@/components/studio/EditVideoDialog.vue";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import { VideoStatus } from "@/types";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { page, videoCard } from "../../helpers/fixtures";

const mine = vi.mocked(catalogService.mine);
const ago = (ms: number) => new Date(Date.now() - ms).toISOString();

const published = videoCard({ id: "pub", title: "Published one", views: 4321 });
const draft = videoCard({ id: "draft", title: null, description: null, status: VideoStatus.Valid, visibility: null });
const stale = videoCard({ id: "stale", title: null, status: VideoStatus.UploadPending, updated_at: ago(20 * 60_000), created_at: ago(20 * 60_000) });
const uploading = videoCard({ id: "up", title: null, status: VideoStatus.Uploaded, updated_at: ago(60_000) });

const mountStudio = async (items = [published, draft, stale]) => {
  isAuthenticated.value = true;
  mine.mockResolvedValue(page(items));
  const r = await mountWithPlugins(Studio);
  await settle();
  return r;
};

const rowOf = (w: Awaited<ReturnType<typeof mountStudio>>["wrapper"], text: string) => w.findAll("tbody tr").find((r) => r.text().includes(text))!;

describe("Studio page", () => {
  afterEach(() => vi.useRealTimers());

  it("shows the empty state when there are no videos", async () => {
    const { wrapper } = await mountStudio([]);
    expect(wrapper.text()).toContain("No content available");
  });

  it("renders a row per video with status chips", async () => {
    const { wrapper } = await mountStudio();
    expect(wrapper.findAll("tbody tr")).toHaveLength(3);
    const pub = rowOf(wrapper, "Published one");
    expect(pub.find(".v-chip").text()).toBe("Published");
    expect(pub.text()).toContain("4.3K");
    expect(pub.find(".text-capitalize").text()).toBe("public");
    expect(pub.html()).toContain("mdi-earth");
    const rows = wrapper.findAll("tbody tr");
    expect(rows[1].find(".v-chip").text()).toBe("Draft");
    expect(rows[1].text()).toContain("Untitled draft");
    expect(rows[2].find(".v-chip").text()).toBe("Upload incomplete");
  });

  it("only drafts get a Finish button; Watch is disabled unless published", async () => {
    const { wrapper } = await mountStudio();
    const rows = wrapper.findAll("tbody tr");
    expect(rows[0].findAll(".v-btn").some((b) => b.text() === "Finish")).toBe(false);
    expect(rows[1].findAll(".v-btn").some((b) => b.text() === "Finish")).toBe(true);
    expect(rows[2].findAll(".v-btn").some((b) => b.text() === "Finish")).toBe(false);
    expect(rows[0].find('[aria-label="Watch"]').classes()).not.toContain("v-btn--disabled");
    expect(rows[1].find('[aria-label="Watch"]').classes()).toContain("v-btn--disabled");
  });

  it("Finish opens the edit dialog in publish mode for the draft", async () => {
    const { wrapper } = await mountStudio();
    await wrapper.findAll("tbody tr")[1].findAll(".v-btn").find((b) => b.text() === "Finish")!.trigger("click");
    await settle();
    const dlg = wrapper.findComponent(EditVideoDialog);
    expect(dlg.props("modelValue")).toBe(true);
    expect(dlg.props("video")?.id).toBe("draft");
    expect(wrapper.text()).toContain("Finish your upload");
  });

  it("Edit opens the details dialog for a titled video", async () => {
    const { wrapper } = await mountStudio();
    await rowOf(wrapper, "Published one").find('[aria-label="Edit"]').trigger("click");
    await settle();
    expect(wrapper.findComponent(EditVideoDialog).props("video")?.id).toBe("pub");
    expect(wrapper.text()).toContain("Video details");
  });

  it("Delete asks for confirmation, then removes and toasts", async () => {
    vi.mocked(catalogService.remove).mockResolvedValue(null);
    const { wrapper } = await mountStudio();
    await rowOf(wrapper, "Published one").find('[aria-label="Delete"]').trigger("click");
    await settle();
    const confirm = wrapper.findComponent(ConfirmDialog);
    expect(confirm.props("modelValue")).toBe(true);
    expect(wrapper.text()).toContain("“Published one” and all of its comments");
    await confirm.findAll(".v-card-actions .v-btn").find((b) => b.text() === "Delete forever")!.trigger("click");
    await settle();
    expect(catalogService.remove).toHaveBeenCalledWith("pub");
    expect(push.success).toHaveBeenCalledWith("Video deleted");
    expect(wrapper.findComponent(ConfirmDialog).props("modelValue")).toBe(false);
  });

  it("polls every 4s while a video is processing", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { wrapper } = await mountStudio([published, uploading]);
    expect(rowOf(wrapper, "Uploading").exists()).toBe(true);
    const calls = mine.mock.calls.length;
    await vi.advanceTimersByTimeAsync(4100);
    await settle();
    expect(mine.mock.calls.length).toBeGreaterThan(calls);
    wrapper.unmount();
  });

  it("does not poll for stale drafts or finished videos", async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { wrapper } = await mountStudio([published, stale, draft]);
    const calls = mine.mock.calls.length;
    await vi.advanceTimersByTimeAsync(9000);
    await settle();
    expect(mine.mock.calls.length).toBe(calls);
    wrapper.unmount();
  });
});
