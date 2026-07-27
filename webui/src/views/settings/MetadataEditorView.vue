<script setup lang="ts">
import { computed, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import Dialog from 'primevue/dialog'
import ConfirmDialog from 'primevue/confirmdialog'
import Dropdown from 'primevue/dropdown'
import Button from 'primevue/button'
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import { useConfirm } from 'primevue/useconfirm'
import { useLibraries } from '@/composables/useLibraries'
import { useTracks, useMetadataCapabilities, useIdentifyTracks } from '@/composables/useMetadataEditor'
import { useEditSession, candidateToOverlay } from '@/composables/useEditSession'
import FolderTree from './metadata-editor/FolderTree.vue'
import TrackList from './metadata-editor/TrackList.vue'
import EditPanel from './metadata-editor/EditPanel.vue'
import IdentifyReviewDialog from './metadata-editor/IdentifyReviewDialog.vue'
import type { IdentifyPick, IdentifyTrackResult, Track, TrackOverlay } from '@/types/metadata'

const { data: libraries } = useLibraries()
const selectedLibraryId = ref<number | null>(null)
const selectedPath = ref<string | null>(null)
const selection = ref<Track[]>([])
const dialogVisible = ref(false)
const confirm = useConfirm()

const libraryOptions = computed(
    () => libraries.value?.map((l) => ({ label: l.name, value: l.id })) ?? []
)

const currentLibraryLabel = computed(() => {
    if (selectedLibraryId.value === null) return null
    return libraries.value?.find((l) => l.id === selectedLibraryId.value)?.name ?? null
})

const currentFolderLabel = computed(() => {
    if (selectedLibraryId.value === null) return 'No folder selected'
    if (selectedPath.value === null || selectedPath.value === '')
        return currentLibraryLabel.value ?? 'Library root'
    const parts = selectedPath.value.split('/')
    return `${currentLibraryLabel.value} / ${parts.join(' / ')}`
})

const tracksQuery = useTracks(
    () => selectedLibraryId.value,
    () => selectedPath.value
)

const session = useEditSession(
    () => tracksQuery.data.value,
    () => selectedLibraryId.value
)

// guardUnsaved runs the action directly, or behind a discard confirmation when
// the session holds staged changes. Cancel leaves everything as-is.
function guardUnsaved(action: () => void) {
    if (!session.hasStagedChanges.value) {
        action()
        return
    }
    confirm.require({
        header: 'Unsaved changes',
        message: 'You have unsaved changes. Discard them?',
        icon: 'pi pi-exclamation-triangle',
        acceptLabel: 'Discard',
        rejectLabel: 'Cancel',
        acceptClass: 'p-button-danger',
        accept: () => {
            session.discardAll()
            action()
        }
    })
}

function onLibraryChange(val: number | null) {
    guardUnsaved(() => {
        selectedLibraryId.value = val
        selectedPath.value = null
        selection.value = []
    })
}

function onFolderSelect(path: string) {
    guardUnsaved(() => {
        selectedPath.value = path
        selection.value = []
        dialogVisible.value = false
    })
}

function onReload() {
    guardUnsaved(() => tracksQuery.refetch())
}

// onCancel reverts every staged change (field overlays and picture ops) after
// confirmation. The selection ref-copy makes EditPanel refresh its edit
// buffers back to the original values.
function onCancel() {
    confirm.require({
        header: 'Discard changes',
        message: 'Discard all staged changes?',
        icon: 'pi pi-exclamation-triangle',
        acceptLabel: 'Discard',
        rejectLabel: 'Keep editing',
        acceptClass: 'p-button-danger',
        accept: () => {
            session.discardAll()
            selection.value = [...selection.value]
        }
    })
}

onBeforeRouteLeave((_to, _from, next) => {
    if (!session.hasStagedChanges.value) {
        next()
        return
    }
    confirm.require({
        header: 'Unsaved changes',
        message: 'You have unsaved changes. Discard them and leave?',
        icon: 'pi pi-exclamation-triangle',
        acceptLabel: 'Discard',
        rejectLabel: 'Cancel',
        acceptClass: 'p-button-danger',
        accept: () => {
            session.discardAll()
            next()
        },
        reject: () => next(false)
    })
})

// ----- Identify -----

const capabilitiesQuery = useMetadataCapabilities()
const canIdentify = computed(() => capabilitiesQuery.data.value?.identify === true)
// When identification is off the button stays visible but disabled, explaining
// what the server is missing — a hidden button reads as a broken build.
const identifyUnavailableReason = computed(() => {
    if (canIdentify.value) return ''
    if (capabilitiesQuery.isPending.value) return 'Checking server capabilities…'
    return (
        capabilitiesQuery.data.value?.identify_unavailable_reason ??
        'Audio identification is not available on this server.'
    )
})
const identifyMutation = useIdentifyTracks()

const identifyResults = ref<IdentifyTrackResult[]>([])
const identifyDialog = ref(false)

async function identify(tracks: Track[]) {
    if (selectedLibraryId.value === null || tracks.length === 0) return
    const results = await identifyMutation.mutateAsync({
        library_id: selectedLibraryId.value,
        paths: tracks.map((t) => t.path)
    })
    identifyResults.value = results
    identifyDialog.value = true
}

function onIdentifyApply(picks: IdentifyPick[]) {
    const entries = new Map<string, TrackOverlay>(
        picks.map((p) => [p.path, candidateToOverlay(p.candidate, p.release)])
    )
    session.stageOverlays(entries)
    identifyDialog.value = false
    // New array reference so EditPanel's selection watcher refreshes its edit
    // buffers with the just-staged values.
    selection.value = [...selection.value]
}
</script>

<template>
    <div class="metadata-editor">
        <div class="editor-header">
            <Button
                icon="pi pi-folder-open"
                label="Select folder"
                severity="secondary"
                @click="dialogVisible = true"
            />
            <span class="folder-breadcrumb">{{ currentFolderLabel }}</span>
            <span class="count">({{ tracksQuery.data.value?.length ?? 0 }} files)</span>
            <Button
                icon="pi pi-refresh"
                text
                rounded
                aria-label="Reload"
                @click="onReload"
            />
            <span class="header-spacer"></span>
            <span v-if="session.isSaving.value" class="saving-note" data-test="saving-note">
                <i class="pi pi-spin pi-spinner"></i>
                Saving and re-indexing…
            </span>
            <span
                v-else-if="session.hasStagedChanges.value"
                class="unsaved-note"
                data-test="unsaved-note"
            >
                Unsaved changes
            </span>
            <Button
                label="Cancel"
                icon="pi pi-times"
                severity="secondary"
                outlined
                data-test="session-cancel"
                :disabled="!session.hasStagedChanges.value || session.isSaving.value"
                @click="onCancel"
            />
            <Button
                label="Save"
                icon="pi pi-save"
                data-test="session-save"
                :disabled="!session.hasStagedChanges.value || session.isSaving.value"
                :loading="session.isSaving.value"
                @click="session.save()"
            />
        </div>

        <Splitter class="editor-splitter">
            <SplitterPanel :size="60" :minSize="20">
                <TrackList
                    :tracks="tracksQuery.data.value ?? []"
                    :isLoading="tracksQuery.isLoading.value"
                    :selection="selection"
                    :stagedPaths="session.stagedPaths.value"
                    @update:selection="(s) => (selection = s)"
                />
            </SplitterPanel>
            <SplitterPanel :size="40" :minSize="20">
                <EditPanel
                    :selection="selection"
                    :libraryId="selectedLibraryId"
                    :session="session"
                    :canIdentify="canIdentify"
                    :identifyUnavailableReason="identifyUnavailableReason"
                    :isIdentifying="identifyMutation.isPending.value"
                    @identify="identify"
                />
            </SplitterPanel>
        </Splitter>

        <Dialog
            v-model:visible="dialogVisible"
            header="Select folder"
            modal
            :style="{ width: '40rem' }"
        >
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
                <FolderTree :libraryId="selectedLibraryId" @select="onFolderSelect" />
            </div>
        </Dialog>

        <IdentifyReviewDialog
            v-model:visible="identifyDialog"
            :results="identifyResults"
            :tracks="tracksQuery.data.value ?? []"
            @apply="onIdentifyApply"
        />

        <ConfirmDialog />
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
    min-width: 0;
    font-size: 0.9rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.count {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
}

.header-spacer {
    flex: 1;
}

.unsaved-note {
    font-size: 0.85rem;
    font-weight: 600;
    color: var(--app-staged);
    background-color: var(--app-staged-soft);
    border: 1px solid var(--app-staged);
    border-radius: 1rem;
    padding: 0.15rem 0.6rem;
    white-space: nowrap;
}

/* A save also re-indexes the written files server-side, so it takes noticeably
   longer than a plain tag write; the note keeps that wait reading as progress. */
.saving-note {
    display: inline-flex;
    align-items: center;
    gap: 0.4rem;
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
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
