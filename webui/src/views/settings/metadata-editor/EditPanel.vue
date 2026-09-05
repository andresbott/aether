<script setup lang="ts">
import { computed, ref, watch, useId } from 'vue'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import AutoComplete from 'primevue/autocomplete'
import Checkbox from 'primevue/checkbox'
import Button from 'primevue/button'
import type { Track, TrackOverlay } from '@/types/metadata'
import {
    diffInitialValues,
    distinctArtistMbids,
    type ArtistMbidRow
} from '@/composables/useMetadataEditor'
import type { EditSession } from '@/composables/useEditSession'
import type { AlbumMatchPayload, ArtistMatchPayload, ReleaseArtistCredit } from '@/types/artists'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'
import MusicBrainzAlbumPicker from '@/components/library/MusicBrainzAlbumPicker.vue'
import RawEditPanel from './RawEditPanel.vue'
import PicturesSection from './PicturesSection.vue'
import ArtistImageSection from './ArtistImageSection.vue'
import CollapsibleSection from './CollapsibleSection.vue'
import { useEditForm } from './useEditForm'

const props = defineProps<{
    selection: Track[]
    libraryId: number | null
    session: EditSession
    // The selected folder (library-relative), for the folder-scoped artist image
    // shown when no track is selected. Optional: absent behaves as no folder.
    folderPath?: string | null
    canIdentify: boolean
    // Explanation shown on the disabled Identify button when canIdentify is
    // false (e.g. fpcalc missing on the server). Empty when identify works.
    identifyUnavailableReason?: string
    isIdentifying: boolean
    isIdentifyingAlbum: boolean
}>()
const emit = defineEmits<{
    (e: 'identify', tracks: Track[]): void
    (e: 'identify-album', tracks: Track[]): void
}>()

type Scope = 'artist' | 'album_artist'

// One artist "group": an editable name and its MusicBrainz ID. `mixed` flags a
// name whose selected tracks disagree on the ID (shown blank until edited).
interface Pair {
    name: string
    mbid: string
    mixed: boolean
}

// Unique per-instance id prefix so each field label/input association is stable
// and collision-free even if two panels ever mount at once.
const uid = useId()
const fid = (name: string) => `${uid}-${name}`

// Load-bearing help/warning copy, shared between the hover tooltip and the
// aria-label so keyboard/AT users reach the same text as mouse users.
const albumGroupingHelp =
    'Songs form one album when their album name, album artist and ' +
    "Release ID all match. Upper/lower case and accents don't matter.\n\n" +
    'An empty Release ID is treated as a value too, so songs ' +
    'without one still group together — but they never join songs ' +
    'that have one. Set the Release ID on all songs of an album or ' +
    'on none; filling it for only some splits the album in two.'
const albumArtistsHelp =
    'The main artist the whole album is filed under, used for ' +
    'grouping in the library. Usually one per album — it stays the ' +
    'same even when individual tracks credit featured guests or, on ' +
    'compilations, is “Various Artists”.'

const form = useEditForm(() => props.selection, props.session)

const title = form.text('title')
const album = form.text('album')
const mbRecordingId = form.text('mb_recording_id')
const mbReleaseId = form.text('mb_release_id')
const mbReleaseGroupId = form.text('mb_release_group_id')
const discSubtitle = form.text('disc_subtitle')
const year = form.num('year')
const trackNumber = form.num('track_number')
const discNumber = form.num('disc_number')
const genres = form.genres
const compilation = form.compilation

const isMass = form.isMass

const artistPairs = ref<Pair[]>([])
const albumArtistPairs = ref<Pair[]>([])

const diff = computed(() => diffInitialValues(form.effective.value))
const originalDiff = form.originalDiff
const selectionPaths = form.paths
// When the selected tracks don't all share the same artist list the names are
// “mixed”: we show a note and start with a blank list. Adding artists overwrites
// the whole list on every selected track; leaving it empty writes nothing.
const artistsMixed = computed(() => props.selection.length > 1 && !diff.value.artists.shared)
const albumArtistsMixed = computed(
    () => props.selection.length > 1 && !diff.value.album_artists.shared
)

