<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter, onBeforeRouteLeave } from 'vue-router'
import Button from 'primevue/button'
import ConfirmDialog from 'primevue/confirmdialog'
import { useConfirm } from 'primevue/useconfirm'
import ContentScaffold from '@/components/layout/ContentScaffold.vue'
import RadioStationForm from '@/components/library/RadioStationForm.vue'
import {
    useRadioStations,
    useCreateRadioStation,
    useUpdateRadioStation,
    useDeleteRadioStation
} from '@/composables/useSubsonicQueries'
import { usePlayer } from '@/composables/usePlayer'
import { stationToSong } from '@/utils/radioSong'
import { fetchRadioFavicon } from '@/lib/api/RadioBrowser'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'

const props = defineProps<{ id?: string; create?: boolean }>()
const route = useRoute()
const router = useRouter()
const player = usePlayer()
const confirm = useConfirm()

const { data: stations, isLoading } = useRadioStations()
const createMutation = useCreateRadioStation()
const updateMutation = useUpdateRadioStation()
const deleteMutation = useDeleteRadioStation()

interface StationInput {
    name: string
    streamUrl: string
    homepageUrl?: string
    coverFile?: File
    coverClear?: boolean
}
const latest = ref<{ input: StationInput; valid: boolean; dirty: boolean }>({
    input: { name: '', streamUrl: '' },
    valid: false,
    dirty: false
})
// After a successful create/save/delete, suppress the unsaved-changes guard.
const submittedClean = ref(false)
function onFormChange(payload: { input: StationInput; valid: boolean; dirty: boolean }) {
    latest.value = payload
    if (payload.dirty) submittedClean.value = false
}

// Edit mode resolves the station from the cached list (Subsonic has no single GET).
const station = computed<InternetRadioStation | null>(() =>
    props.create ? null : (stations.value?.find((s) => s.id === props.id) ?? null)
)
const notFound = computed(() => !props.create && !isLoading.value && !station.value)

// Create-mode prefill from Discover query params; fetch the favicon lazily.
const prefill = ref<RadioStationPrefill | null>(null)
if (props.create) {
    const q = route.query
    const name = typeof q.name === 'string' ? q.name : ''
    const streamUrl = typeof q.streamUrl === 'string' ? q.streamUrl : ''
    const homepage = typeof q.homepage === 'string' ? q.homepage : undefined
    if (name || streamUrl) {
        prefill.value = { name, streamUrl, homepageUrl: homepage }
    }
}
onMounted(async () => {
    if (!props.create || !prefill.value) return
    const q = route.query
    const favicon = typeof q.favicon === 'string' ? q.favicon : ''
    if (!favicon) return
    const base = prefill.value
    const cover = await fetchRadioFavicon(favicon)
    if (cover && prefill.value?.streamUrl === base.streamUrl) {
        prefill.value = { ...base, coverFile: cover }
    }
})

const submitting = computed(() => createMutation.isPending.value || updateMutation.isPending.value)
const title = computed(() => (props.create ? 'Add station' : (station.value?.name ?? '')))
const summary = computed(() => (props.create ? '' : (station.value?.streamUrl ?? '')))

function onCreate() {
    if (!latest.value.valid) return
    createMutation.mutate(latest.value.input, {
        onSuccess: () => {
            submittedClean.value = true
            router.push({ name: 'radio' })
        }
    })
}
function onSave() {
    if (!latest.value.valid || !station.value) return
    updateMutation.mutate(
        { id: station.value.id, ...latest.value.input },
        { onSuccess: () => (submittedClean.value = true) }
    )
}
function onDelete() {
    const s = station.value
    if (!s) return
    confirm.require({
        message: `Delete station "${s.name}"? This cannot be undone.`,
        header: 'Delete station?',
        icon: 'pi pi-exclamation-triangle',
        rejectLabel: 'Cancel',
        acceptLabel: 'Delete',
        acceptClass: 'p-button-danger',
        accept: () =>
            deleteMutation.mutate(s.id, {
                onSuccess: () => {
                    submittedClean.value = true
                    router.push({ name: 'radio' })
                }
            })
    })
}
function onPlay() {
    if (station.value) player.playNow(stationToSong(station.value))
}

onBeforeRouteLeave(() => {
    if (latest.value.dirty && !submittedClean.value) {
        return window.confirm('You have unsaved changes. Leave without saving?')
    }
})
const onBeforeUnload = (e: BeforeUnloadEvent): void => {
    if (!latest.value.dirty || submittedClean.value) return
    e.preventDefault()
    e.returnValue = ''
}
onMounted(() => window.addEventListener('beforeunload', onBeforeUnload))
onUnmounted(() => window.removeEventListener('beforeunload', onBeforeUnload))
</script>

<template>
    <div class="radio-station-detail-view">
        <div class="back-row">
            <Button icon="pi pi-arrow-left" text rounded @click="router.back()" />
        </div>

        <div v-if="isLoading && !create" class="loading">
            <i class="pi pi-spin pi-spinner" style="font-size: 2rem"></i>
        </div>

        <div v-else-if="notFound" class="error">
            <i class="pi pi-exclamation-triangle" style="font-size: 2rem"></i>
            <p>Station not found</p>
        </div>

        <ContentScaffold v-else :title="title" :summary="summary">
            <template #actions>
                <Button
                    v-if="!create"
                    class="play-station"
                    label="Play"
                    icon="pi pi-play"
                    @click="onPlay"
                />
                <Button
                    v-if="create"
                    class="create-station"
                    label="Create"
                    icon="pi pi-plus"
                    :disabled="!latest.valid"
                    :loading="submitting"
                    @click="onCreate"
                />
                <Button
                    v-else
                    class="save-station"
                    label="Save"
                    icon="pi pi-check"
                    :disabled="!latest.valid"
                    :loading="submitting"
                    @click="onSave"
                />
                <Button
                    v-if="!create"
                    class="delete-station"
                    icon="pi pi-trash"
                    text
                    rounded
                    severity="danger"
                    v-tooltip.bottom="'Delete station'"
                    @click="onDelete"
                />
            </template>

            <div class="detail-scroll">
                <div class="detail-body">
                    <RadioStationForm
                        :station="station"
                        :prefill="prefill"
                        @change="onFormChange"
                    />
                </div>
            </div>
        </ContentScaffold>

        <ConfirmDialog />
    </div>
</template>

<style scoped>
.radio-station-detail-view {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
}
.back-row {
    flex-shrink: 0;
    padding: 0.5rem 2rem 0;
}
.loading,
.error {
    display: flex;
    flex-direction: column;
    align-items: center;
    padding: 3rem;
    gap: 1rem;
    color: var(--app-text-secondary);
}
.detail-scroll {
    height: 100%;
    overflow-y: auto;
    scrollbar-gutter: stable;
}
.detail-body {
    max-width: var(--app-content-max-width);
    margin: 0 auto;
    padding: 1rem;
}
</style>
