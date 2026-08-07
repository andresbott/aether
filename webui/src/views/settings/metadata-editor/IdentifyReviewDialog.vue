<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Dialog from 'primevue/dialog'
import Select from 'primevue/select'
import Button from 'primevue/button'
import Checkbox from 'primevue/checkbox'
import type {
    IdentifyCandidate,
    IdentifyPick,
    IdentifyTrackResult,
    Track
} from '@/types/metadata'
import IdentifyFieldSelect from './IdentifyFieldSelect.vue'
import { ALL_IDENTIFY_FIELD_IDS, type IdentifyFieldId } from '@/lib/identifyFields'
import { useReleaseGroupGenres } from '@/composables/useReleaseGroupGenres'

// A candidate above this score is considered a confident match and its track
// is pre-accepted when the dialog opens.
const ACCEPT_THRESHOLD = 0.85

const props = defineProps<{
    visible: boolean
    results: IdentifyTrackResult[]
    tracks: Track[]
    // The dialog opens before the request resolves so the user sees the work
    // start; while this is true the body is a progress note and there is nothing
    // to review yet.
    loading: boolean
    // The files this run was launched for, so the progress note can name a count
    // before any result exists. `tracks` is the whole folder listing here (it is
    // the current-title lookup), so it cannot answer that.
    pending: Track[]
}>()
const emit = defineEmits<{
    (e: 'update:visible', v: boolean): void
    // `fields` is which of the match's values to stage; everything else on the
    // accepted tracks is left as it is on disk.
    (e: 'apply', picks: IdentifyPick[], fields: IdentifyFieldId[]): void
    // Cancel is not just "close": it aborts the in-flight identify request, so
    // the parent needs to hear it rather than only observing visible: false.
    (e: 'cancel'): void
    // Discard the cached answers for these files and fingerprint them again.
    // The dialog stays open: the fresh run repopulates it in place.
    (e: 'reidentify'): void
}>()

// Per-track review state: whether the user accepts the match, which candidate
// is chosen, and which of its releases the album fields come from.
interface RowState {
    accepted: boolean
    candidateIndex: number
    releaseIndex: number
}

const rows = ref(new Map<string, RowState>())

// Which of the match's fields get staged. All selected by default — the common
// case is "take the match"; narrowing it is the deliberate act (e.g. stage only
// the album on a batch of files and leave their titles alone).
const selectedFields = ref<IdentifyFieldId[]>([...ALL_IDENTIFY_FIELD_IDS])

watch(
    () => props.results,
    (results) => {
        const next = new Map<string, RowState>()
        for (const r of results) {
            const best = r.candidates[0]
            next.set(r.path, {
                accepted: !r.error && best !== undefined && best.score >= ACCEPT_THRESHOLD,
                candidateIndex: 0,
                releaseIndex: 0
            })
        }
        rows.value = next
    },
    { immediate: true }
)

const trackByPath = computed(() => {
    const map = new Map<string, Track>()
    for (const t of props.tracks) map.set(t.path, t)
    return map
})

function rowState(path: string): RowState {
    return rows.value.get(path) ?? { accepted: false, candidateIndex: 0, releaseIndex: 0 }
}

// A row is reviewable when identification actually produced something to accept;
// the rest render as inert rows carrying their reason.
function isReviewable(result: IdentifyTrackResult): boolean {
    return !result.error && result.candidates.length > 0
}

// The file name identifies the row: it is unambiguous even for a file whose title
// tag is missing, wrong, or identical to another file's. The current title tag is
// still reachable — it is named in the Title cell's tooltip.
function fileName(path: string): string {
    return trackByPath.value.get(path)?.name || path
}

function currentTitle(path: string): string {
    return trackByPath.value.get(path)?.title ?? ''
}

function currentArtist(path: string): string {
    const names = (trackByPath.value.get(path)?.artists ?? []).filter((n) => n !== '')
    return names.join(', ')
}

function scorePct(score: number): string {
    return `${Math.round(score * 100)}%`
}

function candidateLabel(c: IdentifyCandidate): string {
    const artists = c.artists.map((a) => a.name).join(', ')
    return artists ? `${c.title} — ${artists}` : c.title
}

