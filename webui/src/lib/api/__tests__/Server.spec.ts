import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import type { MeResponse } from '@/types/users'

// The probe runs before any server is set, straight on the host transport.
const httpFetch = vi.hoisted(() =>
    vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>()
)
vi.mock('@/lib/host', () => ({ httpFetch }))

import {
    AetherAnswerError,
    ApiKeyRejectedError,
    InvalidServerAddressError,
    NotAetherServerError,
    ServerErrorAnswerError,
    ServerUnreachableError,
    normalizeServerUrl,
    probeServer,
    savedServerProblem,
    verifyApiKey
} from '@/lib/api/Server'
import { NotAetherMeError } from '@/lib/api/meShape'

const SERVER = 'http://aether.local:8075'
const PING = `${SERVER}/rest/ping?f=json&v=1.16.1&c=aether-player`

const meNative: MeResponse = {
    authMethod: 'native',
    user: null,
    features: { userManagement: true }
}

// What Aether answers a ping without credentials: HTTP 200, error 40, and its
// name as the envelope's type (server/app/router/handlers/subsonic/response.go).
const pingEnvelope = {
    'subsonic-response': {
        status: 'failed',
        version: '1.16.1',
        type: 'Aether',
        openSubsonic: true,
        error: { code: 40, message: 'authentication required' }
    }
}

// Another OpenSubsonic server's answer to the same ping: an envelope too.
function otherServerEnvelope(type?: string) {
    return {
        'subsonic-response': {
            status: 'failed',
            version: '1.16.1',
            ...(type === undefined ? {} : { type }),
            error: { code: 40, message: 'Wrong username or password' }
        }
    }
}

function json(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' }
    })
}

function html(status = 200): Response {
    return new Response('<!doctype html><title>Sign in</title>', {
        status,
        headers: { 'Content-Type': 'text/html' }
    })
}

// A portal's answer to an anonymous call, handed back rather than followed.
function portalRedirect(): Response {
    return new Response(null, {
        status: 302,
        headers: { Location: 'https://auth.example.org/?rd=http%3A%2F%2Faether.local%3A8075' }
    })
}

/** Answers the probe's requests in order. */
function answer(...responses: Array<Response | (() => never)>) {
    for (const r of responses) {
        httpFetch.mockImplementationOnce(async () => (typeof r === 'function' ? r() : r))
    }
}

function requestedUrls(): string[] {
    return httpFetch.mock.calls.map(([input]) => String(input))
}

// What tauri-plugin-http rejects with when a request fails: a bare string that
// names the request URL.
const transportFailure = (): never => {
    throw `error sending request for url (${SERVER}/api/v0/me)`
}

beforeEach(() => {
    httpFetch.mockReset()
})

