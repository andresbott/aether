<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import Checkbox from 'primevue/checkbox'
import Button from 'primevue/button'
import type {
    Track,
    PatchFields,
    CoverTarget,
    CoverSourceEntry,
    StagedCover
} from '@/types/metadata'
import {
    diffInitialValues,
    distinctArtistMbids,
    useApplyCover,
    useDeleteCover,
    type ArtistMbidRow
} from '@/composables/useMetadataEditor'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'
import MusicBrainzAlbumPicker from '@/components/library/MusicBrainzAlbumPicker.vue'
import AlbumCoverPicker from '@/components/library/AlbumCoverPicker.vue'
import { getSourceCoverUrl, getCoverInfo } from '@/lib/api/Metadata'

const props = defineProps<{
    selection: Track[]
    isSaving: boolean
    libraryId: number | null
}>()
const emit = defineEmits<{
    (e: 'save', fields: PatchFields): void
}>()

type Scope = 'artist' | 'album_artist'

// One artist "group": an editable name and its MusicBrainz ID. `mixed` flags a
// name whose selected tracks disagree on the ID (shown blank until edited).
interface Pair {
    name: string
    mbid: string
    mixed: boolean
}

const isMass = computed(() => props.selection.length > 1)

type ScalarValues = {
    title: string
    album: string
    mb_release_id: string
    mb_release_group_id: string
    year: number
    disc_number: number
    disc_subtitle: string
    compilation: boolean
}
type DirtyFlags = {
    title: boolean
    album: boolean
    mb_release_id: boolean
    mb_release_group_id: boolean
    year: boolean
    disc_number: boolean
    disc_subtitle: boolean
    compilation: boolean
}

const values = ref<ScalarValues>({
    title: '',
    album: '',
    mb_release_id: '',
    mb_release_group_id: '',
    year: 0,
    disc_number: 0,
    disc_subtitle: '',
    compilation: false
})
const dirty = ref<DirtyFlags>({
    title: false,
    album: false,
    mb_release_id: false,
    mb_release_group_id: false,
    year: false,
    disc_number: false,
    disc_subtitle: false,
    compilation: false
})
const placeholders = ref({
    title: '',
    album: '',
    mb_release_id: '',
    mb_release_group_id: '',
    year: '',
    disc_number: '',
    disc_subtitle: ''
})

const artistPairs = ref<Pair[]>([])
const albumArtistPairs = ref<Pair[]>([])
// The distinct name/ID rows at load time, for change detection and ID pruning.
const artistOriginal = ref<ArtistMbidRow[]>([])
const albumArtistOriginal = ref<ArtistMbidRow[]>([])

const diff = computed(() => diffInitialValues(props.selection))
// When the selected tracks don't all share the same artist list the names are
// "mixed": we show a note and start with a blank list. Adding artists overwrites
// the whole list on every selected track; leaving it empty writes nothing.
const artistsMixed = computed(() => props.selection.length > 1 && !diff.value.artists.shared)
const albumArtistsMixed = computed(
    () => props.selection.length > 1 && !diff.value.album_artists.shared
)

function pairsFor(scope: Scope) {
    return scope === 'artist' ? artistPairs : albumArtistPairs
}

function addPair(scope: Scope) {
    pairsFor(scope).value.push({ name: '', mbid: '', mixed: false })
}
function removePair(scope: Scope, index: number) {
    pairsFor(scope).value.splice(index, 1)
}

function mbidPlaceholder(pair: Pair): string {
    return pair.mixed && !pair.mbid ? '(mixed)' : ''
}

// Picker dialog state, targeting a specific pair by index.
const picker = ref<{ open: boolean; scope: Scope; index: number }>({
    open: false,
    scope: 'artist',
    index: -1
})
const pickerPair = computed<Pair | undefined>(
    () => pairsFor(picker.value.scope).value[picker.value.index]
)

function openPicker(scope: Scope, index: number) {
    picker.value = { open: true, scope, index }
}
function onPickerSelect(mbid: string, name?: string) {
    const pair = pickerPair.value
    if (!pair) return
    pair.mbid = mbid
    if (name) pair.name = name
}

