import { computed, effectScope, nextTick, watch } from 'vue'
import { useMutation, useQueryClient, type QueryClient } from '@tanstack/vue-query'
import * as AuthApi from '@/lib/api/Auth'
import { useMe, useRestIdentity, userQueryKeys } from '@/composables/useUsers'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSync } from '@/composables/useQueueSync'
import { explicitLogout, lastServerUrl, sessionExpired } from '@/lib/authState'
import { DEVICE_ID_KEY } from '@/lib/deviceIdentity'
import {
    apiKey,
    clearApiKey,
    clearServerUrl,
    isDesktopApp,
    serverUrl,
    sessionKind
} from '@/lib/host'
import {
    subsonicReady,
    spaTokenId,
    remintApiKey,
    resetSubsonicSession,
    mintRetriesExhausted,
    resetMintAttempts,
    sessionGeneration
} from '@/lib/subsonicSession'
import { subsonicClient } from '@/lib/api/subsonic'
import type { MeUser } from '@/types/users'

// One purge per lost session. The purge's own resetQueries refetches active
// /api/v0 queries, which 401 while logged out and flip sessionExpired — on an
// explicit logout that transition would trigger the expiry watcher and purge
// a second time. Re-armed on login (sessionExpired flips back to false).
let purgedThisExpiry = false

// Everything that describes one session on one server — whether it expired or
// was ended on purpose, the purge latch, the mint budget — starts over on a
// fresh login, a saved API key, a switch to another server, and again once the
// next server is saved.
function startFreshSession(): void {
    sessionExpired.value = false
    explicitLogout.value = false
    purgedThisExpiry = false
    resetMintAttempts()
}

// A key session's one credential, forgotten here first — the SPA is signed out
// whatever Rust answers — then on disk through the host. A refusal only warns,
// with a short line: never the error, whose text could hold the key.
async function forgetApiKey(): Promise<void> {
    try {
        await clearApiKey()
    } catch {
        console.warn('Could not forget the API key')
    }
}

// The local session is over — explicit logout, expiry or a server switch — so
// nothing of the old user may survive on this device: in a key session the key
// goes first, then playback stops, localStorage is emptied and every cached
// query is dropped.
async function purgeLocalSession(qc: QueryClient): Promise<void> {
    purgedThisExpiry = true
    if (sessionKind.value === 'apiKey') await forgetApiKey()
    // usePlayer()/useQueueSync() register watchers per call; a throwaway
    // scope keeps this call from leaking them past the purge.
    const scope = effectScope(true)
    scope.run(() => {
        // Unbind the queue sync BEFORE emptying the queue: its debounced save
        // would otherwise push the emptied queue to the server, clobbering the
        // state another device could still resume from. PlayerLayout re-arms
        // it on the next login.
        useQueueSync().stop()
        usePlayer().clearQueue()
    })
    // The player persists its (now empty) state on the next tick; let that
    // flush first so localStorage ends the purge actually empty.
    await nextTick()
    scope.stop()
    // The device id identifies this APP INSTANCE, not the user: wiping it would
    // make the next login mint under a fresh identity, orphaning this instance's
    // server-side session and stacking a dead one per logout/login cycle.
    const deviceId = localStorage.getItem(DEVICE_ID_KEY)
    localStorage.clear()
    if (deviceId !== null) localStorage.setItem(DEVICE_ID_KEY, deviceId)
    resetSubsonicSession()
    // Reset (not clear) so active queries — /me above all — refetch
    // immediately and the login gate closes.
    await qc.resetQueries()
}

let expiryPurgeInstalled = false

/**
 * The SPA's login gate. Two session kinds (lib/host): in a cookie session the
 * /me bootstrap decides — with auth method "none" nothing is ever required;
 * with "native" the login view is shown until /me reports an identity, and
 * again when any /api/v0 call answers 401 (session expired); with
 * "proxy-header" the reverse proxy owns login entirely — no login view, and a
 * 401 triggers a full reload so the proxy's portal redirect kicks in. In a key
 * session (desktop app only) the identity comes from getUser over /rest, no
 * /api/v0 call is ever made, and a /rest error 40/44 means the key was revoked.
 * See docs/agents/authentication.md.
 */
