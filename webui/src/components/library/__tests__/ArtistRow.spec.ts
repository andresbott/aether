import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

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

import ArtistRow from '@/components/library/ArtistRow.vue'
import type { Artist } from '@/types/subsonic'

const artist: Artist = { id: 'ar1', name: 'Radiohead', coverArt: 'ca1', albumCount: 9 }

const mountRow = (a?: Artist) =>
    mount(ArtistRow, { props: { artist: a }, global: { stubs: { RouterLink: RouterLinkStub } } })

beforeEach(() => {
    starMutate.mockReset()
})

describe('ArtistRow', () => {
    it('renders avatar (size 80), name and album count', () => {
        const w = mountRow(artist)
        expect(w.find('img').attributes('src')).toBe('cover:ca1:80')
        expect(w.text()).toContain('Radiohead')
        expect(w.find('.col-count').text()).toBe('9 albums')
    })

    it('links to the artist detail route', () => {
        const w = mountRow(artist)
        expect(w.findComponent(RouterLinkStub).props('to')).toEqual({ name: 'artist', params: { id: 'ar1' } })
    })

    it('renders a placeholder row when the artist is not loaded', () => {
        const w = mountRow(undefined)
        expect(w.findComponent(RouterLinkStub).exists()).toBe(false)
        expect(w.find('.artist-row.placeholder').exists()).toBe(true)
    })

    // The cell stays (it holds the grid slot, or the heart would slide into the
    // count column) but carries no text.
    it('omits the count text when albumCount is undefined', () => {
        const w = mountRow({ id: 'ar2', name: 'Air' })
        expect(w.find('.col-count').text()).toBe('')
    })
})

describe('ArtistRow favorite toggle', () => {
    // In its own column, mirrored by ArtistListView's header, so the heart sits at
    // the same size as on the album rows.
    it('renders the heart in its own column', () => {
        expect(mountRow(artist).find('.col-star .row-star').exists()).toBe(true)
    })

    it('shows an outline heart when unstarred and a filled one when starred', () => {
        expect(mountRow(artist).find('.row-star i').classes()).toContain('pi-heart')
        expect(
            mountRow({ ...artist, starred: '2026-02-01T00:00:00Z' }).find('.row-star i').classes()
        ).toContain('pi-heart-fill')
    })

    it('labels the toggle by the current state', () => {
        expect(mountRow(artist).find('.row-star').attributes('aria-label')).toBe('Add to favorites')
        expect(
            mountRow({ ...artist, starred: '2026-02-01T00:00:00Z' })
                .find('.row-star')
                .attributes('aria-label')
        ).toBe('Remove from favorites')
    })

    it('keeps a starred artist heart visible without hover', () => {
        expect(
            mountRow({ ...artist, starred: '2026-02-01T00:00:00Z' }).find('.row-star').classes()
        ).toContain('is-starred')
        expect(mountRow(artist).find('.row-star').classes()).not.toContain('is-starred')
    })

    it('toggles with the artist id and its current state', async () => {
        const w = mountRow({ ...artist, starred: '2026-02-01T00:00:00Z' })
        await w.find('.row-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 'ar1', starred: true })
    })

    // The row is a router-link, so the toggle must not navigate to the artist.
    it('does not navigate when the heart is clicked', () => {
        const w = mountRow(artist)
        const click = new MouseEvent('click', { bubbles: true, cancelable: true })
        w.find('.row-star').element.dispatchEvent(click)
        expect(click.defaultPrevented).toBe(true)
    })

    it('renders no favorite toggle on the placeholder', () => {
        expect(mountRow(undefined).find('.row-star').exists()).toBe(false)
    })
})
