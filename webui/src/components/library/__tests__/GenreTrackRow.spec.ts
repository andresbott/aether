import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'

const starMutate = vi.fn()
vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: starMutate })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

import GenreTrackRow from '@/components/library/GenreTrackRow.vue'
import type { Song } from '@/types/subsonic'

const song = (over: Partial<Song> = {}): Song =>
    ({
        id: 's1',
        title: 'Song One',
        artist: 'The Artist',
        album: 'The Album',
        duration: 125,
        ...over
    }) as Song

const mountRow = (over: Partial<Song> = {}, props: Partial<{ selected: boolean }> = {}) =>
    mount(GenreTrackRow, {
        props: { song: song(over), index: 0, ...props },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

beforeEach(() => {
    starMutate.mockReset()
})

describe('GenreTrackRow select toggle', () => {
    it('renders the shared select toggle in its own column, left of the heart', () => {
        const w = mountRow()
        expect(w.find('.col-select .row-select').exists()).toBe(true)
        const cols = w.findAll('.genre-track-row > span').map((s) => s.classes())
        expect(cols.findIndex((c) => c.includes('col-select'))).toBeLessThan(
            cols.findIndex((c) => c.includes('col-star'))
        )
    })

    it('reflects the row selection', () => {
        expect(mountRow({}, { selected: true }).find('.row-select').classes()).toContain(
            'is-selected'
        )
        expect(mountRow().find('.row-select').classes()).not.toContain('is-selected')
    })

    // Clicking it must behave exactly like a CTRL/⌘+click on the row.
    it('emits an additive select and does not enqueue the row', async () => {
        const w = mountRow()
        await w.find('.row-select').trigger('click')
        await w.find('.row-select').trigger('dblclick')
        expect(w.emitted('select')).toEqual([[{ additive: true, range: false }]])
        expect(w.emitted('enqueue')).toBeUndefined()
    })

    it('renders no select toggle on the placeholder row', () => {
        const w = mount(GenreTrackRow, {
            props: { index: 0 },
            global: { stubs: { RouterLink: RouterLinkStub } }
        })
        expect(w.find('.row-select').exists()).toBe(false)
    })
})

describe('GenreTrackRow favorite toggle', () => {
    it('renders the shared favorite toggle in its own column', () => {
        expect(mountRow().find('.col-star .row-star').exists()).toBe(true)
    })

    it('toggles the song with its current state', async () => {
        const w = mountRow({ starred: '2026-02-01T00:00:00Z' })
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

    it('renders no favorite toggle on the placeholder row', () => {
        const w = mount(GenreTrackRow, {
            props: { index: 0 },
            global: { stubs: { RouterLink: RouterLinkStub } }
        })
        expect(w.find('.genre-track-row.placeholder').exists()).toBe(true)
        expect(w.find('.row-star').exists()).toBe(false)
    })
})
