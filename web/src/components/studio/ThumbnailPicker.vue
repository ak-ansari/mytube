<template>
  <div>
    <div class="text-body-1 font-weight-medium mb-1">Thumbnail</div>
    <div class="text-caption text-muted mb-3">{{ hint }}</div>
    <div class="d-flex align-center ga-4">
      <div class="preview">
        <v-img v-if="preview || currentUrl" :src="preview || currentUrl" :aspect-ratio="16 / 9" cover />
        <div v-else class="empty d-flex align-center justify-center text-muted text-caption">{{ emptyText }}</div>
        <div v-if="uploading" class="overlay d-flex align-center justify-center">
          <v-progress-circular :model-value="progress" color="white" size="36" width="3" />
        </div>
      </div>
      <div class="d-flex flex-column ga-2">
        <v-btn variant="tonal" prepend-icon="mdi-image-plus" :disabled="uploading" @click="input?.click()">
          {{ preview ? "Change" : "Upload thumbnail" }}
        </v-btn>
        <v-btn v-if="preview" variant="text" size="small" :disabled="uploading" @click="reset">Use default</v-btn>
      </div>
      <input ref="input" type="file" :accept="IMAGE_ACCEPT" hidden @change="onSelect" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, ref } from "vue";
import { push } from "notivue";
import { IMAGE_ACCEPT, isAcceptedImage, uploadObject } from "@/composables/file/uploadObject";
import { errorMessage } from "@/api/http";

const props = withDefaults(
  defineProps<{ prefix: string; currentUrl?: string; hint?: string; emptyText?: string }>(),
  { hint: "Select or upload a picture that shows what's in your video.", emptyText: "Generating…" }
);
/** object key of the uploaded custom thumbnail ("" = use the current/auto-generated one) */
const key = defineModel<string>("thumbnailKey", { default: "" });
const uploading = defineModel<boolean>("uploading", { default: false });

const input = ref<HTMLInputElement>();
const preview = ref("");
const progress = ref(0);

const onSelect = async (e: Event) => {
  const file = (e.target as HTMLInputElement).files?.[0];
  (e.target as HTMLInputElement).value = "";
  if (!file) return;
  if (!isAcceptedImage(file)) {
    push.error("Thumbnails must be JPEG, PNG, WebP or GIF images");
    return;
  }
  if (file.size > 5 * 1024 * 1024) {
    push.error("Thumbnails must be under 5 MB");
    return;
  }
  setPreview(URL.createObjectURL(file));
  uploading.value = true;
  progress.value = 0;
  try {
    key.value = await uploadObject(props.prefix, file, (p) => (progress.value = p));
  } catch (err) {
    reset();
    push.error(errorMessage(err, "Thumbnail upload failed"));
  } finally {
    uploading.value = false;
  }
};

/** swaps the local preview, freeing the previous object URL */
const setPreview = (url: string) => {
  if (preview.value) URL.revokeObjectURL(preview.value);
  preview.value = url;
};

const reset = () => {
  setPreview("");
  key.value = "";
};

onBeforeUnmount(() => setPreview(""));
</script>

<style scoped>
.preview {
  position: relative;
  width: 176px;
  flex-shrink: 0;
  border-radius: 8px;
  overflow: hidden;
  background: rgb(var(--v-theme-surface-bright));
}
.empty {
  aspect-ratio: 16 / 9;
}
.overlay {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.5);
}
</style>
