<template>
  <div class="page studio">
    <div class="d-flex flex-wrap align-center justify-space-between ga-3 mb-6">
      <div>
        <h1 class="page-title">Channel content</h1>
        <div class="text-body-2 text-muted">Manage, edit and track every video you’ve uploaded.</div>
      </div>
      <v-btn color="secondary" variant="flat" prepend-icon="mdi-upload" :to="{ name: 'Upload' }">Upload video</v-btn>
    </div>

    <div v-if="isLoading" class="d-flex justify-center py-16"><v-progress-circular indeterminate color="accent" /></div>

    <EmptyState v-else-if="!items.length" icon="mdi-movie-open-plus-outline" title="No content available" text="Upload a video and it will appear here.">
      <v-btn color="secondary" variant="flat" :to="{ name: 'Upload' }">Upload video</v-btn>
    </EmptyState>

    <template v-else>
      <v-table class="studio-table" hover>
        <thead>
          <tr>
            <th>Video</th>
            <th class="d-none d-md-table-cell">Visibility</th>
            <th>Status</th>
            <th class="d-none d-md-table-cell">Date</th>
            <th class="d-none d-sm-table-cell text-right">Views</th>
            <th class="text-right" />
          </tr>
        </thead>
        <tbody>
          <tr v-for="v in items" :key="v.id">
            <td class="py-3">
              <div class="d-flex ga-3 align-center video-cell">
                <VideoThumbnail class="studio-thumb" :url="v.thumbnail_url" :duration-seconds="v.duration_seconds" />
                <div class="min-w-0">
                  <div class="text-body-2 font-weight-medium line-clamp-2">{{ v.title || "Untitled draft" }}</div>
                  <div class="text-caption text-muted line-clamp-1">{{ v.description || "Add description" }}</div>
                </div>
              </div>
            </td>
            <td class="d-none d-md-table-cell text-no-wrap">
              <template v-if="v.visibility">
                <v-icon size="18" class="mr-1">{{ visibilityIcon[v.visibility] }}</v-icon>
                <span class="text-capitalize">{{ v.visibility }}</span>
              </template>
              <span v-else class="text-muted">—</span>
            </td>
            <td>
              <v-chip :color="statusOf(v).color" size="small" variant="tonal" :prepend-icon="statusOf(v).icon">
                {{ statusOf(v).label }}
              </v-chip>
            </td>
            <td class="d-none d-md-table-cell text-no-wrap text-body-2">
              {{ formatDate(v.created_at) }}
              <div class="text-caption text-muted">Uploaded</div>
            </td>
            <td class="d-none d-sm-table-cell text-right">{{ formatCount(v.views) }}</td>
            <td class="text-right text-no-wrap">
              <v-btn
                v-if="statusOf(v).needsDetails"
                size="small"
                color="accent"
                variant="tonal"
                class="mr-1"
                @click="edit(v)"
              >
                Finish
              </v-btn>
              <v-btn v-else-if="v.title" icon variant="text" size="small" aria-label="Edit" @click="edit(v)">
                <v-icon>mdi-pencil-outline</v-icon>
                <v-tooltip activator="parent" location="top">Details</v-tooltip>
              </v-btn>
              <v-btn
                icon
                variant="text"
                size="small"
                aria-label="Watch"
                :disabled="statusOf(v).label !== 'Published'"
                :to="{ name: 'Watch', params: { videoId: v.id } }"
              >
                <v-icon>mdi-play-circle-outline</v-icon>
                <v-tooltip activator="parent" location="top">Watch</v-tooltip>
              </v-btn>
              <v-btn icon variant="text" size="small" aria-label="Delete" @click="askDelete(v)">
                <v-icon>mdi-delete-outline</v-icon>
                <v-tooltip activator="parent" location="top">Delete forever</v-tooltip>
              </v-btn>
            </td>
          </tr>
        </tbody>
      </v-table>
      <LoadMore :has-more="!!hasNextPage" :loading="isFetchingNextPage" @load="fetchNextPage()" />
    </template>

    <EditVideoDialog v-model="editOpen" :video="selected" />
    <ConfirmDialog
      v-model="deleteOpen"
      danger
      title="Permanently delete this video?"
      confirm-text="Delete forever"
      :loading="deleting"
      @confirm="confirmDelete"
    >
      “{{ selected?.title || "Untitled draft" }}” and all of its comments, likes and files will be removed. This can’t be undone.
    </ConfirmDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { push } from "notivue";
import VideoThumbnail from "@/components/video/VideoThumbnail.vue";
import EmptyState from "@/components/common/EmptyState.vue";
import LoadMore from "@/components/common/LoadMore.vue";
import ConfirmDialog from "@/components/common/ConfirmDialog.vue";
import EditVideoDialog from "@/components/studio/EditVideoDialog.vue";
import { useMyVideos } from "@/composables/catalog/catalog.query";
import { useDeleteVideo } from "@/composables/catalog/catalog.mutation";
import type { VideoCard } from "@/types";
import { formatCount, formatDate, videoStatusInfo, visibilityIcon } from "@/utils";

const { items, isLoading, hasNextPage, isFetchingNextPage, fetchNextPage, refetch } = useMyVideos();
const { mutate: deleteVideo, isPending: deleting } = useDeleteVideo();

// ticks so drafts that stop moving flip to "Upload incomplete" (and stop the polling) without new data
const now = ref(Date.now());
const clock = setInterval(() => (now.value = Date.now()), 30_000);
const statusOf = (v: VideoCard) => videoStatusInfo(v, now.value);
const selected = ref<VideoCard | null>(null);
const editOpen = ref(false);
const deleteOpen = ref(false);

const edit = (v: VideoCard) => {
  selected.value = v;
  editOpen.value = true;
};
const askDelete = (v: VideoCard) => {
  selected.value = v;
  deleteOpen.value = true;
};
const confirmDelete = () => {
  if (!selected.value) return;
  deleteVideo(selected.value.id, {
    onSuccess: () => {
      deleteOpen.value = false;
      push.success("Video deleted");
    },
  });
};

// keep statuses fresh while something is genuinely moving through the pipeline (stale drafts don't count)
const anyProcessing = computed(() => items.value.some((v) => statusOf(v).processing));
let timer: ReturnType<typeof setInterval> | undefined;
watch(
  anyProcessing,
  (processing) => {
    clearInterval(timer);
    if (processing) timer = setInterval(() => refetch(), 4000);
  },
  { immediate: true }
);
onBeforeUnmount(() => {
  clearInterval(timer);
  clearInterval(clock);
});
</script>

<style scoped>
.studio {
  max-width: 1400px;
}
.studio-table {
  background: transparent;
}
.studio-table th {
  font-weight: 500 !important;
  white-space: nowrap;
}
.studio-thumb {
  width: 120px;
  flex-shrink: 0;
}
.studio-thumb :deep(.thumb.rounded),
.studio-thumb.rounded {
  border-radius: 6px;
}
.video-cell {
  max-width: 460px;
}
.min-w-0 {
  min-width: 0;
}
</style>
