import { computed, ref, watch } from 'vue'
import { useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import {
    invalidateAfterMetadataWrite,
    updateTracksPartitioned,
    useApplyPicture,
    useDeletePicture
} from '@/composables/useMetadataEditor'
import { albumKey, dirOf } from '@/lib/albumIdentity'
import { apiErrorMessage } from '@/lib/apiError'
import type {
    ArtistCredit,
    IdentifyCandidate,
    IdentifyRelease,
    PatchFields,
    PictureSlot,
    RescanStatus,
    StagedPictureSource,
    Track,
    TrackOverlay,
    UpdateResult
} from '@/types/metadata'

// ----- Pure helpers (exported for unit tests) -----

// credits builds the aligned name/MBID credit list a track currently carries.
export function trackCredits(track: Track, scope: 'artists' | 'album_artists'): ArtistCredit[] {
    const names = track[scope]
    const ids = scope === 'artists' ? track.mb_artist_ids : track.mb_album_artist_ids
    return names.map((name, i) => ({ name, mbid: ids[i] ?? '' }))
}

function sameCredits(a: ArtistCredit[], b: ArtistCredit[]): boolean {
    return a.length === b.length && a.every((c, i) => c.name === b[i].name && c.mbid === b[i].mbid)
}

function sameStrings(a: string[], b: string[]): boolean {
    return a.length === b.length && a.every((v, i) => v === b[i])
}

// originalValueEquals reports whether a staged value matches the track's
// original, in which case the field needs no overlay entry.
export function originalValueEquals<K extends keyof TrackOverlay>(
    track: Track,
    key: K,
    value: TrackOverlay[K]
): boolean {
    if (key === 'artists' || key === 'album_artists') {
        return sameCredits(value as ArtistCredit[], trackCredits(track, key))
    }
    if (key === 'genres') {
        return sameStrings(value as string[], track.genres)
    }
    return track[key as keyof Track] === value
}

/**
 * applyOverlay merges a track's staged edits onto its original values,
 * producing the "effective" track the editor displays. Artist credit lists
 * are expanded back into the aligned name/MB-ID pairs of the Track shape.
 */
export function applyOverlay(track: Track, overlay: TrackOverlay | undefined): Track {
    if (!overlay) return track
    const out: Track = { ...track }
    if (overlay.title !== undefined) out.title = overlay.title
    if (overlay.album !== undefined) out.album = overlay.album
    if (overlay.mb_recording_id !== undefined) out.mb_recording_id = overlay.mb_recording_id
    if (overlay.mb_release_id !== undefined) out.mb_release_id = overlay.mb_release_id
    if (overlay.mb_release_group_id !== undefined)
        out.mb_release_group_id = overlay.mb_release_group_id
    if (overlay.genres !== undefined) out.genres = [...overlay.genres]
    if (overlay.year !== undefined) out.year = overlay.year
    if (overlay.track_number !== undefined) out.track_number = overlay.track_number
    if (overlay.disc_number !== undefined) out.disc_number = overlay.disc_number
    if (overlay.disc_subtitle !== undefined) out.disc_subtitle = overlay.disc_subtitle
    if (overlay.compilation !== undefined) out.compilation = overlay.compilation
    if (overlay.artists !== undefined) {
        out.artists = overlay.artists.map((c) => c.name)
        out.mb_artist_ids = overlay.artists.map((c) => c.mbid)
    }
    if (overlay.album_artists !== undefined) {
        out.album_artists = overlay.album_artists.map((c) => c.name)
        out.mb_album_artist_ids = overlay.album_artists.map((c) => c.mbid)
    }
    return out
}

// creditPatch writes an artist credit-list edit into a PatchFields. The server
// refuses a request that both renames artists and sets their MB IDs (the
// name-keyed map would misalign), so: when names changed we send the new name
// list plus a complete name->ID map (the caller later splits them into two
// sequential PUTs), and when names are untouched we send only the IDs that
// actually changed so unrelated artists keep theirs.
function creditPatch(
    out: PatchFields,
    scope: 'artists' | 'album_artists',
    staged: ArtistCredit[],
    original: ArtistCredit[]
) {
    const mbidKey = scope === 'artists' ? 'artist_mbids' : 'album_artist_mbids'
    const names = staged.map((c) => c.name.trim()).filter((n) => n !== '')
    const originalNames = original.map((c) => c.name)
    const namesChanged =
        names.length !== originalNames.length || names.some((n, i) => n !== originalNames[i])
    if (namesChanged) {
        out[scope] = names
        const map: Record<string, string> = {}
        for (const c of staged) {
            const n = c.name.trim()
            if (n) map[n] = c.mbid
        }
        if (Object.keys(map).length > 0) out[mbidKey] = map
    } else {
        const orig = new Map(original.map((c) => [c.name, c.mbid]))
        const map: Record<string, string> = {}
        for (const c of staged) {
            const n = c.name.trim()
            if (n && orig.get(n) !== c.mbid) map[n] = c.mbid
        }
        if (Object.keys(map).length > 0) out[mbidKey] = map
    }
}

/**
 * buildTrackPatch converts one track's overlay into the PatchFields the update
 * endpoint expects. Returns an empty object when the overlay stages nothing
 * effective (e.g. an artist list identical to the original).
 */
export function buildTrackPatch(original: Track, overlay: TrackOverlay): PatchFields {
    const out: PatchFields = {}
    if (overlay.title !== undefined) out.title = overlay.title
    if (overlay.album !== undefined) out.album = overlay.album
    if (overlay.mb_recording_id !== undefined) out.mb_recording_id = overlay.mb_recording_id
    if (overlay.mb_release_id !== undefined) out.mb_release_id = overlay.mb_release_id
    if (overlay.mb_release_group_id !== undefined)
        out.mb_release_group_id = overlay.mb_release_group_id
    if (overlay.genres !== undefined) {
        out.genres = overlay.genres.map((g) => g.trim()).filter((g) => g !== '')
    }
    if (overlay.year !== undefined) out.year = overlay.year
    if (overlay.track_number !== undefined) out.track_number = overlay.track_number
    if (overlay.disc_number !== undefined) out.disc_number = overlay.disc_number
    if (overlay.disc_subtitle !== undefined) out.disc_subtitle = overlay.disc_subtitle
    if (overlay.compilation !== undefined) out.compilation = overlay.compilation
    if (overlay.artists !== undefined) {
        creditPatch(out, 'artists', overlay.artists, trackCredits(original, 'artists'))
    }
    if (overlay.album_artists !== undefined) {
        creditPatch(
            out,
            'album_artists',
            overlay.album_artists,
            trackCredits(original, 'album_artists')
        )
    }
    if (overlay.raw !== undefined && Object.keys(overlay.raw).length > 0) {
        // Empty-array entries travel too: they delete the key on the server.
        out.raw_tags = { ...overlay.raw }
    }
    if (overlay.removeUnsupported !== undefined && overlay.removeUnsupported.length > 0) {
        // Sorted so tracks staging the same set group into one batch.
        out.remove_unsupported = [...overlay.removeUnsupported].sort()
    }
    return out
}

export interface PatchBatch {
    fields: PatchFields
    paths: string[]
}

// canonical returns a key-order-independent JSON string of a patch, so tracks
// whose patches are identical group into one request.
function canonical(value: unknown): string {
    if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`
    if (value !== null && typeof value === 'object') {
        const keys = Object.keys(value as object).sort()
        return `{${keys
            .map((k) => `${JSON.stringify(k)}:${canonical((value as Record<string, unknown>)[k])}`)
            .join(',')}}`
    }
    return JSON.stringify(value)
}

/**
 * groupPatches folds per-track overlays into the minimal set of update
 * batches: tracks sharing an identical patch are sent in one request. Tracks
 * whose patch comes out empty are skipped.
 */
export function groupPatches(
    originals: Map<string, Track>,
    overlays: Map<string, TrackOverlay>
): PatchBatch[] {
    const groups = new Map<string, PatchBatch>()
    for (const [path, overlay] of overlays) {
        const original = originals.get(path)
        if (!original) continue
        const fields = buildTrackPatch(original, overlay)
        if (Object.keys(fields).length === 0) continue
        const key = canonical(fields)
        const existing = groups.get(key)
        if (existing) existing.paths.push(path)
        else groups.set(key, { fields, paths: [path] })
    }
    return [...groups.values()]
}

/**
 * candidateToOverlay converts an accepted identify candidate (and the release
 * the user picked, if any) into staged field values.
 *
 * `genres` comes from the caller rather than from the candidate: the fingerprint
 * match carries no genres, so the dialog looks them up for the picked release's
 * release group and passes the answer in. An empty list stages nothing — staging
 * [] would wipe genres the file already has.
 */
export function candidateToOverlay(
    candidate: IdentifyCandidate,
    release: IdentifyRelease | null,
    genres: string[] = []
): TrackOverlay {
    const out: TrackOverlay = {
        title: candidate.title,
        mb_recording_id: candidate.recording_mbid
    }
    if (candidate.artists.length > 0) {
        out.artists = candidate.artists.map((a) => ({ name: a.name, mbid: a.mbid }))
    }
    if (release) {
        out.album = release.album
        out.mb_release_id = release.release_mbid
        out.mb_release_group_id = release.release_group_mbid
        if (release.year > 0) out.year = release.year
        if (release.track_number > 0) out.track_number = release.track_number
        if (release.disc_number > 0) out.disc_number = release.disc_number
    }
    if (genres.length > 0) out.genres = [...genres]
    return out
}

/**
 * albumPickToOverlay converts an accepted album identify pick into staged field
 * values. Stages the album-level fields on every accepted song, plus the song's
 * own recording fields when a position was resolved. Compilation and disc
 * subtitle are deliberately left alone: identification says nothing reliable
 * about them.
 *
 * `genres` comes from the caller for the same reason as in candidateToOverlay:
 * the identify response carries no genres, so the dialog looks them up for the
 * chosen option's release group. An empty list stages nothing rather than
 * clearing the files' existing genres.
 */
export function albumPickToOverlay(
    pick: import('@/types/metadata').AlbumIdentifyPick,
    genres: string[] = []
): TrackOverlay {
    const { option, assignment } = pick
    const out: TrackOverlay = {}
    if (option.album !== '') out.album = option.album
    if (option.release_mbid !== '') out.mb_release_id = option.release_mbid
    if (option.release_group_mbid !== '') out.mb_release_group_id = option.release_group_mbid
    if (option.year > 0) out.year = option.year
    // `?? []` despite the type promising an array: a missing credit list must
    // stage no artists, never abort the whole apply and lose the user's picks.
    const albumArtists = option.artists ?? []
    if (albumArtists.length > 0) {
        out.album_artists = albumArtists.map((a) => ({ name: a.name, mbid: a.mbid }))
    }
    if (genres.length > 0) out.genres = [...genres]
    if (assignment) {
        if (assignment.title !== '') out.title = assignment.title
        if (assignment.recording_mbid !== '') out.mb_recording_id = assignment.recording_mbid
        const trackArtists = assignment.artists ?? []
        if (trackArtists.length > 0) {
            out.artists = trackArtists.map((a) => ({ name: a.name, mbid: a.mbid }))
        }
        if (assignment.track_number > 0) out.track_number = assignment.track_number
        if (assignment.disc_number > 0) out.disc_number = assignment.disc_number
    }
    return out
}

// ----- Session state -----

// PictureOp is one staged change to a picture type+slot cell: set a new image
// (from a local file or a Cover Art Archive URL, with a preview URL for the
// editor) or remove the cell's current image. `paths` are the tracks selected
// when the op was staged — the exact files an embedded op reads/writes, and for
// a folder op the set of directories the art is written into (an album can span
// several: a multi-disc release laid out as CD 1/, CD 2/ subfolders).
export type PictureOp =
    | {
          kind: 'set'
          file: File | null
          imageUrl: string | null
          preview: string | null
          paths: string[]
      }
    | { kind: 'remove'; paths: string[] }

// PictureSessionEntry is the pending picture work for one album (keyed by
// albumKey, not by directory): one op per type+slot cell, each carrying its own
// target paths.
export interface PictureSessionEntry {
    ops: Map<string, Map<PictureSlot, PictureOp>>
}

export type EditSession = ReturnType<typeof useEditSession>

/**
 * useEditSession holds one folder-scoped editing session: per-track staged
 * field overlays plus per-album staged cover changes, none persisted until
 * save(). Instantiate once in the metadata editor view.
 */
export function useEditSession(tracks: () => Track[] | undefined, libraryId: () => number | null) {
    const qc = useQueryClient()
    const toast = useToast()
    // The session drives many picture ops per save and raises one aggregate
    // "index not updated" warning itself, so the mutations must stay quiet
    // about it — otherwise a 6-cell save would stack 6 identical toasts.
    const applyPictureMutation = useApplyPicture({ quietRescanWarning: true })
    const deletePictureMutation = useDeletePicture({ quietRescanWarning: true })

    const overlays = ref(new Map<string, TrackOverlay>())
    const pictures = ref(new Map<string, PictureSessionEntry>())
    const isSaving = ref(false)
    // Bumped when a save wrote picture changes, so thumbnails cache-bust.
    const picturesSavedAt = ref(0)

    const originals = computed(() => {
        const map = new Map<string, Track>()
        for (const t of tracks() ?? []) map.set(t.path, t)
        return map
    })

    // Files can disappear or change under the session on a reload; drop
    // overlays whose path no longer exists so stale edits can't be saved.
    // Picture entries are keyed by album: drop those whose album no longer has
    // any listed track.
    watch(originals, (fresh) => {
        if (tracks() === undefined) return
        for (const path of overlays.value.keys()) {
            if (!fresh.has(path)) overlays.value.delete(path)
        }
        const freshAlbums = new Set([...fresh.values()].map(albumKey))
        for (const key of pictures.value.keys()) {
            if (!freshAlbums.has(key)) {
                const entry = pictures.value.get(key)!
                for (const slots of entry.ops.values()) {
                    for (const op of slots.values()) releaseOpPreview(op)
                }
                pictures.value.delete(key)
            }
        }
    })

    // stagedPaths drives the per-track unsaved indicators: a track is staged
    // when it has field overlays OR pending picture work. Embedded picture ops
    // touch exactly the tracks they were staged for; folder/db ops belong to
    // the whole album, so every track of it is flagged — across all the
    // directories a multi-disc album spans.
    const stagedPaths = computed<ReadonlySet<string>>(() => {
        const out = new Set(overlays.value.keys())
        for (const [key, entry] of pictures.value) {
            let wholeAlbum = false
            for (const slots of entry.ops.values()) {
                for (const [slot, op] of slots) {
                    if (slot === 'embedded') for (const p of op.paths) out.add(p)
                    else wholeAlbum = true
                }
            }
            if (wholeAlbum) {
                for (const track of originals.value.values()) {
                    if (albumKey(track) === key) out.add(track.path)
                }
            }
        }
        return out
    })
    const hasStagedChanges = computed(() => overlays.value.size > 0 || pictures.value.size > 0)

    function effective(track: Track): Track {
        return applyOverlay(track, overlays.value.get(track.path))
    }

    function stageField<K extends keyof TrackOverlay>(
        paths: string[],
        key: K,
        value: TrackOverlay[K]
    ) {
        for (const path of paths) {
            const original = originals.value.get(path)
            if (!original) continue
            const overlay = overlays.value.get(path) ?? {}
            if (originalValueEquals(original, key, value)) {
                delete overlay[key]
            } else {
                overlay[key] = value
            }
            if (Object.keys(overlay).length === 0) overlays.value.delete(path)
            else overlays.value.set(path, overlay)
        }
    }

    function unstageField(paths: string[], key: keyof TrackOverlay) {
        for (const path of paths) {
            const overlay = overlays.value.get(path)
            if (!overlay) continue
            delete overlay[key]
            if (Object.keys(overlay).length === 0) overlays.value.delete(path)
        }
    }

    function isFieldStaged(paths: string[], key: keyof TrackOverlay): boolean {
        return paths.some((p) => overlays.value.get(p)?.[key] !== undefined)
    }

    // ----- Raw tag staging -----
    // Raw edits nest one level deeper than form fields: the overlay's `raw`
    // map holds key -> values, where an empty array stages a delete. The
    // caller passes each track's ORIGINAL values for the key (from the raw
    // read endpoint) so staging the original back auto-clears, mirroring
    // stageField.

    function sameValues(a: string[], b: string[]): boolean {
        return a.length === b.length && a.every((v, i) => v === b[i])
    }

    function stageRawKey(
        paths: string[],
        key: string,
        values: string[],
        originalsByPath: Map<string, string[]>
    ) {
        for (const path of paths) {
            if (!originals.value.has(path)) continue
            const overlay = overlays.value.get(path) ?? {}
            const raw = { ...(overlay.raw ?? {}) }
            const original = originalsByPath.get(path) ?? []
            if (sameValues(values, original)) {
                delete raw[key]
            } else {
                raw[key] = values
            }
            if (Object.keys(raw).length === 0) delete overlay.raw
            else overlay.raw = raw
            if (Object.keys(overlay).length === 0) overlays.value.delete(path)
            else overlays.value.set(path, overlay)
        }
    }

    function unstageRawKey(paths: string[], key: string) {
        for (const path of paths) {
            const overlay = overlays.value.get(path)
            if (!overlay?.raw) continue
            const raw = { ...overlay.raw }
            delete raw[key]
            if (Object.keys(raw).length === 0) delete overlay.raw
            else overlay.raw = raw
            if (Object.keys(overlay).length === 0) overlays.value.delete(path)
        }
    }

    function stagedRawValue(path: string, key: string): string[] | undefined {
        return overlays.value.get(path)?.raw?.[key]
    }

    function isRawKeyStaged(paths: string[], key: string): boolean {
        return paths.some((p) => stagedRawValue(p, key) !== undefined)
    }

    // ----- Hidden-frame (unsupported data) staging -----
    // Descriptors come from the raw read endpoint per track. Staging toggles a
    // descriptor into the overlay's removeUnsupported list for every selected
    // track that actually carries it (availableByPath filters the rest).

    function stageUnsupportedRemoval(
        paths: string[],
        descriptor: string,
        availableByPath: Map<string, string[]>
    ) {
        for (const path of paths) {
            if (!originals.value.has(path)) continue
            if (!(availableByPath.get(path) ?? []).includes(descriptor)) continue
            const overlay = overlays.value.get(path) ?? {}
            const staged = overlay.removeUnsupported ?? []
            if (!staged.includes(descriptor)) {
                overlay.removeUnsupported = [...staged, descriptor]
            }
            overlays.value.set(path, overlay)
        }
    }

    function unstageUnsupportedRemoval(paths: string[], descriptor: string) {
        for (const path of paths) {
            const overlay = overlays.value.get(path)
            if (!overlay?.removeUnsupported) continue
            const staged = overlay.removeUnsupported.filter((d) => d !== descriptor)
            if (staged.length === 0) delete overlay.removeUnsupported
            else overlay.removeUnsupported = staged
            if (Object.keys(overlay).length === 0) overlays.value.delete(path)
        }
    }

    function isUnsupportedRemovalStaged(paths: string[], descriptor: string): boolean {
        return paths.some((p) => overlays.value.get(p)?.removeUnsupported?.includes(descriptor))
    }

    // stageOverlays merges identify picks (or any bulk edit) onto existing
    // overlays, applying the same equals-original normalization per field.
    function stageOverlays(entries: Map<string, TrackOverlay>) {
        for (const [path, overlay] of entries) {
            for (const key of Object.keys(overlay) as (keyof TrackOverlay)[]) {
                stageField([path], key, overlay[key])
            }
        }
    }

    // ----- Pictures -----

    function pictureEntry(album: string): PictureSessionEntry {
        let entry = pictures.value.get(album)
        if (!entry) {
            entry = { ops: new Map() }
            pictures.value.set(album, entry)
        }
        return entry
    }

    function prunePictureEntry(album: string) {
        const entry = pictures.value.get(album)
        if (!entry) return
        for (const [type, slots] of entry.ops) {
            if (slots.size === 0) entry.ops.delete(type)
        }
        if (entry.ops.size === 0) pictures.value.delete(album)
    }

    function releaseOpPreview(op: PictureOp | undefined) {
        if (op?.kind === 'set' && op.preview?.startsWith('blob:')) {
            URL.revokeObjectURL(op.preview)
        }
    }

    // setOp stores one op for a type+slot cell, replacing (and cleaning up)
    // whatever op the cell held before — a set overwrites a remove and vice versa.
    function setOp(album: string, type: string, slot: PictureSlot, op: PictureOp) {
        const entry = pictureEntry(album)
        let slots = entry.ops.get(type)
        if (!slots) {
            slots = new Map()
            entry.ops.set(type, slots)
        }
        releaseOpPreview(slots.get(slot))
        slots.set(slot, op)
    }

    function stagePictureSet(
        album: string,
        type: string,
        slot: PictureSlot,
        src: StagedPictureSource,
        paths: string[]
    ) {
        setOp(album, type, slot, {
            kind: 'set',
            file: src.file,
            imageUrl: src.imageUrl,
            preview: src.file ? URL.createObjectURL(src.file) : src.imageUrl,
            paths
        })
    }

    function stagePictureRemoval(album: string, type: string, slot: PictureSlot, paths: string[]) {
        setOp(album, type, slot, { kind: 'remove', paths })
    }

    function discardPictureOp(album: string, type: string, slot: PictureSlot) {
        const entry = pictures.value.get(album)
        const slots = entry?.ops.get(type)
        if (!entry || !slots) return
        releaseOpPreview(slots.get(slot))
        slots.delete(slot)
        prunePictureEntry(album)
    }

    function getPictureOps(album: string): PictureSessionEntry | undefined {
        return pictures.value.get(album)
    }

    function getPictureOp(album: string, type: string, slot: PictureSlot): PictureOp | undefined {
        return pictures.value.get(album)?.ops.get(type)?.get(slot)
    }

    function discardAll() {
        overlays.value.clear()
        for (const entry of pictures.value.values()) {
            for (const slots of entry.ops.values()) {
                for (const op of slots.values()) releaseOpPreview(op)
            }
        }
        pictures.value.clear()
    }

    // ----- Save -----

    // One savePictures run: whether every staged op was written, plus the last
    // re-index failure any of them reported (null = the index is current).
    interface SavePicturesOutcome {
        ok: boolean
        rescanFailure: string | null
    }

    // savePictures persists all staged picture ops. ok is false to abort the
    // save on the first failure (the mutations show their own error toasts).
    // A failed re-index is not a write failure — the image is on disk — so it
    // does not abort; it is reported alongside the tag batches' failures, with
    // the same "last failure wins, never cleared by a later success" rule
    // save() uses.
    async function savePictures(): Promise<SavePicturesOutcome> {
        const lib = libraryId()
        if (lib === null) {
            return { ok: pictures.value.size === 0, rescanFailure: null }
        }
        let wrote = false
        let rescanFailure: string | null = null
        for (const [key, entry] of pictures.value) {
            for (const [type, slots] of entry.ops) {
                for (const [slot, op] of [...slots]) {
                    try {
                        let out: { rescan?: RescanStatus } | undefined
                        if (op.kind === 'set') {
                            const form = new FormData()
                            form.append('library_id', String(lib))
                            form.append('target', slot)
                            form.append('type', type)
                            for (const p of op.paths) form.append('paths', p)
                            if (op.file) form.append('image', op.file)
                            else if (op.imageUrl) form.append('image_url', op.imageUrl)
                            out = await applyPictureMutation.mutateAsync(form)
                        } else {
                            out = await deletePictureMutation.mutateAsync({
                                libraryId: lib,
                                // The entry key identifies the album, not a
                                // location: the server needs a real folder.
                                path: dirOf(op.paths[0] ?? ''),
                                type,
                                slot,
                                // Both slots need the staged files: embedded
                                // removal applies to them directly, and a folder
                                // removal needs them to reach every directory the
                                // album spans.
                                paths: op.paths
                            })
                        }
                        if (out?.rescan && !out.rescan.ok) {
                            rescanFailure = out.rescan.error ?? 'unknown error'
                        }
                        releaseOpPreview(op)
                        slots.delete(slot)
                        wrote = true
                    } catch {
                        if (wrote) picturesSavedAt.value = Date.now()
                        return { ok: false, rescanFailure }
                    }
                }
            }
            prunePictureEntry(key)
        }
        if (wrote) picturesSavedAt.value = Date.now()
        return { ok: true, rescanFailure }
    }

    // reportRescanFailure warns that the write landed on disk but the library
    // index did not catch up. Deliberately reports the LAST failure of the save
    // and never clears it because a later batch succeeded: a stale index for
    // part of the selection is still a stale index.
    function reportRescanFailure(rescanFailure: string | null) {
        if (rescanFailure === null) return
        toast.add({
            severity: 'warn',
            summary: 'Saved, but the library index was not updated',
            detail: rescanFailure,
            life: 8000
        })
    }

    async function save() {
        const lib = libraryId()
        if (isSaving.value || lib === null) return
        isSaving.value = true
        try {
            const pics = await savePictures()
            // The picture writes carry their own re-index report. Seed the
            // session's failure with it so it is not lost on either exit path
            // below: the images are on disk regardless, only the index lags.
            let rescanFailure: string | null = pics.rescanFailure
            if (!pics.ok) {
                reportRescanFailure(rescanFailure)
                return
            }

            const batches = groupPatches(originals.value, overlays.value)
            if (batches.length === 0) {
                // Overlays may exist whose patch is a no-op; nothing to write.
                overlays.value.clear()
                reportRescanFailure(rescanFailure)
                return
            }
            const results: UpdateResult[] = []
            let transportError: unknown = null
            for (const batch of batches) {
                try {
                    // Sequential on purpose: the server writes tags into files
                    // and the batches may touch the same directories.
                    const out = await updateTracksPartitioned({
                        library_id: lib,
                        paths: batch.paths,
                        fields: batch.fields
                    })
                    results.push(...out.results)
                    // Report the last re-index failure; the tags are written
                    // either way, only the library index lags.
                    if (out.rescan && !out.rescan.ok) {
                        rescanFailure = out.rescan.error ?? 'unknown error'
                    }
                } catch (err) {
                    // A transport-level failure likely affects the remaining
                    // batches too; stop and report what completed.
                    transportError = err
                    break
                }
            }
            for (const r of results) {
                if (r.ok) overlays.value.delete(r.path)
            }
            invalidateAfterMetadataWrite(qc)

            reportRescanFailure(rescanFailure)

            const ok = results.filter((r) => r.ok).length
            const failed = results.length - ok
            if (transportError !== null) {
                const err = transportError as any
                toast.add({
                    severity: 'error',
                    summary: `Failed to save metadata (${ok} track${ok === 1 ? '' : 's'} saved before the error)`,
                    detail: apiErrorMessage(err),
                    life: 8000
                })
            } else if (failed === 0) {
                toast.add({
                    severity: 'success',
                    summary: `${ok} of ${results.length} saved`,
                    life: 3000
                })
            } else {
                toast.add({
                    severity: 'warn',
                    summary: `${ok} of ${results.length} saved, ${failed} failed`,
                    detail: results
                        .filter((r) => !r.ok)
                        .map((r) => `${r.path}: ${r.error}`)
                        .join('\n'),
                    life: 8000
                })
            }
        } finally {
            isSaving.value = false
        }
    }

    return {
        overlays,
        stagedPaths,
        hasStagedChanges,
        isSaving,
        picturesSavedAt,
        effective,
        stageField,
        unstageField,
        isFieldStaged,
        stageRawKey,
        unstageRawKey,
        stagedRawValue,
        isRawKeyStaged,
        stageUnsupportedRemoval,
        unstageUnsupportedRemoval,
        isUnsupportedRemovalStaged,
        stageOverlays,
        stagePictureSet,
        stagePictureRemoval,
        discardPictureOp,
        getPictureOps,
        getPictureOp,
        discardAll,
        save
    }
}
