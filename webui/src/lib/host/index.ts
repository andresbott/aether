// The host layer: the one place the SPA learns whether it runs in a browser tab
// served by the Aether server or inside the desktop player (Tauri), and the only
// code that talks to the player's Rust side. Keeping every app concern behind
// it is what keeps the rest of webui/src origin-agnostic.
import { invoke, isTauri } from '@tauri-apps/api/core'
import { serverUrl } from './serverUrl'
import { installTransport } from './transport'

export { onTrayCommand, type TrayCommand } from './events'
export { serverUrl } from './serverUrl'
export { httpFetch } from './transport'

/** The player's persisted settings, as the Rust settings commands answer them. */
interface Settings {
    serverUrl: string | null
}

let desktopApp: boolean | undefined

/** True inside the desktop player's window, false in a browser tab. */
export function isDesktopApp(): boolean {
    if (desktopApp === undefined) desktopApp = isTauri()
    return desktopApp
}

/**
 * Settles where the server is, and in the app how requests reach it, before the
 * SPA mounts. Never throws: a failure in the app leaves the server unset, so the
 * login view asks for one instead of the window staying blank.
 */
export async function initHost(): Promise<void> {
    if (!isDesktopApp()) {
        serverUrl.value = window.location.origin
        return
    }
    try {
        // Imported here, not at the top, so the browser never evaluates the plugin.
        const { fetch: pluginFetch } = await import('@tauri-apps/plugin-http')
        installTransport((input, init) => pluginFetch(input, withRequestSignal(input, init)))
        serverUrl.value = (await invoke<Settings>('get_settings')).serverUrl
    } catch (err) {
        // Safe to log whole: no credential is involved in either step.
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

/**
 * Saves the server the app talks to and adopts the URL as Rust normalised it
 * (trailing slash trimmed). A refused URL rejects with the IPC error and leaves
 * the current server untouched.
 */
export async function setServerUrl(url: string): Promise<string> {
    assertDesktopApp('Choosing a server')
    // set_server_url answers the saved settings, so serverUrl is never null here.
    const saved = await invoke<{ serverUrl: string }>('set_server_url', { url })
    serverUrl.value = saved.serverUrl
    return saved.serverUrl
}

/** Forgets the saved server; the login view's server step shows again. */
export async function clearServerUrl(): Promise<void> {
    assertDesktopApp('Forgetting the server')
    serverUrl.value = (await invoke<Settings>('clear_server_url')).serverUrl
}

/** Hands the SPA's /rest key to the Rust client, which talks to the same server. */
export async function setSession(apiKey: string): Promise<void> {
    if (!isDesktopApp()) return
    await invoke('set_session', { apiKey })
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
