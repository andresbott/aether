import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import AlbumCoverPicker from '@/components/library/AlbumCoverPicker.vue'

const searchSpy = vi.hoisted(() => vi.fn())
const caState = vi.hoisted(() => ({
    candidates: { __v_isRef: true, value: [] as any[] },
    loading: { __v_isRef: true, value: false },
    error: { __v_isRef: true, value: null as string | null }
}))
vi.mock('@/composables/useCoverArtSearch', () => ({
    useCoverArtSearch: () => ({
        candidates: caState.candidates,
        loading: caState.loading,
        error: caState.error,
        search: searchSpy
    })
}))

const stubs = {
    Dialog: { template: '<div><slot /><slot name="footer" /></div>' },
    Button: {
        props: ['label', 'disabled', 'loading'],
        inheritAttrs: false,
        template: '<button :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>'
    },
    Message: { template: '<div><slot /></div>' },
    RadioButton: {
        props: ['modelValue', 'value', 'disabled', 'inputId'],
        template:
            '<input type="radio" :id="inputId" :disabled="disabled" @change="$emit(\'update:modelValue\', value)" />'
    }
}

const mkCandidate = (over: Record<string, any> = {}) => ({
    id: '1',
    thumbUrl: 'http://img/f-250.jpg',
    imageUrl: 'http://img/f.jpg',
    isFront: true,
    types: ['Front'],
    comment: '',
    ...over
})

const baseProps = {
    visible: true,
    albumName: 'OK Computer',
    releaseMbid: 'rel-1',
    releaseGroupMbid: 'rg-1',
    libraryId: 3,
    paths: ['t1.flac', 't2.flac']
}

describe('AlbumCoverPicker', () => {
    beforeEach(() => {
        searchSpy.mockClear()
        caState.candidates.value = []
        caState.loading.value = false
        caState.error.value = null
    })

    it('searches Cover Art Archive by release + release-group id on open', () => {
        mount(AlbumCoverPicker, { props: baseProps, global: { stubs } })
        expect(searchSpy).toHaveBeenCalledWith('rel-1', 'rg-1')
    })

    it('renders a list of candidates with type descriptions and comments', () => {
        caState.candidates.value = [
            mkCandidate(),
            mkCandidate({ id: '2', isFront: false, types: ['Back', 'Booklet'], comment: 'digipak' })
        ]
        const wrapper = mount(AlbumCoverPicker, { props: baseProps, global: { stubs } })
        const rows = wrapper.findAll('.cover-row')
        expect(rows).toHaveLength(2)
        expect(rows[0].text()).toContain('Front')
        expect(rows[1].text()).toContain('Back, Booklet')
        expect(rows[1].text()).toContain('digipak')
    })

    it('stages a selected candidate (image_url + target) without persisting', async () => {
        caState.candidates.value = [mkCandidate()]
        const wrapper = mount(AlbumCoverPicker, { props: baseProps, global: { stubs } })

        await wrapper.find('.cover-row').trigger('click')
        const selectBtn = wrapper.findAll('button').find((b) => b.text() === 'Select')
        await selectBtn!.trigger('click')

        expect(wrapper.emitted('select')?.[0]?.[0]).toEqual({
            target: 'db',
            file: null,
            imageUrl: 'http://img/f.jpg'
        })
        expect(wrapper.emitted('update:visible')?.[0]).toEqual([false])
    })

    it('disables the embedded target when no tracks are selected', () => {
        const wrapper = mount(AlbumCoverPicker, {
            props: { ...baseProps, paths: [] },
            global: { stubs }
        })
        const embedded = wrapper.find('#cover-target-embedded')
        expect((embedded.element as HTMLInputElement).disabled).toBe(true)
    })

    it('does not emit a selection without a chosen image source', async () => {
        caState.candidates.value = [mkCandidate()]
        const wrapper = mount(AlbumCoverPicker, { props: baseProps, global: { stubs } })
        const selectBtn = wrapper.findAll('button').find((b) => b.text() === 'Select')
        await selectBtn!.trigger('click')
        expect(wrapper.emitted('select')).toBeFalsy()
    })
})
