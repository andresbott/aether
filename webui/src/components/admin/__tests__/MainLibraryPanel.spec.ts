import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import PrimeVue from 'primevue/config'
import type { CatalogSettings } from '@/types/libraries'

const settingsRef = ref<CatalogSettings | undefined>({ split_views: true, show_groupings: false })
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
    settingsRef.value = { split_views: true, show_groupings: false }
    isError.value = false
    mutate.mockReset()
})

describe('MainLibraryPanel', () => {
    it('shows the stored layout', () => {
        const w = mount(MainLibraryPanel, { global: { plugins: [PrimeVue] } })
        expect(radios(w).map((r) => r.element.checked)).toEqual([false, true])
    })

    it('saves the picked layout straight away', async () => {
        const w = mount(MainLibraryPanel, { global: { plugins: [PrimeVue] } })
        await radios(w)[0].setValue(true)
        expect(mutate).toHaveBeenCalledWith({ split_views: false })
    })

    it('shows and saves the Groupings switch', async () => {
        const w = mount(MainLibraryPanel, { global: { plugins: [PrimeVue] } })
        const box = w.find<HTMLInputElement>('#main-library-groupings')
        expect(box.element.checked).toBe(false)
        await box.setValue(true)
        expect(mutate).toHaveBeenCalledWith({ show_groupings: true })

        mutate.mockReset()
        settingsRef.value = { split_views: true, show_groupings: true }
        const on = mount(MainLibraryPanel, { global: { plugins: [PrimeVue] } })
        expect(on.find<HTMLInputElement>('#main-library-groupings').element.checked).toBe(true)
    })

    it('says so when the settings cannot be loaded', () => {
        settingsRef.value = undefined
        isError.value = true
        const w = mount(MainLibraryPanel, { global: { plugins: [PrimeVue] } })
        expect(w.find('[data-test="main-library-error"]').exists()).toBe(true)
        expect(radios(w)).toHaveLength(0)
    })
})
