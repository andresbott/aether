import { describe, it, expect, vi } from 'vitest'
import router from '@/router'

// The Library browse mode (Discover / Albums / Artists) is a real path segment,
// not a URL hash. Discover is the bare cross-collection root; Albums and Artists
// are constrained sub-paths that also compose with a numeric folder segment.
describe('library routes', () => {
    it('resolves the root browse modes to path segments, not a hash', () => {
        const root = router.resolve('/library')
        expect(root.name).toBe('library')
        // An absent optional mode reports as '' (see the settings :tab convention).
        expect(root.params.mode).toBe('')

        const albums = router.resolve('/library/albums')
        expect(albums.name).toBe('library')
        expect(albums.params.mode).toBe('albums')

        const artists = router.resolve('/library/artists')
        expect(artists.name).toBe('library')
        expect(artists.params.mode).toBe('artists')
    })

    it('resolves a numeric folder to its own route, with an optional trailing mode', () => {
        const folder = router.resolve('/library/5')
        expect(folder.name).toBe('library-folder')
        expect(folder.params.folderId).toBe('5')
        expect(folder.params.mode).toBe('')

        const folderArtists = router.resolve('/library/5/artists')
        expect(folderArtists.name).toBe('library-folder')
        expect(folderArtists.params.folderId).toBe('5')
        expect(folderArtists.params.mode).toBe('artists')
    })

    // The mode segment is constrained to the view names and the folder segment
    // to digits, so the two axes never collide: a word is always a mode, a
    // number is always a folder.
    it('keeps the folder and mode axes disjoint', () => {
        const albums = router.resolve('/library/albums')
        expect(albums.name).toBe('library')
        expect(albums.params.folderId).toBeUndefined()
    })

    // The root's Discover is only ever the bare path — there is no
    // /library/discover. A library's own Discover is a segment like its other
    // views, since a library may open on a different one.
    it('addresses discover as a segment inside a library only', () => {
        const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
        expect(router.resolve('/library/discover').matched.length).toBe(0)
        warnSpy.mockRestore()

        const folderDiscover = router.resolve('/library/5/discover')
        expect(folderDiscover.name).toBe('library-folder')
        expect(folderDiscover.params.folderId).toBe('5')
        expect(folderDiscover.params.mode).toBe('discover')
    })

    // Per-folder deep links (e.g. BrowseAlbumShelf) navigate by name; the folderId
    // param must land on the folder route.
    it('resolves the folder route by name with a folderId param', () => {
        expect(
            router.resolve({ name: 'library-folder', params: { folderId: '5' } }).path
        ).toBe('/library/5')
    })
})
