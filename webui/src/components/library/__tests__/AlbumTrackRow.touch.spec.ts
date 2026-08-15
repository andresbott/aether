import { describe, it, expect, vi } from 'vitest'
import { mount, RouterLinkStub } from '@vue/test-utils'
import { ref } from 'vue'
import AlbumTrackRow from '../AlbumTrackRow.vue'
import GenreTrackRow from '../GenreTrackRow.vue'
import type { Song } from '@/types/subsonic'

const isTouch = ref(true)
vi.mock('@/composables/useViewport', () => ({
    useViewport: () => ({ isTouch, tier: ref('phone'), shell: ref('mobile') })
}))

vi.mock('@/composables/useSubsonicQueries', () => ({
    useToggleStar: () => ({ mutate: vi.fn() })
}))

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { isConfigured: () => false, getCoverArtUrl: () => '' }
}))

const SONG = { id: 'tr-1', title: 'T', artist: 'A' } as Song

describe.each([
    ['AlbumTrackRow', AlbumTrackRow, '.album-track-row', {}],
    ['GenreTrackRow', GenreTrackRow, '.genre-track-row', { RouterLink: RouterLinkStub }]
])('%s touch contract', (_name, Row, rootSel, stubs) => {
    const mountRow = () => mount(Row, {
        props: { song: SONG, index: 0 } as any,
        global: { stubs }
    })

    it('tap emits play, not select', async () => {
        isTouch.value = true
        const row = mountRow()
        await row.find(rootSel).trigger('click')
        expect(row.emitted('play')).toHaveLength(1)
        expect(row.emitted('select')).toBeUndefined()
    })

    it('shows the ⋮ menu button and emits menu without playing', async () => {
        isTouch.value = true
        const row = mountRow()
        const menu = row.find('[aria-label="Track actions"]')
        expect(menu.exists()).toBe(true)
        await menu.trigger('click')
        expect(row.emitted('menu')).toHaveLength(1)
        expect(row.emitted('play')).toBeUndefined()
    })

    it('keeps the pointer contract without touch', async () => {
        isTouch.value = false
        const row = mountRow()
        expect(row.find('[aria-label="Track actions"]').exists()).toBe(false)
        await row.find(rootSel).trigger('click')
        expect(row.emitted('select')).toHaveLength(1)
        expect(row.emitted('play')).toBeUndefined()
        await row.find(rootSel).trigger('dblclick')
        expect(row.emitted('enqueue')).toHaveLength(1)
    })
})
