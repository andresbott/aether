import { ref, type Ref } from 'vue'

/**
 * Custom MIME type marking a drag that carries an artist. Only `dataTransfer.types`
 * is readable during `dragover` (not `getData`), so this marker is how the queue
 * dropzone recognizes an artist drag before `drop`. The actual artist id travels in
 * the payload below; the dropzone resolves the albums' songs by id on drop.
 */
export const ARTIST_DRAG_MIME = 'application/x-aether-artist'

export interface ArtistDragPayload {
    artistId: string
    artistName: string
    albumCount: number
}

// Module-scoped singleton: the drag source and the drop target are different
// components. Cleared on both drop and dragend.
const payload: Ref<ArtistDragPayload | null> = ref(null)

export function useArtistDragData() {
    const setArtistDrag = (next: ArtistDragPayload): void => {
        payload.value = next
    }
    const clearArtistDrag = (): void => {
        payload.value = null
    }
    return { artistDragPayload: payload, setArtistDrag, clearArtistDrag }
}
