import { describe, it, expect, vi, beforeEach } from 'vitest'
import { useIdentifyCache } from '@/composables/useIdentifyCache'
import { useIdentifyRuns } from '@/composables/useIdentifyRuns'
import type {
    IdentifyAlbumRequest,
    IdentifyAlbumResponse,
    IdentifyRequest,
    IdentifyTrackResult,
    Track
} from '@/types/metadata'

// The runs composable instantiates the identify mutations (which need a toast
// provider) at setup; stub those and drive the API through spies instead. The
// stubs carry a real isPending that tracks the in-flight call, so the loading
// assertions below are about the composable, not about the stub.
const identifySpy = vi.hoisted(() => vi.fn())
const identifyAlbumSpy = vi.hoisted(() => vi.fn())
const identifyPending = vi.hoisted(() => ({ value: false }))
const identifyAlbumPending = vi.hoisted(() => ({ value: false }))
vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    const track = (
        spy: typeof identifySpy,
        pending: { value: boolean }
    ) => async (vars: unknown) => {
        pending.value = true
        try {
            return await spy(vars)
        } finally {
            pending.value = false
        }
    }
    return {
        ...actual,
        useIdentifyTracks: () => ({
            mutateAsync: track(identifySpy, identifyPending),
            isPending: identifyPending
        }),
        useIdentifyAlbum: () => ({
            mutateAsync: track(identifyAlbumSpy, identifyAlbumPending),
            isPending: identifyAlbumPending
        })
    }
})

const mkTrack = (path: string): Track => ({
    path,
    name: path,
    title: '',
    artists: [],
    album_artists: [],
    album: '',
    genres: [],
    year: 0,
    track_number: 0,
    disc_number: 0,
    disc_subtitle: '',
    compilation: false,
    mb_artist_ids: [],
    mb_album_artist_ids: [],
    mb_recording_id: '',
    mb_release_id: '',
    mb_release_group_id: ''
})

const mkResult = (path: string, over: Partial<IdentifyTrackResult> = {}): IdentifyTrackResult => ({
    path,
    candidates: [
        { recording_mbid: `rec-${path}`, title: `Title ${path}`, artists: [], score: 0.9, releases: [] }
    ],
    ...over
})

const mkAlbumResponse = (album: string): IdentifyAlbumResponse => ({
    options: [
        {
            release_mbid: `rel-${album}`,
            release_group_mbid: `rg-${album}`,
            album,
            year: 1991,
            artists: [],
            track_count: 2,
            disc_count: 1,
            enriched: true,
            matched_count: 2,
            mean_score: 0.9,
            assignments: [],
            tracks: []
        }
    ],
    errors: [{ path: 'c.mp3', error: 'could not be read' }]
})

// The body the spies were called with, so a test can assert which paths were
// actually asked for.
function lastBody(spy: typeof identifySpy): IdentifyRequest | IdentifyAlbumRequest {
    return spy.mock.calls[spy.mock.calls.length - 1][0].body
}

beforeEach(() => {
    useIdentifyCache().clear()
    identifySpy.mockReset()
    identifyAlbumSpy.mockReset()
})

