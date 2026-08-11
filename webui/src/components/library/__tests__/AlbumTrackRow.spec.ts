import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'

// The row's favorite toggle mutates through vue-query; stub it so the row needs
// no VueQueryPlugin. `TrackFavoriteButton` has its own spec for the toggle.
const starMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: starMutate })
}))

import AlbumTrackRow from '@/components/library/AlbumTrackRow.vue'
import type { Song } from '@/types/subsonic'

const song = { id: 's1', title: 'Song One', artist: 'The Artist', duration: 125, track: 4 } as Song

const mountRow = (props: Partial<{ selected: boolean }> = {}, over: Partial<Song> = {}) =>
    mount(AlbumTrackRow, {
        props: { song: { ...song, ...over }, index: 0, ...props },
        global: { directives: { tooltip: {} } }
    })

beforeEach(() => {
    starMutate.mockReset()
})

describe('AlbumTrackRow', () => {
    it('is draggable and shows track number, title, artist and duration columns', () => {
        const w = mountRow()
        expect(w.attributes('draggable')).toBe('true')
        expect(w.find('.track-number').text()).toBe('4')
        expect(w.find('.col-title').text()).toBe('Song One')
        expect(w.find('.col-artist').text()).toBe('The Artist')
        expect(w.find('.row-duration').text()).toBe('2:05')
    })

    it('does not render a hover play button', () => {
        expect(mountRow().find('.play-hover-icon').exists()).toBe(false)
    })

    it('applies the selected class when selected', () => {
        expect(mountRow({ selected: true }).classes()).toContain('selected')
        expect(mountRow().classes()).not.toContain('selected')
    })

    it('emits select with plain modifiers on a plain click', async () => {
        const w = mountRow()
        await w.trigger('click')
        expect(w.emitted('select')?.[0]).toEqual([{ additive: false, range: false }])
    })

    it('maps ctrl/meta to additive and shift to range', async () => {
        const w = mountRow()
        await w.trigger('click', { metaKey: true })
        await w.trigger('click', { shiftKey: true })
        expect(w.emitted('select')?.[0]).toEqual([{ additive: true, range: false }])
        expect(w.emitted('select')?.[1]).toEqual([{ additive: false, range: true }])
    })

    it('emits enqueue on double-click', async () => {
        const w = mountRow()
        await w.trigger('dblclick')
        expect(w.emitted('enqueue')).toHaveLength(1)
    })

    it('forwards dragstart and dragend', async () => {
        const w = mountRow()
        await w.trigger('dragstart')
        await w.trigger('dragend')
        expect(w.emitted('dragstart')).toHaveLength(1)
        expect(w.emitted('dragend')).toHaveLength(1)
    })
})

describe('AlbumTrackRow favorite toggle', () => {
    it('renders the shared favorite toggle in its own column', () => {
        const w = mountRow()
        expect(w.find('.col-star .row-star').exists()).toBe(true)
    })

    it('toggles the song with its current state', async () => {
        const w = mountRow({}, { starred: '2026-02-01T00:00:00Z' })
        await w.find('.row-star').trigger('click')
        expect(starMutate).toHaveBeenCalledWith({ id: 's1', starred: true })
    })

    // The row selects on click and enqueues on double-click, so the heart must
    // swallow both or starring would also select or queue the track.
    it('does not select or enqueue the row when the heart is used', async () => {
        const w = mountRow()
        await w.find('.row-star').trigger('click')
        await w.find('.row-star').trigger('dblclick')
        expect(w.emitted('select')).toBeUndefined()
        expect(w.emitted('enqueue')).toBeUndefined()
    })
})
