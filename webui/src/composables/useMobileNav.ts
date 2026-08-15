import { ref, type Ref } from 'vue'

// The mobile nav drawer is shell chrome, but its trigger (the hamburger in
// ContentScaffold's header) lives inside the route views — so the open state
// is a module-scoped singleton, same pattern as usePlayer/useViewport. Plain
// overlay state, no history entry: MobileNavDrawer closes itself on any route
// change, so a system-back press never navigates underneath an open drawer
// (see the watcher there).
const isOpen = ref(false)

interface MobileNavState {
    isOpen: Ref<boolean>
    open: () => void
    close: () => void
}

export function useMobileNav(): MobileNavState {
    return {
        isOpen,
        open: () => {
            isOpen.value = true
        },
        close: () => {
            isOpen.value = false
        }
    }
}

/** Test hook: reset the singleton between specs. */
export function resetMobileNavForTests(): void {
    isOpen.value = false
}
