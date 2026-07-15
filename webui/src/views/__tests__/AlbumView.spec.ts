import { describe, it, expect, vi, beforeEach } from 'vitest'
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

// Mutable ref so individual tests can swap in a multi-disc album.
const albumData = ref<unknown>(markRaw(album))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useAlbum: () => ({ data: albumData, isLoading: ref(false), error: ref(null) }),
    useToggleStar: () => ({ mutate: vi.fn() })
}))

const start = vi.fn()
const end = vi.fn()
vi.mock('@/composables/useAlbumDrag', () => ({
    useAlbumDrag: () => ({ start, end })
}))

const songsStart = vi.fn()
const songsEnd = vi.fn()
vi.mock('@/composables/useSongsDrag', () => ({
    useSongsDrag: () => ({ start: songsStart, end: songsEnd })
}))

const playAlbum = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ playAlbum, addMultipleToQueue: vi.fn(), currentTrack: ref(null) })
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

beforeEach(() => {
    albumData.value = markRaw(album)
    playAlbum.mockClear()
    songsStart.mockClear()
    songsEnd.mockClear()
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

describe('AlbumView disc grouping', () => {
    const multiDiscAlbum = {
        id: 'al2',
        name: 'Double Album',
        artist: 'The Artist',
        coverArt: 'ca2',
        song: [
            { id: 'd1t1', title: 'D1 One', discNumber: 1 },
            { id: 'd1t2', title: 'D1 Two', discNumber: 1 },
            { id: 'd2t1', title: 'D2 One', discNumber: 2 }
        ]
    }

    it('renders column headers above the track list', () => {
        const w = mountView()
        const header = w.find('.track-list-header')
        expect(header.exists()).toBe(true)
        expect(header.text()).toContain('Title')
        expect(header.text()).toContain('Artist')
        // The duration column header is a clock icon (labelled for a11y).
        expect(header.find('.pi-clock').exists()).toBe(true)
    })

    it('renders no disc header for a single-disc album', () => {
        const w = mountView()
        expect(w.findAll('.disc-header')).toHaveLength(0)
        expect(w.findAll('.album-track-row')).toHaveLength(2)
    })

    it('renders a disc header per disc for a multi-disc album', () => {
        albumData.value = markRaw(multiDiscAlbum)
        const w = mountView()
        const headers = w.findAll('.disc-header')
        expect(headers).toHaveLength(2)
        expect(headers[0].text()).toBe('Disc 1')
        expect(headers[1].text()).toBe('Disc 2')
        expect(w.findAll('.album-track-row')).toHaveLength(3)
    })

    it('plays from the correct flat index when a disc-2 track is double-clicked', async () => {
        albumData.value = markRaw(multiDiscAlbum)
        const w = mountView()
        // Rows are ordered by disc; the 3rd (index 2) is disc 2's first track.
        const rows = w.findAll('.album-track-row')
        expect(rows).toHaveLength(3)
        await rows[2].trigger('dblclick')
        expect(playAlbum).toHaveBeenCalledWith(multiDiscAlbum.song, 2)
    })
})

describe('AlbumView song selection and drag', () => {
    it('highlights a row on click and carries it on dragstart', async () => {
        const w = mountView()
        const rows = w.findAll('.album-track-row')
        await rows[0].trigger('click')
        expect(rows[0].classes()).toContain('selected')

        await rows[0].trigger('dragstart')
        expect(songsStart).toHaveBeenCalledTimes(1)
        expect(songsStart.mock.calls[0][1]).toEqual([album.song[0]])
    })

    it('drags the whole multi-selection when a selected row is grabbed', async () => {
        const w = mountView()
        const rows = w.findAll('.album-track-row')
        await rows[0].trigger('click')
        // Ctrl-click adds the second row to the selection.
        await rows[1].trigger('click', { ctrlKey: true })

        await rows[1].trigger('dragstart')
        expect(songsStart.mock.calls[0][1]).toEqual([album.song[0], album.song[1]])
    })

    it('ends the songs drag on dragend', async () => {
        const w = mountView()
        await w.findAll('.album-track-row')[0].trigger('dragend')
        expect(songsEnd).toHaveBeenCalledTimes(1)
    })
})
