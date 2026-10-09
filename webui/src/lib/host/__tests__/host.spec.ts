import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// The Tauri side, faked. `isTauri` decides browser vs app; `invoke` answers the
// player's IPC commands through `commands`, so each test states the Rust
// answers it relies on; the plugin's fetch stands in for the Rust transport.
const tauri = vi.hoisted(() => ({
    isTauri: vi.fn(() => false),
    invoke: vi.fn(),
    pluginFetch: vi.fn(),
    pluginLoaded: vi.fn()
}))

vi.mock('@tauri-apps/api/core', () => ({ isTauri: tauri.isTauri, invoke: tauri.invoke }))
vi.mock('@tauri-apps/plugin-http', () => {
    tauri.pluginLoaded()
    return { fetch: tauri.pluginFetch }
})

type Handler = (args?: Record<string, unknown>) => unknown

function commands(handlers: Record<string, Handler>): void {
    tauri.invoke.mockImplementation(async (cmd: string, args?: Record<string, unknown>) => {
        const handler = handlers[cmd]
        if (!handler) throw { code: 'Internal', message: `unexpected command ${cmd}` }
        return handler(args)
    })
}

// The host caches what it detected and the server ref is created on import, so
// every test boots a fresh copy of the module in the mode it needs.
async function loadHost(mode: 'browser' | 'app') {
    vi.resetModules()
    tauri.isTauri.mockReturnValue(mode === 'app')
    return import('@/lib/host')
}

const SERVER = 'http://music.lan:8075'

beforeEach(() => {
    tauri.invoke.mockReset()
    tauri.pluginFetch.mockReset()
    tauri.pluginLoaded.mockReset()
})

afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
})

describe('host in the browser', () => {
    it('is not the desktop app', async () => {
        const host = await loadHost('browser')
        expect(host.isDesktopApp()).toBe(false)
    })

    // The SPA is served by the server itself, so the server is the page's own
    // origin — before initHost too, since the subsonic client may read it first.
    it('uses the page origin as the server, before and after initHost', async () => {
        const host = await loadHost('browser')
        expect(host.serverUrl.value).toBe(window.location.origin)

        await host.initHost()

        expect(host.serverUrl.value).toBe(window.location.origin)
        expect(tauri.invoke).not.toHaveBeenCalled()
        expect(tauri.pluginLoaded).not.toHaveBeenCalled()
    })

    // Specs stub the global fetch after the host has loaded; the stub must still
    // answer, or every subsonic spec would hit the network.
    it('sends requests through the page fetch, resolved at call time', async () => {
        const host = await loadHost('browser')
        await host.initHost()
        const response = new Response('{}')
        const pageFetch = vi.fn().mockResolvedValue(response)
        vi.stubGlobal('fetch', pageFetch)
        const init = { method: 'POST' }

        await expect(host.httpFetch('/rest/ping.view', init)).resolves.toBe(response)
        expect(pageFetch).toHaveBeenCalledWith('/rest/ping.view', init)
    })

    it('treats the session hand-off as a no-op and has no app version', async () => {
        const host = await loadHost('browser')
        await expect(host.setSession('aether_key')).resolves.toBeUndefined()
        await expect(host.clearSession()).resolves.toBeUndefined()
        await expect(host.appVersion()).resolves.toBeNull()
        expect(tauri.invoke).not.toHaveBeenCalled()
    })

    it('refuses to change the server, which is the page origin', async () => {
        const host = await loadHost('browser')
        await expect(host.setServerUrl(SERVER)).rejects.toThrow(/desktop app/)
        await expect(host.clearServerUrl()).rejects.toThrow(/desktop app/)
        expect(host.serverUrl.value).toBe(window.location.origin)
        expect(tauri.invoke).not.toHaveBeenCalled()
    })
})

