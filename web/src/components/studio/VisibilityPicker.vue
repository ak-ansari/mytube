<template>
  <div>
    <div class="text-body-1 font-weight-medium mb-1">Visibility</div>
    <div class="text-caption text-muted mb-2">Choose who can see your video.</div>
    <v-radio-group v-model="value" hide-details color="accent">
      <v-radio v-for="o in options" :key="o.value" :value="o.value" class="mb-2">
        <template #label>
          <div>
            <div class="text-body-2 font-weight-medium text-high-emphasis">
              <v-icon size="16" class="mr-1">{{ visibilityIcon[o.value] }}</v-icon>{{ o.title }}
            </div>
            <div class="text-caption text-muted">{{ o.description }}</div>
          </div>
        </template>
      </v-radio>
    </v-radio-group>
  </div>
</template>

<script setup lang="ts">
import { VideoVisibility } from "@/types";
import { visibilityIcon } from "@/utils";

const value = defineModel<VideoVisibility>({ required: true });

const options = [
  { value: VideoVisibility.Public, title: "Public", description: "Everyone can watch and find your video." },
  { value: VideoVisibility.Unlisted, title: "Unlisted", description: "Anyone with the link can watch." },
  { value: VideoVisibility.Private, title: "Private", description: "Only you can watch." },
];
</script>
