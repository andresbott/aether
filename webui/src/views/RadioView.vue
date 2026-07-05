<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import ConfirmDialog from 'primevue/confirmdialog'
import { useConfirm } from 'primevue/useconfirm'
import {
    useRadioStations,
    useCreateRadioStation,
    useUpdateRadioStation,
    useDeleteRadioStation
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song, InternetRadioStation } from '@/types/subsonic'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import RadioStationDialog from '@/components/library/RadioStationDialog.vue'

const { data: stations, isLoading } = useRadioStations()
const player = usePlayer()
const createMutation = useCreateRadioStation()
const updateMutation = useUpdateRadioStation()
const deleteMutation = useDeleteRadioStation()
const confirm = useConfirm()

const dialogVisible = ref(false)
const editing = ref<InternetRadioStation | null>(null)

const submitting = computed(
    () => createMutation.isPending.value || updateMutation.isPending.value
)

const summary = computed(() => {
    const count = stations.value?.length ?? 0
    if (count === 0) return ''
    return `${count} ${count === 1 ? 'station' : 'stations'}`
})

function coverStyle(station: InternetRadioStation) {
    if (!station.coverArt) return {}
    return {
        backgroundImage: `url(${subsonicClient.getCoverArtUrl(station.coverArt, 512)})`,
        backgroundSize: 'cover',
        backgroundPosition: 'center'
    }
}

function playStation(station: InternetRadioStation) {
    const song: Song = {
        id: `radio-${station.name}`,
        title: station.name,
        artist: 'Internet Radio',
        streamUrl: station.streamUrl,
        coverArt: station.coverArt
    }
    player.playNow(song)
}

function openCreate() {
    editing.value = null
    dialogVisible.value = true
}

function openEdit(station: InternetRadioStation) {
    editing.value = station
    dialogVisible.value = true
}

function onSubmit(input: {
    name: string
    streamUrl: string
    homepageUrl?: string
    coverFile?: File
    coverClear?: boolean
}) {
    if (editing.value) {
        updateMutation.mutate(
            {
                id: editing.value.id,
                name: input.name,
                streamUrl: input.streamUrl,
                homepageUrl: input.homepageUrl,
                coverFile: input.coverFile,
                coverClear: input.coverClear
            },
            { onSuccess: () => (dialogVisible.value = false) }
        )
    } else {
        createMutation.mutate(
            {
                name: input.name,
                streamUrl: input.streamUrl,
                homepageUrl: input.homepageUrl,
                coverFile: input.coverFile
            },
            { onSuccess: () => (dialogVisible.value = false) }
        )
    }
}

function onDelete(station: InternetRadioStation) {
    confirm.require({
        message: `Delete station "${station.name}"? This cannot be undone.`,
        header: 'Delete station?',
        icon: 'pi pi-exclamation-triangle',
        rejectLabel: 'Cancel',
        acceptLabel: 'Delete',
        acceptClass: 'p-button-danger',
        accept: () => deleteMutation.mutate(station.id)
    })
}
</script>

<template>
    <ContentScaffold title="Radio" :summary="summary">
        <template #actions>
            <Button label="Add Station" icon="pi pi-plus" @click="openCreate" />
        </template>

        <div class="radio-scroll">
            <div v-if="isLoading" class="loading">
                <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
            </div>

            <div v-else-if="stations && stations.length > 0" class="station-grid">
                <div
                    v-for="station in stations"
                    :key="station.id"
                    class="station-card"
                    :style="coverStyle(station)"
                    @click="playStation(station)"
                >
                    <div class="station-name">{{ station.name }}</div>
                    <div class="play-overlay">
                        <i class="pi pi-play" style="font-size: 1.5rem"></i>
                    </div>
                    <div class="card-actions">
                        <button
                            class="card-action-btn"
                            title="Edit station"
                            @click.stop="openEdit(station)"
                        >
                            <i class="pi pi-pencil"></i>
                        </button>
                        <button
                            class="card-action-btn"
                            title="Delete station"
                            @click.stop="onDelete(station)"
                        >
                            <i class="pi pi-trash"></i>
                        </button>
                    </div>
                </div>
            </div>

            <div v-else class="empty-state">
                <i class="pi pi-wifi" style="font-size: 3rem"></i>
                <p>No radio stations</p>
            </div>
        </div>

        <RadioStationDialog
            v-model:visible="dialogVisible"
            :station="editing"
            :submitting="submitting"
            @submit="onSubmit"
        />

        <ConfirmDialog />
    </ContentScaffold>
</template>

<style scoped>
.radio-scroll { height: 100%; overflow-y: auto; scrollbar-gutter: stable; }
.loading { display: flex; justify-content: center; padding: 3rem; color: var(--app-text-secondary); }
.station-grid { max-width: var(--app-content-max-width); margin: 0 auto; padding: 1rem; display: grid; grid-template-columns: repeat(auto-fill, minmax(200px, 1fr)); gap: 2rem; }
.station-card { position: relative; aspect-ratio: 1; border-radius: 12px; display: flex; align-items: flex-end; padding: 1.25rem; cursor: pointer; transition: transform 0.2s; overflow: hidden; background-color: #1f2937; }
.station-card:hover { transform: translateY(-2px); }
.station-card:hover .play-overlay { opacity: 1; }
.station-card:hover .card-actions { opacity: 1; }
.station-name { color: white; font-size: 1.1rem; font-weight: 600; text-shadow: 0 1px 3px rgba(0, 0, 0, 0.5); z-index: 2; }
.play-overlay { position: absolute; inset: 0; display: flex; align-items: center; justify-content: center; background: rgba(0, 0, 0, 0.3); color: white; opacity: 0; transition: opacity 0.2s; z-index: 1; }
.card-actions { position: absolute; top: 0.5rem; right: 0.5rem; display: flex; gap: 0.25rem; opacity: 0; transition: opacity 0.2s; z-index: 3; }
.card-action-btn { background: rgba(0, 0, 0, 0.5); border: none; color: white; width: 2rem; height: 2rem; border-radius: 50%; cursor: pointer; display: flex; align-items: center; justify-content: center; font-size: 0.85rem; transition: background 0.15s; }
.card-action-btn:hover { background: rgba(0, 0, 0, 0.8); }
.empty-state { display: flex; flex-direction: column; align-items: center; padding: 4rem; gap: 1rem; color: var(--app-text-secondary); }
</style>
