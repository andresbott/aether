import { describe, it, expect, vi, beforeEach } from 'vitest'
import { ref } from 'vue'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

const results = ref<any[]>([])
const loading = ref(false)
const searchError = ref<string | null>(null)
const search = vi.fn()
vi.mock('@/composables/useMusicBrainzSearch', () => ({
    useMusicBrainzSearch: () => ({ results, loading, error: searchError, search })
}))

const setArtistImageFromSearch = vi.fn(() => Promise.resolve())
vi.mock('@/lib/api/Artists', () => ({
    artistImagePreviewUrl: (mbid: string) => `/api/v1/artists/image-preview?mbid=${mbid}`,
    setArtistImageFromSearch: (...a: unknown[]) => setArtistImageFromSearch(...(a as [])),
    parseArtistNumericId: (id: string) => Number(id.split('-').pop())
}))

import ArtistImageSearchDialog from '@/components/library/ArtistImageSearchDialog.vue'

const CANDIDATES = [
    { mbid: 'mbid-a', name: 'Pink Floyd', type: 'Group', disambiguation: '', lifeSpanBegin: '1965', lifeSpanEnd: '', score: 100 },
    { mbid: 'mbid-b', name: 'Pink Floyd Tribute', type: 'Group', disambiguation: 'tribute act', lifeSpanBegin: '', lifeSpanEnd: '', score: 60 }
]

const mountDialog = (props: Record<string, unknown> = {}) =>
    mount(ArtistImageSearchDialog, {
        props: { visible: true, artistId: 'ar-63', artistName: 'Pink Floyd', ...props },
        global: { plugins: [PrimeVue], directives: { tooltip: {} }, stubs: { teleport: true } }
    })

beforeEach(() => {
    results.value = []
    loading.value = false
    searchError.value = null
    search.mockClear()
    setArtistImageFromSearch.mockClear()
})

describe('ArtistImageSearchDialog', () => {
    it('searches for the artist name when opened', () => {
        mountDialog()
        expect(search).toHaveBeenCalledWith('Pink Floyd')
    })

    it('lists the MusicBrainz candidates', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        const rows = w.findAll('.result-row')
        expect(rows).toHaveLength(2)
        expect(rows[0].text()).toContain('Pink Floyd')
        expect(rows[1].text()).toContain('tribute act')
    })

    it('shows no image preview until a candidate is picked', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        expect(w.find('.image-preview img').exists()).toBe(false)
    })

    it('previews the picked candidate\'s image', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[0].trigger('click')

        const img = w.find('.image-preview img')
        expect(img.exists()).toBe(true)
        expect(img.attributes('src')).toContain('mbid=mbid-a')
    })

    // Nothing is stored until the user confirms the previewed image.
    it('disables Save until a candidate is picked', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeDefined()

        await w.findAll('.result-row')[0].trigger('click')
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeUndefined()
    })

    it('saves the picked mbid for the artist and closes', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[1].trigger('click')
        await w.find('[data-test="image-search-save"]').trigger('click')
        await flushPromises()

        expect(setArtistImageFromSearch).toHaveBeenCalledWith(63, 'mbid-b')
        expect(w.emitted('saved')).toHaveLength(1)
        expect(w.emitted('update:visible')?.at(-1)).toEqual([false])
    })

    it('does not save when nothing is picked', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.find('[data-test="image-search-save"]').trigger('click')
        expect(setArtistImageFromSearch).not.toHaveBeenCalled()
    })

    // A provider with no image for the candidate makes the <img> fail to load;
    // the dialog has to say so rather than showing a broken image and letting
    // the user save nothing.
    it('reports when the provider has no image for the pick', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[0].trigger('click')

        await w.find('.image-preview img').trigger('error')
        expect(w.find('.preview-error').exists()).toBe(true)
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeDefined()
    })

    it('surfaces a search error', async () => {
        searchError.value = 'MusicBrainz is unavailable'
        const w = mountDialog()
        await flushPromises()
        expect(w.text()).toContain('MusicBrainz is unavailable')
    })

    it('cancel closes without saving', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[0].trigger('click')
        await w.find('[data-test="image-search-cancel"]').trigger('click')

        expect(setArtistImageFromSearch).not.toHaveBeenCalled()
        expect(w.emitted('update:visible')?.at(-1)).toEqual([false])
    })
})