// Album search-dialog state. Picking a release fills both IDs and (when a title
// is returned) the album name; clearing wipes both IDs.
const albumPicker = ref(false)
function onAlbumPickerSelect(releaseMbid: string, releaseGroupMbid: string, title?: string) {
    values.value.mb_release_id = releaseMbid
    dirty.value.mb_release_id = true
    values.value.mb_release_group_id = releaseGroupMbid
    dirty.value.mb_release_group_id = true
    if (title) {
        values.value.album = title
        dirty.value.album = true
    }
}

// Album-cover section state. A cover lives in the selected track's own directory,
// so cover editing targets that directory (resolved from the selection) rather
// than whatever folder is highlighted in the tree — that node can be a parent of
// the album folder, since the track list is populated recursively.
const coverPicker = ref(false)
// Bumped after a cover is added/removed to cache-bust the <img> srcs (the
// endpoint sends no-cache but the URLs are otherwise unchanged).
const coverBust = ref(0)
const selectionPaths = computed(() => props.selection.map((t) => t.path))

// dirOf returns the library-relative parent directory of a track path ('' = root).
function dirOf(path: string): string {
    const i = path.lastIndexOf('/')
    return i === -1 ? '' : path.slice(0, i)
}
const selectionDirs = computed(() => new Set(props.selection.map((t) => dirOf(t.path))))
// Covers are per-album (per-directory): editing one requires the selection to sit
// in a single directory. A selection spanning albums can't target a cover, so the
// section shows a note instead (mirrors the mixed-artist handling above).
const singleAlbum = computed(() => props.selection.length > 0 && selectionDirs.value.size === 1)
const coverDir = computed<string | null>(() =>
    singleAlbum.value ? [...selectionDirs.value][0] : null
)

const applyCoverMutation = useApplyCover()
const deleteCoverMutation = useDeleteCover()

const coverTargetLabels: Record<CoverTarget, string> = {
    db: 'aether store',
    folder: 'album folder',
    embedded: 'embedded tags'
}

// Every source that currently holds a cover for the folder — each is shown with
// its own thumbnail and a remove button.
const coverSources = ref<CoverSourceEntry[]>([])
function sourceLabel(entry: CoverSourceEntry): string {
    const base = coverTargetLabels[entry.source]
    return entry.detail ? `${base} (${entry.detail})` : base
}
function sourceCoverUrl(source: CoverTarget): string | null {
    if (props.libraryId === null || coverDir.value === null) return null
    return getSourceCoverUrl(props.libraryId, coverDir.value, source, coverBust.value)
}
async function refreshCoverInfo() {
    if (props.libraryId === null || coverDir.value === null) {
        coverSources.value = []
        return
    }
    try {
        coverSources.value = await getCoverInfo(props.libraryId, coverDir.value)
    } catch {
        coverSources.value = []
    }
}
watch(() => [props.libraryId, coverDir.value, coverBust.value], refreshCoverInfo, {
    immediate: true
})

// Sources the user marked for removal: hidden from the list immediately, but the
// actual delete is staged and only persisted on Save (like every other edit).
const stagedRemovals = ref<CoverTarget[]>([])
const visibleCoverSources = computed(() =>
    coverSources.value.filter((e) => !stagedRemovals.value.includes(e.source))
)
const stagedRemovalLabels = computed(() =>
    stagedRemovals.value.map((s) => coverTargetLabels[s]).join(', ')
)
function stageRemoveCoverSource(source: CoverTarget) {
    if (!stagedRemovals.value.includes(source)) stagedRemovals.value.push(source)
}
function undoRemovals() {
    stagedRemovals.value = []
}

// A cover chosen in the picker but not yet persisted: it previews in the panel
// and is only written when the user clicks Save (like every other field edit).
const stagedCover = ref<StagedCover | null>(null)
const stagedPreview = ref<string | null>(null)

