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

    // Only the settings sidebar: below 768px the music chrome is the mobile
    // shell, which never mounts AppSidebar, so collapsing it there was dead.
    // SettingsLayout has no mobile variant yet and still needs the collapse.
    const checkScreenWidth = () => {
        if (window.innerWidth < 768) {
            collapseSettingsSidebar()
        }
    }

    return {
        sidebarCollapsed,
        settingsSidebarCollapsed,
        toggleSidebar,
        collapseSidebar,
        expandSidebar,
        toggleSettingsSidebar,
        collapseSettingsSidebar,
        checkScreenWidth
    }
})
