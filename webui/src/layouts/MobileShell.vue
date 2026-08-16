<script setup lang="ts">
import NowPlayingSheet from '@/components/layout/NowPlayingSheet.vue'
import { usePlayer } from '@/composables/usePlayer'

// Mobile-only docked chrome; PlayerLayout owns the shared skeleton (route
// outlet included) so a shell swap never unmounts the active view. Rendered
// as a fragment so the spacer stays a direct flex child of the shell column.
//
// The sheet is all of it: Now Playing, the queue and the mini strip are one
// always-mounted overlay (NowPlayingSheet, addressed by the route hash), and
// it overlays rather than docks — so the spacer below reserves the strip's
// height in the flex column, keeping list bottoms clear of the bar.
const { queue } = usePlayer()
</script>

<template>
    <div v-if="queue.length > 0" class="mini-spacer" aria-hidden="true"></div>
    <NowPlayingSheet v-if="queue.length > 0" />
</template>

<style scoped>
.mini-spacer {
    height: calc(var(--app-mini-player-height) + env(safe-area-inset-bottom));
    flex-shrink: 0;
}
</style>
