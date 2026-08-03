import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'

const starMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: starMutate })
}))

import TrackFavoriteButton from '@/components/library/TrackFavoriteButton.vue'
import type { Song } from '@/types/subsonic'

const song = (over: Partial<Song> = {}): Song =>
    ({ id: 's1', title: 'Song One', artist: 'The Artist', ...over }) as Song

const mountButton = (over: Partial<Song> = {}) =>
    mount(TrackFavoriteButton, { props: { song: song(over) } })

beforeEach(() => {
    starMutate.mockReset()
})

describe('TrackFavoriteButton', () => {
    it('shows an outline heart when unstarred and a filled one when starred', () => {
        expect(mountButton().find('i').classes()).toContain('pi-heart')
        expect(mountButton({ starred: '2026-02-01T00:00:00Z' }).find('i').classes()).toContain(
            'pi-heart-fill'
        )
    })

    it('labels the toggle by the current state', () => {
        expect(mountButton().attributes('aria-label')).toBe('Add to favorites')
        expect(mountButton({ starred: '2026-02-01T00:00:00Z' }).attributes('aria-label')).toBe(
            'Remove from favorites'
        )
    })

    it('keeps a starred heart visible without hover', () => {
        expect(mountButton({ starred: '2026-02-01T00:00:00Z' }).classes()).toContain('is-starred')
        expect(mountButton().classes()).not.toContain('is-starred')
    })

    it('stars an unstarred song', async () => {
        const w = mountButton()
        await w.trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 's1', starred: false })
    })

    it('unstars a starred song', async () => {
        const w = mountButton({ starred: '2026-02-01T00:00:00Z' })
        await w.trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 's1', starred: true })
    })

    // Rows are click-to-select and sometimes wrapped in a router-link, so an
    // unswallowed click would select the row or navigate instead of starring.
    it('swallows the click so the host row neither selects nor navigates', () => {
        const w = mountButton()
        const click = new MouseEvent('click', { bubbles: true, cancelable: true })
        w.element.dispatchEvent(click)
        expect(click.defaultPrevented).toBe(true)
    })

    // Track rows play on double-click; two fast clicks on the heart must not
    // start playback.
    it('swallows a double-click so the host row does not start playing', () => {
        const w = mountButton()
        const dbl = new MouseEvent('dblclick', { bubbles: true, cancelable: true })
        w.element.dispatchEvent(dbl)
        expect(dbl.defaultPrevented).toBe(true)
    })

    it('does nothing without a song', async () => {
        const w = mount(TrackFavoriteButton, { props: {} })
        await w.trigger('click')
        expect(starMutate).not.toHaveBeenCalled()
    })
})
