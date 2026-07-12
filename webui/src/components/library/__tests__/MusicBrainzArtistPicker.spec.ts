import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'

const searchSpy = vi.hoisted(() => vi.fn())
// Fake refs (__v_isRef) so Vue's template unwraps `.value`, matching the real
// composable's return shape. Tests mutate `.value` to drive render state.
const mbState = vi.hoisted(() => ({
    results: { __v_isRef: true, value: [] as any[] },
    loading: { __v_isRef: true, value: false },
    error: { __v_isRef: true, value: null as string | null }
}))

vi.mock('@/composables/useMusicBrainzSearch', () => ({
    useMusicBrainzSearch: () => ({
        results: mbState.results,
        loading: mbState.loading,
        error: mbState.error,
        search: searchSpy
    })
}))

const stubs = {
    Dialog: { template: '<div><slot /><slot name="footer" /></div>' },
    InputText: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
    Button: { props: ['label', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>' },
    Message: { template: '<div><slot /></div>' }
}

describe('MusicBrainzArtistPicker', () => {
    beforeEach(() => {
        searchSpy.mockClear()
        mbState.results.value = []
        mbState.loading.value = false
        mbState.error.value = null
    })

    it('stages a pasted MBID (no name) in the combined box and applies it on OK', async () => {
        const valid = '056e4f3e-d505-4dad-8ec1-d04f521cbb56'
        const wrapper = mount(MusicBrainzArtistPicker, {
            props: { visible: true, artistName: 'X', currentMbid: '' },
            global: { stubs }
        })
        const query = wrapper.find('[data-test="mbid-query"]')
        const okBtn = () => wrapper.find('[data-test="mbid-confirm"]')
        // A non-MBID name query is not a complete selection: OK stays disabled.
        await query.setValue('not-a-uuid')
        expect(okBtn().attributes('disabled')).toBeDefined()
        // A valid MBID is detected and staged: OK enables.
        await query.setValue(valid)
        expect(okBtn().attributes('disabled')).toBeUndefined()
        await okBtn().trigger('click')
        // A bare ID carries no name, so none is emitted (name left unchanged).
        expect(wrapper.emitted('select')?.[0]).toEqual([valid, undefined])
    })

    it('stages a search result and, on OK, emits both its MBID and name', async () => {
        const mbid = '056e4f3e-d505-4dad-8ec1-d04f521cbb56'
        mbState.results.value = [{ mbid, name: 'The Beatles' }]
        const wrapper = mount(MusicBrainzArtistPicker, {
            props: { visible: true, artistName: 'Beatles', currentMbid: '' },
            global: { stubs }
        })
        // Clicking a result only stages it — the dialog does not close yet.
        await wrapper.find('.result-row').trigger('click')
        expect(wrapper.emitted('select')).toBeUndefined()
        expect(wrapper.find('[data-test="mbid-confirm"]').attributes('disabled')).toBeUndefined()
        await wrapper.find('[data-test="mbid-confirm"]').trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual([mbid, 'The Beatles'])
    })

    describe('search on open', () => {
        beforeEach(() => {
            vi.useFakeTimers()
        })

        afterEach(() => {
            vi.useRealTimers()
        })

        it('searches exactly once when the dialog opens', async () => {
            mount(MusicBrainzArtistPicker, {
                props: { visible: true, artistName: 'X', currentMbid: '' },
                global: { stubs }
            })
            await vi.advanceTimersByTimeAsync(500)
            expect(searchSpy).toHaveBeenCalledTimes(1)
        })
    })
})
