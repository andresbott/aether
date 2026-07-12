import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useSongsDrag } from '@/composables/useSongsDrag'
import { useSongsDragData, SONGS_DRAG_MIME } from '@/composables/songsDragData'
import type { Song } from '@/types/subsonic'

const songs: Song[] = [{ id: 's1', title: 'One' } as Song, { id: 's2', title: 'Two' } as Song]

const fakeDragEvent = () => {
    const store: Record<string, string> = {}
    const dataTransfer = {
        effectAllowed: '',
        setData: vi.fn((type: string, val: string) => {
            store[type] = val
        }),
        setDragImage: vi.fn(),
        get types() {
            return Object.keys(store)
        }
    }
    return { dataTransfer, preventDefault: vi.fn() } as unknown as DragEvent & {
        preventDefault: ReturnType<typeof vi.fn>
    }
}

const rowEl = () => document.createElement('div')

describe('useSongsDrag', () => {
    beforeEach(() => {
        useSongsDragData().clearSongsDrag()
        document.body.innerHTML = ''
    })

    it('sets the MIME marker, songs payload and a stacked drag image for a multi-song drag', () => {
        const e = fakeDragEvent()
        useSongsDrag().start(e, songs, rowEl())

        expect(e.dataTransfer!.effectAllowed).toBe('copy')
        expect(e.dataTransfer!.setData).toHaveBeenCalledWith(SONGS_DRAG_MIME, '2')
        expect(e.dataTransfer!.setDragImage).toHaveBeenCalled()
        expect(document.querySelector('.queue-drag-image')).not.toBeNull()

        const payload = useSongsDragData().songsDragPayload.value
        expect(payload?.count).toBe(2)
        expect(payload?.songs).toEqual(songs)
    })

    it('uses the browser default image (no custom image) for a single-song drag', () => {
        const e = fakeDragEvent()
        useSongsDrag().start(e, [songs[0]], rowEl())

        expect(e.dataTransfer!.setData).toHaveBeenCalledWith(SONGS_DRAG_MIME, '1')
        expect(e.dataTransfer!.setDragImage).not.toHaveBeenCalled()
        expect(document.querySelector('.queue-drag-image')).toBeNull()
        expect(useSongsDragData().songsDragPayload.value?.count).toBe(1)
    })

    it('cancels the drag and sets no payload for an empty selection', () => {
        const e = fakeDragEvent()
        useSongsDrag().start(e, [], rowEl())
        expect(e.preventDefault).toHaveBeenCalled()
        expect(useSongsDragData().songsDragPayload.value).toBeNull()
    })

    it('removes the drag image and clears the payload on end', () => {
        const drag = useSongsDrag()
        drag.start(fakeDragEvent(), songs, rowEl())
        drag.end()
        expect(document.querySelector('.queue-drag-image')).toBeNull()
        expect(useSongsDragData().songsDragPayload.value).toBeNull()
    })
})
