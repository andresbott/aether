import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref, defineComponent, h } from 'vue'
import PrimeVue from 'primevue/config'

const queue = ref<any[]>([])
const currentIndex = ref(0)
const isPlaying = ref(false)
const playQueueItem = vi.fn()
const removeFromQueue = vi.fn()
const togglePlayPause = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue, currentIndex, isPlaying, playQueueItem, removeFromQueue, togglePlayPause })
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
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

vi.mock('@/components/library/SongDetail.vue', () => ({
    default: {
        name: 'SongDetail',
        props: ['song', 'card'],
        template: '<div class="stub-song-detail">{{ song.title }}</div>'
    }
}))

vi.mock('@/components/layout/SavePlaylistDialog.vue', () => ({
    default: { name: 'SavePlaylistDialog', template: '<div class="stub-save-dialog"></div>' }
}))

vi.mock('primevue/menu', () => ({
    default: defineComponent({
        props: ['model'],
        setup(props, { expose }) {
            expose({ toggle: () => {} })
            return () =>
                h(
                    'div',
                    { class: 'mock-menu' },
                    (props.model || []).map((i: any) =>
                        h('button', { class: 'menu-item', onClick: i.command }, i.label)
                    )
                )
        }
    })
}))

import QueueView from '@/components/layout/QueueView.vue'

const song = (id: string, extra: Record<string, unknown> = {}) => ({
    id,
    title: `Song ${id}`,
    artist: 'Artist',
    album: 'Album',
    duration: 60,
    ...extra
})

const mountView = (variant: 'full' | 'sidebar') =>
    mount(QueueView, {
        props: { variant },
        global: { plugins: [PrimeVue], directives: { tooltip: {} } }
    })

beforeEach(() => {
    queue.value = [song('1'), song('2'), song('3')]
    currentIndex.value = 1
    isPlaying.value = false
    playQueueItem.mockReset()
    removeFromQueue.mockReset()
    togglePlayPause.mockReset()
    openSaveDialog.mockReset()
    clearQueue.mockReset()
})

describe('QueueView', () => {
    it('shows the track count and total duration in the header', () => {
        const w = mountView('sidebar')
        expect(w.text()).toContain('3 tracks')
        expect(w.text()).toContain('3 min')
    })

    it('full variant renders the SongDetail card and no compact strip', () => {
        const w = mountView('full')
        expect(w.find('.stub-song-detail').exists()).toBe(true)
        expect(w.find('.now-playing-strip').exists()).toBe(false)
    })

    it('sidebar variant renders the compact strip and no SongDetail card', () => {
        const w = mountView('sidebar')
        expect(w.find('.now-playing-strip').exists()).toBe(true)
        expect(w.find('.stub-song-detail').exists()).toBe(false)
    })

    it('renders history and upcoming rows numbered by queue position', () => {
        const w = mountView('sidebar')
        const history = w.find('.queue-history')
        const upcoming = w.find('.queue-upcoming')
        expect(history.findAll('.queue-row')).toHaveLength(1)
        expect(upcoming.findAll('.queue-row')).toHaveLength(1)
        expect(history.find('.track-number').text()).toBe('1')
        expect(upcoming.find('.track-number').text()).toBe('3')
    })

    it('a row shows a play icon on hover', async () => {
        const w = mountView('sidebar')
        const row = w.find('.queue-upcoming .queue-row')
        expect(row.find('.track-number').exists()).toBe(true)
        await row.trigger('mouseenter')
        expect(row.find('.play-hover-icon').exists()).toBe(true)
    })

    it('clicking a row plays that queue item', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-upcoming .queue-row').trigger('click')
        expect(playQueueItem).toHaveBeenCalledWith(2)
    })

    it('the per-row remove button removes that item from the queue', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-upcoming .remove-button').trigger('click')
        expect(removeFromQueue).toHaveBeenCalledWith(2)
    })

    it('the strip play/pause toggle sits in the index column and toggles playback', async () => {
        const w = mountView('sidebar')
        const toggle = w.find('.now-playing-strip .strip-index')
        expect(toggle.exists()).toBe(true)
        expect(w.find('.strip-toggle-icon').classes()).toContain('pi-play')
        await toggle.trigger('click')
        expect(togglePlayPause).toHaveBeenCalled()
    })

    it('the strip toggle shows a pause icon while playing', () => {
        isPlaying.value = true
        const w = mountView('sidebar')
        expect(w.find('.strip-toggle-icon').classes()).toContain('pi-pause')
    })

    it('the options menu offers Clear Queue and Save as Playlist', async () => {
        const w = mountView('sidebar')
        const items = w.findAll('.menu-item')
        expect(items.map((i) => i.text())).toEqual(['Clear Queue', 'Save as Playlist'])
        await items[0].trigger('click')
        expect(clearQueue).toHaveBeenCalled()
        await items[1].trigger('click')
        expect(openSaveDialog).toHaveBeenCalled()
    })

    it('shows the empty state when the queue is empty', () => {
        queue.value = []
        const w = mountView('full')
        expect(w.find('.queue-empty').exists()).toBe(true)
        expect(w.text()).toContain('Nothing is playing')
    })
})
