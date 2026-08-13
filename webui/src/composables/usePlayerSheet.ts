import { readonly, ref, type Ref } from 'vue'

// The PlayerSheet is overlay state, not a route (same philosophy as LoginView):
// there is no /player URL. But on phones the system back gesture must dismiss
// the sheet rather than leave the app, so open() pushes one duplicate history
// entry and close() consumes it. History handling is best-effort — if pushState
// is unavailable the sheet still works purely via state (spec §6).
//
// There are TWO dismissal paths, and picking the wrong one breaks navigation:
//
//   close()   — the user dismissed the sheet itself (chevron, Esc, swipe, or a
//               link inside the sheet). Our entry is on top, so we consume it
//               with history.back().
//   dismiss() — the sheet is going away as a side effect of something else
//               (a route change started elsewhere, or the shell unmounting).
//               Our entry is no longer on top — the new route's entry is — so
//               history.back() would pop THAT and bounce the navigation right
//               back. This path touches no history: it only drops our state and
//               strips the marker if our entry happens to still be current.
const isOpen = ref(false)

/** Shape of the extra key we write into history.state. */
interface SheetHistoryState {
    aetherPlayerSheet?: number
}

// Marker of the entry pushed for the currently-open sheet, or null when nothing
// of ours is on top (pushState denied, or the entry has been consumed/orphaned).
// Every open() gets a fresh marker so a stale entry can never be mistaken for
// the live one.
let openMarker: number | null = null
let markerSeq = 0
let listening = false

// FIFO of consumptions WE started with history.back(). Each queued item swallows
// exactly one popstate, which is what makes the two races safe:
//   * close() then an immediate open(): the in-flight popstate belongs to the
//     first entry, so it must not close the freshly reopened sheet.
//   * close(onDone): the caller's navigation must run only after the entry is
//     actually gone, otherwise the pending back() rolls it straight back.
// There is deliberately no wall-clock fallback timer: history.back() always
// fires popstate when we pushed the entry ourselves, and a timer racing a slow
// popstate is exactly the double-navigation bug it would be trying to paper
// over. The one case where no popstate can ever arrive — pushState threw, so
// nothing was pushed — is handled synchronously in close().
const pendingConsumptions: Array<{ onDone?: () => void }> = []

/** Current history.state as a plain mutable copy (never null). */
function currentState(): Record<string, unknown> & SheetHistoryState {
    try {
        return { ...(window.history.state as Record<string, unknown> | null) }
    } catch {
        return {}
    }
}

function onPopstate(event: PopStateEvent): void {
    if (pendingConsumptions.length > 0) {
        // Our own history.back() landing. The sheet state was already updated by
        // close(); all that is left is the caller's navigation.
        const { onDone } = pendingConsumptions.shift() as { onDone?: () => void }
        maybeStopListening()
        onDone?.()
        return
    }
    // A real system back. Calling history.back() here would eat the user's own
    // history. If we landed back ON our own live entry (forward/back shuffling
    // around a reopened sheet), leave the sheet alone.
    const landed = (event.state as SheetHistoryState | null)?.aetherPlayerSheet
    if (openMarker !== null && landed === openMarker) return
    openMarker = null
    isOpen.value = false
    maybeStopListening()
}

function startListening(): void {
    if (listening) return
    window.addEventListener('popstate', onPopstate)
    listening = true
}

function stopListening(): void {
    if (!listening) return
    window.removeEventListener('popstate', onPopstate)
    listening = false
}

/** Detach only once nothing of ours is on the stack and nothing is in flight. */
function maybeStopListening(): void {
    if (openMarker === null && pendingConsumptions.length === 0) stopListening()
}

function open(): void {
    if (isOpen.value) return
    isOpen.value = true
    const marker = ++markerSeq
    try {
        // Spread the existing state: vue-router keeps its scroll/position
        // bookkeeping in there and a bare object would strand it.
        window.history.pushState({ ...currentState(), aetherPlayerSheet: marker }, '')
        openMarker = marker
        startListening()
    } catch {
        openMarker = null
    }
}

function close(onDone?: () => void): void {
    if (!isOpen.value) return
    isOpen.value = false
    if (openMarker === null) {
        // Nothing of ours on the stack, so no popstate will ever arrive: run the
        // caller's navigation now.
        maybeStopListening()
        onDone?.()
        return
    }
    openMarker = null
    pendingConsumptions.push({ onDone })
    // Stay subscribed: onPopstate is what tells us the entry is gone.
    startListening()
    window.history.back()
}

/**
 * Dismiss without touching the history stack — for a route change or an unmount
 * that is already a navigation of its own (see the note at the top of the file).
 * The duplicate entry we pushed is left behind as an orphan; it carries the same
 * URL as the entry below it, so a later back through it is visually a no-op
 * rather than a swallowed navigation. Its marker is stripped when our entry is
 * still the current one, so nothing downstream can mistake it for a live sheet.
 */
function dismiss(): void {
    if (!isOpen.value) return
    isOpen.value = false
    openMarker = null
    const state = currentState()
    if (state.aetherPlayerSheet !== undefined) {
        delete state.aetherPlayerSheet
        try {
            window.history.replaceState(state, '')
        } catch {
            // Best-effort, same as pushState in open().
        }
    }
    maybeStopListening()
}

export function usePlayerSheet(): {
    isOpen: Readonly<Ref<boolean>>
    open: () => void
    close: (onDone?: () => void) => void
    dismiss: () => void
} {
    return { isOpen: readonly(isOpen), open, close, dismiss }
}

/** Test hook: reset singleton state between specs. */
export function resetPlayerSheetForTests(): void {
    isOpen.value = false
    openMarker = null
    pendingConsumptions.length = 0
    stopListening()
}
