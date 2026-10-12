<template>
  <div class="page upload-page">
    <!-- 1. pick a file -->
    <v-card v-if="step === 'select'" color="surface-light" class="pa-6 pa-sm-10">
      <h1 class="page-title mb-6">Upload video</h1>
      <div
        class="dropzone d-flex flex-column align-center justify-center text-center"
        :class="{ dragging }"
        tabindex="0"
        role="button"
        aria-label="Select a video file to upload"
        @click="fileInput?.click()"
        @keydown.enter.prevent="fileInput?.click()"
        @keydown.space.prevent="fileInput?.click()"
        @dragover.prevent="dragging = true"
        @dragleave.prevent="dragging = false"
        @drop.prevent="onDrop"
      >
        <div class="upload-icon d-flex align-center justify-center mb-6">
          <v-icon size="64" class="text-muted">mdi-tray-arrow-up</v-icon>
        </div>
        <div class="text-body-1 mb-1">Drag and drop a video file to upload</div>
        <div class="text-body-2 text-muted mb-6">Your video stays private until you publish it.</div>
        <v-btn color="secondary" variant="flat" size="large" tabindex="-1">Select file</v-btn>
        <input ref="fileInput" type="file" accept="video/*" hidden @change="onPick" />
      </div>
    </v-card>

    <!-- 2. details while uploading + processing -->
    <v-card v-else-if="step === 'details'" color="surface-light" class="pa-6 pa-sm-8">
      <div class="d-flex align-center mb-6">
        <h1 class="page-title line-clamp-1">{{ title || file?.name }}</h1>
        <v-spacer />
        <v-chip size="small" variant="tonal" :color="stateChip.color" :prepend-icon="stateChip.icon">{{ stateChip.label }}</v-chip>
      </div>

      <v-row>
        <v-col cols="12" md="7">
          <h2 class="text-h6 font-weight-bold mb-4">Details</h2>
          <v-form ref="formRef" @submit.prevent="publish">
            <v-text-field
              v-model="title"
              label="Title (required)"
              counter="100"
              :rules="[(v: string) => (v.trim().length >= 3 && v.trim().length <= 100) || 'Use 3–100 characters']"
              class="mb-3"
            />
            <v-textarea
              v-model="description"
              label="Description"
              rows="5"
              counter="5000"
              :rules="[(v: string) => v.length <= 5000 || 'Use at most 5000 characters']"
              placeholder="Tell viewers about your video"
              class="mb-4"
            />
          </v-form>
          <ThumbnailPicker
            v-if="videoId"
            v-model:thumbnail-key="thumbnailKey"
            v-model:uploading="uploadingThumb"
            :prefix="`thumbnails/${videoId}`"
            :current-url="draft?.thumbnail_url"
            :empty-text="uploadProgress < 100 ? 'After upload' : 'Generating…'"
          />
          <VisibilityPicker v-model="visibility" class="mt-6" />
        </v-col>

        <v-col cols="12" md="5">
          <div class="preview-card">
            <div class="preview-thumb d-flex align-center justify-center">
              <v-img v-if="draft?.thumbnail_url" :src="draft.thumbnail_url" :aspect-ratio="16 / 9" cover />
              <v-icon v-else size="48" class="text-muted">mdi-filmstrip</v-icon>
            </div>
            <div class="pa-4">
              <div class="text-caption text-muted">Filename</div>
              <div class="text-body-2 mb-3 line-clamp-1">{{ file?.name }}</div>
              <div class="text-caption text-muted">Size</div>
              <div class="text-body-2 mb-3">{{ fileSize }}</div>
              <template v-if="draft?.duration_seconds">
                <div class="text-caption text-muted">Duration</div>
                <div class="text-body-2 mb-3">{{ formatDuration(draft.duration_seconds) }}</div>
              </template>
              <v-progress-linear
                :model-value="uploadProgress"
                :indeterminate="uploadProgress === 100 && !ready && !failed"
                :color="failed ? 'error' : ready ? 'success' : 'accent'"
                height="6"
                rounded
                class="mb-2"
              />
              <div class="text-caption" :class="failed ? 'text-error' : 'text-muted'">{{ stateText }}</div>
            </div>
          </div>
        </v-col>
      </v-row>

      <v-divider class="my-6" />
      <div class="d-flex align-center ga-2">
        <v-btn variant="text" :disabled="discarding" :loading="discarding" @click="discard">Discard</v-btn>
        <v-spacer />
        <span v-if="!ready && !failed" class="text-caption text-muted mr-2 d-none d-sm-inline">Publish unlocks once the check finishes</span>
        <v-btn color="accent" variant="flat" size="large" :disabled="!ready || uploadingThumb" :loading="publishing" @click="publish">
          Publish
        </v-btn>
      </div>
    </v-card>

    <!-- 3. done -->
    <v-card v-else color="surface-light" class="pa-10 text-center">
      <v-icon size="72" color="success" class="mb-4">mdi-check-circle-outline</v-icon>
      <h1 class="page-title mb-2">Video published</h1>
      <p class="text-body-2 text-muted mb-8">
        “{{ title }}” is being transcoded now. It will show up on your channel as soon as processing finishes — you can track it in
        Studio.
      </p>
      <div class="d-flex justify-center ga-3">
        <v-btn variant="tonal" @click="reset">Upload another</v-btn>
        <v-btn color="secondary" variant="flat" :to="{ name: 'Studio' }">Go to Studio</v-btn>
      </div>
    </v-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from "vue";