describe('useIdentifyRuns track identification', () => {
    it('asks the server for uncached paths and shows the results', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const runs = useIdentifyRuns(() => 1)

        await runs.identify([mkTrack('a.mp3')])

        expect(identifySpy).toHaveBeenCalledTimes(1)
        expect(lastBody(identifySpy)).toEqual({ library_id: 1, paths: ['a.mp3'] })
        expect(runs.trackDialog.value).toBe(true)
        expect(runs.trackResults.value.map((r) => r.path)).toEqual(['a.mp3'])
    })

    it('reuses the cached results when the same files are identified again', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const runs = useIdentifyRuns(() => 1)
        await runs.identify([mkTrack('a.mp3')])
        runs.trackDialog.value = false

        await runs.identify([mkTrack('a.mp3')])

        expect(identifySpy).toHaveBeenCalledTimes(1)
        expect(runs.trackDialog.value).toBe(true)
        expect(runs.trackResults.value.map((r) => r.path)).toEqual(['a.mp3'])
    })

    it('asks only for the paths it has no answer for, and merges both in selection order', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const runs = useIdentifyRuns(() => 1)
        await runs.identify([mkTrack('a.mp3')])

        identifySpy.mockResolvedValue([mkResult('b.mp3')])
        await runs.identify([mkTrack('b.mp3'), mkTrack('a.mp3')])

        expect(identifySpy).toHaveBeenCalledTimes(2)
        expect(lastBody(identifySpy)).toEqual({ library_id: 1, paths: ['b.mp3'] })
        expect(runs.trackResults.value.map((r) => r.path)).toEqual(['b.mp3', 'a.mp3'])
    })

    it('asks again for a path whose identification failed', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3', { candidates: [], error: 'fpcalc failed' })])
        const runs = useIdentifyRuns(() => 1)
        await runs.identify([mkTrack('a.mp3')])

        await runs.identify([mkTrack('a.mp3')])

        expect(identifySpy).toHaveBeenCalledTimes(2)
    })

    it('closes the dialog and caches nothing when the request fails', async () => {
        identifySpy.mockRejectedValue(new Error('boom'))
        const runs = useIdentifyRuns(() => 1)

        await runs.identify([mkTrack('a.mp3')])

        expect(runs.trackDialog.value).toBe(false)
        await runs.identify([mkTrack('a.mp3')])
        expect(identifySpy).toHaveBeenCalledTimes(2)
    })

    it('refetches the paths of a forced re-identify instead of serving the cache', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const runs = useIdentifyRuns(() => 1)
        await runs.identify([mkTrack('a.mp3')])

        await runs.identify([mkTrack('a.mp3')], { force: true })

        expect(identifySpy).toHaveBeenCalledTimes(2)
        expect(lastBody(identifySpy)).toEqual({ library_id: 1, paths: ['a.mp3'] })
    })

    it('does nothing without a library or without files', async () => {
        const runs = useIdentifyRuns(() => null)
        await runs.identify([mkTrack('a.mp3')])
        const withLibrary = useIdentifyRuns(() => 1)
        await withLibrary.identify([])
        expect(identifySpy).not.toHaveBeenCalled()
    })

    it('reports a run that reaches the server as loading', async () => {
        identifySpy.mockImplementation(() => new Promise(() => {}))
        const runs = useIdentifyRuns(() => 1)

        void runs.identify([mkTrack('a.mp3')])

        expect(runs.isIdentifying.value).toBe(true)
        runs.cancelIdentify()
    })

    it('reports a cached run as not loading, so no spinner replaces the results', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        const runs = useIdentifyRuns(() => 1)
        await runs.identify([mkTrack('a.mp3')])
        runs.trackDialog.value = false

        // Synchronous by design for a full cache hit: there is nothing to await,
        // so the dialog never passes through a loading state.
        const done = runs.identify([mkTrack('a.mp3')])
        expect(runs.isIdentifying.value).toBe(false)
        expect(runs.trackResults.value).toHaveLength(1)
        await done
    })

    it('cancelling a run aborts the request', async () => {
        let seen: AbortSignal | undefined
        identifySpy.mockImplementation((vars: { signal?: AbortSignal }) => {
            seen = vars.signal
            return new Promise(() => {})
        })
        const runs = useIdentifyRuns(() => 1)
        void runs.identify([mkTrack('a.mp3')])

        runs.cancelIdentify()

        expect(seen?.aborted).toBe(true)
    })
})

describe('useIdentifyRuns forgetAll', () => {
    it('drops the cached answers, so the next run asks the server again', async () => {
        identifySpy.mockResolvedValue([mkResult('a.mp3')])
        identifyAlbumSpy.mockResolvedValue(mkAlbumResponse('Album A'))
        const runs = useIdentifyRuns(() => 1)
        await runs.identify([mkTrack('a.mp3')])
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])

        runs.forgetAll()

        await runs.identify([mkTrack('a.mp3')])
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])
        expect(identifySpy).toHaveBeenCalledTimes(2)
        expect(identifyAlbumSpy).toHaveBeenCalledTimes(2)
    })
})

