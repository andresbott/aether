import { computed, type ComputedRef } from 'vue'
import { usePlayer } from '@/composables/usePlayer'
import { formatTotalDuration } from '@/utils/formatDuration'

/**
 * The queue's header summary ("3 tracks • 12 min"), shared by the desktop
 * Now Playing (QueueView) and the phone play view (QueuePanel). Pre-built
 * as one string so headers have no stray whitespace between the count and the
 * unit word (keeps `.text()` assertions reliable in tests). Empty at zero so
 * ContentScaffold omits the summary element entirely.
 */
export function useQueueSummary(): {
    trackCount: ComputedRef<number>
    summary: ComputedRef<string>
} {
    const player = usePlayer()

    const trackCount = computed(() => player.queue.value.length)

    const totalDuration = computed(() =>
        formatTotalDuration(player.queue.value.reduce((sum, s) => sum + (s.duration || 0), 0))
    )

    const summary = computed(() => {
        if (trackCount.value === 0) return ''
        const tracks = `${trackCount.value} ${trackCount.value === 1 ? 'track' : 'tracks'}`
        return totalDuration.value ? `${tracks} • ${totalDuration.value}` : tracks
    })

    return { trackCount, summary }
}
