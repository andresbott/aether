import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import MusicBrainzArtistPicker from '@/components/library/MusicBrainzArtistPicker.vue'

vi.mock('@/composables/useMusicBrainzSearch', () => ({
    useMusicBrainzSearch: () => ({
        results: { value: [] },
        loading: { value: false },
        error: { value: null },
        search: vi.fn()
    })
}))

const stubs = {
    Dialog: { template: '<div><slot /></div>' },
    InputText: { props: ['modelValue'], template: '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />' },
    Button: { props: ['label', 'disabled'], template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>' },
    Message: { template: '<div><slot /></div>' }
}

describe('MusicBrainzArtistPicker', () => {
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
})
