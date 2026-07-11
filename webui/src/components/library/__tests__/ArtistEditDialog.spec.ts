import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import PrimeVue from 'primevue/config'

vi.mock('primevue/dialog', () => ({
    default: defineComponent({
        props: ['visible', 'header', 'modal', 'style'],
        emits: ['update:visible'],
        setup(_props, { slots }) {
            return () =>
                h('div', { class: 'p-dialog' }, [
                    h('div', { class: 'p-dialog-content' }, slots.default?.()),
                    h('div', { class: 'p-dialog-footer' }, slots.footer?.())
                ])
        }
    })
}))

const searchResults = ref<any[]>([])
const searchFn = vi.fn()
vi.mock('@/composables/useMusicBrainzSearch', () => ({
    useMusicBrainzSearch: () => ({
        results: searchResults,
        loading: ref(false),
        error: ref(null),
        search: searchFn
    })
}))

const mutateFn = vi.fn()
vi.mock('@/composables/useArtistMbid', () => ({
    useSetArtistMBID: () => ({
        mutate: mutateFn,
        isPending: ref(false)
    })
}))

const getArtistMBIDMock = vi.fn().mockResolvedValue('')
vi.mock('@/lib/api/Artists', () => ({
    getArtistMBID: (...args: unknown[]) => getArtistMBIDMock(...args),
    parseArtistNumericId: (id: string) => Number(id.split('-').pop())
}))

import ArtistEditDialog from '@/components/library/ArtistEditDialog.vue'

const mountDialog = (props: Record<string, unknown> = {}) =>
    mount(ArtistEditDialog, {
        props: { visible: true, artistId: 'ar-1', artistName: 'Nirvana', ...props },
        global: { plugins: [PrimeVue] }
    })

beforeEach(() => {
    searchResults.value = []
    searchFn.mockClear()
    mutateFn.mockClear()
    getArtistMBIDMock.mockReset().mockResolvedValue('')
})

describe('ArtistEditDialog', () => {
    it('pre-fills the search box with the artist name and searches on open', async () => {
        const w = mountDialog()
        await flushPromises()
        expect(getArtistMBIDMock).toHaveBeenCalledWith(1)
        expect(searchFn).toHaveBeenCalledWith('Nirvana')
        expect((w.find('input').element as HTMLInputElement).value).toBe('Nirvana')
    })

    it('disables Save until a candidate is selected', async () => {
        const w = mountDialog()
        await flushPromises()
        const saveBtn = w.findAll('button').find((b) => b.text() === 'Save')
        expect(saveBtn?.attributes('disabled')).toBeDefined()
    })

    it('selecting a result enables Save and clicking it saves the new mbid', async () => {
        searchResults.value = [
            { mbid: 'abc-123', name: 'Nirvana', type: 'Group', disambiguation: '', lifeSpanBegin: '', lifeSpanEnd: '', score: 100 }
        ]
        const w = mountDialog()
        await flushPromises()

        await w.find('.result-row').trigger('click')
        const saveBtn = w.findAll('button').find((b) => b.text() === 'Save')
        expect(saveBtn?.attributes('disabled')).toBeUndefined()

        await saveBtn?.trigger('click')
        expect(mutateFn).toHaveBeenCalledWith(
            { numericId: 1, mbid: 'abc-123' },
            expect.objectContaining({ onSuccess: expect.any(Function) })
        )
    })

    it('shows a deprecation note pointing at the metadata editor', async () => {
        const w = mountDialog()
        await flushPromises()
        expect(w.text()).toContain('Deprecated')
        expect(w.text()).toContain('Metadata Editor')
    })

    it('shows the current match and stages a clear when the x is clicked', async () => {
        getArtistMBIDMock.mockResolvedValue('existing-mbid')
        const w = mountDialog()
        await flushPromises()

        expect(w.find('.current-match').exists()).toBe(true)
        await w.find('.clear-btn').trigger('click')

        const saveBtn = w.findAll('button').find((b) => b.text() === 'Save')
        expect(saveBtn?.attributes('disabled')).toBeUndefined()

        await saveBtn?.trigger('click')
        expect(mutateFn).toHaveBeenCalledWith(
            { numericId: 1, mbid: '' },
            expect.objectContaining({ onSuccess: expect.any(Function) })
        )
    })
})
