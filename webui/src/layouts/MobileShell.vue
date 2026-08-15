<script setup lang="ts">
import { useRoute } from 'vue-router'
import MiniPlayer from '@/components/layout/MiniPlayer.vue'
import MobileNavDrawer from '@/components/layout/MobileNavDrawer.vue'
import { usePlayer } from '@/composables/usePlayer'

// Mobile-only docked chrome; PlayerLayout owns the shared skeleton (route
// outlet included) so a shell swap never unmounts the active view. Rendered
// as a fragment so the pieces stay direct flex children of the shell column.
const { queue } = usePlayer()
const route = useRoute()
</script>

<template>
    <!-- Docked chrome: mini player only while there is something to play, and
         never on the Now Playing route — the play view there carries the full
         transport, so the bar would only duplicate it and its tap target
         would go nowhere. -->
    <MiniPlayer v-if="queue.length > 0 && route.name !== 'home'" />
    <!-- Overlay: the nav drawer (opened by ContentScaffold's hamburger). -->
    <MobileNavDrawer />
</template>
