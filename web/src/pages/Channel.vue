<template>
  <div class="page channel-page">
    <EmptyState v-if="isError" icon="mdi-account-off-outline" title="Channel not found" text="This channel doesn't exist.">
      <v-btn color="secondary" variant="flat" :to="{ name: 'Home' }">Go to home</v-btn>
    </EmptyState>

    <template v-else-if="channel">
      <div class="banner mb-6" :style="{ background: bannerGradient }" />

      <header class="d-flex flex-column flex-sm-row align-sm-center ga-6 mb-4">
        <ChannelAvatar :channel="channel" :size="smAndUp ? 160 : 88" />
        <div class="min-w-0">
          <h1 class="channel-title">{{ channelName(channel) }}</h1>
          <div class="text-body-2 text-muted mt-2">
            <span class="font-weight-medium text-high-emphasis">@{{ channel.user_name }}</span>
            · {{ formatSubscribers(channel.subscribers) }} · {{ channel.video_count }} {{ channel.video_count === 1 ? "video" : "videos" }}
          </div>
          <div v-if="channel.description" class="text-body-2 text-muted mt-2 line-clamp-1 about-link" @click="tab = 'about'">
            {{ channel.description }} <v-icon size="16">mdi-chevron-right</v-icon>
          </div>
          <div class="d-flex flex-wrap ga-2 mt-4">
            <template v-if="isOwn">
              <v-btn variant="tonal" @click="editOpen = true">Customize channel</v-btn>
              <v-btn variant="tonal" :to="{ name: 'Studio' }">Manage videos</v-btn>
            </template>
            <SubscribeButton v-else :channel-id="channel.id" :subscribed="channel.is_subscribed" />
          </div>
        </div>
      </header>

      <v-tabs v-model="tab" color="secondary" class="mb-6 tabs">
        <v-tab value="videos" class="text-none">Videos</v-tab>
        <v-tab value="about" class="text-none">About</v-tab>
      </v-tabs>

      <VideoFeed
        v-if="tab === 'videos'"
        :items="items"
        :loading="videosLoading"
        :has-more="!!hasNextPage"
        :loading-more="isFetchingNextPage"
        :show-channel="false"
        @load-more="fetchNextPage()"
      >
        <template #empty>
          <EmptyState
            icon="mdi-video-outline"
            :title="isOwn ? 'Create content on any device' : 'This channel has no videos yet'"
            :text="isOwn ? 'Upload and publish a video to get started.' : undefined"
          >
            <v-btn v-if="isOwn" color="secondary" variant="flat" :to="{ name: 'Upload' }">Upload video</v-btn>
          </EmptyState>
        </template>
      </VideoFeed>

      <div v-else class="about">
        <h2 class="text-h6 font-weight-bold mb-3">Description</h2>
        <p class="text-body-2 desc mb-6">{{ channel.description || "This channel has no description." }}</p>
        <h2 class="text-h6 font-weight-bold mb-3">Channel details</h2>
        <div class="d-flex flex-column ga-3 text-body-2">
          <div><v-icon class="mr-3">mdi-at</v-icon>{{ channel.user_name }}</div>
          <div><v-icon class="mr-3">mdi-account-multiple-outline</v-icon>{{ formatSubscribers(channel.subscribers) }}</div>
          <div><v-icon class="mr-3">mdi-play-box-multiple-outline</v-icon>{{ channel.video_count }} videos</div>
          <div><v-icon class="mr-3">mdi-information-outline</v-icon>Joined {{ formatDate(channel.created_at) }}</div>
        </div>
      </div>

      <ChannelEditDialog v-if="isOwn && me" v-model="editOpen" :profile="me" />
    </template>

    <div v-else class="d-flex justify-center py-16"><v-progress-circular indeterminate color="accent" /></div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute, useRouter } from "vue-router";
import { useDisplay } from "vuetify";
import ChannelAvatar from "@/components/channel/ChannelAvatar.vue";
import ChannelEditDialog from "@/components/channel/ChannelEditDialog.vue";
import SubscribeButton from "@/components/channel/SubscribeButton.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import VideoFeed from "@/components/video/VideoFeed.vue";
import { useChannel, useChannelVideos, useMe } from "@/composables/user/user.query";
import { channelName, formatDate, formatSubscribers } from "@/utils";

const route = useRoute();
const router = useRouter();
const { smAndUp } = useDisplay();
const channelId = computed(() => route.params.channelId as string);

const { data: channel, isError } = useChannel(channelId);
const { data: me } = useMe();
const { items, isLoading: videosLoading, hasNextPage, isFetchingNextPage, fetchNextPage } = useChannelVideos(channelId);

const isOwn = computed(() => !!me.value && me.value.id === channelId.value);
const tab = ref<"videos" | "about">("videos");

// /channel/:id?edit=1 (from the account menu) opens the customize dialog
const editOpen = ref(route.query.edit === "1");
watch(editOpen, (open) => {
  if (!open && route.query.edit) router.replace({ query: {} });
});

const bannerGradient = computed(() => {
  const seed = [...channelId.value].reduce((a, ch) => (a * 31 + ch.charCodeAt(0)) >>> 0, 7);
  const hue = seed % 360;
  return `linear-gradient(120deg, hsl(${hue} 70% 35%), hsl(${(hue + 60) % 360} 70% 25%) 60%, hsl(${(hue + 120) % 360} 60% 20%))`;
});
</script>

<style scoped>
.channel-page {
  max-width: 1284px;
}
.banner {
  height: clamp(100px, 16vw, 176px);
  border-radius: 16px;
}
.channel-title {
  font-size: 36px;
  font-weight: 700;
  line-height: 1.2;
  word-break: break-word;
}
.about-link {
  cursor: pointer;
}
.tabs {
  border-bottom: 1px solid rgba(var(--v-border-color), var(--v-border-opacity));
}
.desc {
  white-space: pre-wrap;
  max-width: 760px;
}
.min-w-0 {
  min-width: 0;
}
@media (max-width: 600px) {
  .channel-title {
    font-size: 24px;
  }
}
</style>
