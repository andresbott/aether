import { describe, it, expect, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`
    }
}))

import ArtistRow from '@/components/library/ArtistRow.vue'
import type { Artist } from '@/types/subsonic'

const artist: Artist = { id: 'ar1', name: 'Radiohead', coverArt: 'ca1', albumCount: 9 }

const mountRow = (a?: Artist) =>
    mount(ArtistRow, { props: { artist: a }, global: { stubs: { RouterLink: RouterLinkStub } } })

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

    it('omits the count when albumCount is undefined', () => {
        const w = mountRow({ id: 'ar2', name: 'Air' })
        expect(w.find('.col-count').exists()).toBe(false)
    })
})
