import { useArtistDragData, ARTIST_DRAG_MIME } from '@/composables/artistDragData'
import { buildArtistDragImage } from '@/utils/artistDragImage'
import type { Artist } from '@/types/subsonic'

/**
 * Drag-source side of dragging an artist into the play queue. Bound to each artist
 * card. Carries only the artist id + display metadata; the queue dropzone resolves
 * the albums' songs by id on drop. `coverSrc` is supplied by the caller (which
 * already builds cover URLs), keeping this composable free of the subsonic client.
 */
export function useArtistDrag() {
    let dragImageEl: HTMLElement | null = null
    const { setArtistDrag, clearArtistDrag } = useArtistDragData()

    const start = (event: DragEvent, artist: Artist, coverSrc: string | null): void => {
        if (!artist.id || !event.dataTransfer) {
            event.preventDefault()
            return
        }

        const albumCount = artist.albumCount ?? 0
        event.dataTransfer.effectAllowed = 'copy'
        event.dataTransfer.setData(ARTIST_DRAG_MIME, artist.id)
        setArtistDrag({ artistId: artist.id, artistName: artist.name, albumCount })

        const img = buildArtistDragImage({
            coverSrc,
            name: artist.name,
            albumCount
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
        clearArtistDrag()
    }

    return { start, end }
}
