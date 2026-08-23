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

const CANDIDATES = [
    {
        mbid: 'mbid-a',
        name: 'Pink Floyd',
        type: 'Group',
        disambiguation: '',
        lifeSpanBegin: '1965',
        lifeSpanEnd: '',
        score: 100
    },
    {
        mbid: 'mbid-b',
        name: 'Pink Floyd Tribute',
        type: 'Group',
        disambiguation: 'tribute act',
        lifeSpanBegin: '',
        lifeSpanEnd: '',
        score: 60
    }
]

const IMAGE_CANDIDATES = [
    { url: 'https://cdn/full-a.jpg', thumbUrl: 'https://cdn/thumb-a.jpg', provider: 'fanart.tv' },
    { url: 'https://cdn/full-b.jpg', thumbUrl: 'https://cdn/thumb-b.jpg', provider: 'theaudiodb' }
]

// The dialog only *stages* a pick — the write happens on the view's main Save —
// so it must not touch the store-the-image endpoint at all.
const setArtistImageFromSearch = vi.fn(() => Promise.resolve())
// useArtistImageCandidates (unlike useMusicBrainzSearch) has no spec of its
// own, so it runs for real here — only the API call underneath it is stubbed.
const getArtistImageCandidates = vi.fn((_mbid: string) => Promise.resolve(IMAGE_CANDIDATES))
vi.mock('@/lib/api/Artists', () => ({
    getArtistImageCandidates: (...a: unknown[]) => getArtistImageCandidates(...(a as [string])),
    setArtistImageFromSearch: (...a: unknown[]) => setArtistImageFromSearch(...(a as [])),
    parseArtistNumericId: (id: string) => Number(id.split('-').pop())
}))

import ArtistImageSearchDialog from '@/components/library/ArtistImageSearchDialog.vue'

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
    getArtistImageCandidates.mockClear()
    getArtistImageCandidates.mockResolvedValue(IMAGE_CANDIDATES)
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

    it('shows no image grid until a candidate is picked', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        expect(w.find('[data-test="candidate-thumb"]').exists()).toBe(false)
        expect(getArtistImageCandidates).not.toHaveBeenCalled()
    })

    it('loads and renders the image candidates for the picked artist', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[0].trigger('click')
        await flushPromises()

        expect(getArtistImageCandidates).toHaveBeenCalledWith('mbid-a')
        const thumbs = w.findAll('[data-test="candidate-thumb"]')
        expect(thumbs).toHaveLength(2)
        expect(thumbs[0].attributes('src')).toBe('https://cdn/thumb-a.jpg')
        expect(thumbs[1].attributes('src')).toBe('https://cdn/thumb-b.jpg')
    })

    // Nothing is staged until the user confirms a picked thumbnail.
    it('disables the confirm button until an image thumbnail is picked', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeDefined()

        await w.findAll('.result-row')[0].trigger('click')
        await flushPromises()
        // An artist is picked, but no image yet.
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeDefined()

        await w.findAll('[data-test="candidate-thumb"]')[0].trigger('click')
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeUndefined()
    })

    // Confirming hands the pick to the parent to stage — it must NOT write to the
    // server, so a Cancel on the main editor discards it like any other edit.
    it("emits the chosen thumbnail's url and closes without storing anything", async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[1].trigger('click')
        await flushPromises()
        await w.findAll('[data-test="candidate-thumb"]')[1].trigger('click')
        await w.find('[data-test="image-search-save"]').trigger('click')
        await flushPromises()

        expect(setArtistImageFromSearch).not.toHaveBeenCalled()
        expect(w.emitted('select')?.[0]).toEqual([
            {
                mbid: 'mbid-b',
                name: 'Pink Floyd Tribute',
                url: 'https://cdn/full-b.jpg',
                previewUrl: 'https://cdn/thumb-b.jpg'
            }
        ])
        expect(w.emitted('update:visible')?.at(-1)).toEqual([false])
    })

    it('emits nothing when no candidate is picked', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.find('[data-test="image-search-save"]').trigger('click')
        expect(w.emitted('select')).toBeUndefined()
    })

    // An empty candidate list means the providers hold no image for the pick;
    // the dialog has to say so rather than showing an empty grid.
    it('reports when no images are available for the pick', async () => {
        getArtistImageCandidates.mockResolvedValueOnce([])
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[0].trigger('click')
        await flushPromises()

        expect(w.find('.image-section .no-results').exists()).toBe(true)
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeDefined()
    })

    it('surfaces a search error', async () => {
        searchError.value = 'MusicBrainz is unavailable'
        const w = mountDialog()
        await flushPromises()
        expect(w.text()).toContain('MusicBrainz is unavailable')
    })

    it('surfaces an image lookup error', async () => {
        getArtistImageCandidates.mockRejectedValueOnce(new Error('Image lookup exploded'))
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[0].trigger('click')
        await flushPromises()

        expect(w.text()).toContain('Image lookup exploded')
        expect(w.find('[data-test="image-search-save"]').attributes('disabled')).toBeDefined()
    })

    it('cancel closes without emitting a pick', async () => {
        results.value = CANDIDATES
        const w = mountDialog()
        await flushPromises()
        await w.findAll('.result-row')[0].trigger('click')
        await w.find('[data-test="image-search-cancel"]').trigger('click')

        expect(w.emitted('select')).toBeUndefined()
        expect(setArtistImageFromSearch).not.toHaveBeenCalled()
        expect(w.emitted('update:visible')?.at(-1)).toEqual([false])
    })
})
