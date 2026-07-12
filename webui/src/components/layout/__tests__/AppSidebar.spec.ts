import { describe, it, expect, vi } from 'vitest'
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
vi.mock('@/store/uiStore', () => ({
    useUiStore: () => ({ sidebarCollapsed: false, toggleSidebar: vi.fn() })
}))

import AppSidebar from '@/components/layout/AppSidebar.vue'

const mountSidebar = () =>
    mount(AppSidebar, { global: { directives: { tooltip: {} } } })

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
