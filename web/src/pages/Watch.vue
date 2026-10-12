<template>
  <div class="watch">
    <EmptyState
      v-if="isError"
      icon="mdi-video-off-outline"
      title="Video unavailable"
      text="This video is private, still processing, or has been removed."
    >
      <v-btn color="secondary" variant="flat" :to="{ name: 'Home' }">Go to home</v-btn>
    </EmptyState>

    <div v-else class="watch-grid">
      <div class="primary-col">
        <!-- player -->
        <VideoPlayer
          v-if="video?.manifest_url"
          :src="video.manifest_url"
          :poster="video.thumbnail_url"
          @play="countView"
        />
        <div v-else class="player-placeholder d-flex flex-column align-center justify-center">
          <template v-if="video">
            <v-icon size="48" class="mb-2">{{ status?.icon }}</v-icon>
            <div class="text-body-1">This video is {{ status?.label.toLowerCase() }}.</div>
            <div class="text-body-2 text-muted">Only you can see it until it’s published.</div>
          </template>
          <v-progress-circular v-else indeterminate color="accent" />
        </div>

        <template v-if="video">
          <h1 class="video-title mt-3 mb-2">{{ video.title }}</h1>

          <!-- channel + actions -->
          <div class="d-flex flex-wrap align-center justify-space-between ga-3">
            <div class="d-flex align-center ga-3 min-w-0">
              <RouterLink :to="{ name: 'Channel', params: { channelId: video.channel.id } }">
                <ChannelAvatar :channel="video.channel" :size="40" />
              </RouterLink>
              <div class="min-w-0 mr-3">
                <RouterLink :to="{ name: 'Channel', params: { channelId: video.channel.id } }" class="text-body-1 font-weight-medium line-clamp-1">
                  {{ channelName(video.channel) }}
                </RouterLink>
                <div class="text-caption text-muted">{{ formatSubscribers(video.subscribers) }}</div>
              </div>
              <v-btn v-if="isOwner" variant="tonal" :to="{ name: 'Studio' }" prepend-icon="mdi-pencil-outline">Edit video</v-btn>
              <SubscribeButton v-else :channel-id="video.channel.id" :subscribed="video.is_subscribed" />
            </div>

            <div class="d-flex align-center ga-2">
              <div class="reactions d-flex align-center">
                <v-btn
                  variant="tonal"
                  rounded="s-pill"
                  class="pr-3"
                  :prepend-icon="video.my_reaction === 1 ? 'mdi-thumb-up' : 'mdi-thumb-up-outline'"
                  :loading="reacting"
                  @click="react(1)"
                >
                  {{ formatCount(video.likes) }}
                </v-btn>
                <v-btn variant="tonal" rounded="e-pill" class="dislike" :loading="reacting" aria-label="Dislike" @click="react(-1)">
                  <v-icon>{{ video.my_reaction === -1 ? "mdi-thumb-down" : "mdi-thumb-down-outline" }}</v-icon>
                </v-btn>
              </div>
              <v-btn variant="tonal" prepend-icon="mdi-share-outline" @click="share">Share</v-btn>
            </div>
          </div>

          <!-- description -->
          <div class="description mt-3 pa-3" :class="{ expanded }" @click="expanded = true">
            <div class="text-body-2 font-weight-medium mb-1">
              {{ formatViews(video.views) }} &nbsp;{{ expanded ? formatDate(video.created_at) : timeAgo(video.created_at) }}
              <v-chip v-if="video.visibility !== 'public'" size="x-small" class="ml-2" :prepend-icon="visibilityIcon[video.visibility ?? 'private']">
                {{ video.visibility }}
              </v-chip>
            </div>
            <div class="text-body-2 desc-text" :class="{ 'line-clamp-2': !expanded }">{{ video.description || "No description." }}</div>
            <button v-if="!expanded" class="text-body-2 font-weight-medium mt-1">...more</button>
            <button v-else class="text-body-2 font-weight-medium mt-3" @click.stop="expanded = false">Show less</button>
          </div>

          <CommentsSection
            v-if="lgAndUp"
            class="mt-6"
            :video-id="video.id"
            :owner-id="video.channel.id"
            :count="video.comment_count"
          />
        </template>
      </div>

      <!-- related -->
      <aside class="secondary-col">
        <div class="d-flex flex-column ga-2">
          <VideoCard v-for="v in related" :key="v.id" :video="v" :variant="lgAndUp ? 'compact' : 'grid'" />
        </div>
        <LoadMore :has-more="!!relatedHasMore" :loading="relatedLoadingMore" @load="loadMoreRelated()" />
      </aside>

      <CommentsSection
        v-if="video && !lgAndUp"
        class="mt-2 comments-mobile"
        :video-id="video.id"
        :owner-id="video.channel.id"
        :count="video.comment_count"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from "vue";
