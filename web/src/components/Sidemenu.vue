<template>
  <!-- mini guide (rail) -->
  <div v-if="rail" class="rail d-flex flex-column align-center pt-1">
    <RouterLink v-for="item in railItems" :key="item.title" :to="item.to" class="rail-item" :class="{ active: isActive(item.to) }">
      <v-icon size="24">{{ isActive(item.to) ? item.activeIcon : item.icon }}</v-icon>
      <span>{{ item.title }}</span>
    </RouterLink>
  </div>

  <div v-else class="guide">
    <v-list nav density="comfortable" bg-color="transparent" class="px-3">
      <GuideItem v-for="item in mainItems" :key="item.title" v-bind="item" :active="isActive(item.to)" />
    </v-list>
    <v-divider class="mx-3 my-2" />

    <template v-if="isAuthenticated">
      <v-list nav density="comfortable" bg-color="transparent" class="px-3">
        <v-list-subheader class="section-title">You <v-icon size="16">mdi-chevron-right</v-icon></v-list-subheader>
        <GuideItem v-for="item in youItems" :key="item.title" v-bind="item" :active="isActive(item.to)" />
      </v-list>
      <v-divider class="mx-3 my-2" />

      <v-list nav density="comfortable" bg-color="transparent" class="px-3">
        <v-list-subheader class="section-title">Subscriptions</v-list-subheader>
        <v-list-item
          v-for="c in subscriptions ?? []"
          :key="c.id"
          :to="{ name: 'Channel', params: { channelId: c.id } }"
          rounded="lg"
          class="guide-item"
        >
          <template #prepend>
            <ChannelAvatar :channel="c" :size="24" />
          </template>
          <v-list-item-title class="line-clamp-1">{{ channelName(c) }}</v-list-item-title>
        </v-list-item>
        <div v-if="!subscriptions?.length" class="text-caption text-muted px-3 py-2">Channels you subscribe to show up here.</div>
      </v-list>
    </template>

    <div v-else class="px-6 py-3">
      <div class="text-body-2 mb-3">Sign in to like videos, comment, and subscribe.</div>
      <v-btn variant="outlined" color="accent" prepend-icon="mdi-account-circle-outline" class="sign-in" @click="login()">Sign in</v-btn>
    </div>

    <v-divider class="mx-3 my-2" />
    <div class="footer text-muted px-6 py-4">
      myTube — a learning project.<br />
      Built with Go, Vue &amp; Keycloak.
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, h } from "vue";
import { useRoute, type RouteLocationRaw } from "vue-router";
import { VIcon, VListItem, VListItemTitle } from "vuetify/components";
import ChannelAvatar from "./channel/ChannelAvatar.vue";
import { isAuthenticated, login } from "@/auth/keycloak";
import { useMe, useMySubscriptions } from "@/composables/user/user.query";
import { channelName } from "@/utils";

defineProps<{ rail?: boolean }>();

const route = useRoute();
const { data: me } = useMe();
const { data: subscriptions } = useMySubscriptions();

type Item = { title: string; icon: string; activeIcon: string; to: RouteLocationRaw };

const mainItems: Item[] = [
  { title: "Home", icon: "mdi-home-outline", activeIcon: "mdi-home", to: { name: "Home" } },
  { title: "Subscriptions", icon: "mdi-youtube-subscription", activeIcon: "mdi-youtube-subscription", to: { name: "Subscriptions" } },
];

const youItems = computed<Item[]>(() => [
  ...(me.value
    ? [{ title: "Your channel", icon: "mdi-account-box-outline", activeIcon: "mdi-account-box", to: { name: "Channel", params: { channelId: me.value.id } } }]
    : []),
  { title: "Your videos", icon: "mdi-play-box-multiple-outline", activeIcon: "mdi-play-box-multiple", to: { name: "Studio" } },
  { title: "Liked videos", icon: "mdi-thumb-up-outline", activeIcon: "mdi-thumb-up", to: { name: "Liked" } },
]);

const railItems = computed<Item[]>(() => [
  ...mainItems,
  { title: "You", icon: "mdi-account-circle-outline", activeIcon: "mdi-account-circle", to: { name: "Studio" } },
]);

const isActive = (to: RouteLocationRaw) => {
  if (typeof to === "string" || !("name" in to)) return false;
  if (to.name !== route.name) return false;
  const params = "params" in to ? to.params : undefined;
  return !params || Object.entries(params).every(([k, v]) => route.params[k] === v);
};

const GuideItem = (props: Item & { active: boolean }) =>
  h(
    VListItem,
    { to: props.to, rounded: "lg", active: props.active, class: "guide-item", color: undefined },
    {
      prepend: () => h(VIcon, { size: 24 }, () => (props.active ? props.activeIcon : props.icon)),
      default: () => h(VListItemTitle, () => props.title),
    }
  );
</script>

<style scoped>
.guide :deep(.guide-item) {
  min-height: 40px !important;
  padding-inline: 12px !important;
}
.guide :deep(.guide-item .v-list-item__spacer) {
  width: 24px !important;
}
.guide :deep(.guide-item .v-list-item-title) {
  font-size: 14px;
  font-weight: 400;
}
.guide :deep(.guide-item.v-list-item--active .v-list-item-title) {
  font-weight: 500;
}
.guide :deep(.guide-item .v-list-item__prepend > .v-icon) {
  opacity: 1;
}
.guide :deep(.guide-item .v-list-item__overlay) {
  background: rgb(var(--v-theme-on-surface));
}
.guide :deep(.guide-item.v-list-item--active > .v-list-item__overlay) {
  opacity: 0.1;
}
.section-title {
  font-size: 16px;
  font-weight: 500;
  color: rgb(var(--v-theme-on-surface));
  opacity: 1;
  padding-inline-start: 12px !important;
}
.sign-in {
  border-color: rgba(var(--v-border-color), 0.3);
}
.footer {
  font-size: 12px;
  line-height: 18px;
}
.rail-item {
  width: 64px;
  padding: 16px 0 14px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  border-radius: 10px;
  font-size: 10px;
}
.rail-item:hover,
.rail-item.active {
  background: rgb(var(--v-theme-surface-bright));
}
</style>
