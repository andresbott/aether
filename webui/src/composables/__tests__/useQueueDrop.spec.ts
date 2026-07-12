import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { useQueueDrop } from '@/composables/useQueueDrop'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import { useSongsDragData, SONGS_DRAG_MIME } from '@/composables/songsDragData'
import { subsonicClient } from '@/lib/api/subsonic'
import type { Song } from '@/types/subsonic'

const insertIntoQueue = vi.fn()
const queue = ref<Array<{ id: string }>>([])
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue, insertIntoQueue })
}))

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getAlbum: vi.fn() }
}))

const getAlbum = subsonicClient.getAlbum as ReturnType<typeof vi.fn>

const dropEvent = (types: string[]) =>
    ({
        clientY: 0,
        dataTransfer: { types, dropEffect: '' },
        preventDefault: vi.fn(),
        relatedTarget: null
    }) as unknown as DragEvent & { preventDefault: ReturnType<typeof vi.fn> }

describe('useQueueDrop', () => {
    const body = document.createElement('div')
    const onInsert = vi.fn()

    const make = () => useQueueDrop({ bodyRef: ref(body), onInsert })

    beforeEach(() => {
        queue.value = [{ id: 'A' }, { id: 'B' }, { id: 'C' }]
        insertIntoQueue.mockReset()
        getAlbum.mockReset()
        onInsert.mockReset()
        useAlbumDragData().clearAlbumDrag()
        useSongsDragData().clearSongsDrag()
        useAlbumDragData().setAlbumDrag({ albumId: 'al1', albumName: 'LP', count: 2 })
    })

    it('reports an active album drag whenever a payload is present', () => {
        const d = make()
        expect(d.dragActive.value).toBe(true) // beforeEach set a payload
        useAlbumDragData().clearAlbumDrag()
        expect(d.dragActive.value).toBe(false)
    })

    it('allows the drop (preventDefault) for an album drag', () => {
        const e = dropEvent([ALBUM_DRAG_MIME])
        make().onDragOver(e)
        expect(e.preventDefault).toHaveBeenCalled()
        expect(e.dataTransfer!.dropEffect).toBe('copy')
    })

    it('ignores dragover for a non-album drag', () => {
        const e = dropEvent(['text/plain'])
        make().onDragOver(e)
        expect(e.preventDefault).not.toHaveBeenCalled()
    })

    it('fetches the album by id and inserts its songs at the computed index on drop', async () => {
        getAlbum.mockResolvedValue({
            id: 'al1',
            name: 'LP',
            song: [
                { id: 'X', title: 'X' },
                { id: 'Y', title: 'Y' }
            ]
        })
        const e = dropEvent([ALBUM_DRAG_MIME])
        await make().onDrop(e)
        expect(e.preventDefault).toHaveBeenCalled()
        expect(getAlbum).toHaveBeenCalledWith('al1')
        // jsdom rects are 0 → pointer above no midpoint → append; queue has 3 → index 3
        expect(insertIntoQueue).toHaveBeenCalledWith(
            [
                { id: 'X', title: 'X' },
                { id: 'Y', title: 'Y' }
            ],
            3
        )
        expect(useAlbumDragData().albumDragPayload.value).toBeNull()
    })

    it('calls onInsert after inserting a dropped album', async () => {
        getAlbum.mockResolvedValue({ id: 'al1', name: 'LP', song: [{ id: 'X', title: 'X' }] })
        await make().onDrop(dropEvent([ALBUM_DRAG_MIME]))
        expect(insertIntoQueue).toHaveBeenCalled()
        expect(onInsert).toHaveBeenCalledTimes(1)
    })

    it('does not insert when the fetched album has no songs', async () => {
        getAlbum.mockResolvedValue({ id: 'al1', name: 'LP', song: [] })
        await make().onDrop(dropEvent([ALBUM_DRAG_MIME]))
        expect(insertIntoQueue).not.toHaveBeenCalled()
        expect(onInsert).not.toHaveBeenCalled()
    })

    it('does not fetch or insert on drop for a non-album drag', async () => {
        await make().onDrop(dropEvent(['text/plain']))
        expect(getAlbum).not.toHaveBeenCalled()
        expect(insertIntoQueue).not.toHaveBeenCalled()
        expect(onInsert).not.toHaveBeenCalled()
    })

    describe('songs drag', () => {
        const dragged: Song[] = [{ id: 'X' } as Song, { id: 'Y' } as Song]

        beforeEach(() => {
            useAlbumDragData().clearAlbumDrag()
            useSongsDragData().setSongsDrag({ songs: dragged, count: dragged.length })
        })

        it('reports an active drag and the song count', () => {
            const d = make()
            expect(d.dragActive.value).toBe(true)
            expect(d.indicatorCount.value).toBe(2)
        })

        it('allows the drop (preventDefault) for a songs drag', () => {
            const e = dropEvent([SONGS_DRAG_MIME])
            make().onDragOver(e)
            expect(e.preventDefault).toHaveBeenCalled()
            expect(e.dataTransfer!.dropEffect).toBe('copy')
        })

        it('inserts the carried songs at the computed index without fetching', async () => {
            const e = dropEvent([SONGS_DRAG_MIME])
            await make().onDrop(e)
            expect(e.preventDefault).toHaveBeenCalled()
            expect(getAlbum).not.toHaveBeenCalled()
            // jsdom rects are 0 → append; queue has 3 → index 3.
            expect(insertIntoQueue).toHaveBeenCalledWith(dragged, 3)
            expect(useSongsDragData().songsDragPayload.value).toBeNull()
        })

        it('calls onInsert after inserting the carried songs', async () => {
            await make().onDrop(dropEvent([SONGS_DRAG_MIME]))
            expect(insertIntoQueue).toHaveBeenCalledWith(dragged, 3)
            expect(onInsert).toHaveBeenCalledTimes(1)
        })

        it('does not insert or call onInsert when the songs payload is empty', async () => {
            useSongsDragData().clearSongsDrag()
            useSongsDragData().setSongsDrag({ songs: [], count: 0 })
            await make().onDrop(dropEvent([SONGS_DRAG_MIME]))
            expect(insertIntoQueue).not.toHaveBeenCalled()
            expect(onInsert).not.toHaveBeenCalled()
        })
    })
})