function onCoverSelected(cover: StagedCover) {
    clearStagedCover()
    stagedCover.value = cover
    stagedPreview.value = cover.file ? URL.createObjectURL(cover.file) : cover.imageUrl
}
function clearStagedCover() {
    if (stagedPreview.value?.startsWith('blob:')) URL.revokeObjectURL(stagedPreview.value)
    stagedCover.value = null
    stagedPreview.value = null
}

function reset() {
    // Changing the selection/folder discards pending (unsaved) cover changes.
    clearStagedCover()
    stagedRemovals.value = []
    if (props.selection.length === 0) return
    const d = diff.value
    values.value = {
        title: d.title.value,
        album: d.album.value,
        mb_release_id: d.mb_release_id.value,
        mb_release_group_id: d.mb_release_group_id.value,
        year: d.year.value,
        disc_number: d.disc_number.value,
        disc_subtitle: d.disc_subtitle.value,
        compilation: d.compilation.value
    }
    dirty.value = {
        title: false,
        album: false,
        mb_release_id: false,
        mb_release_group_id: false,
        year: false,
        disc_number: false,
        disc_subtitle: false,
        compilation: false
    }
    placeholders.value = {
        title: d.title.shared ? '' : '(multiple values)',
        album: d.album.shared ? '' : '(multiple values)',
        mb_release_id: d.mb_release_id.shared ? '' : '(multiple values)',
        mb_release_group_id: d.mb_release_group_id.shared ? '' : '(multiple values)',
        year: d.year.shared ? '' : '(multiple values)',
        disc_number: d.disc_number.shared ? '' : '(multiple values)',
        disc_subtitle: d.disc_subtitle.shared ? '' : '(multiple values)'
    }
    // Shared lists (single track counts) load their distinct name/ID rows for
    // editing. Mixed lists start blank: an empty original means an untouched list
    // writes nothing, while any added artist overwrites the whole list per track.
    artistOriginal.value = d.artists.shared
        ? distinctArtistMbids(props.selection, 'artists', 'mb_artist_ids')
        : []
    albumArtistOriginal.value = d.album_artists.shared
        ? distinctArtistMbids(props.selection, 'album_artists', 'mb_album_artist_ids')
        : []
    const toPair = (r: ArtistMbidRow): Pair => ({ name: r.name, mbid: r.mbid, mixed: r.mixed })
    artistPairs.value = artistOriginal.value.map(toPair)
    albumArtistPairs.value = albumArtistOriginal.value.map(toPair)
}

watch(() => props.selection, reset, { immediate: true, deep: true })

function trimmedNames(pairs: Pair[]): string[] {
    return pairs.map((p) => p.name.trim()).filter((n) => n !== '')
}
function namesChanged(pairs: Pair[], original: ArtistMbidRow[]): boolean {
    const now = trimmedNames(pairs)
    const before = original.map((r) => r.name)
    return now.length !== before.length || now.some((n, i) => n !== before[i])
}

// Full name -> ID map, sent whenever the name list is rewritten so the MB-ID tag
// is rebuilt aligned to the new names (removals/reorders realign correctly).
function completeMbidMap(pairs: Pair[]): Record<string, string> {
    const out: Record<string, string> = {}
    for (const p of pairs) {
        const n = p.name.trim()
        if (n) out[n] = p.mbid
    }
    return out
}
// Only the IDs that changed, keyed by name — used when names are untouched so
// unrelated tracks/artists keep their existing IDs.
function prunedMbidMap(pairs: Pair[], original: ArtistMbidRow[]): Record<string, string> {
    const orig = new Map<string, string | null>()
    for (const r of original) orig.set(r.name, r.mixed ? null : r.mbid)
    const out: Record<string, string> = {}
    for (const p of pairs) {
        const n = p.name.trim()
        if (!n) continue
        const o = orig.get(n)
        if (o === undefined) {
            if (p.mbid) out[n] = p.mbid
        } else if (o !== p.mbid) {
            out[n] = p.mbid
        }
    }
    return out
}

