import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

const start = vi.fn()
const end = vi.fn()
vi.mock('@/composables/useAlbumDrag', () => ({
    useAlbumDrag: () => ({ start, end })
}))

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

import AlbumCard from '@/components/library/AlbumCard.vue'
import type { Album } from '@/types/subsonic'

const album: Album = {
    id: 'al1',
    name: 'Album One',
    artist: 'The Artist',
    coverArt: 'ca1',
    songCount: 9
}

const mountCard = (over: Partial<Album> = {}) =>
    mount(AlbumCard, {
        props: { album: { ...album, ...over } },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

beforeEach(() => {
    starMutate.mockReset()
})

describe('AlbumCard drag source', () => {
    it('makes the card draggable and the cover image non-draggable', () => {
        const w = mountCard()
        expect(w.findComponent(RouterLinkStub).attributes('draggable')).toBe('true')
        expect(w.find('img').attributes('draggable')).toBe('false')
    })

    it('starts the album drag with the album and cover URL on dragstart', async () => {
        const w = mountCard()
        await w.findComponent(RouterLinkStub).trigger('dragstart')
        expect(start).toHaveBeenCalledTimes(1)
        const call = start.mock.calls[0]
        expect(call[1].id).toBe('al1')
        expect(call[2]).toBe('cover:ca1:200')
    })

    it('still links to the album detail route', () => {
        const w = mountCard()
        expect(w.findComponent(RouterLinkStub).props('to')).toEqual({
            name: 'album',
            params: { id: 'al1' }
        })
    })
})

describe('AlbumCard favorite toggle', () => {
    it('shows an outline heart when unstarred and a filled one when starred', () => {
        expect(mountCard().find('.card-star i').classes()).toContain('pi-heart')
        expect(mountCard({ starred: '2026-02-01T00:00:00Z' }).find('.card-star i').classes()).toContain(
            'pi-heart-fill'
        )
    })

    it('labels the toggle by the current state', () => {
        expect(mountCard().find('.card-star').attributes('aria-label')).toBe('Add to favorites')
        expect(
            mountCard({ starred: '2026-02-01T00:00:00Z' }).find('.card-star').attributes('aria-label')
        ).toBe('Remove from favorites')
    })

    it('keeps a starred album heart visible without hover', () => {
        expect(mountCard({ starred: '2026-02-01T00:00:00Z' }).find('.card-star').classes()).toContain(
            'is-starred'
        )
        expect(mountCard().find('.card-star').classes()).not.toContain('is-starred')
    })

    it('toggles with the album id and its current state', async () => {
        const w = mountCard({ starred: '2026-02-01T00:00:00Z' })
        await w.find('.card-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 'al1', starred: true })
    })

    // The card is a router-link, so the toggle must not navigate to the album.
    it('does not navigate when the heart is clicked', async () => {
        const w = mountCard()
        const click = new MouseEvent('click', { bubbles: true, cancelable: true })
        w.find('.card-star').element.dispatchEvent(click)
        expect(click.defaultPrevented).toBe(true)
    })
})

describe('AlbumCard placeholder', () => {
    it('renders a placeholder with no link or image when album is undefined', () => {
        const w = mount(AlbumCard, {
            props: {},
            global: { stubs: { RouterLink: RouterLinkStub } }
        })
        expect(w.find('.album-card.placeholder').exists()).toBe(true)
        expect(w.findComponent(RouterLinkStub).exists()).toBe(false)
        expect(w.find('img').exists()).toBe(false)
    })

    it('renders no favorite toggle on the placeholder', () => {
        const w = mount(AlbumCard, {
            props: {},
            global: { stubs: { RouterLink: RouterLinkStub } }
        })
        expect(w.find('.card-star').exists()).toBe(false)
    })
})
