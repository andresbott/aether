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
})

describe('uiStore library view modes', () => {
    it('defaults to grid for discover, albums, and artists', () => {
        const ui = useUiStore()
        expect(ui.getLibraryViewMode('discover')).toBe('grid')
        expect(ui.getLibraryViewMode('albums')).toBe('grid')
        expect(ui.getLibraryViewMode('artists')).toBe('grid')
    })

    it('defaults to list for songs', () => {
        const ui = useUiStore()
        expect(ui.getLibraryViewMode('songs')).toBe('list')
    })

    it('setting albums to list does not change artists', () => {
        const ui = useUiStore()
        ui.setLibraryViewMode('albums', 'list')
        expect(ui.getLibraryViewMode('albums')).toBe('list')
        expect(ui.getLibraryViewMode('artists')).toBe('grid')
    })

    it('setting discover to list does not change albums or artists', () => {
        const ui = useUiStore()
        ui.setLibraryViewMode('discover', 'list')
        expect(ui.getLibraryViewMode('discover')).toBe('list')
        expect(ui.getLibraryViewMode('albums')).toBe('grid')
        expect(ui.getLibraryViewMode('artists')).toBe('grid')
    })

    it('overrides survive across store re-access (session persistence)', () => {
        const ui1 = useUiStore()
        ui1.setLibraryViewMode('albums', 'list')
        ui1.setLibraryViewMode('artists', 'list')
        expect(ui1.getLibraryViewMode('albums')).toBe('list')
        expect(ui1.getLibraryViewMode('artists')).toBe('list')

        // Simulate re-accessing the same store instance in the same session
        const ui2 = useUiStore()
        expect(ui2.getLibraryViewMode('albums')).toBe('list')
        expect(ui2.getLibraryViewMode('artists')).toBe('list')
        expect(ui2.getLibraryViewMode('discover')).toBe('grid')
    })

    it('fresh store instance returns to defaults (reload semantics)', () => {
        const ui1 = useUiStore()
        ui1.setLibraryViewMode('albums', 'list')
        expect(ui1.getLibraryViewMode('albums')).toBe('list')

        // Simulate a fresh page load: new Pinia instance
        setActivePinia(createPinia())
        const ui2 = useUiStore()
        expect(ui2.getLibraryViewMode('albums')).toBe('grid')
        expect(ui2.getLibraryViewMode('artists')).toBe('grid')
        expect(ui2.getLibraryViewMode('discover')).toBe('grid')
        expect(ui2.getLibraryViewMode('songs')).toBe('list')
    })
})
