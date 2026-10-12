<template>
  <div class="page liked">
    <div class="d-flex align-center ga-3 mb-6">
      <v-icon size="32">mdi-thumb-up</v-icon>
      <h1 class="page-title">Liked videos</h1>
    </div>
    <VideoFeed
      variant="list"
      :items="items"
      :loading="isLoading"
      :has-more="!!hasNextPage"
      :loading-more="isFetchingNextPage"
      @load-more="fetchNextPage()"
    >
      <template #empty>
        <EmptyState icon="mdi-thumb-up-outline" title="No liked videos yet" text="Videos you like will show up here.">
          <v-btn variant="tonal" :to="{ name: 'Home' }">Browse videos</v-btn>
        </EmptyState>
      </template>
    </VideoFeed>
  </div>
</template>

<script lang="ts" setup>
import VideoFeed from "@/components/video/VideoFeed.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import { useLikedVideos } from "@/composables/catalog/catalog.query";

const { items, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage } = useLikedVideos();
</script>

<style scoped>
.liked {
  max-width: 1100px;
}
</style>
