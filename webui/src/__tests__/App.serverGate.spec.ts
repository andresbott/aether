import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createRouter, createMemoryHistory } from 'vue-router'
import type { MeResponse } from '@/types/users'

// The desktop app's gate end to end: the real useAuth and vue-query decide,
// only the server's answers and the host are stood in.
const { getMe, mintSpaToken } = vi.hoisted(() => ({ getMe: vi.fn(), mintSpaToken: vi.fn() }))
vi.mock('@/lib/api/Users', () => ({ getMe }))
vi.mock('@/lib/api/Tokens', () => ({ mintSpaToken }))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ clearQueue: vi.fn() }) }))
vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({ stop: vi.fn(), start: vi.fn(), restore: vi.fn() })
}))

const host = vi.hoisted(() => ({ desktopApp: true, server: null as string | null }))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl: ref<string | null>(host.server),
        httpFetch: vi.fn(),
        clearServerUrl: vi.fn(),
        setSession: async () => {},
        clearSession: async () => {}
    }
})

const meProxyAnonymous: MeResponse = {
    authMethod: 'proxy-header',
    user: null,
    features: { userManagement: false }
}

async function mountApp() {
    const { default: App } = await import('@/App.vue')
    const router = createRouter({
        history: createMemoryHistory(),
        routes: [{ path: '/', component: { template: '<div>Home</div>' } }]
    })
    const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const wrapper = mount(App, {
        global: {
            plugins: [router, [VueQueryPlugin, { queryClient: qc }]],
            stubs: { LoginView: { name: 'LoginView', template: '<div class="login-view" />' } }
        }
    })
    await router.isReady()
    await flushPromises()
    return wrapper
}

describe('App gate in the desktop app', () => {
    beforeEach(() => {
        // useAuth installs its watchers once per module; every case starts clean.
        vi.resetModules()
        getMe.mockReset()
        mintSpaToken.mockReset()
        host.desktopApp = true
        host.server = null
    })

    // A disabled /me is not loading, so nothing holds the login view back.
    it('shows the login view, and asks nobody, while no server is chosen', async () => {
        const w = await mountApp()

        expect(w.find('.login-view').exists()).toBe(true)
        expect(getMe).not.toHaveBeenCalled()
    })

    it('shows the login view instead of a blank window on a portal-only server', async () => {
        host.server = 'http://aether.example.org'
        getMe.mockResolvedValue(meProxyAnonymous)

        const w = await mountApp()

        expect(w.find('.login-view').exists()).toBe(true)
    })

    // In a browser tab the proxy's portal redirect handles it, as before.
    it('leaves a browser tab on a proxy-header server to its proxy', async () => {
        host.desktopApp = false
        host.server = 'http://localhost:3000'
        getMe.mockResolvedValue(meProxyAnonymous)

        const w = await mountApp()

        expect(w.find('.login-view').exists()).toBe(false)
    })
})
