import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, markRaw } from 'vue'
import PrimeVue from 'primevue/config'

const album = {
    id: 'al1',
    name: 'Album One',
    artist: 'The Artist',
    coverArt: 'ca1',
    song: [
        { id: 's1', title: 'One' },
        { id: 's2', title: 'Two' }
    ]
}

vi.mock('@/composables/useSubsonicQueries', () => ({
    useAlbum: () => ({ data: ref(markRaw(album)), isLoading: ref(false), error: ref(null) }),
    useToggleStar: () => ({ mutate: vi.fn() })
}))

const start = vi.fn()
const end = vi.fn()
vi.mock('@/composables/useAlbumDrag', () => ({
    useAlbumDrag: () => ({ start, end })
}))

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum: vi.fn(), addMultipleToQueue: vi.fn() })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`
    }
}))

vi.mock('vue-router', () => ({
    useRouter: () => ({ back: vi.fn() })
}))

import AlbumView from '@/views/AlbumView.vue'

const mountView = () =>
    mount(AlbumView, {
        props: { id: 'al1' },
        global: {
            plugins: [PrimeVue],
            directives: { tooltip: {} },
            stubs: { RouterLink: true }
        }
    })

describe('AlbumView album drag', () => {
    it('renders a draggable handle in the album actions', () => {
        const w = mountView()
        const handle = w.find('.album-drag-handle')
        expect(handle.exists()).toBe(true)
        expect(handle.attributes('draggable')).toBe('true')
    })

    it('starts the album drag with the album and cover URL on dragstart', async () => {
        const w = mountView()
        await w.find('.album-drag-handle').trigger('dragstart')
        expect(start).toHaveBeenCalledTimes(1)
        const call = start.mock.calls[0]
        expect(call[1]).toBe(album)
        expect(call[2]).toBe('cover:ca1:250')
    })
})
