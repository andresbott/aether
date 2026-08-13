import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, computed } from 'vue'
import PlayerSheet from '../PlayerSheet.vue'
import { resetPlayerSheetForTests, usePlayerSheet } from '@/composables/usePlayerSheet'

const push = vi.fn()
const routeRef = { fullPath: '/', name: 'home' }
vi.mock('vue-router', () => ({
    useRouter: () => ({ push }),
    useRoute: () => routeRef
}))

const togglePlayPause = vi.fn()
const playNext = vi.fn()
const playPrevious = vi.fn()
const seek = vi.fn()
const toggleShuffle = vi.fn()
const toggleRepeat = vi.fn()
const isPlaying = ref(true)
const currentTrack = ref<Record<string, unknown> | null>({
    title: 'Karma Police',
    artist: 'Radiohead',
    coverArt: 'cov-1',
    albumId: 'al-9',
    artistId: 'ar-4'
})

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        currentTrack,
        isPlaying,
        currentTime: ref(30),
        duration: ref(120),
        shuffle: ref(false),
        repeat: ref('none'),
        hasNext: computed(() => true),
        hasPrevious: computed(() => false),
        togglePlayPause,
        playNext,
        playPrevious,
        seek,
        toggleShuffle,
        toggleRepeat,
        queue: ref([])
    })
}))

const toggleFavorite = vi.fn()
vi.mock('@/composables/useCurrentTrackFavorite', () => ({
    useCurrentTrackFavorite: () => ({ isStarred: computed(() => false), toggleFavorite })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        isConfigured: () => true,
        getCoverArtUrl: (id: string, size: number) => `/art/${id}?size=${size}`
    }
}))

// QueueBody pulls the whole queue stack; the sheet only needs to place it.
const QueueBodyStub = { name: 'QueueBody', props: ['variant', 'editMode'], template: '<div data-queue-body />' }

const mountSheet = () =>
    mount(PlayerSheet, {
        global: { stubs: { QueueBody: QueueBodyStub, Teleport: true } }
    })

beforeEach(() => {
    vi.clearAllMocks()
    resetPlayerSheetForTests()
    vi.spyOn(window.history, 'pushState').mockImplementation(() => {})
    vi.spyOn(window.history, 'back').mockImplementation(() => {})
})

describe('PlayerSheet', () => {
    it('renders nothing while closed', () => {
        expect(mountSheet().find('.player-sheet').exists()).toBe(false)
    })

    it('opens with artwork, meta and transport', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        expect(sheet.find('.player-sheet').exists()).toBe(true)
        expect(sheet.find('img.sheet-cover').attributes('src')).toBe('/art/cov-1?size=512')
        expect(sheet.text()).toContain('Karma Police')
        expect(sheet.text()).toContain('Radiohead')
        expect(sheet.find('[aria-label="Pause"]').exists()).toBe(true)
        expect(sheet.find('[aria-label="Previous track"]').attributes('disabled')).toBeDefined()
    })

    it('chevron closes the sheet', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        await sheet.find('[aria-label="Close player"]').trigger('click')
        expect(usePlayerSheet().isOpen.value).toBe(false)
    })

    it('seek input drives player.seek with absolute seconds', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        const range = sheet.find('input[type="range"]')
        ;(range.element as HTMLInputElement).value = '90'
        await range.trigger('input')
        expect(seek).toHaveBeenCalledWith(90)
    })

    it('title navigates to the album and closes', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        await sheet.find('.sheet-title').trigger('click')
        expect(push).toHaveBeenCalledWith({ name: 'album', params: { id: 'al-9' } })
        expect(usePlayerSheet().isOpen.value).toBe(false)
    })

    it('queue button swaps to the queue face', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        await sheet.find('[aria-label="Show queue"]').trigger('click')
        expect(sheet.find('[data-queue-body]').exists()).toBe(true)
        expect(sheet.find('.sheet-cover').exists()).toBe(false)
    })

    it('Escape closes the sheet', async () => {
        const sheet = mountSheet()
        usePlayerSheet().open()
        await sheet.vm.$nextTick()
        window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }))
        await sheet.vm.$nextTick()
        expect(usePlayerSheet().isOpen.value).toBe(false)
    })
})
