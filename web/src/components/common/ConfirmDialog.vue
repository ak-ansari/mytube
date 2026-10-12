<template>
  <v-dialog v-model="open" max-width="440">
    <v-card color="surface-light" class="pa-2">
      <v-card-title class="text-h6 font-weight-bold pt-4 text-wrap">{{ title }}</v-card-title>
      <v-card-text class="text-body-2">
        <slot>{{ text }}</slot>
      </v-card-text>
      <v-card-actions class="px-6 pb-4">
        <v-spacer />
        <v-btn variant="text" @click="open = false">Cancel</v-btn>
        <v-btn :color="danger ? 'error' : 'accent'" variant="flat" :loading="loading" @click="$emit('confirm')">{{ confirmText }}</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup lang="ts">
const open = defineModel<boolean>({ default: false });
withDefaults(defineProps<{ title: string; text?: string; confirmText?: string; danger?: boolean; loading?: boolean }>(), {
  confirmText: "Confirm",
});
defineEmits<{ confirm: [] }>();
</script>
