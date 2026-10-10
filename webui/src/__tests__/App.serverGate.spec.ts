import { describe, it, expect, beforeEach, vi } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { createRouter, createMemoryHistory } from 'vue-router'
import PrimeVue from 'primevue/config'
import type { MeResponse } from '@/types/users'

// The desktop app's gate end to end: the real useAuth, login view and vue-query
// decide; only the server's answers, the probe and the host are stood in.
const { getMe, mintSpaToken, probeServer } = vi.hoisted(() => ({
    getMe: vi.fn(),
    mintSpaToken: vi.fn(),
    probeServer: vi.fn()
}))
vi.mock('@/lib/api/Users', () => ({ getMe }))
vi.mock('@/lib/api/Tokens', () => ({ mintSpaToken }))
vi.mock('@/lib/api/Server', async (importOriginal) => ({
    ...(await importOriginal<typeof import('@/lib/api/Server')>()),
    probeServer
}))
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ clearQueue: vi.fn() }) }))
vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({ stop: vi.fn(), start: vi.fn(), restore: vi.fn() })
}))

// A mocked module outlives vi.resetModules, so its serverUrl ref is one for the
// whole file: cases set it through the import, not through the factory.
const host = vi.hoisted(() => ({ desktopApp: true }))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    const serverUrl = ref<string | null>(null)
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl,
        // As the IPC wrapper does: the saved address becomes the server.
        setServerUrl: async (url: string) => {
            serverUrl.value = url
            return url
        },
        httpFetch: vi.fn(),
        clearServerUrl: vi.fn(),
        setSession: async () => {},
        clearSession: async () => {}
    }
})

const SERVER = 'http://aether.local:8075'

const meNone: MeResponse = {
    authMethod: 'none',
    user: null,
    features: { userManagement: false }
}
const meProxyAnonymous: MeResponse = {
    authMethod: 'proxy-header',
    user: null,
    features: { userManagement: false }
}

async function setServer(url: string | null) {
    const { serverUrl } = await import('@/lib/host')
    serverUrl.value = url
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
            plugins: [router, PrimeVue, [VueQueryPlugin, { queryClient: qc }]],
            stubs: { PlayerLayout: { template: '<div class="player-layout" />' } }
        }
    })
    await router.isReady()
    await flushPromises()
    return wrapper
}

describe('App gate in the desktop app', () => {
    beforeEach(async () => {
        // useAuth installs its watchers once per module; every case starts clean.
        vi.resetModules()
        getMe.mockReset()
        mintSpaToken.mockReset()
        probeServer.mockReset()
        host.desktopApp = true
        await setServer(null)
    })

    // A disabled /me is not loading, so nothing holds the login view back.
    it('shows the server step, and asks nobody, while no server is chosen', async () => {
        const w = await mountApp()

        expect(w.find('#login-server').exists()).toBe(true)
        expect(getMe).not.toHaveBeenCalled()
    })

    it('shows the login view instead of a blank window on a portal-only server', async () => {
        await setServer('http://aether.example.org')
        getMe.mockResolvedValue(meProxyAnonymous)

        const w = await mountApp()

        expect(w.find('.login-notice').text()).toContain('web portal')
        expect(w.find('#login-server').exists()).toBe(false)
    })

    // In a browser tab the proxy's portal redirect handles it, as before.
    it('leaves a browser tab on a proxy-header server to its proxy', async () => {
        host.desktopApp = false
        await setServer('http://localhost:3000')
        getMe.mockResolvedValue(meProxyAnonymous)

        const w = await mountApp()

        expect(w.find('.login-view').exists()).toBe(false)
    })

    // The saved server is down at launch: the server step comes back on its
    // address instead of a blank window, and Connect on the same address asks
    // /me again — which the server, back up, now answers.
    it('recovers from a saved server that did not answer at launch', async () => {
        await setServer(SERVER)
        // A /me asked again unprompted would never settle, taking the step away.
        getMe
            .mockRejectedValueOnce(new Error('Network Error'))
            .mockReturnValue(new Promise(() => {}))
        probeServer.mockResolvedValue({ kind: 'aether', url: SERVER, me: meNone })

        const w = await mountApp()

        // The login view mounting on the failure does not ask again by itself.
        expect(getMe).toHaveBeenCalledTimes(1)
        expect((w.find('#login-server').element as HTMLInputElement).value).toBe(SERVER)
        expect(w.find('.login-error').text()).toBe(`Could not reach ${SERVER}.`)
        expect(w.find('.player-layout').exists()).toBe(false)

        // Back up: Connect on the same address asks /me again.
        getMe.mockResolvedValue(meNone)
        await w.find('form').trigger('submit')
        await flushPromises()

        expect(getMe).toHaveBeenCalledTimes(2)
        expect(w.find('.login-view').exists()).toBe(false)
        expect(w.find('.player-layout').exists()).toBe(true)
    })
})
