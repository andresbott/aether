import { describe, it, expect, beforeEach, vi } from 'vitest'
import { useArtistDrag } from '@/composables/useArtistDrag'
import { useArtistDragData, ARTIST_DRAG_MIME } from '@/composables/artistDragData'
import type { Artist } from '@/types/subsonic'

const artist: Artist = {
    id: 'ar1',
    name: 'The Artist',
    albumCount: 5
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

describe('useArtistDrag', () => {
    beforeEach(() => {
        useArtistDragData().clearArtistDrag()
        document.body.innerHTML = ''
    })

    it('sets the MIME marker, id payload and a custom drag image on start', () => {
        const e = fakeDragEvent()
        useArtistDrag().start(e, artist, 'cover-url')

        expect(e.dataTransfer!.effectAllowed).toBe('copy')
        expect(e.dataTransfer!.setData).toHaveBeenCalledWith(ARTIST_DRAG_MIME, 'ar1')
        expect(e.dataTransfer!.setDragImage).toHaveBeenCalled()
        expect(document.querySelector('.artist-drag-image')).not.toBeNull()

        const payload = useArtistDragData().artistDragPayload.value
        expect(payload?.artistId).toBe('ar1')
        expect(payload?.albumCount).toBe(5)
    })

    it('cancels the drag and sets no payload when the artist has no id', () => {
        const e = fakeDragEvent()
        useArtistDrag().start(e, { ...artist, id: '' }, null)
        expect(e.preventDefault).toHaveBeenCalled()
        expect(useArtistDragData().artistDragPayload.value).toBeNull()
    })

    it('removes the drag image and clears the payload on end', () => {
        const drag = useArtistDrag()
        drag.start(fakeDragEvent(), artist, null)
        drag.end()
        expect(document.querySelector('.artist-drag-image')).toBeNull()
        expect(useArtistDragData().artistDragPayload.value).toBeNull()
    })
})
