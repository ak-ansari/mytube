<template>
  <div class="page">
    <VideoFeed
      :items="items"
      :loading="isLoading"
      :has-more="!!hasNextPage"
      :loading-more="isFetchingNextPage"
      @load-more="fetchNextPage()"
    >
      <template #empty>
        <EmptyState
          v-if="isError"
          icon="mdi-cloud-alert-outline"
          title="Couldn't load videos"
          text="Make sure the API is running, then try again."
        >
          <v-btn variant="tonal" @click="refetch()">Retry</v-btn>
        </EmptyState>
        <EmptyState v-else icon="mdi-video-plus-outline" title="No videos yet" text="Be the first to upload something.">
          <v-btn color="secondary" variant="flat" :to="{ name: 'Upload' }">Upload a video</v-btn>
        </EmptyState>
      </template>
    </VideoFeed>
  </div>
</template>

<script lang="ts" setup>
import VideoFeed from "@/components/video/VideoFeed.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import { useBrowseVideos } from "@/composables/catalog/catalog.query";

const { items, isLoading, isError, refetch, hasNextPage, isFetchingNextPage, fetchNextPage } = useBrowseVideos();
</script>
