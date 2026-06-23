import { describe, it, expect, beforeEach } from 'vitest'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import type { Song } from '@/types/subsonic'

const song = (id: string): Song => ({ id, title: `Song ${id}` })

describe('albumDragData', () => {
    beforeEach(() => useAlbumDragData().clearAlbumDrag())

    it('exposes a custom MIME marker', () => {
        expect(ALBUM_DRAG_MIME).toBe('application/x-aether-album')
    })

    it('stores and clears the payload on the shared singleton', () => {
        const a = useAlbumDragData()
        a.setAlbumDrag({ songs: [song('1'), song('2')], albumName: 'LP', count: 2 })
        // a second call returns the same underlying state
        const b = useAlbumDragData()
        expect(b.albumDragPayload.value?.count).toBe(2)
        expect(b.albumDragPayload.value?.albumName).toBe('LP')
        b.clearAlbumDrag()
        expect(a.albumDragPayload.value).toBeNull()
    })
})