describe('probeServer', () => {
    it('recognises Aether by its /me answer, and asks nothing more', async () => {
        answer(json(meNative))

        await expect(probeServer(SERVER)).resolves.toEqual({
            kind: 'aether',
            url: SERVER,
            me: meNative
        })
        expect(requestedUrls()).toEqual([`${SERVER}/api/v0/me`])
    })

    // A portal (Authelia) redirects an anonymous /api/v0 call to its login page,
    // but leaves /rest open: the ping proves Aether is behind it.
    it('reports a portal when /me redirects and the ping answers an envelope', async () => {
        answer(portalRedirect(), json(pingEnvelope))

        await expect(probeServer(SERVER)).resolves.toEqual({ kind: 'portal', url: SERVER })
        expect(requestedUrls()).toEqual([`${SERVER}/api/v0/me`, PING])
    })

    it('reports a portal when /me is refused and the ping answers an envelope', async () => {
        answer(json({ error: 'unauthorized' }, 401), json(pingEnvelope))

        await expect(probeServer(SERVER)).resolves.toEqual({ kind: 'portal', url: SERVER })
    })

    // A server error is no portal's answer — a portal redirects anonymous callers
    // or refuses them — and signing in needs /api/v0, healthy /rest or not.
    // Saved as a portal, the server would ask for an API key it never needed.
    it("reports the server's error when /me fails, even with a healthy ping", async () => {
        answer(json({ title: 'Internal Server Error' }, 500), json(pingEnvelope))

        const probe = probeServer(SERVER)
        await expect(probe).rejects.toBeInstanceOf(ServerErrorAnswerError)
        await expect(probe).rejects.toThrow(`The server at ${SERVER} answered with an error.`)
    })

    it('reports no Aether when /me is missing and the ping is no envelope', async () => {
        answer(html(404), json({ error: 'not found' }, 404))

        const probe = probeServer(SERVER)
        await expect(probe).rejects.toBeInstanceOf(NotAetherServerError)
        await expect(probe).rejects.toThrow(`No Aether server answered at ${SERVER}.`)
    })

    // Any web server that answers every path with its own page.
    it('reports no Aether when /me and the ping both answer a page', async () => {
        answer(html(), html())

        await expect(probeServer(SERVER)).rejects.toBeInstanceOf(NotAetherServerError)
    })

    // Some other API answering JSON at that path is not Aether either.
    it('reports no Aether when /me answers JSON without a known auth method', async () => {
        answer(json({ authMethod: 'oauth' }), json({ status: 'ok' }))

        await expect(probeServer(SERVER)).rejects.toBeInstanceOf(NotAetherServerError)
    })

    // The probe holds /me to the shape the gate reads later, as getMe does: a
    // known auth method alone is not Aether's answer.
    it('reports no Aether when /me names a known auth method in another shape', async () => {
        answer(json({ authMethod: 'native', user: 'ada' }), html())

        await expect(probeServer(SERVER)).rejects.toBeInstanceOf(NotAetherServerError)
    })

    // Navidrome, gonic, Airsonic: no /api/v0/me, and an anonymous ping answered
    // with an error envelope that is not Aether's. Not a portal in front of
    // Aether — simply not Aether.
    it.each([
        ['names another server', otherServerEnvelope('navidrome')],
        ['names no server', otherServerEnvelope()]
    ])('reports no Aether when the ping envelope %s', async (_case, envelope) => {
        answer(html(404), json(envelope))

        const probe = probeServer(SERVER)
        await expect(probe).rejects.toBeInstanceOf(NotAetherServerError)
        await expect(probe).rejects.toThrow(`No Aether server answered at ${SERVER}.`)
    })

    it('reads the envelope type without regard to case', async () => {
        const lowercase = {
            'subsonic-response': { ...pingEnvelope['subsonic-response'], type: 'aether' }
        }
        answer(portalRedirect(), json(lowercase))

        await expect(probeServer(SERVER)).resolves.toEqual({ kind: 'portal', url: SERVER })
    })

    it('reports an unreachable server when the request fails, without its text', async () => {
        answer(transportFailure)

        const err = await probeServer(SERVER).catch((e: unknown) => e)

        expect(err).toBeInstanceOf(ServerUnreachableError)
        expect((err as Error).message).toBe(`Could not reach ${SERVER}.`)
        expect(requestedUrls()).toHaveLength(1)
    })

    it('reports an unreachable server when the ping fails', async () => {
        answer(portalRedirect(), transportFailure)

        await expect(probeServer(SERVER)).rejects.toBeInstanceOf(ServerUnreachableError)
    })

    // The player never stores a password, so one typed into the address must not
    // reach the network at all.
    it('refuses an address with credentials before sending anything', async () => {
        const err = await probeServer('http://ada:s3cret@aether.local:8075').catch(
            (e: unknown) => e
        )

        expect(err).toBeInstanceOf(InvalidServerAddressError)
        expect((err as Error).message).toBe(
            'Enter the server address without a username or password.'
        )
        expect(httpFetch).not.toHaveBeenCalled()
    })

    it('assumes http:// when no scheme is typed', async () => {
        answer(json(meNative))

        await probeServer('aether.local:8075')

        expect(requestedUrls()).toEqual(['http://aether.local:8075/api/v0/me'])
    })

    it('drops trailing slashes and keeps a path prefix', async () => {
        answer(json(meNative), json(meNative))

        await probeServer('http://aether.local:8075///')
        await probeServer('https://music.example.org/aether/')

        expect(requestedUrls()).toEqual([
            'http://aether.local:8075/api/v0/me',
            'https://music.example.org/aether/api/v0/me'
        ])
    })

    // A followed redirect would report what answers somewhere else; the plugin
    // hands a 3xx back only when told not to follow it. Each request is also
    // bounded on its own: ten seconds overall, ten to connect.
    it('asks the transport to hand redirects back and to give up in time', async () => {
        answer(portalRedirect(), json(pingEnvelope))

        await probeServer(SERVER)

        const inits = httpFetch.mock.calls.map(([, init]) => init)
        expect(inits).toHaveLength(2)
        for (const init of inits) {
            expect(init).toMatchObject({
                redirect: 'manual',
                maxRedirections: 0,
                connectTimeout: 10_000
            })
            expect(init?.signal).toBeInstanceOf(AbortSignal)
        }
        expect(inits[0]?.signal).not.toBe(inits[1]?.signal)
    })

    it('reports an aborted request as an unreachable server', async () => {
        httpFetch.mockRejectedValueOnce(
            new DOMException('The operation was aborted.', 'AbortError')
        )

        await expect(probeServer(SERVER)).rejects.toBeInstanceOf(ServerUnreachableError)
    })

    // tauri-plugin-http deletes its client options from the init it receives, so
    // a shared init would let every later request follow redirects.
    it('gives every request its own init', async () => {
        const sent: unknown[] = []
        httpFetch.mockImplementation(async (_input, init) => {
            const options = init as RequestInit & { maxRedirections?: number }
            sent.push(options.maxRedirections)
            delete options.maxRedirections
            return sent.length === 1 ? portalRedirect() : json(pingEnvelope)
        })

        await probeServer(SERVER)

        expect(sent).toEqual([0, 0])
    })
})

