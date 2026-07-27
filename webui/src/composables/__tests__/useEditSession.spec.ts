import { describe, it, expect, vi, beforeEach } from 'vitest'
import { nextTick, ref } from 'vue'
import { albumKey } from '@/lib/albumIdentity'
import {
    albumPickToOverlay,
    applyOverlay,
    buildTrackPatch,
    candidateToOverlay,
    groupPatches,
    originalValueEquals,
    trackCredits,
    useEditSession
} from '@/composables/useEditSession'
import type {
    AlbumIdentifyPick,
    AlbumOption,
    IdentifyCandidate,
    Track,
    TrackOverlay
} from '@/types/metadata'

// The session composable calls useApplyPicture()/useDeletePicture() plus the
// query client and toast at setup; keep the real pure helpers but stub the
// side-effecting pieces so no vue-query/toast providers are needed.
const updateTracksSpy = vi.hoisted(() => vi.fn())
const applyPictureSpy = vi.hoisted(() => vi.fn())
const deletePictureSpy = vi.hoisted(() => vi.fn())
vi.mock('@/lib/api/Metadata', () => ({
    updateTracks: (...args: unknown[]) => updateTracksSpy(...args)
}))
vi.mock('@/composables/useMetadataEditor', async (importActual) => {
    const actual = await importActual<typeof import('@/composables/useMetadataEditor')>()
    return {
        ...actual,
        useApplyPicture: () => ({ mutateAsync: applyPictureSpy }),
        useDeletePicture: () => ({ mutateAsync: deletePictureSpy })
    }
})
const invalidateSpy = vi.hoisted(() => vi.fn())
vi.mock('@tanstack/vue-query', async (importActual) => {
    const actual = await importActual<typeof import('@tanstack/vue-query')>()
    return {
        ...actual,
        useQueryClient: () => ({ invalidateQueries: invalidateSpy })
    }
})
const toastAddSpy = vi.hoisted(() => vi.fn())
vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: toastAddSpy })
}))

const mkTrack = (over: Partial<Track> = {}): Track => ({
    path: 'a.mp3',
    name: 'a.mp3',
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
    mb_release_group_id: '',
    ...over
})

describe('trackCredits', () => {
    it('aligns names with their mbids', () => {
        const t = mkTrack({ artists: ['One', 'Two'], mb_artist_ids: ['id1'] })
        expect(trackCredits(t, 'artists')).toEqual([
            { name: 'One', mbid: 'id1' },
            { name: 'Two', mbid: '' }
        ])
    })
})

describe('applyOverlay', () => {
    it('returns the track unchanged without an overlay', () => {
        const t = mkTrack({ title: 'Original' })
        expect(applyOverlay(t, undefined)).toBe(t)
    })

    it('overrides staged scalars and keeps the rest', () => {
        const t = mkTrack({ title: 'Original', album: 'Album', year: 1990 })
        const out = applyOverlay(t, { title: 'Staged', year: 2000 })
        expect(out.title).toBe('Staged')
        expect(out.year).toBe(2000)
        expect(out.album).toBe('Album')
    })

    it('expands artist credits into aligned names and ids', () => {
        const t = mkTrack({ artists: ['Old'], mb_artist_ids: ['old-id'] })
        const out = applyOverlay(t, {
            artists: [
                { name: 'New', mbid: 'new-id' },
                { name: 'Feat', mbid: '' }
            ]
        })
        expect(out.artists).toEqual(['New', 'Feat'])
        expect(out.mb_artist_ids).toEqual(['new-id', ''])
    })

    it('supports falsy staged values (empty string, zero, false)', () => {
        const t = mkTrack({ title: 'X', year: 1990, compilation: true })
        const out = applyOverlay(t, { title: '', year: 0, compilation: false })
        expect(out.title).toBe('')
        expect(out.year).toBe(0)
        expect(out.compilation).toBe(false)
    })

    it('overrides genres and track number', () => {
        const t = mkTrack({ genres: ['Rock'], track_number: 1 })
        const out = applyOverlay(t, { genres: ['Jazz', 'Blues'], track_number: 9 })
        expect(out.genres).toEqual(['Jazz', 'Blues'])
        expect(out.track_number).toBe(9)
        // The staged list must not be aliased into the effective track.
        out.genres.push('Mutated')
        expect(t.genres).toEqual(['Rock'])
    })
})

