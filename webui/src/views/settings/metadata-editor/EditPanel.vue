<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import InputText from 'primevue/inputtext'
import InputNumber from 'primevue/inputnumber'
import AutoComplete from 'primevue/autocomplete'
import Checkbox from 'primevue/checkbox'
import Button from 'primevue/button'
import type { Track, TrackOverlay } from '@/types/metadata'
import {
    diffInitialValues,
    distinctArtistMbids,
    type ArtistMbidRow,
    type FieldDiff
} from '@/composables/useMetadataEditor'
import type { EditSession } from '@/composables/useEditSession'
import type { AlbumMatchPayload, ArtistMatchPayload, ReleaseArtistCredit } from '@/types/artists'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'
import MusicBrainzAlbumPicker from '@/components/library/MusicBrainzAlbumPicker.vue'
import RawEditPanel from './RawEditPanel.vue'
import PicturesSection from './PicturesSection.vue'
import CollapsibleSection from './CollapsibleSection.vue'

const props = defineProps<{
    selection: Track[]
    libraryId: number | null
    session: EditSession
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

const isMass = computed(() => props.selection.length > 1)
const selectionPaths = computed(() => props.selection.map((t) => t.path))

// The editor displays and diffs EFFECTIVE values: original tags plus any
// staged (unsaved) session edits. Editing stages onto the session; nothing is
// persisted until the view-level Save.
const effectiveSelection = computed(() => props.selection.map((t) => props.session.effective(t)))

type TextKey =
    | 'title'
    | 'album'
    | 'mb_recording_id'
    | 'mb_release_id'
    | 'mb_release_group_id'
    | 'disc_subtitle'
type NumKey = 'year' | 'track_number' | 'disc_number'
type ScalarKey = TextKey | NumKey | 'compilation'

// Numeric buffers are nullable: InputNumber renders null as an empty box (0
// would render "0" and hide the "(multiple values)" placeholder), so null
// means "empty or mixed". Writes coerce it back to 0, which clears the tag.
type ScalarValues = {
    title: string
    album: string
    mb_recording_id: string
    mb_release_id: string
    mb_release_group_id: string
    year: number | null
    track_number: number | null
    disc_number: number | null
    disc_subtitle: string
    compilation: boolean
}

const values = ref<ScalarValues>({
    title: '',
    album: '',
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: '',
    year: null,
    track_number: null,
    disc_number: null,
    disc_subtitle: '',
    compilation: false
})
const placeholders = ref({
    title: '',
    album: '',
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: '',
    year: '',
    track_number: '',
    disc_number: '',
    disc_subtitle: ''
})

const artistPairs = ref<Pair[]>([])
const albumArtistPairs = ref<Pair[]>([])
// The genre chip list edit buffer, staged as a whole like the artist pairs.
const genresList = ref<string[]>([])

const diff = computed(() => diffInitialValues(effectiveSelection.value))
// The overwrite-all-vs-leave-alone rule for artist lists keys off the ORIGINAL
// tags: staged edits must not turn "leave each track's own artists alone" into
// "clear artists everywhere" when the pair list goes back to empty.
const originalDiff = computed(() => diffInitialValues(props.selection))
// When the selected tracks don't all share the same artist list the names are
// "mixed": we show a note and start with a blank list. Adding artists overwrites
// the whole list on every selected track; leaving it empty writes nothing.
const artistsMixed = computed(() => props.selection.length > 1 && !diff.value.artists.shared)
const albumArtistsMixed = computed(
    () => props.selection.length > 1 && !diff.value.album_artists.shared
)
const genresMixed = computed(() => props.selection.length > 1 && !diff.value.genres.shared)
// A checkbox has no placeholder to show "(multiple values)" in, so a mixed
// compilation flag renders in the indeterminate (dash) state instead. Clicking
// it commits an explicit true/false onto every selected track.
const compilationMixed = computed(
    () => props.selection.length > 1 && !diff.value.compilation.shared
)

// fieldDirty reports whether any selected track has this field staged; drives
// the accent coloring and the per-field undo button.
function fieldDirty(key: keyof TrackOverlay): boolean {
    return props.session.isFieldStaged(selectionPaths.value, key)
}

// stageScalar pushes the edit buffer's current value for one field onto the
// session (staged per selected track, normalized away when equal to original).
// A cleared numeric buffer (null) stages 0, the value that clears the tag.
function stageScalar(key: ScalarKey) {
    if (key === 'title' || key === 'mb_recording_id' || key === 'track_number') {
        // Per-recording fields are only editable in single-track mode.
        if (isMass.value) return
    }
    const value = values.value[key]
    props.session.stageField(selectionPaths.value, key, value === null ? 0 : value)
}

// stageNumber writes InputNumber's emitted value into the buffer and stages it.
// The buffer keeps null (empty box) while the patch gets 0.
function stageNumber(key: NumKey, v: number | null) {
    values.value[key] = v
    stageScalar(key)
}

// undoTooltip describes what the per-field undo button restores: the shared
// original value, or a note when the selected tracks' originals differ.
function undoTooltip(key: ScalarKey): string {
    const d = originalDiff.value[key]
    if (!d.shared) return 'Revert to each track’s own value'
    const v = d.value
    if (typeof v === 'boolean') return `Revert to ${v ? 'yes' : 'no'}`
    if (v === '' || v === 0) return 'Revert to empty'
    return `Revert to “${v}”`
}

// undoPairsTooltip is the credit-list variant: lists the original names.
function undoPairsTooltip(scope: Scope): string {
    const d = originalDiff.value[creditsKey(scope)]
    if (!d.shared) return 'Revert to each track’s own value'
    if (d.value.length === 0) return 'Revert to empty'
    return `Revert to “${d.value.join(', ')}”`
}

// undoField reverts one field to the original values and refreshes the buffer.
function undoField(key: ScalarKey) {
    props.session.unstageField(selectionPaths.value, key)
    const d = diff.value
    if (key === 'compilation') {
        values.value.compilation = d.compilation.value
    } else if (key === 'year' || key === 'track_number' || key === 'disc_number') {
        values.value[key] = numBuffer(d[key])
    } else {
        values.value[key] = d[key].value
    }
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

// stageGenres stages the full genre list onto every selected track, mirroring
// stagePairs: a list left empty over originally-mixed tracks stages nothing
// (keeps each track's own genres).
function stageGenres() {
    const originallyMixed = props.selection.length > 1 && !originalDiff.value.genres.shared
    if (originallyMixed && genresList.value.length === 0) {
        props.session.unstageField(selectionPaths.value, 'genres')
        return
    }
    props.session.stageField(selectionPaths.value, 'genres', [...genresList.value])
}

function undoGenres() {
    props.session.unstageField(selectionPaths.value, 'genres')
    genresList.value = diff.value.genres.shared ? [...diff.value.genres.value] : []
}

// undoGenresTooltip lists the original shared genres, like undoPairsTooltip.
function undoGenresTooltip(): string {
    const d = originalDiff.value.genres
    if (!d.shared) return 'Revert to each track’s own value'
    if (d.value.length === 0) return 'Revert to empty'
    return `Revert to “${d.value.join(', ')}”`
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
    if (payload.album !== undefined && payload.album !== values.value.album) {
        values.value.album = payload.album
        stageScalar('album')
    }
    if (payload.year !== undefined && payload.year !== (values.value.year ?? 0)) {
        values.value.year = payload.year
        stageScalar('year')
    }
    if (payload.mbReleaseId !== undefined) {
        values.value.mb_release_id = payload.mbReleaseId
        stageScalar('mb_release_id')
    }
    if (payload.mbReleaseGroupId !== undefined) {
        values.value.mb_release_group_id = payload.mbReleaseGroupId
        stageScalar('mb_release_group_id')
    }
    if (payload.albumArtists !== undefined) {
        albumArtistPairs.value = payload.albumArtists.map((a) => ({
            name: a.name,
            mbid: a.mbid,
            mixed: false
        }))
        stagePairs('album_artist')
    }
    if (payload.genres !== undefined) {
        genresList.value = [...payload.genres]
        stageGenres()
    }
}

// resetPairs refills the artist pair editors from the effective selection.
function resetPairs() {
    const eff = effectiveSelection.value
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
    genresList.value = d.genres.shared ? [...d.genres.value] : []
}

// numBuffer maps a numeric field diff onto the nullable buffer: mixed tracks
// and an unset tag (0) both mean "show an empty box", so the placeholder is
// visible in either case.
function numBuffer(d: FieldDiff<number>): number | null {
    if (!d.shared) return null
    return d.value === 0 ? null : d.value
}

// reset refills the edit buffers when the selection changes. Purely a display
// refresh: staged edits live in the session and are not discarded here.
function reset() {
    if (props.selection.length === 0) return
    const d = diff.value
    values.value = {
        title: d.title.value,
        album: d.album.value,
        mb_recording_id: d.mb_recording_id.value,
        mb_release_id: d.mb_release_id.value,
        mb_release_group_id: d.mb_release_group_id.value,
        year: numBuffer(d.year),
        track_number: numBuffer(d.track_number),
        disc_number: numBuffer(d.disc_number),
        disc_subtitle: d.disc_subtitle.value,
        compilation: d.compilation.value
    }
    placeholders.value = {
        title: d.title.shared ? '' : '(multiple values)',
        album: d.album.shared ? '' : '(multiple values)',
        mb_recording_id: d.mb_recording_id.shared ? '' : '(multiple values)',
        mb_release_id: d.mb_release_id.shared ? '' : '(multiple values)',
        mb_release_group_id: d.mb_release_group_id.shared ? '' : '(multiple values)',
        year: d.year.shared ? '' : '(multiple values)',
        track_number: d.track_number.shared ? '' : '(multiple values)',
        disc_number: d.disc_number.shared ? '' : '(multiple values)',
        disc_subtitle: d.disc_subtitle.shared ? '' : '(multiple values)'
    }
    resetPairs()
}

watch(() => props.selection, reset, { immediate: true, deep: true })

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
</script>

<template>
    <div v-if="selection.length === 0" class="empty">Select one or more tracks to edit.</div>
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
            <div class="field-row" :class="{ 'field-dirty': fieldDirty('title'), disabled: isMass }">
                <label>Title</label>
                <InputText
                    class="field-title"
                    v-model="values.title"
                    @update:modelValue="stageScalar('title')"
                    :placeholder="isMass ? '' : placeholders.title"
                    :disabled="isMass"
                />
                <Button
                    v-if="fieldDirty('title')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset title"
                    data-test="undo-title"
                    v-tooltip.left="undoTooltip('title')"
                    @click="undoField('title')"
                />
            </div>

            <div
                class="field-row"
                :class="{ 'field-dirty': fieldDirty('mb_recording_id'), disabled: isMass }"
            >
                <label>Recording ID</label>
                <InputText
                    class="field-mbid"
                    v-model="values.mb_recording_id"
                    @update:modelValue="stageScalar('mb_recording_id')"
                    :placeholder="isMass ? '' : placeholders.mb_recording_id"
                    :disabled="isMass"
                />
                <Button
                    v-if="fieldDirty('mb_recording_id')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset recording ID"
                    data-test="undo-mb_recording_id"
                    v-tooltip.left="undoTooltip('mb_recording_id')"
                    @click="undoField('mb_recording_id')"
                />
            </div>

            <div
                class="field-row"
                :class="{ 'field-dirty': fieldDirty('track_number'), disabled: isMass }"
            >
                <label>Track number</label>
                <InputNumber
                    class="field-track-number"
                    v-model="values.track_number"
                    @update:modelValue="(v) => stageNumber('track_number', v)"
                    :useGrouping="false"
                    :placeholder="isMass ? '' : placeholders.track_number"
                    :disabled="isMass"
                />
                <Button
                    v-if="fieldDirty('track_number')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset track number"
                    data-test="undo-track_number"
                    v-tooltip.left="undoTooltip('track_number')"
                    @click="undoField('track_number')"
                />
            </div>

            <div
                class="field-block"
                :class="{ 'section-dirty': fieldDirty('genres') }"
                data-test="genres-block"
            >
                <label>
                    Genres
                    <Button
                        v-if="fieldDirty('genres')"
                        icon="pi pi-undo"
                        text
                        size="small"
                        aria-label="Reset genres"
                        data-test="undo-genres"
                        v-tooltip.left="undoGenresTooltip()"
                        @click="undoGenres"
                    />
                </label>
                <div class="genres-field">
                    <small v-if="genresMixed" class="mixed-note" data-test="genres-mixed">
                        Selected tracks have different genres. Add genres to overwrite all of
                        them; leave empty to keep each track's own.
                    </small>
                    <AutoComplete
                        v-model="genresList"
                        multiple
                        :typeahead="false"
                        placeholder="Add genre and press Enter"
                        data-test="genres-input"
                        @update:modelValue="stageGenres"
                    />
                </div>
            </div>
        </CollapsibleSection>

        <CollapsibleSection
            title="Artists"
            :dirty="fieldDirty('artists')"
            :help="
                'The performers credited on each track. Can differ per track ' +
                'and include featured guests (e.g. a duet or a remix).'
            "
            data-test="artists-block"
        >
            <template #actions>
                <Button
                    v-if="fieldDirty('artists')"
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
                            <label>Artist</label>
                            <InputText
                                class="pair-name"
                                v-model="pair.name"
                                placeholder="Artist name"
                                @update:modelValue="stagePairs('artist')"
                            />
                        </div>
                        <div class="pair-field">
                            <label>MusicBrainz ID</label>
                            <InputText
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

            <div class="field-row" :class="{ 'field-dirty': fieldDirty('album') }">
                <label>Name</label>
                <InputText
                    class="album-name"
                    v-model="values.album"
                    @update:modelValue="stageScalar('album')"
                    :placeholder="placeholders.album"
                />
                <Button
                    v-if="fieldDirty('album')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset album name"
                    data-test="undo-album"
                    v-tooltip.left="undoTooltip('album')"
                    @click="undoField('album')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': fieldDirty('mb_release_id') }">
                <label>
                    Release ID
                    <i
                        class="pi pi-exclamation-circle field-warn"
                        v-tooltip.right="
                            'Songs end up in the same album when three things match: the album ' +
                            'name, the album artist and this Release ID (capitalisation and ' +
                            'accents are ignored). Leaving it empty is fine — songs with no ' +
                            'Release ID are grouped by album name and album artist alone. But ' +
                            'an empty ID never matches a filled-in one, so if only some songs ' +
                            'of an album carry it, that album shows up twice in the library. ' +
                            'Set it on every song of the album, or on none of them.'
                        "
                        data-test="release-id-help"
                    ></i>
                </label>
                <InputText
                    class="album-mbid"
                    v-model="values.mb_release_id"
                    @update:modelValue="stageScalar('mb_release_id')"
                    :placeholder="placeholders.mb_release_id"
                />
                <Button
                    v-if="fieldDirty('mb_release_id')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset release ID"
                    data-test="undo-mb_release_id"
                    v-tooltip.left="undoTooltip('mb_release_id')"
                    @click="undoField('mb_release_id')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': fieldDirty('mb_release_group_id') }">
                <label>Release-group ID</label>
                <InputText
                    class="album-mbid"
                    v-model="values.mb_release_group_id"
                    @update:modelValue="stageScalar('mb_release_group_id')"
                    :placeholder="placeholders.mb_release_group_id"
                />
                <Button
                    v-if="fieldDirty('mb_release_group_id')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset release-group ID"
                    data-test="undo-mb_release_group_id"
                    v-tooltip.left="undoTooltip('mb_release_group_id')"
                    @click="undoField('mb_release_group_id')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': fieldDirty('year') }">
                <label>Year</label>
                <InputNumber
                    class="field-year"
                    v-model="values.year"
                    @update:modelValue="(v) => stageNumber('year', v)"
                    :useGrouping="false"
                    :placeholder="placeholders.year"
                />
                <Button
                    v-if="fieldDirty('year')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset year"
                    data-test="undo-year"
                    v-tooltip.left="undoTooltip('year')"
                    @click="undoField('year')"
                />
            </div>

            <div
                class="field-block"
                :class="{ 'section-dirty': fieldDirty('album_artists') }"
            >
                <label>
                    Album artists
                    <i
                        class="pi pi-question-circle field-help"
                        v-tooltip.right="
                            'The main artist the whole album is filed under, used for grouping ' +
                            'in the library. Usually one per album — it stays the same even when ' +
                            'individual tracks credit featured guests or, on compilations, ' +
                            'is “Various Artists”.'
                        "
                        data-test="album-artists-help"
                    ></i>
                    <Button
                        v-if="fieldDirty('album_artists')"
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
                                <label>Album artist</label>
                                <InputText
                                    class="pair-name"
                                    v-model="pair.name"
                                    placeholder="Album artist name"
                                    @update:modelValue="stagePairs('album_artist')"
                                />
                            </div>
                            <div class="pair-field">
                                <label>MusicBrainz ID</label>
                                <InputText
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

            <div class="field-row" :class="{ 'field-dirty': fieldDirty('compilation') }">
                <label>Compilation</label>
                <div class="compilation-field">
                    <Checkbox
                        v-model="values.compilation"
                        @update:modelValue="stageScalar('compilation')"
                        :binary="true"
                        :indeterminate="compilationMixed"
                        data-test="compilation-input"
                    />
                    <small
                        v-if="compilationMixed"
                        class="mixed-note"
                        data-test="compilation-mixed"
                    >
                        (multiple values)
                    </small>
                </div>
                <Button
                    v-if="fieldDirty('compilation')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset compilation"
                    data-test="undo-compilation"
                    v-tooltip.left="undoTooltip('compilation')"
                    @click="undoField('compilation')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': fieldDirty('disc_number') }">
                <label>Disc number</label>
                <InputNumber
                    class="field-disc-number"
                    v-model="values.disc_number"
                    @update:modelValue="(v) => stageNumber('disc_number', v)"
                    :useGrouping="false"
                    :placeholder="placeholders.disc_number"
                />
                <Button
                    v-if="fieldDirty('disc_number')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset disc number"
                    data-test="undo-disc_number"
                    v-tooltip.left="undoTooltip('disc_number')"
                    @click="undoField('disc_number')"
                />
            </div>

            <div class="field-row" :class="{ 'field-dirty': fieldDirty('disc_subtitle') }">
                <label>Disc subtitle</label>
                <InputText
                    class="field-disc-subtitle"
                    v-model="values.disc_subtitle"
                    @update:modelValue="stageScalar('disc_subtitle')"
                    :placeholder="placeholders.disc_subtitle"
                />
                <Button
                    v-if="fieldDirty('disc_subtitle')"
                    icon="pi pi-undo"
                    text
                    size="small"
                    aria-label="Reset disc subtitle"
                    data-test="undo-disc_subtitle"
                    v-tooltip.left="undoTooltip('disc_subtitle')"
                    @click="undoField('disc_subtitle')"
                />
            </div>
        </CollapsibleSection>

        <PicturesSection
            :selection="selection"
            :libraryId="libraryId"
            :session="session"
            :releaseMbid="values.mb_release_id"
            :releaseGroupMbid="values.mb_release_group_id"
            :albumName="values.album"
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
            :albumName="values.album"
            :currentReleaseMbid="values.mb_release_id"
            :currentReleaseGroupMbid="values.mb_release_group_id"
            :currentYear="values.year ?? 0"
            :currentAlbumArtists="currentAlbumArtistCredits"
            :currentGenres="genresList"
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
