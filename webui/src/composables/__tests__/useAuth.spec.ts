import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { flushPromises } from '@vue/test-utils'
import type { MeResponse } from '@/types/users'

const { getMe, login, logout, clearQueue, stopSync } = vi.hoisted(() => ({
    getMe: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
    clearQueue: vi.fn(),
    stopSync: vi.fn()
}))
vi.mock('@/lib/api/Users', () => ({ getMe }))
vi.mock('@/lib/api/Auth', () => ({ login, logout }))
// The session purge reaches into the player and the queue sync; stand in spies
// so the tests observe the calls without dragging audio elements into jsdom.
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ clearQueue }) }))
vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({ stop: stopSync, start: vi.fn(), restore: vi.fn() })
}))

// useAuth holds module state (the expiry watcher and its once-per-session
// purge latch), so every test gets a fresh module. authState must come from
// the same registry generation — a top-level import would point at a stale
// sessionExpired the reloaded useAuth no longer reads.
type UseAuth = (typeof import('@/composables/useAuth'))['useAuth']
let useAuth: UseAuth
let sessionExpired: (typeof import('@/lib/authState'))['sessionExpired']

function withAuth() {
    let auth!: ReturnType<UseAuth>
    const Host = defineComponent({
        setup() {
            auth = useAuth()
            return () => null
        }
    })
    const qc = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } }
    })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient: qc }]] } })
    return auth
}

const meNone: MeResponse = {
    authMethod: 'none',
    user: null,
    features: { userManagement: false }
}
const meAnonymous: MeResponse = {
    authMethod: 'native',
    user: null,
    features: { userManagement: true }
}
const meAlice: MeResponse = {
    authMethod: 'native',
    user: { login: 'alice', role: 'admin' },
    features: { userManagement: true }
}
const meBob: MeResponse = {
    authMethod: 'native',
    user: { login: 'bob', role: 'user' },
    features: { userManagement: true }
}

