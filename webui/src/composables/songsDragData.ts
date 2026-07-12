import { ref, type Ref } from 'vue'
import type { Song } from '@/types/subsonic'

/**
 * Custom MIME type marking a drag that carries a selection of individual songs
 * (e.g. picked in the album view). Only `dataTransfer.types` is readable during
 * `dragover` (not `getData`), so this marker is how the queue dropzone recognizes
 * a songs drag before `drop`. The actual songs travel in the payload below — the
 * source already holds the `Song` objects, so nothing is fetched on drop.
 */
export const SONGS_DRAG_MIME = 'application/x-aether-songs'

export interface SongsDragPayload {
    songs: Song[]
    count: number
}

// Module-scoped singleton: the drag source and the drop target are different
// components. Cleared on both drop and dragend.
const payload: Ref<SongsDragPayload | null> = ref(null)

export function useSongsDragData() {
    const setSongsDrag = (next: SongsDragPayload): void => {
        payload.value = next
    }
    const clearSongsDrag = (): void => {
        payload.value = null
    }
    return { songsDragPayload: payload, setSongsDrag, clearSongsDrag }
}
