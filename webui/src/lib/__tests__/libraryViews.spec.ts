import { describe, it, expect } from 'vitest'
import { ALL_LIBRARY_VIEWS, inDisplayOrder, openingView } from '@/lib/libraryViews'

describe('library views', () => {
    it('lists every view in the sidebar order', () => {
        expect(ALL_LIBRARY_VIEWS).toEqual(['discover', 'artists', 'albums'])
    })

    it('puts any set of views in display order, each once', () => {
        expect(inDisplayOrder(['albums', 'discover'])).toEqual(['discover', 'albums'])
        expect(inDisplayOrder(['artists', 'artists'])).toEqual(['artists'])
        expect(inDisplayOrder([])).toEqual([])
    })

    it('opens on the default when the library offers it', () => {
        expect(openingView(['artists', 'albums'], 'albums')).toBe('albums')
    })

    it('opens on the first view offered when the default is missing or not offered', () => {
        expect(openingView(['albums', 'artists'])).toBe('artists')
        expect(openingView(['artists', 'albums'], 'discover')).toBe('artists')
    })

    it('has nothing to open on when no view is offered', () => {
        expect(openingView([], 'discover')).toBeUndefined()
    })
})
