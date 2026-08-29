import { defineStore } from 'pinia'
import { ref } from 'vue'

type ViewMode = 'discover' | 'albums' | 'artists' | 'songs'
type Layout = 'grid' | 'list'

// Per-type defaults: grid for discover/albums/artists, list for songs.
const defaultLayoutForType = (viewMode: ViewMode): Layout => {
    return viewMode === 'songs' ? 'list' : 'grid'
}

export const useUiStore = defineStore('ui', () => {
    const sidebarCollapsed = ref(false)
    const settingsSidebarCollapsed = ref(false)

    // Library view modes: session-scoped per-type layout state.
    // Each type remembers its own override; unset entries fall back to the default.
    const libraryViewModes = ref<Partial<Record<ViewMode, Layout>>>({})

    const toggleSidebar = () => {
        sidebarCollapsed.value = !sidebarCollapsed.value
    }

    const collapseSidebar = () => {
        sidebarCollapsed.value = true
    }

    const expandSidebar = () => {
        sidebarCollapsed.value = false
    }

    const toggleSettingsSidebar = () => {
        settingsSidebarCollapsed.value = !settingsSidebarCollapsed.value
    }

    const getLibraryViewMode = (viewMode: ViewMode): Layout => {
        return libraryViewModes.value[viewMode] ?? defaultLayoutForType(viewMode)
    }

    const setLibraryViewMode = (viewMode: ViewMode, layout: Layout) => {
        libraryViewModes.value[viewMode] = layout
    }

    return {
        sidebarCollapsed,
        settingsSidebarCollapsed,
        toggleSidebar,
        collapseSidebar,
        expandSidebar,
        toggleSettingsSidebar,
        getLibraryViewMode,
        setLibraryViewMode
    }
})
