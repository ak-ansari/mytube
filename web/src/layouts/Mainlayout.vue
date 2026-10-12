<template>
  <v-app>
    <AppBar @toggle-drawer="toggleDrawer" />

    <v-navigation-drawer
      v-model="drawerOpen"
      :rail="rail"
      :rail-width="72"
      :width="240"
      :temporary="overlay"
      :permanent="!overlay"
      floating
      color="background"
    >
      <SideBar :rail="rail" />
    </v-navigation-drawer>

    <v-main class="bg-background">
      <router-view :key="routeKey" />
    </v-main>
  </v-app>
</template>

<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { useRoute } from "vue-router";
import { useDisplay } from "vuetify";
import AppBar from "@/components/Appbar.vue";
import SideBar from "@/components/Sidemenu.vue";

const route = useRoute();
const { lgAndUp } = useDisplay();

// desktop: permanent guide that collapses to a mini rail · smaller screens and the watch page: overlay guide
const immersive = computed(() => !!route.meta.immersive);
const collapsed = ref(false);
const overlayOpen = ref(false);

const overlay = computed(() => immersive.value || !lgAndUp.value);
const drawerOpen = computed({
  get: () => (overlay.value ? overlayOpen.value : true),
  set: (v: boolean) => {
    if (overlay.value) overlayOpen.value = v;
  },
});
const rail = computed(() => !overlay.value && collapsed.value);

const toggleDrawer = () => {
  if (overlay.value) overlayOpen.value = !overlayOpen.value;
  else collapsed.value = !collapsed.value;
};

watch(
  () => route.fullPath,
  () => (overlayOpen.value = false)
);

// re-mount pages when only params change (watch → watch, channel → channel)
const routeKey = computed(() => `${String(route.name)}:${JSON.stringify(route.params)}`);
</script>
