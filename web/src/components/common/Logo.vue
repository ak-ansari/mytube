<template>
  <RouterLink to="/" class="logo d-flex align-center" aria-label="myTube home">
    <img :src="logo" width="29" height="20" alt="" />
    <span class="wordmark">my<span class="font-weight-bold">Tube</span></span>
  </RouterLink>
</template>

<script setup lang="ts">
const logo = new URL("@/assets/icons/logo.svg", import.meta.url).href;
</script>

<style scoped>
.logo {
  gap: 4px;
  padding: 18px 14px 18px 16px;
}
.wordmark {
  font-size: 20px;
  letter-spacing: -0.8px;
  line-height: 1;
}
</style>
