// The SPA's /rest credential lifecycle. The apiKey lives in memory only —
// never localStorage — and in a cookie session is re-minted transparently when
// it expires (docs/agents/authentication.md); a key session has nothing to mint,
// its key being the one the user pasted. Lives outside vue-query so the subsonic
// client and the player can reach it without an app instance.
import { ref } from 'vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { mintSpaToken } from '@/lib/api/Tokens'
import { sessionExpired } from '@/lib/authState'
import { clearSession, sessionKind, setSession } from '@/lib/host'

/** /rest calls may fire: auth "none", or a token is installed. App.vue gates on it. */
export const subsonicReady = ref(false)

/** The current spa token's id, handed to logout so the server revokes it. */
export const spaTokenId = ref<string | null>(null)

// Single-flight: a burst of 40s after wake must produce ONE mint, not one
// per in-flight query. The session generation moves with resetSubsonicSession,
// so a mint started before logout discards its outcome rather than
// re-installing a live credential the server never knew to revoke — or flagging
// an expiry on a session that is already over. The reset also lets go of the
// mint still out, so the next session mints for itself instead of joining a
// discarded one.
let mintInFlight: Promise<'ok' | 'session-gone' | 'failed'> | null = null

type MintResult = 'ok' | 'session-gone' | 'failed'

/**
 * The current session's generation, bumped by every resetSubsonicSession — the
 * subsonic client's own count, which its requests check against too. Whoever
 * awaits a mint compares it before and after: when it moved, a logout, an
 * expiry or a server switch ended the session the mint was for, and its outcome
 * speaks for that session only.
 */
export function sessionGeneration(): number {
    return subsonicClient.generation()
}

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
 * - 'failed': other failure (409 ErrTooManyTokens, 500, network blip), or a
 *   mint whose session was reset while it was out, whatever it answered
 */
export function remintApiKey(): Promise<MintResult> {
    // A key session has nothing to mint and gives no verdict here: the player
    // calls this on a stream element error, which carries no reason, and
    // signing the user out on a network blip would be worse than the
    // connectivity banner it reports instead. No retry budget is spent. A
    // refused key reaches the gate through handleRestAuthFailure, from a JSON
    // answer that names its code.
    if (sessionKind.value === 'apiKey') return Promise.resolve('failed')
    if (!mintInFlight) {
        const generation = sessionGeneration()
        const mint: Promise<MintResult> = mintSpaToken()
            .then((r) => {
                // Discard the result if a logout happened mid-flight: the old
                // tokenId was revoked and the server never learns of this new one.
                if (generation !== sessionGeneration()) {
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
                // Discard a failure the same way: a 401 for a session already
                // reset (a server switch overtook the mint) must not flag the
                // one running now, nor any failure spend its retry budget.
                if (generation !== sessionGeneration()) {
                    return 'failed' as const
                }
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
                // A reset may have let go of this mint, and another may be out.
                if (mintInFlight === mint) mintInFlight = null
            })
        mintInFlight = mint
    }
    return mintInFlight
}

/**
 * A /rest answer carried error 40 or 44 while a key was installed. In the cookie
 * flow the spa token has expired: mint again, and the caller retries with the
 * fresh key. In a key session there is nothing to mint — the key the user
 * pasted was refused, revoked or expired (Aether answers 44 for an expired key
 * as for a revoked one) — so the session is over: the flag opens the gate
 * (useAuth forgets the key and purges), and the caller fails its request.
 */
export function handleRestAuthFailure(): Promise<MintResult> {
    if (sessionKind.value === 'apiKey') {
        sessionExpired.value = true
        return Promise.resolve('session-gone')
    }
    return remintApiKey()
}

/**
 * Forget the device's /rest session (logout, session expiry, server switch):
 * the credential and the server the client addresses, so nothing reaches the
 * server left behind until the next session configures the client again.
 */
export function resetSubsonicSession(): void {
    // Moves the session generation: whatever the old session still has out — a
    // mint, a /rest request — answers for it alone.
    subsonicClient.reset()
    void withdrawFromDesktopApp()
    spaTokenId.value = null
    subsonicReady.value = false
    // A mint still out belongs to the session just ended; it settles on its own
    // and its outcome is discarded.
    mintInFlight = null
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
