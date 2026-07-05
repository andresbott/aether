import { describe, it, expect, beforeEach } from 'vitest'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'

describe('albumDragData', () => {
    beforeEach(() => useAlbumDragData().clearAlbumDrag())

    it('exposes a custom MIME marker', () => {
        expect(ALBUM_DRAG_MIME).toBe('application/x-aether-album')
    })

    it('stores and clears the id payload on the shared singleton', () => {
        const a = useAlbumDragData()
        a.setAlbumDrag({ albumId: 'al1', albumName: 'LP', count: 2 })
        const b = useAlbumDragData()
        expect(b.albumDragPayload.value?.albumId).toBe('al1')
        expect(b.albumDragPayload.value?.count).toBe(2)
        b.clearAlbumDrag()
        expect(a.albumDragPayload.value).toBeNull()
    })
})
