<template>
  <v-btn
    v-if="subscribed"
    variant="tonal"
    prepend-icon="mdi-bell-outline"
    :loading="isPending"
    @click="toggle(false)"
  >
    Subscribed
  </v-btn>
  <v-btn v-else color="secondary" variant="flat" :loading="isPending" @click="toggle(true)">Subscribe</v-btn>
</template>

<script setup lang="ts">
import { push } from "notivue";
import { isAuthenticated, login } from "@/auth/keycloak";
import { useToggleSubscription } from "@/composables/user/user.mutation";

const props = defineProps<{ channelId: string; subscribed: boolean }>();
const { mutate, isPending } = useToggleSubscription();

const toggle = (subscribe: boolean) => {
  if (!isAuthenticated.value) return login();
  mutate(
    { channelId: props.channelId, subscribe },
    { onSuccess: () => push.success(subscribe ? "Subscription added" : "Subscription removed") }
  );
};
</script>
