import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import type { QueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as MetadataApi from '@/lib/api/Metadata'
import { apiErrorMessage } from '@/lib/apiError'
import type {
    Folder,
    IdentifyRequest,
    MetadataCapabilities,
    PatchFields,
    PictureSlot,
    RescanStatus,
    Track,
    UpdateResult,
    UpdateTracksRequest
} from '@/types/metadata'

// Query keys
export const metadataQueryKeys = {
    folders: (libraryId: number, path: string) => ['metadata', 'folders', libraryId, path] as const,
    tracks: (libraryId: number, path: string) => ['metadata', 'tracks', libraryId, path] as const
}

// invalidateAfterMetadataWrite drops every cache a tag or picture write can
// stale. The music UI is keyed by DB ids, and editing an album name, album
// artist or release MBID moves the tracks to a *new* album row — the editor
// works in file paths and cannot know which ids changed, so there is no precise
// key set to target. Same precedent as the library mutations. Call it only
// after the response resolves: the server re-indexes synchronously, so a
// resolved write means the DB is current.
export function invalidateAfterMetadataWrite(qc: QueryClient) {
    qc.invalidateQueries({ queryKey: ['metadata', 'tracks'] })
    qc.invalidateQueries({ queryKey: ['metadata', 'raw'] })
    qc.invalidateQueries({ queryKey: ['subsonic'] })
}

export function useFolders(libraryId: () => number | null, path: () => string) {
    return useQuery<Folder[]>({
        queryKey: ['metadata', 'folders', libraryId, path] as any,
        queryFn: () => MetadataApi.listFolders(libraryId() as number, path()),
        enabled: () => libraryId() !== null,
        staleTime: 15_000
    })
}

export function useTracks(libraryId: () => number | null, path: () => string | null) {
    return useQuery<Track[]>({
        queryKey: ['metadata', 'tracks', libraryId, path] as any,
        queryFn: () => MetadataApi.listTracks(libraryId() as number, path() as string),
        enabled: () => libraryId() !== null && path() !== null,
        staleTime: 15_000
    })
}

// The server refuses a single request that both renames artists and sets their
// MusicBrainz IDs, because the name-keyed MB-ID map would misalign against the
// new names. When a patch carries both, split it: write the names first, then
// (only if that wrote anything) write the MB-IDs keyed by the new names.
// partitionFields returns null when no split is needed.
export function partitionFields(
    fields: PatchFields
): { names: PatchFields; mbids: PatchFields } | null {
    const conflict =
        (fields.artists !== undefined && fields.artist_mbids !== undefined) ||
        (fields.album_artists !== undefined && fields.album_artist_mbids !== undefined)
    if (!conflict) return null
    const { artist_mbids, album_artist_mbids, ...names } = fields
    const mbids: PatchFields = {}
    if (artist_mbids !== undefined) mbids.artist_mbids = artist_mbids
    if (album_artist_mbids !== undefined) mbids.album_artist_mbids = album_artist_mbids
    return { names, mbids }
}

// mergeUpdateResults folds the per-path results of the two sequential writes
// into one list: a path is ok only if every step that ran succeeded, and error
// messages are concatenated.
export function mergeUpdateResults(a: UpdateResult[], b: UpdateResult[]): UpdateResult[] {
    const byPath = new Map<string, UpdateResult>()
    for (const r of a) byPath.set(r.path, { ...r })
    for (const r of b) {
        const prev = byPath.get(r.path)
        if (!prev) {
            byPath.set(r.path, { ...r })
            continue
        }
        const error = [prev.error, r.error].filter(Boolean).join('; ')
        byPath.set(r.path, { path: r.path, ok: prev.ok && r.ok, error: error || undefined })
    }
    return [...byPath.values()]
}

// One logical tracks update: per-path write results plus the server's report on
// the re-index that followed them.
export interface UpdateTracksResult {
    results: UpdateResult[]
    rescan?: RescanStatus
}

// updateTracksPartitioned performs one logical tracks update, transparently
// splitting it into the two sequential PUTs the server requires when a patch
// both renames artists and sets their MB IDs. Shared by useUpdateTracks and
// the edit-session save (which needs the raw call without per-batch toasts).
// When the patch is split, the second write's rescan status is the current one;
// the first is only reported when the split short-circuits.
export async function updateTracksPartitioned(
    body: UpdateTracksRequest
): Promise<UpdateTracksResult> {
    const parts = partitionFields(body.fields)
    if (!parts) return MetadataApi.updateTracks(body)
    const first = await MetadataApi.updateTracks({ ...body, fields: parts.names })
    if (!first.results.some((r) => r.ok)) return first
    const second = await MetadataApi.updateTracks({ ...body, fields: parts.mbids })
    return {
        results: mergeUpdateResults(first.results, second.results),
        rescan: second.rescan ?? first.rescan
    }
}

// rescanWarning is the toast a failed post-write re-index produces: the write
// itself landed on disk, only the library index lags. Shared by every write
// path (tags and pictures) so the wording never drifts. Returns null when the
// re-index succeeded or the server did not report one.
export function rescanWarning(rescan: RescanStatus | undefined) {
    if (!rescan || rescan.ok) return null
    return {
        severity: 'warn' as const,
        summary: 'Saved, but the library index was not updated',
        detail: rescan.error ?? 'unknown error',
        life: 8000
    }
}

export function useUpdateTracks() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: updateTracksPartitioned,
        onSuccess: (out) => {
            invalidateAfterMetadataWrite(qc)
            const results = out.results
            const ok = results.filter((r) => r.ok).length
            const failed = results.length - ok
            const warning = rescanWarning(out.rescan)
            if (warning) {
                toast.add(warning)
            }
            if (failed === 0) {
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
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to save metadata',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

// useRawTags loads the complete tag maps of the given paths for the raw
// editor. Only enabled while the raw view is open.
export function useRawTags(
    libraryId: () => number | null,
    paths: () => string[],
    enabled: () => boolean
) {
    return useQuery({
        queryKey: ['metadata', 'raw', libraryId, paths] as any,
        queryFn: () => MetadataApi.getRawTags(libraryId() as number, paths()),
        enabled: () => enabled() && libraryId() !== null && paths().length > 0,
        staleTime: 15_000
    })
}

// useMetadataCapabilities reports which optional editor features the server
// supports (currently: fingerprint identification). Static per server run.
export function useMetadataCapabilities() {
    return useQuery<MetadataCapabilities>({
        queryKey: ['metadata', 'capabilities'],
        queryFn: MetadataApi.getMetadataCapabilities,
        staleTime: Infinity
    })
}

// useIdentifyTracks resolves files to MusicBrainz recording candidates by
// acoustic fingerprint. Identification writes nothing, so no invalidation.
export function useIdentifyTracks() {
    const toast = useToast()
    return useMutation({
        mutationFn: (body: IdentifyRequest) => MetadataApi.identifyTracks(body),
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to identify tracks',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

// PictureMutationOptions tunes the shared picture mutations for a caller that
// drives many of them in one logical save. quietRescanWarning suppresses the
// per-call "index not updated" toast so that caller can raise one aggregate
// warning instead of one per op (see useEditSession.savePictures).
export interface PictureMutationOptions {
    quietRescanWarning?: boolean
}

export function useApplyPicture(opts: PictureMutationOptions = {}) {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (form: FormData) => MetadataApi.applyPicture(form),
        onSuccess: (out) => {
            invalidateAfterMetadataWrite(qc)
            // The image is written either way; warn when the index did not catch
            // up, or the album keeps serving the old cover with no explanation.
            const warning = opts.quietRescanWarning ? null : rescanWarning(out.rescan)
            if (warning) {
                toast.add(warning)
            }
            toast.add({ severity: 'success', summary: 'Picture saved', life: 3000 })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to save picture',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

export function useDeletePicture(opts: PictureMutationOptions = {}) {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (v: {
            libraryId: number
            path: string
            type: string
            slot: PictureSlot
            paths?: string[]
        }) => MetadataApi.deletePicture(v.libraryId, v.path, v.type, v.slot, v.paths),
        onSuccess: (out) => {
            invalidateAfterMetadataWrite(qc)
            const warning = opts.quietRescanWarning ? null : rescanWarning(out?.rescan)
            if (warning) {
                toast.add(warning)
            }
            toast.add({ severity: 'success', summary: 'Picture removed', life: 3000 })
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to remove picture',
                detail: apiErrorMessage(err),
                life: 5000
            })
        }
    })
}

// ----- Pure helpers (exported for unit tests) -----

export interface FieldDiff<T> {
    shared: boolean
    value: T
}

export interface InitialValues {
    title: FieldDiff<string>
    album: FieldDiff<string>
    mb_recording_id: FieldDiff<string>
    mb_release_id: FieldDiff<string>
    mb_release_group_id: FieldDiff<string>
    artists: FieldDiff<string[]>
    album_artists: FieldDiff<string[]>
    genres: FieldDiff<string[]>
    year: FieldDiff<number>
    track_number: FieldDiff<number>
    disc_number: FieldDiff<number>
    disc_subtitle: FieldDiff<string>
    compilation: FieldDiff<boolean>
}

/**
 * Compute per-field shared/mixed state across a selection of tracks.
 * `shared: true` means every selected track has the same value (returned as `value`).
 * `shared: false` means values differ; `value` is a neutral zero ('' / [] / 0 / false).
 */
export function diffInitialValues(tracks: Track[]): InitialValues {
    const scalar = <
        K extends
            | 'title'
            | 'album'
            | 'disc_subtitle'
            | 'mb_recording_id'
            | 'mb_release_id'
            | 'mb_release_group_id'
    >(
        key: K
    ): FieldDiff<string> => {
        if (tracks.length === 0) return { shared: true, value: '' }
        const v = tracks[0][key]
        const all = tracks.every((t) => t[key] === v)
        return all ? { shared: true, value: v } : { shared: false, value: '' }
    }
    const num = (key: 'year' | 'track_number' | 'disc_number'): FieldDiff<number> => {
        if (tracks.length === 0) return { shared: true, value: 0 }
        const v = tracks[0][key]
        const all = tracks.every((t) => t[key] === v)
        return all ? { shared: true, value: v } : { shared: false, value: 0 }
    }
    const bool = (key: 'compilation'): FieldDiff<boolean> => {
        if (tracks.length === 0) return { shared: true, value: false }
        const v = tracks[0][key]
        const all = tracks.every((t) => t[key] === v)
        return all ? { shared: true, value: v } : { shared: false, value: false }
    }
    const arr = (key: 'artists' | 'album_artists' | 'genres'): FieldDiff<string[]> => {
        if (tracks.length === 0) return { shared: true, value: [] }
        const first = tracks[0][key]
        const all = tracks.every(
            (t) => t[key].length === first.length && t[key].every((x, i) => x === first[i])
        )
        return all ? { shared: true, value: [...first] } : { shared: false, value: [] }
    }
    return {
        title: scalar('title'),
        album: scalar('album'),
        mb_recording_id: scalar('mb_recording_id'),
        mb_release_id: scalar('mb_release_id'),
        mb_release_group_id: scalar('mb_release_group_id'),
        artists: arr('artists'),
        album_artists: arr('album_artists'),
        genres: arr('genres'),
        year: num('year'),
        track_number: num('track_number'),
        disc_number: num('disc_number'),
        disc_subtitle: scalar('disc_subtitle'),
        compilation: bool('compilation')
    }
}

export interface EditValues {
    title: string
    album: string
    mb_release_id: string
    mb_release_group_id: string
    artists: string[]
    album_artists: string[]
    genres: string[]
    year: number
    track_number: number
    disc_number: number
    disc_subtitle: string
    compilation: boolean
}

export interface ArtistMbidRow {
    name: string
    mbid: string
    mixed: boolean
}

/**
 * Collapse a track selection into one row per distinct artist name, carrying
 * that name's shared MusicBrainz ID. `mixed` is true when selected tracks
 * disagree on the ID for that name (in which case `mbid` is '').
 * `nameField`/`idField` are aligned positionally within each track.
 */
export function distinctArtistMbids(
    tracks: Track[],
    nameField: 'artists' | 'album_artists',
    idField: 'mb_artist_ids' | 'mb_album_artist_ids'
): ArtistMbidRow[] {
    const order: string[] = []
    const seen = new Map<string, Set<string>>()
    for (const t of tracks) {
        const names = t[nameField]
        const ids = t[idField]
        names.forEach((name, i) => {
            const id = ids[i] ?? ''
            if (!seen.has(name)) {
                seen.set(name, new Set())
                order.push(name)
            }
            seen.get(name)!.add(id)
        })
    }
    return order.map((name) => {
        const ids = seen.get(name)!
        if (ids.size === 1) {
            return { name, mbid: [...ids][0], mixed: false }
        }
        return { name, mbid: '', mixed: true }
    })
}