const dirtyFields = form.dirtyFields
const isDirty = form.isDirty

// undoPairsTooltip is the credit-list variant: lists the original names.
function undoPairsTooltip(scope: Scope): string {
    const d = originalDiff.value[creditsKey(scope)]
    if (!d.shared) return 'Revert to each track\'s own value'
    if (d.value.length === 0) return 'Revert to empty'
    return `Revert to “${d.value.join(', ')}”`
}

function pairsFor(scope: Scope) {
    return scope === 'artist' ? artistPairs : albumArtistPairs
}
function creditsKey(scope: Scope): 'artists' | 'album_artists' {
    return scope === 'artist' ? 'artists' : 'album_artists'
}

// stagePairs stages the full credit list of a scope onto every selected track.
// A pair list left empty over originally-mixed tracks stages nothing (keeps
// each track's own artists), mirroring the pre-session mass-edit semantics.
function stagePairs(scope: Scope) {
    const key = creditsKey(scope)
    const pairs = pairsFor(scope).value
    const originallyMixed =
        props.selection.length > 1 && !originalDiff.value[key].shared
    if (originallyMixed && pairs.length === 0) {
        props.session.unstageField(selectionPaths.value, key)
        return
    }
    props.session.stageField(
        selectionPaths.value,
        key,
        pairs.map((p) => ({ name: p.name, mbid: p.mbid }))
    )
}

function undoPairs(scope: Scope) {
    props.session.unstageField(selectionPaths.value, creditsKey(scope))
    resetPairs()
}

