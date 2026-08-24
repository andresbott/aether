<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Button from 'primevue/button'
import InputText from 'primevue/inputtext'
import Message from 'primevue/message'
import Tabs from 'primevue/tabs'
import TabList from 'primevue/tablist'
import Tab from 'primevue/tab'
import TabPanels from 'primevue/tabpanels'
import TabPanel from 'primevue/tabpanel'
import { useCoverArtSearch } from '@/composables/useCoverArtSearch'
import { useMusicBrainzReleaseSearch } from '@/composables/useMusicBrainzReleaseSearch'
import { fetchPictureFile, getPictureCandidateInfo } from '@/lib/api/Metadata'
import { apiErrorMessage } from '@/lib/apiError'
import { formatImageMeta } from '@/lib/imageMeta'
import {
    PICTURE_SLOT_LABELS,
    candidateMatchesType,
    pictureTypeLabel,
    sortCandidatesForType
} from '@/lib/pictureTypes'
import type {
    CoverCandidate,
    PictureCopySource,
    PictureSlot,
    StagedPictureSource
} from '@/types/metadata'
import type { MusicBrainzReleaseCandidate } from '@/types/artists'

const props = withDefaults(
    defineProps<{
        visible: boolean
        // The picture type + storage slot this picker fills — chosen before the
        // dialog opens, shown in the header. (`slot` is reserved in Vue templates,
        // hence pictureSlot.)
        pictureType: string
        pictureSlot: PictureSlot
        releaseMbid: string
        releaseGroupMbid: string
        // Images the album already holds elsewhere (other type+slot cells),
        // offered as copy sources.
        sources?: PictureCopySource[]
        // Prefills the manual release search.
        albumName?: string
    }>(),
    { sources: () => [], albumName: '' }
)
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // Emitted when the user confirms a choice. The image is NOT persisted here;
    // the parent stages it and writes it on Save.
    (e: 'select', source: StagedPictureSource): void
}>()

const {
    candidates,
    loading: searching,
    error: searchError,
    rateLimited: searchRateLimited,
    search
} = useCoverArtSearch()
const {
    results: releases,
    loading: searchingReleases,
    error: releaseError,
    rateLimited: releaseRateLimited,
    search: searchReleases
} = useMusicBrainzReleaseSearch()

// One image source at a time, whichever tab it came from. Switching tabs keeps
// the choice (it stays visible in the footer preview) so the user can browse
// around without losing it.
const selectedCandidate = ref<CoverCandidate | null>(null)
const selectedSource = ref<PictureCopySource | null>(null)
const uploadFile = ref<File | null>(null)
const uploadPreview = ref<string | null>(null)
// The picked online candidate's real size/dimensions/format. The grid shows a
// downscaled thumbnail, so this is probed server-side (downloading the full
// image) and shown before Save. metaSeq discards a slow probe once the pick
// changes.
const candidateMeta = ref<string | null>(null)
const candidateMetaLoading = ref(false)
let metaSeq = 0
// A copy source served by the server is downloaded on confirm; this reports
// that step.
const copying = ref(false)
const copyError = ref<string | null>(null)

// The search tab is a two-step flow so results and covers never stack up:
// 'query' picks which release to look at, 'covers' shows that release's images
// with a way back.
type SearchStep = 'query' | 'covers'
const searchStep = ref<SearchStep>('query')
// The release the shown covers belong to; null when they came from the album's
// own MusicBrainz IDs.
const coversRelease = ref<MusicBrainzReleaseCandidate | null>(null)
const query = ref('')
// Whether a cover lookup has run — tells "no images for this release" apart
// from "you haven't looked yet".
const searched = ref(false)
// Same distinction for the release list: the query prefills from the album
// name, so a non-empty box does NOT mean a search happened.
const searchedReleases = ref(false)
// The Cover Art Archive is a third-party service that goes down and throttles,
// so a lookup needs to be repeatable: remember which ids produced the current
// covers view and let retryCoverSearch re-run exactly that, rather than
// dropping the user back to the release list.
const lastCoverQuery = ref<{ mbid: string; releaseGroup: string } | null>(null)


const tab = ref('search')

