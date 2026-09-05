import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CreditListEditor from '@/views/settings/metadata-editor/CreditListEditor.vue'
import type { Pair } from '@/views/settings/metadata-editor/useEditForm'

vi.mock('@/components/library/MusicBrainzArtistPicker.vue', () => ({
    default: {
        name: 'MusicBrainzArtistPicker',
        props: ['visible', 'artistName', 'currentMbid'],
        emits: ['select', 'update:visible'],
        template: '<div class="picker-stub" />'
    }
}))

const stubs = {
    InputText: {
        props: ['modelValue'],
        template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Button: {
        props: ['label', 'icon'],
        inheritAttrs: false,
        template:
            '<button :aria-label="$attrs[\'aria-label\']" @click="$emit(\'click\')">{{ label }}</button>'
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

describe('CreditListEditor', () => {
    it('renders one pair per credit with name and mbid inputs', () => {
        const pairs: Pair[] = [
            { name: 'Artist One', mbid: 'id-1', mixed: false },
            { name: 'Artist Two', mbid: '', mixed: false }
        ]
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: pairs,
                mixed: false,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: 'Reset',
                mixedNote: 'Tracks have different artists'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        const nameInputs = wrapper.findAll('input.pair-name')
        const mbidInputs = wrapper.findAll('input.pair-mbid')
        expect(nameInputs).toHaveLength(2)
        expect(mbidInputs).toHaveLength(2)
        expect((nameInputs[0].element as HTMLInputElement).value).toBe('Artist One')
        expect((mbidInputs[0].element as HTMLInputElement).value).toBe('id-1')
    })

    it('shows the mixed note when mixed is true', () => {
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: [],
                mixed: true,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: '',
                mixedNote: 'Tracks have different artists'
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('.mixed-note').text()).toBe('Tracks have different artists')
    })

    it('hides the mixed note when mixed is false', () => {
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: [{ name: 'Same', mbid: '', mixed: false }],
                mixed: false,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: '',
                mixedNote: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        expect(wrapper.find('.mixed-note').exists()).toBe(false)
    })

    it('emits add when the add button is clicked', async () => {
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: [],
                mixed: false,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: '',
                mixedNote: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        const addButton = wrapper.findAll('button').find((b) => b.text() === 'Add artist')
        await addButton!.trigger('click')
        expect(wrapper.emitted('add')).toBeTruthy()
    })

    it('emits remove with the index when a remove button is clicked', async () => {
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: [
                    { name: 'First', mbid: '', mixed: false },
                    { name: 'Second', mbid: '', mixed: false }
                ],
                mixed: false,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: '',
                mixedNote: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        const removeButtons = wrapper.findAll('button[aria-label="Remove artist"]')
        await removeButtons[1].trigger('click')
        expect(wrapper.emitted('remove')).toBeTruthy()
        expect(wrapper.emitted('remove')![0]).toEqual([1])
    })

    it('emits stage when a pair input is edited', async () => {
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: [{ name: 'Artist', mbid: '', mixed: false }],
                mixed: false,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: '',
                mixedNote: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        await wrapper.find('input.pair-name').setValue('New Name')
        expect(wrapper.emitted('stage')).toBeTruthy()
    })


    it('opens the MusicBrainz picker when the search button is clicked', async () => {
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: [{ name: 'Artist', mbid: '', mixed: false }],
                mixed: false,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: '',
                mixedNote: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        await wrapper.find('button[aria-label="Search MusicBrainz"]').trigger('click')
        const picker = wrapper.findComponent({ name: 'MusicBrainzArtistPicker' })
        expect(picker.props('visible')).toBe(true)
    })

    it('emits pick with index and payload when the picker selects an artist', async () => {
        const wrapper = mount(CreditListEditor, {
            props: {
                modelValue: [
                    { name: 'First', mbid: '', mixed: false },
                    { name: 'Second', mbid: '', mixed: false }
                ],
                mixed: false,
                dirty: false,
                labels: {
                    heading: 'Artists',
                    namePlaceholder: 'Artist name',
                    mbidPlaceholder: 'MusicBrainz ID',
                    addLabel: 'Add artist',
                    removeAriaLabel: 'Remove artist'
                },
                undoTooltip: '',
                mixedNote: ''
            },
            global: { stubs, directives: { tooltip: tooltipRecorder } }
        })

        // Click search on the second pair
        const searchButtons = wrapper.findAll('button[aria-label="Search MusicBrainz"]')
        await searchButtons[1].trigger('click')

        const picker = wrapper.findComponent({ name: 'MusicBrainzArtistPicker' })
        picker.vm.$emit('select', { name: 'The Beatles', mbid: 'id-beatles' })
        await wrapper.vm.$nextTick()

        expect(wrapper.emitted('pick')).toBeTruthy()
        expect(wrapper.emitted('pick')![0]).toEqual([
            1,
            { name: 'The Beatles', mbid: 'id-beatles' }
        ])
    })
})