// An address whose packets are dropped never answers; without a bound, Connect
// would spin until the system gave up on the connection, minutes later.
describe('probeServer timeout', () => {
    // A request that only ends when its signal aborts, as fetch does.
    function neverAnswers() {
        httpFetch.mockImplementation(
            (_input, init) =>
                new Promise<Response>((_resolve, reject) => {
                    init?.signal?.addEventListener('abort', () =>
                        reject(new DOMException('The operation timed out.', 'TimeoutError'))
                    )
                })
        )
    }

    async function outcomeOverTime() {
        let outcome: unknown = 'pending'
        void probeServer(SERVER).catch((err: unknown) => {
            outcome = err
        })
        await vi.advanceTimersByTimeAsync(9_999)
        const before = outcome
        await vi.advanceTimersByTimeAsync(1)
        return { before, after: outcome }
    }

    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('gives up after ten seconds and reports the server unreachable', async () => {
        neverAnswers()

        const { before, after } = await outcomeOverTime()

        expect(before).toBe('pending')
        expect(after).toBeInstanceOf(ServerUnreachableError)
        expect((after as Error).message).toBe(`Could not reach ${SERVER}.`)
    })

    describe('where AbortSignal.timeout is missing', () => {
        const nativeTimeout = AbortSignal.timeout

        beforeEach(() => {
            ;(AbortSignal as { timeout?: unknown }).timeout = undefined
        })

        afterEach(() => {
            AbortSignal.timeout = nativeTimeout
        })

        it('gives up after ten seconds on its own timer', async () => {
            neverAnswers()

            const { before, after } = await outcomeOverTime()

            expect(before).toBe('pending')
            expect(after).toBeInstanceOf(ServerUnreachableError)
        })

        it('clears its timer once the server has answered', async () => {
            answer(json(meNative))

            await probeServer(SERVER)

            expect(vi.getTimerCount()).toBe(0)
        })
    })
})

// A server that redirects plain http to https is Aether all the same: it is
// asked again over https, and that is the address to save.
describe('probeServer https upgrade', () => {
    const HTTP = 'http://aether.example.org'
    const HTTPS = 'https://aether.example.org'
    const pingAt = (url: string) => `${url}/rest/ping?f=json&v=1.16.1&c=aether-player`

    function redirectTo(location: string, status = 301): Response {
        return new Response(null, { status, headers: { Location: location } })
    }

    it('asks again over https when /me redirects there on the same host', async () => {
        answer(redirectTo(`${HTTPS}/api/v0/me`), json(meNative))

        await expect(probeServer(HTTP)).resolves.toEqual({
            kind: 'aether',
            url: HTTPS,
            me: meNative
        })
        expect(requestedUrls()).toEqual([`${HTTP}/api/v0/me`, `${HTTPS}/api/v0/me`])
    })

    it.each([302, 307, 308])('upgrades on a %i too, keeping the path prefix', async (status) => {
        answer(redirectTo(`${HTTPS}/music/api/v0/me`, status), json(meNative))

        await expect(probeServer(`${HTTP}/music`)).resolves.toMatchObject({
            url: `${HTTPS}/music`
        })
    })

    // A portal on another host is the case the ping exists for.
    it('does not upgrade to another host: the ping decides', async () => {
        answer(redirectTo('https://auth.example.org/?rd=x', 302), json(pingEnvelope))

        await expect(probeServer(HTTP)).resolves.toEqual({ kind: 'portal', url: HTTP })
        expect(requestedUrls()).toEqual([`${HTTP}/api/v0/me`, pingAt(HTTP)])
    })

    it('does not upgrade a redirect that stays on http', async () => {
        answer(redirectTo(`${HTTP}/elsewhere/api/v0/me`), html(404))

        await expect(probeServer(HTTP)).rejects.toBeInstanceOf(NotAetherServerError)
        expect(requestedUrls()).toEqual([`${HTTP}/api/v0/me`, pingAt(HTTP)])
    })

    // A portal behind https on the same host: one upgrade, then the ping there.
    it('upgrades once, then reports what https answers', async () => {
        answer(
            redirectTo(`${HTTPS}/api/v0/me`),
            redirectTo(`${HTTPS}/auth/?rd=x`, 302),
            json(pingEnvelope)
        )

        await expect(probeServer(HTTP)).resolves.toEqual({ kind: 'portal', url: HTTPS })
        expect(requestedUrls()).toEqual([`${HTTP}/api/v0/me`, `${HTTPS}/api/v0/me`, pingAt(HTTPS)])
    })

    it('reports the https address when it cannot be reached', async () => {
        answer(redirectTo(`${HTTPS}/api/v0/me`), transportFailure)

        await expect(probeServer(HTTP)).rejects.toThrow(`Could not reach ${HTTPS}.`)
    })
})