const header = computed(
    () =>
        `Change ${pictureTypeLabel(props.pictureType).toLowerCase()} — ` +
        PICTURE_SLOT_LABELS[props.pictureSlot]
)
const canSearchByMbid = computed(
    () => props.releaseMbid.trim() !== '' || props.releaseGroupMbid.trim() !== ''
)
const canSearchByName = computed(() => query.value.trim().length >= 2)
const hasSource = computed(
    () =>
        selectedCandidate.value !== null ||
        selectedSource.value !== null ||
        uploadFile.value !== null
)
// Candidates depicting the requested type sort first.
const sortedCandidates = computed(() => sortCandidatesForType(candidates.value, props.pictureType))

// What the footer shows as the pending choice, so it stays visible from any tab.
const chosen = computed<{ label: string; thumbUrl: string } | null>(() => {
    if (selectedSource.value) {
        return { label: selectedSource.value.label, thumbUrl: selectedSource.value.thumbUrl }
    }
    if (selectedCandidate.value) {
        return {
            label: coverDescription(selectedCandidate.value),
            thumbUrl: selectedCandidate.value.thumbUrl
        }
    }
    if (uploadFile.value && uploadPreview.value) {
        return { label: uploadFile.value.name, thumbUrl: uploadPreview.value }
    }
    return null
})

function clearMeta() {
    metaSeq++
    candidateMeta.value = null
    candidateMetaLoading.value = false
}

function clearSelection() {
    selectedCandidate.value = null
    selectedSource.value = null
    clearUpload()
    clearMeta()
    copyError.value = null
}

function resetState() {
    clearSelection()
    searched.value = false
    searchedReleases.value = false
    searchStep.value = 'query'
    coversRelease.value = null
    lastCoverQuery.value = null
    candidates.value = []
    releases.value = []
    query.value = props.albumName
    copying.value = false
    // Land on the copy tab when the album already has images to copy — the
    // cheapest source, and the reason the dialog was likely opened.
    tab.value = props.sources.length > 0 ? 'copy' : 'search'
}

watch(
    () => props.visible,
    (visible) => {
        if (visible) resetState()
    },
    { immediate: true }
)

// --- Cover Art Archive ------------------------------------------------------

// searchByMbid uses the album's own MusicBrainz IDs; pickRelease uses the
// release the user found by title. Both land on the covers step.
function searchByMbid() {
    runCoverSearch(props.releaseMbid, props.releaseGroupMbid, null)
}

function runManualSearch() {
    if (!canSearchByName.value) return
    searchedReleases.value = true
    searchReleases(query.value)
}

function pickRelease(release: MusicBrainzReleaseCandidate) {
    runCoverSearch(release.releaseMbid, release.releaseGroupMbid, release)
}

function runCoverSearch(
    mbid: string,
    releaseGroup: string,
    release: MusicBrainzReleaseCandidate | null
) {
    searched.value = true
    coversRelease.value = release
    searchStep.value = 'covers'
    lastCoverQuery.value = { mbid, releaseGroup }
    search(mbid, releaseGroup)
}

function retryCoverSearch() {
    const q = lastCoverQuery.value
    if (!q) return
    search(q.mbid, q.releaseGroup)
}

function backToQuery() {
    searchStep.value = 'query'
    searched.value = false
    candidates.value = []
    searchError.value = null
    lastCoverQuery.value = null
}

// coversTitle names the release the shown covers belong to.
const coversTitle = computed(() => {
    if (coversRelease.value) return coversRelease.value.title
    return props.albumName || 'this album'
})

function releaseMeta(r: MusicBrainzReleaseCandidate): string {
    const parts: string[] = []
    if (r.date) parts.push(r.date)
    if (r.country) parts.push(r.country)
    if (r.trackCount) parts.push(`${r.trackCount} tracks`)
    if (r.disambiguation) parts.push(r.disambiguation)
    return parts.join(' · ')
}

async function pickCandidate(c: CoverCandidate) {
    clearSelection()
    selectedCandidate.value = c
    const seq = ++metaSeq
    candidateMetaLoading.value = true
    try {
        const info = await getPictureCandidateInfo(c.imageUrl)
        if (seq === metaSeq) candidateMeta.value = formatImageMeta(info)
    } catch {
        // A failed probe is non-fatal: the pick still works, just without its
        // metadata line.
        if (seq === metaSeq) candidateMeta.value = null
    } finally {
        if (seq === metaSeq) candidateMetaLoading.value = false
    }
}

