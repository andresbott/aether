import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import FieldRow from '@/views/settings/metadata-editor/FieldRow.vue'

const stubs = {
    InputText: {
        props: ['modelValue', 'disabled', 'placeholder'],
        template:
            '<input :disabled="disabled" :placeholder="placeholder" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    InputNumber: {
        props: ['modelValue', 'disabled', 'placeholder'],
        template:
            '<input :disabled="disabled" :placeholder="placeholder" :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value === \'\' ? null : Number($event.target.value))" />'
    },
    Button: {
        props: ['icon', 'text', 'size', 'disabled'],
        inheritAttrs: false,
        template:
            '<button :disabled="disabled" :aria-label="$attrs[\'aria-label\']" :data-test="$attrs[\'data-test\']" @click="$emit(\'click\')"></button>'
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

describe('FieldRow', () => {
    it('renders label and text input with modelValue', () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Title',
                id: 'title',
                modelValue: 'Some Title',
                type: 'text',
                dirty: false,
                undoTooltip: '',
                undoTestId: 'undo-title',
                undoAriaLabel: 'Reset title'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('label').text()).toBe('Title')
        expect((wrapper.find('input').element as HTMLInputElement).value).toBe('Some Title')
    })

    it('typing in the input emits update:modelValue', async () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Album',
                id: 'album',
                modelValue: '',
                type: 'text',
                dirty: false,
                undoTooltip: '',
                undoTestId: 'undo-album',
                undoAriaLabel: 'Reset album'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        await wrapper.find('input').setValue('New Album')
        expect(wrapper.emitted('update:modelValue')).toBeTruthy()
        expect(wrapper.emitted('update:modelValue')![0]).toEqual(['New Album'])
    })

    it('shows the undo button when dirty is true', () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Year',
                id: 'year',
                modelValue: 2023,
                type: 'number',
                dirty: true,
                undoTooltip: 'Reset to original value',
                undoTestId: 'undo-year',
                undoAriaLabel: 'Reset year'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        const undoBtn = wrapper.find('[data-test="undo-year"]')
        expect(undoBtn.exists()).toBe(true)
        expect(undoBtn.attributes('aria-label')).toBe('Reset year')
        expect(undoBtn.attributes('data-tooltip')).toBe('Reset to original value')
    })

    it('hides the undo button when dirty is false', () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Year',
                id: 'year',
                modelValue: 2023,
                type: 'number',
                dirty: false,
                undoTooltip: '',
                undoTestId: 'undo-year',
                undoAriaLabel: 'Reset year'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('[data-test="undo-year"]').exists()).toBe(false)
    })

    it('clicking the undo button emits undo', async () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Album',
                id: 'album',
                modelValue: 'Changed',
                type: 'text',
                dirty: true,
                undoTooltip: 'Revert',
                undoTestId: 'undo-album',
                undoAriaLabel: 'Reset album'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        await wrapper.find('[data-test="undo-album"]').trigger('click')
        expect(wrapper.emitted('undo')).toBeTruthy()
        expect(wrapper.emitted('undo')![0]).toEqual([])
    })

    it('renders InputNumber when type is number', () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Track Number',
                id: 'track_number',
                modelValue: 5,
                type: 'number',
                dirty: false,
                undoTooltip: '',
                undoTestId: 'undo-track_number',
                undoAriaLabel: 'Reset track number'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect((wrapper.find('input').element as HTMLInputElement).value).toBe('5')
    })

    it('emits null when a numeric input is emptied', async () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Year',
                id: 'year',
                modelValue: 2020,
                type: 'number',
                dirty: false,
                undoTooltip: '',
                undoTestId: 'undo-year',
                undoAriaLabel: 'Reset year'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        await wrapper.find('input').setValue('')
        expect(wrapper.emitted('update:modelValue')).toBeTruthy()
        expect(wrapper.emitted('update:modelValue')![0]).toEqual([null])
    })

    it('forwards placeholder to the input', () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Album',
                id: 'album',
                modelValue: '',
                type: 'text',
                placeholder: '(multiple values)',
                dirty: false,
                undoTooltip: '',
                undoTestId: 'undo-album',
                undoAriaLabel: 'Reset album'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('input').attributes('placeholder')).toBe('(multiple values)')
    })

    it('forwards disabled to the input', () => {
        const wrapper = mount(FieldRow, {
            props: {
                label: 'Title',
                id: 'title',
                modelValue: 'Title',
                type: 'text',
                disabled: true,
                dirty: false,
                undoTooltip: '',
                undoTestId: 'undo-title',
                undoAriaLabel: 'Reset title'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('input').attributes('disabled')).toBeDefined()
    })
})
