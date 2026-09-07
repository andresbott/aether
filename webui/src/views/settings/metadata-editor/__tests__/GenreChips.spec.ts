import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import GenreChips from '@/views/settings/metadata-editor/GenreChips.vue'

const stubs = {
    AutoComplete: {
        name: 'AutoComplete',
        props: ['modelValue', 'multiple', 'typeahead'],
        template: '<div class="genres-autocomplete">{{ (modelValue ?? []).join(",") }}</div>'
    },
    Button: {
        props: ['icon', 'text', 'size'],
        inheritAttrs: false,
        template:
            '<button :aria-label="$attrs[\'aria-label\']" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')"></button>'
    }
}

const tooltipRecorder = {
    mounted(el: HTMLElement, binding: { value: unknown }) {
        el.setAttribute('data-tooltip', String(binding.value ?? ''))
    },
    updated(el: HTMLElement, binding: { value: unknown }) {
        el.setAttribute('data-tooltip', String(binding.value ?? ''))
    }
}

describe('GenreChips', () => {
    it('renders the genre list in the AutoComplete', () => {
        const wrapper = mount(GenreChips, {
            props: {
                modelValue: ['Rock', 'Jazz'],
                mixed: false,
                dirty: false,
                undoTooltip: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('.genres-autocomplete').text()).toBe('Rock,Jazz')
    })

    it('shows the mixed note when mixed is true', () => {
        const wrapper = mount(GenreChips, {
            props: {
                modelValue: [],
                mixed: true,
                dirty: false,
                undoTooltip: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('.mixed-note').exists()).toBe(true)
        expect(wrapper.find('.mixed-note').text()).toContain('different genres')
    })

    it('hides the mixed note when mixed is false', () => {
        const wrapper = mount(GenreChips, {
            props: {
                modelValue: ['Pop'],
                mixed: false,
                dirty: false,
                undoTooltip: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('.mixed-note').exists()).toBe(false)
    })

    it('emits update:modelValue when the AutoComplete value changes', async () => {
        const wrapper = mount(GenreChips, {
            props: {
                modelValue: ['Rock'],
                mixed: false,
                dirty: false,
                undoTooltip: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        wrapper.findComponent({ name: 'AutoComplete' }).vm.$emit('update:modelValue', ['Rock', 'Jazz'])
        await wrapper.vm.$nextTick()
        expect(wrapper.emitted('update:modelValue')).toBeTruthy()
        expect(wrapper.emitted('update:modelValue')![0]).toEqual([['Rock', 'Jazz']])
    })

    it('shows the undo button when dirty is true', () => {
        const wrapper = mount(GenreChips, {
            props: {
                modelValue: ['Rock'],
                mixed: false,
                dirty: true,
                undoTooltip: 'Revert to Pop'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        const undoBtn = wrapper.find('[data-test="undo-genres"]')
        expect(undoBtn.exists()).toBe(true)
        expect(undoBtn.attributes('data-tooltip')).toBe('Revert to Pop')
    })

    it('hides the undo button when dirty is false', () => {
        const wrapper = mount(GenreChips, {
            props: {
                modelValue: ['Rock'],
                mixed: false,
                dirty: false,
                undoTooltip: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('[data-test="undo-genres"]').exists()).toBe(false)
    })

    it('emits undo when the undo button is clicked', async () => {
        const wrapper = mount(GenreChips, {
            props: {
                modelValue: ['Rock'],
                mixed: false,
                dirty: true,
                undoTooltip: 'Revert'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        await wrapper.find('[data-test="undo-genres"]').trigger('click')
        expect(wrapper.emitted('undo')).toBeTruthy()
    })
})