// The candidate the row would stage, and the release its album fields come from.
function chosenCandidate(path: string): IdentifyCandidate | null {
    const result = props.results.find((r) => r.path === path)
    return result?.candidates[rowState(path).candidateIndex] ?? null
}

// ----- current vs target -----
// Only the target renders as a column. When it differs from the file's current
// value it is highlighted and carries a tooltip naming what it replaces, so a
// rename is visible at a glance without spending a column on the old value.

function targetTitle(path: string): string {
    return chosenCandidate(path)?.title ?? ''
}

function titleChanged(path: string): boolean {
    const target = targetTitle(path)
    return target !== '' && target !== currentTitle(path)
}

function targetArtist(path: string): string {
    const names = (chosenCandidate(path)?.artists ?? []).map((a) => a.name).filter((n) => n !== '')
    return names.join(', ')
}

function artistChanged(path: string): boolean {
    const target = targetArtist(path)
    return target !== '' && target !== currentArtist(path)
}

function currentGenres(path: string): string {
    return (trackByPath.value.get(path)?.genres ?? []).filter((g) => g !== '').join(', ')
}

function targetGenres(path: string): string {
    return rowGenres(path).join(', ')
}

function genresChanged(path: string): boolean {
    const target = targetGenres(path)
    return target !== '' && target !== currentGenres(path)
}

// The tooltip on a changed cell: what the value is now, since the column shows
// what it will become. This is the ONLY place the file's existing title and
// artist tags are shown — the File column identifies the file, not its tags — so
// an absent tag has to say so rather than render as a blank tooltip.
function replacesTooltip(current: string): string {
    return current === '' ? 'Currently: (no value)' : `Currently: ${current}`
}

function candidateChoices(result: IdentifyTrackResult) {
    return result.candidates.map((c, i) => ({
        label: `${scorePct(c.score)} · ${candidateLabel(c)}`,
        value: i
    }))
}

function releaseChoices(path: string) {
    const candidate = chosenCandidate(path)
    if (!candidate) return []
    return candidate.releases.map((r, i) => {
        let label = r.year > 0 ? `${r.album} (${r.year})` : r.album
        if (r.track_number > 0) label += ` · track ${r.track_number}`
        return { label, value: i }
    })
}

function onCandidateChange(path: string, index: number) {
    const state = rows.value.get(path)
    if (!state) return
    state.candidateIndex = index
    // A different candidate has its own release list; restart at the first.
    state.releaseIndex = 0
}

const acceptedCount = computed(
    () => [...rows.value.values()].filter((s) => s.accepted).length
)

// ----- Genres -----
// A fingerprint match carries no genres: MusicBrainz keeps genre votes on the
// release GROUP. So they are looked up for the group of whichever release each
// row points at, through a shared cache — the lookup is throttled to one request
// per second server-side, and identifying a folder song by song means every row
// usually lands on the SAME album, which the cache collapses into one request.
const genreCache = useReleaseGroupGenres()
// Release group mbid -> its genres, for the groups the rows currently point at.
const genresByGroup = ref(new Map<string, string[]>())

// The release group a row would stage from, or '' when it has no release.
function chosenGroup(path: string): string {
    const candidate = chosenCandidate(path)
    if (!candidate) return ''
    return candidate.releases[rowState(path).releaseIndex]?.release_group_mbid ?? ''
}

// The genres a row would stage. Empty until its lookup lands, and stays empty for
// a row with no release group or whose lookup failed.
function rowGenres(path: string): string[] {
    return genresByGroup.value.get(chosenGroup(path)) ?? []
}

// The distinct release groups the reviewable rows point at right now. Deduped, so
// a folder of songs from one album costs ONE lookup rather than one per file.
const neededGroups = computed(() => {
    const groups = new Set<string>()
    for (const r of props.results) {
        if (!isReviewable(r)) continue
        const mbid = chosenGroup(r.path)
        if (mbid !== '') groups.add(mbid)
    }
    return [...groups]
})

