<script setup lang="ts">
import { computed, ref } from 'vue'
import Button from 'primevue/button'
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import ConfirmDialog from 'primevue/confirmdialog'
import { useConfirm } from 'primevue/useconfirm'
import {
    useRadioStations,
    useCreateRadioStation,
    useUpdateRadioStation,
    useDeleteRadioStation
} from '@/composables/useSubsonicQueries'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioBrowserStation, RadioStationPrefill } from '@/types/radiobrowser'
import { fetchRadioFavicon } from '@/lib/api/RadioBrowser'
import StationList from './radio-stations/StationList.vue'
import StationEditPanel from './radio-stations/StationEditPanel.vue'
import StationSearchDialog from './radio-stations/StationSearchDialog.vue'

const { data: stations, isLoading } = useRadioStations()
const createMutation = useCreateRadioStation()
const updateMutation = useUpdateRadioStation()
const deleteMutation = useDeleteRadioStation()
const confirm = useConfirm()

// The panel shows a station (edit), a blank form (add), or an idle prompt.
const selected = ref<InternetRadioStation | null>(null)
const adding = ref(false)
// Values to seed the Add form with when importing from radio-browser.
const prefill = ref<RadioStationPrefill | null>(null)
const pickerVisible = ref(false)

const submitting = computed(() => createMutation.isPending.value || updateMutation.isPending.value)

const summary = computed(() => {
    const count = stations.value?.length ?? 0
    if (count === 0) return ''
    return `${count} ${count === 1 ? 'station' : 'stations'}`
})

function openCreate() {
    selected.value = null
    prefill.value = null
    adding.value = true
}

function onSelect(station: InternetRadioStation) {
    selected.value = station
    prefill.value = null
    adding.value = false
}

// A successful create/update returns to the idle prompt; the invalidated query
// refetches the list, so a stale selected reference is never shown.
function resetEditor() {
    selected.value = null
    prefill.value = null
    adding.value = false
}

// Importing a station from radio-browser: seed the Add form immediately so it's
// responsive, then fetch the favicon in the background and fold it in as the
// cover once ready. The fetch is non-blocking (a slow/broken favicon must never
// delay the form) and guarded so a late result can't overwrite a later action.
async function onPickStation(station: RadioBrowserStation) {
    pickerVisible.value = false
    const base: RadioStationPrefill = {
        name: station.name,
        streamUrl: station.streamUrl,
        homepageUrl: station.homepage || undefined
    }
    selected.value = null
    prefill.value = base
    adding.value = true

    if (station.favicon) {
        const cover = await fetchRadioFavicon(station.favicon)
        if (cover && adding.value && prefill.value?.streamUrl === base.streamUrl) {
            prefill.value = { ...base, coverFile: cover }
        }
    }
}

interface StationInput {
    name: string
    streamUrl: string
    homepageUrl?: string
    coverFile?: File
    coverClear?: boolean
}

function onSave(input: StationInput) {
    if (adding.value) {
        createMutation.mutate(input, { onSuccess: resetEditor })
    } else if (selected.value) {
        updateMutation.mutate({ id: selected.value.id, ...input }, { onSuccess: resetEditor })
    }
}

function onDelete() {
    const station = selected.value
    if (!station) return
    confirm.require({
        message: `Delete station "${station.name}"? This cannot be undone.`,
        header: 'Delete station?',
        icon: 'pi pi-exclamation-triangle',
        rejectLabel: 'Cancel',
        acceptLabel: 'Delete',
        acceptClass: 'p-button-danger',
        accept: () => deleteMutation.mutate(station.id, { onSuccess: resetEditor })
    })
}
</script>

<template>
    <div class="radio-stations-editor">
        <div class="editor-header">
            <div class="header-title">
                <h1>Radio Stations</h1>
                <span v-if="summary" class="count">{{ summary }}</span>
            </div>
            <div class="header-actions">
                <Button
                    label="Search Online"
                    icon="pi pi-globe"
                    outlined
                    @click="pickerVisible = true"
                />
                <Button label="Add Station" icon="pi pi-plus" @click="openCreate" />
            </div>
        </div>

        <Splitter class="editor-splitter">
            <SplitterPanel :size="55" :minSize="30">
                <StationList
                    :stations="stations ?? []"
                    :selectedId="selected?.id ?? null"
                    :isLoading="isLoading"
                    @select="onSelect"
                />
            </SplitterPanel>
            <SplitterPanel :size="45" :minSize="30">
                <StationEditPanel
                    :station="selected"
                    :adding="adding"
                    :initial="prefill"
                    :submitting="submitting"
                    @save="onSave"
                    @delete="onDelete"
                />
            </SplitterPanel>
        </Splitter>

        <StationSearchDialog v-model:visible="pickerVisible" @select="onPickStation" />
        <ConfirmDialog />
    </div>
</template>

<style scoped>
.radio-stations-editor {
    display: flex;
    flex-direction: column;
    height: 100%;
    gap: 0.75rem;
    padding: 1rem;
}

.editor-header {
    display: flex;
    align-items: center;
    gap: 1rem;
    flex-shrink: 0;
}

.header-title {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 0.75rem;
}

.header-title h1 {
    margin: 0;
    font-size: 1.5rem;
    font-weight: 700;
}

.count {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}

.header-actions {
    display: flex;
    gap: 0.5rem;
    flex-shrink: 0;
}

.editor-splitter {
    flex: 1;
    min-height: 0;
}

:deep(.p-splitterpanel) {
    display: flex;
    flex-direction: column;
    overflow: hidden;
}
</style>
