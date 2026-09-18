import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PrimeVue from 'primevue/config'

vi.mock('primevue/usetoast', () => ({ useToast: () => ({ add: vi.fn() }) }))

const settingsData = vi.hoisted(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return { current: null as any }
})

const updateMutation = vi.hoisted(() => {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    return { current: null as any }
})

vi.mock('@/composables/useGeneratedCoverSettings', async () => {
    const { ref: vueRef } = await import('vue')
    updateMutation.current = { mutate: vi.fn(), isPending: vueRef(false) }
    return {
        useGeneratedCoverSettings: () => ({ data: vueRef(settingsData.current), isLoading: vueRef(false) }),
        useUpdateGeneratedCoverSettings: () => updateMutation.current
    }
})

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: {
        getGeneratedCoverPreviewUrl: vi.fn(({ style }) => `/generated/${style}.jpg`)
    }
}))

import GeneratedCoversPanel from '@/components/admin/GeneratedCoversPanel.vue'

beforeEach(() => {
    updateMutation.current.mutate.mockClear()
    updateMutation.current.isPending.value = false
})

const mountPanel = (data: { styles: { name: string; label: string }[], default: string[], available: string[] }) => {
    settingsData.current = data
    return mount(GeneratedCoversPanel, {
        global: {
            plugins: [PrimeVue]
        }
    })
}

describe('GeneratedCoversPanel', () => {
    it('renders two checkbox groups with the correct number of rows', async () => {
        const w = mountPanel({
            styles: [
                { name: 'classic', label: 'Classic' },
                { name: 'bauhaus', label: 'Bauhaus' }
            ],
            default: ['classic'],
            available: ['classic', 'bauhaus']
        })
        await flushPromises()

        // Should see both style labels in each group
        const text = w.text()
        expect(text).toContain('Classic')
        expect(text).toContain('Bauhaus')
        expect(text).toContain('Default (auto-generated placeholders)')
        expect(text).toContain('Available (offered when editing a cover)')

        // Should have 4 checkboxes total (2 per style)
        expect(w.findAll('input[type="checkbox"]')).toHaveLength(4)
    })

    it('checks the classic default box initially', async () => {
        const w = mountPanel({
            styles: [
                { name: 'classic', label: 'Classic' },
                { name: 'bauhaus', label: 'Bauhaus' }
            ],
            default: ['classic'],
            available: ['classic', 'bauhaus']
        })
        await flushPromises()

        const checkboxes = w.findAll('input[type="checkbox"]')
        // First checkbox should be classic in the Default group
        expect(checkboxes[0].element.checked).toBe(true)
        // Second should be bauhaus in Default (unchecked)
        expect(checkboxes[1].element.checked).toBe(false)
    })

    it('calls mutate with both names when bauhaus is checked in Default and Save is clicked', async () => {
        const w = mountPanel({
            styles: [
                { name: 'classic', label: 'Classic' },
                { name: 'bauhaus', label: 'Bauhaus' }
            ],
            default: ['classic'],
            available: ['classic', 'bauhaus']
        })
        await flushPromises()

        const checkboxes = w.findAll('input[type="checkbox"]')
        // Check bauhaus in Default group (second checkbox)
        await checkboxes[1].setValue(true)
        await flushPromises()

        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
        await saveBtn.trigger('click')
        await flushPromises()

        expect(updateMutation.current.mutate).toHaveBeenCalledWith(
            expect.objectContaining({
                default: expect.arrayContaining(['classic', 'bauhaus']),
                available: expect.arrayContaining(['classic', 'bauhaus'])
            })
        )
        // Check that default has exactly 2 elements
        const call = updateMutation.current.mutate.mock.calls[0][0]
        expect(call.default).toHaveLength(2)
    })

    it('disables Save when defaultSet is emptied', async () => {
        const w = mountPanel({
            styles: [
                { name: 'classic', label: 'Classic' },
                { name: 'bauhaus', label: 'Bauhaus' }
            ],
            default: ['classic'],
            available: ['classic', 'bauhaus']
        })
        await flushPromises()

        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
        expect(saveBtn.attributes('disabled')).toBeUndefined()

        const checkboxes = w.findAll('input[type="checkbox"]')
        // Uncheck classic in Default group (first checkbox)
        await checkboxes[0].setValue(false)
        await flushPromises()

        expect(saveBtn.attributes('disabled')).toBeDefined()
        expect(w.text()).toContain('Select at least one default style')
    })

    it('disables Save when mutation is pending', async () => {
        const w = mountPanel({
            styles: [
                { name: 'classic', label: 'Classic' }
            ],
            default: ['classic'],
            available: ['classic']
        })
        await flushPromises()

        const saveBtn = w.findAll('button').find((b) => b.text().includes('Save'))!
        expect(saveBtn.attributes('disabled')).toBeUndefined()

        updateMutation.current.isPending.value = true
        await flushPromises()

        expect(saveBtn.attributes('disabled')).toBeDefined()
    })
})
