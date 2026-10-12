<template>
  <v-app-bar flat height="56" color="background" class="app-bar">
    <template #prepend>
      <v-btn icon variant="text" aria-label="Menu" @click="$emit('toggle-drawer')">
        <v-icon>mdi-menu</v-icon>
      </v-btn>
      <Logo />
    </template>

    <div v-if="!mobileSearch" class="d-none d-sm-flex justify-center flex-1-1 px-4">
      <Searchbar />
    </div>
    <div v-else class="d-flex align-center flex-1-1 px-2">
      <v-btn icon variant="text" aria-label="Close search" @click="mobileSearch = false"><v-icon>mdi-arrow-left</v-icon></v-btn>
      <Searchbar />
    </div>

    <template #append>
      <div class="d-flex align-center ga-1 pr-2">
        <v-btn v-if="!mobileSearch" icon variant="text" class="d-sm-none" aria-label="Search" @click="mobileSearch = true">
          <v-icon>mdi-magnify</v-icon>
        </v-btn>

        <v-btn icon variant="text" :aria-label="isDark ? 'Light theme' : 'Dark theme'" @click="toggleTheme">
          <v-icon>{{ isDark ? "mdi-weather-sunny" : "mdi-weather-night" }}</v-icon>
          <v-tooltip activator="parent" location="bottom">{{ isDark ? "Light theme" : "Dark theme" }}</v-tooltip>
        </v-btn>

        <template v-if="isAuthenticated">
          <v-btn :to="{ name: 'Upload' }" variant="tonal" color="secondary" class="d-none d-md-flex" prepend-icon="mdi-plus">Create</v-btn>
          <v-btn :to="{ name: 'Upload' }" icon variant="text" class="d-md-none" aria-label="Create">
            <v-icon>mdi-video-plus-outline</v-icon>
          </v-btn>

          <v-menu location="bottom end" offset="8" :close-on-content-click="true">
            <template #activator="{ props }">
              <v-btn v-bind="props" icon variant="text" aria-label="Account menu" class="ml-1">
                <ChannelAvatar :channel="me" :size="32" />
              </v-btn>
            </template>
            <v-card min-width="280" color="surface-light" rounded="lg" class="py-2">
              <div v-if="me" class="d-flex pa-4 ga-4">
                <ChannelAvatar :channel="me" :size="40" />
                <div class="min-w-0">
                  <div class="text-body-1 font-weight-medium line-clamp-1">{{ channelName(me) }}</div>
                  <div class="text-body-2 line-clamp-1">@{{ me.user_name }}</div>
                  <RouterLink :to="{ name: 'Channel', params: { channelId: me.id } }" class="text-body-2 text-accent">View your channel</RouterLink>
                </div>
              </div>
              <v-divider />
              <v-list density="comfortable" bg-color="transparent" class="py-1">
                <v-list-item prepend-icon="mdi-play-box-multiple-outline" title="Your videos" :to="{ name: 'Studio' }" />
                <v-list-item prepend-icon="mdi-thumb-up-outline" title="Liked videos" :to="{ name: 'Liked' }" />
                <v-list-item
                  v-if="me"
                  prepend-icon="mdi-account-edit-outline"
                  title="Customize channel"
                  :to="{ name: 'Channel', params: { channelId: me.id }, query: { edit: '1' } }"
                />
                <v-list-item prepend-icon="mdi-shield-account-outline" title="Manage account" :href="accountUrl()" target="_blank" />
              </v-list>
              <v-divider />
              <v-list density="comfortable" bg-color="transparent" class="py-1">
                <v-list-item prepend-icon="mdi-logout" title="Sign out" @click="logout" />
              </v-list>
            </v-card>
          </v-menu>
        </template>

        <v-btn v-else variant="outlined" color="accent" prepend-icon="mdi-account-circle-outline" class="sign-in" @click="login()">
          Sign in
        </v-btn>
      </div>
    </template>
  </v-app-bar>
</template>

<script setup lang="ts">
import { ref } from "vue";
import Logo from "./common/Logo.vue";
import Searchbar from "./searchbar/Searchbar.vue";
import ChannelAvatar from "./channel/ChannelAvatar.vue";
import { accountUrl, isAuthenticated, login, logout } from "@/auth/keycloak";
import { useMe } from "@/composables/user/user.query";
import { useThemeToggle } from "@/composables/useThemeToggle";
import { channelName } from "@/utils";

defineEmits<{ "toggle-drawer": [] }>();

const { data: me } = useMe();
const { isDark, toggle: toggleTheme } = useThemeToggle();
const mobileSearch = ref(false);
</script>

<style scoped>
.sign-in {
  border-color: rgba(var(--v-border-color), 0.3);
}
.min-w-0 {
  min-width: 0;
}
</style>