describe('originalValueEquals', () => {
    it('compares scalars', () => {
        const t = mkTrack({ title: 'Same' })
        expect(originalValueEquals(t, 'title', 'Same')).toBe(true)
        expect(originalValueEquals(t, 'title', 'Other')).toBe(false)
    })

    it('compares credit lists by name and mbid', () => {
        const t = mkTrack({ artists: ['A'], mb_artist_ids: ['id'] })
        expect(originalValueEquals(t, 'artists', [{ name: 'A', mbid: 'id' }])).toBe(true)
        expect(originalValueEquals(t, 'artists', [{ name: 'A', mbid: 'other' }])).toBe(false)
        expect(originalValueEquals(t, 'artists', [])).toBe(false)
    })

    it('compares genre lists element-wise', () => {
        const t = mkTrack({ genres: ['Rock', 'Jazz'] })
        expect(originalValueEquals(t, 'genres', ['Rock', 'Jazz'])).toBe(true)
        expect(originalValueEquals(t, 'genres', ['Jazz', 'Rock'])).toBe(false)
        expect(originalValueEquals(t, 'genres', [])).toBe(false)
    })
})

describe('buildTrackPatch', () => {
    it('copies staged scalars verbatim', () => {
        const patch = buildTrackPatch(mkTrack(), {
            title: 'T',
            mb_recording_id: 'rec',
            year: 2001
        })
        expect(patch).toEqual({ title: 'T', mb_recording_id: 'rec', year: 2001 })
    })

    it('sends names plus complete mbid map when names changed', () => {
        const original = mkTrack({ artists: ['Old'], mb_artist_ids: ['old-id'] })
        const patch = buildTrackPatch(original, {
            artists: [{ name: 'New', mbid: 'new-id' }]
        })
        expect(patch.artists).toEqual(['New'])
        expect(patch.artist_mbids).toEqual({ New: 'new-id' })
    })

    it('sends only changed mbids when names are untouched', () => {
        const original = mkTrack({
            artists: ['A', 'B'],
            mb_artist_ids: ['a-id', 'b-id']
        })
        const patch = buildTrackPatch(original, {
            artists: [
                { name: 'A', mbid: 'a-id' },
                { name: 'B', mbid: 'b-new' }
            ]
        })
        expect(patch.artists).toBeUndefined()
        expect(patch.artist_mbids).toEqual({ B: 'b-new' })
    })

    it('produces an empty patch for a credit list identical to the original', () => {
        const original = mkTrack({ artists: ['A'], mb_artist_ids: ['a-id'] })
        const patch = buildTrackPatch(original, {
            artists: [{ name: 'A', mbid: 'a-id' }]
        })
        expect(patch).toEqual({})
    })

    it('trims and drops empty artist names', () => {
        const original = mkTrack({ artists: ['Old'] })
        const patch = buildTrackPatch(original, {
            artists: [
                { name: '  New  ', mbid: 'id' },
                { name: '   ', mbid: 'ignored' }
            ]
        })
        expect(patch.artists).toEqual(['New'])
        expect(patch.artist_mbids).toEqual({ New: 'id' })
    })

    it('handles album_artists through the same path', () => {
        const original = mkTrack({ album_artists: ['Old'] })
        const patch = buildTrackPatch(original, {
            album_artists: [{ name: 'New', mbid: 'x' }]
        })
        expect(patch.album_artists).toEqual(['New'])
        expect(patch.album_artist_mbids).toEqual({ New: 'x' })
    })

    it('copies genres (trimmed, empties dropped) and track number', () => {
        const patch = buildTrackPatch(mkTrack(), {
            genres: [' Rock ', '', 'Jazz'],
            track_number: 4
        })
        expect(patch.genres).toEqual(['Rock', 'Jazz'])
        expect(patch.track_number).toBe(4)
    })
})

