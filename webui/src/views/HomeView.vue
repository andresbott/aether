<script setup lang="ts">
import SongDetail from '@/components/library/SongDetail.vue'
import { usePlayer } from '@/composables/usePlayer'

const player = usePlayer()
</script>

<template>
    <div class="home-view">
        <SongDetail
            v-if="player.currentTrack.value"
            :song="player.currentTrack.value"
            @play="player.togglePlayPause"
        />
        <div v-else class="empty-state">
            <i class="pi pi-play-circle" style="font-size: 4rem"></i>
            <h2>Nothing is playing</h2>
            <p>Browse your library and start playing music</p>
        </div>
    </div>
</template>

<style scoped>
.home-view {
    min-height: 60vh;
    display: flex;
    align-items: center;
    justify-content: center;
}

.empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.75rem;
    color: var(--app-text-secondary);
    text-align: center;
}

.empty-state h2 {
    margin: 0;
    font-size: 1.5rem;
    color: var(--app-text-primary);
}

.empty-state p {
    margin: 0;
}
</style>
