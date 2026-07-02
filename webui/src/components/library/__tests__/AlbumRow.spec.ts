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
})
