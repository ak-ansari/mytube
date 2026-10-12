<template>
  <div>
    <div v-if="loading" :class="variant === 'grid' ? 'video-grid' : 'd-flex flex-column ga-4'">
      <div v-for="n in variant === 'grid' ? 12 : 5" :key="n" :class="`skeleton-${variant}`">
        <div class="skeleton-thumb" />
        <div class="skeleton-lines">
          <div class="skeleton-line" style="width: 90%" />
          <div class="skeleton-line" style="width: 60%" />
        </div>
      </div>
    </div>

    <template v-else-if="items.length">
      <div :class="variant === 'grid' ? 'video-grid' : 'd-flex flex-column list-gap'">
        <VideoCard v-for="video in items" :key="video.id" :video="video" :variant="variant" :show-channel="showChannel" />
      </div>
      <LoadMore :has-more="hasMore" :loading="loadingMore" @load="$emit('loadMore')" />
    </template>

    <slot v-else name="empty" />
  </div>
</template>

<script setup lang="ts">
import type { VideoCard as VideoCardType } from "@/types";
import VideoCard from "./VideoCard.vue";
import LoadMore from "@/components/common/LoadMore.vue";

withDefaults(
  defineProps<{
    items: VideoCardType[];
    loading: boolean;
    hasMore: boolean;
    loadingMore: boolean;
    variant?: "grid" | "list";
    showChannel?: boolean;
  }>(),
  { variant: "grid", showChannel: true }
);
defineEmits<{ loadMore: [] }>();
</script>

<style scoped>
.list-gap {
  gap: 16px;
}
.skeleton-thumb {
  aspect-ratio: 16 / 9;
  border-radius: 12px;
  background: rgb(var(--v-theme-surface-bright));
  animation: pulse 1.4s ease-in-out infinite;
}
.skeleton-lines {
  padding-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  flex: 1;
}
.skeleton-line {
  height: 14px;
  border-radius: 4px;
  background: rgb(var(--v-theme-surface-bright));
  animation: pulse 1.4s ease-in-out infinite;
}
.skeleton-list {
  display: flex;
  gap: 16px;
}
.skeleton-list .skeleton-thumb {
  width: min(360px, 45%);
}
@keyframes pulse {
  50% {
    opacity: 0.5;
  }
}
</style>
