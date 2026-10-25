import { describe, it, expect, vi, afterEach } from 'vitest'
import { computed, ref } from 'vue'

const tier = ref<'phone' | 'tablet' | 'desktop'>('desktop')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ tier: computed(() => tier.value) })
}))

import { LIST_CARD_METRICS, useListCardMetrics } from '@/composables/useListCardMetrics'

afterEach(() => {
    tier.value = 'desktop'
})

describe('useListCardMetrics', () => {
    it('uses the wide card on tablets and desktops and the small one on phones', () => {
        const { metrics } = useListCardMetrics()
        expect(metrics.value).toEqual(LIST_CARD_METRICS.wide)
        tier.value = 'tablet'
        expect(metrics.value).toEqual(LIST_CARD_METRICS.wide)
        tier.value = 'phone'
        expect(metrics.value).toEqual(LIST_CARD_METRICS.phone)
    })

    // ListCard's box is cover + 2 × pad tall with the gap below it, so a slot in
    // the virtual list must be exactly that.
    it('makes a slot the card plus the gap below it', () => {
        const { itemSize } = useListCardMetrics()
        expect(itemSize.value).toBe(96 + 2 * 12 + 10)
        tier.value = 'phone'
        expect(itemSize.value).toBe(80 + 2 * 10 + 8)
    })
})
