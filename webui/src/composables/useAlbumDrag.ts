import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import { buildAlbumDragImage } from '@/utils/albumDragImage'
import type { Album } from '@/types/subsonic'

/**
 * Drag-source side of dragging an album into the play queue. Bound to the album
 * detail-page handle and to each album card. Carries only the album id + display
 * metadata; the queue dropzone resolves the songs by id on drop. `coverSrc` is
 * supplied by the caller (which already builds cover URLs), keeping this
 * composable free of the subsonic client.
 */
export function useAlbumDrag() {
    let dragImageEl: HTMLElement | null = null
    const { setAlbumDrag, clearAlbumDrag } = useAlbumDragData()

    const start = (event: DragEvent, album: Album, coverSrc: string | null): void => {
        if (!album.id || !event.dataTransfer) {
            event.preventDefault()
            return
        }

        const count = album.songCount ?? 0
        event.dataTransfer.effectAllowed = 'copy'
        event.dataTransfer.setData(ALBUM_DRAG_MIME, album.id)
        setAlbumDrag({ albumId: album.id, albumName: album.name, count })

        const img = buildAlbumDragImage({
            coverSrc,
            name: album.name,
            artist: album.artist ?? '',
            count
        })
        document.body.appendChild(img)
        dragImageEl = img
        event.dataTransfer.setDragImage(img, 24, 24)
    }

    const end = (): void => {
        if (dragImageEl) {
            dragImageEl.remove()
            dragImageEl = null
        }
        clearAlbumDrag()
    }

    return { start, end }
}
