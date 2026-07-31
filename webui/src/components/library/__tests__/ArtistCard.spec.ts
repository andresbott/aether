import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

const starMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: starMutate })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`
    }
}))

import ArtistCard from '@/components/library/ArtistCard.vue'
import type { Artist } from '@/types/subsonic'

const artist: Artist = {
    id: 'ar1',
    name: 'The Artist',
    coverArt: 'ar1',
    albumCount: 3
}

const mountCard = (over: Partial<Artist> = {}) =>
    mount(ArtistCard, {
        props: { artist: { ...artist, ...over } },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

beforeEach(() => {
    starMutate.mockReset()
})

describe('ArtistCard', () => {
    it('links to the artist detail route', () => {
        expect(mountCard().findComponent(RouterLinkStub).props('to')).toEqual({
            name: 'artist',
            params: { id: 'ar1' }
        })
    })

    it('pluralizes the album count', () => {
        expect(mountCard().find('.card-subtitle').text()).toBe('3 albums')
        expect(mountCard({ albumCount: 1 }).find('.card-subtitle').text()).toBe('1 album')
    })
})

describe('ArtistCard favorite toggle', () => {
    it('shows an outline heart when unstarred and a filled one when starred', () => {
        expect(mountCard().find('.card-star i').classes()).toContain('pi-heart')
        expect(
            mountCard({ starred: '2026-02-01T00:00:00Z' }).find('.card-star i').classes()
        ).toContain('pi-heart-fill')
    })

    it('labels the toggle by the current state', () => {
        expect(mountCard().find('.card-star').attributes('aria-label')).toBe('Add to favorites')
        expect(
            mountCard({ starred: '2026-02-01T00:00:00Z' })
                .find('.card-star')
                .attributes('aria-label')
        ).toBe('Remove from favorites')
    })

    it('keeps a starred artist heart visible without hover', () => {
        expect(
            mountCard({ starred: '2026-02-01T00:00:00Z' }).find('.card-star').classes()
        ).toContain('is-starred')
        expect(mountCard().find('.card-star').classes()).not.toContain('is-starred')
    })

    it('toggles with the artist id and its current state', async () => {
        const w = mountCard({ starred: '2026-02-01T00:00:00Z' })
        await w.find('.card-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 'ar1', starred: true })
    })

    // The card is a router-link, so the toggle must not navigate to the artist.
    it('does not navigate when the heart is clicked', () => {
        const w = mountCard()
        const click = new MouseEvent('click', { bubbles: true, cancelable: true })
        w.find('.card-star').element.dispatchEvent(click)
        expect(click.defaultPrevented).toBe(true)
    })
})
