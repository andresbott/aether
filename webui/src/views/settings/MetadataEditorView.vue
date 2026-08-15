<script setup lang="ts">
import { computed, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import Dialog from 'primevue/dialog'
import ConfirmDialog from 'primevue/confirmdialog'
import Select from 'primevue/select'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import { useConfirm } from 'primevue/useconfirm'
import { useLibraries } from '@/composables/useLibraries'
import { useTracks, useMetadataCapabilities } from '@/composables/useMetadataEditor'
import { useEditSession, candidateToOverlay, albumPickToOverlay } from '@/composables/useEditSession'
import { useIdentifyRuns } from '@/composables/useIdentifyRuns'
import { useViewport } from '@/composables/useViewport'
import FolderTree from './metadata-editor/FolderTree.vue'
import TrackList from './metadata-editor/TrackList.vue'
import EditPanel from './metadata-editor/EditPanel.vue'
import IdentifyReviewDialog from './metadata-editor/IdentifyReviewDialog.vue'
import IdentifyAlbumDialog from './metadata-editor/IdentifyAlbumDialog.vue'
import { pickOverlayFields, type IdentifyFieldId } from '@/lib/identifyFields'
import type { AlbumIdentifyPick, IdentifyPick, Track, TrackOverlay } from '@/types/metadata'

const { data: libraries } = useLibraries()
const selectedLibraryId = ref<number | null>(null)
const selectedPath = ref<string | null>(null)
const selection = ref<Track[]>([])
const dialogVisible = ref(false)
const confirm = useConfirm()

const { tier } = useViewport()
const stacked = computed(() => tier.value === 'phone')

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

// Both identify flows (and the in-memory cache behind them) live in
// useIdentifyRuns: the dialog state, the abort controllers and the cache reads
// are one concern, and the view only wires them to its children.
const runs = useIdentifyRuns(() => selectedLibraryId.value)

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

// Reload re-reads the folder AND forgets the cached identify answers: reloading
// is the user saying "read this from disk again", and a cached fingerprint answer
// for a file that has since been replaced is exactly the staleness they are
// clearing.
function onReload() {
    guardUnsaved(() => {
        runs.forgetAll()
        tracksQuery.refetch()
    })
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

function onIdentifyApply(picks: IdentifyPick[], fields: IdentifyFieldId[]) {
    const entries = new Map<string, TrackOverlay>(
        picks.map((p) => [
            p.path,
            pickOverlayFields(candidateToOverlay(p.candidate, p.release, p.genres), fields)
        ])
    )
    session.stageOverlays(entries)
    runs.trackDialog.value = false
    // New array reference so EditPanel's selection watcher refreshes its edit
    // buffers with the just-staged values.
    selection.value = [...selection.value]
}

function onAlbumIdentifyApply(picks: AlbumIdentifyPick[], fields: IdentifyFieldId[]) {
    const entries = new Map<string, TrackOverlay>(
        picks.map((p) => [p.path, pickOverlayFields(albumPickToOverlay(p, p.genres), fields)])
    )
    session.stageOverlays(entries)
    runs.albumDialog.value = false
    // New array reference so EditPanel's selection watcher refreshes its edit
    // buffers with the just-staged values.
    selection.value = [...selection.value]
}

// Re-identify: the user is asking past a cached answer, so the same files are
// looked up again with the cache bypassed. The dialog stays open — the fresh run
// repopulates it in place.
function onReidentify() {
    void runs.identify(runs.pendingTracks.value, { force: true })
}

function onAlbumReidentify() {
    void runs.identifyAlbum(runs.albumTracks.value, { force: true })
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

        <Message v-if="stacked" severity="info" :closable="false">
            The metadata editor works best on a larger screen.
        </Message>

        <Splitter :layout="stacked ? 'vertical' : 'horizontal'" class="editor-splitter">
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
                    :isIdentifying="runs.isIdentifying.value"
                    :isIdentifyingAlbum="runs.isIdentifyingAlbum.value"
                    @identify="(t) => runs.identify(t)"
                    @identify-album="(t) => runs.identifyAlbum(t)"
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
                    <Select
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
            v-model:visible="runs.trackDialog.value"
            :results="runs.trackResults.value"
            :tracks="tracksQuery.data.value ?? []"
            :loading="runs.isIdentifying.value"
            :pending="runs.pendingTracks.value"
            @apply="onIdentifyApply"
            @cancel="runs.cancelIdentify"
            @reidentify="onReidentify"
        />

        <IdentifyAlbumDialog
            v-model:visible="runs.albumDialog.value"
            :options="runs.albumOptions.value"
            :tracks="runs.albumTracks.value"
            :pathErrors="runs.albumPathErrors.value"
            :loading="runs.isIdentifyingAlbum.value"
            @apply="onAlbumIdentifyApply"
            @cancel="runs.cancelAlbumIdentify"
            @reidentify="onAlbumReidentify"
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

@media (max-width: 767.98px) {
    .editor-header {
        flex-wrap: wrap;
        row-gap: 0.75rem;
    }
}
</style>
