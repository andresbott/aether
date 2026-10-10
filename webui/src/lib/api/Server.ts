// The desktop app's server step: what answers at the address the user typed.
// It runs before any server is configured, so it talks through the host
// transport directly, never through apiClient — that client refuses to send
// without a server, and its 401 handling would read the probe's answers as a
// lost session.
import { httpFetch } from '@/lib/host'
import type { AuthMethod, MeResponse } from '@/types/users'

/**
 * What answered at a server address. `aether`: its /me answered, so the app can
 * connect (and sign in, if the server asks). `portal`: Aether is there, but an
 * auth portal in front of it keeps /api/v0 closed to the app, which cannot sign
 * in through a portal yet.
 */
export type ProbeResult = { kind: 'aether'; me: MeResponse } | { kind: 'portal' }

/** The typed text does not name a server the app may connect to. */
export class InvalidServerAddressError extends Error {
    name = 'InvalidServerAddressError'
}

/** Nothing answered at the address: wrong host or port, server down, no network. */
export class ServerUnreachableError extends Error {
    name = 'ServerUnreachableError'

    constructor(url: string) {
        super(`Could not reach ${url}.`)
    }
}

/** Something answered at the address, but not Aether. */
export class NotAetherServerError extends Error {
    name = 'NotAetherServerError'

    constructor(url: string) {
        super(`No Aether server answered at ${url}.`)
    }
}

const INVALID_ADDRESS = 'Enter a server address such as http://aether.local:8075.'
const CREDENTIALS_IN_ADDRESS = 'Enter the server address without a username or password.'

const ANY_SCHEME = /^[a-z][a-z\d+.-]*:\/\//i
const HTTP_SCHEME = /^https?:\/\//i

/**
 * The canonical form of a typed server address, the form the player's Rust side
 * saves: `http://` is assumed when no scheme is typed (`new URL('host:8075')`
 * would read `host` as the scheme), the origin and a path prefix are kept, and a
 * query, a fragment and trailing slashes are dropped.
 *
 * Throws InvalidServerAddressError for text that names no http(s) server, and
 * for an address carrying a username or password: the app never sends a typed
 * password anywhere, nor shows it back.
 */
export function normalizeServerUrl(address: string): string {
    const text = address.trim()
    if (ANY_SCHEME.test(text) && !HTTP_SCHEME.test(text)) {
        throw new InvalidServerAddressError(INVALID_ADDRESS)
    }
    let url: URL
    try {
        url = new URL(HTTP_SCHEME.test(text) ? text : `http://${text}`)
    } catch {
        throw new InvalidServerAddressError(INVALID_ADDRESS)
    }
    if (url.username !== '' || url.password !== '') {
        throw new InvalidServerAddressError(CREDENTIALS_IN_ADDRESS)
    }
    return `${url.origin}${url.pathname}`.replace(/\/+$/, '')
}

const AUTH_METHODS: readonly AuthMethod[] = ['none', 'native', 'proxy-header']

// A Subsonic ping as a client without credentials sends it.
const PING = '/rest/ping?f=json&v=1.16.1&c=aether-player'

/**
 * Asks the server at `address` (normalised first) who it is. Its /me answering
 * with a known auth method is Aether. Any other answer may still be Aether behind
 * an auth portal, which redirects or refuses anonymous /api/v0 calls: every proxy
 * setup leaves /rest open for Subsonic clients, and there Aether answers a ping
 * without credentials with an error envelope (code 40).
 *
 * Throws InvalidServerAddressError before any request, ServerUnreachableError
 * when a request fails, NotAetherServerError when neither answer is Aether's.
 */
export async function probeServer(address: string): Promise<ProbeResult> {
    const url = normalizeServerUrl(address)
    const me = await probe(url, `${url}/api/v0/me`)
    if (me.ok && isMeResponse(me.body)) return { kind: 'aether', me: me.body }
    const ping = await probe(url, `${url}${PING}`)
    if (isSubsonicEnvelope(ping.body)) return { kind: 'portal' }
    throw new NotAetherServerError(url)
}

interface ProbeAnswer {
    ok: boolean
    /** The body parsed as JSON; undefined when it is not JSON. */
    body: unknown
}

// RequestInit plus tauri-plugin-http's client option; a browser's fetch ignores
// the extra key.
type ProbeRequestInit = RequestInit & { maxRedirections: number }

async function probe(url: string, target: string): Promise<ProbeAnswer> {
    let response: Response
    let text: string
    try {
        response = await httpFetch(target, noRedirects())
        text = await response.text()
    } catch {
        // The transport's rejection names the request URL; report only the
        // address the user typed.
        throw new ServerUnreachableError(url)
    }
    return { ok: response.ok, body: parseJson(text) }
}

// A redirect is an answer to look at, not to follow: a portal sends anonymous
// callers to its login page, and a followed redirect would report what answers
// somewhere else (an http-to-https redirect would pass for a portal). The plugin
// takes maxRedirections and hands the 3xx back; `redirect: 'manual'` is the
// standard spelling. Built per request, because the plugin deletes its options
// from the init it is given.
function noRedirects(): ProbeRequestInit {
    return { redirect: 'manual', maxRedirections: 0 }
}

function parseJson(text: string): unknown {
    try {
        return JSON.parse(text)
    } catch {
        return undefined
    }
}

function isObject(value: unknown): value is Record<string, unknown> {
    return typeof value === 'object' && value !== null
}

function isMeResponse(body: unknown): body is MeResponse {
    return isObject(body) && AUTH_METHODS.includes(body.authMethod as AuthMethod)
}

function isSubsonicEnvelope(body: unknown): boolean {
    return isObject(body) && isObject(body['subsonic-response'])
}
