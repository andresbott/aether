import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useAlbumDrag } from '@/composables/useAlbumDrag'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import type { AlbumWithSongs } from '@/types/subsonic'

const album: AlbumWithSongs = {
    id: 'al1',
    name: 'LP',
    artist: 'The Artist',
    song: [
        { id: 's1', title: 'One' },
        { id: 's2', title: 'Two' }
    ]
}

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

describe('useAlbumDrag', () => {
    beforeEach(() => {
        useAlbumDragData().clearAlbumDrag()
        document.body.innerHTML = ''
    })

    it('sets the MIME marker, payload and a custom drag image on start', () => {
        const e = fakeDragEvent()
        useAlbumDrag().start(e, album, 'cover-url')

        expect(e.dataTransfer!.effectAllowed).toBe('copy')
        expect(e.dataTransfer!.setData).toHaveBeenCalledWith(ALBUM_DRAG_MIME, 'al1')
        expect(e.dataTransfer!.setDragImage).toHaveBeenCalled()
        expect(document.querySelector('.album-drag-image')).not.toBeNull()

        const payload = useAlbumDragData().albumDragPayload.value
        expect(payload?.count).toBe(2)
        expect(payload?.songs.map((s) => s.id)).toEqual(['s1', 's2'])
    })

    it('cancels the drag and sets no payload when the album has no songs', () => {
        const e = fakeDragEvent()
        useAlbumDrag().start(e, { ...album, song: [] }, null)
        expect(e.preventDefault).toHaveBeenCalled()
        expect(useAlbumDragData().albumDragPayload.value).toBeNull()
    })

    it('removes the drag image and clears the payload on end', () => {
        const drag = useAlbumDrag()
        drag.start(fakeDragEvent(), album, null)
        drag.end()
        expect(document.querySelector('.album-drag-image')).toBeNull()
        expect(useAlbumDragData().albumDragPayload.value).toBeNull()
    })
})