function addPair(scope: Scope) {
    pairsFor(scope).value.push({ name: '', mbid: '', mixed: false })
    stagePairs(scope)
}
function removePair(scope: Scope, index: number) {
    pairsFor(scope).value.splice(index, 1)
    stagePairs(scope)
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
function onPickerSelect(payload: ArtistMatchPayload) {
    const pair = pickerPair.value
    if (!pair) return
    if (payload.mbid !== undefined) pair.mbid = payload.mbid
    if (payload.name !== undefined) pair.name = payload.name
    stagePairs(picker.value.scope)
}

// Album search-dialog state. The picker emits only the fields the user left
// checked in its preview; clearing the match sends empty-string IDs.
const albumPicker = ref(false)
const currentAlbumArtistCredits = computed<ReleaseArtistCredit[]>(() =>
    albumArtistPairs.value.map((p) => ({ name: p.name, mbid: p.mbid }))
)
function onAlbumPickerSelect(payload: AlbumMatchPayload) {
    if (payload.album !== undefined) props.session.stageField(form.paths.value, 'album', payload.album)
    if (payload.year !== undefined) props.session.stageField(form.paths.value, 'year', payload.year)
    if (payload.mbReleaseId !== undefined)
        props.session.stageField(form.paths.value, 'mb_release_id', payload.mbReleaseId)
    if (payload.mbReleaseGroupId !== undefined)
        props.session.stageField(form.paths.value, 'mb_release_group_id', payload.mbReleaseGroupId)
    if (payload.genres !== undefined)
        props.session.stageField(form.paths.value, 'genres', [...payload.genres])
    if (payload.albumArtists !== undefined) {
        albumArtistPairs.value = payload.albumArtists.map((a) => ({ name: a.name, mbid: a.mbid, mixed: false }))
        stagePairs('album_artist')
    }
}

// resetPairs refills the artist pair editors from the effective selection.
function resetPairs() {
    const eff = form.effective.value
    const d = diff.value
    const artistRows = d.artists.shared
        ? distinctArtistMbids(eff, 'artists', 'mb_artist_ids')
        : []
    const albumArtistRows = d.album_artists.shared
        ? distinctArtistMbids(eff, 'album_artists', 'mb_album_artist_ids')
        : []
    const toPair = (r: ArtistMbidRow): Pair => ({ name: r.name, mbid: r.mbid, mixed: r.mixed })
    artistPairs.value = artistRows.map(toPair)
    albumArtistPairs.value = albumArtistRows.map(toPair)
}

// Watches the array reference, not its contents: the parent only ever replaces
// `selection` with a new array (folder commit, identify apply, cancel) — the
// Track objects are never mutated in place — so `deep` would traverse every
// selected track's fields on each change for nothing.
watch(() => props.selection, resetPairs, { immediate: true })

const identifiable = computed(() => props.selection.filter((t) => !t.error))

// The Identify button is always rendered so the feature is discoverable; it
// goes grey with an explanatory tooltip when the server lacks the dependency
// or the selection has nothing readable to fingerprint.
const identifyDisabled = computed(
    () => !props.canIdentify || identifiable.value.length === 0 || props.isIdentifying
)
const identifyTooltip = computed(() => {
    if (!props.canIdentify) {
        return (
            props.identifyUnavailableReason ||
            'Audio identification is not available on this server.'
        )
    }
    if (identifiable.value.length === 0) {
        return 'None of the selected tracks could be read, so there is nothing to fingerprint'
    }
    return 'Look up these tracks on AcoustID by acoustic fingerprint'
})

// Album identification only makes sense for a set of files, so the button
// appears from two selected tracks up — below that, per-track Identify is
// strictly better and this button would just be a worse duplicate.
const showAlbumIdentify = computed(() => props.selection.length > 1)
const albumIdentifyDisabled = computed(
    () =>
        !props.canIdentify ||
        identifiable.value.length < 2 ||
        props.isIdentifying ||
        props.isIdentifyingAlbum
)
const albumIdentifyTooltip = computed(() => {
    if (!props.canIdentify) {
        return (
            props.identifyUnavailableReason ||
            'Audio identification is not available on this server.'
        )
    }
    if (identifiable.value.length < 2) {
        return 'At least two readable tracks are needed to identify an album'
    }
    return 'Map these tracks onto a single album on MusicBrainz'
})

// Raw mode swaps the form body for the raw tag editor; the panel header (count
// + Identify + Raw buttons) stays so mode switching is always reachable.
const rawMode = ref(false)

// A save flushes staged edits (raw edits included), so once a save cycle
// finishes drop back to the form view — the raw table would otherwise keep
// showing the just-saved (now unstaged) values. isSaving goes true→false once
// per save.
watch(
    () => props.session.isSaving.value,
    (saving, wasSaving) => {
        if (wasSaving && !saving) rawMode.value = false
    }
)
</script>

<template>
    <div v-if="selection.length === 0" class="empty-state">
        <!-- Folder-scoped artist image, shown only when no track is selected and
             the folder resolves to an artist folder (the component self-hides
             otherwise). -->
        <ArtistImageSection
            :libraryId="libraryId"
            :session="session"
            :folderPath="folderPath ?? null"
        />
        <div class="empty">Select one or more tracks to edit.</div>
    </div>
    <div v-else class="edit-panel">
        <div class="panel-header">
            <h3>
                {{ isMass ? `Editing ${selection.length} tracks` : 'Editing 1 track' }}
            </h3>
            <div class="panel-header-actions">
                <span
                    v-if="!rawMode && showAlbumIdentify"
                    v-tooltip.left="albumIdentifyTooltip"
                    class="identify-wrap"
                >
                    <Button
                        :label="`Identify album (${identifiable.length})`"
                        icon="pi pi-compact-disc"
                        size="small"
                        outlined
                        data-test="identify-album-button"
                        :disabled="albumIdentifyDisabled"
                        :loading="isIdentifyingAlbum"
                        @click="emit('identify-album', identifiable)"
                    />
                </span>
                <span v-if="!rawMode" v-tooltip.left="identifyTooltip" class="identify-wrap">
                    <Button
                        :label="isMass ? `Identify ${identifiable.length} tracks` : 'Identify'"
                        icon="pi pi-wave-pulse"
                        size="small"
                        outlined
                        data-test="identify-button"
                        :disabled="identifyDisabled"
                        :loading="isIdentifying"
                        @click="emit('identify', identifiable)"
                    />
                </span>
                <Button
                    label="Raw"
                    icon="pi pi-code"
                    size="small"
                    :outlined="!rawMode"
                    data-test="raw-toggle"
                    v-tooltip.left="'Edit all raw tags, including fields not shown in the form'"
                    @click="rawMode = !rawMode"
                />
            </div>
        </div>

        <RawEditPanel
            v-if="rawMode"
            :selection="selection"
            :libraryId="libraryId"
            :session="session"
        />

        <template v-else>
        <CollapsibleSection title="Song" data-test="song-block">
            <div class="field-row" :class="{ 'field-dirty': form.isDirty('title'), disabled: isMass }">
                <label :for="fid('title')">Title</label>
                <InputText
                    :id="fid('title')"
                    class="field-title"
                    v-model="title"
                    :placeholder="isMass ? '' : form.placeholder('title').value"
                    :disabled="isMass"
                />
                <Button
                    v-if="form.isDirty('title')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset title"
                    data-test="undo-title"
                    v-tooltip.left="form.undoTooltip('title')"
                    @click="form.undo('title')"
                />
            </div>

            <div
                class="field-row"
                :class="{ 'field-dirty': form.isDirty('mb_recording_id'), disabled: isMass }"
            >
                <label :for="fid('mb_recording_id')">Recording ID</label>
                <InputText
                    :id="fid('mb_recording_id')"
                    class="field-mbid"
                    v-model="mbRecordingId"
                    :placeholder="isMass ? '' : form.placeholder('mb_recording_id').value"
                    :disabled="isMass"
                />
                <Button
                    v-if="form.isDirty('mb_recording_id')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset recording ID"
                    data-test="undo-mb_recording_id"
                    v-tooltip.left="form.undoTooltip('mb_recording_id')"
                    @click="form.undo('mb_recording_id')"
                />
            </div>

            <div
                class="field-row"
                :class="{ 'field-dirty': form.isDirty('track_number'), disabled: isMass }"
            >
                <label :for="fid('track_number')">Track number</label>
                <InputNumber
                    :inputId="fid('track_number')"
                    class="field-track-number"
                    v-model="trackNumber"
                    :useGrouping="false"
                    :placeholder="isMass ? '' : form.placeholder('track_number').value"
                    :disabled="isMass"
                />
                <Button
                    v-if="form.isDirty('track_number')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset track number"
                    data-test="undo-track_number"
                    v-tooltip.left="form.undoTooltip('track_number')"
                    @click="form.undo('track_number')"
                />
            </div>

            <div
                class="field-block"
                :class="{ 'section-dirty': form.isDirty('genres') }"
                data-test="genres-block"
            >
                <label :for="fid('genres')">
                    Genres
                    <Button
                        v-if="form.isDirty('genres')"
                        icon="pi pi-undo"
                        text
                        size="small"
                        aria-label="Reset genres"
                        data-test="undo-genres"
                        v-tooltip.left="form.undoGenresTooltip()"
                        @click="form.undo('genres')"
                    />
                </label>
                <div class="genres-field">
                    <small v-if="form.genresMixed.value" class="mixed-note" data-test="genres-mixed">
                        Selected tracks have different genres. Add genres to overwrite all of
                        them; leave empty to keep each track's own.
                    </small>
                    <AutoComplete
                        :inputId="fid('genres')"
                        v-model="genres"
                        multiple
                        :typeahead="false"
                        placeholder="Add genre and press Enter"
                        data-test="genres-input"
                    />
                </div>
            </div>
        </CollapsibleSection>

        <CollapsibleSection
            title="Artists"
            :dirty="isDirty('artists')"
            :help="
                'The performers credited on each track. Can differ per track ' +
                'and include featured guests (e.g. a duet or a remix).'
            "
            data-test="artists-block"
        >
            <template #actions>
                <Button
                    v-if="isDirty('artists')"
                    icon="pi pi-undo"
                    label="Reset"
                    text
                    size="small"
                    aria-label="Reset artists"
                    data-test="undo-artists"
                    v-tooltip.left="undoPairsTooltip('artist')"
                    @click="undoPairs('artist')"
                />
            </template>

            <div class="pairs">
                <small v-if="artistsMixed" class="mixed-note">
                    Selected tracks have different artists. Add artists to overwrite all of them;
                    leave empty to keep each track's own.
                </small>
                <div v-for="(pair, i) in artistPairs" :key="i" class="pair">
                    <div class="pair-fields">
                        <div class="pair-field">
                            <label :for="fid(`artist-name-${i}`)">Artist</label>
                            <InputText
                                :id="fid(`artist-name-${i}`)"
                                class="pair-name"
                                v-model="pair.name"
                                placeholder="Artist name"
                                @update:modelValue="stagePairs('artist')"
                            />
                        </div>
                        <div class="pair-field">
                            <label :for="fid(`artist-mbid-${i}`)">MusicBrainz ID</label>
                            <InputText
                                :id="fid(`artist-mbid-${i}`)"
                                class="pair-mbid"
                                v-model="pair.mbid"
                                :placeholder="mbidPlaceholder(pair)"
                                @update:modelValue="stagePairs('artist')"
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
        </CollapsibleSection>

        <CollapsibleSection title="Album" data-test="album-block">
            <template #actions>
                <Button
                    icon="pi pi-search"
                    label="Search MusicBrainz"
                    text
                    size="small"
                    aria-label="Search MusicBrainz album"
                    @click="albumPicker = true"
                />
            </template>

            <div class="field-row" :class="{ 'field-dirty': form.isDirty('album') }">
                <label :for="fid('album')">
                    Name
                    <i
                        class="pi pi-exclamation-circle field-warn"
                        tabindex="0"
                        role="note"
                        :aria-label="albumGroupingHelp"
                        v-tooltip.right="{
                            value: albumGroupingHelp,
                            class: 'wide-tooltip'
                        }"
                        data-test="album-grouping-help"
                    ></i>
                </label>
                <InputText
                    :id="fid('album')"
                    class="album-name"
                    v-model="album"
                    :placeholder="form.placeholder('album').value"
                />
                <Button
                    v-if="form.isDirty('album')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset album name"
                    data-test="undo-album"
                    v-tooltip.left="form.undoTooltip('album')"
                    @click="form.undo('album')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': form.isDirty('mb_release_id') }">
                <label :for="fid('mb_release_id')">Release ID</label>
                <InputText
                    :id="fid('mb_release_id')"
                    class="album-mbid"
                    v-model="mbReleaseId"
                    :placeholder="form.placeholder('mb_release_id').value"
                />
                <Button
                    v-if="form.isDirty('mb_release_id')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset release ID"
                    data-test="undo-mb_release_id"
                    v-tooltip.left="form.undoTooltip('mb_release_id')"
                    @click="form.undo('mb_release_id')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': form.isDirty('mb_release_group_id') }">
                <label :for="fid('mb_release_group_id')">Release-group ID</label>
                <InputText
                    :id="fid('mb_release_group_id')"
                    class="album-mbid"
                    v-model="mbReleaseGroupId"
                    :placeholder="form.placeholder('mb_release_group_id').value"
                />
                <Button
                    v-if="form.isDirty('mb_release_group_id')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset release-group ID"
                    data-test="undo-mb_release_group_id"
                    v-tooltip.left="form.undoTooltip('mb_release_group_id')"
                    @click="form.undo('mb_release_group_id')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': form.isDirty('year') }">
                <label :for="fid('year')">Year</label>
                <InputNumber
                    :inputId="fid('year')"
                    class="field-year"
                    v-model="year"
                    :useGrouping="false"
                    :placeholder="form.placeholder('year').value"
                />
                <Button
                    v-if="form.isDirty('year')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset year"
                    data-test="undo-year"
                    v-tooltip.left="form.undoTooltip('year')"
                    @click="form.undo('year')"
                />
            </div>

            <div
                class="field-block"
                :class="{ 'section-dirty': isDirty('album_artists') }"
            >
                <label>
                    Album artists
                    <i
                        class="pi pi-question-circle field-help"
                        tabindex="0"
                        role="note"
                        :aria-label="albumArtistsHelp"
                        v-tooltip.right="{
                            value: albumArtistsHelp,
                            class: 'wide-tooltip'
                        }"
                        data-test="album-artists-help"
                    ></i>
                    <Button
                        v-if="isDirty('album_artists')"
                        icon="pi pi-undo"
                        text
                        size="small"
                        aria-label="Reset album artists"
                        data-test="undo-album-artists"
                        v-tooltip.left="undoPairsTooltip('album_artist')"
                        @click="undoPairs('album_artist')"
                    />
                </label>
                <div class="pairs">
                    <small v-if="albumArtistsMixed" class="mixed-note">
                        Selected tracks have different album artists. Add album artists to overwrite
                        all of them; leave empty to keep each track's own.
                    </small>
                    <div v-for="(pair, i) in albumArtistPairs" :key="i" class="pair">
                        <div class="pair-fields">
                            <div class="pair-field">
                                <label :for="fid(`album-artist-name-${i}`)">Album artist</label>
                                <InputText
                                    :id="fid(`album-artist-name-${i}`)"
                                    class="pair-name"
                                    v-model="pair.name"
                                    placeholder="Album artist name"
                                    @update:modelValue="stagePairs('album_artist')"
                                />
                            </div>
                            <div class="pair-field">
                                <label :for="fid(`album-artist-mbid-${i}`)">MusicBrainz ID</label>
                                <InputText
                                    :id="fid(`album-artist-mbid-${i}`)"
                                    class="pair-mbid"
                                    v-model="pair.mbid"
                                    :placeholder="mbidPlaceholder(pair)"
                                    @update:modelValue="stagePairs('album_artist')"
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

            <div class="field-row" :class="{ 'field-dirty': form.isDirty('compilation') }">
                <label :for="fid('compilation')">Compilation</label>
                <div class="compilation-field">
                    <Checkbox
                        :inputId="fid('compilation')"
                        v-model="compilation"
                        :binary="true"
                        :indeterminate="form.compilationMixed.value"
                        data-test="compilation-input"
                    />
                    <small
                        v-if="form.compilationMixed.value"
                        class="mixed-note"
                        data-test="compilation-mixed"
                    >
                        (multiple values)
                    </small>
                </div>
                <Button
                    v-if="form.isDirty('compilation')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset compilation"
                    data-test="undo-compilation"
                    v-tooltip.left="form.undoTooltip('compilation')"
                    @click="form.undo('compilation')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': form.isDirty('disc_number') }">
                <label :for="fid('disc_number')">Disc number</label>
                <InputNumber
                    :inputId="fid('disc_number')"
                    class="field-disc-number"
                    v-model="discNumber"
                    :useGrouping="false"
                    :placeholder="form.placeholder('disc_number').value"
                />
                <Button
                    v-if="form.isDirty('disc_number')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset disc number"
                    data-test="undo-disc_number"
                    v-tooltip.left="form.undoTooltip('disc_number')"
                    @click="form.undo('disc_number')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': form.isDirty('disc_subtitle') }">
                <label :for="fid('disc_subtitle')">Disc subtitle</label>
                <InputText
                    :id="fid('disc_subtitle')"
                    class="field-disc-subtitle"
                    v-model="discSubtitle"
                    :placeholder="form.placeholder('disc_subtitle').value"
                />
                <Button
                    v-if="form.isDirty('disc_subtitle')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset disc subtitle"
                    data-test="undo-disc_subtitle"
                    v-tooltip.left="form.undoTooltip('disc_subtitle')"
                    @click="form.undo('disc_subtitle')"
                />
            </div>
        </CollapsibleSection>

        <PicturesSection
            :selection="selection"
            :libraryId="libraryId"
            :session="session"
            :releaseMbid="mbReleaseId"
            :releaseGroupMbid="mbReleaseGroupId"
            :albumName="album"
        />

        </template>

        <MusicBrainzArtistPicker
            v-model:visible="picker.open"
            :artistName="pickerPair?.name ?? ''"
            :currentMbid="pickerPair?.mbid ?? ''"
            @select="onPickerSelect"
        />

        <MusicBrainzAlbumPicker
            v-model:visible="albumPicker"
            :albumName="album"
            :currentReleaseMbid="mbReleaseId"
            :currentReleaseGroupMbid="mbReleaseGroupId"
            :currentYear="year ?? 0"
            :currentAlbumArtists="currentAlbumArtistCredits"
            :currentGenres="genres"
            @select="onAlbumPickerSelect"
        />
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
.panel-header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 0.5rem;
}
.panel-header h3 {
    margin: 0;
}
.panel-header-actions {
    display: flex;
    align-items: center;
    gap: 0.35rem;
}
/* A disabled PrimeVue Button swallows pointer events, so the tooltip lives on
   this wrapper instead — it must not alter the flex layout. */
