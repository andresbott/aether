import { describe, it, expect } from 'vitest'
import { queryKeys } from '@/composables/useSubsonicQueries'

describe('queryKeys release-type awareness', () => {
    it('keys albumList pages by releaseType', () => {
        const a = queryKeys.albumList('alphabeticalByName', 100, 0, undefined, 'Single')
        const b = queryKeys.albumList('alphabeticalByName', 100, 0, undefined, undefined)
        expect(a).not.toEqual(b)
        expect(a).toContain('Single')
    })

    it('exposes an albumIndex key that varies by releaseType', () => {
        const a = queryKeys.albumIndex(undefined, 'EP')
        const b = queryKeys.albumIndex(undefined, undefined)
        expect(a).not.toEqual(b)
    })
})
