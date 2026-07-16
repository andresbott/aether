import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import StationSearchDialog from '@/components/library/StationSearchDialog.vue'
import type { RadioBrowserStation } from '@/types/radiobrowser'

const searchSpy = vi.hoisted(() => vi.fn())
// Fake refs (__v_isRef) so Vue's template unwraps `.value`, matching the real
// composable's return shape. Tests mutate `.value` to drive render state.
const rbState = vi.hoisted(() => ({
    results: { __v_isRef: true, value: [] as any[] },
    loading: { __v_isRef: true, value: false },
    error: { __v_isRef: true, value: null as string | null }
}))

vi.mock('@/composables/useRadioBrowserSearch', () => ({
    useRadioBrowserSearch: () => ({
        results: rbState.results,
        loading: rbState.loading,
        error: rbState.error,
        search: searchSpy
    })
}))

const stubs = {
    Dialog: { template: '<div><slot /><slot name="footer" /></div>' },
    InputText: {
        props: ['modelValue'],
        template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Button: {
        props: ['label', 'disabled'],
        template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>'
    },
    Message: { template: '<div><slot /></div>' }
}

const station: RadioBrowserStation = {
    name: 'BBC Radio 1',
    streamUrl: 'http://bbc/stream',
    homepage: 'https://bbc.co.uk',
    favicon: 'https://bbc.co.uk/favicon.png',
    tags: 'pop,uk',
    country: 'United Kingdom',
    countryCode: 'GB',
    language: 'english',
    codec: 'MP3',
    bitrate: 128,
    votes: 4200,
    uuid: 'uuid-1'
}

describe('StationSearchDialog', () => {
    beforeEach(() => {
        searchSpy.mockClear()
        rbState.results.value = []
        rbState.loading.value = false
        rbState.error.value = null
    })

    it('stages a station and emits it on OK (Add stays disabled until picked)', async () => {
        rbState.results.value = [station]
        const wrapper = mount(StationSearchDialog, {
            props: { visible: true },
            global: { stubs }
        })
        const addBtn = () => wrapper.find('[data-test="rb-confirm"]')
        // Nothing picked yet: Add is disabled.
        expect(addBtn().attributes('disabled')).toBeDefined()

        // Clicking a result only stages it — no emit, dialog stays open.
        await wrapper.find('.result-row').trigger('click')
        expect(wrapper.emitted('select')).toBeUndefined()
        expect(addBtn().attributes('disabled')).toBeUndefined()

        await addBtn().trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual([station])
        expect(wrapper.emitted('update:visible')?.[0]).toEqual([false])
    })

    it('shows a hint before searching and "No matches" after an empty search', async () => {
        const wrapper = mount(StationSearchDialog, { props: { visible: true }, global: { stubs } })
        expect(wrapper.find('.hint').exists()).toBe(true)
        expect(wrapper.find('.no-results').exists()).toBe(false)

        await wrapper.find('[data-test="rb-query"]').setValue('bbc')
        expect(wrapper.find('.hint').exists()).toBe(false)
        expect(wrapper.find('.no-results').exists()).toBe(true)
    })

    describe('debounced search', () => {
        beforeEach(() => vi.useFakeTimers())
        afterEach(() => vi.useRealTimers())

        it('runs a debounced search when the query changes', async () => {
            const wrapper = mount(StationSearchDialog, {
                props: { visible: true },
                global: { stubs }
            })
            await wrapper.find('[data-test="rb-query"]').setValue('jazz')
            expect(searchSpy).not.toHaveBeenCalled()
            await vi.advanceTimersByTimeAsync(500)
            expect(searchSpy).toHaveBeenCalledTimes(1)
            expect(searchSpy).toHaveBeenCalledWith('jazz')
        })
    })
})