.identify-wrap {
    display: inline-flex;
}
.empty-state {
    flex: 1;
    min-height: 0;
    overflow-y: auto;
    padding: 1rem;
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
}
.empty {
    padding: 2rem;
    text-align: center;
    color: var(--app-text-secondary);
}
.field-row {
    display: grid;
    grid-template-columns: 8rem 1fr auto;
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
/* Staged (unsaved) fields: amber label, border and background until saved —
   a color used nowhere else in the editor so pending edits pop. */
.field-row.field-dirty > label {
    color: var(--app-staged);
    font-weight: 600;
}
.field-row.field-dirty :deep(.p-inputtext),
.field-row.field-dirty :deep(.p-inputnumber-input) {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.field-row.field-dirty :deep(.p-checkbox .p-checkbox-box) {
    border-color: var(--app-staged);
}
.field-block.section-dirty > label {
    color: var(--app-staged);
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
.section-dirty .pair {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
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
.pair-mbid,
.field-mbid {
    font-family: var(--font-mono, monospace);
}
.field-row.disabled label {
    color: var(--app-text-secondary);
    opacity: 0.6;
}
.compilation-field {
    display: flex;
    align-items: center;
    gap: 0.5rem;
}
.genres-field {
    display: flex;
    flex-direction: column;
    gap: 0.4rem;
}
.genres-field :deep(.p-autocomplete) {
    width: 100%;
}
.genres-field :deep(.p-autocomplete-input-multiple) {
    width: 100%;
}
.field-block.section-dirty .genres-field :deep(.p-autocomplete-input-multiple) {
    border-color: var(--app-staged);
    background-color: var(--app-staged-soft);
}
.album-mbid {
    font-family: var(--font-mono, monospace);
    font-size: 0.85rem;
}
.mixed-note {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
.field-help {
    font-size: 0.8rem;
    color: var(--app-text-secondary);
    cursor: help;
    vertical-align: middle;
    margin-left: 0.15rem;
}
/* Same affordance as .field-help, but flagged rather than merely explanatory:
   getting this field wrong silently splits an album in two in the library. */
.field-warn {
    font-size: 0.8rem;
    color: var(--app-staged);
    cursor: help;
    vertical-align: middle;
    margin-left: 0.15rem;
}
</style>
