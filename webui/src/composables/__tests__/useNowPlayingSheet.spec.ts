import { describe, it, expect, vi, beforeEach } from 'vitest'
import type { Router } from 'vue-router'
import {
    commitDetent,
    detentForHash,
    hashForDetent,
    resetNowPlayingSheetForTests,
    useNowPlayingSheet
} from '@/composables/useNowPlayingSheet'

beforeEach(() => {
    resetNowPlayingSheetForTests()
    // vue-router keeps the previous entry's fullPath in history.state.back;
    // commitDetent reads it to choose back() over replace(). Reset per test.
    window.history.replaceState({}, '', '/library')
})

describe('hash mapping', () => {
    it('maps the three sheet addresses both ways', () => {
        expect(detentForHash('')).toBe('collapsed')
        expect(detentForHash('#playing')).toBe('playing')
        expect(detentForHash('#queue')).toBe('queue')
        expect(hashForDetent('collapsed')).toBe('')
        expect(hashForDetent('playing')).toBe('#playing')
        expect(hashForDetent('queue')).toBe('#queue')
    })

    it('reads any foreign hash as collapsed — the sheet only owns its own two', () => {
        expect(detentForHash('#section-2')).toBe('collapsed')
    })
})

describe('useNowPlayingSheet', () => {
    it('is a singleton starting collapsed', () => {
        const a = useNowPlayingSheet()
        const b = useNowPlayingSheet()
        expect(a).toBe(b)
        expect(a.detent.value).toBe('collapsed')
        expect(a.position.value).toBe(0)
        expect(a.open.value).toBe(false)
    })

    it('snapTo moves detent and position together and clears dragging', () => {
        const sheet = useNowPlayingSheet()
        sheet.dragging.value = true
        sheet.snapTo('queue')
        expect(sheet.detent.value).toBe('queue')
        expect(sheet.position.value).toBe(2)
        expect(sheet.dragging.value).toBe(false)
        expect(sheet.open.value).toBe(true)
    })

    it('reset returns to collapsed', () => {
        const sheet = useNowPlayingSheet()
        sheet.snapTo('playing')
        sheet.reset()
        expect(sheet.detent.value).toBe('collapsed')
        expect(sheet.position.value).toBe(0)
    })
})

describe('commitDetent', () => {
    const makeRouter = () => {
        const router = {
            push: vi.fn(),
            replace: vi.fn(),
            back: vi.fn(),
            resolve: vi.fn(({ hash }: { hash: string }) => ({ fullPath: `/library${hash}` }))
        }
        return router as unknown as Router & typeof router
    }

    it('pushes when going deeper, so back can walk the chain down', () => {
        const router = makeRouter()
        commitDetent(router, 'collapsed', 'playing')
        expect(router.push).toHaveBeenCalledWith({ hash: '#playing' })
        commitDetent(router, 'playing', 'queue')
        expect(router.push).toHaveBeenCalledWith({ hash: '#queue' })
        expect(router.back).not.toHaveBeenCalled()
    })

    it('pops the matching entry when going shallower — repeated swipes must not grow history', () => {
        const router = makeRouter()
        window.history.replaceState({ back: '/library#playing' }, '', '/library#queue')
        commitDetent(router, 'queue', 'playing')
        expect(router.back).toHaveBeenCalledOnce()
        expect(router.replace).not.toHaveBeenCalled()
    })

    it('rewrites in place when there is no matching entry (deep link / reload)', () => {
        const router = makeRouter()
        window.history.replaceState({ back: null }, '', '/library#playing')
        commitDetent(router, 'playing', 'collapsed')
        expect(router.replace).toHaveBeenCalledWith({ hash: '' })
        expect(router.back).not.toHaveBeenCalled()
    })

    it('never backs into a DIFFERENT page — only the exact shallower address counts', () => {
        const router = makeRouter()
        window.history.replaceState({ back: '/album/9' }, '', '/library#playing')
        commitDetent(router, 'playing', 'collapsed')
        expect(router.replace).toHaveBeenCalledWith({ hash: '' })
        expect(router.back).not.toHaveBeenCalled()
    })

    it('does nothing for a same-detent settle (spring-back)', () => {
        const router = makeRouter()
        commitDetent(router, 'playing', 'playing')
        expect(router.push).not.toHaveBeenCalled()
        expect(router.replace).not.toHaveBeenCalled()
        expect(router.back).not.toHaveBeenCalled()
    })
})
