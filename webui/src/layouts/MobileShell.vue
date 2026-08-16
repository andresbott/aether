<script setup lang="ts">
import { useRoute } from 'vue-router'
import MiniPlayer from '@/components/layout/MiniPlayer.vue'
import { usePlayer } from '@/composables/usePlayer'

// Mobile-only docked chrome; PlayerLayout owns the shared skeleton (route
// outlet included) so a shell swap never unmounts the active view. Rendered
// as a fragment so the pieces stay direct flex children of the shell column.
//
// The mini player is all of it: navigation is a ROUTE on this shell (/browse,
// reached by the header hamburger), not the overlay drawer that used to live
// here, so the chrome holds nothing that has to outlive a navigation.
const { queue } = usePlayer()
const route = useRoute()
</script>

<template>
    <!-- Docked chrome: mini player only while there is something to play, and
         never on the Now Playing route — the play view there carries the full
         transport, so the bar would only duplicate it and its tap target
         would go nowhere. -->
    <MiniPlayer v-if="queue.length > 0 && route.name !== 'home'" />
</template>
