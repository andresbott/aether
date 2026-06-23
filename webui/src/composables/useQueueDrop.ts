import { ref, computed, type Ref } from 'vue'
import { useToast } from 'primevue/usetoast'
import { usePlayer } from '@/composables/usePlayer'
import { useAlbumDragData, ALBUM_DRAG_MIME } from '@/composables/albumDragData'
import { computeInsertIndex, type QueueRowRect } from '@/utils/queueInsert'
import { subsonicClient } from '@/lib/api/subsonic'

/**
 * Drop-target side of dragging an album into the play queue. Bound to the
 * QueueView body. Native HTML5 DnD; gated on the album MIME marker and disabled
 * while the queue is in edit mode (where SortableJS owns the lists). The drop
 * index is computed from the pointer Y against the rows' on-screen rects; the
 * album's songs are resolved by id on drop.
 */
export function useQueueDrop(options: {
    bodyRef: Ref<HTMLElement | null>
    isEditing: () => boolean
}) {
    const { bodyRef, isEditing } = options
    const player = usePlayer()
    const toast = useToast()
    const { albumDragPayload, clearAlbumDrag } = useAlbumDragData()

    const indicatorTop = ref<number | null>(null)
    const indicatorCount = computed(() => albumDragPayload.value?.count ?? 0)

    const isAlbumDrag = (e: DragEvent): boolean =>
        !!e.dataTransfer && e.dataTransfer.types.includes(ALBUM_DRAG_MIME)

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

    const indicatorYFor = (
        rows: QueueRowRect[],
        target: number,
        body: HTMLElement
    ): number => {
        if (rows.length === 0) return 0
        const bodyTop = body.getBoundingClientRect().top
        const before = rows.find((r) => r.queueIndex === target)
        const edge = before ? before.top : rows[rows.length - 1].bottom
        return edge - bodyTop + body.scrollTop
    }

    const onDragOver = (e: DragEvent): void => {
        const body = bodyRef.value
        if (!body || isEditing() || !isAlbumDrag(e)) return
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
        if (!body || isEditing() || !isAlbumDrag(e)) return
        e.preventDefault()
        indicatorTop.value = null
        const payload = albumDragPayload.value
        clearAlbumDrag()
        if (!payload) return
        const rows = collectRows(body)
        const target = computeInsertIndex(rows, e.clientY, player.queue.value.length)
        try {
            const album = await subsonicClient.getAlbum(payload.albumId)
            if (album?.song?.length) {
                player.insertIntoQueue(album.song, target)
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

    return { indicatorTop, indicatorCount, onDragOver, onDragLeave, onDrop }
}
