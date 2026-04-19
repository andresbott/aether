<script setup lang="ts">
import Button from 'primevue/button'
import { useRadioStations } from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import type { Song } from '@/types/subsonic'

const { data: stations, isLoading } = useRadioStations()
const player = usePlayer()

const gradients = [
    'linear-gradient(135deg, #667eea 0%, #764ba2 100%)',
    'linear-gradient(135deg, #f093fb 0%, #f5576c 100%)',
    'linear-gradient(135deg, #4facfe 0%, #00f2fe 100%)',
    'linear-gradient(135deg, #43e97b 0%, #38f9d7 100%)',
    'linear-gradient(135deg, #fa709a 0%, #fee140 100%)',
    'linear-gradient(135deg, #a18cd1 0%, #fbc2eb 100%)',
    'linear-gradient(135deg, #fccb90 0%, #d57eeb 100%)',
    'linear-gradient(135deg, #e0c3fc 0%, #8ec5fc 100%)',
    'linear-gradient(135deg, #f5576c 0%, #ff6a88 100%)',
    'linear-gradient(135deg, #89f7fe 0%, #66a6ff 100%)'
]

const playStation = (station: { name: string; streamUrl: string }) => {
    const song: Song = {
        id: `radio-${station.name}`,
        title: station.name,
        artist: 'Internet Radio',
        streamUrl: station.streamUrl
    }
    player.playNow(song)
}
</script>

<template>
    <div class="radio-view">
        <div class="view-header">
            <h1>Radio</h1>
            <Button label="Add Station" icon="pi pi-plus" disabled />
        </div>

        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="stations && stations.length > 0" class="station-grid">
            <div
                v-for="(station, index) in stations"
                :key="station.id"
                class="station-card"
                :style="{ background: gradients[index % gradients.length] }"
                @click="playStation(station)"
            >
                <div class="station-name">{{ station.name }}</div>
                <div class="play-overlay">
                    <i class="pi pi-play" style="font-size: 1.5rem"></i>
                </div>
            </div>
        </div>

        <div v-else class="empty-state">
            <i class="pi pi-wifi" style="font-size: 3rem"></i>
            <p>No radio stations</p>
        </div>
    </div>
</template>

<style scoped>
.radio-view { max-width: 1400px; margin: 0 auto; }
.view-header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 2rem; }
.view-header h1 { font-size: 2rem; font-weight: 700; margin: 0; }
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.station-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 1.5rem; }
.station-card { position: relative; aspect-ratio: 1; border-radius: 12px; display: flex; align-items: flex-end; padding: 1.25rem; cursor: pointer; transition: transform 0.2s; overflow: hidden; }
.station-card:hover { transform: translateY(-2px); }
.station-card:hover .play-overlay { opacity: 1; }
.station-name { color: white; font-size: 1.1rem; font-weight: 600; text-shadow: 0 1px 3px rgba(0, 0, 0, 0.3); z-index: 1; }
.play-overlay { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; background: rgba(0, 0, 0, 0.3); color: white; opacity: 0; transition: opacity 0.2s; }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
</style>