// The server step, reopened because the saved server's identity bootstrap
// failed, says why — from what that bootstrap rejected with, and never in that
// error's own words (a transport rejection names the request URL, key included).
describe('savedServerProblem', () => {
    it.each([
        ['no answer (axios)', { message: 'Network Error', code: 'ERR_NETWORK', request: {} }],
        ['a bare transport rejection', `error sending request for url (${SERVER}/api/v0/me)`],
        ['a timeout', { message: 'timeout of 10000ms exceeded', code: 'ECONNABORTED', request: {} }]
    ])('reads %s as a server it could not reach', (_case, err) => {
        expect(savedServerProblem(err, SERVER)).toBe(`Could not reach ${SERVER}.`)
    })

    it.each([
        ['a server error', 503],
        ['a refusal', 401]
    ])('reads %s as a server that answered with an error', (_case, status) => {
        const err = { message: 'Request failed', response: { status, data: {} }, request: {} }

        expect(savedServerProblem(err, SERVER)).toBe(
            `The server at ${SERVER} answered with an error.`
        )
    })

    it('reads an answer that is not Aether’s /me as something else answering', () => {
        expect(savedServerProblem(new NotAetherMeError(), SERVER)).toBe(
            `No Aether server answered at ${SERVER}.`
        )
    })
})

describe('normalizeServerUrl', () => {
    function refusal(typed: string): unknown {
        try {
            normalizeServerUrl(typed)
        } catch (err) {
            return err
        }
        return undefined
    }

    it.each([
        ['aether.local:8075', 'http://aether.local:8075'],
        ['192.168.1.5:8075', 'http://192.168.1.5:8075'],
        ['  http://aether.local:8075/\n', 'http://aether.local:8075'],
        ['HTTPS://Music.Example.org/', 'https://music.example.org'],
        ['https://music.example.org:443/aether/', 'https://music.example.org/aether'],
        // Only the origin and a path prefix name a server.
        ['http://aether.local:8075/?token=x#top', 'http://aether.local:8075']
    ])('reads %j as %j', (typed, normalised) => {
        expect(normalizeServerUrl(typed)).toBe(normalised)
    })

    // Where URL parsing rewrites what was typed. The gate compares this form with
    // the one the player's Rust side saves (normalize_server_url) to tell whether
    // Connect saved the same server again, which keeps a stored key: both must
    // rewrite these the same way. client.rs pins the same table.
    it.each([
        ['http://h:80', 'http://h'],
        ['https://h:443/aether/', 'https://h/aether'],
        // 443 is https's default, not http's
        ['http://h:443', 'http://h:443'],
        // scheme and host are case-insensitive, a path prefix is not
        ['HTTP://Music.Example.COM:8075/Aether', 'http://music.example.com:8075/Aether'],
        ['http://[::1]:8075/', 'http://[::1]:8075'],
        ['http://[FE80:0:0:0:0:0:0:1]/', 'http://[fe80::1]']
    ])('rewrites %j as the player does, to %j', (typed, normalised) => {
        expect(normalizeServerUrl(typed)).toBe(normalised)
    })

    it.each([
        '',
        'http://',
        'ftp://aether.local/music',
        'aether local',
        'http://aether.local:99999'
    ])('refuses %j', (typed) => {
        const err = refusal(typed)
        expect(err).toBeInstanceOf(InvalidServerAddressError)
        expect((err as Error).message).toBe(
            'Enter a server address such as http://aether.local:8075.'
        )
    })

    it.each(['http://ada:s3cret@aether.local', 'ada@aether.local:8075', 'ada:s3cret@aether.local'])(
        'refuses credentials in %j without echoing them',
        (typed) => {
            const err = refusal(typed)
            expect(err).toBeInstanceOf(InvalidServerAddressError)
            expect((err as Error).message).toBe(
                'Enter the server address without a username or password.'
            )
        }
    )
})

