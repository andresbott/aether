import { readonly, ref, type Ref } from 'vue'

// The PlayerSheet is overlay state, not a route (same philosophy as LoginView):
// there is no /player URL. But on phones the system back gesture must dismiss
// the sheet rather than leave the app, so open() pushes one history entry and
// dismissal consumes it. History handling is best-effort — if pushState is
// unavailable the sheet still works purely via state (spec §6).
const isOpen = ref(false)
// Whether OUR entry is on top of the stack. Cleared by popstate (browser took
// it) and by close() (we take it with history.back()).
let pushed = false
let listening = false
// Pending callback set by close(onDone) — invoked after the next popstate.
let pendingCallback: (() => void) | null = null
let pendingTimeoutId: ReturnType<typeof setTimeout> | null = null

function onPopstate(): void {
    // Back consumed our entry: just close. Calling history.back() here would
    // eat the caller's own history.
    pushed = false
    isOpen.value = false
    // If a navigation callback is pending, invoke it now (capture before clearing)
    const cb = pendingCallback
    stopListening()
    clearPendingCallback()
    if (cb && typeof cb === 'function') {
        cb()
    }
}

function clearPendingCallback(): void {
    pendingCallback = null
    if (pendingTimeoutId !== null) {
        clearTimeout(pendingTimeoutId)
        pendingTimeoutId = null
    }
}

function stopListening(): void {
    if (!listening) return
    window.removeEventListener('popstate', onPopstate)
    listening = false
}

function open(): void {
    if (isOpen.value) return
    isOpen.value = true
    try {
        window.history.pushState({ aetherPlayerSheet: true }, '')
        pushed = true
        window.addEventListener('popstate', onPopstate)
        listening = true
    } catch {
        pushed = false
    }
}

function close(onDone?: () => void): void {
    if (!isOpen.value) return
    isOpen.value = false
    if (pushed) {
        pushed = false
        // history.back() fires popstate asynchronously. When the caller needs
        // to navigate after close(), they pass onDone — we invoke it after the
        // popstate lands (or ~200ms as a safety net), so the router.push() runs
        // after the history entry is consumed and cannot be rolled back.
        if (onDone) {
            clearPendingCallback() // clear any stale callback
            pendingCallback = onDone
            pendingTimeoutId = setTimeout(() => {
                if (pendingCallback === onDone) {
                    clearPendingCallback()
                    onDone()
                }
            }, 200)
            // DO NOT call stopListening() yet — onPopstate needs to run to invoke the callback
        } else {
            // No callback, safe to remove listener immediately
            stopListening()
        }
        window.history.back()
    } else {
        // Nothing was pushed, safe to remove listener
        stopListening()
        if (onDone) {
            // Nothing was pushed, so no popstate will fire. Invoke callback immediately.
            onDone()
        }
    }
}

export function usePlayerSheet(): {
    isOpen: Readonly<Ref<boolean>>
    open: () => void
    close: (onDone?: () => void) => void
} {
    return { isOpen: readonly(isOpen), open, close }
}

/** Test hook: reset singleton state between specs. */
export function resetPlayerSheetForTests(): void {
    isOpen.value = false
    pushed = false
    stopListening()
    clearPendingCallback()
}
