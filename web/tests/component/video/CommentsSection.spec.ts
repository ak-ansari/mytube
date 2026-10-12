import { describe, expect, it, vi } from "vitest";

vi.mock("@/api/catalog.service", () => ({ default: { comments: vi.fn(), addComment: vi.fn(), deleteComment: vi.fn() } }));
vi.mock("@/api/user.service", () => ({ default: { me: vi.fn() } }));

import { push } from "notivue";
import catalogService from "@/api/catalog.service";
import userService from "@/api/user.service";
import { isAuthenticated, login } from "@/auth/keycloak";
import CommentsSection from "@/components/video/CommentsSection.vue";
import { mountWithPlugins, settle } from "../../helpers/mount";
import { channelSummary, comment, page, profile } from "../../helpers/fixtures";
import { intersectAll } from "../../helpers/intersection";

const svc = vi.mocked(catalogService);
const owner = channelSummary({ id: "owner", user_name: "owner" });
const bob = channelSummary({ id: "bob", user_name: "bob" });
const carol = channelSummary({ id: "carol", user_name: "carol" });

const comments = [
  comment({ id: "c1", content: "First!", author: bob }),
  comment({ id: "c2", content: "Thanks for watching", author: owner }),
  comment({ id: "c3", content: "Line one\nline two", author: carol }),
];

const mountSection = async (opts: { me?: string; count?: number } = {}) => {
  if (opts.me) {
    isAuthenticated.value = true;
    vi.mocked(userService.me).mockResolvedValue(profile({ id: opts.me, user_name: opts.me }));
  }
  svc.comments.mockResolvedValue(page(comments));
  const r = await mountWithPlugins(CommentsSection, { props: { videoId: "v1", ownerId: "owner", count: opts.count ?? 3 } });
  await settle();
  return r;
};

const actionButtons = (w: Awaited<ReturnType<typeof mountSection>>["wrapper"]) => w.findAll("[aria-label='Comment actions']");

describe("CommentsSection", () => {
  it("renders the count and the comment list with author links and creator badge", async () => {
    const { wrapper } = await mountSection();
    expect(svc.comments).toHaveBeenCalledWith("v1", 1);
    expect(wrapper.find("h2").text()).toBe("3 Comments");
    const rows = wrapper.findAll(".comment");
    expect(rows).toHaveLength(3);
    expect(rows[0].text()).toContain("@bob");
    expect(rows[0].text()).toContain("First!");
    expect(rows[0].text()).toContain("an hour ago");
    expect(rows[0].find("a").attributes("href")).toBe("/channel/bob");
    expect(rows[1].find(".owner-badge").text()).toBe("creator");
    expect(rows[0].find(".owner-badge").exists()).toBe(false);
  });

  it("singular heading", async () => {
    const { wrapper } = await mountSection({ count: 1 });
    expect(wrapper.find("h2").text()).toBe("1 Comment");
  });

  it("anonymous: no delete actions, focusing the box asks to log in", async () => {
    const { wrapper } = await mountSection();
    expect(actionButtons(wrapper)).toHaveLength(0);
    await wrapper.find("textarea").trigger("focus");
    expect(login).toHaveBeenCalledTimes(1);
    expect(wrapper.text()).not.toContain("Cancel");
  });

  it("signed-in viewer can delete only their own comment", async () => {
    const { wrapper } = await mountSection({ me: "bob" });
    const rows = wrapper.findAll(".comment");
    expect(rows[0].find("[aria-label='Comment actions']").exists()).toBe(true);
    expect(rows[1].find("[aria-label='Comment actions']").exists()).toBe(false);
    expect(rows[2].find("[aria-label='Comment actions']").exists()).toBe(false);
  });

  it("video owner can delete every comment", async () => {
    const { wrapper } = await mountSection({ me: "owner" });
    expect(actionButtons(wrapper)).toHaveLength(3);
  });

  it("delete calls the API and toasts", async () => {
    svc.deleteComment.mockResolvedValue(null);
    const { wrapper } = await mountSection({ me: "bob" });
    await actionButtons(wrapper)[0].trigger("click");
    await settle();
    const del = wrapper.findAll(".v-list-item").find((i) => i.text() === "Delete")!;
    await del.trigger("click");
    await settle();
    expect(svc.deleteComment).toHaveBeenCalledWith("v1", "c1");
    expect(push.success).toHaveBeenCalledWith("Comment deleted");
    // list is refetched after the mutation
    expect(svc.comments.mock.calls.length).toBeGreaterThanOrEqual(2);
  });

  it("signed-in: compose, submit trimmed text, then reset the box", async () => {
    svc.addComment.mockResolvedValue(comment());
    const { wrapper } = await mountSection({ me: "carol" });
    const box = wrapper.find("textarea");
    await box.trigger("focus");
    expect(login).not.toHaveBeenCalled();
    const commentBtn = () => wrapper.findAll("button").find((b) => b.text() === "Comment")!;
    expect(commentBtn().attributes("disabled")).toBeDefined();
    await box.setValue("   Great video   ");
    expect(commentBtn().attributes("disabled")).toBeUndefined();
    await commentBtn().trigger("click");
    await settle();
    expect(svc.addComment).toHaveBeenCalledWith("v1", "Great video");
    expect((wrapper.find("textarea").element as HTMLTextAreaElement).value).toBe("");
    expect(wrapper.findAll("button").some((b) => b.text() === "Comment")).toBe(false);
  });

  it("cancel clears the draft", async () => {
    const { wrapper } = await mountSection({ me: "carol" });
    await wrapper.find("textarea").trigger("focus");
    await wrapper.find("textarea").setValue("draft");
    await wrapper.findAll("button").find((b) => b.text() === "Cancel")!.trigger("click");
    expect((wrapper.find("textarea").element as HTMLTextAreaElement).value).toBe("");
    expect(svc.addComment).not.toHaveBeenCalled();
  });

  it("keeps the draft if posting fails", async () => {
    svc.addComment.mockRejectedValue({ message: "too long" });
    const { wrapper } = await mountSection({ me: "carol" });
    await wrapper.find("textarea").trigger("focus");
    await wrapper.find("textarea").setValue("hello");
    await wrapper.findAll("button").find((b) => b.text() === "Comment")!.trigger("click");
    await settle();
    expect((wrapper.find("textarea").element as HTMLTextAreaElement).value).toBe("hello");
  });

  it("loads the next page when scrolled to the end", async () => {
    svc.comments
      .mockResolvedValueOnce(page([comments[0]], { has_more: true }))
      .mockResolvedValueOnce(page([comments[1]], { page: 2, has_more: false }));
    await mountWithPlugins(CommentsSection, { props: { videoId: "v1", ownerId: "owner", count: 2 } }).then(async ({ wrapper }) => {
      await settle();
      intersectAll(true);
      await settle();
      expect(svc.comments).toHaveBeenLastCalledWith("v1", 2);
      expect(wrapper.findAll(".comment")).toHaveLength(2);
    });
  });
});
