import { ref } from 'vue'

const sidebarCollapsed = ref(false)
const sidebarWidth = ref(480)

// The floor is set by the row content, not by taste: a queue row carries index,
// cover, title/artist, the favorite toggle and the duration, and below ~370px the
// title starts ellipsizing to nothing.
const MIN_WIDTH = 370
const MAX_WIDTH = 800

export function useQueueSidebar() {
    const toggleSidebar = (): void => {
        sidebarCollapsed.value = !sidebarCollapsed.value
    }

    const expandSidebar = (): void => {
        sidebarCollapsed.value = false
    }

    const collapseSidebar = (): void => {
        sidebarCollapsed.value = true
    }

    const setSidebarWidth = (width: number): void => {
        sidebarWidth.value = Math.max(MIN_WIDTH, Math.min(MAX_WIDTH, width))
    }

    return {
        sidebarCollapsed,
        sidebarWidth,
        toggleSidebar,
        expandSidebar,
        collapseSidebar,
        setSidebarWidth,
        MIN_WIDTH,
        MAX_WIDTH
    }
}
