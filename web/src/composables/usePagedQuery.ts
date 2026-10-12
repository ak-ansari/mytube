import { useInfiniteQuery } from "@tanstack/vue-query";
import { computed, type MaybeRefOrGetter, toValue } from "vue";
import type { Page } from "@/types";

/** Infinite list over a paginated endpoint; `items` is every loaded page flattened. */
export const usePagedQuery = <T>(
  queryKey: MaybeRefOrGetter<unknown[]>,
  fetchPage: (page: number) => Promise<Page<T>>,
  enabled: MaybeRefOrGetter<boolean> = true
) => {
  const query = useInfiniteQuery({
    queryKey: computed(() => toValue(queryKey)),
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: 1,
    getNextPageParam: (last: Page<T>) => (last.has_more ? last.page + 1 : undefined),
    enabled: computed(() => toValue(enabled)),
  });
  const items = computed<T[]>(() => query.data.value?.pages.flatMap((p) => p.items) ?? []);
  return { ...query, items };
};
