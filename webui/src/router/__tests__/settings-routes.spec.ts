import { describe, it, expect } from 'vitest'
import router from '@/router'

describe('settings routes', () => {
    it('resolves each settings topic to its named route', () => {
        expect(router.resolve('/settings/profile').name).toBe('settings-profile')
        expect(router.resolve('/settings/libraries').name).toBe('settings-libraries')
        expect(router.resolve('/settings/tasks').name).toBe('settings-tasks')
        expect(router.resolve('/settings/metadata').name).toBe('settings-metadata')
    })

    it('applies the settings layout to settings routes', () => {
        expect(router.resolve('/settings/libraries').meta.layout).toBe('settings')
    })

    it('no longer matches any /admin route', () => {
        expect(router.resolve('/admin').matched.length).toBe(0)
        expect(router.resolve('/admin/libraries').matched.length).toBe(0)
    })
})
