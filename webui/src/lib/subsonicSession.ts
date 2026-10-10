// The SPA's /rest credential lifecycle. The apiKey lives in memory only —
// never localStorage — and is re-minted transparently when it expires
// (docs/agents/authentication.md). Lives outside vue-query so the subsonic
// client and the player can reach it without an app instance.
import { ref } from 'vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { mintSpaToken } from '@/lib/api/Tokens'
import { sessionExpired } from '@/lib/authState'
import { clearSession, setSession } from '@/lib/host'

/** /rest calls may fire: auth "none", or a token is installed. App.vue gates on it. */
export const subsonicReady = ref(false)

/** The current spa token's id, handed to logout so the server revokes it. */
export const spaTokenId = ref<string | null>(null)

// Single-flight: a burst of 40s after wake must produce ONE mint, not one
// per in-flight query. The generation is bumped by resetSubsonicSession so a
// mint started before logout discards its result rather than re-installing a
// live credential the server never knew to revoke.
let mintInFlight: Promise<'ok' | 'session-gone' | 'failed'> | null = null
let mintGeneration = 0

type MintResult = 'ok' | 'session-gone' | 'failed'

/**
 * How many consecutive 'failed' mints are tolerated before the SPA stops
 * retrying. A boot-mint failure surfaces the login gate, whose purge refetches
 * /me and re-runs the mint watcher — with a persistent failure (a 500, a broken
 * proxy) that is a retry loop, so it is bounded here.
 */
export const MAX_MINT_ATTEMPTS = 3

// Consecutive 'failed' results. Deliberately NOT reset by
// resetSubsonicSession: the session purge runs on the very failure path this
// counter bounds, so resetting there would re-arm the loop it prevents. Only a
// successful mint or a fresh login clears it.
let consecutiveMintFailures = 0

/** True once the mint has failed MAX_MINT_ATTEMPTS times in a row. */
export function mintRetriesExhausted(): boolean {
    return consecutiveMintFailures >= MAX_MINT_ATTEMPTS
}

/** Re-arm the retry budget: a successful login starts a fresh session-epoch. */
export function resetMintAttempts(): void {
    consecutiveMintFailures = 0
}

/**
 * Mint a fresh spa token and install it on the subsonic client.
 * - 'ok': minted and installed
 * - 'session-gone': mint answered 401 (sessionExpired is flipped)
 * - 'failed': other failure (409 ErrTooManyTokens, 500, network blip)
 */
export function remintApiKey(): Promise<MintResult> {
    if (!mintInFlight) {
        const generation = mintGeneration
        mintInFlight = mintSpaToken()
            .then((r) => {
                // Discard the result if a logout happened mid-flight: the old
                // tokenId was revoked and the server never learns of this new one.
                if (generation !== mintGeneration) {
                    return 'failed' as const
                }
                subsonicClient.setApiKey(r.token)
                // Only past the generation check: the Rust side takes a bare key
                // for whatever server is set, so a key minted for a server the
                // user has since left must never reach it.
                void shareWithDesktopApp(r.token)
                spaTokenId.value = r.tokenId
                consecutiveMintFailures = 0
                return 'ok' as const
            })
            .catch((err: unknown) => {
                const status = (err as { response?: { status?: number } })?.response?.status
                if (status === 401) {
                    sessionExpired.value = true
                    return 'session-gone' as const
                }
                // Non-401 failures (409 ErrTooManyTokens, 500, network blip)
                // leave the session state alone — the watcher will decide
                // whether to surface the login gate or a retry affordance.
                consecutiveMintFailures++
                return 'failed' as const
            })
            .finally(() => {
                mintInFlight = null
            })
    }
    return mintInFlight
}

/**
 * Forget the device's /rest session (logout, session expiry, server switch):
 * the credential and the server the client addresses, so nothing reaches the
 * server left behind until the next session configures the client again.
 */
export function resetSubsonicSession(): void {
    subsonicClient.reset()
    void withdrawFromDesktopApp()
    spaTokenId.value = null
    subsonicReady.value = false
    mintGeneration++
}

// The desktop player's Rust client calls the same server with the SPA's key;
// setSession and clearSession are no-ops in the browser. A failure leaves only
// that client without the right credential, so the SPA carries on. The
// warnings stay short: never the key, nor an error whose text could hold one.

async function shareWithDesktopApp(apiKey: string): Promise<void> {
    try {
        await setSession(apiKey)
    } catch {
        console.warn('Could not hand the session to the desktop player')
    }
}

async function withdrawFromDesktopApp(): Promise<void> {
    try {
        await clearSession()
    } catch {
        console.warn('Could not clear the desktop player session')
    }
}
