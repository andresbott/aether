import { describe, it, expect, vi, beforeEach } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { VueQueryPlugin, QueryClient } from '@tanstack/vue-query'
import type { GeneratedCoverSettings, GeneratedCoverSettingsInput } from '@/types/generatedCovers'

const getGeneratedCoverSettingsMock = vi.fn()
const updateGeneratedCoverSettingsMock = vi.fn()

vi.mock('@/lib/api/GeneratedCovers', () => ({
    getGeneratedCoverSettings: (...args: unknown[]) => getGeneratedCoverSettingsMock(...args),
    updateGeneratedCoverSettings: (...args: unknown[]) => updateGeneratedCoverSettingsMock(...args)
}))

const toastAdd = vi.hoisted(() => vi.fn())
vi.mock('primevue/usetoast', () => ({
    useToast: () => ({ add: toastAdd })
}))

import {
    useGeneratedCoverSettings,
    useUpdateGeneratedCoverSettings
} from '@/composables/useGeneratedCoverSettings'

function sampleSettings(): GeneratedCoverSettings {
    return {
        styles: [
            { name: 'classic', label: 'Classic' },
            { name: 'bauhaus', label: 'Bauhaus' }
        ],
        default: ['classic'],
        available: ['classic', 'bauhaus']
    }
}

const sampleInput: GeneratedCoverSettingsInput = {
    default: ['classic', 'bauhaus'],
    available: ['classic', 'bauhaus']
}

/** Mounts a query composable inside a real vue-query context and returns the query. */
function mountQuery<T>(composable: () => T) {
    const queryClient = new QueryClient()

    let query!: T
    const Comp = defineComponent({
        setup() {
            query = composable()
            return () => h('div')
        }
    })
    mount(Comp, {
        global: { plugins: [[VueQueryPlugin, { queryClient }]] }
    })
    return query
}

/** Mounts a mutation composable inside a real vue-query context and returns the mutation + the invalidate spy. */
function mountMutation<T>(composable: () => T) {
    const queryClient = new QueryClient()
    const invalidateSpy = vi.spyOn(queryClient, 'invalidateQueries')

    let mutation!: T
    const Comp = defineComponent({
        setup() {
            mutation = composable()
            return () => h('div')
        }
    })
    mount(Comp, {
        global: { plugins: [[VueQueryPlugin, { queryClient }]] }
    })
    return { mutation, invalidateSpy }
}

beforeEach(() => {
    getGeneratedCoverSettingsMock.mockReset()
    updateGeneratedCoverSettingsMock.mockReset()
    toastAdd.mockReset()
})

describe('useGeneratedCoverSettings', () => {
    it('calls getGeneratedCoverSettings and returns the data', async () => {
        getGeneratedCoverSettingsMock.mockResolvedValue(sampleSettings())
        const query = mountQuery(useGeneratedCoverSettings)

        // Wait for the query to resolve
        await vi.waitFor(() => {
            expect(query.isSuccess.value).toBe(true)
        })

        expect(getGeneratedCoverSettingsMock).toHaveBeenCalled()
        expect(query.data.value).toEqual(sampleSettings())
    })
})

describe('useUpdateGeneratedCoverSettings', () => {
    it('calls updateGeneratedCoverSettings with the input', async () => {
        updateGeneratedCoverSettingsMock.mockResolvedValue(sampleSettings())
        const { mutation } = mountMutation(useUpdateGeneratedCoverSettings)

        await mutation.mutateAsync(sampleInput)

        expect(updateGeneratedCoverSettingsMock).toHaveBeenCalledWith(sampleInput)
    })

    it('invalidates the generated-covers cache on success', async () => {
        updateGeneratedCoverSettingsMock.mockResolvedValue(sampleSettings())
        const { mutation, invalidateSpy } = mountMutation(useUpdateGeneratedCoverSettings)

        await mutation.mutateAsync(sampleInput)

        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['generated-covers'] })
    })

    it('invalidates the subsonic cache so placeholder covers refresh', async () => {
        updateGeneratedCoverSettingsMock.mockResolvedValue(sampleSettings())
        const { mutation, invalidateSpy } = mountMutation(useUpdateGeneratedCoverSettings)

        await mutation.mutateAsync(sampleInput)

        expect(invalidateSpy).toHaveBeenCalledWith({ queryKey: ['subsonic'] })
    })

    it('shows a success toast on successful update', async () => {
        updateGeneratedCoverSettingsMock.mockResolvedValue(sampleSettings())
        const { mutation } = mountMutation(useUpdateGeneratedCoverSettings)

        await mutation.mutateAsync(sampleInput)

        expect(toastAdd).toHaveBeenCalledWith({
            severity: 'success',
            summary: 'Generated covers updated',
            life: 3000
        })
    })

    it('shows an error toast on failure', async () => {
        updateGeneratedCoverSettingsMock.mockRejectedValue({
            response: { status: 500, data: { detail: 'boom' } }
        })
        const { mutation } = mountMutation(useUpdateGeneratedCoverSettings)

        await mutation.mutateAsync(sampleInput).catch(() => {})

        expect(toastAdd).toHaveBeenCalledWith(
            expect.objectContaining({
                severity: 'error',
                summary: 'Failed to update generated covers'
            })
        )
    })
})
