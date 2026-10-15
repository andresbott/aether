import axios, { type CreateAxiosDefaults } from 'axios'
import { sessionExpired } from '@/lib/authState'
import { httpFetch, isDesktopApp, serverUrl, sessionKind } from '@/lib/host'

const API_BASE_URL = import.meta.env.VITE_SERVER_URL_V0 || '/api/v0'

// Inside the desktop app the webview cannot send the server's session cookie
// cross-site, so requests ride the host transport, which executes them from the
// Rust process. axios's fetch adapter takes its fetch from `env`; the arrow looks
// httpFetch up on every request. The browser keeps axios's default adapter.
const desktopTransport: CreateAxiosDefaults = {
    adapter: 'fetch',
    env: { fetch: (input, init) => httpFetch(input, init) }
}

export const apiClient = axios.create({
    baseURL: API_BASE_URL,
    headers: {
        'Content-Type': 'application/json'
    },
    // The session cookie must ride along when the Vite dev server points at a
    // different origin (VITE_SERVER_URL_V0); same-origin production is unaffected.
    withCredentials: true,
    ...(isDesktopApp() ? desktopTransport : {})
})

// In the app the server is a setting the user can change while the app runs, so
// the base URL is read on every request rather than fixed at creation.
if (isDesktopApp()) {
    apiClient.interceptors.request.use((config) => {
        // Nothing may request before the login view's server step has picked a
        // server; failing loudly beats a request to the window's own origin.
        if (serverUrl.value === null) {
            throw new Error('No Aether server is configured')
        }
        // A key session is /rest only — its server may sit behind a portal that
        // answers /api/v0 with a login page. The UI hides every caller; this
        // makes a stray one fail here, fast, instead of reaching the portal.
        if (sessionKind.value === 'apiKey') {
            throw new Error('Not available with an API key')
        }
        config.baseURL = `${serverUrl.value}/api/v0`
        return config
    })
}

// A 401 from any /api/v0 call means the session is gone (expired, or the
// server restarted with fresh cookie keys) — flag it so the SPA renders the
// login view. The login endpoint itself is exempt: its 401 is the uniform
// wrong-credentials answer, not a lost session. So is, in the app, a 401 from a
// server that is no longer the configured one.
apiClient.interceptors.response.use(
    (response) => response,
    (error) => {
        const status = error?.response?.status
        const url: string = error?.config?.url ?? ''
        if (
            status === 401 &&
            !url.includes('/auth/login') &&
            answersForConfiguredServer(error?.config?.baseURL)
        ) {
            sessionExpired.value = true
        }
        return Promise.reject(error)
    }
)

// In the app a 401 speaks for the server its request was sent to, which the
// request interceptor stamped into its baseURL. A request a server switch
// overtook (a poll sent while the logout was out) answers for a session that is
// already over: with no server configured nothing can expire, and a server
// chosen since has a session of its own the flag would hold back. A tab's
// server is the page's own and never changes, so every 401 speaks for it.
function answersForConfiguredServer(baseURL: string | undefined): boolean {
    if (!isDesktopApp()) return true
    return serverUrl.value !== null && baseURL === `${serverUrl.value}/api/v0`
}
