<template>
  <!--
    The whole card is one link (to Watch). The channel avatar/name inside are plain, non-focusable elements with a
    click handler that routes to the channel instead, so there is no <a>/<button> nested inside the <a>; keyboard and
    screen-reader users get a single link per card and reach the channel from the watch page.
  -->
  <RouterLink :to="{ name: 'Watch', params: { videoId: video.id } }" class="video-card" :class="`variant-${variant}`">
    <VideoThumbnail class="card-thumb" :url="video.thumbnail_url" :duration-seconds="video.duration_seconds" />

    <div class="card-body">
      <ChannelAvatar
        v-if="variant === 'grid' && showChannel"
        :channel="video.channel"
        :size="36"
        class="mr-3 mt-1 channel-link"
        @click.prevent.stop="openChannel"
      />
      <div class="card-text">
        <h3 class="title" :class="variant === 'list' ? 'line-clamp-2' : 'line-clamp-2'">{{ video.title }}</h3>

        <template v-if="variant === 'list'">
          <div class="meta text-muted">{{ formatViews(video.views) }} • {{ timeAgo(video.created_at) }}</div>
          <div v-if="showChannel" class="d-flex align-center my-3 channel-link meta text-muted" @click.prevent.stop="openChannel">
            <ChannelAvatar :channel="video.channel" :size="24" class="mr-2" />
            {{ channelName(video.channel) }}
          </div>
          <div v-if="video.description" class="meta text-muted line-clamp-2">{{ video.description }}</div>
        </template>

        <template v-else>
          <div v-if="showChannel" class="meta text-muted channel-link channel-name" @click.prevent.stop="openChannel">
            {{ channelName(video.channel) }}
          </div>
          <div class="meta text-muted">{{ formatViews(video.views) }} • {{ timeAgo(video.created_at) }}</div>
        </template>
      </div>
    </div>
  </RouterLink>
</template>

<script setup lang="ts">
import { useRouter } from "vue-router";
import type { VideoCard } from "@/types";
import { channelName, formatViews, timeAgo } from "@/utils";
import ChannelAvatar from "@/components/channel/ChannelAvatar.vue";
import VideoThumbnail from "./VideoThumbnail.vue";

const props = withDefaults(
  defineProps<{
    video: VideoCard;
    /** grid: home/channel tiles · list: search results · compact: watch-page sidebar */
    variant?: "grid" | "list" | "compact";
    showChannel?: boolean;
  }>(),
  { variant: "grid", showChannel: true }
);

const router = useRouter();
const openChannel = () => router.push({ name: "Channel", params: { channelId: props.video.channel.id } });
</script>

<style scoped>
.video-card {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.video-card:hover :deep(.thumb-img) {
  transform: scale(1.03);
}
.card-body {
  display: flex;
  padding-top: 12px;
  min-width: 0;
}
.card-text {
  min-width: 0;
  flex: 1;
}
.title {
  font-size: 16px;
  font-weight: 500;
  line-height: 22px;
  margin-bottom: 4px;
}
.meta {
  font-size: 14px;
  line-height: 20px;
}
.channel-link {
  cursor: pointer;
}
.channel-name:hover {
  color: rgb(var(--v-theme-on-surface)) !important;
}

/* search results */
.variant-list {
  flex-direction: row;
  gap: 16px;
}
.variant-list .card-thumb {
  width: min(360px, 45%);
  flex-shrink: 0;
}
.variant-list .card-body {
  padding-top: 0;
}
.variant-list .title {
  font-size: 18px;
  line-height: 26px;
  font-weight: 400;
}
.variant-list .meta {
  font-size: 12px;
}

/* watch-page sidebar */
.variant-compact {
  flex-direction: row;
  gap: 8px;
}
.variant-compact .card-thumb {
  width: 168px;
  flex-shrink: 0;
}
.variant-compact :deep(.thumb.rounded) {
  border-radius: 8px;
}
.variant-compact .card-body {
  padding-top: 0;
}
.variant-compact .title {
  font-size: 14px;
  line-height: 20px;
}
.variant-compact .meta {
  font-size: 12px;
  line-height: 18px;
}

@media (max-width: 600px) {
  .variant-list {
    flex-direction: column;
    gap: 0;
  }
  .variant-list .card-thumb {
    width: 100%;
  }
  .variant-list .card-body {
    padding-top: 12px;
  }
}
</style>