describe('useIdentifyRuns album identification', () => {
    it('asks the server and shows the options and per-path errors', async () => {
        identifyAlbumSpy.mockResolvedValue(mkAlbumResponse('Album A'))
        const runs = useIdentifyRuns(() => 1)

        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])

        expect(identifyAlbumSpy).toHaveBeenCalledTimes(1)
        expect(runs.albumDialog.value).toBe(true)
        expect(runs.albumOptions.value.map((o) => o.album)).toEqual(['Album A'])
        expect(runs.albumPathErrors.value).toEqual([{ path: 'c.mp3', error: 'could not be read' }])
    })

    it('reuses the cached response for the same selection', async () => {
        identifyAlbumSpy.mockResolvedValue(mkAlbumResponse('Album A'))
        const runs = useIdentifyRuns(() => 1)
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])
        runs.albumDialog.value = false

        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])

        expect(identifyAlbumSpy).toHaveBeenCalledTimes(1)
        expect(runs.albumDialog.value).toBe(true)
        expect(runs.albumOptions.value.map((o) => o.album)).toEqual(['Album A'])
        expect(runs.albumPathErrors.value).toHaveLength(1)
    })

    it('asks again for a different selection', async () => {
        identifyAlbumSpy.mockResolvedValue(mkAlbumResponse('Album A'))
        const runs = useIdentifyRuns(() => 1)
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])

        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3'), mkTrack('c.mp3')])

        expect(identifyAlbumSpy).toHaveBeenCalledTimes(2)
    })

    it('refetches a forced re-identify instead of serving the cache', async () => {
        identifyAlbumSpy.mockResolvedValue(mkAlbumResponse('Album A'))
        const runs = useIdentifyRuns(() => 1)
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])

        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')], { force: true })

        expect(identifyAlbumSpy).toHaveBeenCalledTimes(2)
    })

    it('closes the dialog and caches nothing when the request fails', async () => {
        identifyAlbumSpy.mockRejectedValue(new Error('boom'))
        const runs = useIdentifyRuns(() => 1)

        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])

        expect(runs.albumDialog.value).toBe(false)
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])
        expect(identifyAlbumSpy).toHaveBeenCalledTimes(2)
    })

    it('needs two files, since one file is not an album', async () => {
        const runs = useIdentifyRuns(() => 1)
        await runs.identifyAlbum([mkTrack('a.mp3')])
        expect(identifyAlbumSpy).not.toHaveBeenCalled()
    })

    it('cancelling a run aborts the request', async () => {
        let seen: AbortSignal | undefined
        identifyAlbumSpy.mockImplementation((vars: { signal?: AbortSignal }) => {
            seen = vars.signal
            return new Promise(() => {})
        })
        const runs = useIdentifyRuns(() => 1)
        void runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])

        runs.cancelAlbumIdentify()

        expect(seen?.aborted).toBe(true)
    })

    it('reports a run that reaches the server as loading, a cached one as not', async () => {
        identifyAlbumSpy.mockImplementation(() => new Promise(() => {}))
        const runs = useIdentifyRuns(() => 1)
        void runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])
        expect(runs.isIdentifyingAlbum.value).toBe(true)
        runs.cancelAlbumIdentify()

        identifyAlbumSpy.mockResolvedValue(mkAlbumResponse('Album A'))
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])
        runs.albumDialog.value = false

        const done = runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])
        expect(runs.isIdentifyingAlbum.value).toBe(false)
        expect(runs.albumOptions.value).toHaveLength(1)
        await done
    })

    it('keeps the tracks the run was launched for, so the dialog can list them', async () => {
        identifyAlbumSpy.mockResolvedValue(mkAlbumResponse('Album A'))
        const runs = useIdentifyRuns(() => 1)
        await runs.identifyAlbum([mkTrack('a.mp3'), mkTrack('b.mp3')])
        expect(runs.albumTracks.value.map((t) => t.path)).toEqual(['a.mp3', 'b.mp3'])
    })
})