function fieldPatch(out: PatchFields, scope: Scope, pairs: Pair[], original: ArtistMbidRow[]) {
    const namesKey = scope === 'artist' ? 'artists' : 'album_artists'
    const mbidKey = scope === 'artist' ? 'artist_mbids' : 'album_artist_mbids'
    if (namesChanged(pairs, original)) {
        out[namesKey] = trimmedNames(pairs)
        const map = completeMbidMap(pairs)
        if (Object.keys(map).length > 0) out[mbidKey] = map
    } else {
        const map = prunedMbidMap(pairs, original)
        if (Object.keys(map).length > 0) out[mbidKey] = map
    }
}

const patchFields = computed<PatchFields>(() => {
    const out: PatchFields = {}
    if (dirty.value.album) out.album = values.value.album
    if (dirty.value.mb_release_id) out.mb_release_id = values.value.mb_release_id
    if (dirty.value.mb_release_group_id) out.mb_release_group_id = values.value.mb_release_group_id
    if (dirty.value.year) out.year = values.value.year
    if (dirty.value.disc_number) out.disc_number = values.value.disc_number
    if (dirty.value.disc_subtitle) out.disc_subtitle = values.value.disc_subtitle
    if (dirty.value.compilation) out.compilation = values.value.compilation
    // title is only editable in single-track mode
    if (!isMass.value && dirty.value.title) out.title = values.value.title
    fieldPatch(out, 'artist', artistPairs.value, artistOriginal.value)
    fieldPatch(out, 'album_artist', albumArtistPairs.value, albumArtistOriginal.value)
    return out
})

const canSave = computed(
    () =>
        (Object.keys(patchFields.value).length > 0 ||
            stagedCover.value !== null ||
            stagedRemovals.value.length > 0) &&
        !props.isSaving &&
        !applyCoverMutation.isPending.value &&
        !deleteCoverMutation.isPending.value
)

async function save() {
    // Persist staged cover changes (their own endpoints) first, then field edits.
    if (!(await persistStagedCover())) return
    if (!(await persistStagedRemovals())) return
    if (Object.keys(patchFields.value).length > 0) {
        emit('save', patchFields.value)
    }
}

// persistStagedCover writes a staged (added) cover. Returns false to abort the
// save when the write fails (the mutation shows its own error toast).
async function persistStagedCover(): Promise<boolean> {
    if (stagedCover.value === null || props.libraryId === null) return true
    const form = new FormData()
    form.append('library_id', String(props.libraryId))
    form.append('target', stagedCover.value.target)
    for (const p of selectionPaths.value) form.append('paths', p)
    if (stagedCover.value.file) form.append('image', stagedCover.value.file)
    else if (stagedCover.value.imageUrl) form.append('image_url', stagedCover.value.imageUrl)
    try {
        await applyCoverMutation.mutateAsync(form)
        clearStagedCover()
        coverBust.value = Date.now()
        return true
    } catch {
        return false
    }
}

// persistStagedRemovals deletes every source marked for removal. Returns false
// to abort the save when a delete fails.
async function persistStagedRemovals(): Promise<boolean> {
    if (stagedRemovals.value.length === 0 || props.libraryId === null || coverDir.value === null) {
        return true
    }
    try {
        for (const source of stagedRemovals.value) {
            await deleteCoverMutation.mutateAsync({
                libraryId: props.libraryId,
                path: coverDir.value,
                source,
                // Embedded removal applies to the selected files only.
                paths: source === 'embedded' ? selectionPaths.value : undefined
            })
        }
        stagedRemovals.value = []
        coverBust.value = Date.now()
        return true
    } catch {
        return false
    }
}
</script>

