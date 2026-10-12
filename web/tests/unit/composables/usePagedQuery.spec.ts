import { describe, expect, it, vi } from "vitest";
import { ref } from "vue";
import { usePagedQuery } from "@/composables/usePagedQuery";
import { withSetup } from "../../helpers/withSetup";
import { settle } from "../../helpers/mount";
import { page } from "../../helpers/fixtures";

describe("usePagedQuery", () => {
  it("flattens pages and maps has_more to hasNextPage", async () => {
    const fetchPage = vi.fn(async (p: number) =>
      p === 1 ? page(["a", "b"], { page: 1, has_more: true }) : page(["c"], { page: 2, has_more: false })
    );
    const { result } = withSetup(() => usePagedQuery(["things"], fetchPage));
    expect(result.items.value).toEqual([]);
    await settle();
    expect(fetchPage).toHaveBeenCalledWith(1);
    expect(result.items.value).toEqual(["a", "b"]);
    expect(result.hasNextPage.value).toBe(true);

    await result.fetchNextPage();
    await settle();
    expect(fetchPage).toHaveBeenLastCalledWith(2);
    expect(result.items.value).toEqual(["a", "b", "c"]);
    expect(result.hasNextPage.value).toBe(false);
  });

  it("uses page + 1 from the last response as the next page param", async () => {
    const fetchPage = vi.fn(async (p: number) => page([p], { page: p === 1 ? 5 : p, has_more: p === 1 }));
    const { result } = withSetup(() => usePagedQuery(["k"], fetchPage));
    await settle();
    await result.fetchNextPage();
    expect(fetchPage).toHaveBeenLastCalledWith(6);
  });

  it("does not fetch while disabled and starts once enabled", async () => {
    const enabled = ref(false);
    const fetchPage = vi.fn(async () => page([1]));
    const { result } = withSetup(() => usePagedQuery(["k2"], fetchPage, enabled));
    await settle();
    expect(fetchPage).not.toHaveBeenCalled();
    expect(result.items.value).toEqual([]);
    enabled.value = true;
    await settle();
    expect(fetchPage).toHaveBeenCalledTimes(1);
    expect(result.items.value).toEqual([1]);
  });

  it("refetches from page 1 when a reactive key changes", async () => {
    const term = ref("a");
    const fetchPage = vi.fn(async () => page([term.value]));
    const { result } = withSetup(() => usePagedQuery(() => ["search", term.value], fetchPage));
    await settle();
    expect(result.items.value).toEqual(["a"]);
    term.value = "b";
    await settle();
    expect(result.items.value).toEqual(["b"]);
    expect(fetchPage).toHaveBeenLastCalledWith(1);
  });
});
