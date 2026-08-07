import { computed, effectScope, nextTick, watch } from 'vue'
import { useMutation, useQueryClient, type QueryClient } from '@tanstack/vue-query'
import * as AuthApi from '@/lib/api/Auth'
import { useMe, userQueryKeys } from '@/composables/useUsers'
import { usePlayer } from '@/composables/usePlayer'
import { useQueueSync } from '@/composables/useQueueSync'
import { explicitLogout, sessionExpired } from '@/lib/authState'
import {
    subsonicReady,
    spaTokenId,
    remintApiKey,
    resetSubsonicSession,
    mintRetriesExhausted,
    resetMintAttempts
} from '@/lib/subsonicSession'
import { subsonicClient } from '@/lib/api/subsonic'

// One purge per lost session. The purge's own resetQueries refetches active
// /api/v1 queries, which 401 while logged out and flip sessionExpired — on an
// explicit logout that transition would trigger the expiry watcher and purge
// a second time. Re-armed on login (sessionExpired flips back to false).
let purgedThisExpiry = false

// The local session is over — explicit logout or expiry — so nothing of the
// old user may survive on this device: playback stops, localStorage is
// emptied and every cached query is dropped.
async function purgeLocalSession(qc: QueryClient): Promise<void> {
    purgedThisExpiry = true
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
    localStorage.clear()
    resetSubsonicSession()
    // Reset (not clear) so active queries — /me above all — refetch
    // immediately and the login gate closes.
    await qc.resetQueries()
}

let expiryPurgeInstalled = false

/**
 * The SPA's login gate, driven by the /me bootstrap: with auth method "none"
 * nothing is ever required; with "native" the login view is shown until /me
 * reports an identity, and again when any /api/v1 call answers 401 (session
 * expired). See docs/agents/authentication.md (mode: builtin).
 */
export function useAuth() {
    const qc = useQueryClient()
    const me = useMe()

    // An expired session (the axios interceptor flipping sessionExpired on a
    // 401) must scrub the device exactly like an explicit logout. Installed
    // once, in a detached scope: useAuth() runs in many components and the
    // purge must not die with whichever one mounted first.
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
                void purgeLocalSession(qc)
            })

            // /rest credential lifecycle: with auth "none" the client is ready
            // as-is; in native mode a logged-in session mints the spa token
            // first. Runs again after login (invalidateQueries refetches /me).
            watch(
                [me.data, sessionExpired],
                async ([data, expired]) => {
                    if (!data || expired) return
                    if (data.authMethod !== 'native') {
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
                            const result = await remintApiKey()
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

    const authRequired = computed(() => me.data.value?.authMethod === 'native')
    const currentUser = computed(() => me.data.value?.user ?? null)

    // Gates the administration UI (the /settings area and the Admin menu
    // entry). With auth method "none" nothing is ever restricted, so every
    // visitor counts as admin; the backend enforces the same policy on
    // /api/v1. False while /me is still loading — admin affordances appear
    // rather than flash away.
    const isAdmin = computed(() => {
        if (!me.data.value) return false
        if (!authRequired.value) return true
        return currentUser.value?.role === 'admin'
    })

    // While /me is still loading nothing is known yet — callers keep showing
    // the boot state rather than flashing the login form at every visitor.
    const needsLogin = computed(() => {
        if (!authRequired.value) return false
        return currentUser.value === null || sessionExpired.value
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
            sessionExpired.value = false
            explicitLogout.value = false
            purgedThisExpiry = false
            // A fresh session gets a fresh mint budget.
            resetMintAttempts()
            // The cookie is set: refetch /me so the identity (and any queries
            // that 401ed while logged out) repopulate.
            await qc.invalidateQueries({ queryKey: userQueryKeys.me })
            await qc.invalidateQueries()
        }
    })

    const logoutMutation = useMutation({
        mutationFn: () => AuthApi.logout(spaTokenId.value ?? undefined),
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

    return {
        /** True while the /me bootstrap is in flight. */
        isLoading: computed(() => me.isLoading.value),
        /** Server requires login (auth method "native"). */
        authRequired,
        /** Identity from /me; null when anonymous. */
        currentUser,
        /** Current visitor may administer the server (see computed above). */
        isAdmin,
        /** Render the login view instead of the app. */
        needsLogin,
        login: loginMutation,
        logout: logoutMutation
    }
}
