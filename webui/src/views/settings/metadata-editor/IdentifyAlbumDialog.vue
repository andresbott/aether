<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Select from 'primevue/select'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import type {
    AlbumAssignment,
    AlbumIdentifyPick,
    AlbumOption,
    Track
} from '@/types/metadata'
import IdentifyFieldSelect from './IdentifyFieldSelect.vue'
import AlbumCandidatePicker from './AlbumCandidatePicker.vue'
import { ALL_IDENTIFY_FIELD_IDS, type IdentifyFieldId } from '@/lib/identifyFields'
import { useReleaseGroupGenres } from '@/composables/useReleaseGroupGenres'

// Sentinel slot values for the per-row re-point dropdown: keep the server's
// proposal, or drop the position entirely (album fields only).
const SLOT_KEEP = -1
const SLOT_CLEAR = 0

// Composite slot identity: a position is a (disc, track) pair, not a bare track number.
function slotKey(disc: number, track: number): string {
    return `${disc}-${track}`
}

const props = defineProps<{
    visible: boolean
    options: AlbumOption[]
    tracks: Track[]
    pathErrors: Array<{ path: string; error: string }>
    // The dialog opens before the request resolves so the user sees the work
    // start; while this is true the body is a progress note and there is nothing
    // to review yet.
    loading: boolean
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // `fields` is which of the release's values to stage; everything else on the
    // included songs is left as it is on disk.
    (e: 'apply', picks: AlbumIdentifyPick[], fields: IdentifyFieldId[]): void
    // Cancel is not just "close": it aborts the in-flight identify request, so
    // the parent needs to hear it rather than only observing visible: false.
    (e: 'cancel'): void
    // Discard the cached answer for this selection and identify it again. The
    // dialog stays open: the fresh run repopulates it in place.
    (e: 'reidentify'): void
}>()

// Per-song review state. `slot` is the tracklist position the user settled on:
// SLOT_KEEP defers to the chosen album's own assignment, SLOT_CLEAR stages the
// album fields with no position, any other value is a slotKey string (disc-track).
interface RowState {
    included: boolean
    slot: number | string
}

const selectedMbid = ref('')
const rows = ref(new Map<string, RowState>())

// The candidate comparison dialog. Opened from the Compare button; picking a row
// repoints `selectedMbid`, which re-derives the whole review below.
const pickerVisible = ref(false)

// Which of the release's fields get staged. All selected by default — the common
// case is "take the release"; narrowing it is the deliberate act (e.g. stage the
// album and year over a rip whose titles are already right).
const selectedFields = ref<IdentifyFieldId[]>([...ALL_IDENTIFY_FIELD_IDS])

const selectedOption = computed<AlbumOption | undefined>(() =>
    props.options.find((o) => o.release_mbid === selectedMbid.value)
)

// Switching albums invalidates every manual re-point: the positions belonged to
// the previous tracklist. Reset the rows rather than carry meaningless slots.
function resetRows() {
    const next = new Map<string, RowState>()
    for (const t of props.tracks) {
        next.set(t.path, { included: true, slot: SLOT_KEEP })
    }
    rows.value = next
}

watch(
    () => props.options,
    (options) => {
        selectedMbid.value = options[0]?.release_mbid ?? ''
        resetRows()
    },
    { immediate: true }
)

watch(selectedMbid, resetRows)

const trackByPath = computed(() => {
    const map = new Map<string, Track>()
    for (const t of props.tracks) map.set(t.path, t)
    return map
})

const assignmentByPath = computed(() => {
    const map = new Map<string, AlbumAssignment>()
    for (const a of selectedOption.value?.assignments ?? []) map.set(a.path, a)
    return map
})

const slotByKey = computed(() => {
    const map = new Map<string, { disc_number: number; track_number: number; title: string; recording_mbid: string }>()
    for (const s of selectedOption.value?.tracks ?? []) {
        map.set(slotKey(s.disc_number, s.track_number), {
            disc_number: s.disc_number,
            track_number: s.track_number,
            title: s.title,
            recording_mbid: s.recording_mbid
        })
    }
    return map
})

function rowState(path: string): RowState {
    return rows.value.get(path) ?? { included: true, slot: SLOT_KEEP }
}

// resolved is what the row actually stages: the album's assignment, the user's
// re-point, or nothing.
function resolved(path: string): AlbumAssignment | null {
    const state = rowState(path)
    const proposed = assignmentByPath.value.get(path) ?? null
    if (state.slot === SLOT_KEEP) {
        if (!proposed || proposed.source === 'none') return null
        return proposed
    }
    if (state.slot === SLOT_CLEAR) return null
    // state.slot is a slotKey string (disc-track).
    const slot = slotByKey.value.get(state.slot as string)
    if (!slot) return null
    return {
        path,
        // A hand-picked position is an assertion by the user, not an inference.
        source: 'fingerprint',
        title: slot.title,
        recording_mbid: slot.recording_mbid,
        artists: proposed?.artists ?? [],
        disc_number: slot.disc_number,
        track_number: slot.track_number,
        score: 0
    }
}

