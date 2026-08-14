import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { computed, reactive, ref } from 'vue'
import PlayerLayout from '../PlayerLayout.vue'

// Both shells are stubbed: this spec owns two facts — PlayerLayout mounts one
// and only one chrome, chosen by useViewport().shell and swapped reactively,
// and the route outlet lives OUTSIDE the swap so a rotation never unmounts
// the active view (that teardown would bypass onBeforeRouteLeave /
// beforeunload guards and discard staged edits). Chrome internals belong to
// the shells' own specs.
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

vi.mock('@/composables/useScrollbarWidth', () => ({
    useScrollbarWidth: () => ref(0)
}))

const route = reactive({ name: 'library', meta: { flush: true } })
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push: vi.fn() }),
    RouterView: { template: '<div class="router-outlet" />' },
    RouterLink: { template: '<a><slot /></a>' }
}))

const mountLayout = () =>
    mount(PlayerLayout, {
        global: {
            stubs: {
                AppSidebar: true,
                QueueSidebar: true,
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

    it('keeps the route outlet mounted across a shell swap', async () => {
        shell.value = 'desktop'
        const layout = mountLayout()
        const outletBefore = layout.find('.router-outlet').element
        shell.value = 'mobile'
        await layout.vm.$nextTick()
        // Same DOM node, not a remounted equivalent: the swap replaces chrome
        // only, so view state (edits, scroll, loaded pages) survives rotation.
        expect(layout.find('.router-outlet').element).toBe(outletBefore)
    })

    it('renders the desktop sidebars only in the desktop shell', async () => {
        shell.value = 'desktop'
        const layout = mountLayout()
        expect(layout.findComponent({ name: 'AppSidebar' }).exists()).toBe(true)
        shell.value = 'mobile'
        await layout.vm.$nextTick()
        expect(layout.findComponent({ name: 'AppSidebar' }).exists()).toBe(false)
    })
})
