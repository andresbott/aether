import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import type { AlbumWithSongs, Song } from '@/types/subsonic'

// Mutable so each test controls the album the card resolves its disc title from.
const albumData = ref<AlbumWithSongs | null>(null)
const useAlbumIds: (string | undefined)[] = []

vi.mock('@/composables/useSubsonicQueries', () => ({
    useAlbum: (id: () => string | undefined) => {
        useAlbumIds.push(typeof id === 'function' ? id() : id)
        return { data: albumData }
    },
    useToggleStar: () => ({ mutate: vi.fn() })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (art: string, size: number) => `cover:${art}:${size}`
    }
}))

import SongDetail from '@/components/library/SongDetail.vue'

const song = (over: Partial<Song> = {}): Song => ({
    id: 's1',
    title: 'One',
    album: 'Box Set',
    albumId: 'al1',
    artist: 'The Artist',
    ...over
})

const mountCard = (s: Song, card = true) =>
    mount(SongDetail, {
        props: { song: s, card },
        global: {
            plugins: [PrimeVue],
            stubs: { RouterLink: { template: '<a><slot /></a>' } }
        }
    })

beforeEach(() => {
    albumData.value = null
    useAlbumIds.length = 0
})

describe('SongDetail disc label', () => {
    it('shows nothing extra when the disc number is unset', () => {
        const w = mountCard(song())
        expect(w.find('.disc-label').exists()).toBe(false)
    })

    it('treats disc 0 as unset', () => {
        const w = mountCard(song({ discNumber: 0 }))
        expect(w.find('.disc-label').exists()).toBe(false)
    })

    it('shows the disc number next to the album title', () => {
        const w = mountCard(song({ discNumber: 2 }))
        expect(w.find('.disc-label').text()).toBe('Disc 2')
        expect(w.find('.album-line').text()).toContain('Box Set')
    })

    it('appends the matching disc subtitle from the album', () => {
        albumData.value = {
            id: 'al1',
            name: 'Box Set',
            discTitles: [
                { disc: 1, title: 'The Album' },
                { disc: 2, title: 'Bonus Tracks' }
            ]
        }
        const w = mountCard(song({ discNumber: 2 }))
        expect(w.find('.disc-label').text()).toBe('Disc 2 · Bonus Tracks')
    })

    it('shows the subtitle alone when the disc number is unset', () => {
        albumData.value = {
            id: 'al1',
            name: 'Box Set',
            discTitles: [{ disc: 0, title: 'Original Score' }]
        }
        const w = mountCard(song())
        expect(w.find('.disc-label').text()).toBe('Original Score')
    })

    it('ignores disc titles for other discs', () => {
        albumData.value = {
            id: 'al1',
            name: 'Box Set',
            discTitles: [{ disc: 3, title: 'Rarities' }]
        }
        const w = mountCard(song({ discNumber: 2 }))
        expect(w.find('.disc-label').text()).toBe('Disc 2')
    })

    it('does not fetch the album outside the card variant', () => {
        mountCard(song({ discNumber: 2 }), false)
        expect(useAlbumIds).toEqual([undefined])
    })

    it('fetches the song album for the card variant', () => {
        mountCard(song({ discNumber: 2 }))
        expect(useAlbumIds).toEqual(['al1'])
    })
})
