// DiscoveryFeed renders CardGrid directly (no VirtualCardGrid in between), so the
// phone cap has to live here too — otherwise Discover shows one monster column at
// 390px. Mirrors VirtualCardGrid.phone.spec.ts.
import { describe, it, expect, vi, beforeAll, afterAll } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick, ref } from 'vue'
import CardGrid from '../CardGrid.vue'

const tier = ref<'phone' | 'tablet' | 'desktop'>('phone')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ tier, isTouch: ref(true), shell: ref('mobile') })
}))

// The grid measures itself through clientWidth, which jsdom always reports as 0.
// Pin the content width a 390px phone leaves for the grid so the column math runs.
const PHONE_CONTENT_WIDTH = 350

beforeAll(() => {
    Object.defineProperty(HTMLElement.prototype, 'clientWidth', {
        configurable: true,
        get: () => PHONE_CONTENT_WIDTH
    })
})

afterAll(() => {
    delete (HTMLElement.prototype as unknown as Record<string, unknown>).clientWidth
})

const mountGrid = () =>
    mount(CardGrid, {
        props: { items: [{ id: 'a' }, { id: 'b' }, { id: 'c' }] },
        slots: { card: '<span class="card" />' }
    })

// The width is measured in onMounted, so the first render still carries the
// unmeasured template — wait one tick for the measured one.
const mountAndMeasure = async (): Promise<string | undefined> => {
    const grid = mountGrid()
    await nextTick()
    return grid.attributes('style')
}

describe('CardGrid phone tuning', () => {
    it('caps the column width and gap on the phone tier', async () => {
        tier.value = 'phone'
        const style = await mountAndMeasure()
        // 350px fits two 150px columns with a 16px gap; the uncapped 200/32 fits one.
        expect(style).toContain('grid-template-columns: repeat(2, minmax(0, 1fr))')
        expect(style).toContain('gap: 16px')
    })

    it('leaves the defaults alone on desktop', async () => {
        tier.value = 'desktop'
        const style = await mountAndMeasure()
        expect(style).toContain('grid-template-columns: repeat(1, minmax(0, 1fr))')
        expect(style).toContain('gap: 32px')
    })
})
