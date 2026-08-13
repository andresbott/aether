import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, ref } from 'vue'
import PlayerLayout from '../PlayerLayout.vue'

// Both shells are stubbed: this spec owns exactly one fact — PlayerLayout
// mounts one and only one chrome, chosen by useViewport().shell, and swaps
// reactively. Chrome internals belong to the shells' own specs.
const shell = ref<'desktop' | 'mobile'>('desktop')
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({
        shell: computed(() => shell.value),
        tier: computed(() => (shell.value === 'desktop' ? 'desktop' : 'phone')),
        isTouch: ref(false)
    })
}))

vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({ restore: vi.fn().mockResolvedValue(undefined), start: vi.fn(), stop: vi.fn() })
}))

const mountLayout = () =>
    mount(PlayerLayout, {
        global: {
            stubs: {
                DesktopShell: { template: '<div data-desktop-shell />' },
                MobileShell: { template: '<div data-mobile-shell />' },
                Toast: true
            }
        }
    })

describe('PlayerLayout shell switch', () => {
    it('mounts only the desktop chrome when shell is desktop', () => {
        shell.value = 'desktop'
        const layout = mountLayout()
        expect(layout.find('[data-desktop-shell]').exists()).toBe(true)
        expect(layout.find('[data-mobile-shell]').exists()).toBe(false)
    })

    it('mounts only the mobile chrome when shell is mobile', () => {
        shell.value = 'mobile'
        const layout = mountLayout()
        expect(layout.find('[data-mobile-shell]').exists()).toBe(true)
        expect(layout.find('[data-desktop-shell]').exists()).toBe(false)
    })

    it('swaps chrome reactively (tablet rotation)', async () => {
        shell.value = 'desktop'
        const layout = mountLayout()
        shell.value = 'mobile'
        await layout.vm.$nextTick()
        expect(layout.find('[data-mobile-shell]').exists()).toBe(true)
        expect(layout.find('[data-desktop-shell]').exists()).toBe(false)
    })
})
