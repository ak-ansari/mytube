<template>
  <div class="thumb" :class="{ rounded: !square }">
    <!-- decorative: the surrounding card link already carries the title -->
    <v-img :src="src || placeholder" alt="" :aspect-ratio="16 / 9" cover class="thumb-img" @error="failed = true">
      <template #placeholder>
        <div class="thumb-placeholder" />
      </template>
    </v-img>
    <span v-if="duration" class="duration">{{ duration }}</span>
    <slot />
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { formatDuration } from "@/utils";

const props = defineProps<{ url?: string; durationSeconds?: number | null; square?: boolean }>();

const placeholder = new URL("@/assets/images/no-image-placeholder.jpg", import.meta.url).href;
const failed = ref(false);
watch(
  () => props.url,
  () => (failed.value = false)
);
const src = computed(() => (failed.value ? "" : props.url));
const duration = computed(() => formatDuration(props.durationSeconds));
</script>

<style scoped>
.thumb {
  position: relative;
  overflow: hidden;
  background: rgb(var(--v-theme-surface-bright));
}
.thumb.rounded {
  border-radius: 12px;
}
.thumb-img {
  transition: transform 0.25s ease;
}
.thumb-placeholder {
  width: 100%;
  height: 100%;
  background: rgb(var(--v-theme-surface-bright));
}
.duration {
  position: absolute;
  right: 6px;
  bottom: 6px;
  padding: 1px 5px;
  border-radius: 4px;
  background: rgba(0, 0, 0, 0.8);
  color: #fff;
  font-size: 12px;
  font-weight: 500;
  line-height: 18px;
}
</style>