import { onBeforeRouteLeave } from "vue-router";
import { useQuery, useQueryClient } from "@tanstack/vue-query";
import { push } from "notivue";
import type { VForm } from "vuetify/components";
import ThumbnailPicker from "@/components/studio/ThumbnailPicker.vue";
import VisibilityPicker from "@/components/studio/VisibilityPicker.vue";
import videoService from "@/api/video.service";
import fileService from "@/api/file.service";
import catalogService from "@/api/catalog.service";
import { errorMessage } from "@/api/http";
import { catalogKeys } from "@/composables/catalog/catalog.keys";
import { VideoStatus, VideoVisibility } from "@/types";
import { formatDuration, getExtension, stripExtension } from "@/utils";

const MAX_SIZE = 5 * 1024 * 1024 * 1024;
const VIDEO_EXTENSIONS = ["mp4", "mov", "mkv", "webm", "avi", "m4v", "mpeg", "mpg", "ogv", "3gp", "flv", "wmv"];

// some browsers/OSes report an empty MIME type for less common containers, so fall back to the extension
const isVideoFile = (f: File) =>
  f.type.startsWith("video/") || (!f.type && VIDEO_EXTENSIONS.includes((getExtension(f.name) ?? "").toLowerCase()));

const step = ref<"select" | "details" | "done">("select");
const fileInput = ref<HTMLInputElement>();
const formRef = ref<InstanceType<typeof VForm>>();
const dragging = ref(false);

const file = ref<File | null>(null);
const videoId = ref("");
const uploadProgress = ref(0);
const title = ref("");
const description = ref("");
const visibility = ref<VideoVisibility>(VideoVisibility.Public);
const thumbnailKey = ref("");
const uploadingThumb = ref(false);
const publishing = ref(false);
const discarding = ref(false);
/** aborts the in-flight video PUT (and tells `start` the upload was cancelled, not failed) */
let uploadController: AbortController | null = null;

const qc = useQueryClient();

// After the file lands in storage the worker validates it and generates a thumbnail (phase 1).
// Confirming before that finishes would advance the pipeline twice, so publish waits for it.
const { data: draft } = useQuery({
  queryKey: computed(() => catalogKeys.details(videoId.value)),
  queryFn: () => catalogService.details(videoId.value),
  enabled: computed(() => !!videoId.value && uploadProgress.value === 100 && step.value === "details"),
  refetchInterval: (q) => {
    const d = q.state.data;
    return d && (d.status === VideoStatus.Phase1Failed || (d.status === VideoStatus.Valid && d.thumbnail)) ? false : 2000;
  },
});
const ready = computed(() => draft.value?.status === VideoStatus.Valid && !!draft.value.thumbnail);
const failed = computed(() => draft.value?.status === VideoStatus.Phase1Failed);

const stateText = computed(() => {
  if (failed.value) return "We couldn't process this file. Try a different video.";
  if (uploadProgress.value < 100) return `Uploading ${uploadProgress.value}%…`;
  if (!ready.value) return "Upload complete. Checking your video…";
  return "Checks complete. Ready to publish.";
});
const stateChip = computed(() => {
  if (failed.value) return { label: "Failed", color: "error", icon: "mdi-alert-circle" };
  if (uploadProgress.value < 100) return { label: `Uploading ${uploadProgress.value}%`, color: "info", icon: "mdi-cloud-upload" };
  if (!ready.value) return { label: "Checking", color: "info", icon: "mdi-progress-clock" };
  return { label: "Ready", color: "success", icon: "mdi-check-circle" };
});
const fileSize = computed(() => {
  const size = file.value?.size ?? 0;
  return size >= 1024 ** 3 ? `${(size / 1024 ** 3).toFixed(2)} GB` : `${(size / 1024 ** 2).toFixed(1)} MB`;
});

const isUploading = computed(() => step.value === "details" && uploadProgress.value < 100);

/** best effort: drop the draft row + objects so an abandoned upload doesn't linger in Studio */
const removeDraft = (id: string) => {
  if (id) catalogService.remove(id).catch(() => {});
};