function badge(path: string): string {
    const state = rowState(path)
    if (state.slot !== SLOT_KEEP) return state.slot === SLOT_CLEAR ? 'none' : 'chosen'
    return assignmentByPath.value.get(path)?.source ?? 'none'
}

function rowError(path: string): string {
    return assignmentByPath.value.get(path)?.error ?? ''
}

// The file name, which is what the File column shows: it identifies the row
// unambiguously even for a file whose title tag is missing, wrong, or identical
// to another file's. The current title tag is still reachable — it is named in
// the Title cell's tooltip.
function fileName(path: string): string {
    return trackByPath.value.get(path)?.name || path
}

// The title the file carries today. Not shown as its own column any more, but it
// is what a proposed title is compared against and what the tooltip reports.
function currentTitle(path: string): string {
    const t = trackByPath.value.get(path)
    return t?.title ?? ''
}

// The artist the file carries today. Like currentTitle, not a column of its own:
// it is what a proposed artist is compared against and what the tooltip reports.
function currentArtist(path: string): string {
    const names = (trackByPath.value.get(path)?.artists ?? []).filter((n) => n !== '')
    return names.length > 0 ? names.join(', ') : ''
}

function currentAlbum(path: string): string {
    return trackByPath.value.get(path)?.album ?? ''
}

// Rendered as a string so an absent year is '' like every other missing tag,
// rather than a 0 the user has to interpret.
function currentYear(path: string): string {
    const year = trackByPath.value.get(path)?.year ?? 0
    return year > 0 ? String(year) : ''
}

// ----- current vs target -----
// The table shows only what a save would write. When the target differs from the
// value on disk the cell is highlighted and carries a tooltip naming what it
// replaces, so a rename is visible at a glance without spending a column on the
// old value.

// targetTitle is what the row would write, or '' when it stages no title (an
// unplaced row keeps the file's own).
function targetTitle(path: string): string {
    return resolved(path)?.title ?? ''
}

function titleChanged(path: string): boolean {
    const target = targetTitle(path)
    return target !== '' && target !== currentTitle(path)
}

function targetArtist(path: string): string {
    const names = (resolved(path)?.artists ?? []).map((a) => a.name).filter((n) => n !== '')
    if (names.length > 0) return names.join(', ')
    // A recording without its own credits inherits the release's artists — that
    // is what a save would actually write as the album artist.
    const albumArtists = (selectedOption.value?.artists ?? [])
        .map((a) => a.name)
        .filter((n) => n !== '')
    return albumArtists.join(', ')
}

function artistChanged(path: string): boolean {
    const target = targetArtist(path)
    return target !== '' && target !== currentArtist(path)
}

// Album and year come from the chosen release, so every row stages the same
// value — but whether it CHANGES anything is per row: a selection can mix files
// that already carry the album with files that do not, and only the latter should
// light up. They are also staged for a row with no position at all, which is why
// they read from the option rather than from `resolved`.
function targetAlbum(): string {
    return selectedOption.value?.album ?? ''
}

function albumChanged(path: string): boolean {
    const target = targetAlbum()
    return target !== '' && target !== currentAlbum(path)
}

function targetYear(): string {
    const year = selectedOption.value?.year ?? 0
    return year > 0 ? String(year) : ''
}

function yearChanged(path: string): boolean {
    const target = targetYear()
    return target !== '' && target !== currentYear(path)
}

// The tooltip on a changed cell: what the value is now, since the column shows
// what it will become. This is the ONLY place the file's existing title and
// artist tags are shown — the File column identifies the file, not its tags — so
// an absent tag has to say so rather than render as a blank tooltip.
function replacesTooltip(current: string): string {
    return current === '' ? 'Currently: (no value)' : `Currently: ${current}`
}

// Rows in tracklist order, then the unplaced ones — the order the album reads
// in, not the order the file system happened to list.
const orderedPaths = computed(() => {
    const paths = props.tracks.map((t) => t.path)
    return [...paths].sort((a, b) => {
        const ra = resolved(a)
        const rb = resolved(b)
        // Sort placed rows before unplaced, then by disc and track separately (no magic multiplier).
        if (!ra && !rb) return a.localeCompare(b)
        if (!ra) return 1
        if (!rb) return -1
        if (ra.disc_number !== rb.disc_number) return ra.disc_number - rb.disc_number
        if (ra.track_number !== rb.track_number) return ra.track_number - rb.track_number
        return a.localeCompare(b)
    })
})

