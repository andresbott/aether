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

const mountRow = (over: Partial<Song> = {}) =>
    mount(GenreTrackRow, {
        props: { song: song(over), index: 0 },
        global: { stubs: { RouterLink: RouterLinkStub } }
    })

beforeEach(() => {
    starMutate.mockReset()
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

    // The row selects on click and plays on double-click, so the heart must
    // swallow both or starring would also select or start playback.
    it('does not select or play the row when the heart is used', async () => {
        const w = mountRow()
        await w.find('.row-star').trigger('click')
        await w.find('.row-star').trigger('dblclick')
        expect(w.emitted('select')).toBeUndefined()
        expect(w.emitted('play')).toBeUndefined()
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
