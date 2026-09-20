<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import Dialog from 'primevue/dialog'
import ConfirmDialog from 'primevue/confirmdialog'
import Listbox from 'primevue/listbox'
import InputText from 'primevue/inputtext'
import Button from 'primevue/button'
import Message from 'primevue/message'
import Splitter from 'primevue/splitter'
import SplitterPanel from 'primevue/splitterpanel'
import { useConfirm } from 'primevue/useconfirm'
import { useScanFolders } from '@/composables/useScanFolders'
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

const { data: scanFolders } = useScanFolders()
const selectedScanFolder = ref<string | null>(null)
const selectedPath = ref<string | null>(null)
const selection = ref<Track[]>([])
const dialogVisible = ref(false)
// The path FolderTree should expand to when the picker opens (a clicked
// breadcrumb segment); null opens at the scan folder root. Cleared when the
// picker closes so re-picking the same segment re-triggers the expand.
const pendingExpandPath = ref<string | null>(null)

// Folder search: `folderSearch` is the raw text field; `folderFilter` is the
// debounced value handed to FolderTree, which switches to a server-side name
// search when it is non-blank. Debounced so a fast typist does not trigger a
// folder walk per keystroke; clearing is applied at once (nothing to wait for).
const folderSearch = ref('')
const folderFilter = ref('')
let folderSearchTimer: ReturnType<typeof setTimeout> | undefined
watch(folderSearch, (v) => {
    if (folderSearchTimer) clearTimeout(folderSearchTimer)
    if (v.trim() === '') {
        folderFilter.value = ''
        return
    }
    folderSearchTimer = setTimeout(() => (folderFilter.value = v), 400)
})
function clearFolderSearch() {
    if (folderSearchTimer) clearTimeout(folderSearchTimer)
    folderSearch.value = ''
    folderFilter.value = ''
}

// Clear the pending debounce timer on unmount so it cannot fire after the
// component is gone and mutate folderFilter on a torn-down instance.
onUnmounted(() => {
    if (folderSearchTimer) clearTimeout(folderSearchTimer)
})

const confirm = useConfirm()

const { tier } = useViewport()
const stacked = computed(() => tier.value === 'phone')

const folderOptions = computed(
    () => scanFolders.value?.map((f) => ({ label: f.name, value: f.name })) ?? []
)

// The scan-folder picker list only appears when there is a real choice to make.
// With a single scan folder it is redundant — that folder is auto-selected below.
const showFolderList = computed(() => folderOptions.value.length > 1)

// A single configured scan folder gets no picker list, so select it up front —
// otherwise the folder tree has no scan folder to browse and nothing to pick it
// with. Only fires while nothing is selected yet, so it never fights a choice.
watch(
    folderOptions,
    (opts) => {
        if (opts.length === 1 && selectedScanFolder.value === null) {
            selectedScanFolder.value = opts[0].value
        }
    },
    { immediate: true }
)

// crumbs is the selected scan folder + path as clickable breadcrumb segments:
// the scan folder (path '') then one per path part, each carrying the path
// accumulated up to it, so a click can expand the picker straight there. The
// folder's name IS the label — a selected folder always has a name.
const crumbs = computed<{ label: string; path: string }[]>(() => {
    if (selectedScanFolder.value === null) return []
    const out = [{ label: selectedScanFolder.value, path: '' }]
    if (selectedPath.value) {
        let acc = ''
        for (const part of selectedPath.value.split('/')) {
            acc = acc ? `${acc}/${part}` : part
            out.push({ label: part, path: acc })
        }
    }
    return out
})

// The selected folder's problem, when the server reports its root unusable
// (not mounted, not a directory, a symlinked root): browsing it can only fail,
// so say why.
const selectedFolderProblem = computed(() => {
    const f = scanFolders.value?.find((x) => x.name === selectedScanFolder.value)
    return f && !f.available ? (f.problem ?? 'its directory is not available') : null
})

