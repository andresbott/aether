import { ref, type Ref } from 'vue'

/**
 * How the SPA signs in to the server. `cookie` is the native flow: a login
 * posted through the transport, the session cookie in the player's jar, a spa
 * token minted on /api/v0. `apiKey` is a /rest-only session on a key the user
 * pasted, which the player's Rust side stores; nothing is asked of /api/v0 then.
 * A browser tab is always `cookie`: the key session exists for the desktop app,
 * whose server may sit behind a portal that keeps /api/v0 closed to it.
 *
 * Set here, not only by initHost(): the gate and the api client read it from
 * the first render, in specs that never boot the host too.
 */
export type SessionKind = 'cookie' | 'apiKey'

export const sessionKind: Ref<SessionKind> = ref('cookie')

/**
 * The stored API key, in memory, for the subsonic client to sign /rest calls and
 * media URLs with — the one credential of a key session. Null in a browser tab,
 * in a cookie session, and in a key session that has no key yet or lost it.
 */
export const apiKey: Ref<string | null> = ref(null)
