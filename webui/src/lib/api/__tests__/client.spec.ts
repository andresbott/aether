import { describe, it, expect, vi, afterEach } from 'vitest'
import axios, { AxiosError, type InternalAxiosRequestConfig } from 'axios'
import { ref } from 'vue'

const SERVER = 'http://music.lan:8075'

// A stand-in for @/lib/host: where the SPA runs, the server it talks to and the
// transport requests ride.
function fakeHost(desktopApp: boolean, server: string | null) {
    return {
        isDesktopApp: () => desktopApp,
        serverUrl: ref<string | null>(server),
        sessionKind: ref<'cookie' | 'apiKey'>('cookie'),
        httpFetch: vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>()
    }
}

// client.ts picks its transport when it is created, so each test builds a fresh
// one against its own host. authState is re-imported after the reset to get the
// instance this client writes to.
async function loadClient(host: ReturnType<typeof fakeHost>) {
    vi.resetModules()
    vi.doMock('@/lib/host', () => host)
    const { apiClient } = await import('@/lib/api/client')
    const { sessionExpired } = await import('@/lib/authState')
    return { apiClient, sessionExpired }
}

function jsonResponse(body: unknown, status = 200): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' }
    })
}

function requestsSent(host: ReturnType<typeof fakeHost>): Request[] {
    return host.httpFetch.mock.calls.map(([input]) => input as Request)
}

afterEach(() => {
    vi.doUnmock('@/lib/host')
})

describe('apiClient in the desktop app', () => {
    it('sends requests to the configured server through the host transport', async () => {
        const host = fakeHost(true, SERVER)
        host.httpFetch.mockResolvedValue(jsonResponse({ authMethod: 'native' }))
        const { apiClient } = await loadClient(host)

        const { data } = await apiClient.get('/me')

        expect(data).toEqual({ authMethod: 'native' })
        expect(requestsSent(host).map((r) => r.url)).toEqual([`${SERVER}/api/v0/me`])
    })

    // The login form posts through the same client, so its JSON body must
    // survive the trip through the transport.
    it('carries a JSON body through the transport', async () => {
        const host = fakeHost(true, SERVER)
        host.httpFetch.mockResolvedValue(jsonResponse({}))
        const { apiClient } = await loadClient(host)

        await apiClient.post('/auth/login', { username: 'ada', password: 'pw' })

        const [request] = requestsSent(host)
        expect(request.method).toBe('POST')
        expect(request.url).toBe(`${SERVER}/api/v0/auth/login`)
        expect(request.headers.get('Content-Type')).toContain('application/json')
        await expect(request.json()).resolves.toEqual({ username: 'ada', password: 'pw' })
    })

    // The user can change the server while the app runs; the client is created
    // once, so the base URL has to be read per request.
    it('follows a server change on the next request', async () => {
        const host = fakeHost(true, SERVER)
        host.httpFetch.mockImplementation(async () => jsonResponse({}))
        const { apiClient } = await loadClient(host)

        await apiClient.get('/me')
        host.serverUrl.value = 'https://aether.example.org'
        await apiClient.get('/me')

        expect(requestsSent(host).map((r) => r.url)).toEqual([
            `${SERVER}/api/v0/me`,
            'https://aether.example.org/api/v0/me'
        ])
    })

    // The plugin rejects a request that got no answer with a bare string naming
    // its URL. Through axios that must read as a connectivity failure — the
    // banner's cue — and its text must never become the message.
    it('turns a bare-string transport rejection into the network message', async () => {
        const host = fakeHost(true, SERVER)
        host.httpFetch.mockRejectedValue(`error sending request for url (${SERVER}/api/v0/me)`)
        const { apiClient } = await loadClient(host)
        const { apiErrorMessage, NETWORK_MESSAGE } = await import('@/lib/apiError')

        const err = await apiClient.get('/me').catch((e: unknown) => e)

        expect(apiErrorMessage(err)).toBe(NETWORK_MESSAGE)
    })

    // Nothing should request before the login view's server step has picked a
    // server; if something does, it must fail loudly instead of going nowhere.
    it('refuses to send a request while no server is configured', async () => {
        const host = fakeHost(true, null)
        const { apiClient } = await loadClient(host)

        await expect(apiClient.get('/me')).rejects.toThrow(/no aether server is configured/i)
        expect(host.httpFetch).not.toHaveBeenCalled()
    })

    // A key session is /rest only: the server may sit behind a portal that
    // answers /api/v0 with its login page. The UI hides every caller; a stray one
    // must fail here, fast, and never read the portal's answer as an expiry.
    it('refuses to send anything in a key session', async () => {
        const host = fakeHost(true, SERVER)
        host.sessionKind.value = 'apiKey'
        const { apiClient, sessionExpired } = await loadClient(host)

        await expect(apiClient.get('/me')).rejects.toThrow(/not available with an api key/i)

        expect(host.httpFetch).not.toHaveBeenCalled()
        expect(sessionExpired.value).toBe(false)
    })

    it('still flags the session as expired when the server answers 401', async () => {
        const host = fakeHost(true, SERVER)
        host.httpFetch.mockResolvedValue(jsonResponse({ title: 'Unauthorized' }, 401))
        const { apiClient, sessionExpired } = await loadClient(host)

        await expect(apiClient.get('/libraries')).rejects.toMatchObject({
            response: { status: 401 }
        })
        expect(sessionExpired.value).toBe(true)
    })

    // A request a server switch overtook (a poll sent while the logout was out)
    // answers 401 once the server is forgotten. With no server configured there
    // is no session to expire, and the flag would hold back the next server.
    it('flags no expiry for a 401 that lands once the server is forgotten', async () => {
        const host = fakeHost(true, SERVER)
        let answer!: (response: Response) => void
        host.httpFetch.mockReturnValue(
            new Promise<Response>((resolve) => {
                answer = resolve
            })
        )
        const { apiClient, sessionExpired } = await loadClient(host)

        const poll = apiClient.get('/tasks/executions')
        await vi.waitFor(() => expect(host.httpFetch).toHaveBeenCalled())
        host.serverUrl.value = null
        answer(jsonResponse({ title: 'Unauthorized' }, 401))

        await expect(poll).rejects.toMatchObject({ response: { status: 401 } })
        expect(sessionExpired.value).toBe(false)
    })

    // Once another server is chosen, a 401 a switch overtook still answers for
    // the server its request went to, and has no say over the new session.
    it('flags an expiry only for a 401 from the server configured now', async () => {
        const host = fakeHost(true, SERVER)
        let answer!: (response: Response) => void
        host.httpFetch.mockReturnValueOnce(
            new Promise<Response>((resolve) => {
                answer = resolve
            })
        )
        const { apiClient, sessionExpired } = await loadClient(host)

        const overtaken = apiClient.get('/tasks/executions')
        await vi.waitFor(() => expect(host.httpFetch).toHaveBeenCalled())
        host.serverUrl.value = 'https://aether.example.org'
        answer(jsonResponse({ title: 'Unauthorized' }, 401))

        await expect(overtaken).rejects.toMatchObject({ response: { status: 401 } })
        expect(sessionExpired.value).toBe(false)

        host.httpFetch.mockResolvedValueOnce(jsonResponse({ title: 'Unauthorized' }, 401))
        await expect(apiClient.get('/libraries')).rejects.toMatchObject({
            response: { status: 401 }
        })
        expect(sessionExpired.value).toBe(true)
    })
})

