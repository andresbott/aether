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
        await expect(host.setServerUrl(SERVER, 'cookie')).rejects.toThrow(/desktop app/)
        await expect(host.clearServerUrl()).rejects.toThrow(/desktop app/)
        expect(host.serverUrl.value).toBe(window.location.origin)
        expect(tauri.invoke).not.toHaveBeenCalled()
    })

    it('is always a cookie session with no key, and refuses to change either', async () => {
        const host = await loadHost('browser')
        expect(host.sessionKind.value).toBe('cookie')
        expect(host.apiKey.value).toBeNull()
        await expect(host.setSessionKind('apiKey')).rejects.toThrow(/desktop app/)
        await expect(host.setApiKey('aether_k3y')).rejects.toThrow(/desktop app/)
        await expect(host.clearApiKey()).rejects.toThrow(/desktop app/)
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
        commands({ get_settings: () => ({ serverUrl: SERVER, sessionKind: 'cookie' }) })
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
        commands({ get_settings: () => ({ serverUrl: null, sessionKind: 'cookie' }) })
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
        commands({ get_settings: () => ({ serverUrl: SERVER, sessionKind: 'cookie' }) })
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
            get_settings: () => ({ serverUrl: null, sessionKind: 'cookie' }),
            set_server_url: (args) => ({
                serverUrl: String(args?.url).replace(/\/+$/, ''),
                sessionKind: 'cookie'
            })
        })
        const host = await loadHost('app')
        await host.initHost()

        await expect(host.setServerUrl(`${SERVER}/`, 'cookie')).resolves.toBe(SERVER)

        expect(host.serverUrl.value).toBe(SERVER)
    })

    it('setServerUrl keeps the current server when Rust refuses the URL', async () => {
        const refusal = { code: 'InvalidServerUrl', message: 'invalid server url: ftp://h' }
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'cookie' }),
            set_server_url: () => {
                throw refusal
            }
        })
        const host = await loadHost('app')
        await host.initHost()

        await expect(host.setServerUrl('ftp://h', 'cookie')).rejects.toEqual(refusal)

        expect(host.serverUrl.value).toBe(SERVER)
    })

    it('clearServerUrl forgets the server', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'cookie' }),
            clear_server_url: () => ({ serverUrl: null, sessionKind: 'cookie' })
        })
        const host = await loadHost('app')
        await host.initHost()

        await host.clearServerUrl()

        expect(host.serverUrl.value).toBeNull()
    })

    it('initHost adopts a key session: the kind, the server and the stored key', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => 'aether_k3y'
        })
        const host = await loadHost('app')

        await host.initHost()

        expect(host.sessionKind.value).toBe('apiKey')
        expect(host.serverUrl.value).toBe(SERVER)
        expect(host.apiKey.value).toBe('aether_k3y')
    })

    it('asks Rust for no key in a cookie session', async () => {
        commands({ get_settings: () => ({ serverUrl: SERVER, sessionKind: 'cookie' }) })
        const host = await loadHost('app')

        await host.initHost()

        expect(host.sessionKind.value).toBe('cookie')
        expect(host.apiKey.value).toBeNull()
        expect(tauri.invoke).not.toHaveBeenCalledWith('get_api_key')
    })

    // Review focus 3: the key card asks again; the window never stays blank.
    it('keeps the kind and server, and resolves, when the stored key cannot be read', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => {
                throw { code: 'Internal', message: 'Permission denied (os error 13)' }
            }
        })
        const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
        const host = await loadHost('app')

        await expect(host.initHost()).resolves.toBeUndefined()

        expect(host.sessionKind.value).toBe('apiKey')
        expect(host.serverUrl.value).toBe(SERVER)
        expect(host.apiKey.value).toBeNull()
        expect(warn).toHaveBeenCalledTimes(1)
    })

    // The kind rides with the address, so a portal server is never saved as a
    // cookie session even for a tick — useMe must not see it as one.
    it('setServerUrl saves the kind with the address and forgets a key for another server', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => 'aether_k3y',
            set_server_url: (args) => ({
                serverUrl: String(args?.url),
                sessionKind: args?.sessionKind
            })
        })
        const host = await loadHost('app')
        await host.initHost()
        const seen: string[] = []
        const { watch } = await import('vue')
        // Synchronous, so the kind is read at the very moment the server changes.
        watch(
            host.serverUrl,
            () => seen.push(`server ${host.serverUrl.value} as ${host.sessionKind.value}`),
            { flush: 'sync' }
        )

        await host.setServerUrl('http://other.lan:8075', 'cookie')

        expect(tauri.invoke).toHaveBeenCalledWith('set_server_url', {
            url: 'http://other.lan:8075',
            sessionKind: 'cookie'
        })
        expect(host.sessionKind.value).toBe('cookie')
        expect(host.apiKey.value).toBeNull()
        await host.setServerUrl('http://portal.example.org', 'apiKey')
        expect(seen).toEqual([
            'server http://other.lan:8075 as cookie',
            'server http://portal.example.org as apiKey'
        ])
    })

    // Connect on the same address after an outage: Rust kept the key, and the
    // host mirrors what Rust kept rather than guessing.
    it('setServerUrl keeps the key Rust kept for the same server', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => 'aether_k3y',
            set_server_url: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' })
        })
        const host = await loadHost('app')
        await host.initHost()

        await host.setServerUrl(SERVER, 'apiKey')

        expect(host.apiKey.value).toBe('aether_k3y')
        expect(host.sessionKind.value).toBe('apiKey')
    })

    // A key belongs to the server that issued it. Rust deletes it when another
    // server is saved, even for a key session, and the host must read that back:
    // carrying its own copy over would sign the new server's /rest calls with
    // the old server's key. The mock is stateful, as Rust is, so what the host
    // reads after the save is whatever Rust left, not a canned answer.
    it('setServerUrl never keeps a key for another server, even in a key session', async () => {
        const stored = { key: 'aether_k3y' as string | null }
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => stored.key,
            set_server_url: (args) => {
                if (args?.url !== SERVER) stored.key = null
                return { serverUrl: String(args?.url), sessionKind: args?.sessionKind }
            }
        })
        const host = await loadHost('app')
        await host.initHost()
        // Loaded, so the null below is Rust's doing and not a key never read.
        expect(host.apiKey.value).toBe('aether_k3y')

        await host.setServerUrl('http://other.lan:8075', 'apiKey')

        expect(host.serverUrl.value).toBe('http://other.lan:8075')
        expect(host.sessionKind.value).toBe('apiKey')
        expect(host.apiKey.value).toBeNull()
    })

    it('setSessionKind persists the kind and drops the key when leaving the key session', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => 'aether_k3y',
            set_session_kind: (args) => ({ serverUrl: SERVER, sessionKind: args?.sessionKind })
        })
        const host = await loadHost('app')
        await host.initHost()

        await host.setSessionKind('cookie')

        expect(tauri.invoke).toHaveBeenCalledWith('set_session_kind', { sessionKind: 'cookie' })
        expect(host.sessionKind.value).toBe('cookie')
        expect(host.apiKey.value).toBeNull()
    })

    it('setApiKey stores the key and makes this a key session', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'cookie' }),
            set_api_key: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' })
        })
        const host = await loadHost('app')
        await host.initHost()

        await host.setApiKey('aether_k3y')

        expect(tauri.invoke).toHaveBeenCalledWith('set_api_key', { apiKey: 'aether_k3y' })
        expect(host.sessionKind.value).toBe('apiKey')
        expect(host.apiKey.value).toBe('aether_k3y')
    })

    it('setApiKey leaves the session untouched when Rust refuses the key', async () => {
        const notConnected = { code: 'NotConnected', message: 'no server configured' }
        commands({
            get_settings: () => ({ serverUrl: null, sessionKind: 'cookie' }),
            set_api_key: () => {
                throw notConnected
            }
        })
        const host = await loadHost('app')
        await host.initHost()

        await expect(host.setApiKey('aether_k3y')).rejects.toEqual(notConnected)

        expect(host.sessionKind.value).toBe('cookie')
        expect(host.apiKey.value).toBeNull()
    })

    it('clearApiKey forgets the key but keeps the kind', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => 'aether_k3y',
            clear_api_key: () => null
        })
        const host = await loadHost('app')
        await host.initHost()

        await host.clearApiKey()

        expect(tauri.invoke).toHaveBeenCalledWith('clear_api_key')
        expect(host.apiKey.value).toBeNull()
        expect(host.sessionKind.value).toBe('apiKey')
    })

    it('clearServerUrl forgets the key and resets the kind', async () => {
        commands({
            get_settings: () => ({ serverUrl: SERVER, sessionKind: 'apiKey' }),
            get_api_key: () => 'aether_k3y',
            clear_server_url: () => ({ serverUrl: null, sessionKind: 'cookie' })
        })
        const host = await loadHost('app')
        await host.initHost()

        await host.clearServerUrl()

        expect(host.serverUrl.value).toBeNull()
        expect(host.sessionKind.value).toBe('cookie')
        expect(host.apiKey.value).toBeNull()
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
