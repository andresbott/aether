import { describe, it, expect, vi, beforeAll } from 'vitest'
import { mount } from '@vue/test-utils'
import type { InternetRadioStation } from '@/types/subsonic'
import type { RadioStationPrefill } from '@/types/radiobrowser'
import RadioStationForm from '@/components/library/RadioStationForm.vue'

vi.mock('@/lib/api/subsonic', () => ({
    subsonicClient: { getCoverArtUrl: (id: string) => `/cover/${id}` }
}))

beforeAll(() => {
    globalThis.URL.createObjectURL = vi.fn(() => 'blob:mock')
    globalThis.URL.revokeObjectURL = vi.fn()
})

const stubs = {
    InputText: {
        props: ['modelValue'],
        template:
            '<input :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
    },
    Button: {
        props: ['label'],
        inheritAttrs: false,
        template: '<button @click="$emit(\'click\')">{{ label }}</button>'
    },
    FileUpload: { name: 'FileUpload', emits: ['select'], template: '<div class="file-upload" />' },
    Message: { template: '<div class="p-message"><slot /></div>' }
}

const station: InternetRadioStation = {
    id: 'r-1',
    name: 'BBC Radio 1',
    streamUrl: 'http://bbc/stream',
    homepageUrl: 'http://bbc/home',
    coverArt: 'r-1'
}

const mountForm = (props: Partial<{ station: InternetRadioStation | null; prefill: RadioStationPrefill | null }>) =>
    mount(RadioStationForm, { props: { station: null, ...props }, global: { stubs } })

// The last change payload the form emitted.
const last = (w: ReturnType<typeof mountForm>) => {
    const events = w.emitted('change') as Array<[{ input: any; valid: boolean; dirty: boolean }]>
    return events[events.length - 1][0]
}

describe('RadioStationForm', () => {
    it('starts invalid and not dirty when blank', () => {
        const w = mountForm({ station: null })
        expect(last(w).valid).toBe(false)
        expect(last(w).dirty).toBe(false)
    })

    it('becomes valid once name and stream URL are present, trimming them', async () => {
        const w = mountForm({ station: null })
        await w.find('input.field-name').setValue('  Jazz FM  ')
        await w.find('input.field-stream-url').setValue('  http://jazz  ')
        const p = last(w)
        expect(p.valid).toBe(true)
        expect(p.dirty).toBe(true)
        expect(p.input.name).toBe('Jazz FM')
        expect(p.input.streamUrl).toBe('http://jazz')
        expect(p.input.homepageUrl).toBeUndefined()
    })

    it('pre-fills fields from an existing station and stays clean until edited', async () => {
        const w = mountForm({ station })
        expect((w.find('input.field-name').element as HTMLInputElement).value).toBe('BBC Radio 1')
        expect(last(w).dirty).toBe(false)
        await w.find('input.field-name').setValue('BBC R1')
        expect(last(w).dirty).toBe(true)
        expect(last(w).input.name).toBe('BBC R1')
    })

    it('seeds fields and cover from a prefill', () => {
        const file = new File(['x'], 'favicon.png', { type: 'image/png' })
        const prefill: RadioStationPrefill = {
            name: 'Radio Paradise',
            streamUrl: 'http://rp/stream',
            homepageUrl: 'https://radioparadise.com',
            coverFile: file
        }
        const w = mountForm({ station: null, prefill })
        const p = last(w)
        expect(p.input.name).toBe('Radio Paradise')
        expect(p.input.coverFile).toBe(file)
    })

    it('rejects an oversized cover and stays invalid', async () => {
        const w = mountForm({ station: null })
        await w.find('input.field-name').setValue('Jazz')
        await w.find('input.field-stream-url').setValue('http://jazz')
        const big = new File([new Uint8Array(6 * 1024 * 1024)], 'big.png', { type: 'image/png' })
        w.findComponent({ name: 'FileUpload' }).vm.$emit('select', { files: [big] })
        await w.vm.$nextTick()
        expect(last(w).valid).toBe(false)
        expect(w.find('.p-message').text()).toContain('max is 5 MB')
    })

    it('includes a chosen cover file in the input', async () => {
        const w = mountForm({ station: null })
        await w.find('input.field-name').setValue('Jazz')
        await w.find('input.field-stream-url').setValue('http://jazz')
        const file = new File(['x'], 'cover.png', { type: 'image/png' })
        w.findComponent({ name: 'FileUpload' }).vm.$emit('select', { files: [file] })
        await w.vm.$nextTick()
        expect(last(w).input.coverFile).toBe(file)
    })

    it('stages a cover clear when the existing cover is removed', async () => {
        const w = mountForm({ station })
        const removeBtn = w.findAll('button').find((b) => b.text() === 'Remove cover')!
        await removeBtn.trigger('click')
        expect(last(w).input.coverClear).toBe(true)
        expect(last(w).dirty).toBe(true)
    })

    it('does not revert in-progress edits when the prefill later gains a favicon cover', async () => {
        const textOnlyPrefill: RadioStationPrefill = {
            name: 'Radio Paradise',
            streamUrl: 'http://rp/stream',
            homepageUrl: 'https://radioparadise.com'
        }
        const w = mountForm({ station: null, prefill: textOnlyPrefill })

        await w.find('input.field-name').setValue('My Custom Name')

        const file = new File(['x'], 'favicon.png', { type: 'image/png' })
        await w.setProps({ prefill: { ...textOnlyPrefill, coverFile: file } })

        const p = last(w)
        expect(p.input.name).toBe('My Custom Name')
        expect(p.input.coverFile).toBe(file)
    })
})
