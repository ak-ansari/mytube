<template>
  <div class="page">
    <template v-if="channels?.length">
      <div class="d-flex align-center justify-space-between mb-3">
        <h2 class="text-h6 font-weight-bold">Channels</h2>
      </div>
      <div class="channels d-flex ga-6 mb-8">
        <RouterLink
          v-for="c in channels"
          :key="c.id"
          :to="{ name: 'Channel', params: { channelId: c.id } }"
          class="channel d-flex flex-column align-center text-center"
        >
          <ChannelAvatar :channel="c" :size="88" class="mb-2" />
          <div class="text-body-2 font-weight-medium line-clamp-1">{{ channelName(c) }}</div>
          <div class="text-caption text-muted">{{ formatSubscribers(c.subscribers) }}</div>
        </RouterLink>
      </div>
      <h2 class="text-h6 font-weight-bold mb-4">Latest</h2>
    </template>

    <VideoFeed
      :items="items"
      :loading="isLoading"
      :has-more="!!hasNextPage"
      :loading-more="isFetchingNextPage"
      @load-more="fetchNextPage()"
    >
      <template #empty>
        <EmptyState
          icon="mdi-youtube-subscription"
          :title="channels?.length ? 'No new videos' : 'Don’t miss new videos'"
          :text="channels?.length ? 'Channels you subscribe to haven’t posted yet.' : 'Subscribe to channels and their latest videos will appear here.'"
        >
          <v-btn variant="tonal" :to="{ name: 'Home' }">Explore videos</v-btn>
        </EmptyState>
      </template>
    </VideoFeed>
  </div>
</template>

<script lang="ts" setup>
import VideoFeed from "@/components/video/VideoFeed.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import ChannelAvatar from "@/components/channel/ChannelAvatar.vue";
import { useSubscriptionFeed } from "@/composables/catalog/catalog.query";
import { useMySubscriptions } from "@/composables/user/user.query";
import { channelName, formatSubscribers } from "@/utils";

const { data: channels } = useMySubscriptions();
const { items, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage } = useSubscriptionFeed();
</script>

<style scoped>
.channels {
  overflow-x: auto;
  padding-bottom: 8px;
}
.channel {
  width: 112px;
  flex-shrink: 0;
}
</style>
