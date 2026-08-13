import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUiStore = defineStore('ui', () => {
    const sidebarCollapsed = ref(false)
    const settingsSidebarCollapsed = ref(false)

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

    const collapseSettingsSidebar = () => {
        settingsSidebarCollapsed.value = true
    }

    return {
        sidebarCollapsed,
        settingsSidebarCollapsed,
        toggleSidebar,
        collapseSidebar,
        expandSidebar,
        toggleSettingsSidebar,
        collapseSettingsSidebar
    }
})
