import { describe, it, expect, vi } from 'vitest'
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

import AlbumCard from '@/components/library/AlbumCard.vue'
import type { Album } from '@/types/subsonic'

const album: Album = {
    id: 'al1',
    name: 'Album One',
    artist: 'The Artist',
    coverArt: 'ca1',
    songCount: 9
}

const mountCard = () =>
    mount(AlbumCard, {
        props: { album },
        global: { stubs: { RouterLink: RouterLinkStub } }
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
})