const start = async (f: File) => {
  if (!isVideoFile(f)) return push.error("Please choose a video file");
  if (f.size > MAX_SIZE) return push.error("Videos must be under 5 GB");
  file.value = f;
  title.value = stripExtension(f.name).slice(0, 100);
  step.value = "details";
  uploadProgress.value = 0;
  const controller = new AbortController();
  uploadController = controller;
  let id = "";
  try {
    const presigned = await videoService.presignedUrl({ file_name: f.name, size: f.size });
    id = presigned.id;
    // cancelled while presigning: the draft was never stored in videoId, so clean it up here
    if (controller.signal.aborted) return removeDraft(id);
    videoId.value = id;
    await fileService.uploadToS3(presigned.url, f, (p) => (uploadProgress.value = Math.min(p, 99)), controller.signal);
    uploadProgress.value = 100;
  } catch (err) {
    // cancelled by discard / leaving the page: whoever aborted handles the cleanup
    if (controller.signal.aborted) return;
    push.error(errorMessage(err, "Upload failed"));
    removeDraft(id);
    reset();
  } finally {
    if (uploadController === controller) uploadController = null;
  }
};

/** abort the PUT and delete the draft; used when leaving the page mid-upload */
const cancelUpload = () => {
  uploadController?.abort();
  uploadController = null;
  removeDraft(videoId.value);
  videoId.value = "";
};

const onPick = (e: Event) => {
  const f = (e.target as HTMLInputElement).files?.[0];
  if (f) start(f);
};
const onDrop = (e: DragEvent) => {
  dragging.value = false;
  const f = e.dataTransfer?.files?.[0];
  if (f) start(f);
};

const publish = async () => {
  const { valid } = (await formRef.value?.validate()) ?? { valid: false };
  if (!valid || !ready.value) return;
  publishing.value = true;
  try {
    await videoService.confirmVideo({
      id: videoId.value,
      video: { title: title.value.trim(), description: description.value, visibility: visibility.value, thumbnail: thumbnailKey.value },
    });
    await qc.invalidateQueries({ queryKey: catalogKeys.all });
    step.value = "done";
  } catch (err) {
    push.error(errorMessage(err, "Couldn't publish the video"));
  } finally {
    publishing.value = false;
  }
};

const discard = async () => {
  const wasUploading = isUploading.value;
  discarding.value = true;
  uploadController?.abort();
  try {
    if (videoId.value) await catalogService.remove(videoId.value);
    push.info("Upload discarded");
    reset();
  } catch (err) {
    push.error(errorMessage(err));
    // the upload is already aborted, so there is nothing left to come back to
    if (wasUploading) reset();
  } finally {
    discarding.value = false;
  }
};

const reset = () => {
  step.value = "select";
  file.value = null;
  videoId.value = "";
  uploadProgress.value = 0;
  title.value = "";
  description.value = "";
  visibility.value = VideoVisibility.Public;
  thumbnailKey.value = "";
  if (fileInput.value) fileInput.value.value = "";
};

onBeforeRouteLeave(() => {
  if (!isUploading.value) return;
  const leave = window.confirm("Your video is still uploading. Leave and cancel the upload?");
  if (leave) cancelUpload();
  return leave;
});

// closing/reloading the tab mid-upload: let the browser show its "leave site?" prompt
const onBeforeUnload = (e: BeforeUnloadEvent) => {
  e.preventDefault();
  e.returnValue = "";
};
watch(
  isUploading,
  (uploading) => {
    if (uploading) window.addEventListener("beforeunload", onBeforeUnload);
    else window.removeEventListener("beforeunload", onBeforeUnload);
  },
  { immediate: true }
);

onBeforeUnmount(() => {
  window.removeEventListener("beforeunload", onBeforeUnload);
  if (isUploading.value) cancelUpload();
});
</script>

<style scoped>
.upload-page {
  max-width: 1080px;
}
.dropzone {
  min-height: 380px;
  border: 2px dashed rgba(var(--v-border-color), 0.25);
  border-radius: 16px;
  cursor: pointer;
  transition: background 0.2s ease, border-color 0.2s ease;
}
.dropzone:hover,
.dropzone.dragging {
  border-color: rgb(var(--v-theme-accent));
  background: rgba(var(--v-theme-accent), 0.06);
}
.upload-icon {
  width: 136px;
  height: 136px;
  border-radius: 50%;
  background: rgb(var(--v-theme-surface-bright));
}
.preview-card {
  border-radius: 12px;
  overflow: hidden;
  background: rgb(var(--v-theme-surface-bright));
}
.preview-thumb {
  aspect-ratio: 16 / 9;
  background: #000;
}
.preview-thumb > .v-img {
  width: 100%;
}
</style>
