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

const mountSidebar = () =>
    mount(AppSidebar, { global: { directives: { tooltip: {} } } })

beforeEach(() => {
    collapsed.value = false
    hiddenUnlocked.value = false
    toastAdd.mockClear()
    unlockHiddenThemes.mockClear()
    cycleHiddenTheme.mockClear()
})

describe('AppSidebar footer nav', () => {
    it('shows a single Settings entry and drops the Admin/User split', () => {
        const labels = mountSidebar()
            .findAll('.sidebar-footer-nav .nav-item')
            .map((b) => b.text())
        expect(labels).toEqual(['Settings'])
        expect(labels).not.toContain('Admin Settings')
        expect(labels).not.toContain('User Settings')
        expect(labels).not.toContain('Logout')
    })

    it('navigates to /settings when Settings is clicked', async () => {
        const w = mountSidebar()
        const btn = w
            .findAll('.sidebar-footer-nav .nav-item')
            .find((b) => b.text() === 'Settings')!
        await btn.trigger('click')
        expect(push).toHaveBeenCalledWith('/settings')
    })
})

describe('AppSidebar hidden-theme easter egg', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    const clickAccent = async (times: number) => {
        const w = mountSidebar()
        const e = w.find('.logo-accent')
        for (let i = 0; i < times; i++) await e.trigger('click')
        return w
    }

    it('does nothing before the fifth click', async () => {
        await clickAccent(4)
        expect(unlockHiddenThemes).not.toHaveBeenCalled()
        expect(cycleHiddenTheme).not.toHaveBeenCalled()
    })

    it('unlocks and activates a theme on the fifth rapid click', async () => {
        await clickAccent(5)
        expect(unlockHiddenThemes).toHaveBeenCalledOnce()
        expect(cycleHiddenTheme).toHaveBeenCalledOnce()
        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({ summary: 'Hidden themes unlocked' })
        )
    })

    it('resets the streak when the clicks are too slow', async () => {
        const w = mountSidebar()
        const e = w.find('.logo-accent')
        for (let i = 0; i < 4; i++) await e.trigger('click')

        vi.advanceTimersByTime(1600)
        await e.trigger('click')

        expect(unlockHiddenThemes).not.toHaveBeenCalled()
    })

    it('needs a fresh five clicks to cycle again', async () => {
        const w = mountSidebar()
        const e = w.find('.logo-accent')
        for (let i = 0; i < 10; i++) await e.trigger('click')

        expect(cycleHiddenTheme).toHaveBeenCalledTimes(2)
        // Already unlocked the second time, so the toast names the theme instead.
        expect(toastAdd).toHaveBeenLastCalledWith(
            expect.objectContaining({ summary: 'Theme: Winamp' })
        )
    })

    it('leaves the trigger unfocusable so it stays hidden', async () => {
        const e = mountSidebar().find('.logo-accent')
        expect(e.element.tagName).toBe('SPAN')
        expect(e.attributes('tabindex')).toBeUndefined()
        expect(e.attributes('role')).toBeUndefined()
    })
})
