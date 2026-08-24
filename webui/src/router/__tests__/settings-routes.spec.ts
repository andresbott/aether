import { describe, it, expect, vi } from 'vitest'
import router from '@/router'

describe('settings routes', () => {
    it('resolves each settings topic to its named route', () => {
        expect(router.resolve('/settings/libraries').name).toBe('settings-libraries')
        expect(router.resolve('/settings/tasks').name).toBe('settings-tasks')
    })

    // The metadata editor moved out of the settings children to a top-level
    // route, but keeps the admin-only settings layout. It is reached from the
    // sidebar UserMenu, not the Settings side-nav.
    it('serves the metadata editor as a top-level route in the settings layout', () => {
        const meta = router.resolve('/metadata-editor')
        expect(meta.name).toBe('metadata-editor')
        expect(meta.meta.layout).toBe('settings')
        // The old nested path no longer matches anything.
        expect(router.resolve('/settings/metadata').matched.length).toBe(0)
    })

    // The profile page moved out of settings: it is the User settings main
    // content view in the player layout now, reached from the sidebar's user
    // menu.
    it('serves User settings as a flush main content view, not a settings child', () => {
        const userSettings = router.resolve('/user-settings')
        expect(userSettings.name).toBe('user-settings')
        expect(userSettings.meta.flush).toBe(true)
        expect(userSettings.meta.layout).toBeUndefined()
        expect(router.hasRoute('settings-profile')).toBe(false)
    })

    // The active section is a path segment so it survives a reload and can be
    // linked to directly.
    it('resolves a User settings section to the same route with a tab param', () => {
        const access = router.resolve('/user-settings/access')
        expect(access.name).toBe('user-settings')
        expect(access.params.tab).toBe('access')
        expect(access.meta.flush).toBe(true)
        // Bare /user-settings is the default section: no tab param.
        expect(router.resolve('/user-settings').params.tab).toBe('')
    })

    it('applies the settings layout to settings routes', () => {
        expect(router.resolve('/settings/libraries').meta.layout).toBe('settings')
    })

    it('registers a named "settings" parent route', () => {
        expect(router.hasRoute('settings')).toBe(true)
    })

    it('no longer matches any /admin route', () => {
        const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
        expect(router.resolve('/admin').matched.length).toBe(0)
        expect(router.resolve('/admin/libraries').matched.length).toBe(0)
        warnSpy.mockRestore()
    })
})
