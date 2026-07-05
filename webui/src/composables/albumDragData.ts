import { ref, type Ref } from 'vue'

/**
 * Custom MIME type marking a drag that carries an album. Only `dataTransfer.types`
 * is readable during `dragover` (not `getData`), so this marker is how the queue
 * dropzone recognizes an album drag before `drop`. The actual album id travels in
 * the payload below; the dropzone resolves the songs by id on drop.
 */
export const ALBUM_DRAG_MIME = 'application/x-aether-album'

export interface AlbumDragPayload {
    albumId: string
    albumName: string
    count: number
}

// Module-scoped singleton: the drag source and the drop target are different
// components. Cleared on both drop and dragend.
const payload: Ref<AlbumDragPayload | null> = ref(null)

export function useAlbumDragData() {
    const setAlbumDrag = (next: AlbumDragPayload): void => {
        payload.value = next
    }
    const clearAlbumDrag = (): void => {
        payload.value = null
    }
    return { albumDragPayload: payload, setAlbumDrag, clearAlbumDrag }
}
