<template>
  <div class="empty d-flex flex-column align-center text-center">
    <v-icon :icon="icon" size="96" class="text-muted mb-4" />
    <div class="text-h6 font-weight-medium mb-1">{{ title }}</div>
    <div v-if="text" class="text-body-2 text-muted mb-5">{{ text }}</div>
    <slot />
  </div>
</template>

<script setup lang="ts">
withDefaults(defineProps<{ icon?: string; title: string; text?: string }>(), { icon: "mdi-video-off-outline" });
</script>

<style scoped>
.empty {
  padding: 72px 16px;
  max-width: 460px;
  margin: 0 auto;
}
</style>