describe('groupPatches', () => {
    it('groups tracks with identical patches into one batch', () => {
        const originals = new Map([
            ['a.mp3', mkTrack({ path: 'a.mp3' })],
            ['b.mp3', mkTrack({ path: 'b.mp3' })]
        ])
        const overlays = new Map<string, TrackOverlay>([
            ['a.mp3', { album: 'Same' }],
            ['b.mp3', { album: 'Same' }]
        ])
        const batches = groupPatches(originals, overlays)
        expect(batches).toHaveLength(1)
        expect(batches[0].paths.sort()).toEqual(['a.mp3', 'b.mp3'])
        expect(batches[0].fields).toEqual({ album: 'Same' })
    })

    it('splits tracks whose patches differ', () => {
        const originals = new Map([
            ['a.mp3', mkTrack({ path: 'a.mp3' })],
            ['b.mp3', mkTrack({ path: 'b.mp3' })]
        ])
        const overlays = new Map<string, TrackOverlay>([
            ['a.mp3', { title: 'One' }],
            ['b.mp3', { title: 'Two' }]
        ])
        const batches = groupPatches(originals, overlays)
        expect(batches).toHaveLength(2)
    })

    it('skips overlays that produce empty patches and unknown paths', () => {
        const originals = new Map([
            ['a.mp3', mkTrack({ path: 'a.mp3', artists: ['A'], mb_artist_ids: ['id'] })]
        ])
        const overlays = new Map<string, TrackOverlay>([
            ['a.mp3', { artists: [{ name: 'A', mbid: 'id' }] }],
            ['gone.mp3', { title: 'X' }]
        ])
        expect(groupPatches(originals, overlays)).toHaveLength(0)
    })
})

describe('buildTrackPatch raw tags', () => {
    it('emits raw_tags including delete entries', () => {
        const patch = buildTrackPatch(mkTrack(), {
            raw: { CUSTOM: ['x'], OLD_JUNK: [] }
        })
        expect(patch.raw_tags).toEqual({ CUSTOM: ['x'], OLD_JUNK: [] })
    })

    it('omits raw_tags for an empty raw map', () => {
        const patch = buildTrackPatch(mkTrack(), { raw: {} })
        expect('raw_tags' in patch).toBe(false)
    })

    it('combines raw and structured edits in one patch', () => {
        const patch = buildTrackPatch(mkTrack(), {
            title: 'T',
            raw: { CUSTOM: ['x'] }
        })
        expect(patch.title).toBe('T')
        expect(patch.raw_tags).toEqual({ CUSTOM: ['x'] })
    })

    it('emits remove_unsupported sorted for stable batch grouping', () => {
        const patch = buildTrackPatch(mkTrack(), {
            removeUnsupported: ['PRIV/z', 'GEOB']
        })
        expect(patch.remove_unsupported).toEqual(['GEOB', 'PRIV/z'])
    })

    it('omits remove_unsupported for an empty list', () => {
        const patch = buildTrackPatch(mkTrack(), { removeUnsupported: [] })
        expect('remove_unsupported' in patch).toBe(false)
    })
})

