import { computed, ref, watch, type ComputedRef, type Ref, type WritableComputedRef } from 'vue'
import type { Track, TrackOverlay } from '@/types/metadata'
import { diffInitialValues, distinctArtistMbids, type FieldDiff, type InitialValues } from '@/composables/useMetadataEditor'
import type { EditSession } from '@/composables/useEditSession'
import type { AlbumMatchPayload, ArtistMatchPayload } from '@/types/artists'
import { mergeReleaseTypes, splitReleaseTypes } from '@/lib/releaseTypes'

export type TextKey =
    | 'title' | 'album' | 'mb_recording_id' | 'mb_release_id' | 'mb_release_group_id' | 'disc_subtitle'
export type NumKey = 'year' | 'track_number' | 'disc_number'
type PlaceholderKey = TextKey | NumKey

export type Scope = 'artist' | 'album_artist'
export interface Pair { name: string; mbid: string; mixed: boolean }

// Per-recording fields are only editable on a single track; guarded so a mass
// selection can never stage them even if a set slips through.
const PER_RECORDING = new Set<TextKey | NumKey>(['title', 'mb_recording_id', 'track_number'])

// numBuffer maps a numeric field diff onto the nullable input model: a mixed
// selection and an unset tag (0) both show an empty box, so the placeholder is
// visible in either case.
function numBuffer(d: FieldDiff<number>): number | null {
    if (!d.shared) return null
    return d.value === 0 ? null : d.value
}

