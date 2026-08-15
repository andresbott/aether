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

const isTouch = ref(false)
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ isTouch, tier: ref('desktop'), shell: ref('classic') })
}))

const toggleStarMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useAlbum: () => ({ data: albumData, isLoading: ref(false), error: ref(null) }),
    useToggleStar: () => ({ mutate: toggleStarMutate })
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
const addMultipleToQueue = vi.fn()
const enqueueAndPlayIfIdle = vi.fn()
const playNow = vi.fn()
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        playAlbum,
        addMultipleToQueue,
        enqueueAndPlayIfIdle,
        playNow,
        currentTrack: ref(null)
    })
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
            stubs: {
                RouterLink: true,
                TrackActionSheet: {
                    name: 'TrackActionSheet',
                    props: ['song', 'visible'],
                    template: '<div />'
                }
            }
        }
    })

beforeEach(() => {
    albumData.value = markRaw(album)
    isTouch.value = false
    playAlbum.mockClear()
    addMultipleToQueue.mockClear()
    enqueueAndPlayIfIdle.mockClear()
    playNow.mockClear()
    toggleStarMutate.mockClear()
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

    it('enqueues the track at the correct flat index when a disc-2 track is double-clicked', async () => {
        albumData.value = markRaw(multiDiscAlbum)
        const w = mountView()
        // Rows are ordered by disc; the 3rd (index 2) is disc 2's first track.
        const rows = w.findAll('.album-track-row')
        expect(rows).toHaveLength(3)
        await rows[2].trigger('dblclick')
        expect(enqueueAndPlayIfIdle).toHaveBeenCalledWith([multiDiscAlbum.song[2]])
        // Double-click appends — it must never replace the queue.
        expect(playAlbum).not.toHaveBeenCalled()
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

describe('AlbumView hero actions', () => {
    it('Play in the hero plays the album', async () => {
        const w = mountView()
        await w.find('.hero-action-play').trigger('click')
        expect(playAlbum).toHaveBeenCalledWith(album.song)
    })

    it('Add to queue in the hero enqueues the album songs', async () => {
        const w = mountView()
        await w.find('.hero-action-queue').trigger('click')
        expect(addMultipleToQueue).toHaveBeenCalledWith(album.song)
    })

    it('Star in the hero toggles the album star', async () => {
        const w = mountView()
        await w.find('.hero-action-star').trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 'al1', starred: false })
    })

    it('keeps only the drag handle in the scaffold actions', () => {
        const w = mountView()
        expect(w.find('.album-drag-handle').exists()).toBe(true)
        // Hero owns play/queue/star now; they render inside the HeroHeader.
        expect(w.find('.hero-header .hero-action-play').exists()).toBe(true)
    })
})

describe('AlbumView track favorites', () => {
    it('gives every track row its own favorite toggle', () => {
        const w = mountView()
        expect(w.findAll('.album-track-row .row-star')).toHaveLength(2)
    })

    // The hero star is the ALBUM's; a row star is that track's. Sharing a mutation
    // makes it easy to send the wrong id, so this pins the distinction.
    it('a row heart stars the track, not the album', async () => {
        const w = mountView()
        await w.findAll('.album-track-row .row-star')[1].trigger('click')
        expect(toggleStarMutate).toHaveBeenCalledWith({ id: 's2', starred: false })
    })

    it('the star column has a header cell so rows stay aligned', () => {
        const w = mountView()
        expect(w.find('.track-list-header .col-star').exists()).toBe(true)
    })
})

describe('AlbumView touch interactions', () => {
    // A tap must queue the whole VISIBLE list and start at the tapped track — the
    // touch counterpart of the hero Play. `playNow` would set queue=[song], so
    // tapping track 2 used to discard track 1 and everything already queued.
    it('tapping a row queues the album and starts at that track', async () => {
        isTouch.value = true
        const w = mountView()
        await w.findAll('.album-track-row')[1].trigger('click')
        expect(playAlbum).toHaveBeenCalledWith(album.song, 1)
        expect(playNow).not.toHaveBeenCalled()
    })

    // The queued list is the flat disc-ordered list, so the start index is the
    // row's position in it — not its position within its own disc.
    it('starts at the tapped track flat index across discs', async () => {
        const multiDiscAlbum = {
            id: 'al2',
            name: 'Double Album',
            artist: 'The Artist',
            song: [
                { id: 'd1t1', title: 'D1 One', discNumber: 1 },
                { id: 'd1t2', title: 'D1 Two', discNumber: 1 },
                { id: 'd2t1', title: 'D2 One', discNumber: 2 }
            ]
        }
        albumData.value = markRaw(multiDiscAlbum)
        isTouch.value = true
        const w = mountView()
        await w.findAll('.album-track-row')[2].trigger('click')
        expect(playAlbum).toHaveBeenCalledWith(multiDiscAlbum.song, 2)
    })

    it('opens the action sheet from ⋮', async () => {
        isTouch.value = true
        const w = mountView()
        await w.findAll('[aria-label="Track actions"]')[0].trigger('click')
        expect(w.findComponent({ name: 'TrackActionSheet' }).props('visible')).toBe(true)
        // The ⋮ is not a play affordance.
        expect(playAlbum).not.toHaveBeenCalled()
    })
})
