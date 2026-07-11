import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'

const searchSpy = vi.hoisted(() => vi.fn())

vi.mock('@/composables/useMusicBrainzSearch', () => ({
    useMusicBrainzSearch: () => ({
        results: { value: [] },
        loading: { value: false },
        error: { value: null },
        search: searchSpy
    })
}))

const stubs = {
    Dialog: { template: '<div><slot /></div>' },
    InputText: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
    Button: { props: ['label', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>' },
    Message: { template: '<div><slot /></div>' }
}

describe('MusicBrainzArtistPicker', () => {
    beforeEach(() => {
        searchSpy.mockClear()
    })

    it('emits select with a valid pasted MBID and rejects an invalid one', async () => {
        const valid = '056e4f3e-d505-4dad-8ec1-d04f521cbb56'
        const wrapper = mount(MusicBrainzArtistPicker, {
            props: { visible: true, artistName: 'X', currentMbid: '' },
            global: { stubs }
        })
        const paste = wrapper.find('[data-test="mbid-paste"]')
        await paste.setValue('not-a-uuid')
        expect(wrapper.find('[data-test="mbid-apply"]').attributes('disabled')).toBeDefined()
        await paste.setValue(valid)
        expect(wrapper.find('[data-test="mbid-apply"]').attributes('disabled')).toBeUndefined()
        await wrapper.find('[data-test="mbid-apply"]').trigger('click')
        expect(wrapper.emitted('select')?.[0]).toEqual([valid])
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