export function useEditForm(selection: () => Track[], session: EditSession) {
    const isMass = computed(() => selection().length > 1)
    const paths = computed(() => selection().map((t) => t.path))

    // The editor displays and diffs EFFECTIVE values: original tags plus staged
    // (unsaved) session edits. `originalDiff` keeps the untouched baseline for
    // the overwrite-vs-leave rule and the undo tooltips.
    const effective = computed(() => selection().map((t) => session.effective(t)))
    const diff = computed<InitialValues>(() => diffInitialValues(effective.value))
    const originalDiff = computed<InitialValues>(() => diffInitialValues(selection()))

    function text(key: TextKey): WritableComputedRef<string> {
        return computed({
            get: () => diff.value[key].value,
            set: (v) => {
                if (isMass.value && PER_RECORDING.has(key)) return
                session.stageField(paths.value, key, v)
            }
        })
    }
    function num(key: NumKey): WritableComputedRef<number | null> {
        return computed({
            get: () => numBuffer(diff.value[key]),
            set: (v) => {
                if (isMass.value && PER_RECORDING.has(key)) return
                session.stageField(paths.value, key, v === null ? 0 : v)
            }
        })
    }
    const compilation: WritableComputedRef<boolean> = computed({
        get: () => diff.value.compilation.value,
        set: (v) => session.stageField(paths.value, 'compilation', v)
    })
    const genres: WritableComputedRef<string[]> = computed({
        get: () => (diff.value.genres.shared ? [...diff.value.genres.value] : []),
        set: (list) => {
            const mixed = isMass.value && !originalDiff.value.genres.shared
            if (mixed && list.length === 0) session.unstageField(paths.value, 'genres')
            else session.stageField(paths.value, 'genres', [...list])
        }
    })
    const releaseTypes: WritableComputedRef<string[]> = computed({
        get: () => (diff.value.release_types.shared ? [...diff.value.release_types.value] : []),
        set: (list) => {
            const mixed = isMass.value && !originalDiff.value.release_types.shared
            if (mixed && list.length === 0) session.unstageField(paths.value, 'release_types')
            else session.stageField(paths.value, 'release_types', [...list])
        }
    })
    // The primary (single) and secondary (multi) controls are two views of the
    // one flat release-type list; each setter re-merges through the other's
    // current value so neither clobbers it.
    const primaryReleaseType: WritableComputedRef<string> = computed({
        get: () => splitReleaseTypes(releaseTypes.value).primary,
        set: (p) => {
            releaseTypes.value = mergeReleaseTypes(p, splitReleaseTypes(releaseTypes.value).secondary)
        }
    })
    const secondaryReleaseTypes: WritableComputedRef<string[]> = computed({
        get: () => splitReleaseTypes(releaseTypes.value).secondary,
        set: (s) => {
            releaseTypes.value = mergeReleaseTypes(splitReleaseTypes(releaseTypes.value).primary, s)
        }
    })

    function placeholder(key: PlaceholderKey): ComputedRef<string> {
        return computed(() => (diff.value[key].shared ? '' : '(multiple values)'))
    }

    const genresMixed = computed(() => isMass.value && !diff.value.genres.shared)
    const releaseTypesMixed = computed(() => isMass.value && !diff.value.release_types.shared)
    const compilationMixed = computed(() => isMass.value && !diff.value.compilation.shared)

    const dirtyFields = computed<Set<keyof TrackOverlay>>(() => {
        const s = new Set<keyof TrackOverlay>()
        for (const p of paths.value) {
            const overlay = session.overlays.value.get(p)
            if (overlay) for (const k of Object.keys(overlay)) s.add(k as keyof TrackOverlay)
        }
        return s
    })
    const isDirty = (key: keyof TrackOverlay) => dirtyFields.value.has(key)

    // Reverting a field just unstages it; the computeds reflect the original
    // effective value automatically — no buffer to refresh.
    function undo(key: keyof TrackOverlay) {
        session.unstageField(paths.value, key)
    }

    function undoTooltip(key: keyof InitialValues): string {
        const d = originalDiff.value[key]
        if (!d.shared) return 'Revert to each track\'s own value'
        const v = d.value
        if (typeof v === 'boolean') return `Revert to ${v ? 'yes' : 'no'}`
        if (v === '' || v === 0) return 'Revert to empty'
        return `Revert to "${v}"`
    }
    function undoGenresTooltip(): string {
        const d = originalDiff.value.genres
        if (!d.shared) return 'Revert to each track\'s own value'
        if (d.value.length === 0) return 'Revert to empty'
        return `Revert to "${d.value.join(', ')}"`
    }
    function undoReleaseTypesTooltip(): string {
        const d = originalDiff.value.release_types
        if (!d.shared) return 'Revert to each track\'s own value'
        if (d.value.length === 0) return 'Revert to empty'
        return `Revert to "${d.value.join(', ')}"`
    }

    // ----- Artist / album-artist pairs -----
    // Pairs need local state: the editor allows in-progress rows (empty name,
    // unpicked MBID) the session's normalized credit list can't represent. We
    // reconcile them from a DERIVED computed and reseed only when it actually
    // changes — so an external identify reseeds, but local typing (which stages
    // and makes the derived value match the rows) never clobbers the edit.
    const artistRows = ref<Pair[]>([])
    const albumArtistRows = ref<Pair[]>([])

    const creditsKey = (scope: Scope): 'artists' | 'album_artists' =>
        scope === 'artist' ? 'artists' : 'album_artists'
    const rowsFor = (scope: Scope): Ref<Pair[]> =>
        scope === 'artist' ? artistRows : albumArtistRows

    function deriveRows(scope: Scope): Pair[] {
        const key = creditsKey(scope)
        if (!diff.value[key].shared) return []
        const nameField = scope === 'artist' ? 'artists' : 'album_artists'
        const idField = scope === 'artist' ? 'mb_artist_ids' : 'mb_album_artist_ids'
        return distinctArtistMbids(effective.value, nameField, idField).map(
            (r): Pair => ({ name: r.name, mbid: r.mbid, mixed: r.mixed })
        )
    }
    const derivedArtistRows = computed(() => deriveRows('artist'))
    const derivedAlbumArtistRows = computed(() => deriveRows('album_artist'))

    function sameRows(a: Pair[], b: Pair[]): boolean {
        return a.length === b.length && a.every((p, i) => p.name === b[i].name && p.mbid === b[i].mbid)
    }
    // Reseed only when the derived rows differ from the current rows: local
    // edits stage onto the session so the derived value matches (no reseed);
    // external identify / undo / a selection change makes them differ (reseed).
    watch(derivedArtistRows, (d) => { if (!sameRows(artistRows.value, d)) artistRows.value = d }, { immediate: true })
    watch(derivedAlbumArtistRows, (d) => { if (!sameRows(albumArtistRows.value, d)) albumArtistRows.value = d }, { immediate: true })

    // stagePairs stages the full credit list of a scope onto every selected
    // track. Left empty over an originally-mixed selection it stages nothing
    // (keeps each track's own artists), mirroring the pre-session semantics.
    function stagePairs(scope: Scope) {
        const key = creditsKey(scope)
        const rows = rowsFor(scope).value
        const originallyMixed = isMass.value && !originalDiff.value[key].shared
        if (originallyMixed && rows.length === 0) {
            session.unstageField(paths.value, key)
            return
        }
        session.stageField(paths.value, key, rows.map((p) => ({ name: p.name, mbid: p.mbid })))
    }
    function addPair(scope: Scope) {
        // Known/accepted: adding multiple blank rows before typing collapses to one
        // on reconcile (distinctArtistMbids dedups empty names) — intentional.
        rowsFor(scope).value.push({ name: '', mbid: '', mixed: false })
        stagePairs(scope)
    }
    function removePair(scope: Scope, index: number) {
        rowsFor(scope).value.splice(index, 1)
        stagePairs(scope)
    }
    function undoPairs(scope: Scope) {
        session.unstageField(paths.value, creditsKey(scope))
        // The derived rows now revert; force the local rows to match at once so
        // the editor reflects the undo without waiting for the watcher's tick.
        rowsFor(scope).value = deriveRows(scope)
    }
    function undoPairsTooltip(scope: Scope): string {
        const d = originalDiff.value[creditsKey(scope)]
        if (!d.shared) return 'Revert to each track\'s own value'
        if (d.value.length === 0) return 'Revert to empty'
        return `Revert to "${d.value.join(', ')}"`
    }
    const artistsMixed = computed(() => isMass.value && !diff.value.artists.shared)
    const albumArtistsMixed = computed(() => isMass.value && !diff.value.album_artists.shared)

    // The MusicBrainz artist picker edits one existing row in place, then stages.
    function applyArtistPick(scope: Scope, index: number, payload: ArtistMatchPayload) {
        const pair = rowsFor(scope).value[index]
        if (!pair) return
        if (payload.mbid !== undefined) pair.mbid = payload.mbid
        if (payload.name !== undefined) pair.name = payload.name
        stagePairs(scope)
    }
    // The album picker replaces the whole album-artist list; assign the rows
    // directly (form-local) so the reconcile watcher sees them as current.
    function applyAlbumArtists(payload: AlbumMatchPayload) {
        if (payload.albumArtists === undefined) return
        albumArtistRows.value = payload.albumArtists.map((a) => ({ name: a.name, mbid: a.mbid, mixed: false }))
        stagePairs('album_artist')
    }

    return {
        isMass, paths, effective, diff, originalDiff,
        text, num, compilation, genres, placeholder,
        releaseTypes, primaryReleaseType, secondaryReleaseTypes,
        genresMixed, releaseTypesMixed, compilationMixed,
        dirtyFields, isDirty, undo, undoTooltip, undoGenresTooltip, undoReleaseTypesTooltip,
        artistRows, albumArtistRows,
        addPair, removePair, stagePairs, undoPairs, undoPairsTooltip,
        artistsMixed, albumArtistsMixed,
        applyArtistPick, applyAlbumArtists
    }
}
