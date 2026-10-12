<template>
  <form class="searchbar d-flex align-center" role="search" @submit.prevent="submit()">
    <div class="field-wrap">
      <v-text-field
        v-model="query"
        placeholder="Search"
        hide-details
        density="compact"
        variant="outlined"
        rounded="s-pill"
        autocomplete="off"
        color="accent"
        class="search-field"
        :clearable="!!query"
        @focus="focused = true"
        @blur="onBlur"
        @keydown.down.prevent="move(1)"
        @keydown.up.prevent="move(-1)"
        @keydown.esc="focused = false"
      >
        <template #prepend-inner>
          <v-icon v-if="focused" size="20" class="mr-1">mdi-magnify</v-icon>
        </template>
      </v-text-field>

      <v-sheet v-if="showSuggestions" class="suggestions py-3" elevation="8" rounded="lg" color="surface-light">
        <div
          v-for="(s, i) in visibleSuggestions"
          :key="s"
          class="suggestion d-flex align-center px-4"
          :class="{ active: i === highlighted }"
          @mousedown.prevent="submit(s)"
        >
          <v-icon size="18" class="mr-4">mdi-magnify</v-icon>
          <span class="line-clamp-1">{{ s }}</span>
        </div>
      </v-sheet>
    </div>
    <v-btn type="submit" class="search-btn" variant="flat" color="surface-bright" rounded="e-pill" height="40" width="64" aria-label="Search">
      <v-icon>mdi-magnify</v-icon>
    </v-btn>
  </form>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useDebouncedRef } from "@/composables/useDebouce";
import { useSuggestSearch } from "@/composables/video/video.query";

const route = useRoute();
const router = useRouter();

const { state: query, debounced } = useDebouncedRef<string>((route.query.q as string) || "", 200);
const focused = ref(false);
const highlighted = ref(-1);

// keep the box in sync when navigating between result pages
watch(
  () => route.query.q,
  (q) => (query.value = (q as string) || "")
);

const suggestTerm = computed(() => (debounced.value || "").trim());
const { data: suggestions } = useSuggestSearch(suggestTerm);
const visibleSuggestions = computed(() => (suggestTerm.value ? (suggestions.value ?? []).slice(0, 8) : []));
const showSuggestions = computed(() => focused.value && visibleSuggestions.value.length > 0);
watch(visibleSuggestions, () => (highlighted.value = -1));

const move = (delta: number) => {
  const n = visibleSuggestions.value.length;
  if (!n) return;
  highlighted.value = (highlighted.value + delta + n) % n;
};

const onBlur = () => (focused.value = false);

const submit = (term?: string) => {
  const q = (term ?? (highlighted.value >= 0 ? visibleSuggestions.value[highlighted.value] : query.value) ?? "").trim();
  focused.value = false;
  if (!q) return;
  query.value = q;
  router.push({ name: "Results", query: { q } });
};
</script>

<style scoped>
.searchbar {
  width: 100%;
  max-width: 640px;
}
.field-wrap {
  position: relative;
  flex: 1;
}
.search-field :deep(.v-field) {
  border-top-right-radius: 0;
  border-bottom-right-radius: 0;
  padding-left: 12px;
}
.search-field :deep(.v-field__outline__end) {
  border-top-right-radius: 0 !important;
  border-bottom-right-radius: 0 !important;
}
.search-btn {
  margin-left: -1px;
  border: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}
.suggestions {
  position: absolute;
  top: 44px;
  left: 0;
  right: 0;
  z-index: 20;
}
.suggestion {
  height: 32px;
  cursor: pointer;
  font-size: 16px;
  font-weight: 500;
}
.suggestion:hover,
.suggestion.active {
  background: rgb(var(--v-theme-surface-bright));
}
</style>
