import { describe, it, expect, beforeEach, vi } from 'vitest'
import { ref, nextTick, effectScope } from 'vue'
import { useQueueDragExpand } from '@/composables/useQueueDragExpand'
import { useAlbumDragData } from '@/composables/albumDragData'
import { useSongsDragData } from '@/composables/songsDragData'
import type { Song } from '@/types/subsonic'

const sidebarCollapsed = ref(true)
const expandSidebar = vi.fn(() => {
    sidebarCollapsed.value = false
})
const collapseSidebar = vi.fn(() => {
    sidebarCollapsed.value = true
})
vi.mock('@/composables/useQueueSidebar', () => ({
    useQueueSidebar: () => ({ sidebarCollapsed, expandSidebar, collapseSidebar })
}))

const song: Song = { id: 's1', title: 'A', artist: 'B' } as Song

// Run the composable inside an effect scope so its `watch` is disposed between
// tests and doesn't leak reactivity across cases.
const make = () => {
    const scope = effectScope()
    const api = scope.run(() => useQueueDragExpand())!
    return { ...api, stop: () => scope.stop() }
}

describe('useQueueDragExpand', () => {
    beforeEach(() => {
        sidebarCollapsed.value = true
        expandSidebar.mockClear()
        collapseSidebar.mockClear()
        useAlbumDragData().clearAlbumDrag()
        useSongsDragData().clearSongsDrag()
    })

    it('expands when a queue drag enters the collapsed sidebar', () => {
        useAlbumDragData().setAlbumDrag({ albumId: 'a1', albumName: 'Album', count: 3 })
        const { onDragEnter, stop } = make()
        onDragEnter()
        expect(expandSidebar).toHaveBeenCalledOnce()
        stop()
    })

    it('does nothing when the sidebar is already expanded', () => {
        sidebarCollapsed.value = false
        useAlbumDragData().setAlbumDrag({ albumId: 'a1', albumName: 'Album', count: 3 })
        const { onDragEnter, stop } = make()
        onDragEnter()
        expect(expandSidebar).not.toHaveBeenCalled()
        stop()
    })

    it('does nothing when no queue drag is active', () => {
        const { onDragEnter, stop } = make()
        onDragEnter()
        expect(expandSidebar).not.toHaveBeenCalled()
        stop()
    })

    it('collapses again when the drag ends after an auto-expand', async () => {
        useSongsDragData().setSongsDrag({ songs: [song], count: 1 })
        const { onDragEnter, stop } = make()
        onDragEnter()
        expect(expandSidebar).toHaveBeenCalledOnce()

        useSongsDragData().clearSongsDrag()
        await nextTick()
        expect(collapseSidebar).toHaveBeenCalledOnce()
        stop()
    })

    it('leaves a manually-expanded sidebar alone when a drag ends', async () => {
        sidebarCollapsed.value = false
        useSongsDragData().setSongsDrag({ songs: [song], count: 1 })
        const { stop } = make()
        // No onDragEnter() → the expansion was not drag-triggered.
        useSongsDragData().clearSongsDrag()
        await nextTick()
        expect(collapseSidebar).not.toHaveBeenCalled()
        stop()
    })
})
