import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUiStore } from '@/store/uiStore'

beforeEach(() => {
    setActivePinia(createPinia())
})

describe('uiStore settings sidebar', () => {
    it('defaults to expanded and toggles independently of the main sidebar', () => {
        const ui = useUiStore()
        expect(ui.settingsSidebarCollapsed).toBe(false)
        ui.toggleSettingsSidebar()
        expect(ui.settingsSidebarCollapsed).toBe(true)
        // main sidebar is unaffected
        expect(ui.sidebarCollapsed).toBe(false)
        ui.toggleSettingsSidebar()
        expect(ui.settingsSidebarCollapsed).toBe(false)
    })

    it('collapseSettingsSidebar forces collapsed', () => {
        const ui = useUiStore()
        ui.collapseSettingsSidebar()
        expect(ui.settingsSidebarCollapsed).toBe(true)
    })

    // Only the settings sidebar: the music sidebar does not exist below 768px
    // (the mobile shell replaces it), so collapsing it there would be dead.
    it('checkScreenWidth collapses only the settings sidebar below 768px', () => {
        const ui = useUiStore()
        window.innerWidth = 500
        ui.checkScreenWidth()
        expect(ui.settingsSidebarCollapsed).toBe(true)
        expect(ui.sidebarCollapsed).toBe(false)
    })
})
