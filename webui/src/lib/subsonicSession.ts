// The SPA's /rest credential lifecycle. The apiKey lives in memory only —
// never localStorage — and is re-minted transparently when it expires
// (docs/agents/authentication.md). Lives outside vue-query so the subsonic
// client and the player can reach it without an app instance.
import { ref } from 'vue'
import { subsonicClient } from '@/lib/api/subsonic'
import { mintSpaToken } from '@/lib/api/Tokens'
import { sessionExpired } from '@/lib/authState'

/** /rest calls may fire: auth "none", or a token is installed. App.vue gates on it. */
export const subsonicReady = ref(false)

/** The current spa token's id, handed to logout so the server revokes it. */
export const spaTokenId = ref<string | null>(null)

// Single-flight: a burst of 40s after wake must produce ONE mint, not one
// per in-flight query.
let mintInFlight: Promise<boolean> | null = null

/**
 * Mint a fresh spa token and install it on the subsonic client. Returns
 * false when the session itself is gone (mint 401 → sessionExpired, the
 * login gate takes over) or the mint failed for any other reason.
 */
export function remintApiKey(): Promise<boolean> {
    if (!mintInFlight) {
        mintInFlight = mintSpaToken()
            .then((r) => {
                subsonicClient.setApiKey(r.token)
                spaTokenId.value = r.tokenId
                return true
            })
            .catch((err: unknown) => {
                const status = (err as { response?: { status?: number } })?.response?.status
                if (status === 401) {
                    sessionExpired.value = true
                }
                return false
            })
            .finally(() => {
                mintInFlight = null
            })
    }
    return mintInFlight
}

/** Forget the device's /rest credential (logout or session expiry). */
export function resetSubsonicSession(): void {
    subsonicClient.clearApiKey()
    spaTokenId.value = null
    subsonicReady.value = false
}
