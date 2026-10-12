<template>
  <EmptyState icon="mdi-map-marker-question-outline" title="This page isn't available" text="The link may be broken, or the page may have been removed.">
    <v-btn color="secondary" variant="flat" :to="{ name: 'Home' }">Go to home</v-btn>
  </EmptyState>
</template>

<script setup lang="ts">
import EmptyState from "@/components/common/EmptyState.vue";
</script>