// A table row is either one of the selected files or a tracklist position no
// selected file fills — the release has a song there, this selection just does
// not include it. Showing those as placeholders makes a partial selection
// obvious ("I only picked 9 of the 11 tracks") instead of silently looking
// complete.
type TableRow =
    | { kind: 'file'; path: string; disc: number; track: number }
    | { kind: 'gap'; disc: number; track: number; title: string }

function rowKey(row: TableRow): string {
    return row.kind === 'file' ? `file:${row.path}` : `gap:${row.disc}-${row.track}`
}

// Grouped by disc so the table reads like the album view's track list: a disc
// banner only when the release actually spans several, and unplaced files in a
// trailing group of their own (disc 0) because they belong to no disc yet.
const discGroups = computed(() => {
    const groups: Array<{ discNumber: number; rows: TableRow[] }> = []
    const pushRow = (disc: number, row: TableRow) => {
        const last = groups[groups.length - 1]
        if (last && last.discNumber === disc) last.rows.push(row)
        else groups.push({ discNumber: disc, rows: [row] })
    }

    // Positions the selection accounts for, so the gaps are everything else on
    // the release. Only included rows count: unchecking a file means its slot is
    // no longer being filled, so the placeholder should come back.
    const filled = new Set<string>()
    for (const path of includedPaths.value) {
        const r = resolved(path)
        if (r && r.track_number > 0) filled.add(slotKey(r.disc_number, r.track_number))
    }

    // Walk the placed files and the release's own tracklist together, in position
    // order, so a gap lands where the missing song actually sits.
    const gaps = (selectedOption.value?.tracks ?? [])
        .filter((s) => !filled.has(slotKey(s.disc_number, s.track_number)))
        .map<TableRow>((s) => ({
            kind: 'gap',
            disc: s.disc_number,
            track: s.track_number,
            title: s.title
        }))

    const placed: TableRow[] = []
    const unplaced: TableRow[] = []
    for (const path of orderedPaths.value) {
        const r = resolved(path)
        if (r && r.track_number > 0) {
            placed.push({ kind: 'file', path, disc: r.disc_number, track: r.track_number })
        } else {
            unplaced.push({ kind: 'file', path, disc: 0, track: 0 })
        }
    }

    const merged = [...placed, ...gaps].sort((a, b) => {
        if (a.disc !== b.disc) return a.disc - b.disc
        return a.track - b.track
    })
    for (const row of merged) pushRow(row.disc, row)
    // Files with no position at all trail the whole table, under their own banner.
    for (const row of unplaced) pushRow(0, row)
    return groups
})

// Only banner the discs when there is more than one REAL disc; a lone unplaced
// group is not a disc and must not make a single-disc release look multi-disc.
const hasMultipleDiscs = computed(
    () => discGroups.value.filter((g) => g.discNumber > 0).length > 1
)

function albumLabel(o: AlbumOption): string {
    // `?? []` despite the type promising an array: this label is the first thing
    // rendered for every option, so a single missing credit list would take the
    // whole dialog down rather than degrade one row.
    const artist = (o.artists ?? []).map((a) => a.name).join(', ')
    const year = o.year > 0 ? ` (${o.year})` : ''
    return artist ? `${o.album} — ${artist}${year}` : `${o.album}${year}`
}

function albumDetail(o: AlbumOption): string {
    const parts = [`${o.matched_count} of ${props.tracks.length} songs matched`]
    if (o.enriched) {
        parts.push(`${o.track_count} track${o.track_count === 1 ? '' : 's'}`)
        if (o.disc_count > 1) parts.push(`${o.disc_count} discs`)
    } else {
        parts.push('track list unavailable')
    }
    return parts.join(' · ')
}

const selectedDetail = computed(() =>
    selectedOption.value ? albumDetail(selectedOption.value) : ''
)

// ----- Genres -----
// The identify response carries no genres: MusicBrainz keeps genre votes on the
// release GROUP, and the resolver only fetches the release's tracklist. So they
// are looked up separately for whichever option the user has settled on, through
// a shared cache — the lookup is throttled to one request per second server-side,
// and the same group comes up again every time this dialog is reopened.
const genreCache = useReleaseGroupGenres()
const selectedGenres = ref<string[]>([])

watch(
    selectedOption,
    (option) => {
        const mbid = option?.release_group_mbid ?? ''
        // A group already in the cache renders immediately: no request, and no
        // empty row flashing before the answer arrives.
        selectedGenres.value = genreCache.cached(mbid) ?? []
        if (mbid === '' || selectedGenres.value.length > 0) return
        void genreCache.lookup(mbid).then((genres) => {
            // Guard against a slow answer landing after the user moved on: it
            // belongs to an option that is no longer selected.
            if (selectedOption.value?.release_group_mbid !== mbid) return
            selectedGenres.value = genres
        })
    },
    { immediate: true }
)

