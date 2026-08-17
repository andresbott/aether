import { describe, it, expect, vi, beforeEach } from 'vitest'
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

const queueLength = ref(0)
vi.mock('@/composables/usePlayer', () => ({
    usePlayer: () => ({
        queue: computed(() => Array.from({ length: queueLength.value }, (_, i) => ({ id: i })))
    })
}))

const sheetDetent = ref<'collapsed' | 'playing' | 'queue'>('collapsed')
const sheetOpen = computed(() => sheetDetent.value !== 'collapsed')
const sheetState = {
    detent: sheetDetent,
    open: sheetOpen,
    snapTo: (d: 'collapsed' | 'playing' | 'queue') => { sheetDetent.value = d }
}
vi.mock('@/composables/useNowPlayingSheet', () => ({
    useNowPlayingSheet: () => sheetState
}))

vi.mock('@/composables/useMediaSession', () => ({
    useMediaSession: () => {}
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
                MobileShell: {
                    template: `
                        <div data-mobile-shell>
                            <div v-if="queue.length > 0" class="mini-spacer" />
                            <div v-if="queue.length > 0" class="now-playing-sheet" />
                        </div>
                    `,
                    setup() {
                        return { queue: computed(() => Array.from({ length: queueLength.value })) }
                    }
                },
                Toast: true
            }
        }
    })

beforeEach(() => {
    shell.value = 'desktop'
    queueLength.value = 0
    sheetDetent.value = 'collapsed'
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

    it('.body-row is inert on mobile with an open sheet, not inert on desktop or when collapsed', async () => {
        const isInert = (layout: ReturnType<typeof mountLayout>): boolean => {
            const el = layout.find('.body-row').element as HTMLElement & { inert?: boolean }
            return el.inert === true || el.hasAttribute('inert')
        }

        // Test 1: mobile + playing = inert
        shell.value = 'mobile'
        queueLength.value = 1
        sheetDetent.value = 'playing'
        const layout1 = mountLayout()
        await layout1.vm.$nextTick()
        expect(isInert(layout1)).toBe(true)
        layout1.unmount()

        // Test 2: mobile + collapsed = not inert
        shell.value = 'mobile'
        queueLength.value = 1
        sheetDetent.value = 'collapsed'
        const layout2 = mountLayout()
        await layout2.vm.$nextTick()
        expect(isInert(layout2)).toBe(false)
        layout2.unmount()

        // Test 3: desktop = never inert
        shell.value = 'desktop'
        queueLength.value = 1
        sheetDetent.value = 'playing'
        const layout3 = mountLayout()
        await layout3.vm.$nextTick()
        expect(isInert(layout3)).toBe(false)
        layout3.unmount()
    })

    it('MobileShell renders no sheet and no .mini-spacer when the queue is empty', async () => {
        shell.value = 'mobile'
        queueLength.value = 0
        const layout = mountLayout()
        await layout.vm.$nextTick()
        expect(layout.find('.now-playing-sheet').exists()).toBe(false)
        expect(layout.find('.mini-spacer').exists()).toBe(false)
    })

    it('MobileShell renders both sheet and .mini-spacer when the queue is non-empty', async () => {
        shell.value = 'mobile'
        queueLength.value = 1
        const layout = mountLayout()
        await layout.vm.$nextTick()
        expect(layout.find('.now-playing-sheet').exists()).toBe(true)
        expect(layout.find('.mini-spacer').exists()).toBe(true)
    })
})
