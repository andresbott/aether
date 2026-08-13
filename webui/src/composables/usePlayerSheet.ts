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

function onPopstate(): void {
    // Back consumed our entry: just close. Calling history.back() here would
    // eat the caller's own history.
    pushed = false
    isOpen.value = false
    stopListening()
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

function close(): void {
    if (!isOpen.value) return
    isOpen.value = false
    stopListening()
    if (pushed) {
        pushed = false
        window.history.back()
    }
}

export function usePlayerSheet(): {
    isOpen: Readonly<Ref<boolean>>
    open: () => void
    close: () => void
} {
    return { isOpen: readonly(isOpen), open, close }
}

/** Test hook: reset singleton state between specs. */
export function resetPlayerSheetForTests(): void {
    isOpen.value = false
    pushed = false
    stopListening()
}
