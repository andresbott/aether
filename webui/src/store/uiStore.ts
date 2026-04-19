import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useUiStore = defineStore('ui', () => {
    const sidebarCollapsed = ref(false)

    const toggleSidebar = () => {
        sidebarCollapsed.value = !sidebarCollapsed.value
    }

    const collapseSidebar = () => {
        sidebarCollapsed.value = true
    }

    const expandSidebar = () => {
        sidebarCollapsed.value = false
    }

    const checkScreenWidth = () => {
        if (window.innerWidth < 768) {
            collapseSidebar()
        }
    }

    return {
        sidebarCollapsed,
        toggleSidebar,
        collapseSidebar,
        expandSidebar,
        checkScreenWidth
    }
})
