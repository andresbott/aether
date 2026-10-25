import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

const start = vi.fn()
const end = vi.fn()
vi.mock('@/composables/useAlbumDrag', () => ({
    useAlbumDrag: () => ({ start, end })
}))
vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`
    }
}))

const starMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: starMutate })
}))

import AlbumRow from '@/components/library/AlbumRow.vue'
import type { Album } from '@/types/subsonic'

const album: Album = {
    id: 'al1',
    name: 'Album One',
    artist: 'The Artist',
    coverArt: 'ca1',
    songCount: 9,
    duration: 125
}

const mountRow = (a?: Album) =>
    mount(AlbumRow, { props: { album: a }, global: { stubs: { RouterLink: RouterLinkStub } } })

beforeEach(() => {
    start.mockReset()
    end.mockReset()
    starMutate.mockReset()
})

const metaParts = (w: ReturnType<typeof mountRow>) =>
    w.findAll('.card-meta .meta-part').map((p) => p.text())
const detailParts = (w: ReturnType<typeof mountRow>) =>
    w.findAll('.card-details .meta-part').map((p) => p.text())

describe('AlbumRow', () => {
    it('renders the title, the artist and a meta line of song count and running time', () => {
        const w = mountRow(album)
        expect(w.find('.card-title').text()).toBe('Album One')
        expect(w.find('.card-subtitle').text()).toBe('The Artist')
        expect(metaParts(w)).toEqual(['9 songs', '2 min'])
    })

    it('counts a single song in the singular', () => {
        expect(metaParts(mountRow({ ...album, songCount: 1 }))).toEqual(['1 song', '2 min'])
    })

    it('leaves out what the album does not report', () => {
        const w = mountRow({ id: 'al2', name: 'Bare' })
        expect(w.find('.card-subtitle').exists()).toBe(false)
        expect(w.find('.card-facts').exists()).toBe(false)
    })

    const full: Album = { ...album, year: 2004, genre: 'Latin Pop', releaseTypes: ['EP'] }

    it('puts the year before the size, and the type and the genre on a second line', () => {
        const w = mountRow(full)
        expect(metaParts(w)).toEqual(['2004', '9 songs', '2 min'])
        expect(detailParts(w)).toEqual(['EP', 'Latin Pop'])
    })

    it('names no type for a plain album, and a compilation by its flag', () => {
        expect(detailParts(mountRow({ ...full, releaseTypes: ['Album'] }))).toEqual(['Latin Pop'])
        const flagged = { ...full, releaseTypes: undefined, isCompilation: true }
        expect(detailParts(mountRow(flagged))).toEqual(['Compilation', 'Latin Pop'])
    })

    // The server sends year 0 for an album without one.
    it('leaves out a zero year, and the second line with nothing on it', () => {
        const w = mountRow({ ...album, year: 0 })
        expect(metaParts(w)).toEqual(['9 songs', '2 min'])
        expect(w.find('.card-details').exists()).toBe(false)
    })

    // The grid card's size, so switching layouts reuses the one cached image.
    it('links to the album detail route and starts the drag with the cover', async () => {
        const w = mountRow(album)
        const link = w.findComponent(RouterLinkStub)
        expect(link.props('to')).toEqual({ name: 'album', params: { id: 'al1' } })
        await link.trigger('dragstart')
        expect(start).toHaveBeenCalledTimes(1)
        expect(start.mock.calls[0][1].id).toBe('al1')
        expect(start.mock.calls[0][2]).toBe('cover:ca1:200')
    })

    it('renders a placeholder card while the album is not loaded', () => {
        const w = mountRow(undefined)
        expect(w.findComponent(RouterLinkStub).exists()).toBe(false)
        expect(w.find('.album-row.placeholder').exists()).toBe(true)
    })

    it('renders no favorite toggle on the placeholder', () => {
        expect(mountRow(undefined).find('.row-star').exists()).toBe(false)
    })
})

describe('AlbumRow favorite toggle', () => {
    it('renders the heart on the card', () => {
        expect(mountRow(album).find('.list-card .row-star').exists()).toBe(true)
    })

    it('shows an outline heart when unstarred and a filled one when starred', () => {
        expect(mountRow(album).find('.row-star i').classes()).toContain('mso-favorite')
        expect(
            mountRow({ ...album, starred: '2026-02-01T00:00:00Z' })
                .find('.row-star i')
                .classes()
        ).toContain('ms-favorite')
    })

    it('labels the toggle by the current state', () => {
        expect(mountRow(album).find('.row-star').attributes('aria-label')).toBe('Add to favorites')
        expect(
            mountRow({ ...album, starred: '2026-02-01T00:00:00Z' })
                .find('.row-star')
                .attributes('aria-label')
        ).toBe('Remove from favorites')
    })

    it('keeps a starred album heart visible without hover', () => {
        expect(
            mountRow({ ...album, starred: '2026-02-01T00:00:00Z' })
                .find('.row-star')
                .classes()
        ).toContain('is-starred')
        expect(mountRow(album).find('.row-star').classes()).not.toContain('is-starred')
    })

    it('toggles with the album id and its current state', async () => {
        const w = mountRow({ ...album, starred: '2026-02-01T00:00:00Z' })
        await w.find('.row-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 'al1', starred: true })
    })

    // The row is a router-link, so the toggle must not navigate to the album.
    it('does not navigate when the heart is clicked', () => {
        const w = mountRow(album)
        const click = new MouseEvent('click', { bubbles: true, cancelable: true })
        w.find('.row-star').element.dispatchEvent(click)
        expect(click.defaultPrevented).toBe(true)
    })
})