const includedPaths = computed(() =>
    props.tracks.map((t) => t.path).filter((p) => rowState(p).included)
)

// Positions already claimed by INCLUDED rows, so a re-point dropdown only offers free slots.
// Unchecking a row frees its position.
const takenPositions = computed(() => {
    const taken = new Set<string>()
    for (const path of includedPaths.value) {
        const r = resolved(path)
        if (r && r.track_number > 0) taken.add(slotKey(r.disc_number, r.track_number))
    }
    return taken
})

function slotChoices(path: string): Array<{ value: number | string; label: string }> {
    const mine = resolved(path)
    const mineKey = mine ? slotKey(mine.disc_number, mine.track_number) : null
    const isMultiDisc = (selectedOption.value?.disc_count ?? 0) > 1
    const choices: Array<{ value: number | string; label: string }> = [
        { value: SLOT_KEEP, label: 'Keep proposed' },
        { value: SLOT_CLEAR, label: 'No position' }
    ]
    for (const s of selectedOption.value?.tracks ?? []) {
        const key = slotKey(s.disc_number, s.track_number)
        if (takenPositions.value.has(key) && mineKey !== key) {
            continue
        }
        const label = isMultiDisc
            ? `${s.disc_number}-${s.track_number}. ${s.title}`
            : `${s.track_number}. ${s.title}`
        choices.push({ value: key, label })
    }
    return choices
}

// Two included songs on one position (same disc AND track number) would write
// the same track number twice; the server's gap-fill avoids it, but a stale
// option or a manual pick could still collide.
const conflictingPositions = computed(() => {
    const seen = new Map<string, number>()
    for (const path of includedPaths.value) {
        const r = resolved(path)
        if (!r || r.track_number <= 0) continue
        const key = slotKey(r.disc_number, r.track_number)
        seen.set(key, (seen.get(key) ?? 0) + 1)
    }
    return [...seen.entries()].filter(([, n]) => n > 1).map(([key]) => key)
})

function isConflicting(path: string): boolean {
    const r = resolved(path)
    if (!r) return false
    return conflictingPositions.value.includes(slotKey(r.disc_number, r.track_number))
}

const canApply = computed(
    () =>
        selectedOption.value !== undefined &&
        includedPaths.value.length > 0 &&
        conflictingPositions.value.length === 0 &&
        // Every field unchecked would stage nothing at all, so the button has
        // nothing to do rather than silently applying an empty overlay.
        selectedFields.value.length > 0
)

function apply() {
    const option = selectedOption.value
    if (!option) return
    const picks: AlbumIdentifyPick[] = includedPaths.value.map((path) => ({
        path,
        option,
        assignment: resolved(path),
        // Album-level, like the album name itself: every included song gets the
        // release group's genres.
        genres: [...selectedGenres.value]
    }))
    emit('apply', picks, [...selectedFields.value])
}

// One exit path for Cancel, the header X and an Escape press: all three mean
// "stop", and while a request is in flight stopping must abort it, not leave it
// running invisibly against the user's rate limit.
function cancel() {
    emit('cancel')
    emit('update:visible', false)
}
</script>

