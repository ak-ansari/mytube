<template>
  <Notivue v-slot="item">
    <Notification :item="item" :theme="darkTheme" :ripple="true">
      <NotificationProgress :item="item" />
    </Notification>
  </Notivue>
</template>
<script lang="ts" setup>
  
import {
  Notivue,
  Notification,
  darkTheme,
  NotificationProgress,
} from "notivue";
</script>
