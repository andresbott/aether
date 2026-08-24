import { describe, it, expect, vi, beforeEach } from 'vitest'
import { reactive, ref, nextTick } from 'vue'
import { mount } from '@vue/test-utils'

const push = vi.fn()
const route = { path: '/settings/libraries' }
vi.mock('vue-router', () => ({
    useRoute: () => route,
    useRouter: () => ({ push })
}))

const settingsSidebarCollapsed = ref(false)
const toggleSettingsSidebar = vi.fn(() => {
    settingsSidebarCollapsed.value = !settingsSidebarCollapsed.value
})
vi.mock('@/store/uiStore', () => ({
    useUiStore: () =>
        reactive({
            settingsSidebarCollapsed,
            toggleSettingsSidebar
        })
}))

const versionData = ref<Record<string, string> | undefined>(undefined)
vi.mock('@/composables/useVersion', () => ({
    useVersion: () => ({ data: versionData })
}))

const userManagement = ref(false)
vi.mock('@/composables/useUsers', () => ({
    useUserManagement: () => userManagement
}))

import SettingsLayout from '@/layouts/SettingsLayout.vue'

const mountLayout = () =>
    mount(SettingsLayout, {
        global: {
            directives: { tooltip: {} },
            stubs: { RouterView: true, Toast: { template: '<div class="toast-outlet" />' } }
        }
    })

describe('SettingsLayout', () => {
    beforeEach(() => {
        route.path = '/settings/libraries'
        settingsSidebarCollapsed.value = false
        versionData.value = undefined
        userManagement.value = false
        push.mockClear()
        toggleSettingsSidebar.mockClear()
    })

    // Settings is administration only: the account concerns (profile, logout)
    // moved to the sidebar's UserMenu popup and the /profile main view.
    it('renders the Administration group and no Account group', () => {
        const w = mountLayout()
        const text = w.text()
        expect(text).toContain('Administration')
        expect(text).toContain('Libraries')
        expect(text).toContain('Tasks')
        expect(text).toContain('Metadata Editor')
        expect(text).not.toContain('Account')
        expect(text).not.toContain('Profile')
        expect(text).not.toContain('Logout')
    })

    // The metadata editor sits under its own Tools header, in addition to being
    // reachable from the sidebar UserMenu. Its side-nav entry points at the
    // top-level /metadata-editor route.
    it('renders a Tools group with the Metadata Editor entry', () => {
        const text = mountLayout().text()
        expect(text).toContain('Tools')
        expect(text).toContain('Metadata Editor')
    })

    it('navigates to /metadata-editor from the Metadata Editor entry', async () => {
        const w = mountLayout()
        const item = w
            .findAll('.sidebar-nav .nav-item')
            .find((b) => b.text() === 'Metadata Editor')!
        await item.trigger('click')
        expect(push).toHaveBeenCalledWith('/metadata-editor')
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

    // The Users section only exists when the server reports the
    // user-management feature: without it the users API is not even mounted,
    // so the entry would lead to a dead view.
    it('hides the Users entry without the user-management feature and shows it with it', async () => {
        const w = mountLayout()
        expect(w.text()).not.toContain('Users')
        userManagement.value = true
        await nextTick()
        expect(w.text()).toContain('Users')
    })

    // Settings views report failures through useToast (library CRUD, task
    // actions). PrimeVue drops any toast raised with no Toast component mounted,
    // and the only other outlet is in PlayerLayout, which never renders under
    // /settings — so without this outlet those errors are silently swallowed.
    it('mounts a Toast outlet so settings views can surface errors', () => {
        expect(mountLayout().find('.toast-outlet').exists()).toBe(true)
    })

    // On phone, .nav-label is hidden, leaving icon-only buttons that must carry
    // an accessible name (WCAG 4.1.2).
    it('provides accessible names on all nav buttons', () => {
        const w = mountLayout()
        const navItems = w.findAll('.sidebar-nav .nav-item')
        expect(navItems.length).toBeGreaterThan(0)
        navItems.forEach((item) => {
            expect(item.attributes('aria-label')).toBeTruthy()
        })
        const backBtn = w.find('.sidebar-footer-nav .nav-item')
        expect(backBtn.attributes('aria-label')).toBe('Back to player')
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
