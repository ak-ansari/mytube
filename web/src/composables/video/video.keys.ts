import type { Ref } from "vue";

export const videoKeys = {
  suggest: (search: Ref<string>) => ["search_suggestions", search],
};
