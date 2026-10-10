import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { MeResponse } from '@/types/users'

// The probe runs before any server is set, straight on the host transport.
const httpFetch = vi.hoisted(() =>
    vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>()
)
vi.mock('@/lib/host', () => ({ httpFetch }))

import {
    InvalidServerAddressError,
    NotAetherServerError,
    ServerUnreachableError,
    normalizeServerUrl,
    probeServer
} from '@/lib/api/Server'

const SERVER = 'http://aether.local:8075'
const PING = `${SERVER}/rest/ping?f=json&v=1.16.1&c=aether-player`

const meNative: MeResponse = {
    authMethod: 'native',
    user: null,
    features: { userManagement: true }
}

// What Aether answers a ping without credentials: HTTP 200, error 40.
const pingEnvelope = {
    'subsonic-response': {
        status: 'failed',
        version: '1.16.1',
        type: 'aether',
        openSubsonic: true,
        error: { code: 40, message: 'authentication required' }
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

        await expect(probeServer(SERVER)).resolves.toEqual({ kind: 'aether', me: meNative })
        expect(requestedUrls()).toEqual([`${SERVER}/api/v0/me`])
    })

    // A portal (Authelia) redirects an anonymous /api/v0 call to its login page,
    // but leaves /rest open: the ping proves Aether is behind it.
    it('reports a portal when /me redirects and the ping answers an envelope', async () => {
        answer(portalRedirect(), json(pingEnvelope))

        await expect(probeServer(SERVER)).resolves.toEqual({ kind: 'portal' })
        expect(requestedUrls()).toEqual([`${SERVER}/api/v0/me`, PING])
    })

    it('reports a portal when /me is refused and the ping answers an envelope', async () => {
        answer(json({ error: 'unauthorized' }, 401), json(pingEnvelope))

        await expect(probeServer(SERVER)).resolves.toEqual({ kind: 'portal' })
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
    // hands a 3xx back only when told not to follow it.
    it('asks the transport to hand redirects back, not follow them', async () => {
        answer(json(meNative))

        await probeServer(SERVER)

        expect(httpFetch.mock.calls[0][1]).toMatchObject({
            redirect: 'manual',
            maxRedirections: 0
        })
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
