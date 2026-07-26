import { describe, it, expect, beforeEach } from 'vitest'
import { computed } from 'vue'
import {
    coverVersion,
    bumpCoverVersion,
    resetCoverVersions,
    versionedCoverUrl
} from '@/composables/useCoverVersion'

beforeEach(() => resetCoverVersions())

describe('useCoverVersion', () => {
    it('starts every cover unversioned', () => {
        expect(coverVersion('ar-1')).toBe(0)
    })

    it('bumps one cover without touching the others', () => {
        bumpCoverVersion('ar-1')
        expect(coverVersion('ar-1')).toBe(1)
        expect(coverVersion('ar-2')).toBe(0)
    })

    // The whole point: the version must survive the component that bumped it
    // being unmounted, so navigating away and back still busts the browser's
    // in-memory image cache. Module state gives us that for free — this test
    // pins it so nobody moves it into a component ref.
    it('keeps the version across unrelated reads (module-level state)', () => {
        bumpCoverVersion('ar-1')
        bumpCoverVersion('ar-1')
        expect(coverVersion('ar-1')).toBe(2)
    })

    it('is reactive so a computed cover url re-evaluates on a bump', () => {
        const url = computed(() => versionedCoverUrl('base?id=ar-1', 'ar-1'))
        expect(url.value).toBe('base?id=ar-1')
        bumpCoverVersion('ar-1')
        expect(url.value).toBe('base?id=ar-1&_v=1')
    })

    it('leaves an unbumped url untouched', () => {
        expect(versionedCoverUrl('base?id=ar-9', 'ar-9')).toBe('base?id=ar-9')
    })
})