// Fetch whatever the current rows need and nothing else. Re-runs when a row is
// re-pointed at another release or a fresh identify run replaces the results; the
// cache absorbs the repeats, so switching back and forth costs no requests.
watch(
    neededGroups,
    (groups) => {
        for (const mbid of groups) {
            if (genresByGroup.value.has(mbid)) continue
            const hit = genreCache.cached(mbid)
            if (hit !== undefined) {
                genresByGroup.value.set(mbid, hit)
                continue
            }
            // Claim the key before awaiting so a re-point back to this group
            // during the request does not queue a second one.
            genresByGroup.value.set(mbid, [])
            void genreCache.lookup(mbid).then((genres) => {
                genresByGroup.value.set(mbid, genres)
            })
        }
    },
    { immediate: true }
)

function apply() {
    const picks: IdentifyPick[] = []
    for (const r of props.results) {
        const state = rows.value.get(r.path)
        if (!state?.accepted) continue
        const candidate = r.candidates[state.candidateIndex]
        if (!candidate) continue
        picks.push({
            path: r.path,
            candidate,
            release: candidate.releases[state.releaseIndex] ?? null,
            genres: rowGenres(r.path)
        })
    }
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
    <!-- Same near-full-screen footprint as the album dialog: a review pass over a
         multi-file selection is the same job at the same scale, and reviewing
         candidates plus their release dropdowns through a keyhole is worse here
         than there, not better. -->
    <Dialog
        :visible="visible"
        @update:visible="(v) => !v && cancel()"
        header="Identify tracks"
        modal
        :style="{ width: '96vw', maxWidth: '96vw', height: '92vh' }"
        class="song-identify-dialog"
    >
        <div v-if="loading" class="identify-loading" data-test="identify-loading">
            <i class="pi pi-spin pi-spinner"></i>
            <div class="loading-text">
                <p class="loading-headline">
                    Identifying {{ pending.length }} track{{ pending.length === 1 ? '' : 's' }}…
                </p>
                <!-- Sets the expectation: one fpcalc run plus a rate-limited
                     AcoustID call per file, so several seconds per track is
                     normal rather than a hang. -->
                <small class="loading-note">
                    Fingerprinting each file and looking it up on AcoustID. This can take a while;
                    Cancel stops it.
                </small>
            </div>
        </div>

        <IdentifyFieldSelect v-else v-model="selectedFields" testPrefix="identify" />

        <div v-if="!loading" class="track-list">
            <div class="track-list-header">
                <span class="col-include" aria-label="Accept"></span>
                <span class="col-current">File</span>
                <span class="col-target">Title</span>
                <span class="col-target">Artist</span>
                <span class="col-target">Genres</span>
                <span class="col-choice">Recording</span>
                <span class="col-choice">Release</span>
            </div>

            <div
                v-for="(result, i) in results"
                :key="result.path"
                class="identify-track-row"
                :class="{
                    striped: i % 2 === 1,
                    unmatched: !isReviewable(result),
                    excluded: isReviewable(result) && !rowState(result.path).accepted
                }"
                :data-test="`identify-row-${result.path}`"
            >
                <span class="col-include">
                    <Checkbox
                        v-if="isReviewable(result)"
                        :modelValue="rowState(result.path).accepted"
                        @update:modelValue="(v: boolean) => (rowState(result.path).accepted = v)"
                        :binary="true"
                        :inputId="`accept-${result.path}`"
                    />
                </span>

                <label
                    :for="`accept-${result.path}`"
                    class="col-current cell-current"
                    :data-test="`identify-file-${result.path}`"
                >
                    <span v-tooltip.top="result.path" class="cell-value file-name">
                        {{ fileName(result.path) }}
                    </span>
                    <!-- Why this row cannot be staged, in the column that names
                         the file: an unfingerprintable file and one AcoustID
                         simply does not know are different problems. -->
                    <small v-if="result.error" class="row-error" data-test="identify-error">
                        {{ result.error }}
                    </small>
                    <small
                        v-else-if="result.candidates.length === 0"
                        class="row-nomatch"
                        data-test="identify-nomatch"
                    >
                        No match found.
                    </small>
                </label>

                <span
                    class="col-target cell-target"
                    :class="{ changed: titleChanged(result.path) }"
                    :data-test="`identify-title-${result.path}`"
                >
                    <span
                        v-if="targetTitle(result.path) !== ''"
                        v-tooltip.top="
                            titleChanged(result.path)
                                ? replacesTooltip(currentTitle(result.path))
                                : ''
                        "
                        class="cell-value"
                        >{{ targetTitle(result.path) }}</span
                    >
                    <span v-else class="cell-unchanged">unchanged</span>
                </span>

                <span
                    class="col-target cell-target"
                    :class="{ changed: artistChanged(result.path) }"
                    :data-test="`identify-artist-${result.path}`"
                >
                    <span
                        v-if="targetArtist(result.path) !== ''"
                        v-tooltip.top="
                            artistChanged(result.path)
                                ? replacesTooltip(currentArtist(result.path))
                                : ''
                        "
                        class="cell-value"
                        >{{ targetArtist(result.path) }}</span
                    >
                    <span v-else class="cell-unchanged">unchanged</span>
                </span>

                <!-- Genres do not come from the fingerprint match: they are
                     looked up for the picked release's group, so this cell fills
                     in a moment after the row appears. Empty reads as "unchanged"
                     like every other target that stages nothing. -->
                <span
                    class="col-target cell-target"
                    :class="{ changed: genresChanged(result.path) }"
                    :data-test="`identify-genres-${result.path}`"
                >
                    <span
                        v-if="targetGenres(result.path) !== ''"
                        v-tooltip.top="
                            genresChanged(result.path)
                                ? replacesTooltip(currentGenres(result.path))
                                : ''
                        "
                        class="cell-value"
                        >{{ targetGenres(result.path) }}</span
                    >
                    <span v-else class="cell-unchanged">unchanged</span>
                </span>

                <!-- Candidate and release as dropdowns rather than a radio list:
                     AcoustID routinely returns several recordings each with a
                     dozen releases, which as stacked radios turns one file into
                     half a screen. One row per file keeps the table scannable. -->
                <Select
                    v-if="isReviewable(result)"
                    class="col-choice row-choice"
                    :data-test="`identify-candidate-${result.path}`"
                    :modelValue="rowState(result.path).candidateIndex"
                    @update:modelValue="(v: number) => onCandidateChange(result.path, v)"
                    :options="candidateChoices(result)"
                    optionLabel="label"
                    optionValue="value"
                />
                <span v-else class="col-choice"></span>

                <Select
                    v-if="isReviewable(result) && releaseChoices(result.path).length > 1"
                    class="col-choice row-choice"
                    :data-test="`identify-release-${result.path}`"
                    :modelValue="rowState(result.path).releaseIndex"
                    @update:modelValue="(v: number) => (rowState(result.path).releaseIndex = v)"
                    :options="releaseChoices(result.path)"
                    optionLabel="label"
                    optionValue="value"
                />
                <span
                    v-else-if="isReviewable(result) && releaseChoices(result.path).length === 1"
                    class="col-choice cell-release-single"
                    :data-test="`identify-release-single-${result.path}`"
                >
                    <span class="cell-value">{{ releaseChoices(result.path)[0].label }}</span>
                </span>
                <span
                    v-else-if="isReviewable(result)"
                    class="col-choice cell-release-single"
                    :data-test="`identify-release-none-${result.path}`"
                >
                    <span class="cell-unchanged">no release</span>
                </span>
                <span v-else class="col-choice"></span>
            </div>
        </div>

        <template #footer>
            <Button label="Cancel" text data-test="identify-cancel" @click="cancel" />
            <!-- Results can be served from the session's identify cache, which
                 makes reopening this dialog instant but also means a run that was
                 rate-limited or looked at the wrong file would otherwise be
                 unrepeatable. This is the way back to a fresh lookup. -->
            <Button
                v-if="!loading"
                label="Re-identify"
                icon="pi pi-refresh"
                text
                v-tooltip.top="'Ignore the cached answer and fingerprint these files again'"
                data-test="identify-reidentify"
                @click="emit('reidentify')"
            />
            <Button
                v-if="!loading"
                :label="`Stage ${acceptedCount} track${acceptedCount === 1 ? '' : 's'}`"
                icon="pi pi-check"
                data-test="identify-apply"
                :disabled="acceptedCount === 0 || selectedFields.length === 0"
                @click="apply"
            />
        </template>
    </Dialog>
