import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import type { CatalogSettings } from '@/types/libraries'

const settingsRef = ref<CatalogSettings | undefined>({ split_views: true })
const isError = ref(false)
const mutate = vi.fn()
vi.mock('@/composables/useLibraries', () => ({
    useCatalogSettings: () => ({ data: settingsRef, isLoading: ref(false), isError }),
    useUpdateCatalogSettings: () => ({ mutate, isPending: ref(false) })
}))

import MainLibraryPanel from '@/components/admin/MainLibraryPanel.vue'

const radios = (w: ReturnType<typeof mount>) =>
    w.findAll<HTMLInputElement>('input[name="main-library-sidebar"]')

beforeEach(() => {
    settingsRef.value = { split_views: true }
    isError.value = false
    mutate.mockReset()
})

describe('MainLibraryPanel', () => {
    it('shows the stored layout', () => {
        const w = mount(MainLibraryPanel)
        expect(radios(w).map((r) => r.element.checked)).toEqual([false, true])
    })

    it('saves the picked layout straight away', async () => {
        const w = mount(MainLibraryPanel)
        await radios(w)[0].setValue(true)
        expect(mutate).toHaveBeenCalledWith({ split_views: false })
    })

    it('says so when the settings cannot be loaded', () => {
        settingsRef.value = undefined
        isError.value = true
        const w = mount(MainLibraryPanel)
        expect(w.find('[data-test="main-library-error"]').exists()).toBe(true)
        expect(radios(w)).toHaveLength(0)
    })
})
