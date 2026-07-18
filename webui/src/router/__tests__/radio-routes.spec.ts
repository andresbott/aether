import { describe, it, expect } from 'vitest'
import router from '@/router'

describe('radio routes', () => {
    it('resolves /radio/new to the create route with the create prop', () => {
        const r = router.resolve('/radio/new')
        expect(r.name).toBe('radio-station-new')
        expect(r.meta.flush).toBe(true)
    })

    it('resolves /radio/:id to the detail route with the id param', () => {
        const r = router.resolve('/radio/abc123')
        expect(r.name).toBe('radio-station-detail')
        expect(r.params.id).toBe('abc123')
    })

    it('prefers /radio/new over the :id param', () => {
        expect(router.resolve('/radio/new').name).toBe('radio-station-new')
    })
})
