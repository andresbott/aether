<script setup lang="ts">
import MiniPlayer from '@/components/layout/MiniPlayer.vue'
import MobileTabBar from '@/components/layout/MobileTabBar.vue'
import PlayerSheet from '@/components/layout/PlayerSheet.vue'
import { usePlayer } from '@/composables/usePlayer'

// Mobile-only docked chrome; PlayerLayout owns the shared skeleton (route
// outlet included) so a shell swap never unmounts the active view. Rendered
// as a fragment so the pieces stay direct flex children of the shell column.
const { queue } = usePlayer()
</script>

<template>
    <!-- Docked chrome: mini-player only while there is something to play. -->
    <MiniPlayer v-if="queue.length > 0" />
    <MobileTabBar />
    <PlayerSheet />
</template>
