import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const queue = ref<any[]>([])
const currentIndex = ref(0)
const isPlaying = ref(false)
const playQueueItem = vi.fn()
const removeFromQueue = vi.fn()
const togglePlayPause = vi.fn()
const insertIntoQueue = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue,
        currentIndex,
        isPlaying,
        playQueueItem,
        removeFromQueue,
        togglePlayPause,
        insertIntoQueue
    })
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
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '', getAlbum: vi.fn() }
}))

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

const sortableCreate = vi.fn(() => ({ destroy: vi.fn() }))
vi.mock('sortablejs', () => ({
    default: { create: (...args: unknown[]) => sortableCreate(...(args as [])) }
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

import QueueView from '@/components/layout/QueueView.vue'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import { subsonicClient } from '@/lib/api/subsonic'

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
    insertIntoQueue.mockReset()
    openSaveDialog.mockReset()
    clearQueue.mockReset()
    sortableCreate.mockClear()
    ;(subsonicClient.getAlbum as ReturnType<typeof vi.fn>).mockReset()
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

    it('has no per-row remove control outside edit mode', () => {
        const w = mountView('sidebar')
        expect(w.find('.queue-upcoming .delete-button').exists()).toBe(false)
        expect(w.find('.queue-upcoming input[type="checkbox"]').exists()).toBe(false)
    })

    it('the pencil button toggles edit mode on the rows', async () => {
        const w = mountView('sidebar')
        expect(w.find('.queue-row--editing').exists()).toBe(false)
        await w.find('.queue-action-edit').trigger('click')
        expect(w.find('.queue-row--editing').exists()).toBe(true)
        expect(w.find('.queue-upcoming input[type="checkbox"]').exists()).toBe(true)
    })

    it('deletes a track via the per-row delete button in edit mode', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        await w.find('.queue-upcoming .delete-button').trigger('click')
        expect(removeFromQueue).toHaveBeenCalledWith(2)
    })

    it('shows a history drop zone in edit mode when the first track is playing', async () => {
        currentIndex.value = 0
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        const history = w.find('.queue-history')
        expect(history.exists()).toBe(true)
        expect(history.classes()).toContain('queue-list--drop-empty')
    })

    it('shows an upcoming drop zone in edit mode when the last track is playing', async () => {
        currentIndex.value = 2
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        const upcoming = w.find('.queue-upcoming')
        expect(upcoming.exists()).toBe(true)
        expect(upcoming.classes()).toContain('queue-list--drop-empty')
    })

    it('does not render an empty history list outside edit mode', () => {
        currentIndex.value = 0
        const w = mountView('sidebar')
        expect(w.find('.queue-history').exists()).toBe(false)
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

    it('the header exposes Save as Playlist and Clear Queue as icon buttons', async () => {
        const w = mountView('sidebar')
        const save = w.find('.queue-action-save')
        const clear = w.find('.queue-action-clear')
        expect(save.exists()).toBe(true)
        expect(clear.exists()).toBe(true)
        await save.trigger('click')
        expect(openSaveDialog).toHaveBeenCalled()
        await clear.trigger('click')
        expect(clearQueue).toHaveBeenCalled()
    })

    it('disables the header action buttons when the queue is empty', () => {
        queue.value = []
        const w = mountView('sidebar')
        expect(w.find('.queue-action-save').attributes('disabled')).toBeDefined()
        expect(w.find('.queue-action-clear').attributes('disabled')).toBeDefined()
    })

    it('shows the empty state when the queue is empty', () => {
        queue.value = []
        const w = mountView('full')
        expect(w.find('.queue-empty').exists()).toBe(true)
        expect(w.text()).toContain('Nothing is playing')
    })

    it('creates Sortable instances on the row lists when entering edit mode', async () => {
        const w = mountView('sidebar')
        expect(sortableCreate).not.toHaveBeenCalled()
        await w.find('.queue-action-edit').trigger('click')
        await w.vm.$nextTick()
        // history (1) + upcoming (1) lists both present → two Sortable instances
        expect(sortableCreate).toHaveBeenCalledTimes(2)
        const opts = (sortableCreate.mock.calls[0] as unknown[])[1] as { handle: string; group: string }
        expect(opts.handle).toBe('.drag-handle')
        expect(opts.group).toBe('queue')
    })

    it('destroys Sortable instances when leaving edit mode', async () => {
        const destroy = vi.fn()
        sortableCreate.mockReturnValue({ destroy })
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        await w.vm.$nextTick()
        await w.find('.queue-action-edit').trigger('click') // turn off
        expect(destroy).toHaveBeenCalled()
    })
})

describe('QueueView album drop', () => {
    const getAlbum = subsonicClient.getAlbum as ReturnType<typeof vi.fn>

    const setAlbumPayload = () =>
        useAlbumDragData().setAlbumDrag({ albumId: 'al1', albumName: 'LP', count: 1 })

    const dataTransfer = (types: string[]) => ({ types, dropEffect: '' })

    it('fetches the album and inserts its songs when dropped on the queue body', async () => {
        getAlbum.mockResolvedValue({ id: 'al1', name: 'LP', song: [{ id: 'X', title: 'X' }] })
        setAlbumPayload()
        const w = mountView('sidebar')
        await w
            .find('.queue-body')
            .trigger('drop', { dataTransfer: dataTransfer([ALBUM_DRAG_MIME]) })
        await flushPromises()
        expect(getAlbum).toHaveBeenCalledWith('al1')
        // jsdom rects are 0 → append; queue has 3 items → index 3
        expect(insertIntoQueue).toHaveBeenCalledWith([{ id: 'X', title: 'X' }], 3)
    })

    it('ignores a drop while in edit mode', async () => {
        setAlbumPayload()
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        await w
            .find('.queue-body')
            .trigger('drop', { dataTransfer: dataTransfer([ALBUM_DRAG_MIME]) })
        await flushPromises()
        expect(getAlbum).not.toHaveBeenCalled()
        expect(insertIntoQueue).not.toHaveBeenCalled()
    })

    it('ignores a non-album drop', async () => {
        setAlbumPayload()
        const w = mountView('sidebar')
        await w
            .find('.queue-body')
            .trigger('drop', { dataTransfer: dataTransfer(['text/plain']) })
        await flushPromises()
        expect(insertIntoQueue).not.toHaveBeenCalled()
    })

    it('accepts an album drop into an empty queue', async () => {
        getAlbum.mockResolvedValue({ id: 'al1', name: 'LP', song: [{ id: 'X', title: 'X' }] })
        queue.value = []
        setAlbumPayload()
        const w = mountView('sidebar')
        await w
            .find('.queue-empty')
            .trigger('drop', { dataTransfer: dataTransfer([ALBUM_DRAG_MIME]) })
        await flushPromises()
        expect(insertIntoQueue).toHaveBeenCalledWith([{ id: 'X', title: 'X' }], 0)
    })

    it('highlights the empty queue as a drop zone while an album is dragged over it', async () => {
        queue.value = []
        setAlbumPayload()
        const w = mountView('sidebar')
        const empty = w.find('.queue-empty')
        expect(empty.classes()).not.toContain('queue-empty--drop-active')

        await empty.trigger('dragover', { dataTransfer: dataTransfer([ALBUM_DRAG_MIME]) })
        expect(w.find('.queue-empty').classes()).toContain('queue-empty--drop-active')
        expect(w.text()).toContain('Drop to add album')

        await w.find('.queue-empty').trigger('dragleave')
        expect(w.find('.queue-empty').classes()).not.toContain('queue-empty--drop-active')
    })
})
