import { useSongsDragData, SONGS_DRAG_MIME } from '@/composables/songsDragData'
import { buildMultiDragImage } from '@/utils/queueDragImage'
import type { Song } from '@/types/subsonic'

/**
 * Drag-source side of dragging a selection of songs into the play queue. Bound to
 * each selectable album track row. Carries the songs directly (the album view
 * already holds them); the queue dropzone inserts them on drop. A stacked drag
 * image is shown only for multi-song drags — a single row uses the browser's
 * default image of the grabbed element, matching the queue reorder behaviour.
 */
export function useSongsDrag() {
    let dragImageEl: HTMLElement | null = null
    const { setSongsDrag, clearSongsDrag } = useSongsDragData()

    const start = (event: DragEvent, songs: Song[], dragEl: HTMLElement): void => {
        if (songs.length === 0 || !event.dataTransfer) {
            event.preventDefault()
            return
        }

        const count = songs.length
        event.dataTransfer.effectAllowed = 'copy'
        event.dataTransfer.setData(SONGS_DRAG_MIME, String(count))
        setSongsDrag({ songs, count })

        if (count > 1) {
            const img = buildMultiDragImage(dragEl, count)
            document.body.appendChild(img)
            dragImageEl = img
            event.dataTransfer.setDragImage(img, 24, 24)
        }
    }

    const end = (): void => {
        if (dragImageEl) {
            dragImageEl.remove()
            dragImageEl = null
        }
        clearSongsDrag()
    }

    return { start, end }
}