</template>

<style scoped>
.identify-loading {
    display: flex;
    align-items: flex-start;
    gap: 0.9rem;
    padding: 2rem 0.5rem;
}
.identify-loading .pi-spinner {
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
/* A fixed-height dialog only pays off if the body actually fills it: make the
   PrimeVue content region a flex column so the row list absorbs the leftover
   height and scrolls internally, instead of the whole dialog overflowing. Needs
   :deep() because these are PrimeVue's own elements, not ours. */
.song-identify-dialog:deep(.p-dialog-content) {
    display: flex;
    flex-direction: column;
    min-height: 0;
    flex: 1;
    overflow: hidden;
}
/* One row per file, styled as the album dialog's track table: a shared grid
   template for the header and every row, uppercase column labels, zebra striping,
   and the same current-vs-target column pairs. Kept as a local copy of the
   pattern rather than extracted — the two tables share a look, not a data shape
   (no disc grouping or tracklist gaps here; a recording/release pair instead of a
   position dropdown). */
.track-list {
    /* accept · file · title · artist · genres · recording · release. Only the
       target side of each pair gets a column — the file's own title, artist and
       genre tags live in the target cell's "Currently: …" tooltip, which keeps the
       table narrow enough for the two choice columns to show their long labels
       (the recording label carries the match score). Genres get the narrowest
       target column: a release group carries a handful of short names. */
    --identify-track-cols: 2.2rem minmax(0, 1.2fr) minmax(0, 1.2fr) minmax(0, 1fr)
        minmax(0, 0.9fr) minmax(0, 1.5fr) minmax(0, 1.5fr);
    display: flex;
    flex-direction: column;
    /* Takes the height the field row leaves over and scrolls on its own, so the
       "Stage fields" checkboxes stay pinned while the results scroll under them. */
    flex: 1;
    min-height: 0;
    overflow-y: auto;
}
.track-list-header {
    display: grid;
    grid-template-columns: var(--identify-track-cols);
    column-gap: 0.75rem;
    padding: 0 0.5rem 0.4rem;
    border-bottom: 1px solid var(--app-border);
    margin-bottom: 0.25rem;
    font-size: 0.75rem;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
    color: var(--app-text-secondary);
    /* The header stays put while a long result list scrolls under it. */
    position: sticky;
    top: 0;
    background: var(--app-surface, #fff);
    z-index: 1;
}
.identify-track-row {
    display: grid;
    grid-template-columns: var(--identify-track-cols);
    align-items: center;
    column-gap: 0.75rem;
    width: 100%;
    min-height: 56px;
    padding: 0 0.5rem;
    box-sizing: border-box;
}
.identify-track-row.striped {
    background-color: rgba(0, 0, 0, 0.025);
}
.identify-track-row:hover {
    background-color: var(--app-hover);
}
/* A file identification could not place at all. Inert and greyed: there is
   nothing to accept or re-point, so it reads as a hole in the batch rather than a
   row the user forgot to tick. */
.identify-track-row.unmatched {
    opacity: 0.55;
}
.identify-track-row.unmatched:hover {
    background-color: transparent;
}
.identify-track-row.unmatched.striped:hover {
    background-color: rgba(0, 0, 0, 0.025);
}
/* An unaccepted row stages nothing; dim it so the accepted set reads clearly. */
.identify-track-row.excluded .col-current,
.identify-track-row.excluded .col-target {
    opacity: 0.5;
}
/* The File column (muted — it is context) and the target columns (emphasised,
   since that is what a save writes) share a layout. */
.col-current,
.col-target {
    display: flex;
    flex-direction: column;
    min-width: 0;
}
/* The File cell is a label for the row's accept checkbox, so it clicks. */
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
/* A value that actually differs from the file's own: emphasised, and the only one
   that carries the "Currently: …" tooltip. --app-staged is the editor's
   established colour for a pending tag change (see EditPanel's .field-dirty
   rows), which is exactly what this is. */
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
.row-nomatch {
    font-size: 0.75rem;
    color: var(--app-text-secondary);
}
.row-choice {
    /* The grid column already sizes these; keep them from overflowing on a long
       recording or release label. */
    min-width: 0;
    max-width: 100%;
}
.cell-release-single {
    display: flex;
    flex-direction: column;
    min-width: 0;
    font-size: 0.8rem;
    color: var(--app-text-secondary);
}
</style>