// Nothing configured: the picker has nothing to offer and no folder can be
// auto-selected.
const noScanFolders = computed(
    () => scanFolders.value !== undefined && scanFolders.value.length === 0
)

const tracksQuery = useTracks(
    () => selectedScanFolder.value,
    () => selectedPath.value
)

const session = useEditSession(
    () => tracksQuery.data.value,
    () => selectedScanFolder.value
)

// A save (and any reload/invalidation) refetches the tracks into brand-new
// Track objects, but `selection` still holds the references captured before the
// write. EditPanel diffs its "original" baseline off `props.selection`, so left
// alone it stays pre-save until the user reselects — wrong whenever the server
// normalizes on write (genre trimming, the artist+MBID two-PUT split, a partial
// per-path failure). Remap by path onto the fresh data as it settles (dropping
// paths that vanished, preserving order) so the baseline tracks what is on disk.
// vue-query's structural sharing keeps this ref stable when nothing changed, so
// the watch only fires on a real refetch — no race with save() completing.
watch(
    () => tracksQuery.data.value,
    (fresh) => {
        if (!fresh || selection.value.length === 0) return
        const byPath = new Map(fresh.map((t) => [t.path, t]))
        selection.value = selection.value
            .map((t) => byPath.get(t.path))
            .filter((t): t is Track => t !== undefined)
    }
)

// Both identify flows (and the in-memory cache behind them) live in
// useIdentifyRuns: the dialog state, the abort controllers and the cache reads
// are one concern, and the view only wires them to its children.
const runs = useIdentifyRuns(() => selectedScanFolder.value)

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

function onScanFolderChange(val: string | null) {
    guardUnsaved(() => {
        // A different scan folder invalidates any pending expand target and any
        // active folder filter (its matches were scoped to the old scan folder).
        pendingExpandPath.value = null
        clearFolderSearch()
        selectedScanFolder.value = val
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

// openFolderPicker opens the folder dialog. expandTo tells FolderTree to expand
// straight to a path (a clicked breadcrumb segment); null opens at the scan
// folder root. Opening is unguarded — only committing a new folder discards edits.
function openFolderPicker(expandTo: string | null) {
    pendingExpandPath.value = expandTo
    dialogVisible.value = true
}

// Forget the target once the picker closes, so clicking the same segment again
// is a real change FolderTree's watcher acts on.
watch(dialogVisible, (open) => {
    if (!open) {
        pendingExpandPath.value = null
        clearFolderSearch()
    }
})

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
// confirmation.
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
            pickOverlayFields(
                candidateToOverlay(p.candidate, p.release, p.genres, p.releaseTypes),
                fields
            )
        ])
    )
    session.stageOverlays(entries)
    runs.trackDialog.value = false
}

