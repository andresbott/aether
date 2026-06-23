import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import { buildAlbumDragImage } from '@/utils/albumDragImage'
import type { AlbumWithSongs } from '@/types/subsonic'

/**
 * Drag-source side of dragging an album into the play queue. Bound to the album
 * header's drag handle in AlbumView. `coverSrc` is supplied by the caller (which
 * already builds cover URLs), keeping this composable free of the subsonic client.
 */
export function useAlbumDrag() {
    let dragImageEl: HTMLElement | null = null
    const { setAlbumDrag, clearAlbumDrag } = useAlbumDragData()

    const start = (
        event: DragEvent,
        album: AlbumWithSongs,
        coverSrc: string | null
    ): void => {
        const songs = album.song ?? []
        if (songs.length === 0 || !event.dataTransfer) {
            event.preventDefault()
            return
        }

        event.dataTransfer.effectAllowed = 'copy'
        event.dataTransfer.setData(ALBUM_DRAG_MIME, album.id)
        setAlbumDrag({ songs, albumName: album.name, count: songs.length })

        const img = buildAlbumDragImage({
            coverSrc,
            name: album.name,
            artist: album.artist ?? '',
            count: songs.length
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
