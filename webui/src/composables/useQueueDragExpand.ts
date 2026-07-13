import { computed, watch } from 'vue'
import { useAlbumDragData } from '@/composables/albumDragData'
import { useSongsDragData } from '@/composables/songsDragData'
import { useQueueSidebar } from '@/composables/useQueueSidebar'

/**
 * Bridges "a queue drag is in progress" to the queue sidebar's collapse state.
 * When collapsed, the sidebar renders only a cover strip with no dropzone (the
 * dropzone lives in QueueView, rendered only when expanded), so dragging content
 * onto it does nothing. This auto-expands the sidebar the moment a queue drag
 * enters the collapsed strip, then re-collapses once the drag ends — whether it
 * dropped or was cancelled (the album/songs payload is cleared in both cases).
 * A sidebar the user expanded manually is never touched: only a drag-triggered
 * expansion is reverted.
 */
export function useQueueDragExpand() {
    const { albumDragPayload } = useAlbumDragData()
    const { songsDragPayload } = useSongsDragData()
    const { sidebarCollapsed, expandSidebar, collapseSidebar } = useQueueSidebar()

    let autoExpanded = false

    const queueDragActive = computed(
        () => albumDragPayload.value !== null || songsDragPayload.value !== null
    )

    const onDragEnter = (): void => {
        if (queueDragActive.value && sidebarCollapsed.value) {
            autoExpanded = true
            expandSidebar()
        }
    }

    watch(queueDragActive, (active) => {
        if (!active && autoExpanded) {
            autoExpanded = false
            collapseSidebar()
        }
    })

    return { onDragEnter }
}