// The key card's check before anything stores a key: one getUser with it.
describe('verifyApiKey', () => {
    const GET_USER = `${SERVER}/rest/getUser.view?f=json&v=1.16.1&c=aether-player&apiKey=aether_k3y`
    const userEnvelope = (username: string, adminRole: boolean) => ({
        'subsonic-response': {
            status: 'ok',
            version: '1.16.1',
            type: 'Aether',
            openSubsonic: true,
            user: { username, adminRole, settingsRole: adminRole, streamRole: true }
        }
    })
    const errorEnvelope = (code: number, message: string) => ({
        'subsonic-response': {
            status: 'failed',
            version: '1.16.1',
            type: 'Aether',
            error: { code, message }
        }
    })

    it('asks getUser with the key and no username, and answers the identity', async () => {
        answer(json(userEnvelope('alice', true)))

        await expect(verifyApiKey(SERVER, 'aether_k3y')).resolves.toEqual({
            login: 'alice',
            role: 'admin'
        })
        expect(requestedUrls()).toEqual([GET_USER])
    })

    it('reads a regular user off adminRole', async () => {
        answer(json(userEnvelope('bob', false)))

        await expect(verifyApiKey(SERVER, 'aether_k3y')).resolves.toEqual({
            login: 'bob',
            role: 'user'
        })
    })

    // 40 (credentials refused) and 44 (invalid apiKey) both mean the key is no
    // good here — wrong, or revoked. The message never carries it.
    it.each([40, 44])('reports a refused key on error %i, without the key', async (code) => {
        answer(json(errorEnvelope(code, 'Invalid API key')))

        const err = await verifyApiKey(SERVER, 'aether_k3y').catch((e: unknown) => e)

        expect(err).toBeInstanceOf(ApiKeyRejectedError)
        expect((err as Error).message).toBe('This API key was not accepted.')
        expect((err as Error).message).not.toContain('aether_k3y')
    })

    it("relays Aether's own message for another error", async () => {
        answer(json(errorEnvelope(0, 'internal error')))

        const err = await verifyApiKey(SERVER, 'aether_k3y').catch((e: unknown) => e)

        expect(err).toBeInstanceOf(AetherAnswerError)
        expect((err as Error).message).toBe('internal error')
    })

    // A proxy that does not bypass /rest answers its login page or a redirect; an
    // ok without a user is no Aether getUser; another server's envelope is not
    // Aether. None of them is a key to store.
    it.each([
        ['a proxy redirect', () => portalRedirect()],
        ['a login page', () => html()],
        [
            'an ok envelope without a user',
            () => json({ 'subsonic-response': { status: 'ok', version: '1.16.1', type: 'Aether' } })
        ],
        ["another server's envelope", () => json(otherServerEnvelope('navidrome'))]
    ])('reports no Aether on %s', async (_case, response) => {
        answer(response())

        await expect(verifyApiKey(SERVER, 'aether_k3y')).rejects.toBeInstanceOf(
            NotAetherServerError
        )
    })

    it('reports an unreachable server when the request fails, without its text', async () => {
        answer(transportFailure)

        const err = await verifyApiKey(SERVER, 'aether_k3y').catch((e: unknown) => e)

        expect(err).toBeInstanceOf(ServerUnreachableError)
        expect((err as Error).message).toBe(`Could not reach ${SERVER}.`)
    })

    it('asks the transport to hand redirects back and to give up in time', async () => {
        answer(json(userEnvelope('alice', false)))

        await verifyApiKey(SERVER, 'aether_k3y')

        const [, init] = httpFetch.mock.calls[0]
        expect(init).toMatchObject({
            redirect: 'manual',
            maxRedirections: 0,
            connectTimeout: 10_000
        })
        expect(init?.signal).toBeInstanceOf(AbortSignal)
    })
})
