import { describe, it, expect, vi } from 'vitest'
import { ref } from 'vue'
import { mount } from '@vue/test-utils'

const albums = ref<Array<{ id: string; name: string }>>([{ id: 'al1', name: 'A' }])
const isLoading = ref(false)
vi.mock('@/composables/useSubsonicQueries', () => ({
    useAlbumList: () => ({ data: albums, isLoading })
}))

import AlbumGrid from '@/components/library/AlbumGrid.vue'

const mountGrid = () =>
    mount(AlbumGrid, { props: { folderId: 1 }, global: { stubs: { AlbumCard: true } } })

describe('AlbumGrid', () => {
    it('renders an AlbumCard per album', () => {
        albums.value = [{ id: 'al1', name: 'A' }, { id: 'al2', name: 'B' }]
        isLoading.value = false
        const w = mountGrid()
        expect(w.findAll('album-card-stub')).toHaveLength(2)
    })

    it('shows an empty state when there are no albums', () => {
        albums.value = []
        isLoading.value = false
        const w = mountGrid()
        expect(w.text()).toContain('No albums found')
    })
})
