import { ref, type Ref } from 'vue'
import type { Song } from '@/types/subsonic'

/**
 * Custom MIME type marking a drag that carries an album's songs. Only
 * `dataTransfer.types` is readable during `dragover` (not `getData`), so this
 * marker is how the queue dropzone recognizes an album drag before `drop`.
 */
export const ALBUM_DRAG_MIME = 'application/x-aether-album'

export interface AlbumDragPayload {
    songs: Song[]
    albumName: string
    count: number
}

// Module-scoped singleton: the drag source and the drop target are different
// components, and the typed Song[] cannot travel through dataTransfer, so the
// payload lives here. Cleared on both drop and dragend.
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
