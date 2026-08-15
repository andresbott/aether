import { computed, type ComputedRef } from 'vue'
import { usePlayer } from '@/composables/usePlayer'

/**
 * The queue's header summary ("3 tracks • 12 min"), shared by the desktop
 * Now Playing (QueueView) and the phone play view (MobilePlayView). Pre-built
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

    const totalDuration = computed(() => {
        const total = player.queue.value.reduce((sum, s) => sum + (s.duration || 0), 0)
        if (!total) return ''
        const hours = Math.floor(total / 3600)
        const mins = Math.floor((total % 3600) / 60)
        return hours > 0 ? `${hours} hr ${mins} min` : `${mins} min`
    })

    const summary = computed(() => {
        if (trackCount.value === 0) return ''
        const tracks = `${trackCount.value} ${trackCount.value === 1 ? 'track' : 'tracks'}`
        return totalDuration.value ? `${tracks} • ${totalDuration.value}` : tracks
    })

    return { trackCount, summary }
}
