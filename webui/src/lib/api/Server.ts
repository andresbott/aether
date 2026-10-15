// The login view's transport-level checks: the server step's probe (what answers
// at the address the user typed) and the key card's check (whether the saved
// server accepts a pasted API key). Both are built on httpFetch rather than on
// the clients: apiClient refuses to send without a server and its 401 handling
// would read these answers as a lost session, and the subsonic client's 40/44
// branch would read a wrong key as an expiry.
import { httpFetch } from '@/lib/host'
import { isMeResponse, NotAetherMeError } from '@/lib/api/meShape'
import type { MeResponse, MeUser } from '@/types/users'

/**
 * What answered at a server address, and where: `url` is the address as the
 * probe settled it, normalised and on https when the server redirected there —
 * the one to save. `aether`: its /me answered, so the app can connect (and sign
 * in, if the server asks). `portal`: Aether is there, but an auth portal in
 * front of it keeps /api/v0 closed to the app, so only an API key signs the app
 * in there.
 */
export type ProbeResult =
    | { kind: 'aether'; url: string; me: MeResponse }
    | { kind: 'portal'; url: string }

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

/** The server answered, with an error of its own (a 5xx): it is there, but not working. */
export class ServerErrorAnswerError extends Error {
    name = 'ServerErrorAnswerError'

    constructor(url: string) {
        super(`The server at ${url} answered with an error.`)
    }
}

/** The server refused the key (Subsonic error 40 or 44): wrong here, or revoked. */
export class ApiKeyRejectedError extends Error {
    name = 'ApiKeyRejectedError'

    constructor() {
        super('This API key was not accepted.')
    }
}

/** Aether answered an error of its own (any other code); its message is shown as written. */
export class AetherAnswerError extends Error {
    name = 'AetherAnswerError'
}

/**
 * Why the saved server at `url` did not let the app in, for the server step: read
 * off what its identity bootstrap (/me, or getUser in a key session) rejected
 * with. Something other than Aether answered; Aether, or a proxy in front of it,
 * answered with an error status; or nothing answered at all. Only these
 * sentences reach the screen, never the error's own text — a transport rejection
 * names the request URL, apiKey included.
 */
