<script setup lang="ts">
import { computed, ref } from 'vue'
import Dialog from 'primevue/dialog'
import Dropdown from 'primevue/dropdown'
import Button from 'primevue/button'
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
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
const dialogVisible = ref(false)

const libraryOptions = computed(
    () => libraries.value?.map((l) => ({ label: l.name, value: l.id })) ?? []
)

const currentLibraryLabel = computed(() => {
    if (selectedLibraryId.value === null) return null
    return libraries.value?.find((l) => l.id === selectedLibraryId.value)?.name ?? null
})

const currentFolderLabel = computed(() => {
    if (selectedLibraryId.value === null) return 'No folder selected'
    if (selectedPath.value === null || selectedPath.value === '') return currentLibraryLabel.value ?? 'Library root'
    const parts = selectedPath.value.split('/')
    return `${currentLibraryLabel.value} / ${parts.join(' / ')}`
})

function onLibraryChange(val: number | null) {
    selectedLibraryId.value = val
    selectedPath.value = null
    selection.value = []
}

function onFolderSelect(path: string) {
    selectedPath.value = path
    selection.value = []
    dialogVisible.value = false
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
        <div class="editor-header">
            <Button
                icon="pi pi-folder-open"
                label="Change folder"
                severity="secondary"
                @click="dialogVisible = true"
            />
            <span class="folder-breadcrumb">{{ currentFolderLabel }}</span>
        </div>

        <Splitter class="editor-splitter">
            <SplitterPanel :size="60" :minSize="20">
                <TrackList
                    :tracks="tracksQuery.data.value ?? []"
                    :isLoading="tracksQuery.isLoading.value"
                    :selection="selection"
                    @update:selection="(s) => (selection = s)"
                    @reload="tracksQuery.refetch()"
                />
            </SplitterPanel>
            <SplitterPanel :size="40" :minSize="20">
                <EditPanel
                    :selection="selection"
                    :isSaving="updateMutation.isPending.value"
                    @save="save"
                />
            </SplitterPanel>
        </Splitter>

        <Dialog v-model:visible="dialogVisible" header="Select folder" modal :style="{ width: '40rem' }">
            <div class="dialog-content">
                <div class="dialog-library">
                    <label>Library</label>
                    <Dropdown
                        :modelValue="selectedLibraryId"
                        @update:modelValue="onLibraryChange"
                        :options="libraryOptions"
                        optionLabel="label"
                        optionValue="value"
                        placeholder="Select a library"
                        class="w-full"
                    />
                </div>
                <FolderTree
                    :libraryId="selectedLibraryId"
                    @select="onFolderSelect"
                />
            </div>
        </Dialog>
    </div>
</template>

<style scoped>
.metadata-editor {
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

.folder-breadcrumb {
    flex: 1;
    min-width: 0;
    font-size: 0.9rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
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

.dialog-content {
    display: flex;
    flex-direction: column;
    gap: 1rem;
}

.dialog-library {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.dialog-library label {
    font-size: 0.85rem;
    font-weight: 600;
}
</style>
