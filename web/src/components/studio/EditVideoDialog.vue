<template>
  <v-dialog v-model="open" max-width="860" scrollable>
    <v-card v-if="video" color="surface-light">
      <v-card-title class="d-flex align-center pt-5 px-6">
        <span class="text-h6 font-weight-bold">{{ needsDetails ? "Finish your upload" : "Video details" }}</span>
        <v-spacer />
        <v-btn icon variant="text" aria-label="Close" @click="open = false"><v-icon>mdi-close</v-icon></v-btn>
      </v-card-title>
      <v-card-text class="px-6">
        <v-form ref="formRef" @submit.prevent="save">
          <v-row>
            <v-col cols="12" md="7">
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
                rows="6"
                counter="5000"
                :rules="[(v: string) => v.length <= 5000 || 'Use at most 5000 characters']"
                placeholder="Tell viewers about your video"
                class="mb-3"
              />
              <ThumbnailPicker
                v-model:thumbnail-key="thumbnailKey"
                v-model:uploading="uploadingThumb"
                :prefix="`thumbnails/${video.id}`"
                :current-url="video.thumbnail_url"
              />
            </v-col>
            <v-col cols="12" md="5">
              <VisibilityPicker v-model="visibility" />
            </v-col>
          </v-row>
        </v-form>
      </v-card-text>
      <v-card-actions class="px-6 pb-5">
        <v-spacer />
        <v-btn variant="text" @click="open = false">Cancel</v-btn>
        <v-btn color="accent" variant="flat" :loading="saving" :disabled="uploadingThumb" @click="save">
          {{ needsDetails ? "Publish" : "Save" }}
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { push } from "notivue";
import { useQueryClient } from "@tanstack/vue-query";
import type { VForm } from "vuetify/components";
import ThumbnailPicker from "./ThumbnailPicker.vue";
import VisibilityPicker from "./VisibilityPicker.vue";
import videoService from "@/api/video.service";
import { errorMessage } from "@/api/http";
import { catalogKeys } from "@/composables/catalog/catalog.keys";
import { useUpdateVideo } from "@/composables/catalog/catalog.mutation";
import { VideoVisibility, type VideoCard } from "@/types";
import { videoStatusInfo } from "@/utils";

const open = defineModel<boolean>({ default: false });
const props = defineProps<{ video: VideoCard | null }>();

const formRef = ref<InstanceType<typeof VForm>>();
const title = ref("");
const description = ref("");
const visibility = ref<VideoVisibility>(VideoVisibility.Public);
const thumbnailKey = ref("");
const uploadingThumb = ref(false);
const confirming = ref(false);

const qc = useQueryClient();
const { mutate: update, isPending: updating } = useUpdateVideo();
const saving = computed(() => updating.value || confirming.value);
const needsDetails = computed(() => !!props.video && videoStatusInfo(props.video).needsDetails);

watch(open, (isOpen) => {
  if (!isOpen || !props.video) return;
  title.value = props.video.title ?? "";
  description.value = props.video.description ?? "";
  visibility.value = props.video.visibility ?? VideoVisibility.Public;
  thumbnailKey.value = "";
});

const done = (message: string) => {
  push.success(message);
  open.value = false;
};

const save = async () => {
  const { valid } = (await formRef.value?.validate()) ?? { valid: false };
  if (!valid || !props.video) return;
  const id = props.video.id;

  // drafts go through confirm, which also starts phase 2 of the pipeline
  if (needsDetails.value) {
    confirming.value = true;
    try {
      await videoService.confirmVideo({
        id,
        video: { title: title.value.trim(), description: description.value, visibility: visibility.value, thumbnail: thumbnailKey.value },
      });
      await qc.invalidateQueries({ queryKey: catalogKeys.all });
      done("Publishing — your video is processing");
    } catch (err) {
      push.error(errorMessage(err));
    } finally {
      confirming.value = false;
    }
    return;
  }

  update(
    {
      id,
      input: {
        title: title.value.trim(),
        description: description.value,
        visibility: visibility.value,
        ...(thumbnailKey.value ? { thumbnail: thumbnailKey.value } : {}),
      },
    },
    { onSuccess: () => done("Changes saved") }
  );
};
</script>
