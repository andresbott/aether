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

describe('AlbumRow', () => {
    it('renders title, artist, song count and formatted duration', () => {
        const w = mountRow(album)
        expect(w.text()).toContain('Album One')
        expect(w.text()).toContain('The Artist')
        expect(w.find('.col-songs').text()).toBe('9')
        expect(w.text()).toContain('2:05')
    })

    it('links to the album detail route and starts the drag', async () => {
        const w = mountRow(album)
        const link = w.findComponent(RouterLinkStub)
        expect(link.props('to')).toEqual({ name: 'album', params: { id: 'al1' } })
        await link.trigger('dragstart')
        expect(start).toHaveBeenCalledTimes(1)
        expect(start.mock.calls[0][1].id).toBe('al1')
        expect(start.mock.calls[0][2]).toBe('cover:ca1:80')
    })

    it('renders a placeholder row while the album is not loaded', () => {
        const w = mountRow(undefined)
        expect(w.findComponent(RouterLinkStub).exists()).toBe(false)
        expect(w.find('.album-row.placeholder').exists()).toBe(true)
    })

    it('renders no favorite toggle on the placeholder', () => {
        expect(mountRow(undefined).find('.row-star').exists()).toBe(false)
    })
})

describe('AlbumRow favorite toggle', () => {
    // In its own column, mirrored by PlaylistRow and both list headers, so a
    // Discovery feed interleaving albums and playlists lines its columns up.
    it('renders the heart in its own column', () => {
        expect(mountRow(album).find('.col-star .row-star').exists()).toBe(true)
    })

    it('shows an outline heart when unstarred and a filled one when starred', () => {
        expect(mountRow(album).find('.row-star i').classes()).toContain('pi-heart')
        expect(
            mountRow({ ...album, starred: '2026-02-01T00:00:00Z' })
                .find('.row-star i')
                .classes()
        ).toContain('pi-heart-fill')
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
