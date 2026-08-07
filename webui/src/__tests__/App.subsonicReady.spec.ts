import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createRouter, createMemoryHistory } from 'vue-router'
import { nextTick } from 'vue'
import App from '@/App.vue'
import LoginView from '@/views/LoginView.vue'
import PlayerLayout from '@/layouts/PlayerLayout.vue'

const { getMe, mintSpaToken } = vi.hoisted(() => ({ getMe: vi.fn(), mintSpaToken: vi.fn() }))
vi.mock('@/lib/api/Users', () => ({ getMe }))
vi.mock('@/lib/api/Tokens', () => ({ mintSpaToken }))
// Stub player/sync so the purge doesn't drag audio elements into jsdom
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ clearQueue: vi.fn() }) }))
vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({ stop: vi.fn(), start: vi.fn(), restore: vi.fn() })
}))

const meNone = {
    authMethod: 'none' as const,
    user: null,
    features: { userManagement: false }
}

describe('App subsonicReady gate', () => {
    let subsonicReady: (typeof import('@/lib/subsonicSession'))['subsonicReady']

    beforeEach(async () => {
        vi.resetModules()
        ;({ subsonicReady } = await import('@/lib/subsonicSession'))
        subsonicReady.value = false
        getMe.mockReset()
        mintSpaToken.mockReset()
        mintSpaToken.mockResolvedValue({
            token: 'aether_test_key',
            tokenId: 'tid_test',
            expiresAt: '2999-01-01T00:00:00Z'
        })
    })

    it('renders layout only when subsonicReady is true', async () => {
        getMe.mockResolvedValue(meNone)
        subsonicReady.value = true

        const router = createRouter({
            history: createMemoryHistory(),
            routes: [{ path: '/', component: { template: '<div>Home</div>' } }]
        })
        const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })

        const wrapper = mount(App, {
            global: {
                plugins: [router, [VueQueryPlugin, { queryClient: qc }]],
                stubs: { PlayerLayout: true }
            }
        })
        await router.isReady()
        await flushPromises()

        expect(wrapper.findComponent(LoginView).exists()).toBe(false)
        expect(wrapper.findComponent(PlayerLayout).exists()).toBe(true)
    })

    it('renders nothing when subsonicReady is false (not LoginView)', async () => {
        getMe.mockResolvedValue(meNone)
        // Prevent the watcher from running by keeping /me loading
        getMe.mockReturnValue(new Promise(() => {}))

        const router = createRouter({
            history: createMemoryHistory(),
            routes: [{ path: '/', component: { template: '<div>Home</div>' } }]
        })
        const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
        const wrapper = mount(App, {
            global: {
                plugins: [router, [VueQueryPlugin, { queryClient: qc }]],
                stubs: { PlayerLayout: true }
            }
        })
        await router.isReady()
        await wrapper.vm.$nextTick()

        expect(subsonicReady.value).toBe(false)
        expect(wrapper.findComponent(LoginView).exists()).toBe(false)
        expect(wrapper.findComponent(PlayerLayout).exists()).toBe(false)
    })

    it('renders LoginView when needsLogin regardless of subsonicReady', async () => {
        getMe.mockResolvedValue({ authMethod: 'native', user: null, features: { userManagement: true } })

        const router = createRouter({
            history: createMemoryHistory(),
            routes: [{ path: '/', component: { template: '<div>Home</div>' } }]
        })
        const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
        const wrapper = mount(App, {
            global: {
                plugins: [router, [VueQueryPlugin, { queryClient: qc }]],
                stubs: { LoginView: true }
            }
        })
        await router.isReady()
        await flushPromises()

        expect(wrapper.findComponent(LoginView).exists()).toBe(true)
        expect(wrapper.findComponent(PlayerLayout).exists()).toBe(false)
    })
})
