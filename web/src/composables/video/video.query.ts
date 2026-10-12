import { useQuery } from "@tanstack/vue-query";
import { videoKeys } from "./video.keys";
import videoService from "@/api/video.service";
import { computed, type Ref, unref } from "vue";

export const useSuggestSearch = (search: Ref<string>) => {
  return useQuery({
    queryKey: videoKeys.suggest(search),
    queryFn: () => videoService.getSuggestions(unref(search)),
    enabled: computed(() => unref(search) !== ""), // only fetch if search is non-empty
  });
};