// coverDescription summarises what an image depicts, from its Cover Art Archive
// types (e.g. "Front", "Back", "Booklet, Medium"), falling back to "Front"/"Cover".
function coverDescription(c: CoverCandidate): string {
    if (c.types && c.types.length > 0) return c.types.join(', ')
    if (c.isFront) return 'Front'
    return 'Cover'
}

// --- Copy from this album ---------------------------------------------------

function pickSource(s: PictureCopySource) {
    clearSelection()
    selectedSource.value = s
}

// --- Upload ----------------------------------------------------------------

const dragging = ref(false)

function acceptUpload(file: File | null) {
    if (!file) return
    selectedCandidate.value = null
    selectedSource.value = null
    clearMeta()
    copyError.value = null
    uploadFile.value = file
    if (uploadPreview.value) URL.revokeObjectURL(uploadPreview.value)
    uploadPreview.value = URL.createObjectURL(file)
}

function onFileChange(e: Event) {
    acceptUpload((e.target as HTMLInputElement).files?.[0] ?? null)
}

function onDrop(e: DragEvent) {
    dragging.value = false
    acceptUpload(e.dataTransfer?.files?.[0] ?? null)
}

function clearUpload() {
    uploadFile.value = null
    if (uploadPreview.value) {
        URL.revokeObjectURL(uploadPreview.value)
        uploadPreview.value = null
    }
}

// --- Confirm ---------------------------------------------------------------

async function confirmSelection() {
    if (!hasSource.value || copying.value) return
    const source = selectedSource.value
    if (source) {
        // A staged file/URL can be handed over as it is; an image the server
        // holds has to be downloaded first so it can be staged like an upload.
        if (source.fetchUrl) {
            copying.value = true
            copyError.value = null
            try {
                const file = await fetchPictureFile(source.fetchUrl)
                emit('select', { file, imageUrl: null })
            } catch (err: unknown) {
                copyError.value = apiErrorMessage(err, 'This image could not be copied. Try again in a moment.')
                return
            } finally {
                copying.value = false
            }
        } else {
            emit('select', { file: source.file, imageUrl: source.imageUrl })
        }
        emit('update:visible', false)
        return
    }
    emit('select', {
        file: uploadFile.value,
        imageUrl: uploadFile.value ? null : (selectedCandidate.value?.imageUrl ?? null),
        // Reuse the archive thumbnail for the staged preview; the full imageUrl is
        // downloaded server-side on save.
        previewUrl: uploadFile.value ? null : (selectedCandidate.value?.thumbUrl ?? null)
    })
    emit('update:visible', false)
}

function cancel() {
    emit('update:visible', false)
}
</script>