describe('picture staging', () => {
    // The album the single-folder tests operate on: these tracks carry no
    // album tag, so their identity falls back to their directory.
    const ALBUM = albumKey(mkTrack({ path: 'album/a.mp3' }))

    const mkSession = (tracks: Track[] = [mkTrack({ path: 'album/a.mp3' })], lib = 3) =>
        useEditSession(
            () => tracks,
            () => lib
        )

    beforeEach(() => {
        updateTracksSpy.mockReset()
        updateTracksSpy.mockResolvedValue({ results: [{ path: 'album/a.mp3', ok: true }] })
        applyPictureSpy.mockReset()
        applyPictureSpy.mockResolvedValue({ ok: true, target: 'folder', type: 'Back Cover' })
        deletePictureSpy.mockReset()
        deletePictureSpy.mockResolvedValue({ ok: true })
        invalidateSpy.mockReset()
        toastAddSpy.mockReset()
        vi.stubGlobal('URL', {
            ...URL,
            createObjectURL: vi.fn(() => 'blob:preview'),
            revokeObjectURL: vi.fn()
        })
    })

    it('stages a set op per type+slot cell and flags unsaved changes', () => {
        const session = mkSession()
        expect(session.hasStagedChanges.value).toBe(false)
        session.stagePictureSet(
            ALBUM,
            'Back Cover',
            'folder',
            { file: null, imageUrl: 'http://img/x.jpg' },
            ['album/a.mp3']
        )
        expect(session.hasStagedChanges.value).toBe(true)
        const op = session.getPictureOp(ALBUM, 'Back Cover', 'folder')
        expect(op).toEqual({
            kind: 'set',
            file: null,
            imageUrl: 'http://img/x.jpg',
            preview: 'http://img/x.jpg',
            paths: ['album/a.mp3']
        })
        // Other cells stay empty.
        expect(session.getPictureOp(ALBUM, 'Back Cover', 'embedded')).toBeUndefined()
        expect(session.getPictureOp(ALBUM, 'Front Cover', 'folder')).toBeUndefined()
    })

    it('a set op overwrites a staged removal on the same cell and vice versa', () => {
        const session = mkSession()
        session.stagePictureRemoval(ALBUM, 'Media', 'db', ['album/a.mp3'])
        expect(session.getPictureOp(ALBUM, 'Media', 'db')).toEqual({
            kind: 'remove',
            paths: ['album/a.mp3']
        })
        session.stagePictureSet(ALBUM, 'Media', 'db', { file: null, imageUrl: 'u' }, [
            'album/a.mp3'
        ])
        expect(session.getPictureOp(ALBUM, 'Media', 'db')?.kind).toBe('set')
        session.stagePictureRemoval(ALBUM, 'Media', 'db', ['album/a.mp3'])
        expect(session.getPictureOp(ALBUM, 'Media', 'db')).toEqual({
            kind: 'remove',
            paths: ['album/a.mp3']
        })
    })

    it('discarding the last op clears the unsaved flag', () => {
        const session = mkSession()
        session.stagePictureSet(ALBUM, 'Back Cover', 'folder', { file: null, imageUrl: 'u' }, [
            'album/a.mp3'
        ])
        session.discardPictureOp(ALBUM, 'Back Cover', 'folder')
        expect(session.getPictureOp(ALBUM, 'Back Cover', 'folder')).toBeUndefined()
        expect(session.hasStagedChanges.value).toBe(false)
    })

    it('creates and revokes blob previews for file-based ops', () => {
        const session = mkSession()
        const file = new File(['x'], 'art.png', { type: 'image/png' })
        session.stagePictureSet(ALBUM, 'Back Cover', 'folder', { file, imageUrl: null }, [
            'album/a.mp3'
        ])
        expect(URL.createObjectURL).toHaveBeenCalledWith(file)
        expect(session.getPictureOp(ALBUM, 'Back Cover', 'folder')?.kind).toBe('set')
        session.discardPictureOp(ALBUM, 'Back Cover', 'folder')
        expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview')
    })

    it('save posts one form per set op including type and target', async () => {
        const session = mkSession()
        session.stagePictureSet(
            ALBUM,
            'Back Cover',
            'folder',
            { file: null, imageUrl: 'http://img/x.jpg' },
            ['album/a.mp3']
        )
        await session.save()
        expect(applyPictureSpy).toHaveBeenCalledTimes(1)
        const form = applyPictureSpy.mock.calls[0][0] as FormData
        expect(form.get('target')).toBe('folder')
        expect(form.get('type')).toBe('Back Cover')
        expect(form.get('image_url')).toBe('http://img/x.jpg')
        expect(form.getAll('paths')).toEqual(['album/a.mp3'])
        expect(session.hasStagedChanges.value).toBe(false)
        expect(session.picturesSavedAt.value).toBeGreaterThan(0)
    })

    it('save deletes removals, omitting paths only for the album-wide db slot', async () => {
        const session = mkSession()
        session.stagePictureRemoval(ALBUM, 'Front Cover', 'embedded', ['album/a.mp3'])
        session.stagePictureRemoval(ALBUM, 'Back Cover', 'folder', ['album/a.mp3'])
        session.stagePictureRemoval(ALBUM, 'Media', 'db', ['album/a.mp3'])
        await session.save()
        expect(deletePictureSpy).toHaveBeenCalledWith({
            libraryId: 3,
            path: 'album',
            type: 'Front Cover',
            slot: 'embedded',
            paths: ['album/a.mp3']
        })
        // The folder slot needs the paths too: they name the directories to
        // clean, which for a multi-disc album is more than one.
        expect(deletePictureSpy).toHaveBeenCalledWith({
            libraryId: 3,
            path: 'album',
            type: 'Back Cover',
            slot: 'folder',
            paths: ['album/a.mp3']
        })
        expect(deletePictureSpy).toHaveBeenCalledWith({
            libraryId: 3,
            path: 'album',
            type: 'Media',
            slot: 'db',
            paths: undefined
        })
        expect(session.hasStagedChanges.value).toBe(false)
    })

    // The image is on disk either way, so a failed re-index must not be silent:
    // otherwise the album keeps serving the old cover with a green toast.
    const rescanWarnings = () =>
        toastAddSpy.mock.calls
            .map((c) => c[0])
            .filter((t) => t.summary === 'Saved, but the library index was not updated')

    it('warns when a picture write reports a failed re-index', async () => {
        const session = mkSession()
        applyPictureSpy.mockResolvedValue({
            ok: true,
            target: 'folder',
            type: 'Back Cover',
            rescan: { ok: false, error: 'db is locked' }
        })
        session.stagePictureSet('album', 'Back Cover', 'folder', { file: null, imageUrl: 'u' }, [
            'album/a.mp3'
        ])
        await session.save()
        expect(rescanWarnings()).toEqual([
            expect.objectContaining({
                severity: 'warn',
                detail: 'db is locked',
                life: 8000
            })
        ])
    })

    it('warns when a picture removal reports a failed re-index', async () => {
        const session = mkSession()
        deletePictureSpy.mockResolvedValue({ ok: true, rescan: { ok: false } })
        session.stagePictureRemoval('album', 'Back Cover', 'folder', ['album/a.mp3'])
        await session.save()
        expect(rescanWarnings()).toEqual([
            expect.objectContaining({ severity: 'warn', detail: 'unknown error' })
        ])
    })

    // Deliberate: the session reports the LAST failure and does not clear it
    // when a later batch succeeds — a partly stale index is still stale. Only
    // one warning is raised no matter how many ops failed.
    it('keeps a picture rescan failure even when a later tag write succeeds', async () => {
        const session = mkSession()
        deletePictureSpy.mockResolvedValue({
            ok: true,
            rescan: { ok: false, error: 'picture reindex failed' }
        })
        updateTracksSpy.mockResolvedValue({
            results: [{ path: 'album/a.mp3', ok: true }],
            rescan: { ok: true }
        })
        session.stagePictureRemoval('album', 'Back Cover', 'folder', ['album/a.mp3'])
        session.stageField(['album/a.mp3'], 'title', 'New')
        await session.save()
        expect(updateTracksSpy).toHaveBeenCalledTimes(1)
        expect(rescanWarnings()).toEqual([
            expect.objectContaining({ detail: 'picture reindex failed' })
        ])
    })

    it('reports a picture rescan failure even when the save then aborts', async () => {
        const session = mkSession()
        deletePictureSpy.mockResolvedValue({
            ok: true,
            rescan: { ok: false, error: 'picture reindex failed' }
        })
        applyPictureSpy.mockRejectedValue(new Error('boom'))
        session.stagePictureRemoval('album', 'Back Cover', 'folder', ['album/a.mp3'])
        session.stagePictureSet('album', 'Front Cover', 'folder', { file: null, imageUrl: 'u' }, [
            'album/a.mp3'
        ])
        await session.save()
        expect(rescanWarnings()).toEqual([
            expect.objectContaining({ detail: 'picture reindex failed' })
        ])
    })

    it('stays silent when every picture write re-indexed cleanly', async () => {
        const session = mkSession()
        deletePictureSpy.mockResolvedValue({ ok: true, rescan: { ok: true } })
        session.stagePictureRemoval('album', 'Back Cover', 'folder', ['album/a.mp3'])
        await session.save()
        expect(rescanWarnings()).toEqual([])
    })

    it('aborts the save on the first failure, keeping the failed op staged', async () => {
        const session = mkSession()
        applyPictureSpy.mockRejectedValue(new Error('boom'))
        session.stagePictureSet(ALBUM, 'Back Cover', 'folder', { file: null, imageUrl: 'u' }, [
            'album/a.mp3'
        ])
        await session.save()
        expect(session.getPictureOp(ALBUM, 'Back Cover', 'folder')).toBeDefined()
        expect(session.hasStagedChanges.value).toBe(true)
    })

    it('flags the staged tracks in stagedPaths for embedded ops', () => {
        const tracks = [
            mkTrack({ path: 'album/a.mp3' }),
            mkTrack({ path: 'album/b.mp3' }),
            mkTrack({ path: 'album/c.mp3' })
        ]
        const session = mkSession(tracks)
        // Embedded op staged with a two-track selection: only those two flag.
        session.stagePictureSet(ALBUM, 'Back Cover', 'embedded', { file: null, imageUrl: 'u' }, [
            'album/a.mp3',
            'album/b.mp3'
        ])
        expect(session.stagedPaths.value.has('album/a.mp3')).toBe(true)
        expect(session.stagedPaths.value.has('album/b.mp3')).toBe(true)
        expect(session.stagedPaths.value.has('album/c.mp3')).toBe(false)
    })

    it('flags every track of the directory in stagedPaths for folder/db ops', () => {
        const tracks = [
            mkTrack({ path: 'album/a.mp3' }),
            mkTrack({ path: 'album/b.mp3' }),
            mkTrack({ path: 'other/x.mp3' })
        ]
        const session = mkSession(tracks)
        // A folder-art change belongs to the whole album, whoever was selected.
        session.stagePictureRemoval(ALBUM, 'Front Cover', 'folder', ['album/a.mp3'])
        expect(session.stagedPaths.value.has('album/a.mp3')).toBe(true)
        expect(session.stagedPaths.value.has('album/b.mp3')).toBe(true)
        expect(session.stagedPaths.value.has('other/x.mp3')).toBe(false)
    })

    it('clears stagedPaths again when the op is discarded or saved', async () => {
        const session = mkSession()
        session.stagePictureSet(ALBUM, 'Back Cover', 'db', { file: null, imageUrl: 'u' }, [
            'album/a.mp3'
        ])
        expect(session.stagedPaths.value.has('album/a.mp3')).toBe(true)
        session.discardPictureOp(ALBUM, 'Back Cover', 'db')
        expect(session.stagedPaths.value.has('album/a.mp3')).toBe(false)

        session.stagePictureRemoval(ALBUM, 'Back Cover', 'embedded', ['album/a.mp3'])
        expect(session.stagedPaths.value.has('album/a.mp3')).toBe(true)
        await session.save()
        expect(session.stagedPaths.value.has('album/a.mp3')).toBe(false)
    })

    // ----- Multi-folder (multi-disc) albums -----

    const discTracks = [
        mkTrack({ path: 'Release/CD 1/01.mp3', album: 'Sensaciones' }),
        mkTrack({ path: 'Release/CD 1/02.mp3', album: 'Sensaciones' }),
        mkTrack({ path: 'Release/CD 2/01.mp3', album: 'Sensaciones' })
    ]
    const discPaths = discTracks.map((t) => t.path)

    it('keys picture ops by album, so both disc folders share one cell', () => {
        const session = mkSession(discTracks)
        const key = albumKey(discTracks[0])
        session.stagePictureSet(
            key,
            'Front Cover',
            'folder',
            { file: null, imageUrl: 'http://img/x.jpg' },
            discPaths
        )
        // The op is reachable under the album's key regardless of which disc
        // folder the user had selected.
        expect(session.getPictureOp(key, 'Front Cover', 'folder')?.kind).toBe('set')
        expect(albumKey(discTracks[2])).toBe(key)
    })

    it('save sends every selected path so folder art lands in each disc folder', async () => {
        const session = mkSession(discTracks)
        session.stagePictureSet(
            albumKey(discTracks[0]),
            'Front Cover',
            'folder',
            { file: null, imageUrl: 'http://img/x.jpg' },
            discPaths
        )
        await session.save()
        const form = applyPictureSpy.mock.calls[0][0] as FormData
        expect(form.getAll('paths')).toEqual(discPaths)
    })

    it('save sends the paths on a folder removal too, so each disc folder is cleaned', async () => {
        const session = mkSession(discTracks)
        session.stagePictureRemoval(albumKey(discTracks[0]), 'Front Cover', 'folder', discPaths)
        await session.save()
        expect(deletePictureSpy).toHaveBeenCalledWith({
            libraryId: 3,
            path: 'Release/CD 1',
            type: 'Front Cover',
            slot: 'folder',
            paths: discPaths
        })
    })

    it('flags every track of the album in stagedPaths for a folder op', () => {
        const tracks = [...discTracks, mkTrack({ path: 'Other/01.mp3', album: 'Something Else' })]
        const session = mkSession(tracks)
        session.stagePictureRemoval(albumKey(discTracks[0]), 'Front Cover', 'folder', [
            'Release/CD 1/01.mp3'
        ])
        // Both disc folders belong to the album; the unrelated album does not.
        for (const p of discPaths) {
            expect(session.stagedPaths.value.has(p)).toBe(true)
        }
        expect(session.stagedPaths.value.has('Other/01.mp3')).toBe(false)
    })

    it('drops ops whose album no longer has any listed track', async () => {
        const tracks = ref<Track[]>(discTracks)
        const session = useEditSession(
            () => tracks.value,
            () => 3
        )
        session.stagePictureRemoval(albumKey(discTracks[0]), 'Front Cover', 'folder', discPaths)
        expect(session.hasStagedChanges.value).toBe(true)
        // Navigating to a folder with an unrelated album drops the stale op.
        tracks.value = [mkTrack({ path: 'Other/01.mp3', album: 'Something Else' })]
        await nextTick()
        expect(session.hasStagedChanges.value).toBe(false)
    })

    it('discardAll drops all picture ops and revokes previews', () => {
        const session = mkSession()
        const file = new File(['x'], 'art.png', { type: 'image/png' })
        session.stagePictureSet(ALBUM, 'Back Cover', 'folder', { file, imageUrl: null }, [
            'album/a.mp3'
        ])
        session.stagePictureRemoval(ALBUM, 'Media', 'db', ['album/a.mp3'])
        session.discardAll()
        expect(session.hasStagedChanges.value).toBe(false)
        expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:preview')
    })

    it('save drops the music-UI caches, not just the editor views', async () => {
        const track = mkTrack({ path: 'album/a.mp3', title: 'Original' })
        const session = mkSession([track])
        session.overlays.value.set('album/a.mp3', { title: 'Changed' })
        await session.save()
        const keys = invalidateSpy.mock.calls.map((c) => c[0].queryKey)
        expect(keys).toContainEqual(['metadata', 'tracks'])
        expect(keys).toContainEqual(['subsonic'])
    })
})

