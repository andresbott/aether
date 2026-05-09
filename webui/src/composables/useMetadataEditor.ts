import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { useToast } from 'primevue/usetoast'
import * as MetadataApi from '@/lib/api/Metadata'
import type {
    Folder,
    PatchFields,
    Track,
    UpdateTracksRequest
} from '@/types/metadata'

// Query keys
export const metadataQueryKeys = {
    folders: (libraryId: number, path: string) =>
        ['metadata', 'folders', libraryId, path] as const,
    tracks: (libraryId: number, path: string) =>
        ['metadata', 'tracks', libraryId, path] as const
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

export function useUpdateTracks() {
    const qc = useQueryClient()
    const toast = useToast()
    return useMutation({
        mutationFn: (body: UpdateTracksRequest) => MetadataApi.updateTracks(body),
        onSuccess: (results, req) => {
            qc.invalidateQueries({ queryKey: ['metadata', 'tracks'] })
            const ok = results.filter((r) => r.ok).length
            const failed = results.length - ok
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
                    detail: results.filter((r) => !r.ok).map((r) => `${r.path}: ${r.error}`).join('\n'),
                    life: 8000
                })
            }
        },
        onError: (err: any) => {
            toast.add({
                severity: 'error',
                summary: 'Failed to save metadata',
                detail: err?.response?.data?.error ?? err.message,
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
    artists: FieldDiff<string[]>
    album_artists: FieldDiff<string[]>
    year: FieldDiff<number>
    compilation: FieldDiff<boolean>
}

/**
 * Compute per-field shared/mixed state across a selection of tracks.
 * `shared: true` means every selected track has the same value (returned as `value`).
 * `shared: false` means values differ; `value` is a neutral zero ('' / [] / 0 / false).
 */
export function diffInitialValues(tracks: Track[]): InitialValues {
    const scalar = <K extends 'title' | 'album'>(key: K): FieldDiff<string> => {
        if (tracks.length === 0) return { shared: true, value: '' }
        const v = tracks[0][key]
        const all = tracks.every((t) => t[key] === v)
        return all ? { shared: true, value: v } : { shared: false, value: '' }
    }
    const num = (key: 'year'): FieldDiff<number> => {
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
    const arr = (key: 'artists' | 'album_artists'): FieldDiff<string[]> => {
        if (tracks.length === 0) return { shared: true, value: [] }
        const first = tracks[0][key]
        const all = tracks.every(
            (t) =>
                t[key].length === first.length &&
                t[key].every((x, i) => x === first[i])
        )
        return all ? { shared: true, value: [...first] } : { shared: false, value: [] }
    }
    return {
        title: scalar('title'),
        album: scalar('album'),
        artists: arr('artists'),
        album_artists: arr('album_artists'),
        year: num('year'),
        compilation: bool('compilation')
    }
}

export interface AppliedFlags {
    title: boolean
    album: boolean
    artists: boolean
    album_artists: boolean
    year: boolean
    compilation: boolean
}

export interface EditValues {
    title: string
    album: string
    artists: string[]
    album_artists: string[]
    year: number
    compilation: boolean
}

/**
 * Build the `fields` object for PUT /metadata/tracks, including only fields
 * whose "apply" flag is on.
 */
export function buildPatchFields(applied: AppliedFlags, values: EditValues): PatchFields {
    const out: PatchFields = {}
    if (applied.title) out.title = values.title
    if (applied.album) out.album = values.album
    if (applied.artists) out.artists = [...values.artists]
    if (applied.album_artists) out.album_artists = [...values.album_artists]
    if (applied.year) out.year = values.year
    if (applied.compilation) out.compilation = values.compilation
    return out
}