<template>
    <Dialog
        :visible="visible"
        @update:visible="emit('update:visible', $event)"
        modal
        :header="header"
        :style="{ width: '46rem' }"
    >
        <!-- lazy: only the active panel is in the DOM. Safe here because every
             piece of state (the chosen source, the search step, the query) lives
             in this component, not in the panels. -->
        <Tabs v-model:value="tab" lazy>
            <TabList>
                <Tab
                    v-if="sources.length > 0"
                    value="copy"
                    class="picker-tab"
                    data-test="picture-tab-copy"
                >
                    <i class="pi pi-copy"></i>
                    <span>This album ({{ sources.length }})</span>
                </Tab>
                <Tab value="search" class="picker-tab" data-test="picture-tab-search">
                    <i class="pi pi-search"></i>
                    <span>Search online</span>
                </Tab>
                <Tab value="upload" class="picker-tab" data-test="picture-tab-upload">
                    <i class="pi pi-upload"></i>
                    <span>Upload</span>
                </Tab>
            </TabList>

            <TabPanels>
                <!-- Copy: images this album already has in another type/slot. -->
                <TabPanel v-if="sources.length > 0" value="copy" data-test="picture-sources">
                    <p class="panel-hint">
                        Reuse an image this album already carries somewhere else.
                    </p>
                    <div class="tile-grid">
                        <button
                            v-for="s in sources"
                            :key="s.key"
                            type="button"
                            class="tile source-tile"
                            :class="{ selected: selectedSource?.key === s.key }"
                            :data-test="`picture-source-${s.key}`"
                            @click="pickSource(s)"
                        >
                            <span class="tile-art">
                                <img :src="s.thumbUrl" :alt="s.label" />
                                <i
                                    v-if="selectedSource?.key === s.key"
                                    class="pi pi-check-circle tile-check"
                                ></i>
                            </span>
                            <span class="tile-label">{{ s.label }}</span>
                            <span v-if="s.detail" class="tile-detail">{{ s.detail }}</span>
                        </button>
                    </div>
                    <Message v-if="copyError" severity="error" :closable="false">{{
                        copyError
                    }}</Message>
                </TabPanel>

                <!-- Search: step 1 find a release, step 2 pick one of its covers. -->
                <TabPanel value="search">
                    <template v-if="searchStep === 'query'">
                        <div class="search-bar">
                            <InputText
                                v-model="query"
                                class="search-input"
                                placeholder="Search MusicBrainz by album title"
                                data-test="picture-manual-query"
                                @keyup.enter="runManualSearch"
                            />
                            <Button
                                label="Search"
                                icon="pi pi-search"
                                data-test="picture-manual-search"
                                :disabled="!canSearchByName"
                                :loading="searchingReleases"
                                @click="runManualSearch"
                            />
                        </div>
                        <div class="shortcut-row">
                            <Button
                                label="Use this album’s MusicBrainz ID"
                                icon="pi pi-bolt"
                                size="small"
                                text
                                data-test="picture-search"
                                :disabled="!canSearchByMbid"
                                :loading="searching"
                                @click="searchByMbid"
                            />
                            <small v-if="!canSearchByMbid" class="shortcut-note">
                                No release or release-group MusicBrainz ID on these files yet.
                            </small>
                        </div>

                        <Message
                            v-if="releaseError"
                            :severity="releaseRateLimited ? 'warn' : 'error'"
                            :closable="false"
                            data-test="picture-release-error"
                        >
                            <div class="lookup-error">
                                <span>{{ releaseError }}</span>
                                <Button
                                    label="Try again"
                                    icon="pi pi-refresh"
                                    size="small"
                                    text
                                    data-test="picture-release-retry"
                                    :loading="searchingReleases"
                                    @click="runManualSearch"
                                />
                            </div>
                        </Message>

                        <div class="results">
                            <div v-if="searchingReleases" class="searching">
                                <i class="pi pi-spin pi-spinner"></i>
                            </div>
                            <ul
                                v-else-if="releases.length > 0"
                                class="release-list"
                                data-test="picture-release-list"
                            >
                                <li
                                    v-for="r in releases"
                                    :key="r.releaseMbid"
                                    class="release-row"
                                    @click="pickRelease(r)"
                                >
                                    <div class="release-body">
                                        <span class="release-title">{{ r.title }}</span>
                                        <span v-if="r.artist" class="release-artist">{{
                                            r.artist
                                        }}</span>
                                        <span v-if="releaseMeta(r)" class="release-meta">{{
                                            releaseMeta(r)
                                        }}</span>
                                    </div>
                                    <i class="pi pi-angle-right release-go"></i>
                                </li>
                            </ul>
                            <!-- A failed search is not an empty one: the error
                                 block already explains it, so don't also claim
                                 nothing matched. -->
                            <p v-else-if="searchedReleases && !releaseError" class="hint">
                                No releases matched. Try a shorter title.
                            </p>
                            <p v-else class="hint">
                                Search a release to browse its Cover Art Archive images.
                            </p>
                        </div>
                    </template>

                    <template v-else>
                        <div class="covers-head">
                            <Button
                                icon="pi pi-arrow-left"
                                label="Change release"
                                size="small"
                                text
                                data-test="picture-covers-back"
                                @click="backToQuery"
                            />
                            <span class="covers-title" data-test="picture-release-note">
                                Images for “{{ coversTitle }}”
                            </span>
                        </div>

                        <Message
                            v-if="searchError"
                            :severity="searchRateLimited ? 'warn' : 'error'"
                            :closable="false"
                            data-test="picture-search-error"
                        >
                            <div class="lookup-error">
                                <span>{{ searchError }}</span>
                                <Button
                                    label="Try again"
                                    icon="pi pi-refresh"
                                    size="small"
                                    text
                                    data-test="picture-search-retry"
                                    :loading="searching"
                                    @click="retryCoverSearch"
                                />
                            </div>
                        </Message>

                        <div class="results">
                            <div v-if="searching" class="searching">
                                <i class="pi pi-spin pi-spinner"></i>
                            </div>
                            <div v-else-if="sortedCandidates.length > 0" class="tile-grid">
                                <button
                                    v-for="c in sortedCandidates"
                                    :key="c.id"
                                    type="button"
                                    class="tile cover-tile"
                                    :class="{ selected: selectedCandidate?.id === c.id }"
                                    @click="pickCandidate(c)"
                                >
                                    <span class="tile-art">
                                        <img :src="c.thumbUrl" :alt="coverDescription(c)" />
                                        <i
                                            v-if="selectedCandidate?.id === c.id"
                                            class="pi pi-check-circle tile-check"
                                        ></i>
                                    </span>
                                    <span class="tile-label">
                                        {{ coverDescription(c) }}
                                        <span
                                            v-if="candidateMatchesType(c, pictureType)"
                                            class="type-match"
                                            data-test="type-match"
                                            >match</span
                                        >
                                    </span>
                                    <span v-if="c.comment" class="tile-detail">{{
                                        c.comment
                                    }}</span>
                                </button>
                            </div>
                            <p v-else-if="searched && !searchError" class="hint">
                                No images found for this release.
                            </p>
                        </div>
                    </template>
                </TabPanel>

                <!-- Upload: click or drop a file. -->
                <TabPanel value="upload">
                    <label
                        class="dropzone"
                        :class="{ dragging }"
                        @dragover.prevent="dragging = true"
                        @dragleave="dragging = false"
                        @drop.prevent="onDrop"
                    >
                        <input
                            type="file"
                            class="dropzone-input"
                            accept="image/png,image/jpeg"
                            data-test="picture-upload"
                            @change="onFileChange"
                        />
                        <i class="pi pi-image dropzone-icon"></i>
                        <span class="dropzone-title">Drop an image here, or click to choose</span>
                        <small class="dropzone-note">PNG or JPEG</small>
                    </label>
                    <div v-if="uploadPreview" class="upload-preview" data-test="picture-upload-preview">
                        <img :src="uploadPreview" alt="Upload preview" />
                        <div class="upload-meta">
                            <span class="upload-name">{{ uploadFile?.name }}</span>
                            <Button
                                label="Remove"
                                icon="pi pi-times"
                                size="small"
                                text
                                severity="secondary"
                                data-test="picture-upload-clear"
                                @click="clearUpload()"
                            />
                        </div>
                    </div>
                </TabPanel>
            </TabPanels>
        </Tabs>

        <!-- The global footer rule reverses dialog footers (confirm | Cancel),
             so extra items are authored in reverse: the pending-choice preview
             goes LAST here to render flush left. See _main.scss. -->
        <template #footer>
            <Button label="Cancel" text @click="cancel" />
            <Button
                label="Select"
                data-test="picture-select"
                :disabled="!hasSource"
                :loading="copying"
                @click="confirmSelection"
            />
            <div class="footer-chosen" data-test="picture-chosen">
                <template v-if="chosen">
                    <img class="chosen-thumb" :src="chosen.thumbUrl" alt="" />
                    <div class="chosen-text">
                        <span class="chosen-label">{{ chosen.label }}</span>
                        <span v-if="selectedCandidate && candidateMetaLoading" class="chosen-meta"
                            >Checking image…</span
                        >
                        <span
                            v-else-if="selectedCandidate && candidateMeta"
                            class="chosen-meta"
                            data-test="candidate-meta"
                            >{{ candidateMeta }}</span
                        >
                    </div>
                </template>
                <span v-else class="chosen-empty">No image chosen yet</span>
            </div>
        </template>
    </Dialog>