describe('candidateToOverlay', () => {
    const candidate: IdentifyCandidate = {
        score: 0.95,
        recording_mbid: 'rec-id',
        title: 'Song',
        artists: [{ name: 'Artist', mbid: 'artist-id' }],
        releases: [
            {
                release_mbid: 'rel-id',
                release_group_mbid: 'rg-id',
                album: 'Album',
                year: 1999,
                track_number: 5,
                disc_number: 1
            }
        ]
    }

    it('stages title, recording id and artists without a release', () => {
        const overlay = candidateToOverlay(candidate, null)
        expect(overlay).toEqual({
            title: 'Song',
            mb_recording_id: 'rec-id',
            artists: [{ name: 'Artist', mbid: 'artist-id' }]
        })
    })

    it('adds album fields when a release is picked', () => {
        const overlay = candidateToOverlay(candidate, candidate.releases[0])
        expect(overlay.album).toBe('Album')
        expect(overlay.mb_release_id).toBe('rel-id')
        expect(overlay.mb_release_group_id).toBe('rg-id')
        expect(overlay.year).toBe(1999)
        expect(overlay.track_number).toBe(5)
        expect(overlay.disc_number).toBe(1)
    })

    it('omits artists, year and positions when absent', () => {
        const overlay = candidateToOverlay(
            { ...candidate, artists: [] },
            {
                release_mbid: 'r',
                release_group_mbid: 'g',
                album: 'A',
                year: 0,
                track_number: 0,
                disc_number: 0
            }
        )
        expect(overlay.artists).toBeUndefined()
        expect(overlay.year).toBeUndefined()
        expect(overlay.track_number).toBeUndefined()
        expect(overlay.disc_number).toBeUndefined()
    })
})

