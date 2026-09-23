import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { LibraryView as ViewMode } from '@/types/libraries'

type Layout = 'grid' | 'list'

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
        return libraryViewModes.value[viewMode] ?? 'grid'
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
