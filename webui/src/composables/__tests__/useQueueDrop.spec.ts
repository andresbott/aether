import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref } from 'vue'
import { useQueueDrop } from '@/composables/useQueueDrop'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'

const insertIntoQueue = vi.fn()
const queue = ref<Array<{ id: string }>>([])
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({ queue, insertIntoQueue })
}))

const dropEvent = (types: string[]) =>
    ({
        clientY: 0,
        dataTransfer: { types, dropEffect: '' },
        preventDefault: vi.fn(),
        relatedTarget: null
    }) as unknown as DragEvent & { preventDefault: ReturnType<typeof vi.fn> }

describe('useQueueDrop', () => {
    let editing = false
    const body = document.createElement('div')

    const make = () =>
        useQueueDrop({ bodyRef: ref(body), isEditing: () => editing })

    beforeEach(() => {
        editing = false
        queue.value = [{ id: 'A' }, { id: 'B' }, { id: 'C' }]
        insertIntoQueue.mockReset()
        useAlbumDragData().setAlbumDrag({
            songs: [{ id: 'X', title: 'X' }],
            albumName: 'LP',
            count: 1
        })
    })

    it('allows the drop (preventDefault) for an album drag when not editing', () => {
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

    it('ignores dragover while the queue is in edit mode', () => {
        editing = true
        const e = dropEvent([ALBUM_DRAG_MIME])
        make().onDragOver(e)
        expect(e.preventDefault).not.toHaveBeenCalled()
    })

    it('inserts the payload songs at the computed index on drop', () => {
        // jsdom rects are all 0 → pointer not above any midpoint → append at end (3)
        const e = dropEvent([ALBUM_DRAG_MIME])
        make().onDrop(e)
        expect(e.preventDefault).toHaveBeenCalled()
        expect(insertIntoQueue).toHaveBeenCalledWith(
            [{ id: 'X', title: 'X' }],
            3
        )
        expect(useAlbumDragData().albumDragPayload.value).toBeNull()
    })

    it('does not insert on drop while editing', () => {
        editing = true
        make().onDrop(dropEvent([ALBUM_DRAG_MIME]))
        expect(insertIntoQueue).not.toHaveBeenCalled()
    })

    it('does not insert on drop for a non-album drag', () => {
        make().onDrop(dropEvent(['text/plain']))
        expect(insertIntoQueue).not.toHaveBeenCalled()
    })
})
