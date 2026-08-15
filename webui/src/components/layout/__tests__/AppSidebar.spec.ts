import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'

const push = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => ({ name: 'home', path: '/', params: {} }),
    useRouter: () => ({ push })
}))
vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: ref([{ id: 1, name: 'Main' }]) })
}))
const collapsed = ref(false)
vi.mock('@/store/uiStore', () => ({
    useUiStore: () => ({
        get sidebarCollapsed() {
            return collapsed.value
        },
        toggleSidebar: vi.fn()
    })
}))
const toastAdd = vi.fn()
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: toastAdd }) }))

// Only the queue length matters here — the brand picks its destination from it.
const queue = ref<unknown[]>([])
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ queue }) }))

// The easter egg only needs to observe that the composable was driven correctly;
// the theme behaviour itself is covered by useTheme.spec.ts.
const hiddenUnlocked = ref(false)
const unlockHiddenThemes = vi.fn(() => {
    hiddenUnlocked.value = true
})
const cycleHiddenTheme = vi.fn(() => ({ label: 'Winamp', value: 'winamp' }))
vi.mock('@/composables/useTheme', () => ({
    useTheme: () => ({ hiddenUnlocked, unlockHiddenThemes, cycleHiddenTheme })
}))

import AppSidebar from '@/components/layout/AppSidebar.vue'

// The footer's split controls (identity chip + gear) have their own spec —
// UserMenu.spec.ts; here it only matters that they replaced the nav entry.
const mountSidebar = () =>
    mount(AppSidebar, {
        global: { directives: { tooltip: {} }, stubs: { UserMenu: true } }
    })

beforeEach(() => {
    collapsed.value = false
    hiddenUnlocked.value = false
    queue.value = []
    push.mockClear()
    toastAdd.mockClear()
    unlockHiddenThemes.mockClear()
    cycleHiddenTheme.mockClear()
})

describe('AppSidebar footer nav', () => {
    it('renders the split user controls instead of a Settings nav entry', () => {
        const w = mountSidebar()
        expect(w.find('.sidebar-footer-nav user-menu-stub').exists()).toBe(true)
        expect(w.findAll('.sidebar-footer-nav .nav-item')).toHaveLength(0)
        expect(w.text()).not.toContain('Settings')
    })

    it('passes the collapsed state down to the user controls', async () => {
        const w = mountSidebar()
        expect(w.find('user-menu-stub').attributes('collapsed')).toBe('false')
        collapsed.value = true
        await w.vm.$nextTick()
        expect(w.find('user-menu-stub').attributes('collapsed')).toBe('true')
    })
})

describe('AppSidebar brand navigation', () => {
    it('goes to Now Playing when the queue has tracks', async () => {
        queue.value = [{ id: '1' }]
        await mountSidebar().find('.brand').trigger('click')
        expect(push).toHaveBeenCalledWith('/')
    })

    it('goes to the library when the queue is empty', async () => {
        await mountSidebar().find('.brand').trigger('click')
        expect(push).toHaveBeenCalledWith('/library')
    })

    it('exposes the whole brand as one button, not just the wordmark', () => {
        const brand = mountSidebar().find('.brand')
        expect(brand.element.tagName).toBe('BUTTON')
        expect(brand.text()).toBe('Aether')
    })

    // The mark carries no text, so the wordmark alone must name the app: an
    // announced mark would just repeat it.
    it('renders the brand mark as a decorative image beside the wordmark', () => {
        const mark = mountSidebar().find('.brand-mark')
        expect(mark.element.tagName).toBe('IMG')
        expect(mark.attributes('alt')).toBe('')
        expect(mark.attributes('aria-hidden')).toBe('true')
    })

    it('names the destination in the accessible label', () => {
        expect(mountSidebar().find('.brand').attributes('aria-label')).toContain('Library')
        queue.value = [{ id: '1' }]
        expect(mountSidebar().find('.brand').attributes('aria-label')).toContain('Now Playing')
    })

    // The mark is the easter-egg trigger, but it must not swallow the click: a
    // click there navigates like anywhere else in the header.
    it('still navigates when the mark itself is clicked', async () => {
        await mountSidebar().find('.brand-mark').trigger('click')
        expect(push).toHaveBeenCalledWith('/library')
    })
})

describe('AppSidebar hidden-theme easter egg', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    const clickMark = async (times: number) => {
        const w = mountSidebar()
        const e = w.find('.brand-mark')
        for (let i = 0; i < times; i++) await e.trigger('click')
        return w
    }

    it('does nothing before the fifth click', async () => {
        await clickMark(4)
        expect(unlockHiddenThemes).not.toHaveBeenCalled()
        expect(cycleHiddenTheme).not.toHaveBeenCalled()
    })

    it('unlocks and activates a theme on the fifth rapid click', async () => {
        await clickMark(5)
        expect(unlockHiddenThemes).toHaveBeenCalledOnce()
        expect(cycleHiddenTheme).toHaveBeenCalledOnce()
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ summary: 'Hidden themes unlocked' })
        )
    })

    it('resets the streak when the clicks are too slow', async () => {
        const w = mountSidebar()
        const e = w.find('.brand-mark')
        for (let i = 0; i < 4; i++) await e.trigger('click')

        vi.advanceTimersByTime(1600)
        await e.trigger('click')

        expect(unlockHiddenThemes).not.toHaveBeenCalled()
    })

    it('needs a fresh five clicks to cycle again', async () => {
        const w = mountSidebar()
        const e = w.find('.brand-mark')
        for (let i = 0; i < 10; i++) await e.trigger('click')

        expect(cycleHiddenTheme).toHaveBeenCalledTimes(2)
        // Already unlocked the second time, so the toast names the theme instead.
        expect(toastAdd).toHaveBeenLastCalledWith(
            expect.objectContaining({ summary: 'Theme: Winamp' })
        )
    })

    it('leaves the trigger unfocusable so it stays hidden', async () => {
        const e = mountSidebar().find('.brand-mark')
        // An image, like the span before it: never a button, and nothing that
        // would put it in the tab order or announce it as interactive.
        expect(e.element.tagName).toBe('IMG')
        expect(e.attributes('tabindex')).toBeUndefined()
        expect(e.attributes('role')).toBeUndefined()
    })

    it('is not reachable through the wordmark any more', async () => {
        const w = mountSidebar()
        const accent = w.find('.logo-accent')
        for (let i = 0; i < 5; i++) await accent.trigger('click')
        expect(unlockHiddenThemes).not.toHaveBeenCalled()
    })
})