describe('useAuth', () => {
    beforeEach(async () => {
        vi.resetModules()
        ;({ useAuth } = await import('@/composables/useAuth'))
        ;({ sessionExpired } = await import('@/lib/authState'))
        sessionExpired.value = false
        localStorage.clear()
        getMe.mockReset()
        login.mockReset()
        logout.mockReset()
        clearQueue.mockReset()
        stopSync.mockReset()
    })

    it('never requires login with auth method none', async () => {
        getMe.mockResolvedValue(meNone)
        const auth = withAuth()
        await flushPromises()
        expect(auth.authRequired.value).toBe(false)
        expect(auth.needsLogin.value).toBe(false)
    })

    it('requires login when native auth reports no identity', async () => {
        getMe.mockResolvedValue(meAnonymous)
        const auth = withAuth()
        await flushPromises()
        expect(auth.needsLogin.value).toBe(true)
        expect(auth.currentUser.value).toBeNull()
    })

    it('does not require login with a session identity', async () => {
        getMe.mockResolvedValue(meAlice)
        const auth = withAuth()
        await flushPromises()
        expect(auth.needsLogin.value).toBe(false)
        expect(auth.currentUser.value).toEqual({ login: 'alice', role: 'admin' })
    })

    // isAdmin drives every admin affordance in the UI; the /api/v1 guard
    // enforces the same policy server-side.
    it('reports admin for an admin session and not for a regular user', async () => {
        getMe.mockResolvedValue(meAlice)
        let auth = withAuth()
        await flushPromises()
        expect(auth.isAdmin.value).toBe(true)

        getMe.mockResolvedValue(meBob)
        auth = withAuth()
        await flushPromises()
        expect(auth.isAdmin.value).toBe(false)
    })

    // With auth method "none" nothing is restricted: every visitor may
    // administer the server, matching the open /api/v1.
    it('treats every visitor as admin when the server needs no login', async () => {
        getMe.mockResolvedValue(meNone)
        const auth = withAuth()
        await flushPromises()
        expect(auth.isAdmin.value).toBe(true)
    })

    it('is not admin while /me is still loading', () => {
        getMe.mockReturnValue(new Promise(() => {}))
        const auth = withAuth()
        expect(auth.isAdmin.value).toBe(false)
    })

    // A 401 mid-session flips the shared flag; the gate reopens even though
    // the cached /me still says alice.
    it('requires login again when the session expires', async () => {
        getMe.mockResolvedValue(meAlice)
        const auth = withAuth()
        await flushPromises()
        sessionExpired.value = true
        expect(auth.needsLogin.value).toBe(true)
    })

    it('clears the expired flag and refetches /me after login', async () => {
        getMe.mockResolvedValue(meAnonymous)
        login.mockResolvedValue({ done: true })
        const auth = withAuth()
        await flushPromises()
        sessionExpired.value = true
        await flushPromises()

        getMe.mockResolvedValue(meAlice)
        await auth.login.mutateAsync({ username: 'alice', password: 'pw', rememberMe: true })
        await flushPromises()

        expect(login).toHaveBeenCalledWith('alice', 'pw', true)
        expect(sessionExpired.value).toBe(false)
        expect(auth.needsLogin.value).toBe(false)
    })

    it('drops the cache and re-bootstraps after logout', async () => {
        getMe.mockResolvedValue(meAlice)
        logout.mockResolvedValue(undefined)
        const auth = withAuth()
        await flushPromises()

        getMe.mockResolvedValue(meAnonymous)
        await auth.logout.mutateAsync()
        await flushPromises()

        expect(logout).toHaveBeenCalled()
        expect(auth.needsLogin.value).toBe(true)
    })

    // Logging out ends the local session entirely: playback stops (the queue
    // sync unbinds first so the emptied queue is not pushed to the server)
    // and localStorage keeps nothing of the old user.
    it('stops playback and empties localStorage on logout', async () => {
        getMe.mockResolvedValue(meAlice)
        logout.mockResolvedValue(undefined)
        const auth = withAuth()
        await flushPromises()
        localStorage.setItem('musicPlayer:queue', '[{"id":"s1"}]')

        getMe.mockResolvedValue(meAnonymous)
        await auth.logout.mutateAsync()
        await flushPromises()

        expect(stopSync).toHaveBeenCalled()
        expect(clearQueue).toHaveBeenCalled()
        expect(stopSync.mock.invocationCallOrder[0]).toBeLessThan(
            clearQueue.mock.invocationCallOrder[0]
        )
        expect(localStorage.getItem('musicPlayer:queue')).toBeNull()
    })

    // An expired session scrubs the device exactly like an explicit logout.
    it('stops playback and empties localStorage when the session expires', async () => {
        getMe.mockResolvedValue(meAlice)
        withAuth()
        await flushPromises()
        localStorage.setItem('musicPlayer:queue', '[{"id":"s1"}]')

        sessionExpired.value = true
        await flushPromises()

        expect(stopSync).toHaveBeenCalled()
        expect(clearQueue).toHaveBeenCalled()
        expect(localStorage.getItem('musicPlayer:queue')).toBeNull()
    })

    // The purge's own cache reset refetches queries that now 401, flipping
    // sessionExpired again — that echo must not purge a second time.
    it('purges only once per lost session', async () => {
        getMe.mockResolvedValue(meAlice)
        withAuth()
        await flushPromises()

        sessionExpired.value = true
        await flushPromises()
        // The echo: another /api/v1 call 401s while the gate is already open.
        sessionExpired.value = true
        await flushPromises()

        expect(clearQueue).toHaveBeenCalledTimes(1)
    })

    it('purges again for a session lost after a fresh login', async () => {
        getMe.mockResolvedValue(meAlice)
        login.mockResolvedValue({ done: true })
        const auth = withAuth()
        await flushPromises()

        sessionExpired.value = true
        await flushPromises()
        expect(clearQueue).toHaveBeenCalledTimes(1)

        await auth.login.mutateAsync({ username: 'alice', password: 'pw', rememberMe: false })
        await flushPromises()

        sessionExpired.value = true
        await flushPromises()
        expect(clearQueue).toHaveBeenCalledTimes(2)
    })
})
