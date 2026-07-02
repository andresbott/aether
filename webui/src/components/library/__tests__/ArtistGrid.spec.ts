import { describe, it, expect, vi } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'

const items = ref<Array<{ id: string; name: string }>>([])
const isLoading = ref(false)
vi.mock('@/composables/useArtistTable', () => ({
    useArtistTable: () => ({ items, isLoading, total: ref(items.value.length) })
}))

import ArtistGrid from '@/components/library/ArtistGrid.vue'

const mountGrid = () =>
    mount(ArtistGrid, { props: { folderId: 1 }, global: { stubs: { ArtistCard: true } } })

describe('ArtistGrid', () => {
    it('renders an ArtistCard per artist', () => {
        items.value = [{ id: 'ar1', name: 'A' }, { id: 'ar2', name: 'B' }]
        isLoading.value = false
        const w = mountGrid()
        expect(w.findAll('artist-card-stub')).toHaveLength(2)
    })

    it('shows an empty state when there are no artists', () => {
        items.value = []
        isLoading.value = false
        const w = mountGrid()
        expect(w.text()).toContain('No artists found')
    })
})