<template>
    <!-- Near-full-screen: eight columns of current-vs-target comparison over a
         whole album need the width, and the row list needs the height so a
         20-track release is not read through a keyhole. -->
    <Dialog
        :visible="visible"
        @update:visible="(v) => !v && cancel()"
        header="Identify album"
        modal
        :style="{ width: '96vw', maxWidth: '96vw', height: '92vh' }"
        class="album-identify-dialog"
    >
        <div v-if="loading" class="album-loading" data-test="album-loading">
            <i class="pi pi-spin pi-spinner"></i>
            <div class="loading-text">
                <p class="loading-headline">
                    Identifying {{ tracks.length }} song{{ tracks.length === 1 ? '' : 's' }}…
                </p>
                <!-- Sets the expectation: this is one fpcalc run plus a
                     rate-limited AcoustID call per file, then MusicBrainz
                     lookups, so tens of seconds is normal rather than a hang. -->
                <small class="loading-note">
                    Fingerprinting each file and looking up matching releases. This can take a
                    while; Cancel stops it.
                </small>
            </div>
        </div>

        <div
            v-else-if="pathErrors.length > 0"
            class="album-path-errors"
            data-test="album-path-errors"
        >
            <!-- Covers both kinds the server reports here: files it could not
                 fingerprint or look up, and paths it refused before
                 identification (outside the library). Naming only fingerprinting
                 would mislabel the latter, which was never fingerprinted. -->
            <p class="path-errors-header">Some files were not identified:</p>
            <ul class="path-errors-list">
                <li v-for="e in pathErrors" :key="e.path" class="path-error-item">
                    <span class="path-error-path">{{ e.path }}</span>
                    <span class="path-error-message">{{ e.error }}</span>
                </li>
            </ul>
        </div>

        <div
            v-if="!loading && options.length === 0 && pathErrors.length === 0"
            class="album-empty"
            data-test="album-empty"
        >
            None of these songs matched a known release.
        </div>

        <div
            v-else-if="!loading && options.length === 0 && pathErrors.length > 0"
            class="album-empty"
            data-test="album-empty"
        >
            No songs matched a known release.
        </div>

        <template v-else-if="!loading">
            <div class="album-pick">
                <span class="album-pick-label">Album</span>
                <div class="album-current" data-test="album-current">
                    <span class="album-current-name">{{
                        selectedOption ? albumLabel(selectedOption) : ''
                    }}</span>
                    <small class="album-detail">{{ selectedDetail }}</small>
                </div>
                <!-- The dropdown became a table: when several releases matched,
                     this opens a comparison the user can actually judge (coverage,
                     confidence and what each would change) rather than a bare list
                     of names. Absent for a lone match — nothing to compare. -->
                <Button
                    v-if="options.length > 1"
                    :label="`Compare ${options.length} candidates`"
                    icon="pi pi-list"
                    text
                    data-test="album-compare"
                    @click="pickerVisible = true"
                />
            </div>

            <!-- What the Genres checkbox would write, shown because it is the one
                 staged value with no column in the table below: genres are
                 album-level, identical on every row, and they come from a
                 separate release-group lookup rather than from the match. Absent
                 when the lookup found nothing or failed — there is then nothing
                 to stage and nothing to preview. -->
            <p v-if="selectedGenres.length > 0" class="album-genres" data-test="album-genres">
                <span class="album-genres-label">Genres</span>
                <span class="album-genres-value">{{ selectedGenres.join(', ') }}</span>
            </p>

            <p v-if="conflictingPositions.length > 0" class="album-conflict" data-test="album-conflict">
                Two songs are on the same track position. Change one before staging.
            </p>

            <IdentifyFieldSelect
                v-model="selectedFields"
                testPrefix="album"
            />

            <div class="track-list">
                <div class="track-list-header">
                    <span class="col-include" aria-label="Include"></span>
                    <span class="col-index">#</span>
                    <span class="col-current">File</span>
                    <!-- The target columns are the whole point of the table, so they
                         carry the plain field names; each cell that would overwrite
                         something names the current value in its tooltip, which is
                         why no "current artist" column is needed beside them. -->
                    <span class="col-target">Title</span>
                    <span class="col-target">Artist</span>
                    <span class="col-target">Album</span>
                    <span class="col-target col-year">Year</span>
                    <span class="col-source">Match</span>
                    <span class="col-slot">Position</span>
                </div>

                <template v-for="(group, gi) in discGroups" :key="group.discNumber">
                    <div v-if="hasMultipleDiscs && group.discNumber > 0" class="disc-header">
                        Disc {{ group.discNumber }}
                    </div>
                    <!-- The unplaced group carries no disc, so it gets its own
                         banner rather than being mistaken for disc 1's tail. -->
                    <div
                        v-else-if="group.discNumber === 0 && gi > 0"
                        class="disc-header unplaced-header"
                    >
                        No position on this release
                    </div>

                    <template v-for="(row, i) in group.rows" :key="rowKey(row)">
                        <!-- A position on the release that none of the selected
                             files fills: shown in place, greyed and inert, so a
                             partial selection is visible instead of looking
                             complete. -->
                        <div
                            v-if="row.kind === 'gap'"
                            class="album-track-row gap-row"
                            :class="{ striped: i % 2 === 1 }"
                            :data-test="`album-gap-${row.disc}-${row.track}`"
                        >
                            <span class="col-include"></span>
                            <span class="col-index track-number">{{ row.track }}</span>
                            <span class="col-current cell-current">
                                <span class="cell-value gap-note">not in selection</span>
                            </span>
                            <span class="col-target cell-target">
                                <span class="cell-value gap-title">{{ row.title }}</span>
                            </span>
                            <!-- Artist, album and year stay blank: no file is being
                                 written here, so there is nothing to stage. -->
                            <span class="col-target"></span>
                            <span class="col-target"></span>
                            <span class="col-target col-year"></span>
                            <span class="col-source"></span>
                            <span class="col-slot"></span>
                        </div>

                        <div
                            v-else
                            class="album-track-row"
                            :class="{
                                striped: i % 2 === 1,
                                conflicting: isConflicting(row.path),
                                excluded: !rowState(row.path).included
                            }"
                            :data-test="`album-row-${row.path}`"
                        >
                        <span class="col-include">
                            <Checkbox
                                :modelValue="rowState(row.path).included"
                                @update:modelValue="(v: boolean) => (rowState(row.path).included = v)"
                                :binary="true"
                                :inputId="`album-include-${row.path}`"
                            />
                        </span>
                        <span class="col-index track-number">
                            <template v-if="resolved(row.path)">
                                {{ resolved(row.path)!.track_number }}
                            </template>
                            <template v-else>—</template>
                        </span>
                        <label
                            :for="`album-include-${row.path}`"
                            class="col-current cell-current"
                            :data-test="`album-file-${row.path}`"
                        >
                            <span v-tooltip.top="row.path" class="cell-value file-name">
                                {{ fileName(row.path) }}
                            </span>
                            <small v-if="rowError(row.path)" class="row-error">
                                {{ rowError(row.path) }}
                            </small>
                        </label>
                        <span
                            class="col-target cell-target"
                            :class="{ changed: titleChanged(row.path) }"
                            :data-test="`album-title-${row.path}`"
                        >
                            <span
                                v-if="targetTitle(row.path) !== ''"
                                v-tooltip.top="
                                    titleChanged(row.path) ? replacesTooltip(currentTitle(row.path)) : ''
                                "
                                class="cell-value"
                                >{{ targetTitle(row.path) }}</span
                            >
                            <span v-else class="cell-unchanged">unchanged</span>
                        </span>

                        <span
                            class="col-target cell-target"
                            :class="{ changed: artistChanged(row.path) }"
                            :data-test="`album-artist-${row.path}`"
                        >
                            <span
                                v-if="targetArtist(row.path) !== ''"
                                v-tooltip.top="
                                    artistChanged(row.path) ? replacesTooltip(currentArtist(row.path)) : ''
                                "
                                class="cell-value"
                                >{{ targetArtist(row.path) }}</span
                            >
                            <span v-else class="cell-unchanged">unchanged</span>
                        </span>

                        <span
                            class="col-target cell-target"
                            :class="{ changed: albumChanged(row.path) }"
                            :data-test="`album-album-${row.path}`"
                        >
                            <span
                                v-if="targetAlbum() !== ''"
                                v-tooltip.top="
                                    albumChanged(row.path) ? replacesTooltip(currentAlbum(row.path)) : ''
                                "
                                class="cell-value"
                                >{{ targetAlbum() }}</span
                            >
                            <span v-else class="cell-unchanged">unchanged</span>
                        </span>
                        <span
                            class="col-target col-year cell-target"
                            :class="{ changed: yearChanged(row.path) }"
                            :data-test="`album-year-${row.path}`"
                        >
                            <span
                                v-if="targetYear() !== ''"
                                v-tooltip.top="
                                    yearChanged(row.path) ? replacesTooltip(currentYear(row.path)) : ''
                                "
                                class="cell-value"
                                >{{ targetYear() }}</span
                            >
                            <span v-else class="cell-unchanged">unchanged</span>
                        </span>
                        <span class="col-source">
                            <span
                                class="row-badge"
                                :class="`badge-${badge(row.path)}`"
                                :data-test="`album-badge-${row.path}`"
                            >
                                {{ badge(row.path) }}
                            </span>
                        </span>
                        <Select
                            class="col-slot row-slot"
                            :data-test="`album-slot-${row.path}`"
                            :modelValue="rowState(row.path).slot"
                            @update:modelValue="(v: number | string) => (rowState(row.path).slot = v)"
                            :options="slotChoices(row.path)"
                            optionLabel="label"
                            optionValue="value"
                        />
                    </div>
                    </template>
                </template>
            </div>

            <AlbumCandidatePicker
                v-model:visible="pickerVisible"
                :options="options"
                :selectedMbid="selectedMbid"
                :tracks="tracks"
                @select="(mbid: string) => (selectedMbid = mbid)"
            />
        </template>

        <template #footer>
            <Button label="Cancel" text data-test="album-cancel" @click="cancel" />
            <!-- Options can be served from the session's identify cache, which
                 makes reopening this dialog instant but also means an answer that
                 came back empty or rate-limited would otherwise be unrepeatable.
                 Offered even with no options for exactly that case. -->
            <Button
                v-if="!loading"
                label="Re-identify"
                icon="pi pi-refresh"
                text
                v-tooltip.top="'Ignore the cached answer and identify these files again'"
                data-test="album-reidentify"
                @click="emit('reidentify')"
            />
            <Button
                v-if="!loading && options.length > 0"
                :label="`Stage ${includedPaths.length} song${includedPaths.length === 1 ? '' : 's'}`"
                icon="pi pi-check"
                data-test="album-apply"
                :disabled="!canApply"
                @click="apply"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.album-loading {
    display: flex;
    align-items: flex-start;
    gap: 0.9rem;
    padding: 2rem 0.5rem;
}
.album-loading .pi-spinner {
    font-size: 1.6rem;
    color: var(--app-accent);
}
.loading-text {
    display: flex;
    flex-direction: column;
    gap: 0.3rem;
    min-width: 0;
}
.loading-headline {
    margin: 0;
    font-weight: 600;
}
.loading-note {
    color: var(--app-text-secondary);
}
.album-empty {
    padding: 1rem 0;
    color: var(--app-text-secondary);
}
.album-path-errors {
    margin-bottom: 1rem;
    padding: 0.75rem;
    border: 1px solid var(--p-red-300, #fca5a5);
    border-radius: 6px;
    background: var(--p-red-50, #fef2f2);
}
.path-errors-header {
    margin: 0 0 0.5rem;
    font-weight: 600;
    color: var(--p-red-800, #991b1b);
}
.path-errors-list {
    margin: 0;
    padding-left: 1.25rem;
    list-style: disc;
}
.path-error-item {
    margin-bottom: 0.25rem;
    font-size: 0.875rem;
}
.path-error-path {
    font-family: var(--p-font-mono, 'Courier New', monospace);
    font-weight: 600;
    color: var(--p-red-900, #7f1d1d);
}
.path-error-message {
    margin-left: 0.5rem;
    color: var(--p-red-700, #b91c1c);
}
.album-pick {
    display: flex;
    align-items: center;
    gap: 0.6rem;
    padding-bottom: 0.8rem;
}
.album-pick-label {
    font-weight: 600;
    color: var(--app-text-secondary);
}
/* The chosen release, read-only: switching is the Compare button's job now, not
   an inline control. Truncates so a long "Album — Artist (year)" cannot push the
   Compare button off the row. */
.album-current {
    display: flex;
    align-items: baseline;
    gap: 0.6rem;
    min-width: 0;
}
.album-current-name {
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.album-detail {
    color: var(--app-text-secondary);
    white-space: nowrap;
}
.album-conflict {
    margin: 0 0 0.6rem;
    color: var(--p-red-600, #dc2626);
}
/* The genre preview reads as one more staged value, so the list is styled like a
   target cell in the table below (--app-staged = a pending tag change) behind a
   muted label matching the table's column headers. */
.album-genres {
    display: flex;
    align-items: baseline;
    gap: 0.6rem;
    margin: 0 0 0.8rem;
    font-size: 0.85rem;
}
.album-genres-label {
    font-weight: 600;
    text-transform: uppercase;
    font-size: 0.75rem;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
}
.album-genres-value {
    color: var(--app-staged);
    font-weight: 600;
}
/* A fixed-height dialog only pays off if the body actually fills it: make the
   PrimeVue content region a flex column so the track table absorbs the leftover
   height and scrolls internally, instead of the whole dialog overflowing. Needs
   :deep() because these are PrimeVue's own elements, not ours. */
.album-identify-dialog:deep(.p-dialog-content) {
    display: flex;
    flex-direction: column;
    min-height: 0;
    flex: 1;
    overflow: hidden;
}

/* The album track table, styled to match AlbumView's track list: one shared grid
   template for the header and every row, uppercase column labels, zebra
   striping, and accent disc banners. Kept as a local copy of the pattern rather
   than reusing AlbumTrackRow — that component renders a playable Subsonic Song
   with drag and selection, while these rows are editor controls (a checkbox, a
   match badge, a position dropdown) over identify results. */
.track-list {
    /* Shared grid template so the header and every row align. */
    /* include · # · file · title · artist · album · year · match · position. The
       file and the free-text staged values share the flexible width; the current
       tags live in the target cells' tooltips instead of columns of their own.
       Year, match and position are fixed since their content is bounded — a year
       is always four digits, so a flexible track would only add dead space. */
    --album-track-cols: 2.2rem 34px minmax(0, 1.4fr) minmax(0, 1.4fr) minmax(0, 1.1fr)
        minmax(0, 1.1fr) 3.5rem 7rem 15rem;
    display: flex;
    flex-direction: column;
    /* Fills the dialog's remaining height rather than a fixed viewport slice, so
       the taller dialog actually shows more rows. */
    flex: 1;
    min-height: 0;
    overflow-y: auto;
}
.track-list-header {
    display: grid;
    grid-template-columns: var(--album-track-cols);
    column-gap: 0.75rem;
    padding: 0 0.5rem 0.4rem;
    border-bottom: 1px solid var(--app-border);
    margin-bottom: 0.25rem;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
    /* The header stays put while a long tracklist scrolls under it. */
    position: sticky;
    top: 0;
    background: var(--app-surface, #fff);
    z-index: 1;
}
.track-list-header .col-index {
    text-align: right;
}
.disc-header {
    background: var(--app-accent);
    color: #fff;
    text-align: center;
    padding: 0.7rem 1rem;
    margin: 0.5rem 0 0.25rem;
    font-size: 0.8rem;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.05em;
}
/* The unplaced group is not a disc, so it is muted rather than accented. */
.disc-header.unplaced-header {
    background: var(--app-surface-alt, #f1f5f9);
    color: var(--app-text-secondary);
}
.album-track-row {
    display: grid;
    grid-template-columns: var(--album-track-cols);
    align-items: center;
    column-gap: 0.75rem;
    width: 100%;
    min-height: 56px;
    padding: 0 0.5rem;
    box-sizing: border-box;
}
.album-track-row.striped {
    background-color: rgba(0, 0, 0, 0.025);
}
.album-track-row:hover {
    background-color: var(--app-hover);
}
/* A conflicting position blocks staging, so it has to be visible at a glance in
   a long list — a left accent bar rather than a full border, which would break
   the grid's alignment. */
.album-track-row.conflicting,
.album-track-row.conflicting.striped {
    background-color: var(--p-red-50, #fef2f2);
    box-shadow: inset 3px 0 0 var(--p-red-600, #dc2626);
}
/* A tracklist position no selected file fills. Inert and greyed: there is
   nothing to include, re-point or stage, so it reads as a hole in the selection
   rather than as a row the user forgot to tick. */
.album-track-row.gap-row {
    opacity: 0.45;
    cursor: default;
}
.album-track-row.gap-row:hover {
    /* No hover affordance — nothing here responds to a click. */
    background-color: transparent;
}
.album-track-row.gap-row.striped:hover {
    background-color: rgba(0, 0, 0, 0.025);
}
.gap-note {
    font-style: italic;
}
.gap-title {
    font-style: italic;
}

/* An excluded row stages nothing; dim it so the included set reads clearly. */
.album-track-row.excluded .col-current,
.album-track-row.excluded .col-target,
.album-track-row.excluded .col-index {
    opacity: 0.5;
}
.col-index {
    text-align: right;
}
/* A year is a number in a narrow column: tabular figures so the digits line up
   down the table the way the track numbers do. */
.col-year .cell-value {
    font-variant-numeric: tabular-nums;
}
.track-number {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
    font-weight: 500;
    font-variant-numeric: tabular-nums;
}
/* The File column (context, muted) and the staged-value columns (emphasised,
   since that is what a save writes). .col-current is the file cell only — it is a
   <label> for the row checkbox, hence the pointer. */
.col-current,
.col-target {
    display: flex;
    flex-direction: column;
    min-width: 0;
}
.col-current {
    cursor: pointer;
}
.cell-value {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.cell-current .cell-value {
    font-size: 0.85rem;
    color: var(--app-text-secondary);
}
/* The file name identifies the row, so it reads as a path rather than prose, and
   hovering shows the full path relative to the library. */
.file-name {
    font-family: var(--app-font-mono, ui-monospace, SFMono-Regular, Menlo, monospace);
    font-size: 0.8rem;
    cursor: help;
}
.cell-target .cell-value {
    font-size: 0.9rem;
    color: var(--app-text-primary);
}
/* A value that actually differs from the file's own: emphasised, and the only
   one that carries the "Currently: …" tooltip. */
/* --app-staged is the editor's established colour for a pending tag change (see
   EditPanel's .field-dirty rows). A value this dialog would overwrite is exactly
   that, so it reuses the token rather than picking its own orange — and inherits
   the per-theme shades already defined for light, dark and the hidden themes. */
.cell-target.changed .cell-value {
    font-weight: 600;
    color: var(--app-staged);
    cursor: help;
}
.cell-unchanged {
    font-size: 0.8rem;
    font-style: italic;
    color: var(--app-text-secondary);
    opacity: 0.7;
}
.row-error {
    font-size: 0.75rem;
    color: var(--p-red-600, #dc2626);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.row-badge {
    display: inline-block;
    font-size: 0.7rem;
    text-transform: uppercase;
    letter-spacing: 0.03em;
    padding: 0.1rem 0.4rem;
    border-radius: 4px;
    width: 100%;
    box-sizing: border-box;
    text-align: center;
}
.badge-fingerprint,
.badge-chosen {
    background: var(--p-green-100, #dcfce7);
    color: var(--p-green-800, #166534);
}
/* Same token as a changed value: an inferred position is a proposal the user
   should look at, not a confirmed match, so it reads in the editor's staged
   colour rather than a second hand-picked yellow. */
.badge-inferred {
    background: var(--app-staged-soft);
    color: var(--app-staged);
}
.badge-none {
    background: var(--app-surface-alt, #f1f5f9);
    color: var(--app-text-secondary);
}
.row-slot {
    /* The grid column already sizes this; keep it from overflowing on a long
       track title. */
    min-width: 0;
    max-width: 100%;
}
</style>