export function savedServerProblem(err: unknown, url: string): string {
    if (err instanceof NotAetherMeError) return new NotAetherServerError(url).message
    const status = (err as { response?: { status?: unknown } } | null)?.response?.status
    if (typeof status === 'number') return new ServerErrorAnswerError(url).message
    return new ServerUnreachableError(url).message
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

// A Subsonic ping as a client without credentials sends it.
const PING = '/rest/ping?f=json&v=1.16.1&c=aether-player'

/**
 * Asks the server at `address` (normalised first) who it is. Its /me answering
 * as Aether's does (`isMeResponse`, the check getMe applies too) is Aether. A
 * server that redirects /me from http to https on the same host is asked again
 * there, once. Any other answer may still be Aether behind an auth portal, which
 * redirects or refuses anonymous /api/v0 calls: every proxy setup leaves /rest
 * open for Subsonic clients, and there Aether answers a ping without credentials
 * with an error envelope (code 40) that names it (`type: "Aether"`) — another
 * Subsonic server's does not.
 *
 * Throws InvalidServerAddressError before any request, ServerUnreachableError
 * when a request fails or takes over ten seconds, NotAetherServerError when
 * neither answer is Aether's.
 */
export async function probeServer(address: string): Promise<ProbeResult> {
    return probeAt(normalizeServerUrl(address))
}

async function probeAt(url: string): Promise<ProbeResult> {
    const me = await probe(url, `${url}/api/v0/me`)
    if (me.ok && isMeResponse(me.body)) return { kind: 'aether', url, me: me.body }
    // Only an http address upgrades, so the https one asked instead never does:
    // one upgrade at most.
    const secure = httpsUpgrade(url, me)
    if (secure !== null) return probeAt(secure)
    // A server error is no portal's answer — a portal redirects or refuses an
    // anonymous call — and signing in needs /api/v0, whatever /rest says.
    if (me.status >= 500) throw new ServerErrorAnswerError(url)
    const ping = await probe(url, `${url}${PING}`)
    if (isAetherEnvelope(ping.body)) return { kind: 'portal', url }
    throw new NotAetherServerError(url)
}

// getUser as the key card asks it: no `username` — Aether answers the caller's
// own record whatever that parameter says — and the probe's client name.
const GET_USER = '/rest/getUser.view?f=json&v=1.16.1&c=aether-player'

/**
 * Asks the saved server who `apiKey` is, before anything stores it: one getUser
 * through the host transport, built like the probe (its timeout, no redirect
 * followed), so no unverified key ever touches the subsonic client, whose 40/44
 * branch would read a wrong key as an expiry. Resolves to the identity in the
 * shape /me gives.
 *
 * Throws ApiKeyRejectedError on error 40 or 44, AetherAnswerError with the
 * server's own message on another Aether error, NotAetherServerError when the
 * answer is no Aether envelope (a proxy's login page or redirect, an ok without
 * a user, another server), ServerUnreachableError when the request fails or
 * times out. No message carries the key.
 */
export async function verifyApiKey(url: string, apiKey: string): Promise<MeUser> {
    const target = new URL(`${url}${GET_USER}`)
    target.searchParams.set('apiKey', apiKey)
    const answer = await probe(url, target.toString())
    const envelope = subsonicEnvelope(answer.body)
    if (envelope === null || !isAetherEnvelope(answer.body)) throw new NotAetherServerError(url)
    if (envelope.status === 'ok') {
        const user = envelope.user
        if (isObject(user) && typeof user.username === 'string') {
            return { login: user.username, role: user.adminRole === true ? 'admin' : 'user' }
        }
        throw new NotAetherServerError(url)
    }
    const error = isObject(envelope.error) ? envelope.error : null
    if (error?.code === 40 || error?.code === 44) throw new ApiKeyRejectedError()
    if (typeof error?.message === 'string') throw new AetherAnswerError(error.message)
    throw new NotAetherServerError(url)
}

function subsonicEnvelope(body: unknown): Record<string, unknown> | null {
    if (!isObject(body)) return null
    const envelope = body['subsonic-response']
    return isObject(envelope) ? envelope : null
}

interface ProbeAnswer {
    ok: boolean
    status: number
    /** Where a redirect points; null without a Location header. */
    location: string | null
    /** The body parsed as JSON; undefined when it is not JSON. */
    body: unknown
}

// Each request — connecting, the answer, its body — gets this long in all.
// Without a bound, an address whose packets are dropped holds Connect until the
// system gives up on the connection, minutes later.
const PROBE_TIMEOUT_MS = 10_000

// RequestInit plus tauri-plugin-http's client options; a browser's fetch
// ignores the extra keys.
type ProbeRequestInit = RequestInit & { maxRedirections: number; connectTimeout: number }

async function probe(url: string, target: string): Promise<ProbeAnswer> {
    const deadline = timeoutSignal(PROBE_TIMEOUT_MS)
    let response: Response
    let text: string
    try {
        response = await httpFetch(target, probeInit(deadline.signal))
        text = await response.text()
    } catch {
        // Refused, unresolved, timed out: the transport's rejection names the
        // request URL, so only the address being probed is reported.
        throw new ServerUnreachableError(url)
    } finally {
        deadline.clear()
    }
    return {
        ok: response.ok,
        status: response.status,
        location: response.headers.get('Location'),
        body: parseJson(text)
    }
}

// A redirect is an answer to look at, not to follow: a portal sends anonymous
// callers to its login page, and a followed redirect would report what answers
// somewhere else. The plugin takes maxRedirections and hands the 3xx back;
// `redirect: 'manual'` is the standard spelling. Built per request, because the
// plugin deletes its options from the init it is given.
function probeInit(signal: AbortSignal): ProbeRequestInit {
    return {
        redirect: 'manual',
        signal,
        maxRedirections: 0,
        connectTimeout: PROBE_TIMEOUT_MS
    }
}

// AbortSignal.timeout where the webview has it; elsewhere a timer of our own,
// cleared once the request settles.
function timeoutSignal(ms: number): { signal: AbortSignal; clear: () => void } {
    if (typeof AbortSignal.timeout === 'function') {
        return { signal: AbortSignal.timeout(ms), clear: () => {} }
    }
    const controller = new AbortController()
    const timer = setTimeout(() => controller.abort(), ms)
    return { signal: controller.signal, clear: () => clearTimeout(timer) }
}

const REDIRECT_STATUSES = [301, 302, 307, 308]

/**
 * The https address to ask instead, when /me at an http address redirects to
 * https on the same host name (the port may change): the origin the redirect
 * names, with the typed path prefix. Null for any other answer — a portal on
 * another host above all.
 */
function httpsUpgrade(url: string, me: ProbeAnswer): string | null {
    if (!REDIRECT_STATUSES.includes(me.status) || me.location === null) return null
    const from = new URL(url)
    let to: URL
    try {
        to = new URL(me.location, `${url}/api/v0/me`)
    } catch {
        return null
    }
    if (from.protocol !== 'http:' || to.protocol !== 'https:') return null
    if (to.hostname !== from.hostname) return null
    return `${to.origin}${from.pathname}`.replace(/\/+$/, '')
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

// Any OpenSubsonic server (Navidrome, gonic, Airsonic…) answers a ping without
// credentials with an error envelope; only Aether's names it as its `type`, on
// every response, errors included. Compared without case.
function isAetherEnvelope(body: unknown): boolean {
    if (!isObject(body)) return false
    const envelope = body['subsonic-response']
    return (
        isObject(envelope) &&
        typeof envelope.type === 'string' &&
        envelope.type.toLowerCase() === 'aether'
    )
}