<template>
    <div v-if="selection.length === 0" class="empty">Select one or more tracks to edit.</div>
    <div v-else class="edit-panel">
        <h3>
            {{ isMass ? `Editing ${selection.length} tracks` : 'Editing 1 track' }}
        </h3>

        <div v-if="!isMass" class="field-row">
            <label>Title</label>
            <InputText
                v-model="values.title"
                @update:modelValue="dirty.title = true"
                :placeholder="placeholders.title"
            />
        </div>

        <div v-if="!isMass" class="section-spacer" />

        <div class="field-block">
            <label>Album</label>
            <div class="album-fields">
                <div class="album-name-row">
                    <InputText
                        class="album-name"
                        v-model="values.album"
                        @update:modelValue="dirty.album = true"
                        :placeholder="placeholders.album"
                    />
                    <Button
                        icon="pi pi-search"
                        text
                        size="small"
                        aria-label="Search MusicBrainz album"
                        @click="albumPicker = true"
                    />
                </div>
                <div class="pair-field">
                    <label>Release MusicBrainz ID</label>
                    <InputText
                        class="album-mbid"
                        v-model="values.mb_release_id"
                        @update:modelValue="dirty.mb_release_id = true"
                        :placeholder="placeholders.mb_release_id"
                    />
                </div>
                <div class="pair-field">
                    <label>Release-group MusicBrainz ID</label>
                    <InputText
                        class="album-mbid"
                        v-model="values.mb_release_group_id"
                        @update:modelValue="dirty.mb_release_group_id = true"
                        :placeholder="placeholders.mb_release_group_id"
                    />
                </div>
            </div>
        </div>

        <div class="section-spacer" />

        <div class="field-block">
            <label>Album cover</label>
            <div class="cover-fields">
                <template v-if="singleAlbum">
                    <Button
                        label="Change cover…"
                        icon="pi pi-images"
                        size="small"
                        outlined
                        data-test="change-cover"
                        :disabled="libraryId === null"
                        @click="coverPicker = true"
                    />

                    <!-- A cover chosen but not yet saved. -->
                    <div v-if="stagedCover" class="cover-row pending" data-test="cover-pending">
                        <img v-if="stagedPreview" :src="stagedPreview" class="cover-thumb" alt="" />
                        <span class="cover-row-note">
                            Pending — saves to {{ coverTargetLabels[stagedCover.target] }} on Save
                        </span>
                        <Button
                            label="Discard"
                            icon="pi pi-times"
                            size="small"
                            text
                            data-test="cover-discard"
                            @click="clearStagedCover"
                        />
                    </div>

                    <!-- Every source that currently holds a cover (minus staged removals). -->
                    <div
                        v-for="entry in visibleCoverSources"
                        :key="entry.source"
                        class="cover-row"
                        :data-test="`cover-source-${entry.source}`"
                    >
                        <img
                            :src="sourceCoverUrl(entry.source) ?? undefined"
                            class="cover-thumb"
                            :alt="sourceLabel(entry)"
                        />
                        <span class="cover-row-note">{{ sourceLabel(entry) }}</span>
                        <Button
                            label="Remove"
                            icon="pi pi-trash"
                            size="small"
                            text
                            severity="danger"
                            :data-test="`cover-remove-${entry.source}`"
                            @click="stageRemoveCoverSource(entry.source)"
                        />
                    </div>

                    <!-- Covers marked for removal, applied on Save. -->
                    <div
                        v-if="stagedRemovals.length > 0"
                        class="cover-row muted"
                        data-test="cover-removals"
                    >
                        <span class="cover-row-note">
                            Will remove {{ stagedRemovalLabels }} on Save
                        </span>
                        <Button
                            label="Undo"
                            icon="pi pi-undo"
                            size="small"
                            text
                            data-test="cover-removals-undo"
                            @click="undoRemovals"
                        />
                    </div>

                    <div
                        v-if="
                            !stagedCover &&
                            visibleCoverSources.length === 0 &&
                            stagedRemovals.length === 0
                        "
                        class="cover-row muted"
                    >
                        <div class="cover-placeholder"><i class="pi pi-image"></i></div>
                        <span class="cover-row-note">No cover</span>
                    </div>
                </template>
                <small v-else class="mixed-note" data-test="cover-multi-album">
                    Select tracks from a single album to manage its cover.
                </small>
            </div>
        </div>

        <div class="section-spacer" />

        <div class="field-block">
            <label>Artists</label>
            <div class="pairs">
                <small v-if="artistsMixed" class="mixed-note">
                    Selected tracks have different artists. Add artists to overwrite all of them;
                    leave empty to keep each track's own.
                </small>
                <div v-for="(pair, i) in artistPairs" :key="i" class="pair">
                    <div class="pair-fields">
                        <div class="pair-field">
                            <label>Artist</label>
                            <InputText
                                class="pair-name"
                                v-model="pair.name"
                                placeholder="Artist name"
                            />
                        </div>
                        <div class="pair-field">
                            <label>MusicBrainz ID</label>
                            <InputText
                                class="pair-mbid"
                                v-model="pair.mbid"
                                :placeholder="mbidPlaceholder(pair)"
                            />
                        </div>
                    </div>
                    <div class="pair-actions">
                        <Button
                            icon="pi pi-search"
                            text
                            size="small"
                            aria-label="Search MusicBrainz"
                            @click="openPicker('artist', i)"
                        />
                        <Button
                            icon="pi pi-times"
                            text
                            size="small"
                            severity="secondary"
                            aria-label="Remove artist"
                            @click="removePair('artist', i)"
                        />
                    </div>
                </div>
                <Button
                    icon="pi pi-plus"
                    label="Add artist"
                    text
                    size="small"
                    @click="addPair('artist')"
                />
            </div>
        </div>

        <div class="section-spacer" />

        <div class="field-block">
            <label>Album artists</label>
            <div class="pairs">
                <small v-if="albumArtistsMixed" class="mixed-note">
                    Selected tracks have different album artists. Add album artists to overwrite all
                    of them; leave empty to keep each track's own.
                </small>
                <div v-for="(pair, i) in albumArtistPairs" :key="i" class="pair">
                    <div class="pair-fields">
                        <div class="pair-field">
                            <label>Album artist</label>
                            <InputText
                                class="pair-name"
                                v-model="pair.name"
                                placeholder="Album artist name"
                            />
                        </div>
                        <div class="pair-field">
                            <label>MusicBrainz ID</label>
                            <InputText
                                class="pair-mbid"
                                v-model="pair.mbid"
                                :placeholder="mbidPlaceholder(pair)"
                            />
                        </div>
                    </div>
                    <div class="pair-actions">
                        <Button
                            icon="pi pi-search"
                            text
                            size="small"
                            aria-label="Search MusicBrainz"
                            @click="openPicker('album_artist', i)"
                        />
                        <Button
                            icon="pi pi-times"
                            text
                            size="small"
                            severity="secondary"
                            aria-label="Remove album artist"
                            @click="removePair('album_artist', i)"
                        />
                    </div>
                </div>
                <Button
                    icon="pi pi-plus"
                    label="Add album artist"
                    text
                    size="small"
                    @click="addPair('album_artist')"
                />
            </div>
        </div>

        <div class="field-row">
            <label>Year</label>
            <InputNumber
                v-model="values.year"
                @update:modelValue="dirty.year = true"
                :useGrouping="false"
                :placeholder="placeholders.year"
            />
        </div>

        <div class="field-row">
            <label>Disc number</label>
            <InputNumber
                class="field-disc-number"
                v-model="values.disc_number"
                @update:modelValue="dirty.disc_number = true"
                :useGrouping="false"
                :placeholder="placeholders.disc_number"
            />
        </div>

        <div class="field-row">
            <label>Disc subtitle</label>
            <InputText
                class="field-disc-subtitle"
                v-model="values.disc_subtitle"
                @update:modelValue="dirty.disc_subtitle = true"
                :placeholder="placeholders.disc_subtitle"
            />
        </div>

        <div class="field-row">
            <label>Compilation</label>
            <Checkbox
                v-model="values.compilation"
                @update:modelValue="dirty.compilation = true"
                :binary="true"
            />
        </div>

        <MusicBrainzArtistPicker
            v-model:visible="picker.open"
            :artistName="pickerPair?.name ?? ''"
            :currentMbid="pickerPair?.mbid ?? ''"
            @select="onPickerSelect"
        />

        <MusicBrainzAlbumPicker
            v-model:visible="albumPicker"
            :albumName="values.album"
            :currentReleaseMbid="values.mb_release_id"
            :currentReleaseGroupMbid="values.mb_release_group_id"
            @select="onAlbumPickerSelect"
        />

        <AlbumCoverPicker
            v-if="libraryId !== null"
            v-model:visible="coverPicker"
            :albumName="values.album"
            :releaseMbid="values.mb_release_id"
            :releaseGroupMbid="values.mb_release_group_id"
            :libraryId="libraryId"
            :paths="selectionPaths"
            @select="onCoverSelected"
        />

        <div class="actions">
            <Button
                label="Save"
                icon="pi pi-save"
                :disabled="!canSave"
                :loading="isSaving"
                @click="save"
            />
        </div>
    </div>