describe('apiClient in the browser', () => {
    it('keeps the relative /api/v0 base and axios\'s own adapter', async () => {
        const host = fakeHost(false, 'http://localhost:3000')
        const { apiClient } = await loadClient(host)
        let sent: InternalAxiosRequestConfig | undefined

        await apiClient.get('/me', {
            // Capture the request as the adapter would receive it.
            adapter: async (config) => {
                sent = config
                return { data: null, status: 200, statusText: 'OK', headers: {}, config }
            }
        })

        expect(sent?.baseURL).toBe('/api/v0')
        expect(apiClient.defaults.adapter).toEqual(axios.defaults.adapter)
        expect(host.httpFetch).not.toHaveBeenCalled()
    })

    // A tab's server is the page's own and never changes: every 401 speaks for
    // it, whatever base the request was sent with.
    it('flags the session as expired on a 401', async () => {
        const host = fakeHost(false, 'http://localhost:3000')
        const { apiClient, sessionExpired } = await loadClient(host)

        const request = apiClient.get('/libraries', {
            adapter: async (config) => {
                throw new AxiosError('Unauthorized', AxiosError.ERR_BAD_REQUEST, config, null, {
                    data: {},
                    status: 401,
                    statusText: 'Unauthorized',
                    headers: {},
                    config
                })
            }
        })

        await expect(request).rejects.toMatchObject({ response: { status: 401 } })
        expect(sessionExpired.value).toBe(true)
    })
})
