import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { defineComponent, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import { flushPromises } from '@vue/test-utils'
import type { MeResponse } from '@/types/users'

const { getMe, login, logout, clearQueue, stopSync, mintSpaToken } = vi.hoisted(() => ({
    getMe: vi.fn(),
    login: vi.fn(),
    logout: vi.fn(),
    clearQueue: vi.fn(),
    stopSync: vi.fn(),
    mintSpaToken: vi.fn()
}))
vi.mock('@/lib/api/Users', () => ({ getMe }))
vi.mock('@/lib/api/Auth', () => ({ login, logout }))
vi.mock('@/lib/api/Tokens', () => ({ mintSpaToken }))
// The session purge reaches into the player and the queue sync; stand in spies
// so the tests observe the calls without dragging audio elements into jsdom.
vi.mock('@/composables/usePlayer', () => ({ usePlayer: () => ({ clearQueue }) }))
vi.mock('@/composables/useQueueSync', () => ({
    useQueueSync: () => ({ stop: stopSync, start: vi.fn(), restore: vi.fn() })
}))

// Where the SPA runs: a browser tab on the server's origin unless a case moves
// it into the desktop app. The IPC wrappers are stood in with what they do to
// the host's refs — setServerUrl saves the server and the kind, keeping the key
// only for the same server saved again as a key session (Rust's rule),
// clearServerUrl forgets the server, the key and the kind, clearApiKey forgets
// the key, setApiKey installs one; the session hand-off has its own spec, but
// clearSession — what resetSubsonicSession tells the Rust client — is counted.
const host = vi.hoisted(() => ({
    desktopApp: false,
    setServerUrl: vi.fn(),
    clearServerUrl: vi.fn(),
    clearApiKey: vi.fn(),
    clearSession: vi.fn(),
    setSessionKind: vi.fn(),
    setApiKey: vi.fn()
}))
vi.mock('@/lib/host', async () => {
    const { ref } = await import('vue')
    const serverUrl = ref<string | null>('http://localhost:3000')
    const sessionKind = ref<'cookie' | 'apiKey'>('cookie')
    const apiKey = ref<string | null>(null)
    return {
        isDesktopApp: () => host.desktopApp,
        serverUrl,
        sessionKind,
        apiKey,
        httpFetch: (input: RequestInfo | URL, init?: RequestInit) => fetch(input, init),
        setServerUrl: async (url: string, kind: 'cookie' | 'apiKey') => {
            host.setServerUrl(url, kind)
            if (kind !== 'apiKey' || url !== serverUrl.value) apiKey.value = null
            sessionKind.value = kind
            serverUrl.value = url
            return url
        },
        clearServerUrl: async () => {
            host.clearServerUrl()
            apiKey.value = null
            sessionKind.value = 'cookie'
            serverUrl.value = null
        },
        clearApiKey: async () => {
            host.clearApiKey()
            apiKey.value = null
        },
        setSessionKind: async (kind: 'cookie' | 'apiKey') => {
            host.setSessionKind(kind)
            if (kind !== 'apiKey') apiKey.value = null
            sessionKind.value = kind
        },
        setApiKey: async (key: string) => {
            host.setApiKey(key)
            sessionKind.value = 'apiKey'
            apiKey.value = key
        },
        setSession: async () => {},
        clearSession: async () => {
            host.clearSession()
        }
    }
})

// useAuth holds module state (the expiry watcher and its once-per-session
// purge latch), so every test gets a fresh module. authState must come from
// the same registry generation — a top-level import would point at a stale
// sessionExpired the reloaded useAuth no longer reads.
type UseAuth = (typeof import('@/composables/useAuth'))['useAuth']
let useAuth: UseAuth
let sessionExpired: (typeof import('@/lib/authState'))['sessionExpired']
let explicitLogout: (typeof import('@/lib/authState'))['explicitLogout']
let sessionLostUnexpectedly: (typeof import('@/lib/authState'))['sessionLostUnexpectedly']
let subsonicClient: (typeof import('@/lib/api/subsonic'))['subsonicClient']
let subsonicReady: (typeof import('@/lib/subsonicSession'))['subsonicReady']
let spaTokenId: (typeof import('@/lib/subsonicSession'))['spaTokenId']
let MAX_MINT_ATTEMPTS: (typeof import('@/lib/subsonicSession'))['MAX_MINT_ATTEMPTS']
let serverUrl: (typeof import('@/lib/host'))['serverUrl']
let sessionKind: (typeof import('@/lib/host'))['sessionKind']
let apiKey: (typeof import('@/lib/host'))['apiKey']
let lastServerUrl: (typeof import('@/lib/authState'))['lastServerUrl']

// Mounts a host component that runs useAuth over a fresh QueryClient. The client
// comes back with the composable for the cases that reach the cache themselves,
// and onRender sees the gate's state at every render of the host.
function mountAuth(onRender?: (auth: ReturnType<UseAuth>) => void) {
    let auth!: ReturnType<UseAuth>
    const Host = defineComponent({
        setup() {
            auth = useAuth()
            return () => {
                onRender?.(auth)
                return null
            }
        }
    })
    const qc = new QueryClient({
        defaultOptions: { queries: { retry: false }, mutations: { retry: false } }
    })
    mount(Host, { global: { plugins: [[VueQueryPlugin, { queryClient: qc }]] } })
    return { auth, qc }
}

function withAuth() {
    return mountAuth().auth
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
        ;({ sessionExpired, explicitLogout, sessionLostUnexpectedly } = await import(
            '@/lib/authState'
        ))
        ;({ subsonicClient } = await import('@/lib/api/subsonic'))
        ;({ subsonicReady, spaTokenId, MAX_MINT_ATTEMPTS } = await import(
            '@/lib/subsonicSession'
        ))
        // The mocked host outlives resetModules: its refs are reset by hand.
        ;({ serverUrl, sessionKind, apiKey } = await import('@/lib/host'))
        serverUrl.value = 'http://localhost:3000'
        sessionKind.value = 'cookie'
        apiKey.value = null
        ;({ lastServerUrl } = await import('@/lib/authState'))
        host.desktopApp = false
        host.setServerUrl.mockReset()
        host.clearServerUrl.mockReset()
        host.clearApiKey.mockReset()
        host.clearSession.mockReset()
        host.setSessionKind.mockReset()
        host.setApiKey.mockReset()
        sessionExpired.value = false
        explicitLogout.value = false
        subsonicReady.value = false
        spaTokenId.value = null
        localStorage.clear()
        getMe.mockReset()
        login.mockReset()
        logout.mockReset()
        clearQueue.mockReset()
        stopSync.mockReset()
        mintSpaToken.mockReset()
        // Default mock: return a valid token so tests that don't care about
        // minting don't crash trying to call .then on undefined
        mintSpaToken.mockResolvedValue({
            token: 'aether_default_key',
            tokenId: 'tid_default',
            expiresAt: '2999-01-01T00:00:00Z'
        })
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

    // isAdmin drives every admin affordance in the UI; the /api/v0 guard
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
    // administer the server, matching the open /api/v0.
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

    // The device id is not user data: it identifies THIS browser's session
    // slot on the server. Wiping it on logout would leave the next login
    // minting under a fresh identity, orphaning this browser's session and
    // stacking one dead row per logout/login cycle.
    it('keeps this browser device identity across the logout purge', async () => {
        const { getDeviceId, DEVICE_ID_KEY } = await import('@/lib/deviceIdentity')
        getMe.mockResolvedValue(meAlice)
        logout.mockResolvedValue(undefined)
        const auth = withAuth()
        await flushPromises()
        const deviceId = getDeviceId()

        getMe.mockResolvedValue(meAnonymous)
        await auth.logout.mutateAsync()
        await flushPromises()

        expect(localStorage.getItem(DEVICE_ID_KEY)).toBe(JSON.stringify(deviceId))
        expect(getDeviceId()).toBe(deviceId)
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

    // The purge's cache reset refetches queries that now 401, so sessionExpired
    // ends up set either way — only the logout intent tells the login view not
    // to blame an expiry for a form the user asked for.
    it('marks a deliberate logout so no expiry is reported', async () => {
        getMe.mockResolvedValue(meAlice)
        logout.mockResolvedValue(undefined)
        const auth = withAuth()
        await flushPromises()

        getMe.mockResolvedValue(meAnonymous)
        await auth.logout.mutateAsync()
        await flushPromises()

        expect(explicitLogout.value).toBe(true)
        expect(sessionLostUnexpectedly.value).toBe(false)
    })

    // A failed logout request still ends the local session (onSettled purges
    // regardless), so it stays a deliberate one.
    it('marks the logout even when the request fails', async () => {
        getMe.mockResolvedValue(meAlice)
        logout.mockRejectedValue(new Error('offline'))
        const auth = withAuth()
        await flushPromises()

        await expect(auth.logout.mutateAsync()).rejects.toThrow()
        await flushPromises()

        expect(explicitLogout.value).toBe(true)
    })

    it('reports an unexpected loss when the session expires on its own', async () => {
        getMe.mockResolvedValue(meAlice)
        withAuth()
        await flushPromises()

        sessionExpired.value = true
        await flushPromises()

        expect(sessionLostUnexpectedly.value).toBe(true)
    })

    // Signing in again re-arms the distinction: a session lost after this
    // login is an expiry, not the previous logout.
    it('clears the logout mark after a fresh login', async () => {
        getMe.mockResolvedValue(meAlice)
        logout.mockResolvedValue(undefined)
        login.mockResolvedValue({ done: true })
        const auth = withAuth()
        await flushPromises()

        getMe.mockResolvedValue(meAnonymous)
        await auth.logout.mutateAsync()
        await flushPromises()

        getMe.mockResolvedValue(meAlice)
        await auth.login.mutateAsync({ username: 'alice', password: 'pw', rememberMe: false })
        await flushPromises()

        expect(explicitLogout.value).toBe(false)

        sessionExpired.value = true
        expect(sessionLostUnexpectedly.value).toBe(true)
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
        // The echo: another /api/v0 call 401s while the gate is already open.
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

    describe('spa token lifecycle', () => {
        it('sets subsonicReady with initWithDefaults for auth none', async () => {
            getMe.mockResolvedValue(meNone)
            withAuth()
            await flushPromises()
            expect(subsonicReady.value).toBe(true)
            expect(subsonicClient.isConfigured()).toBe(true)
            expect(mintSpaToken).not.toHaveBeenCalled()
        })

        it('mints a token and sets subsonicReady for native auth with a user', async () => {
            mintSpaToken.mockResolvedValue({
                token: 'aether_test_key',
                tokenId: 'tid1',
                expiresAt: '2999-01-01T00:00:00Z'
            })
            getMe.mockResolvedValue(meAlice)
            withAuth()
            await flushPromises()
            expect(mintSpaToken).toHaveBeenCalledTimes(1)
            expect(subsonicReady.value).toBe(true)
            expect(subsonicClient.hasApiKey()).toBe(true)
            expect(spaTokenId.value).toBe('tid1')
        })

        it('flips sessionExpired on boot-mint failure (non-401 errors)', async () => {
            mintSpaToken.mockRejectedValue({ response: { status: 500 } })
            getMe.mockResolvedValue(meAlice)
            withAuth()
            await flushPromises()
            expect(mintSpaToken).toHaveBeenCalledTimes(1)
            expect(sessionExpired.value).toBe(true)
            expect(subsonicReady.value).toBe(false)
        })

        // The failure feedback loop: flipping sessionExpired opens the login
        // gate, whose purge refetches /me and re-runs this watcher. With a
        // persistent non-401 failure that is an unbounded mint loop, so the
        // attempts are capped per session-epoch.
        it('stops re-minting after MAX_MINT_ATTEMPTS consecutive failures', async () => {
            mintSpaToken.mockRejectedValue({ response: { status: 500 } })
            getMe.mockResolvedValue(meAlice)
            withAuth()
            await flushPromises()
            expect(mintSpaToken).toHaveBeenCalledTimes(1)
            expect(sessionExpired.value).toBe(true)

            // Each cycle stands in for one gate-purge-refetch round trip.
            for (let i = 0; i < MAX_MINT_ATTEMPTS - 1; i++) {
                sessionExpired.value = false
                await flushPromises()
            }
            expect(mintSpaToken).toHaveBeenCalledTimes(MAX_MINT_ATTEMPTS)
            // The gate stays up rather than spinning: the last failure set it.
            expect(sessionExpired.value).toBe(true)

            // Budget spent: further gate cycles no longer mint.
            sessionExpired.value = false
            await flushPromises()
            expect(mintSpaToken).toHaveBeenCalledTimes(MAX_MINT_ATTEMPTS)
            expect(subsonicReady.value).toBe(false)
        })

        // A fresh login is a new session-epoch: the budget is re-armed so a
        // transient outage does not permanently wedge the SPA.
        it('re-arms the mint budget after a successful login', async () => {
            mintSpaToken.mockRejectedValue({ response: { status: 500 } })
            getMe.mockResolvedValue(meAlice)
            login.mockResolvedValue({ done: true })
            const auth = withAuth()
            await flushPromises()
            // Spend the budget, leaving the gate up (the last failure set it).
            for (let i = 0; i < MAX_MINT_ATTEMPTS - 1; i++) {
                sessionExpired.value = false
                await flushPromises()
            }
            expect(mintSpaToken).toHaveBeenCalledTimes(MAX_MINT_ATTEMPTS)
            expect(sessionExpired.value).toBe(true)

            mintSpaToken.mockResolvedValue({
                token: 'aether_after_login',
                tokenId: 'tid-after',
                expiresAt: '2999-01-01T00:00:00Z'
            })
            await auth.login.mutateAsync({ username: 'alice', password: 'pw', rememberMe: false })
            await flushPromises()

            expect(mintSpaToken).toHaveBeenCalledTimes(MAX_MINT_ATTEMPTS + 1)
            expect(subsonicReady.value).toBe(true)
            expect(spaTokenId.value).toBe('tid-after')
        })

        // 'session-gone' (a 401 mint) is unchanged: no budget is consumed, the
        // gate is opened by remintApiKey itself.
        it('does not consume the mint budget on a 401 (session-gone)', async () => {
            mintSpaToken.mockRejectedValue({ response: { status: 401 } })
            getMe.mockResolvedValue(meAlice)
            withAuth()
            await flushPromises()
            expect(sessionExpired.value).toBe(true)

            for (let i = 0; i < 5; i++) {
                sessionExpired.value = false
                await flushPromises()
            }
            // Every cycle still attempts: a lost session is cured by logging in,
            // not by a retry cap.
            expect(mintSpaToken.mock.calls.length).toBeGreaterThan(MAX_MINT_ATTEMPTS)
        })

        it('re-mints after logout+login', async () => {
            // First mint
            mintSpaToken.mockResolvedValueOnce({
                token: 'aether_key1',
                tokenId: 'tid1',
                expiresAt: '2999-01-01T00:00:00Z'
            })
            getMe.mockResolvedValue(meAlice)
            login.mockResolvedValue({ done: true })
            logout.mockResolvedValue(undefined)
            const auth = withAuth()
            await flushPromises()
            expect(mintSpaToken).toHaveBeenCalledTimes(1)
            expect(spaTokenId.value).toBe('tid1')

            // Logout clears the key; /me becomes anonymous so no re-mint
            getMe.mockResolvedValue(meAnonymous)
            await auth.logout.mutateAsync()
            await flushPromises()
            expect(subsonicClient.hasApiKey()).toBe(false)
            expect(spaTokenId.value).toBeNull()

            // Re-login mints a fresh token
            mintSpaToken.mockResolvedValueOnce({
                token: 'aether_key2',
                tokenId: 'tid2',
                expiresAt: '2999-01-01T00:00:00Z'
            })
            getMe.mockResolvedValue(meAlice)
            await auth.login.mutateAsync({ username: 'alice', password: 'pw', rememberMe: false })
            await flushPromises()
            expect(mintSpaToken).toHaveBeenCalledTimes(2)
            expect(spaTokenId.value).toBe('tid2')
        })
    })

    describe('in the desktop app', () => {
        const SERVER = 'http://aether.local:8075'
        const meProxyAnonymous: MeResponse = {
            authMethod: 'proxy-header',
            user: null,
            features: { userManagement: false }
        }
        const meProxyAlice: MeResponse = {
            ...meProxyAnonymous,
            user: { login: 'alice', role: 'admin' }
        }

        beforeEach(() => {
            host.desktopApp = true
            serverUrl.value = SERVER
        })

        it('asks for a server first, and nothing of a server until then', async () => {
            serverUrl.value = null
            getMe.mockResolvedValue(meNone)
            const auth = withAuth()
            await flushPromises()

            expect(auth.needsServer.value).toBe(true)
            // A disabled /me is not loading, so the gate can show the server step.
            expect(auth.isLoading.value).toBe(false)
            expect(getMe).not.toHaveBeenCalled()
        })

        it('asks /me once a server is chosen', async () => {
            serverUrl.value = null
            getMe.mockResolvedValue(meAnonymous)
            const auth = withAuth()
            await flushPromises()

            serverUrl.value = SERVER
            await flushPromises()

            expect(getMe).toHaveBeenCalledTimes(1)
            expect(auth.needsServer.value).toBe(false)
            expect(auth.needsLogin.value).toBe(true)
        })

        // In the browser the server is the page's origin; there is nothing to ask.
        it('never asks for a server in the browser', async () => {
            host.desktopApp = false
            serverUrl.value = null
            getMe.mockResolvedValue(meNone)
            const auth = withAuth()
            await flushPromises()

            expect(auth.needsServer.value).toBe(false)
            expect(getMe).toHaveBeenCalledTimes(1)
        })

        // Down, or moved: without this the window stays blank, with no way to
        // reach another server.
        describe('when the saved server does not answer', () => {
            beforeEach(() => {
                getMe.mockRejectedValueOnce(new Error('Network Error'))
            })

            it('asks for a server again', async () => {
                const auth = withAuth()
                await flushPromises()

                expect(auth.needsServer.value).toBe(true)
                expect(auth.isLoading.value).toBe(false)
            })

            it('stays out of the way in the browser', async () => {
                host.desktopApp = false
                const auth = withAuth()
                await flushPromises()

                expect(auth.needsServer.value).toBe(false)
            })

            // The server step says why (savedServerProblem, Server.spec), read off
            // what /me rejected with.
            it('hands the server step what /me failed with', async () => {
                const failure = { message: 'Request failed', response: { status: 503 } }
                getMe.mockReset()
                getMe.mockRejectedValue(failure)
                const auth = withAuth()
                await flushPromises()

                // (vue-query hands it back reactive: equal, not the same object)
                expect(auth.identityError.value).toEqual(failure)
            })

            // The same address saved again changes nothing /me watches.
            it('asks /me again once a server is saved, even the same one', async () => {
                getMe.mockResolvedValue(meNone)
                const auth = withAuth()
                await flushPromises()
                expect(getMe).toHaveBeenCalledTimes(1)

                auth.serverSaved()
                await flushPromises()

                expect(getMe).toHaveBeenCalledTimes(2)
                expect(auth.needsServer.value).toBe(false)
                expect(subsonicReady.value).toBe(true)
            })
        })

        // The flip from no server enables /me by itself; a reset on top would
        // cancel that request and ask again.
        it('asks a first saved server for /me once', async () => {
            serverUrl.value = null
            getMe.mockResolvedValue(meNone)
            const auth = withAuth()
            await flushPromises()

            // As setServerUrl resolves: the flip has enabled /me by the time the
            // login view tells the gate.
            serverUrl.value = SERVER
            await nextTick()
            auth.serverSaved()
            await flushPromises()

            expect(getMe).toHaveBeenCalledTimes(1)
            expect(subsonicReady.value).toBe(true)
        })

        // What the server step's Connect saves, and how to sign in to it.
        describe('connectTo', () => {
            const OTHER = 'http://other.lan:8075'

            beforeEach(() => {
                getMe.mockResolvedValue(meNone)
            })

            it('saves the address the probe settled on as a cookie session', async () => {
                serverUrl.value = null
                const auth = withAuth()
                await flushPromises()

                await auth.connectTo({ kind: 'aether', url: 'https://aether.example.org', me: meNone })

                expect(host.setServerUrl).toHaveBeenCalledWith('https://aether.example.org', 'cookie')
                expect(serverUrl.value).toBe('https://aether.example.org')
            })

            // Aether behind an auth portal: only an API key gets in, so the gate
            // shows the key card.
            it('saves a portal as a key session', async () => {
                serverUrl.value = null
                const auth = withAuth()
                await flushPromises()

                await auth.connectTo({ kind: 'portal', url: OTHER })

                expect(host.setServerUrl).toHaveBeenCalledWith(OTHER, 'apiKey')
                expect(auth.needsApiKey.value).toBe(true)
            })

            // Connect after an outage, on a server the user chose a key for: saved
            // as a cookie session, Rust would delete the stored key.
            it('saves the saved server of a key session again as a key session', async () => {
                sessionKind.value = 'apiKey'
                apiKey.value = 'aether_k3y'
                const auth = withAuth()
                await flushPromises()

                await auth.connectTo({ kind: 'aether', url: SERVER, me: meAnonymous })

                expect(host.setServerUrl).toHaveBeenCalledWith(SERVER, 'apiKey')
                expect(apiKey.value).toBe('aether_k3y')
            })

            // Another server is not the one the key was stored for.
            it('saves another server as a cookie session, key session or not', async () => {
                sessionKind.value = 'apiKey'
                apiKey.value = 'aether_k3y'
                const auth = withAuth()
                await flushPromises()

                await auth.connectTo({ kind: 'aether', url: OTHER, me: meAnonymous })

                expect(host.setServerUrl).toHaveBeenLastCalledWith(OTHER, 'cookie')
                expect(apiKey.value).toBeNull()
            })

            // The saved server did not answer, and Connect saves it again: nothing
            // is left behind, and the gate asks it again (the same address
            // changes nothing /me watches).
            it('keeps the saved server’s session when it is saved again', async () => {
                getMe.mockRejectedValueOnce(new Error('Network Error'))
                localStorage.setItem('musicPlayer:queue', '[{"id":"t1"}]')
                const auth = withAuth()
                await flushPromises()
                expect(auth.needsServer.value).toBe(true)

                await auth.connectTo({ kind: 'aether', url: SERVER, me: meNone })
                await flushPromises()

                expect(clearQueue).not.toHaveBeenCalled()
                expect(localStorage.getItem('musicPlayer:queue')).toBe('[{"id":"t1"}]')
                expect(getMe).toHaveBeenCalledTimes(2)
                expect(auth.needsServer.value).toBe(false)
            })

            // The saved server is down at launch and the user moves to another one
            // from the server step. As with Change server, nothing of the server
            // left behind may carry over: its queue would play against the next
            // server, and the queue sync would push its track ids there.
            it('leaves the saved server behind before saving another', async () => {
                getMe.mockRejectedValueOnce(new Error('Network Error'))
                localStorage.setItem('musicPlayer:queue', '[{"id":"track-on-a"}]')
                const auth = withAuth()
                await flushPromises()
                expect(auth.needsServer.value).toBe(true)

                await auth.connectTo({ kind: 'aether', url: OTHER, me: meNone })
                await flushPromises()

                expect(clearQueue).toHaveBeenCalled()
                expect(localStorage.getItem('musicPlayer:queue')).toBeNull()
                // left first — the queue sync unbound — and only then the next saved
                expect(stopSync.mock.invocationCallOrder[0]).toBeLessThan(
                    host.setServerUrl.mock.invocationCallOrder[0]
                )
                expect(serverUrl.value).toBe(OTHER)
            })
        })

        it('forgets the server switched away from once another is saved', async () => {
            lastServerUrl.value = 'http://old.lan:8075'
            getMe.mockResolvedValue(meNone)
            const auth = withAuth()
            await flushPromises()

            auth.serverSaved()

            expect(lastServerUrl.value).toBeNull()
        })

        // A proxy that lets an anonymous /me through: the app cannot reach the
        // portal, so the login view asks for an API key instead of a blank window.
        it('asks for an API key when /me answers proxy-header without a user', async () => {
            getMe.mockResolvedValue(meProxyAnonymous)
            const auth = withAuth()
            await flushPromises()

            expect(auth.needsApiKey.value).toBe(true)
            expect(auth.needsLogin.value).toBe(false)
        })

        it.each([
            ['a proxy-header identity', meProxyAlice],
            ['native auth', meAnonymous],
            ['no auth', meNone]
        ])('asks for no API key for %s', async (_case, me) => {
            getMe.mockResolvedValue(me)
            const auth = withAuth()
            await flushPromises()

            expect(auth.needsApiKey.value).toBe(false)
        })

        it('leaves the portal to the proxy in the browser', async () => {
            host.desktopApp = false
            getMe.mockResolvedValue(meProxyAnonymous)
            const auth = withAuth()
            await flushPromises()

            expect(auth.needsApiKey.value).toBe(false)
        })

        // Everything riding /api/v0 renders only on managementApi; a cookie
        // session with an answered /me has it, whatever the auth method.
        it.each([
            ['no auth', meNone],
            ['native auth', meAlice],
            ['a proxy-header identity', meProxyAlice]
        ])('has the management API with %s', async (_case, me) => {
            getMe.mockResolvedValue(me)
            const auth = withAuth()
            await flushPromises()

            expect(auth.managementApi.value).toBe(true)
        })

        describe('switchServer', () => {
            afterEach(() => {
                vi.unstubAllGlobals()
            })

            // The order is the point. The logout goes out while the server is
            // still set (the api client refuses to send once it is forgotten);
            // the server is forgotten before the purge, whose cache reset must
            // not refetch /me from it; the saved setting goes last.
            it('signs out, forgets the server, purges, then drops the setting', async () => {
                getMe.mockResolvedValue(meAlice)
                const events: string[] = []
                logout.mockImplementation(async () => {
                    events.push(`logout, server ${serverUrl.value}`)
                })
                stopSync.mockImplementation(() => {
                    events.push(`purge, server ${serverUrl.value}`)
                })
                host.clearServerUrl.mockImplementation(() => {
                    events.push('setting cleared')
                })
                const auth = withAuth()
                await flushPromises()

                await auth.switchServer()

                expect(events).toEqual([
                    `logout, server ${SERVER}`,
                    'purge, server null',
                    'setting cleared'
                ])
                expect(logout).toHaveBeenCalledWith('tid_default')
                expect(getMe).toHaveBeenCalledTimes(1)
                expect(clearQueue).toHaveBeenCalled()
                expect(subsonicClient.hasApiKey()).toBe(false)
                expect(subsonicReady.value).toBe(false)
                expect(auth.needsServer.value).toBe(true)
            })

            // Nobody to sign out, but the old server's queue must not play on,
            // nor its track ids reach the next server through the queue sync.
            it.each([
                ['without login', meNone],
                ['with nobody signed in', meAnonymous]
            ])('signs nobody out %s, but purges all the same', async (_case, me) => {
                getMe.mockResolvedValue(me)
                const events: string[] = []
                stopSync.mockImplementation(() => {
                    events.push(`purge, server ${serverUrl.value}`)
                })
                host.clearServerUrl.mockImplementation(() => {
                    events.push('setting cleared')
                })
                const { DEVICE_ID_KEY } = await import('@/lib/deviceIdentity')
                localStorage.setItem(DEVICE_ID_KEY, '"device-1"')
                const auth = withAuth()
                await flushPromises()
                localStorage.setItem('musicPlayer:queue', '[{"id":"s1"}]')
                expect(subsonicReady.value).toBe(me === meNone)

                await auth.switchServer()

                expect(logout).not.toHaveBeenCalled()
                expect(events).toEqual(['purge, server null', 'setting cleared'])
                expect(clearQueue).toHaveBeenCalled()
                expect(localStorage.getItem('musicPlayer:queue')).toBeNull()
                expect(localStorage.getItem(DEVICE_ID_KEY)).toBe('"device-1"')
                expect(getMe).toHaveBeenCalledTimes(1)
                expect(subsonicReady.value).toBe(false)
                // Nothing addresses the server left behind, with or without login.
                expect(subsonicClient.isConfigured()).toBe(false)
                expect(auth.needsServer.value).toBe(true)
            })

            it('remembers the server it switched away from', async () => {
                getMe.mockResolvedValue(meNone)
                const auth = withAuth()
                await flushPromises()

                await auth.switchServer()

                expect(lastServerUrl.value).toBe(SERVER)
                expect(serverUrl.value).toBeNull()
            })

            it('forgets the server even when the logout fails', async () => {
                getMe.mockResolvedValue(meAlice)
                logout.mockRejectedValue(new Error('offline'))
                const auth = withAuth()
                await flushPromises()

                await expect(auth.switchServer()).resolves.toBeUndefined()

                expect(clearQueue).toHaveBeenCalled()
                expect(host.clearServerUrl).toHaveBeenCalledTimes(1)
                expect(auth.needsServer.value).toBe(true)
            })

            // Only the saved setting stays stale; choosing the next server
            // replaces it, so the caller has nothing to handle.
            it('resolves when the saved setting cannot be cleared', async () => {
                getMe.mockResolvedValue(meNone)
                host.clearServerUrl.mockImplementation(() => {
                    throw { code: 'Internal', message: 'read-only file system' }
                })
                const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
                const auth = withAuth()
                await flushPromises()

                await expect(auth.switchServer()).resolves.toBeUndefined()

                expect(warn).toHaveBeenCalledWith('Could not forget the saved server')
                expect(auth.needsServer.value).toBe(true)
                warn.mockRestore()
            })

            // An expiry belongs to the server being left. Kept, the flag would
            // stop a server without login from ever rendering the app.
            it('starts the next server afresh after an expired session', async () => {
                getMe.mockResolvedValue(meAlice)
                const auth = withAuth()
                await flushPromises()
                getMe.mockResolvedValue(meAnonymous)
                sessionExpired.value = true
                await flushPromises()
                expect(auth.needsLogin.value).toBe(true)

                await auth.switchServer()
                getMe.mockResolvedValue(meNone)
                serverUrl.value = 'http://other.lan:8075'
                await flushPromises()

                expect(sessionExpired.value).toBe(false)
                expect(auth.needsLogin.value).toBe(false)
                expect(subsonicReady.value).toBe(true)
            })

            it('asks the next server for a login without blaming an expiry', async () => {
                getMe.mockResolvedValue(meAlice)
                const auth = withAuth()
                await flushPromises()

                await auth.switchServer()
                getMe.mockResolvedValue(meAnonymous)
                serverUrl.value = 'http://other.lan:8075'
                await flushPromises()

                expect(auth.needsLogin.value).toBe(true)
                expect(sessionLostUnexpectedly.value).toBe(false)
            })

            // As the login view's Connect does once the probe has answered.
            // The server step's Connect, once the probe found Aether at `url`.
            async function connectOnServerStep(
                url: string,
                me: MeResponse,
                auth: ReturnType<UseAuth>
            ) {
                getMe.mockResolvedValue(me)
                await auth.connectTo({ kind: 'aether', url, me })
                await flushPromises()
            }

            // A poll on the wire when the switch starts answers 401 only once
            // the switch is over — before the next server is chosen or after.
            // It speaks for the session just ended: it flags nothing, and a
            // next server without login renders.
            it.each(['before', 'after'])(
                'lets a 401 that lands after the switch, %s Connect, hold nothing back',
                async (when) => {
                    getMe.mockResolvedValue(meAlice)
                    const auth = withAuth()
                    await flushPromises()
                    let answer!: (response: Response) => void
                    vi.stubGlobal(
                        'fetch',
                        vi.fn(
                            () =>
                                new Promise<Response>((resolve) => {
                                    answer = resolve
                                })
                        )
                    )
                    const { apiClient } = await import('@/lib/api/client')
                    const poll = apiClient.get('/tasks/executions').catch((err: unknown) => err)
                    await flushPromises()
                    const late401 = async () => {
                        answer(new Response(null, { status: 401 }))
                        await expect(poll).resolves.toMatchObject({ response: { status: 401 } })
                        await flushPromises()
                    }

                    await auth.switchServer()
                    if (when === 'before') await late401()
                    await connectOnServerStep('http://other.lan:8075', meNone, auth)
                    if (when === 'after') await late401()

                    expect(sessionExpired.value).toBe(false)
                    expect(subsonicReady.value).toBe(true)
                }
            )

            // A mint the switch overtook is refused once the switch is over —
            // before the next server is chosen or after. Its outcome belongs
            // to the session just ended: it flags nothing, and a next server
            // without login renders.
            it.each(['before', 'after'])(
                'lets a mint the switch overtook fail, %s Connect, without holding anything back',
                async (when) => {
                    let failMint!: (reason: unknown) => void
                    mintSpaToken.mockReturnValue(
                        new Promise((_resolve, reject) => {
                            failMint = reject
                        })
                    )
                    getMe.mockResolvedValue(meAlice)
                    const auth = withAuth()
                    await flushPromises()
                    const lateRefusal = async () => {
                        failMint({ response: { status: 401 } })
                        await flushPromises()
                    }

                    await auth.switchServer()
                    if (when === 'before') await lateRefusal()
                    await connectOnServerStep('http://other.lan:8075', meNone, auth)
                    if (when === 'after') await lateRefusal()

                    expect(sessionExpired.value).toBe(false)
                    expect(subsonicReady.value).toBe(true)
                }
            )

            // Whatever still flagged the old session between the switch and
            // Connect belongs to the server left behind: the server saved next
            // starts afresh, and without login it renders.
            it('starts the server saved next afresh, whatever flagged the old session', async () => {
                getMe.mockResolvedValue(meAlice)
                const auth = withAuth()
                await flushPromises()

                await auth.switchServer()
                sessionExpired.value = true
                await flushPromises()
                await connectOnServerStep('http://other.lan:8075', meNone, auth)

                expect(sessionExpired.value).toBe(false)
                expect(subsonicReady.value).toBe(true)
            })
        })
    })

    describe('useAuth in a key session (desktop app)', () => {
        const SERVER = 'http://aether.local:8075'
        const userEnvelope = (username: string, adminRole: boolean) => ({
            'subsonic-response': {
                status: 'ok',
                version: '1.16.1',
                type: 'Aether',
                user: { username, adminRole }
            }
        })
        const refused = {
            'subsonic-response': {
                status: 'failed',
                version: '1.16.1',
                type: 'Aether',
                error: { code: 44, message: 'Invalid API key' }
            }
        }
        // The real subsonic client asks getUser through the mocked host's httpFetch,
        // which is the page fetch: stub it with the server's answer.
        function restAnswers(body: unknown) {
            vi.stubGlobal(
                'fetch',
                vi.fn().mockResolvedValue({ ok: true, json: () => Promise.resolve(body) })
            )
        }

        beforeEach(() => {
            host.desktopApp = true
            serverUrl.value = SERVER
            sessionKind.value = 'apiKey'
        })

        afterEach(() => {
            vi.unstubAllGlobals()
        })

        it('installs the stored key, asks getUser and never /me', async () => {
            apiKey.value = 'aether_k3y'
            restAnswers(userEnvelope('alice', true))
            const auth = withAuth()
            await flushPromises()

            expect(getMe).not.toHaveBeenCalled()
            expect(mintSpaToken).not.toHaveBeenCalled()
            expect(subsonicReady.value).toBe(true)
            expect(subsonicClient.hasApiKey()).toBe(true)
            expect(auth.currentUser.value).toEqual({ login: 'alice', role: 'admin' })
            expect(auth.isAdmin.value).toBe(true)
            expect(auth.managementApi.value).toBe(false)
            expect(auth.authRequired.value).toBe(false)
            expect(auth.canSignOut.value).toBe(true)
            expect(auth.needsLogin.value).toBe(false)
            expect(auth.needsApiKey.value).toBe(false)
            expect(auth.needsServer.value).toBe(false)
            expect(auth.isLoading.value).toBe(false)
        })

        it('is loading until getUser answers, and a regular user is no admin', async () => {
            apiKey.value = 'aether_k3y'
            let answer!: (response: unknown) => void
            vi.stubGlobal(
                'fetch',
                vi.fn(
                    () =>
                        new Promise((resolve) => {
                            answer = resolve
                        })
                )
            )
            const auth = withAuth()
            await flushPromises()
            expect(auth.isLoading.value).toBe(true)
            expect(auth.isAdmin.value).toBe(false)

            answer({ ok: true, json: () => Promise.resolve(userEnvelope('bob', false)) })
            await flushPromises()

            expect(auth.isLoading.value).toBe(false)
            expect(auth.currentUser.value).toEqual({ login: 'bob', role: 'user' })
            expect(auth.isAdmin.value).toBe(false)
        })

        it('asks for a key when none is stored, and asks the server nothing', async () => {
            apiKey.value = null
            const fetchMock = vi.fn()
            vi.stubGlobal('fetch', fetchMock)
            const auth = withAuth()
            await flushPromises()

            expect(auth.needsApiKey.value).toBe(true)
            expect(auth.needsServer.value).toBe(false)
            expect(auth.isLoading.value).toBe(false)
            expect(subsonicReady.value).toBe(false)
            expect(fetchMock).not.toHaveBeenCalled()
            expect(getMe).not.toHaveBeenCalled()
        })

        // The server stopped accepting the key (revoked in the web UI): like an
        // expired session — the key is forgotten, the device purged, and the key
        // card explains. The kind stays, so a relaunch comes back to that card.
        it('treats a refused key as a lost session: forgets it and purges', async () => {
            apiKey.value = 'aether_revoked'
            restAnswers(refused)
            const auth = withAuth()
            await flushPromises()

            expect(sessionExpired.value).toBe(true)
            expect(host.clearApiKey).toHaveBeenCalledTimes(1)
            expect(apiKey.value).toBeNull()
            expect(clearQueue).toHaveBeenCalled()
            expect(subsonicReady.value).toBe(false)
            expect(auth.needsApiKey.value).toBe(true)
            expect(auth.needsServer.value).toBe(false)
            expect(sessionLostUnexpectedly.value).toBe(true)
            expect(sessionKind.value).toBe('apiKey')
        })

        // Down, or moved: the server step, as for a failed /me in the cookie flow.
        // Nothing says the key is wrong, so it is kept.
        it('asks for a server again when getUser cannot be reached', async () => {
            apiKey.value = 'aether_k3y'
            vi.stubGlobal(
                'fetch',
                vi.fn().mockRejectedValue(`error sending request for url (${SERVER}/rest/getUser.view)`)
            )
            const auth = withAuth()
            await flushPromises()

            expect(auth.needsServer.value).toBe(true)
            expect(auth.needsApiKey.value).toBe(false)
            expect(auth.isLoading.value).toBe(false)
            expect(host.clearApiKey).not.toHaveBeenCalled()
            expect(apiKey.value).toBe('aether_k3y')
        })

        it('hands the server step what getUser failed with', async () => {
            apiKey.value = 'aether_k3y'
            const rejection = `error sending request for url (${SERVER}/rest/getUser.view)`
            vi.stubGlobal('fetch', vi.fn().mockRejectedValue(rejection))
            const auth = withAuth()
            await flushPromises()

            expect(auth.identityError.value).toBe(rejection)
        })

        it('logs out by forgetting the key, deliberately, with no /api/v0 call', async () => {
            apiKey.value = 'aether_k3y'
            restAnswers(userEnvelope('alice', false))
            const auth = withAuth()
            await flushPromises()

            await auth.logout.mutateAsync()
            await flushPromises()

            expect(logout).not.toHaveBeenCalled()
            expect(host.clearApiKey).toHaveBeenCalledTimes(1)
            expect(apiKey.value).toBeNull()
            expect(clearQueue).toHaveBeenCalled()
            expect(explicitLogout.value).toBe(true)
            expect(sessionLostUnexpectedly.value).toBe(false)
            expect(auth.needsApiKey.value).toBe(true)
        })

        // The key is forgotten here first and on disk second, so a player that
        // cannot delete the file leaves the user signed out all the same.
        it('signs out locally even when the player cannot delete the key', async () => {
            apiKey.value = 'aether_k3y'
            restAnswers(userEnvelope('alice', false))
            const auth = withAuth()
            await flushPromises()
            expect(subsonicClient.hasApiKey()).toBe(true)
            // As the real clearApiKey: the ref goes first, whatever Rust answers.
            host.clearApiKey.mockImplementation(() => {
                apiKey.value = null
                throw { code: 'Internal', message: 'could not delete the key: permission denied' }
            })
            const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})

            await auth.logout.mutateAsync()
            await flushPromises()

            expect(host.clearApiKey).toHaveBeenCalledTimes(1)
            expect(apiKey.value).toBeNull()
            expect(subsonicClient.hasApiKey()).toBe(false)
            expect(subsonicReady.value).toBe(false)
            expect(auth.needsApiKey.value).toBe(true)
            // A short line: the error's own text could hold the key.
            expect(warn).toHaveBeenCalledWith('Could not forget the API key')
            warn.mockRestore()
        })

        it('switches server without an /api/v0 logout, forgetting the key and the kind', async () => {
            apiKey.value = 'aether_k3y'
            restAnswers(userEnvelope('alice', false))
            const auth = withAuth()
            await flushPromises()

            await auth.switchServer()

            expect(logout).not.toHaveBeenCalled()
            expect(host.clearServerUrl).toHaveBeenCalledTimes(1)
            expect(apiKey.value).toBeNull()
            expect(sessionKind.value).toBe('cookie')
            expect(lastServerUrl.value).toBe(SERVER)
            expect(auth.needsServer.value).toBe(true)
        })

        // The key card verified the key with getUser already; its answer seeds the
        // identity, so the app renders without asking a second time.
        it('seeds the identity a verified key answered and renders without a second getUser', async () => {
            apiKey.value = null
            const auth = withAuth()
            await flushPromises()
            const fetchMock = vi.fn()
            vi.stubGlobal('fetch', fetchMock)

            auth.apiKeySaved({ login: 'alice', role: 'user' })
            apiKey.value = 'aether_k3y'
            await flushPromises()

            expect(auth.currentUser.value).toEqual({ login: 'alice', role: 'user' })
            expect(auth.needsApiKey.value).toBe(false)
            expect(subsonicReady.value).toBe(true)
            expect(fetchMock).not.toHaveBeenCalled()
        })

        // A refused key's expiry must not outlive the key that replaces it.
        it('starts afresh when a new key is saved after a refused one', async () => {
            apiKey.value = 'aether_revoked'
            restAnswers(refused)
            const auth = withAuth()
            await flushPromises()
            expect(auth.needsApiKey.value).toBe(true)

            auth.apiKeySaved({ login: 'alice', role: 'user' })
            apiKey.value = 'aether_fresh'
            await flushPromises()

            expect(sessionExpired.value).toBe(false)
            expect(auth.needsApiKey.value).toBe(false)
            expect(subsonicReady.value).toBe(true)
        })

        // The key card retrying a revoked key: its seed ends the expiry before the
        // player has answered set_api_key, so the gate looks at a key session whose
        // key is still forgotten. That session has no key to lose, and the reset for
        // one lost would withdraw the Rust client the stored key is about to connect.
        it('keeps the Rust client when a revoked key is retried', async () => {
            apiKey.value = 'aether_revoked'
            restAnswers(refused)
            const auth = withAuth()
            await flushPromises()
            expect(sessionExpired.value).toBe(true)
            expect(apiKey.value).toBeNull()
            expect(auth.needsApiKey.value).toBe(true)
            const { setApiKey } = await import('@/lib/host')
            // The seed stands for the identity: nothing asks the server again.
            const fetchMock = vi.fn()
            vi.stubGlobal('fetch', fetchMock)
            // A difference, not a total: the spy is shared with the credential
            // watchers earlier cases left running.
            const withdrawals = host.clearSession.mock.calls.length

            auth.apiKeySaved({ login: 'alice', role: 'user' })
            // Rust answers a round trip later: the gate is evaluated in between.
            await flushPromises()
            await setApiKey('aether_k2')
            await flushPromises()

            expect(host.clearSession).toHaveBeenCalledTimes(withdrawals)
            expect(subsonicReady.value).toBe(true)
            expect(subsonicClient.hasApiKey()).toBe(true)
            expect(auth.needsApiKey.value).toBe(false)
            expect(auth.currentUser.value).toEqual({ login: 'alice', role: 'user' })
            expect(fetchMock).not.toHaveBeenCalled()
        })

        // The gate renders the app as soon as no card is due, so the very first
        // render must already know the identity is in flight. Seen as not loading,
        // the app would mount for a tick — and start the queue sync — only to be
        // torn down again once getUser is asked.
        it('is loading from the first render while getUser is pending', async () => {
            apiKey.value = 'aether_k3y'
            vi.stubGlobal(
                'fetch',
                vi.fn(() => new Promise(() => {}))
            )
            const seen: boolean[] = []
            mountAuth((auth) => {
                seen.push(auth.isLoading.value)
            })
            await flushPromises()

            expect(seen[0]).toBe(true)
        })

        // A favorite toggle invalidates every ['subsonic', …] query. The identity
        // is the session's own, asked once: it lives outside that prefix, so a
        // toggle neither asks getUser again nor risks a failed ask.
        it('is not asked again when the subsonic queries are invalidated', async () => {
            apiKey.value = 'aether_k3y'
            restAnswers(userEnvelope('alice', false))
            const fetchMock = vi.mocked(fetch)
            const { auth, qc } = mountAuth()
            await flushPromises()
            expect(fetchMock).toHaveBeenCalledTimes(1)

            fetchMock.mockRejectedValue('error sending request')
            await qc.invalidateQueries({ queryKey: ['subsonic'] })
            await flushPromises()

            expect(fetchMock).toHaveBeenCalledTimes(1)
            expect(auth.needsServer.value).toBe(false)
            expect(auth.currentUser.value).toEqual({ login: 'alice', role: 'user' })
        })

        // Only a bootstrap that failed sends the user to the server step: an ask
        // that fails once the identity is known leaves the running session be.
        it('keeps a running session when asking the identity again fails', async () => {
            apiKey.value = 'aether_k3y'
            restAnswers(userEnvelope('alice', false))
            const fetchMock = vi.mocked(fetch)
            const { auth, qc } = mountAuth()
            await flushPromises()
            const { userQueryKeys } = await import('@/composables/useUsers')

            fetchMock.mockRejectedValue('error sending request')
            await qc.invalidateQueries({ queryKey: userQueryKeys.restIdentity })
            await flushPromises()

            expect(fetchMock).toHaveBeenCalledTimes(2)
            expect(auth.needsServer.value).toBe(false)
            expect(auth.currentUser.value).toEqual({ login: 'alice', role: 'user' })
        })

        // Moving on from a key session to a cookie server through the server step:
        // the host forgets the key and flips the kind, and the /rest client must
        // let go of the old key and address with them. The cookie flow takes a key
        // on the client for one it minted, so a stale one would never be replaced.
        it('mints for a cookie server chosen after a key session', async () => {
            apiKey.value = 'aether_k3y'
            const unreachable = `error sending request for url (${SERVER}/rest/getUser.view)`
            vi.stubGlobal('fetch', vi.fn().mockRejectedValue(unreachable))
            const auth = withAuth()
            await flushPromises()
            expect(auth.needsServer.value).toBe(true)
            // Counts what the cookie session puts on the client, not the key above.
            // The mint itself is counted on the client rather than on mintSpaToken:
            // that mock is shared with the credential watchers earlier cases left
            // running, which the flip below wakes too.
            const installKey = vi.spyOn(subsonicClient, 'setApiKey')

            // As setServerUrl(url, 'cookie') leaves the host's refs.
            apiKey.value = null
            sessionKind.value = 'cookie'
            serverUrl.value = 'http://new.lan:8075'
            getMe.mockResolvedValue(meAlice)
            auth.serverSaved()
            await flushPromises()

            expect(installKey).toHaveBeenCalledTimes(1)
            expect(subsonicClient.hasApiKey()).toBe(true)
            expect(subsonicClient.getStreamUrl('s1')).toMatch(/^http:\/\/new\.lan:8075\//)
        })

        // The server step saving a different server for another key session: Rust
        // drops the key for a server it was not stored for, so the host's key goes
        // to null while the kind stays. The client must forget the old key and
        // address with it. Left configured, with subsonicReady still true, the
        // identity query stays enabled and the reset in serverSaved() asks the old
        // server with the old key, which brings the server step back where the key
        // card is due.
        it('forgets the stale client when a different server is saved without a key', async () => {
            apiKey.value = 'aether_k3y'
            const unreachable = `error sending request for url (${SERVER}/rest/getUser.view)`
            const fetchMock = vi.fn().mockRejectedValue(unreachable)
            vi.stubGlobal('fetch', fetchMock)
            const auth = withAuth()
            await flushPromises()
            expect(auth.needsServer.value).toBe(true)
            expect(fetchMock).toHaveBeenCalledTimes(1)

            // As setServerUrl(url, 'apiKey') leaves the host's refs for a different
            // server. The watchers flush before the login view calls serverSaved().
            apiKey.value = null
            serverUrl.value = 'http://portal.lan'
            await flushPromises()
            auth.serverSaved()
            await flushPromises()

            expect(fetchMock).toHaveBeenCalledTimes(1)
            expect(auth.needsServer.value).toBe(false)
            expect(auth.needsApiKey.value).toBe(true)
            expect(subsonicReady.value).toBe(false)
            expect(subsonicClient.hasApiKey()).toBe(false)
        })

        // The same server saved again after an outage: Rust keeps the key for it,
        // so the host's refs do not move and nothing resets the client. The retry
        // is serverSaved() alone, asking getUser again with the key kept.
        it('asks getUser again with the kept key when the same server is saved', async () => {
            apiKey.value = 'aether_k3y'
            const unreachable = `error sending request for url (${SERVER}/rest/getUser.view)`
            const fetchMock = vi.fn().mockRejectedValue(unreachable)
            vi.stubGlobal('fetch', fetchMock)
            const auth = withAuth()
            await flushPromises()
            expect(auth.needsServer.value).toBe(true)
            expect(fetchMock).toHaveBeenCalledTimes(1)

            // The server is back.
            fetchMock.mockResolvedValue({
                ok: true,
                json: () => Promise.resolve(userEnvelope('alice', false))
            })
            auth.serverSaved()
            await flushPromises()

            expect(fetchMock).toHaveBeenCalledTimes(2)
            expect(auth.needsServer.value).toBe(false)
            expect(auth.currentUser.value).toEqual({ login: 'alice', role: 'user' })
            expect(apiKey.value).toBe('aether_k3y')
            expect(subsonicReady.value).toBe(true)
            expect(subsonicClient.hasApiKey()).toBe(true)
        })
    })
})