</template>

<style scoped>
.edit-panel {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 1rem;
    background: var(--app-surface);
    border: 1px solid var(--app-border);
    border-radius: 6px;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}
.cover-fields {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 0.4rem;
}
.cover-row {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.cover-row.muted .cover-row-note {
    color: var(--app-text-secondary);
}
.cover-thumb {
    width: 3rem;
    height: 3rem;
    object-fit: cover;
    border-radius: 6px;
    border: 1px solid var(--app-border);
    flex: 0 0 auto;
    background: var(--app-bg-subtle, #f3f4f6);
}
.cover-row.pending .cover-thumb {
    border: 2px solid var(--app-primary, #6366f1);
}
.cover-row-note {
    font-size: 0.8rem;
}
.cover-row.pending .cover-row-note {
    color: var(--app-primary, #6366f1);
}
.cover-placeholder {
    width: 3rem;
    height: 3rem;
    display: flex;
    align-items: center;
    justify-content: center;
    border-radius: 6px;
    border: 1px dashed var(--app-border);
    color: var(--app-text-secondary);
    font-size: 1.25rem;
    flex: 0 0 auto;
}
.empty {
    padding: 2rem;
    text-align: center;
    color: var(--app-text-secondary);
}
.field-row {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: center;
    gap: 0.5rem;
}
.field-block {
    display: grid;
    grid-template-columns: 8rem 1fr;
    align-items: start;
    gap: 0.5rem;
}
.field-row label,
.field-block label {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    padding-top: 0.35rem;
}
.field-row :deep(.p-inputtext),
.field-row :deep(.p-inputnumber) {
    width: 100%;
}
.field-row :deep(.p-inputnumber-input) {
    width: 100%;
}
.pairs {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}
.pair {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: 0.5rem;
    padding: 0.5rem 0.6rem;
    border: 1px solid var(--app-border);
    border-radius: 6px;
}
.pair-fields {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    min-width: 0;
}
.pair-field {
    display: flex;
    flex-direction: column;
    gap: 0.15rem;
    min-width: 0;
}
.pair-field label {
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    color: var(--app-text-secondary);
}
.pair-actions {
    display: flex;
    align-items: center;
    gap: 0.15rem;
}
.pair :deep(.p-inputtext) {
    width: 100%;
    font-size: 0.85rem;
}
.pair-mbid {
    font-family: var(--font-mono, monospace);
}
.album-fields {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
    min-width: 0;
}
.album-name-row {
    display: flex;
    align-items: center;
    gap: 0.25rem;
}
.album-name-row :deep(.p-inputtext) {
    width: 100%;
}
.album-mbid {
    font-family: var(--font-mono, monospace);
}
.album-fields :deep(.p-inputtext) {
    width: 100%;
    font-size: 0.85rem;
}
.mixed-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
.section-spacer {
    height: 1px;
    margin: 0.5rem 0;
    background: var(--app-border);
}
.actions {
    margin-top: 1rem;
    display: flex;
    justify-content: flex-end;
}
</style>
