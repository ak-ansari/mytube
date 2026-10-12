<template>
  <div ref="sentinel" class="d-flex justify-center py-6">
    <v-progress-circular v-if="loading" indeterminate size="28" width="3" color="accent" />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";

/**
 * Emits `load` whenever the sentinel scrolls into view and there is more to load.
 * If it is still visible when a page finishes loading (short pages, tall screens), it asks again.
 */
const props = defineProps<{ hasMore: boolean; loading: boolean }>();
const emit = defineEmits<{ load: [] }>();

const sentinel = ref<HTMLElement>();
let observer: IntersectionObserver | null = null;
let visible = false;

const maybeLoad = () => {
  if (visible && props.hasMore && !props.loading) emit("load");
};

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      visible = !!entries[0]?.isIntersecting;
      maybeLoad();
    },
    { rootMargin: "400px" }
  );
  if (sentinel.value) observer.observe(sentinel.value);
});
watch(
  () => [props.loading, props.hasMore] as const,
  ([loading, hasMore], [wasLoading, hadMore]) => {
    if ((wasLoading && !loading) || (!hadMore && hasMore)) maybeLoad();
  }
);
onBeforeUnmount(() => observer?.disconnect());
</script>
