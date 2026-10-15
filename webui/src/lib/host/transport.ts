// The HTTP transport every /api/v0 and /rest request rides. In the browser it is
// the page's own fetch. Inside the desktop app the webview cannot send the
// server's session cookie cross-site, so initHost() installs tauri-plugin-http's
// fetch here instead: the request leaves from the Rust process, which keeps the
// cookie jar.

let installed: typeof fetch | null = null

/**
 * fetch through the installed transport, or the global fetch when none is
 * installed. Resolved on every call, never captured at import: specs stub the
 * global fetch after this module has loaded, and their stub must still answer.
 */
export function httpFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
    return installed ? installed(input, init) : globalThis.fetch(input, init)
}

/** Route every later httpFetch call through `f`. */
export function installTransport(f: typeof fetch): void {
    installed = f
}