function onAlbumIdentifyApply(picks: AlbumIdentifyPick[], fields: IdentifyFieldId[]) {
    const entries = new Map<string, TrackOverlay>(
        picks.map((p) => [
            p.path,
            pickOverlayFields(albumPickToOverlay(p, p.genres, p.releaseTypes), fields)
        ])
    )
    session.stageOverlays(entries)
    runs.albumDialog.value = false
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
                @click="openFolderPicker(null)"
            />
            <nav v-if="crumbs.length" class="folder-breadcrumb" aria-label="Selected folder">
                <template v-for="(c, i) in crumbs" :key="c.path || '__root__'">
                    <span v-if="i > 0" class="crumb-sep" aria-hidden="true">/</span>
                    <button
                        type="button"
                        class="crumb"
                        :data-test="`crumb-${i}`"
                        :title="`Browse from ${c.label}`"
                        @click="openFolderPicker(c.path)"
                    >
                        {{ c.label }}
                    </button>
                </template>
            </nav>
            <span v-else class="folder-breadcrumb">No folder selected</span>
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
                    :folderPath="selectedPath"
                    @update:selection="(s) => (selection = s)"
                />
            </SplitterPanel>
            <SplitterPanel :size="40" :minSize="20">
                <EditPanel
                    :selection="selection"
                    :scanFolder="selectedScanFolder"
                    :session="session"
                    :folderPath="selectedPath"
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
            :style="{ width: 'min(92vw, 60rem)' }"
        >
            <div class="dialog-content">
                <div v-if="showFolderList" class="scan-folder-column" data-test="scan-folder-column">
                    <label class="scan-folder-label">Scan folder</label>
                    <Listbox
                        :modelValue="selectedScanFolder"
                        @update:modelValue="onScanFolderChange"
                        :options="folderOptions"
                        optionLabel="label"
                        optionValue="value"
                        class="scan-folder-listbox"
                    />
                </div>
                <div class="tree-column">
                    <Message
                        v-if="selectedFolderProblem"
                        severity="warn"
                        :closable="false"
                        data-test="scan-folder-problem"
                    >
                        This scan folder cannot be browsed right now: {{ selectedFolderProblem }}
                    </Message>
                    <template v-if="noScanFolders">
                        <div class="empty" data-test="no-scan-folders">
                            No scan folders are configured. Add them under <code>ScanFolders</code>
                            in the server's config file and restart the server.
                        </div>
                    </template>
                    <template v-else>
                        <div class="tree-search">
                            <label class="tree-search-label" for="folder-filter">Search folders</label>
                            <InputText
                                id="folder-filter"
                                v-model="folderSearch"
                                placeholder="Filter folders by name"
                                class="tree-search-input"
                                :disabled="selectedScanFolder === null"
                            />
                        </div>
                        <div class="tree-body">
                            <FolderTree
                                :scanFolder="selectedScanFolder"
                                :filter="folderFilter"
                                :expandTo="pendingExpandPath"
                                @select="onFolderSelect"
                            />
                        </div>
                    </template>
                </div>
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
    display: flex;
    align-items: center;
    gap: 0.15rem;
    min-width: 0;
    font-size: 0.9rem;
    color: var(--app-text-secondary);
    white-space: nowrap;
    overflow: hidden;
}

.crumb {
    flex: 0 1 auto;
    min-width: 0;
    max-width: 16rem;
    padding: 0.1rem 0.3rem;
    margin: 0;
    background: none;
    border: none;
    border-radius: 4px;
    font: inherit;
    color: var(--app-text-secondary);
    cursor: pointer;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

/* The current folder is the last crumb; keep it readable but still clickable. */
.crumb:last-child {
    color: var(--app-text-primary);
}

.crumb:hover,
.crumb:focus-visible {
    color: var(--app-text-primary);
    background: var(--app-surface-alt);
    text-decoration: underline;
    outline: none;
}

.crumb-sep {
    flex: 0 0 auto;
    opacity: 0.5;
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
    gap: 1rem;
    height: min(70vh, 34rem);
}

.scan-folder-column {
    flex: 0 0 15rem;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
    min-height: 0;
}

.scan-folder-label {
    font-size: 0.85rem;
    font-weight: 600;
}

.scan-folder-listbox {
    flex: 1;
    min-height: 0;
}

.tree-column {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

/* The search sits at the top of the tree column, level with the Scan folder
   label in the left column. */
.tree-search {
    display: flex;
    flex-direction: column;
    gap: 0.5rem;
}

.tree-search-label {
    font-size: 0.85rem;
    font-weight: 600;
}

.tree-search-input {
    width: 100%;
}

.tree-body {
    flex: 1;
    min-height: 0;
}

/* Matches FolderTree.vue's own .empty rule: this view has none of its own,
   and the no-scan-folders notice replaces the tree here rather than inside it. */
.empty {
    padding: 1rem;
    color: var(--app-text-secondary);
    font-size: 0.9rem;
}

@media (max-width: 767.98px) {
    .editor-header {
        flex-wrap: wrap;
        row-gap: 0.75rem;
    }

    /* Stack the picker's two columns so neither is cramped on a phone. */
    .dialog-content {
        flex-direction: column;
        height: auto;
        max-height: 80vh;
    }

    .scan-folder-column {
        flex: 0 0 auto;
        max-height: 10rem;
    }
}
</style>
