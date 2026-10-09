import { ref, type Ref } from 'vue'
import { isTauri } from '@tauri-apps/api/core'

/**
 * The address of the Aether server every /api/v0 and /rest request goes to: an
 * origin without a trailing slash. In the browser the server serves the SPA, so
 * it is the page's own origin and never null. Inside the desktop app the page
 * comes from a tauri:// origin and the server is a user setting: null until
 * initHost() reads it, and whenever none is chosen — the login view then asks
 * for one.
 *
 * Set from the start, not only by initHost(): the subsonic client reads it
 * whenever it is configured, including in specs that never boot the host.
 */
export const serverUrl: Ref<string | null> = ref(isTauri() ? null : window.location.origin)
