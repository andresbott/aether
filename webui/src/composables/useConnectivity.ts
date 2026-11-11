import { ref, readonly } from 'vue'

// Module-level singleton state: ONE app-wide connectivity indicator, not
// per-request toasts. isOffline stays false until reportNetworkError flips it
// true. It clears on any successful request (dismissBanner), or when the probe
// finds the server answering again.
//
// While offline, a cheap probe (main.ts installs it: a /rest ping, which also
// re-mints an expired key) runs on a backoff, and again straight away when the
// browser reports a reconnection or the page comes back to the foreground. A
// phone's 'online' event is not reliable on its own — it can fire before the
// route works, or not at all after a Wi-Fi ↔ mobile handover — so it only
// triggers a probe, never declares recovery itself. Recovery runs the
// onReconnect listeners: errored queries refetch, and a track that died with the
// network resumes. Without this, a blip left views broken until a reload.
const _isOffline = ref(false)

type Probe = () => Promise<boolean>
let probe: Probe | null = null
let probeTimer: ReturnType<typeof setTimeout> | null = null
let probeInFlight = false
let probeAttempt = 0
const reconnectListeners = new Set<() => void>()

// Waits between probes while offline; the last one repeats.
const PROBE_DELAYS_MS = [2_000, 5_000, 10_000, 20_000, 30_000]

/** Install the check that tells whether the server is reachable again. */
export function setConnectivityProbe(p: Probe | null): void {
    probe = p
}

/**
 * Run `cb` every time connectivity comes back after a reported failure.
 * Returns the function that unsubscribes it.
 */
export function onReconnect(cb: () => void): () => void {
    reconnectListeners.add(cb)
    return () => reconnectListeners.delete(cb)
}

function cancelProbe(): void {
    if (probeTimer !== null) clearTimeout(probeTimer)
    probeTimer = null
}

function scheduleProbe(): void {
    if (!probe || probeTimer !== null || probeInFlight) return
    const delay = PROBE_DELAYS_MS[Math.min(probeAttempt, PROBE_DELAYS_MS.length - 1)]
    probeAttempt++
    probeTimer = setTimeout(() => {
        probeTimer = null
        void runProbe()
    }, delay)
}

async function runProbe(): Promise<void> {
    if (!probe || probeInFlight) return
    probeInFlight = true
    let reachable = false
    try {
        reachable = await probe()
    } catch {
        reachable = false
    } finally {
        probeInFlight = false
    }
    if (reachable) {
        recovered()
    } else if (_isOffline.value) {
        scheduleProbe()
    }
}

function recovered(): void {
    const wasOffline = _isOffline.value
    _isOffline.value = false
    probeAttempt = 0
    cancelProbe()
    if (!wasOffline) return
    for (const cb of [...reconnectListeners]) {
        try {
            cb()
        } catch (err) {
            console.error('reconnect listener failed:', err)
        }
    }
}

/**
 * Mark the app as offline and start probing for the way back. Idempotent: while
 * the banner is already showing, further failures do nothing (no spam).
 */
export function reportNetworkError(): void {
    _isOffline.value = true
    scheduleProbe()
}

/**
 * Check right now instead of waiting for the next probe: the browser reported a
 * reconnection, or the page came back to the foreground. Without a probe there
 * is nothing to check, and the hint is taken at its word.
 */
export function checkConnectivityNow(): void {
    if (!_isOffline.value) return
    if (!probe) {
        recovered()
        return
    }
    cancelProbe()
    void runProbe()
}

/**
 * Dismiss the connectivity banner. Called when any request succeeds or the user
 * closes it; a dismissal that ends an outage runs the reconnect listeners.
 */
export function dismissBanner(): void {
    recovered()
}

// Alias for clarity in the QueryCache success handler.
export const markOnline = dismissBanner

// Register window/document listeners EXACTLY ONCE at module scope (NOT inside
// the useConnectivity function body — avoid duplicate listeners on repeated
// calls). Guard for SSR / test environments where window is undefined.
if (typeof window !== 'undefined') {
    window.addEventListener('offline', reportNetworkError)
    window.addEventListener('online', checkConnectivityNow)
}
if (typeof document !== 'undefined' && typeof document.addEventListener === 'function') {
    // A backgrounded phone throttles or freezes the probe's timers: check as soon
    // as the user is looking again.
    document.addEventListener('visibilitychange', () => {
        if (document.visibilityState === 'visible') checkConnectivityNow()
    })
}

/** Test hook: forget the probe, its timer and every listener. */
export function resetConnectivityForTests(): void {
    cancelProbe()
    probe = null
    probeInFlight = false
    probeAttempt = 0
    reconnectListeners.clear()
    _isOffline.value = false
}

/**
 * useConnectivity exposes read-only app-wide connectivity state. The banner
 * component reads `isOffline`; the QueryCache handlers and player call
 * `reportNetworkError()` / `dismissBanner()` directly — no need to instantiate
 * this composable there.
 */
export function useConnectivity() {
    return {
        isOffline: readonly(_isOffline)
    }
}
