// The host layer: the one place the SPA learns whether it runs in a browser tab
// served by the Aether server or inside the desktop player (Tauri), and the only
// code that talks to the player's Rust side. Keeping every app concern behind
// it is what keeps the rest of webui/src origin-agnostic.
import { invoke, isTauri } from '@tauri-apps/api/core'
import { serverUrl } from './serverUrl'
import { apiKey, sessionKind, type SessionKind } from './session'
import { installTransport } from './transport'

export { onTrayCommand, type TrayCommand } from './events'
export { serverHost, serverUrl } from './serverUrl'
export { apiKey, sessionKind, type SessionKind } from './session'
export { httpFetch } from './transport'

/** The player's persisted settings, as the Rust settings commands answer them. */
interface Settings {
    serverUrl: string | null
    sessionKind: SessionKind
}

let desktopApp: boolean | undefined

const TRANSPORT_UNAVAILABLE = "The desktop app's HTTP transport did not load"

/** True inside the desktop player's window, false in a browser tab. */
export function isDesktopApp(): boolean {
    if (desktopApp === undefined) desktopApp = isTauri()
    return desktopApp
}

/**
 * Settles where the server is, how the SPA signs in to it and, in the app, how
 * requests reach it, before the SPA mounts. Never throws: a failure in the app
 * leaves whatever was not read unset, so the login view asks for it instead of
 * the window staying blank.
 */
export async function initHost(): Promise<void> {
    if (!isDesktopApp()) {
        serverUrl.value = window.location.origin
        return
    }
    // Until the plugin has loaded — and for good if it cannot — every request
    // fails with a reason. The webview's own fetch would fail too, blocked by the
    // window's CSP, but without saying why.
    installTransport(() => Promise.reject(new Error(TRANSPORT_UNAVAILABLE)))
    try {
        // Imported here, not at the top, so the browser never evaluates the plugin.
        const { fetch: pluginFetch } = await import('@tauri-apps/plugin-http')
        installTransport((input, init) => pluginFetch(input, withRequestSignal(input, init)))
        const settings = await invoke<Settings>('get_settings')
        adopt(settings)
        if (settings.sessionKind === 'apiKey') {
            apiKey.value = await invoke<string | null>('get_api_key')
        }
    } catch (err) {
        // Safe to log whole: no credential is in the error of any of the three
        // calls above — the plugin import, get_settings and get_api_key, which
        // answers the key but never puts it in an error.
        console.warn('Desktop host setup failed', err)
    }
}

// The plugin cancels a request only through init.signal and ignores the signal
// a Request carries, which is where axios's fetch adapter puts it. Lifted into
// init, an abort (a superseded preview, a cancelled identify) stops the request
// in Rust and rejects the call, as it does in the browser.
function withRequestSignal(input: RequestInfo | URL, init?: RequestInit): RequestInit | undefined {
    if (!(input instanceof Request) || init?.signal) return init
    return { ...init, signal: input.signal }
}

// The kind before the server: useMe is enabled by a server in a cookie session,
// so on the way into a key session the new server is never read as a cookie one,
// not even for the tick between the two assignments. Out of a key session the
// order does not help: until the second assignment the old server reads as a
// cookie session's.
function adopt(settings: Settings): void {
    sessionKind.value = settings.sessionKind
    serverUrl.value = settings.serverUrl
}

/**
 * Saves the server the app talks to, and how to sign in to it, and adopts the
 * URL as Rust normalised it (trailing slash trimmed). Rust keeps a stored key
 * only when the same server is saved again for a key session (Connect after an
 * outage); the host mirrors what it kept. A refused URL rejects with the IPC
 * error and leaves the current server untouched.
 */
export async function setServerUrl(url: string, kind: SessionKind): Promise<string> {
    assertDesktopApp('Choosing a server')
    // set_server_url answers the saved settings, so serverUrl is never null here.
    const saved = await invoke<Settings & { serverUrl: string }>('set_server_url', {
        url,
        sessionKind: kind
    })
    apiKey.value =
        saved.sessionKind === 'apiKey' ? await invoke<string | null>('get_api_key') : null
    adopt(saved)
    return saved.serverUrl
}

/** Forgets the saved server, the sign-in kind and any stored key; the login view's server step shows again. */
export async function clearServerUrl(): Promise<void> {
    assertDesktopApp('Forgetting the server')
    apiKey.value = null
    adopt(await invoke<Settings>('clear_server_url'))
}

/**
 * Switches how the SPA signs in to the saved server. Leaving the key session
 * drops the stored key (Rust deletes its file).
 */
export async function setSessionKind(kind: SessionKind): Promise<void> {
    assertDesktopApp('Changing how to sign in')
    const saved = await invoke<Settings>('set_session_kind', { sessionKind: kind })
    if (saved.sessionKind !== 'apiKey') apiKey.value = null
    adopt(saved)
}

/**
 * Stores a key the login view has verified against the saved server; Rust
 * makes this a key session and connects its own client with it. A refusal
 * rejects with the IPC error and changes nothing here.
 */
export async function setApiKey(key: string): Promise<void> {
    assertDesktopApp('Signing in with an API key')
    const saved = await invoke<Settings>('set_api_key', { apiKey: key })
    adopt(saved)
    apiKey.value = key
}

/**
 * Forgets the stored key (logout, or a key the server stopped accepting): here
 * first, so the SPA is signed out whatever Rust answers, then on disk. The
 * kind stays, so the key card comes back rather than the server step.
 */
export async function clearApiKey(): Promise<void> {
    assertDesktopApp('Forgetting the API key')
    apiKey.value = null
    await invoke('clear_api_key')
}

/** Hands the SPA's /rest key to the Rust client, which talks to the same server (cookie flow). */
export async function setSession(key: string): Promise<void> {
    if (!isDesktopApp()) return
    await invoke('set_session', { apiKey: key })
}

/** Drops the Rust client's /rest key. */
export async function clearSession(): Promise<void> {
    if (!isDesktopApp()) return
    await invoke('clear_session')
}

/** The desktop player's version, or null in the browser, where there is no app. */
export async function appVersion(): Promise<string | null> {
    if (!isDesktopApp()) return null
    return invoke<string>('app_version')
}

// In the browser the server is the page's own origin, so only the app can
// change it; without this guard the call would fail on a missing Tauri global.
function assertDesktopApp(action: string): void {
    if (!isDesktopApp()) throw new Error(`${action} is only possible in the desktop app`)
}
