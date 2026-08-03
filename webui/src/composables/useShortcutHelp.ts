import { ref } from 'vue'

// Module-scoped so the key handler in PlayerLayout and the overlay component
// share one flag without prop threading — same pattern as useQueueSidebar.
const open = ref(false)

export function useShortcutHelp() {
    return {
        open,
        toggle: (): void => {
            open.value = !open.value
        },
        close: (): void => {
            open.value = false
        }
    }
}
