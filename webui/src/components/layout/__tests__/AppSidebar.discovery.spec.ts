import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { createPinia, setActivePinia } from 'pinia'

const pushSpy = vi.fn()
vi.mock('vue-router', () => ({
    useRoute: () => ({ name: 'home', path: '/', params: {} }),
    useRouter: () => ({ push: pushSpy })
}))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useMusicFolders: () => ({ data: ref([]) })
}))
vi.mock('@/composables/useTheme', () => ({
    useTheme: () => ({
        hiddenUnlocked: ref(false),
        unlockHiddenThemes: vi.fn(),
        cycleHiddenTheme: vi.fn(() => ({ label: 'X' }))
    })
}))
vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

import AppSidebar from '@/components/layout/AppSidebar.vue'

beforeEach(() => {
    setActivePinia(createPinia())
    pushSpy.mockReset()
})

describe('AppSidebar Discover entry', () => {
    it('renders a Discover nav item', () => {
        const w = mount(AppSidebar, { global: { directives: { tooltip: {} } } })
        expect(w.text()).toContain('Discover')
    })

    it('navigates to /discover when clicked', async () => {
        const w = mount(AppSidebar, { global: { directives: { tooltip: {} } } })
        const item = w.findAll('.nav-item').find((n) => n.text().includes('Discover'))
        await item!.trigger('click')
        expect(pushSpy).toHaveBeenCalledWith('/discover')
    })
})
