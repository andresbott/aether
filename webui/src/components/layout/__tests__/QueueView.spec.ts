import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'

const queue = ref<any[]>([])
const currentIndex = ref(0)
const isPlaying = ref(false)
const playQueueItem = vi.fn()
const removeFromQueue = vi.fn()
const removeManyFromQueue = vi.fn()
const togglePlayPause = vi.fn()
const insertIntoQueue = vi.fn()

vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue,
        currentIndex,
        isPlaying,
        playQueueItem,
        removeFromQueue,
        removeManyFromQueue,
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

// Queue rows carry a favorite toggle, whose real mutation needs a
// VueQueryPlugin-provided client this spec has no use for.
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: vi.fn() })
}))

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
    removeManyFromQueue.mockReset()
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

    // The rows must be handed the queue's own song objects, not `{ ...song }`
    // copies: the favorite toggle writes `starred` straight onto them, because the
    // queue is plain reactive state with no query to refetch.
    it('starring a queue row writes through to the queue song', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-upcoming .row-star').trigger('click')
        expect(typeof queue.value[2].starred).toBe('string')
        expect(queue.value[0].starred).toBeUndefined()
    })

    it('a starred queue row renders a filled heart', () => {
        queue.value = [song('1'), song('2'), song('3', { starred: '2026-02-01T00:00:00Z' })]
        const w = mountView('sidebar')
        const heart = w.find('.queue-upcoming .row-star')
        expect(heart.classes()).toContain('is-starred')
        expect(heart.find('i').classes()).toContain('pi-heart-fill')
    })

    // The now-playing track is a track like any other and needs the same heart;
    // the compact strip is a hand-rolled row, so it does not get one from QueueRow.
    // It sits inside `.strip-info`, on its own line below the title/artist/album
    // stack rather than in a star column — the strip is a stacked block, not a row.
    it('the sidebar now-playing strip carries a favorite toggle under its text', async () => {
        const w = mountView('sidebar')
        expect(w.find('.strip-info .strip-star .row-star').exists()).toBe(true)
        await w.find('.now-playing-strip .row-star').trigger('click')
        expect(typeof queue.value[1].starred).toBe('string')
    })

    it('the strip heart reflects the current track state', () => {
        queue.value = [song('1'), song('2', { starred: '2026-02-01T00:00:00Z' }), song('3')]
        const w = mountView('sidebar') // currentIndex = 1
        expect(w.find('.strip-star .row-star i').classes()).toContain('pi-heart-fill')
    })

    // The full variant's SongDetail card carries its own, so a second one here
    // would be a duplicate affordance for the same track.
    it('the full variant has no strip star (SongDetail owns it there)', () => {
        const w = mountView('full')
        expect(w.find('.now-playing-strip').exists()).toBe(false)
        expect(w.find('.strip-star').exists()).toBe(false)
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
        expect(w.find('.queue-edit-list .delete-button').exists()).toBe(true)
    })

    it('edit mode renders one flat list containing every track', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        expect(w.findAll('.queue-edit-list .queue-row')).toHaveLength(3)
        // The history/upcoming split and the now-playing strip give way to the
        // single reorderable list.
        expect(w.find('.queue-history').exists()).toBe(false)
        expect(w.find('.queue-upcoming').exists()).toBe(false)
        expect(w.find('.now-playing-strip').exists()).toBe(false)
    })

    it('deletes a track via the per-row delete button in edit mode', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        await w.find('[data-queue-index="2"] .delete-button').trigger('click')
        expect(removeFromQueue).toHaveBeenCalledWith(2)
    })

    it('renders the now-playing track as a selectable checkbox row in edit mode', async () => {
        const w = mountView('sidebar') // currentIndex = 1
        await w.find('.queue-action-edit').trigger('click')
        const currentRow = w.find('[data-queue-index="1"]')
        expect(currentRow.find('input[type="checkbox"]').exists()).toBe(true)
        expect(currentRow.find('.current-play-toggle').exists()).toBe(false)
        expect(currentRow.classes()).toContain('queue-row--current')
    })

    it('the current track can be deleted like any other row in edit mode', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        await w.find('[data-queue-index="1"] .delete-button').trigger('click')
        expect(removeFromQueue).toHaveBeenCalledWith(1)
    })

    // currentIndex is 1, so rows 0 and 2 are the selectable ones.
    const selectRowsZeroAndTwo = async (w: ReturnType<typeof mountView>) => {
        await w.find('.queue-action-edit').trigger('click')
        await w.find('[data-queue-index="0"]').trigger('click')
        await w.find('[data-queue-index="2"]').trigger('click', { ctrlKey: true })
    }

    it('deletes every selected track when clicking delete on a selected row', async () => {
        const w = mountView('sidebar')
        await selectRowsZeroAndTwo(w)
        await w.find('[data-queue-index="2"] .delete-button').trigger('click')
        expect(removeManyFromQueue).toHaveBeenCalledWith([0, 2])
        expect(removeFromQueue).not.toHaveBeenCalled()
    })

    it('deletes only the clicked row when it is not part of the selection', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        await w.find('[data-queue-index="0"]').trigger('click') // select row 0 only
        await w.find('[data-queue-index="2"] .delete-button').trigger('click')
        expect(removeFromQueue).toHaveBeenCalledWith(2)
        expect(removeManyFromQueue).not.toHaveBeenCalled()
    })

    it('plain checkbox clicks build a multi-selection that Delete removes', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        // No modifier keys — each checkbox-cell click adds to the selection.
        await w.find('[data-queue-index="0"] .row-index--checkbox').trigger('click')
        await w.find('[data-queue-index="2"] .row-index--checkbox').trigger('click')
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(removeManyFromQueue).toHaveBeenCalledWith([0, 2])
    })

    it('the now-playing row can be selected via its checkbox and bulk-deleted', async () => {
        const w = mountView('sidebar') // currentIndex = 1 is now playing
        await w.find('.queue-action-edit').trigger('click')
        await w.find('[data-queue-index="1"] .row-index--checkbox').trigger('click')
        await w.find('[data-queue-index="2"] .row-index--checkbox').trigger('click')
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(removeManyFromQueue).toHaveBeenCalledWith([1, 2])
    })

    it('the Delete key removes every selected track', async () => {
        const w = mountView('sidebar')
        await selectRowsZeroAndTwo(w)
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(removeManyFromQueue).toHaveBeenCalledWith([0, 2])
    })

    it('the Backspace key removes every selected track', async () => {
        const w = mountView('sidebar')
        await selectRowsZeroAndTwo(w)
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Backspace' })
        expect(removeManyFromQueue).toHaveBeenCalledWith([0, 2])
    })

    it('a Delete keypress with nothing selected does nothing', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        await w.find('.queue-edit-list').trigger('keydown', { key: 'Delete' })
        expect(removeFromQueue).not.toHaveBeenCalled()
        expect(removeManyFromQueue).not.toHaveBeenCalled()
    })

    it('exposes the edit list as a focusable listbox', async () => {
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        const list = w.find('.queue-edit-list')
        expect(list.attributes('role')).toBe('listbox')
        expect(list.attributes('tabindex')).toBe('0')
    })

    it('full variant renders the artist as a column; the sidebar keeps it stacked', async () => {
        const full = mountView('full')
        expect(full.find('.queue-upcoming .row-info').classes()).toContain('row-info--columns')
        await full.find('.queue-action-edit').trigger('click')
        expect(full.find('.queue-edit-list .row-info').classes()).toContain('row-info--columns')

        const sidebar = mountView('sidebar')
        expect(sidebar.find('.queue-upcoming .row-info').classes()).not.toContain(
            'row-info--columns'
        )
        await sidebar.find('.queue-action-edit').trigger('click')
        expect(sidebar.find('.queue-edit-list .row-info').classes()).not.toContain(
            'row-info--columns'
        )
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
        // One flat edit list → a single Sortable instance.
        expect(sortableCreate).toHaveBeenCalledTimes(1)
        const opts = (sortableCreate.mock.calls[0] as unknown[])[1] as {
            handle: string
            group: string
        }
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

    it('inserts a dropped album and clears the selection while in edit mode', async () => {
        getAlbum.mockResolvedValue({ id: 'al1', name: 'LP', song: [{ id: 'X', title: 'X' }] })
        setAlbumPayload()
        const w = mountView('sidebar')
        await w.find('.queue-action-edit').trigger('click')
        // Select a row so we can prove the drop clears the selection.
        await w.find('[data-queue-index="0"]').trigger('click')
        expect(w.find('[data-queue-index="0"]').classes()).toContain('selected')
        await w
            .find('.queue-body')
            .trigger('drop', { dataTransfer: dataTransfer([ALBUM_DRAG_MIME]) })
        await flushPromises()
        await w.vm.$nextTick()
        expect(getAlbum).toHaveBeenCalledWith('al1')
        // jsdom rects are 0 → append; queue has 3 items → index 3
        expect(insertIntoQueue).toHaveBeenCalledWith([{ id: 'X', title: 'X' }], 3)
        // The drop cleared the prior selection.
        expect(w.find('[data-queue-index="0"]').classes()).not.toContain('selected')
    })

    it('ignores a non-album drop', async () => {
        setAlbumPayload()
        const w = mountView('sidebar')
        await w.find('.queue-body').trigger('drop', { dataTransfer: dataTransfer(['text/plain']) })
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

    it('highlights the empty queue as a drop zone as soon as an album drag starts', async () => {
        queue.value = []
        useAlbumDragData().clearAlbumDrag()
        const w = mountView('sidebar')
        expect(w.find('.queue-empty').classes()).not.toContain('queue-empty--drop-active')

        // A drag starting anywhere sets the shared payload → the empty queue
        // advertises itself immediately, without the cursor being over it.
        setAlbumPayload()
        await w.vm.$nextTick()
        expect(w.find('.queue-empty').classes()).toContain('queue-empty--drop-active')
        expect(w.text()).toContain('Drop to add album')

        useAlbumDragData().clearAlbumDrag()
        await w.vm.$nextTick()
        expect(w.find('.queue-empty').classes()).not.toContain('queue-empty--drop-active')
    })
})
