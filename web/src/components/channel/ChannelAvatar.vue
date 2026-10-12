<template>
  <v-avatar :size="size" :style="avatarUrl ? undefined : { background: color }" class="channel-avatar">
    <v-img v-if="avatarUrl" :src="avatarUrl" cover :alt="name" />
    <span v-else class="initial" :style="{ fontSize: `${Math.round(size * 0.45)}px` }">{{ initial }}</span>
  </v-avatar>
</template>

<script setup lang="ts">
import { computed } from "vue";

const props = withDefaults(
  defineProps<{
    channel?: { id: string; channel_name?: string | null; user_name?: string; avatar_url?: string } | null;
    size?: number;
  }>(),
  { size: 36 }
);

const palette = ["#e53935", "#8e24aa", "#3949ab", "#00897b", "#f4511e", "#6d4c41", "#039be5", "#7cb342", "#5e35b1", "#d81b60"];

const avatarUrl = computed(() => props.channel?.avatar_url || "");
const name = computed(() => props.channel?.channel_name || props.channel?.user_name || "?");
const initial = computed(() => name.value.trim().charAt(0).toUpperCase() || "?");
const color = computed(() => {
  const id = props.channel?.id ?? "";
  let hash = 0;
  for (let i = 0; i < id.length; i++) hash = (hash * 31 + id.charCodeAt(i)) >>> 0;
  return palette[hash % palette.length];
});
</script>

<style scoped>
.channel-avatar {
  flex-shrink: 0;
}
.initial {
  color: #fff;
  font-weight: 500;
  line-height: 1;
}
</style>
