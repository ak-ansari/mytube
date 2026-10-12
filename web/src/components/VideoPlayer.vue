<template>
  <div class="player-shell">
    <video ref="videoRef" class="video-js vjs-big-play-centered" playsinline></video>
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from "vue";
import videojs from "video.js";
import "video.js/dist/video-js.css";

const props = defineProps<{ src: string; poster?: string }>();
const emit = defineEmits<{ play: [] }>();

const videoRef = ref<HTMLVideoElement>();
let player: ReturnType<typeof videojs> | null = null;

// the backend produces an HLS master playlist (segments/<id>/master.m3u8)
const source = (src: string) => ({ src, type: src.includes(".m3u8") ? "application/x-mpegURL" : "video/mp4" });

onMounted(() => {
  player = videojs(videoRef.value!, {
    controls: true,
    autoplay: true,
    preload: "auto",
    fluid: true,
    aspectRatio: "16:9",
    playbackRates: [0.5, 1, 1.25, 1.5, 2],
    poster: props.poster,
    sources: [source(props.src)],
  });
  player.one("play", () => emit("play"));
});

watch(
  () => props.src,
  (src) => {
    if (!player) return;
    player.poster(props.poster ?? "");
    player.src(source(src));
    player.one("play", () => emit("play"));
  }
);

onBeforeUnmount(() => {
  player?.dispose();
  player = null;
});
</script>

<style scoped>
.player-shell {
  border-radius: 12px;
  overflow: hidden;
  background: #000;
}
.player-shell :deep(.video-js) {
  font-family: inherit;
}
.player-shell :deep(.vjs-big-play-button) {
  border-radius: 50%;
  width: 72px;
  height: 72px;
  line-height: 72px;
  margin-left: -36px;
  margin-top: -36px;
  border: none;
  background: rgba(0, 0, 0, 0.6);
}
.player-shell :deep(.vjs-play-progress),
.player-shell :deep(.vjs-volume-level) {
  background: #ff0033;
}
</style>