describe('host in the desktop app', () => {
    it('is the desktop app, with no server known until initHost reads it', async () => {
        const host = await loadHost('app')
        expect(host.isDesktopApp()).toBe(true)
        expect(host.serverUrl.value).toBeNull()
    })

    it('initHost adopts the saved server and routes requests through the plugin', async () => {
        commands({ get_settings: () => ({ serverUrl: SERVER }) })
        const host = await loadHost('app')
        const pageFetch = vi.fn()
        vi.stubGlobal('fetch', pageFetch)
        const response = new Response('{}')
        tauri.pluginFetch.mockResolvedValue(response)

        await host.initHost()

        expect(host.serverUrl.value).toBe(SERVER)
        await expect(host.httpFetch(`${SERVER}/api/v0/me`)).resolves.toBe(response)
        expect(tauri.pluginFetch).toHaveBeenCalledWith(`${SERVER}/api/v0/me`, undefined)
        expect(pageFetch).not.toHaveBeenCalled()
    })

    // First launch: nothing saved yet, so the login view's server step shows.
    it('leaves the server unset when none is saved', async () => {
        commands({ get_settings: () => ({ serverUrl: null }) })
        const host = await loadHost('app')

        await host.initHost()

        expect(host.serverUrl.value).toBeNull()
    })

    // A broken settings read must not stop the app from mounting: the server
    // step lets the user pick a server again.
    it('leaves the server unset and resolves when the settings cannot be read', async () => {
        commands({
            get_settings: () => {
                throw { code: 'Internal', message: 'Permission denied (os error 13)' }
            }
        })
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
        const host = await loadHost('app')

        await expect(host.initHost()).resolves.toBeUndefined()

        expect(host.serverUrl.value).toBeNull()
        expect(warn).toHaveBeenCalledTimes(1)
    })

    // axios's fetch adapter puts its abort signal on the Request, but the plugin
    // only listens to init.signal: the Request's signal must arrive there, or an
    // aborted call keeps running in Rust and never rejects.
    it('hands the plugin the abort signal a Request carries', async () => {
        commands({ get_settings: () => ({ serverUrl: SERVER }) })
        const host = await loadHost('app')
        await host.initHost()
        tauri.pluginFetch.mockResolvedValue(new Response('{}'))
        // (jsdom's AbortController is foreign to Node's Request, so the test
        // checks the signal's identity rather than aborting it.)
        const request = new Request(`${SERVER}/api/v0/libraries/preview`, { method: 'POST' })

        await host.httpFetch(request)

        const [sent, init] = tauri.pluginFetch.mock.calls[0] as [Request, RequestInit]
        expect(sent).toBe(request)
        expect(init.signal).toBe(request.signal)
    })

    it('setServerUrl adopts the URL as Rust normalised it', async () => {
        commands({
            get_settings: () => ({ serverUrl: null }),
            set_server_url: (args) => ({ serverUrl: String(args?.url).replace(/\/+$/, '') })
        })
        const host = await loadHost('app')
        await host.initHost()

        await expect(host.setServerUrl(`${SERVER}/`)).resolves.toBe(SERVER)

        expect(host.serverUrl.value).toBe(SERVER)
    })

    it('setServerUrl keeps the current server when Rust refuses the URL', async () => {
        const refusal = { code: 'InvalidServerUrl', message: 'invalid server url: ftp://h' }
        commands({
            get_settings: () => ({ serverUrl: SERVER }),
            set_server_url: () => {
                throw refusal
            }
        })
        const host = await loadHost('app')
        await host.initHost()

        await expect(host.setServerUrl('ftp://h')).rejects.toEqual(refusal)

        expect(host.serverUrl.value).toBe(SERVER)
    })

    it('clearServerUrl forgets the server', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER }),
            clear_server_url: () => ({ serverUrl: null })
        })
        const host = await loadHost('app')
        await host.initHost()

        await host.clearServerUrl()

        expect(host.serverUrl.value).toBeNull()
    })

    // The IPC argument names are a contract with the Rust commands (camelCase on
    // the JS side): a misspelt key reaches Rust as a missing argument.
    it('hands the session key to Rust and takes it back', async () => {
        const session = { key: null as string | null }
        commands({
            set_session: (args) => {
                session.key = String(args?.apiKey)
                return null
            },
            clear_session: () => {
                session.key = null
                return null
            }
        })
        const host = await loadHost('app')

        await host.setSession('aether_key')
        expect(session.key).toBe('aether_key')

        await host.clearSession()
        expect(session.key).toBeNull()
    })

    it('passes a refused session hand-off on to the caller', async () => {
        const notConnected = { code: 'NotConnected', message: 'no server configured' }
        commands({
            set_session: () => {
                throw notConnected
            }
        })
        const host = await loadHost('app')

        await expect(host.setSession('aether_key')).rejects.toEqual(notConnected)
    })

    it('reports the app version', async () => {
        commands({ app_version: () => '0.1.0' })
        const host = await loadHost('app')

        await expect(host.appVersion()).resolves.toBe('0.1.0')
    })
})
