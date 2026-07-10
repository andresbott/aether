import { describe, it, expect, vi } from 'vitest'
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
vi.mock('@/store/uiStore', () => ({
    useUiStore: () => reactive({ settingsSidebarCollapsed, toggleSettingsSidebar })
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
    it('renders both topic groups and every entry', () => {
        const w = mountLayout()
        const text = w.text()
        expect(text).toContain('Account')
        expect(text).toContain('Administration')
        expect(text).toContain('Profile')
        expect(text).toContain('Libraries')
        expect(text).toContain('Tasks')
        expect(text).toContain('Metadata Editor')
    })

    it('highlights the entry matching the current route', () => {
        route.path = '/settings/tasks'
        const w = mountLayout()
        const tasks = w.findAll('.sidebar-nav .nav-item').find((b) => b.text() === 'Tasks')!
        expect(tasks.classes()).toContain('active')
        route.path = '/settings/profile'
    })

    it('toggles the sidebar via the store and hides labels when collapsed', async () => {
        settingsSidebarCollapsed.value = false
        const w = mountLayout()
        await w.find('.collapse-btn').trigger('click')
        expect(toggleSettingsSidebar).toHaveBeenCalled()
        await nextTick()
        expect(w.find('.settings-sidebar').classes()).toContain('collapsed')
        expect(w.find('.nav-section-label').exists()).toBe(false)
        expect(w.find('.nav-label').exists()).toBe(false)
        settingsSidebarCollapsed.value = false
    })

    it('navigates back to the player from the footer button', async () => {
        const w = mountLayout()
        const back = w.findAll('.sidebar-footer-nav .nav-item').find((b) => b.text().includes('Back'))!
        await back.trigger('click')
        expect(push).toHaveBeenCalledWith('/')
    })
})