describe('albumPickToOverlay', () => {
    const option: AlbumOption = {
        release_mbid: 'rel-id',
        release_group_mbid: 'rg-id',
        album: 'Album',
        year: 2000,
        artists: [{ name: 'Artist', mbid: 'artist-id' }],
        track_count: 10,
        disc_count: 1,
        matched_count: 8,
        mean_score: 0.92,
        enriched: true,
        tracks: [],
        assignments: []
    }

    // A Go nil slice marshals to JSON null, so `artists` can arrive null even
    // though the type declares an array. That must degrade to "no artists
    // staged", never throw and lose the whole apply.
    it('survives null artist lists on the option and the assignment', () => {
        const pick = {
            path: 'a.mp3',
            option: { ...option, artists: null },
            assignment: {
                path: 'a.mp3',
                source: 'fingerprint',
                title: 'Song',
                recording_mbid: 'rec-id',
                artists: null,
                disc_number: 1,
                track_number: 5,
                score: 0.95
            }
        } as unknown as AlbumIdentifyPick

        const overlay = albumPickToOverlay(pick)
        expect(overlay.album_artists).toBeUndefined()
        expect(overlay.artists).toBeUndefined()
        // The rest of the pick still stages normally.
        expect(overlay.album).toBe('Album')
        expect(overlay.title).toBe('Song')
        expect(overlay.track_number).toBe(5)
    })

    it('stages both album-level and song-level fields when assignment is present', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option,
            assignment: {
                path: 'a.mp3',
                source: 'fingerprint',
                title: 'Song',
                recording_mbid: 'rec-id',
                artists: [{ name: 'Song Artist', mbid: 'song-artist-id' }],
                disc_number: 1,
                track_number: 5,
                score: 0.95
            }
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.album).toBe('Album')
        expect(overlay.mb_release_id).toBe('rel-id')
        expect(overlay.mb_release_group_id).toBe('rg-id')
        expect(overlay.year).toBe(2000)
        expect(overlay.album_artists).toEqual([{ name: 'Artist', mbid: 'artist-id' }])
        expect(overlay.title).toBe('Song')
        expect(overlay.mb_recording_id).toBe('rec-id')
        expect(overlay.artists).toEqual([{ name: 'Song Artist', mbid: 'song-artist-id' }])
        expect(overlay.disc_number).toBe(1)
        expect(overlay.track_number).toBe(5)
    })

    it('stages ONLY album-level fields when assignment is null', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option,
            assignment: null
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.album).toBe('Album')
        expect(overlay.mb_release_id).toBe('rel-id')
        expect(overlay.mb_release_group_id).toBe('rg-id')
        expect(overlay.year).toBe(2000)
        expect(overlay.album_artists).toEqual([{ name: 'Artist', mbid: 'artist-id' }])
        expect(overlay.title).toBeUndefined()
        expect(overlay.mb_recording_id).toBeUndefined()
        expect(overlay.artists).toBeUndefined()
        expect(overlay.track_number).toBeUndefined()
        expect(overlay.disc_number).toBeUndefined()
    })

    it('omits year when zero', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option: { ...option, year: 0 },
            assignment: null
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.year).toBeUndefined()
        expect(overlay.album).toBe('Album')
    })

    it('omits track_number and disc_number when zero', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option,
            assignment: {
                path: 'a.mp3',
                source: 'fingerprint',
                title: 'Song',
                recording_mbid: 'rec-id',
                artists: [],
                disc_number: 0,
                track_number: 0,
                score: 0.95
            }
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.track_number).toBeUndefined()
        expect(overlay.disc_number).toBeUndefined()
        expect(overlay.title).toBe('Song')
    })

    it('omits empty album string', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option: { ...option, album: '' },
            assignment: null
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.album).toBeUndefined()
        expect(overlay.mb_release_id).toBe('rel-id')
    })

    it('omits empty title string', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option,
            assignment: {
                path: 'a.mp3',
                source: 'fingerprint',
                title: '',
                recording_mbid: 'rec-id',
                artists: [],
                disc_number: 1,
                track_number: 1,
                score: 0.95
            }
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.title).toBeUndefined()
        expect(overlay.mb_recording_id).toBe('rec-id')
    })

    it('omits empty recording_mbid string', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option,
            assignment: {
                path: 'a.mp3',
                source: 'fingerprint',
                title: 'Song',
                recording_mbid: '',
                artists: [],
                disc_number: 1,
                track_number: 1,
                score: 0.95
            }
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.mb_recording_id).toBeUndefined()
        expect(overlay.title).toBe('Song')
    })

    it('omits empty release_mbid and release_group_mbid strings', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option: { ...option, release_mbid: '', release_group_mbid: '' },
            assignment: null
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.mb_release_id).toBeUndefined()
        expect(overlay.mb_release_group_id).toBeUndefined()
        expect(overlay.album).toBe('Album')
    })

    it('omits album_artists when the list is empty', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option: { ...option, artists: [] },
            assignment: null
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.album_artists).toBeUndefined()
        expect(overlay.album).toBe('Album')
    })

    it('omits artists when the assignment list is empty', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option,
            assignment: {
                path: 'a.mp3',
                source: 'fingerprint',
                title: 'Song',
                recording_mbid: 'rec-id',
                artists: [],
                disc_number: 1,
                track_number: 1,
                score: 0.95
            }
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.artists).toBeUndefined()
        expect(overlay.title).toBe('Song')
    })

    it('never stages genres, compilation or disc_subtitle', () => {
        const pick: AlbumIdentifyPick = {
            path: 'a.mp3',
            option,
            assignment: {
                path: 'a.mp3',
                source: 'fingerprint',
                title: 'Song',
                recording_mbid: 'rec-id',
                artists: [],
                disc_number: 1,
                track_number: 1,
                score: 0.95
            }
        }
        const overlay = albumPickToOverlay(pick)
        expect(overlay.genres).toBeUndefined()
        expect(overlay.compilation).toBeUndefined()
        expect(overlay.disc_subtitle).toBeUndefined()
    })
})
