import { describe, it, expect, vi, beforeEach } from 'vitest'
import { reactive, ref, nextTick } from 'vue'
import { mount } from '@vue/test-utils'

const push = vi.fn()
const route = { path: '/settings/profile' }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push })
}))

const settingsSidebarCollapsed = ref(false)
const toggleSettingsSidebar = vi.fn(() => {
    settingsSidebarCollapsed.value = !settingsSidebarCollapsed.value
})
const checkScreenWidth = vi.fn()
vi.mock('@/store/uiStore', () => ({
    useUiStore: () => reactive({ settingsSidebarCollapsed, toggleSettingsSidebar, checkScreenWidth })
}))

const versionData = ref<Record<string, string> | undefined>(undefined)
vi.mock('@/composables/useVersion', () => ({
    useVersion: () => ({ data: versionData })
}))

import SettingsLayout from '@/layouts/SettingsLayout.vue'

const mountLayout = () =>
    mount(SettingsLayout, {
        global: {
            directives: { tooltip: {} },
            stubs: { RouterView: true }
        }
    })

describe('SettingsLayout', () => {
    beforeEach(() => {
        route.path = '/settings/profile'
        settingsSidebarCollapsed.value = false
        versionData.value = undefined
        push.mockClear()
        toggleSettingsSidebar.mockClear()
        checkScreenWidth.mockClear()
    })

    it('renders both topic groups and every entry', () => {
        const w = mountLayout()
        const text = w.text()
        expect(text).toContain('Account')
        expect(text).toContain('Administration')
        expect(text).toContain('Profile')
        expect(text).toContain('Libraries')
        expect(text).toContain('Tasks')
        expect(text).toContain('Metadata Editor')
        expect(text).toContain('Logout')
    })

    it('highlights the entry matching the current route', () => {
        route.path = '/settings/tasks'
        const w = mountLayout()
        const tasks = w.findAll('.sidebar-nav .nav-item').find((b) => b.text() === 'Tasks')!
        expect(tasks.classes()).toContain('active')
    })

    it('toggles the sidebar via the store and hides labels when collapsed', async () => {
        const w = mountLayout()
        expect(w.find('.settings-sidebar').classes()).not.toContain('collapsed')
        expect(w.find('.nav-section-label').exists()).toBe(true)
        await w.find('.collapse-btn').trigger('click')
        expect(toggleSettingsSidebar).toHaveBeenCalled()
        await nextTick()
        expect(w.find('.settings-sidebar').classes()).toContain('collapsed')
        expect(w.find('.nav-section-label').exists()).toBe(false)
        expect(w.find('.nav-label').exists()).toBe(false)
    })

    it('navigates back to the player from the footer button', async () => {
        const w = mountLayout()
        const back = w.findAll('.sidebar-footer-nav .nav-item').find((b) => b.text().includes('Back'))!
        await back.trigger('click')
        expect(push).toHaveBeenCalledWith('/')
    })

    it('checks the screen width on mount to auto-collapse on narrow screens', () => {
        mountLayout()
        expect(checkScreenWidth).toHaveBeenCalled()
    })

    it('runs the logout placeholder from the Account area without navigating', async () => {
        const info = vi.spyOn(console, 'info').mockImplementation(() => {})
        const w = mountLayout()
        const logout = w.findAll('.sidebar-nav .nav-item').find((b) => b.text() === 'Logout')!
        await logout.trigger('click')
        expect(info).toHaveBeenCalled()
        expect(push).not.toHaveBeenCalled()
        info.mockRestore()
    })
})

describe('SettingsLayout version string', () => {
    beforeEach(() => {
        settingsSidebarCollapsed.value = false
        versionData.value = undefined
    })

    it('renders nothing while the version is unknown', () => {
        expect(mountLayout().find('.sidebar-version').exists()).toBe(false)
    })

    it('shows the version above the footer spacer with build details in the title', () => {
        versionData.value = {
            version: '0.1.1',
            commit: 'abcdef1234567890',
            build_time: '2026-07-25T10:00:00Z'
        }
        const w = mountLayout()
        const el = w.find('.sidebar-version')
        expect(el.text()).toBe('version: v0.1.1')
        expect(el.attributes('title')).toBe('v0.1.1 · abcdef12 · 2026-07-25T10:00:00Z')

        // The version sits outside the footer nav, directly above its top border.
        const children = Array.from(w.find('.settings-sidebar').element.children)
        const versionIdx = children.findIndex((c) => c.classList.contains('sidebar-version'))
        const footerIdx = children.findIndex((c) => c.classList.contains('sidebar-footer-nav'))
        expect(versionIdx).toBeGreaterThanOrEqual(0)
        expect(versionIdx).toBe(footerIdx - 1)
    })

    it('does not double the v prefix for tagged versions', () => {
        versionData.value = { version: 'v2.0.0', commit: 'undefined', build_time: '' }
        const el = mountLayout().find('.sidebar-version')
        expect(el.text()).toBe('version: v2.0.0')
        expect(el.attributes('title')).toBe('v2.0.0')
    })

    it('leaves non-release build names unprefixed', () => {
        versionData.value = { version: 'dev-build', commit: 'undefined', build_time: '' }
        const el = mountLayout().find('.sidebar-version')
        expect(el.text()).toBe('version: dev-build')
        expect(el.attributes('title')).toBe('dev-build')
    })

    it('hides the version when the sidebar is collapsed', () => {
        versionData.value = { version: '0.1.1', commit: 'abc', build_time: '' }
        settingsSidebarCollapsed.value = true
        expect(mountLayout().find('.sidebar-version').exists()).toBe(false)
    })
})