</template>

<style scoped>
/* The theme lays tabs out as plain inline content; space the icon off the label. */
.picker-tab {
    display: inline-flex;
    align-items: center;
    gap: 0.45rem;
}

.panel-hint,
.hint {
    color: var(--app-text-secondary);
    font-size: 0.85rem;
}
.panel-hint {
    margin: 0 0 0.75rem;
}
.hint {
    text-align: center;
    padding: 2rem 0;
    margin: 0;
}

/* --- Image tiles (shared by the copy and cover grids) --- */
.tile-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(8.5rem, 1fr));
    gap: 0.75rem;
}
.tile {
    display: flex;
    flex-direction: column;
    gap: 0.2rem;
    padding: 0.4rem;
    background: none;
    border: 1px solid transparent;
    border-radius: 8px;
    cursor: pointer;
    text-align: left;
    color: inherit;
    font: inherit;
}
.tile:hover {
    background: var(--app-bg-subtle, #f3f4f6);
}
.tile.selected {
    border-color: var(--app-accent);
    background: var(--app-accent-soft);
}
.tile-art {
    position: relative;
    display: block;
    width: 100%;
    aspect-ratio: 1 / 1;
}
.tile-art img {
    width: 100%;
    height: 100%;
    object-fit: cover;
    border-radius: 6px;
    border: 1px solid var(--app-border);
    background: var(--app-bg-subtle, #f3f4f6);
    display: block;
}
.tile-check {
    position: absolute;
    top: 0.3rem;
    right: 0.3rem;
    font-size: 1.15rem;
    color: var(--app-accent);
    background: var(--app-surface-2);
    border-radius: 50%;
}
.tile-label {
    font-size: 0.8rem;
    font-weight: 500;
    line-height: 1.25;
}
.tile-detail {
    font-size: 0.72rem;
    color: var(--app-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.type-match {
    font-size: 0.65rem;
    font-weight: 600;
    color: var(--app-accent);
    border: 1px solid var(--app-accent);
    border-radius: 999px;
    padding: 0.05rem 0.4rem;
    margin-left: 0.25rem;
    white-space: nowrap;
}

/* --- Search tab --- */
.search-bar {
    display: flex;
    gap: 0.5rem;
}
.search-input {
    flex: 1;
    min-width: 0;
}
.shortcut-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    flex-wrap: wrap;
    margin-top: 0.35rem;
}
.shortcut-note {
    color: var(--app-text-secondary);
    font-size: 0.75rem;
}
.results {
    min-height: 10rem;
    max-height: 22rem;
    overflow-y: auto;
    margin-top: 0.5rem;
}
.searching {
    display: flex;
    justify-content: center;
    padding: 3rem 0;
    color: var(--app-text-secondary);
}
.release-list {
    list-style: none;
    margin: 0;
    padding: 0;
}
.release-row {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    padding: 0.5rem 0.65rem;
    border-radius: 6px;
    cursor: pointer;
}
.release-row:hover {
    background: var(--app-bg-subtle, #f3f4f6);
}
.release-body {
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
    min-width: 0;
    flex: 1;
}
.release-title {
    font-weight: 500;
}
.release-artist,
.release-meta {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.release-go {
    color: var(--app-text-secondary);
    flex: 0 0 auto;
}
.covers-head {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    padding-bottom: 0.5rem;
    border-bottom: 1px solid var(--app-border);
}
.covers-title {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

/* --- Upload tab --- */
.dropzone {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    padding: 2.5rem 1rem;
    border: 1px dashed var(--app-border);
    border-radius: 8px;
    cursor: pointer;
    color: var(--app-text-secondary);
}
.dropzone:hover,
.dropzone.dragging {
    border-color: var(--app-accent);
    background: var(--app-accent-soft);
}
.dropzone-input {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
    pointer-events: none;
}
.dropzone-icon {
    font-size: 1.75rem;
}
.dropzone-title {
    font-size: 0.9rem;
    font-weight: 500;
    color: var(--app-text);
}
.dropzone-note {
    font-size: 0.75rem;
}
.upload-preview {
    display: flex;
    align-items: center;
    gap: 0.75rem;
    margin-top: 0.75rem;
}
.upload-preview img {
    width: 5rem;
    height: 5rem;
    object-fit: cover;
    border-radius: 6px;
    border: 1px solid var(--app-border);
}
.upload-meta {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.15rem;
    min-width: 0;
}
.upload-name {
    font-size: 0.85rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

/* A lookup failure reads as a sentence plus a way to try again. */
.lookup-error {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.75rem;
    flex-wrap: wrap;
}

/* --- Footer --- */
.footer-chosen {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    margin-right: auto;
    min-width: 0;
}
.chosen-thumb {
    width: 2.25rem;
    height: 2.25rem;
    object-fit: cover;
    border-radius: 4px;
    border: 1px solid var(--app-border);
}
.chosen-text {
    display: flex;
    flex-direction: column;
    min-width: 0;
}
.chosen-label {
    font-size: 0.8rem;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    max-width: 18rem;
}
.chosen-meta {
    font-size: 0.72rem;
    color: var(--app-text-secondary);
    font-variant-numeric: tabular-nums;
}
.chosen-empty {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
</style>
