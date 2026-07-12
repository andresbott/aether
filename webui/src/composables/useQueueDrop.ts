import { ref, computed, type Ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import { usePlayer } from '@/composables/usePlayer'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import { useSongsDragData, SONGS_DRAG_MIME } from '@/composables/songsDragData'
import { computeInsertIndex, type QueueRowRect } from '@/utils/queueInsert'
import { subsonicClient } from '@/lib/api/subsonic'

/**
 * Drop-target side of dragging content into the play queue. Bound to the
 * QueueView body in both view and edit mode. Native HTML5 DnD, gated only on the
 * album or songs MIME markers: an internal SortableJS reorder drag carries no such
 * marker, so it is ignored here and left to SortableJS. The drop index is computed
 * from the pointer Y against the rows' on-screen rects (edit rows carry the same
 * `data-queue-index`). An album drag resolves its songs by id on drop; a songs drag
 * carries them in its payload. `onInsert` fires only when tracks are actually added
 * (the caller uses it to clear the edit-mode row selection, whose indices the insert
 * would otherwise invalidate).
 */
export function useQueueDrop(options: { bodyRef: Ref<HTMLElement | null>; onInsert?: () => void }) {
    const { bodyRef, onInsert } = options
    const player = usePlayer()
    const toast = useToast()
    const { albumDragPayload, clearAlbumDrag } = useAlbumDragData()
    const { songsDragPayload, clearSongsDrag } = useSongsDragData()

    const indicatorTop = ref<number | null>(null)
    const indicatorCount = computed(
        () => songsDragPayload.value?.count ?? albumDragPayload.value?.count ?? 0
    )
    // True for the whole duration of a queue drag (payload set at dragstart,
    // cleared at dragend/drop) — independent of whether the cursor is over the
    // queue, so a drop target can advertise itself the moment a drag begins.
    const dragActive = computed(
        () => albumDragPayload.value !== null || songsDragPayload.value !== null
    )

    const isQueueDrag = (e: DragEvent): boolean =>
        !!e.dataTransfer &&
        (e.dataTransfer.types.includes(ALBUM_DRAG_MIME) ||
            e.dataTransfer.types.includes(SONGS_DRAG_MIME))

    const collectRows = (body: HTMLElement): QueueRowRect[] =>
        Array.from(body.querySelectorAll<HTMLElement>('[data-queue-index]'))
            .map((el) => {
                const r = el.getBoundingClientRect()
                return {
                    queueIndex: Number(el.dataset.queueIndex),
                    top: r.top,
                    bottom: r.bottom
                }
            })
            .filter((row) => !Number.isNaN(row.queueIndex))
            .sort((a, b) => a.queueIndex - b.queueIndex)

    const indicatorYFor = (rows: QueueRowRect[], target: number, body: HTMLElement): number => {
        if (rows.length === 0) return 0
        const bodyTop = body.getBoundingClientRect().top
        const before = rows.find((r) => r.queueIndex === target)
        const edge = before ? before.top : rows[rows.length - 1].bottom
        return edge - bodyTop + body.scrollTop
    }

    const onDragOver = (e: DragEvent): void => {
        const body = bodyRef.value
        if (!body || !isQueueDrag(e)) return
        e.preventDefault()
        if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy'
        const rows = collectRows(body)
        const target = computeInsertIndex(rows, e.clientY, player.queue.value.length)
        indicatorTop.value = indicatorYFor(rows, target, body)
    }

    const onDragLeave = (e: DragEvent): void => {
        const body = bodyRef.value
        if (body && e.relatedTarget instanceof Node && body.contains(e.relatedTarget)) return
        indicatorTop.value = null
    }

    const onDrop = async (e: DragEvent): Promise<void> => {
        const body = bodyRef.value
        if (!body || !isQueueDrag(e)) return
        e.preventDefault()
        indicatorTop.value = null
        const songs = songsDragPayload.value
        const album = albumDragPayload.value
        clearSongsDrag()
        clearAlbumDrag()
        const rows = collectRows(body)
        const target = computeInsertIndex(rows, e.clientY, player.queue.value.length)

        // A songs drag carries its tracks directly — insert them without a fetch.
        if (songs) {
            if (songs.songs.length) {
                player.insertIntoQueue(songs.songs, target)
                onInsert?.()
            }
            return
        }

        if (!album) return
        try {
            const fetched = await subsonicClient.getAlbum(album.albumId)
            if (fetched?.song?.length) {
                player.insertIntoQueue(fetched.song, target)
                onInsert?.()
            } else {
                toast.add({
                    severity: 'warn',
                    summary: 'Nothing to add',
                    detail: 'This album has no tracks.',
                    life: 3000
                })
            }
        } catch (err) {
            toast.add({
                severity: 'error',
                summary: 'Could not load album',
                detail: (err as Error).message,
                life: 5000
            })
        }
    }

    return { indicatorTop, indicatorCount, dragActive, onDragOver, onDragLeave, onDrop }
}
