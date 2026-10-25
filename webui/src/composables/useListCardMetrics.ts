import { computed, type ComputedRef } from 'vue'
import { useViewport } from '@/composables/useViewport'

/** One list-layout card (`ListCard`), in px: its cover, the padding around it, the gap below it. */
export interface ListCardMetrics {
    cover: number
    pad: number
    gap: number
}

// The cover is also the height the card's text has: a title, a subtitle and two
// lines of facts fit beside it at either size (ListCard's type scale).
export const LIST_CARD_METRICS: Record<'wide' | 'phone', ListCardMetrics> = {
    wide: { cover: 96, pad: 12, gap: 10 },
    phone: { cover: 80, pad: 10, gap: 8 }
}

/**
 * The size of a list card for the current viewport. `ListCard` binds `metrics` as
 * CSS variables and the virtual lists hand `itemSize` (the card plus the gap below
 * it) to `VirtualScroller`, so the height a card takes in the flow and the slot the
 * scroller reserves for it come from one place. A drift between the two would
 * misplace every card below the first and send the alphabet rail to the wrong
 * letter.
 */
export function useListCardMetrics(): {
    metrics: ComputedRef<ListCardMetrics>
    itemSize: ComputedRef<number>
} {
    const { tier } = useViewport()
    const metrics = computed(() =>
        tier.value === 'phone' ? LIST_CARD_METRICS.phone : LIST_CARD_METRICS.wide
    )
    const itemSize = computed(() => metrics.value.cover + 2 * metrics.value.pad + metrics.value.gap)
    return { metrics, itemSize }
}
