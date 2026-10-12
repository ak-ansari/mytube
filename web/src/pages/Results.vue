<template>
  <div class="page results">
    <div v-if="q" class="text-body-2 text-muted mb-4">Results for “{{ q }}”</div>
    <VideoFeed
      variant="list"
      :items="items"
      :loading="isLoading"
      :has-more="!!hasNextPage"
      :loading-more="isFetchingNextPage"
      @load-more="fetchNextPage()"
    >
      <template #empty>
        <EmptyState icon="mdi-magnify-close" title="No results found" text="Try different keywords or remove search filters." />
      </template>
    </VideoFeed>
  </div>
</template>

<script lang="ts" setup>
import { computed } from "vue";
import { useRoute } from "vue-router";
import VideoFeed from "@/components/video/VideoFeed.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import { useBrowseVideos } from "@/composables/catalog/catalog.query";

const route = useRoute();
const q = computed(() => ((route.query.q as string) || "").trim());
const { items, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage } = useBrowseVideos(q);
</script>

<style scoped>
.results {
  max-width: 1100px;
}
</style>
