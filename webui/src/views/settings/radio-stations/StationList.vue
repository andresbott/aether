<script setup lang="ts">
import { subsonicClient } from '@/lib/api/subsonic'
import type { InternetRadioStation } from '@/types/subsonic'

defineProps<{
    stations: InternetRadioStation[]
    selectedId: string | null
    isLoading: boolean
}>()

const emit = defineEmits<{
    (e: 'select', station: InternetRadioStation): void
}>()

function coverUrl(station: InternetRadioStation): string | null {
    if (!station.coverArt) return null
    return subsonicClient.getCoverArtUrl(station.coverArt, 48)
}
</script>

<template>
    <div class="station-list">
        <div v-if="isLoading" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 1.5rem"></i>
        </div>

        <div v-else-if="stations.length === 0" class="empty">No radio stations yet.</div>

        <div v-else class="list-scroll">
            <button
                v-for="station in stations"
                :key="station.id"
                type="button"
                class="station-row"
                :class="{ selected: station.id === selectedId }"
                @click="emit('select', station)"
            >
                <span class="row-cover">
                    <img v-if="coverUrl(station)" :src="coverUrl(station)!" alt="" />
                    <i v-else class="pi pi-wifi"></i>
                </span>
                <span class="row-info">
                    <span class="row-name">{{ station.name }}</span>
                    <span class="row-url">{{ station.streamUrl }}</span>
                </span>
            </button>
        </div>
    </div>
</template>

<style scoped>
.station-list {
    flex: 1;
    display: flex;
    flex-direction: column;
    min-height: 0;
    overflow: hidden;
}
.list-scroll {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
}
.loading,
.empty {
    padding: 2rem;
    text-align: center;
    color: var(--app-text-secondary);
}
.station-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    width: 100%;
    padding: 0.4rem 0.5rem;
    border: none;
    background: none;
    cursor: pointer;
    text-align: left;
    transition: background-color 0.15s;
    box-sizing: border-box;
}
.station-row:hover {
    background-color: var(--app-hover);
}
.station-row.selected {
    background-color: var(--app-accent-soft);
}
.row-cover {
    width: 40px;
    height: 40px;
    flex-shrink: 0;
    border-radius: 4px;
    overflow: hidden;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    display: flex;
    align-items: center;
    justify-content: center;
}
.row-cover img {
    width: 100%;
    height: 100%;
    object-fit: cover;
}
.row-cover i {
    font-size: 0.9rem;
    color: rgba(255, 255, 255, 0.85);
}
.row-info {
    min-width: 0;
    flex: 1;
    display: flex;
    flex-direction: column;
}
.row-name {
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--app-text-primary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.row-url {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
</style>
