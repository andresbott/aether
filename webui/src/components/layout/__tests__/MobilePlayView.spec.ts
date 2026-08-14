import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRouter: () => ({ push })
}))

// ContentScaffold decides the hamburger from the viewport singleton.
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ shell: ref('mobile'), tier: ref('phone'), isTouch: ref(true) })
}))

const song = (id: string, extra: Record<string, unknown> = {}) => ({
    id,
    title: `Song ${id}`,
    artist: 'Artist',
    album: 'Album',
    duration: 60,
    ...extra
})

const queue = ref<Array<Record<string, unknown>>>([])
const currentTrack = ref<Record<string, unknown> | null>(null)
const isPlaying = ref(false)
const shuffle = ref(false)
const repeat = ref<'none' | 'all' | 'one'>('none')
const seek = vi.fn()
const togglePlayPause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()
const toggleShuffle = vi.fn()
const toggleRepeat = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue,
        currentTrack,
        isPlaying,
        shuffle,
        repeat,
        currentTime: ref(30),
        duration: ref(120),
        hasNext: ref(true),
        hasPrevious: ref(true),
        seek,
        togglePlayPause,
        playNext,
        playPrevious,
        toggleShuffle,
        toggleRepeat
    })
}))

const toggleFavorite = vi.fn()
const isStarred = ref(false)
vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred, toggleFavorite })
}))

const openSaveDialog = vi.fn()
const clearQueue = vi.fn()
vi.mock('@/composables/useQueueActions', () => ({
    useQueueActions: () => ({
        showSaveDialog: ref(false),
        playlistName: ref(''),
        openSaveDialog,
        handleSave: vi.fn(),
        isSaving: ref(false),
        clearQueue
    })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

vi.mock('@/components/layout/QueueBody.vue', () => ({
    default: {
        name: 'QueueBody',
        props: ['variant', 'editMode'],
        template: '<div class="stub-queue-body">{{ variant }}</div>'
    }
}))

vi.mock('@/components/layout/SavePlaylistDialog.vue', () => ({
    default: { name: 'SavePlaylistDialog', template: '<div class="stub-save-dialog"></div>' }
}))

import MobilePlayView from '@/components/layout/MobilePlayView.vue'

const mountView = () =>
    mount(MobilePlayView, { global: { plugins: [PrimeVue], directives: { tooltip: {} } } })

beforeEach(() => {
    queue.value = [
        song('1'),
        song('2', { albumId: 'al2', artistId: 'ar2', coverArt: 'cov-2' }),
        song('3')
    ]
    currentTrack.value = queue.value[1]
    isPlaying.value = false
    shuffle.value = false
    repeat.value = 'none'
    isStarred.value = false
    vi.clearAllMocks()
})

describe('MobilePlayView', () => {
    it('renders the scaffold header with title, summary and the nav hamburger', () => {
        const w = mountView()
        expect(w.find('h1').text()).toBe('Now Playing')
        expect(w.find('.scaffold-summary').text()).toBe('3 tracks • 3 min')
        // First-class screen: the drawer trigger, not a dismiss chevron.
        expect(w.find('[aria-label="Open navigation"]').exists()).toBe(true)
    })

    it('shows track, artist and cover art on the player face', () => {
        const w = mountView()
        expect(w.find('.play-title').text()).toBe('Song 2')
        expect(w.find('.play-artist').text()).toBe('Artist')
        expect(w.find('img.play-cover').attributes('src')).toBe('/art/cov-2?size=512')
    })

    it('wires the transport to the player', async () => {
        const w = mountView()
        await w.find('[aria-label="Play"]').trigger('click')
        expect(togglePlayPause).toHaveBeenCalledOnce()
        await w.find('[aria-label="Next track"]').trigger('click')
        expect(playNext).toHaveBeenCalledOnce()
        await w.find('[aria-label="Previous track"]').trigger('click')
        expect(playPrevious).toHaveBeenCalledOnce()
        await w.find('[aria-label="Shuffle"]').trigger('click')
        expect(toggleShuffle).toHaveBeenCalledOnce()
        await w.find('[aria-label="Repeat"]').trigger('click')
        expect(toggleRepeat).toHaveBeenCalledOnce()
    })

    it('seeking through the range input calls seek', async () => {
        const w = mountView()
        const range = w.find('input[type="range"]')
        await range.setValue('45')
        expect(seek).toHaveBeenCalledWith(45)
    })

    it('title and artist navigate to the album and artist routes', async () => {
        const w = mountView()
        await w.find('.play-title').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'album', params: { id: 'al2' } })
        await w.find('.play-artist').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'artist', params: { id: 'ar2' } })
    })

    it('disables the title/artist links when the track has no ids', () => {
        currentTrack.value = queue.value[0] // no albumId/artistId
        const w = mountView()
        expect(w.find('.play-title').attributes('disabled')).toBeDefined()
        expect(w.find('.play-artist').attributes('disabled')).toBeDefined()
    })

    it('the favorite heart flips the current track', async () => {
        const w = mountView()
        await w.find('[aria-label="Add to favorites"]').trigger('click')
        expect(toggleFavorite).toHaveBeenCalledOnce()
        isStarred.value = true
        await w.vm.$nextTick()
        expect(w.find('[aria-label="Remove from favorites"]').exists()).toBe(true)
    })

    it('the header toggle swaps between the player face and the queue', async () => {
        const w = mountView()
        expect(w.find('.play-face').exists()).toBe(true)
        expect(w.find('.stub-queue-body').exists()).toBe(false)
        await w.find('[aria-label="Show queue"]').trigger('click')
        expect(w.find('.stub-queue-body').text()).toBe('sidebar')
        expect(w.find('.play-face').exists()).toBe(false)
        await w.find('[aria-label="Show now playing"]').trigger('click')
        expect(w.find('.play-face').exists()).toBe(true)
    })

    // The queue face keeps the queue management the desktop header has —
    // without it phones would have no way to save or clear the queue.
    it('the queue face carries the edit/save/clear header actions', async () => {
        const w = mountView()
        expect(w.find('.queue-action-save').exists()).toBe(false)
        await w.find('[aria-label="Show queue"]').trigger('click')
        await w.find('.queue-action-save').trigger('click')
        expect(openSaveDialog).toHaveBeenCalledOnce()
        await w.find('.queue-action-clear').trigger('click')
        expect(clearQueue).toHaveBeenCalledOnce()
    })
})