export function useAuth() {
    const qc = useQueryClient()
    const me = useMe()
    const apiKeySession = computed(() => sessionKind.value === 'apiKey')

    // An expired session (the axios interceptor flipping sessionExpired on a
    // 401, or a refused key on /rest) must scrub the device exactly like an
    // explicit logout. Installed once, in a detached scope: useAuth() runs in
    // many components and the purge must not die with whichever one mounted
    // first.
    if (!expiryPurgeInstalled) {
        expiryPurgeInstalled = true
        const scope = effectScope(true)
        scope.run(() => {
            watch(sessionExpired, (expired) => {
                if (!expired) {
                    purgedThisExpiry = false
                    return
                }
                if (purgedThisExpiry) return
                // In proxy mode the session belongs to the proxy: there is no
                // login view to fall back to, so a lost session reloads the
                // page and the proxy's portal redirect takes over. (Configure
                // the proxy to answer non-HTML requests with 401, not 302.)
                if (!apiKeySession.value && me.data.value?.authMethod === 'proxy-header') {
                    purgedThisExpiry = true
                    window.location.reload()
                    return
                }
                void purgeLocalSession(qc)
            })

            // /rest credential lifecycle. Key session: the stored key is the
            // credential and Rust already holds it — install it, no mint, no
            // hand-off; without one the gate shows the key card. Cookie
            // session: with auth "none" the client is ready as-is; in the
            // authenticated modes (native, proxy-header) an established
            // identity mints the spa token first. Runs again after login
            // (invalidateQueries refetches /me) and after a key is saved.
            watch(
                [me.data, sessionExpired, sessionKind, apiKey],
                async ([data, expired, kind, key], [, , previousKind]) => {
                    // A key session that is left — the server step saved a cookie
                    // server, or the kind was switched — or that loses its key —
                    // a different portal server was saved, which Rust drops the key
                    // for, or the key was forgotten — must not carry its client
                    // over: it still holds the key and the address of the server
                    // left. The cookie branch below takes a key on the client for
                    // one it minted, so it would never mint for the new server; and
                    // with subsonicReady still true the identity query stays enabled
                    // and asks the old server with the old key.
                    if (previousKind === 'apiKey' && (kind !== 'apiKey' || key === null)) {
                        resetSubsonicSession()
                    }
                    if (kind === 'apiKey') {
                        if (expired || key === null) return
                        subsonicClient.setApiKey(key)
                        subsonicReady.value = true
                        return
                    }
                    if (!data || expired) return
                    if (data.authMethod !== 'native' && data.authMethod !== 'proxy-header') {
                        subsonicClient.initWithDefaults()
                        subsonicReady.value = true
                        return
                    }
                    if (data.user) {
                        if (!subsonicClient.hasApiKey()) {
                            // Give up once the retry budget is spent: the login
                            // gate stays up (sessionExpired is still true from
                            // the last failure) instead of minting forever.
                            if (mintRetriesExhausted()) return
                            const generation = sessionGeneration()
                            const result = await remintApiKey()
                            // A logout, an expiry or a server switch reset the
                            // session while the mint was out, and its outcome
                            // speaks for that session only: acted on here, it
                            // would expire or unready whatever runs now (a
                            // server chosen since, without login, would never
                            // render). That one has watcher runs of its own.
                            if (sessionGeneration() !== generation) return
                            // A boot-mint failure for any non-401 reason (409
                            // ErrTooManyTokens, 500, network blip) would leave
                            // subsonicReady=false and sessionExpired=false,
                            // rendering a blank screen. Simplest recoverable state:
                            // show the login gate; a re-login re-runs the mint.
                            //
                            // That gate purges and refetches /me, which re-runs
                            // this watcher — a persistent failure would loop, so
                            // remintApiKey counts consecutive failures and
                            // mintRetriesExhausted() above stops after N.
                            if (result === 'failed') {
                                sessionExpired.value = true
                                return
                            }
                        }
                        subsonicReady.value = subsonicClient.hasApiKey()
                    }
                },
                { immediate: true }
            )
        })
    }

    // Created after the credential watcher, whose first run installs a stored key
    // and flips subsonicReady: a query created before it is built disabled, and
    // vue-query applies the change only on the next flush, so the gate's first
    // render would see "not loading, nothing to ask" and mount the app for a tick
    // before getUser is asked. Created here it is enabled from the start, and
    // loading from the first render. Neither watcher reads it.
    const identity = useRestIdentity()

    // Native mode is the only one with aether-owned login/logout: the login
    // view's password form keys on this. Proxy-header authenticates too, but
    // its session lives at the proxy; a key session has no password at all.
    const authRequired = computed(
        () => !apiKeySession.value && me.data.value?.authMethod === 'native'
    )
    const authenticated = computed(
        () =>
            apiKeySession.value ||
            me.data.value?.authMethod === 'native' ||
            me.data.value?.authMethod === 'proxy-header'
    )
    // Identity: /me's user in a cookie session, getUser's record in a key one.
    const currentUser = computed<MeUser | null>(() =>
        apiKeySession.value ? (identity.data.value ?? null) : (me.data.value?.user ?? null)
    )

    // Whether /api/v0 can be used at all: a cookie session whose /me answered.
    // A key session is /rest only — its server may keep /api/v0 behind a
    // portal — so everything riding apiClient renders only when this is true,
    // whatever the role says.
    const managementApi = computed(() => !apiKeySession.value && me.data.value !== undefined)

    // Gates the admin affordances. With auth method "none" nothing is ever
    // restricted, so every visitor counts as admin; in the authenticated modes
    // the role decides — /me's in a cookie session, getUser's adminRole in a
    // key session, so an admin's key still edits covers and radio stations over
    // /rest. The backend enforces the same policy. False while the identity is
    // still loading — admin affordances appear rather than flash away.
    const isAdmin = computed(() => {
        if (apiKeySession.value) return identity.data.value?.role === 'admin'
        if (!me.data.value) return false
        if (!authenticated.value) return true
        return currentUser.value?.role === 'admin'
    })

    // What shows Log out: a native cookie session, or a key session (whose
    // logout forgets the key). Proxy-header and "none" have nothing to leave.
    const canSignOut = computed(() => authRequired.value || apiKeySession.value)

    // While /me is still loading nothing is known yet — callers keep showing
    // the boot state rather than flashing the login form at every visitor.
    const needsLogin = computed(() => {
        if (!authRequired.value) return false
        return currentUser.value === null || sessionExpired.value
    })

    // Inside the desktop app the server is the user's choice: until there is
    // one, there is no one to ask, and the login view asks for a server first.
    // It asks again when the chosen server does not answer — /me in a cookie
    // session, getUser in a key session, for any reason but a refused key,
    // which the key card handles — where the window would otherwise stay blank
    // with no way to pick another. In a key session only a bootstrap that failed
    // counts: once the identity is known, an ask that fails (a blip on a refresh)
    // must not replace the running app with the server step. Never in the
    // browser, where the server is the page's own origin.
    const needsServer = computed(() => {
        if (!isDesktopApp()) return false
        if (serverUrl.value === null) return true
        if (apiKeySession.value) {
            return (
                identity.isError.value && identity.data.value === undefined && !sessionExpired.value
            )
        }
        return me.isError.value
    })

    // The key card: a key session without a key (none pasted yet, or forgotten
    // after a logout or a refusal), and the one cookie case the app cannot
    // sign in to — a proxy-header /me without a user, a proxy that lets an
    // anonymous /me through while its portal keeps the login. In the browser
    // the proxy's own redirect handles that.
    const needsApiKey = computed(() => {
        if (!isDesktopApp()) return false
        if (apiKeySession.value) return apiKey.value === null || sessionExpired.value
        return me.data.value?.authMethod === 'proxy-header' && currentUser.value === null
    })

    const loginMutation = useMutation({
        mutationFn: ({
            username,
            password,
            rememberMe
        }: {
            username: string
            password: string
            rememberMe: boolean
        }) => AuthApi.login(username, password, rememberMe),
        onSuccess: async () => {
            // A fresh session, with a fresh mint budget.
            startFreshSession()
            // The cookie is set: refetch /me so the identity (and any queries
            // that 401ed while logged out) repopulate.
            await qc.invalidateQueries({ queryKey: userQueryKeys.me })
            await qc.invalidateQueries()
        }
    })

    const logoutMutation = useMutation({
        // A key session has no /api/v0 logout: the purge forgets the key (Rust
        // deletes its file) and that is the whole sign-out — revoking the key
        // itself stays in the web UI, where /api/v0 is reachable.
        mutationFn: () =>
            apiKeySession.value
                ? Promise.resolve()
                : AuthApi.logout(spaTokenId.value ?? undefined),
        // Mark the intent BEFORE the request: the purge's cache reset (and any
        // in-flight call) 401s and flips sessionExpired, and the login view
        // must not mistake that for an expiry the user did not ask for.
        onMutate: () => {
            explicitLogout.value = true
        },
        onSettled: async () => {
            // Whatever the server said, the local session is over.
            await purgeLocalSession(qc)
        }
    })

    /**
     * Signs out of the desktop app's server and forgets it, back to the login
     * view's server step. Never rejects: the step asks for a server either way.
     */
    async function switchServer(): Promise<void> {
        if (authRequired.value && currentUser.value !== null) {
            // Best-effort, and while the server is still set: the api client
            // refuses every request once it is forgotten. Clears the session
            // cookie in the app's jar and revokes the spa token. A key session
            // has nothing to call: its key goes with the purge below.
            try {
                await AuthApi.logout(spaTokenId.value ?? undefined)
            } catch {
                // The local session ends whatever the server answered.
            }
        }
        // The server step offers the address back, to correct rather than retype.
        if (serverUrl.value !== null) lastServerUrl.value = serverUrl.value
        // Before any cache reset: with no server the /me query is disabled, so
        // the reset cannot refetch it from the server being left.
        serverUrl.value = null
        // Signed in or not, nothing of this server may play on: its queue and
        // the queue sync would carry its track ids over to the next server.
        await purgeLocalSession(qc)
        // Nothing of this server's session may carry over: an expiry flag
        // left set would keep a next server without login from ever rendering.
        startFreshSession()
        try {
            // Also deletes a stored key and resets the kind to cookie.
            await clearServerUrl()
        } catch {
            // Only the saved setting is stale; the next server chosen replaces it.
            console.warn('Could not forget the saved server')
        }
    }

    /**
     * Desktop app: the login view's server step saved a server. Its session
     * starts afresh: whatever flagged the old one after the switch — a mint the
     * switch overtook answers 401 by itself — belongs to the server left behind,
     * and an expiry flag kept would stop a server without login from rendering.
     * Saving a first server enables /me by itself; saving over one whose /me
     * (or, in a key session, getUser) failed — even the same address — changes
     * nothing those queries watch, so their error is reset and the server just
     * saved is asked again.
     */
    function serverSaved(): void {
        lastServerUrl.value = null
        startFreshSession()
        if (me.isError.value) void qc.resetQueries({ queryKey: userQueryKeys.me })
        if (identity.isError.value) void qc.resetQueries({ queryKey: userQueryKeys.restIdentity })
    }

    /**
     * Desktop app: the login view's key card verified a key with getUser and is
     * about to store it. The session starts afresh — a refused key's expiry is
     * over — and the identity the verification answered seeds the rest identity
     * query. The seed must land before the key is stored: storing it installs
     * the key on the subsonic client and enables that query, which would then
     * ask getUser a second time instead of using the verified identity.
     */
    function apiKeySaved(user: MeUser): void {
        startFreshSession()
        qc.setQueryData(userQueryKeys.restIdentity, user)
    }

    return {
        /** True while the identity bootstrap (/me, or getUser in a key session) is in flight. */
        isLoading: computed(() =>
            apiKeySession.value ? identity.isLoading.value : me.isLoading.value
        ),
        /** Server requires login (auth method "native", cookie session). */
        authRequired,
        /** Identity from /me, or getUser in a key session; null when anonymous. */
        currentUser,
        /** Current visitor may administer the server (see computed above). */
        isAdmin,
        /** /api/v0 is usable: a cookie session whose /me answered. Gates everything that rides apiClient. */
        managementApi,
        /** Something to log out of: a native session or a key session. */
        canSignOut,
        /** Render the login view instead of the app. */
        needsLogin,
        /** Desktop app without a server, or its server did not answer: the login view asks for one. */
        needsServer,
        /** Desktop app: the login view asks for an API key. */
        needsApiKey,
        login: loginMutation,
        logout: logoutMutation,
        /** Desktop app: sign out of the server and choose another. */
        switchServer,
        /** Desktop app: the server step saved a server; its identity is asked. */
        serverSaved,
        /** Desktop app: the key card verified a key and is about to store it. */
        apiKeySaved
    }
}
