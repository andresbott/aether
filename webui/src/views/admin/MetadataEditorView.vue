<script setup lang="ts">
import { computed, ref } from 'vue'
import Dropdown from 'primevue/dropdown'
import { useLibraries } from '@/composables/useLibraries'
import {
    useTracks,
    useUpdateTracks
} from '@/composables/useMetadataEditor'
import FolderTree from './metadata-editor/FolderTree.vue'
import TrackList from './metadata-editor/TrackList.vue'
import EditPanel from './metadata-editor/EditPanel.vue'
import type { Track, PatchFields } from '@/types/metadata'

const { data: libraries } = useLibraries()
const selectedLibraryId = ref<number | null>(null)
const selectedPath = ref<string | null>(null)
const selection = ref<Track[]>([])

const libraryOptions = computed(
    () =>
        libraries.value?.map((l) => ({ label: l.name, value: l.id })) ?? []
)

// Reset selection when the library changes.
function onLibraryChange(val: number | null) {
    selectedLibraryId.value = val
    selectedPath.value = null
    selection.value = []
}

function onFolderSelect(path: string) {
    selectedPath.value = path
    selection.value = []
}

const tracksQuery = useTracks(
    () => selectedLibraryId.value,
    () => selectedPath.value
)

const updateMutation = useUpdateTracks()

function save(fields: PatchFields) {
    if (selectedLibraryId.value === null || selection.value.length === 0) return
    updateMutation.mutate({
        library_id: selectedLibraryId.value,
        paths: selection.value.map((t) => t.path),
        fields
    })
}
</script>

<template>
    <div class="metadata-editor">
        <h1>Metadata Editor</h1>

        <div class="library-picker">
            <label>Library</label>
            <Dropdown
                :modelValue="selectedLibraryId"
                @update:modelValue="onLibraryChange"
                :options="libraryOptions"
                optionLabel="label"
                optionValue="value"
                placeholder="Select a library"
            />
        </div>

        <div class="layout">
            <div class="left">
                <FolderTree
                    :libraryId="selectedLibraryId"
                    @select="onFolderSelect"
                />
            </div>
            <div class="right">
                <TrackList
                    :tracks="tracksQuery.data.value ?? []"
                    :isLoading="tracksQuery.isLoading.value"
                    :selection="selection"
                    @update:selection="(s) => (selection = s)"
                    @reload="tracksQuery.refetch()"
                />
                <EditPanel
                    :selection="selection"
                    :isSaving="updateMutation.isPending.value"
                    @save="save"
                />
            </div>
        </div>
    </div>
</template>

<style scoped>
.metadata-editor {
    max-width: 1400px;
    margin: 0 auto;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 1rem;
    height: calc(100vh - var(--app-player-height, 80px));
}
.library-picker {
    display: flex;
    gap: 0.75rem;
    align-items: center;
}
.library-picker label {
    font-size: 0.9rem;
    font-weight: 600;
}
.layout {
    display: grid;
    grid-template-columns: 20rem 1fr;
    gap: 1rem;
    flex: 1;
    min-height: 0;
}
.left {
    overflow-y: auto;
}
.right {
    display: flex;
    flex-direction: column;
    gap: 1rem;
    min-height: 0;
}
</style>
