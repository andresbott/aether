import { ref, readonly } from 'vue'

// Module-level singleton state: ONE app-wide connectivity indicator, not
// per-request toasts. isOffline stays false until reportNetworkError flips it
// true; it auto-dismisses on any successful request (dismissBanner) or when the
// browser reports reconnection (window 'online' event).
const _isOffline = ref(false)

/**
 * Mark the app as offline. Idempotent: while the banner is already showing,
 * further failures do nothing (no spam).
 */
export function reportNetworkError(): void {
    _isOffline.value = true
}

/**
 * Dismiss the connectivity banner. Called when any request succeeds or the
 * browser reports reconnection.
 */
export function dismissBanner(): void {
    _isOffline.value = false
}

// Alias for clarity in the QueryCache success handler.
export const markOnline = dismissBanner

// Register window offline/online listeners EXACTLY ONCE at module scope (NOT
// inside the useConnectivity function body — avoid duplicate listeners on
// repeated calls). Guard for SSR / test environments where window is undefined.
if (typeof window !== 'undefined') {
    window.addEventListener('offline', reportNetworkError)
    window.addEventListener('online', dismissBanner)
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