import { useRoute } from "vue-router";
import { useDisplay } from "vuetify";
import { push } from "notivue";
import VideoPlayer from "@/components/VideoPlayer.vue";
import VideoCard from "@/components/video/VideoCard.vue";
import CommentsSection from "@/components/video/CommentsSection.vue";
import ChannelAvatar from "@/components/channel/ChannelAvatar.vue";
import SubscribeButton from "@/components/channel/SubscribeButton.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import LoadMore from "@/components/common/LoadMore.vue";
import catalogService from "@/api/catalog.service";
import { isAuthenticated, login } from "@/auth/keycloak";
import { useRelatedVideos, useVideoDetails } from "@/composables/catalog/catalog.query";
import { useReactToVideo } from "@/composables/catalog/catalog.mutation";
import { useMe } from "@/composables/user/user.query";
import {
  channelName,
  formatCount,
  formatDate,
  formatSubscribers,
  formatViews,
  timeAgo,
  videoStatusInfo,
  visibilityIcon,
} from "@/utils";

const route = useRoute();
const { lgAndUp } = useDisplay();
const videoId = computed(() => route.params.videoId as string);

const { data: video, isError } = useVideoDetails(videoId);
const { data: me } = useMe();
const {
  items: related,
  hasNextPage: relatedHasMore,
  isFetchingNextPage: relatedLoadingMore,
  fetchNextPage: loadMoreRelated,
} = useRelatedVideos(videoId);
const { mutate: reactTo, isPending: reacting } = useReactToVideo();

const isOwner = computed(() => !!me.value && video.value?.channel.id === me.value.id);
const status = computed(() => (video.value ? videoStatusInfo(video.value) : null));
const expanded = ref(false);

let viewCounted = false;
const countView = () => {
  if (viewCounted) return;
  viewCounted = true;
  catalogService.recordView(videoId.value).catch(() => undefined);
};

const react = (value: 1 | -1) => {
  if (!isAuthenticated.value) return login();
  if (!video.value) return;
  reactTo({ id: video.value.id, value: video.value.my_reaction === value ? 0 : value });
};

const share = async () => {
  try {
    await navigator.clipboard.writeText(window.location.href);
    push.success("Link copied to clipboard");
  } catch {
    push.info(window.location.href);
  }
};
</script>

<style scoped>
.watch {
  padding: 24px;
}
.watch-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 402px;
  gap: 24px;
  max-width: 1754px;
  margin: 0 auto;
}
.primary-col {
  min-width: 0;
}
.player-placeholder {
  aspect-ratio: 16 / 9;
  border-radius: 12px;
  background: rgb(var(--v-theme-surface-bright));
}
.video-title {
  font-size: 20px;
  font-weight: 700;
  line-height: 28px;
  word-break: break-word;
}
.reactions .dislike {
  min-width: 48px;
  border-left: 1px solid rgba(var(--v-border-color), 0.2);
}
.description {
  border-radius: 12px;
  background: rgb(var(--v-theme-surface-bright));
  cursor: pointer;
}
.description.expanded {
  cursor: default;
}
.desc-text {
  white-space: pre-wrap;
  word-break: break-word;
}
.min-w-0 {
  min-width: 0;
}
@media (max-width: 1279px) {
  .watch {
    padding: 0 0 24px;
  }
  .watch-grid {
    grid-template-columns: 1fr;
    gap: 16px;
  }
  .primary-col,
  .secondary-col,
  .comments-mobile {
    padding: 0 16px;
  }
  .primary-col :deep(.player-shell),
  .player-placeholder {
    border-radius: 0;
    margin: 0 -16px;
  }
}
</style>
