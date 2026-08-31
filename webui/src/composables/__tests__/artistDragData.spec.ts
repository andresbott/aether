import { describe, it, expect, beforeEach } from 'vitest'
import { useArtistDragData, ARTIST_DRAG_MIME } from '@/composables/artistDragData'

describe('artistDragData', () => {
    beforeEach(() => useArtistDragData().clearArtistDrag())

    it('exposes a custom MIME marker', () => {
        expect(ARTIST_DRAG_MIME).toBe('application/x-aether-artist')
    })

    it('stores and clears the id payload on the shared singleton', () => {
        const a = useArtistDragData()
        a.setArtistDrag({ artistId: 'ar1', artistName: 'The Artist', albumCount: 5 })
        const b = useArtistDragData()
        expect(b.artistDragPayload.value?.artistId).toBe('ar1')
        expect(b.artistDragPayload.value?.albumCount).toBe(5)
        b.clearArtistDrag()
        expect(a.artistDragPayload.value).toBeNull()
    })
})
